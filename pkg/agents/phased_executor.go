package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/soypete/pedro-agentware/middleware"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AllTools is a sentinel value for Tools field indicating all tools are available
const AllTools = "*"

// Phase represents a single phase in a phased workflow
type Phase struct {
	Name         string   // Phase identifier (e.g., "analyze", "plan", "implement")
	Description  string   // Human-readable description
	SystemPrompt string   // Custom system prompt for this phase
	Tools        []string // Tool filtering: []string{AllTools} = all, []string{} = none, []string{"tool1"} = specific
	MaxRounds    int      // Max inference rounds for this phase (0 = use default)
	// Validator validates the phase output and returns error if invalid
	Validator func(result *PhaseResult) error
	// Optional: allow the phase to produce structured output
	ExpectsJSON bool
	// PhaseGenerator dynamically creates new phases based on this phase's result
	// Used for workflows where the number of phases is determined at runtime
	// (e.g., generating N sections from an outline)
	PhaseGenerator func(result *PhaseResult) ([]Phase, error)
}

// PhaseResult contains the result of executing a phase
type PhaseResult struct {
	PhaseName   string                 `json:"phase_name"`
	Success     bool                   `json:"success"`
	Output      string                 `json:"output"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at"`
	RoundsUsed  int                    `json:"rounds_used"`
}

// Checkpoint represents a saved workflow state for resume capability
type Checkpoint struct {
	Version         int                     `json:"checkpoint_version"` // Version for future compatibility
	JobID           string                  `json:"job_id"`
	CreatedAt       time.Time               `json:"created_at"`
	CurrentPhase    int                     `json:"current_phase"`    // Index of next phase to execute
	CompletedPhases []string                `json:"completed_phases"` // Names of completed phases
	PhaseResults    map[string]*PhaseResult `json:"phase_results"`    // Results of completed phases
	LastInput       string                  `json:"last_input"`       // Input for next phase
	TotalPhases     int                     `json:"total_phases"`     // Total number of phases (including generated)
	GeneratedPhases int                     `json:"generated_phases"` // Number of dynamically generated phases
}

// PhaseCallback is called after each phase completes
// Return true to continue, false to stop execution
type PhaseCallback func(phase Phase, result *PhaseResult) (shouldContinue bool, err error)

// PhasedExecutor handles multi-phase workflow execution
type PhasedExecutor struct {
	agent             *BaseAgent
	contextMgr        *llmcontext.Manager
	phases            []Phase
	phaseResults      map[string]*PhaseResult
	currentPhase      int
	jobID             string
	defaultMaxRounds  int
	phaseCallback     PhaseCallback // Optional callback after each phase
	initialPhaseCount int           // Number of phases at creation (before dynamic generation)
}

// NewPhasedExecutor creates a new phased executor
func NewPhasedExecutor(agent *BaseAgent, contextMgr *llmcontext.Manager, phases []Phase) *PhasedExecutor {
	return &PhasedExecutor{
		agent:             agent,
		contextMgr:        contextMgr,
		phases:            phases,
		phaseResults:      make(map[string]*PhaseResult),
		currentPhase:      0,
		jobID:             contextMgr.GetJobID(),
		defaultMaxRounds:  agent.config.Limits.MaxInferenceRuns,
		phaseCallback:     nil,
		initialPhaseCount: len(phases), // Track initial count for generated phase tracking
	}
}

// SetPhaseCallback sets a callback to be called after each phase completes
func (pe *PhasedExecutor) SetPhaseCallback(callback PhaseCallback) {
	pe.phaseCallback = callback
}

// Execute runs all phases sequentially
func (pe *PhasedExecutor) Execute(ctx context.Context, initialInput string) error {
	// Check if a phase callback is provided via context
	if callback, ok := GetPhaseCallback(ctx); ok {
		pe.SetPhaseCallback(callback)
	}

	currentInput := initialInput

	for pe.currentPhase < len(pe.phases) {
		phase := pe.phases[pe.currentPhase]

		fmt.Fprintf(os.Stderr, "\n📋 Phase %d/%d: %s\n", pe.currentPhase+1, len(pe.phases), phase.Name)
		fmt.Fprintf(os.Stderr, "   %s\n", phase.Description)

		// Show registered tools for debugging (first phase only)
		if pe.currentPhase == 0 && pe.agent.config.Debug.Enabled {
			toolCount := len(pe.agent.tools)
			if pe.agent.registry != nil {
				toolCount = len(pe.agent.registry.List())
			}
			fmt.Fprintf(os.Stderr, "   [DEBUG] Registered tools: %d", toolCount)
			if toolCount > 0 {
				toolNames := []string{}
				if pe.agent.registry != nil {
					for _, t := range pe.agent.registry.List() {
						toolNames = append(toolNames, t.Name())
					}
				} else {
					for name := range pe.agent.tools {
						toolNames = append(toolNames, name)
					}
				}
				fmt.Fprintf(os.Stderr, " (%v)", toolNames)
			}
			fmt.Fprintf(os.Stderr, "\n")
			fmt.Fprintf(os.Stderr, "   [DEBUG] Tool calling enabled: %v\n", pe.agent.config.Model.EnableTools)
		}

		// Update job with current phase
		pe.updateJobPhase(ctx, phase.Name)

		// Execute the phase
		result, err := pe.executePhase(ctx, phase, currentInput)
		if err != nil {
			result = &PhaseResult{
				PhaseName:   phase.Name,
				Success:     false,
				Error:       err.Error(),
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}
			pe.phaseResults[phase.Name] = result
			pe.savePhaseResults(ctx)
			return fmt.Errorf("phase %s failed: %w", phase.Name, err)
		}

		// Validate phase result if validator is provided
		if phase.Validator != nil {
			if err := phase.Validator(result); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("validation failed: %v", err)
				pe.phaseResults[phase.Name] = result
				pe.savePhaseResults(ctx)
				return fmt.Errorf("phase %s validation failed: %w", phase.Name, err)
			}
		}

		pe.phaseResults[phase.Name] = result
		pe.savePhaseResults(ctx)

		fmt.Fprintf(os.Stderr, "   ✅ Phase %s completed in %d rounds\n", phase.Name, result.RoundsUsed)

		// Call phase callback if set (for interactive stepwise mode)
		if pe.phaseCallback != nil {
			shouldContinue, err := pe.phaseCallback(phase, result)
			if err != nil {
				return fmt.Errorf("phase callback error: %w", err)
			}
			if !shouldContinue {
				return fmt.Errorf("execution stopped by user")
			}
		}

		// Check if this phase generates additional phases dynamically
		if phase.PhaseGenerator != nil {
			generatedPhases, err := phase.PhaseGenerator(result)
			if err != nil {
				return fmt.Errorf("phase generator failed for %s: %w", phase.Name, err)
			}

			if len(generatedPhases) > 0 {
				fmt.Fprintf(os.Stderr, "   📋 Generated %d dynamic phases from %s\n", len(generatedPhases), phase.Name)

				// Insert generated phases immediately after current phase
				pe.phases = insertPhases(pe.phases, pe.currentPhase+1, generatedPhases)

				if pe.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "   [DEBUG] Total phases: %d (added %d)\n",
						len(pe.phases), len(generatedPhases))
				}
			}
		}

		// Use phase output as input for next phase
		currentInput = pe.buildNextPhaseInput(result)
		pe.currentPhase++

		// Save checkpoint after phase completion
		if err := pe.SaveCheckpoint(currentInput); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️ Failed to save checkpoint: %v\n", err)
			// Continue execution even if checkpoint fails
		}
	}

	fmt.Fprintf(os.Stderr, "\n✅ All %d phases completed successfully!\n", len(pe.phases))
	return nil
}

// ExecutePhase executes a single phase and returns the result
func (pe *PhasedExecutor) executePhase(ctx context.Context, phase Phase, input string) (*PhaseResult, error) {
	result := &PhaseResult{
		PhaseName: phase.Name,
		StartedAt: time.Now(),
		Data:      make(map[string]interface{}),
	}

	// Determine max rounds for this phase
	maxRounds := pe.defaultMaxRounds
	if phase.MaxRounds > 0 {
		maxRounds = phase.MaxRounds
	}

	// Create a phase-specific inference executor
	executor := &phaseInferenceExecutor{
		agent:        pe.agent,
		contextMgr:   pe.contextMgr,
		phase:        phase,
		maxRounds:    maxRounds,
		currentRound: 0,
		jobID:        pe.jobID,
		result:       result,                      // Pass result so executor can track tool calls
		callHistory:  middleware.NewCallHistory(), // Track tool calls and failures via middleware
	}

	// Execute the inference loop for this phase
	output, rounds, err := executor.execute(ctx, input)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.CompletedAt = time.Now()
		result.RoundsUsed = rounds
		return result, err
	}

	result.Success = true
	result.Output = output
	result.CompletedAt = time.Now()
	result.RoundsUsed = rounds

	// If phase expects JSON, try to parse it
	if phase.ExpectsJSON {
		if data, err := extractJSONData(output); err == nil {
			result.Data = data
		}
	}

	return result, nil
}

// GetPhaseResult returns the result for a specific phase
func (pe *PhasedExecutor) GetPhaseResult(phaseName string) (*PhaseResult, bool) {
	result, ok := pe.phaseResults[phaseName]
	return result, ok
}

// GetAllResults returns all phase results
func (pe *PhasedExecutor) GetAllResults() map[string]*PhaseResult {
	return pe.phaseResults
}

// GetCurrentPhase returns the current phase index
func (pe *PhasedExecutor) GetCurrentPhase() int {
	return pe.currentPhase
}

// sanitizePhaseOutput removes context pollution from phase outputs.
// It strips:
// - JSON blocks (fenced with ```json or bare objects)
// - File paths (lines containing .go, .js, .py, etc.)
// - Tool call examples (lines with {"tool":)
// - Code snippets (fenced code blocks)
// Keeps:
// - Plain text summaries
// - High-level descriptions
// - Phase completion markers
func sanitizePhaseOutput(output string, phaseName string) string {
	if output == "" {
		return output
	}

	// Special handling for specific phases
	switch phaseName {
	case "plan":
		// Plan output is pure JSON - extract summary only
		return sanitizePlanOutput(output)
	case "analyze":
		// Analyze may have code snippets - remove them
		return sanitizeAnalyzeOutput(output)
	case "implement":
		// Implement may have tool call examples - remove them
		return sanitizeImplementOutput(output)
	default:
		// Generic sanitization for other phases
		return sanitizeGenericOutput(output)
	}
}

// sanitizePlanOutput extracts high-level summary from plan JSON
func sanitizePlanOutput(output string) string {
	// Try to extract just the title and step count
	var summary strings.Builder
	summary.WriteString("A detailed implementation plan was created.\n\n")

	// Count steps if JSON is parseable
	var planData map[string]interface{}
	if err := json.Unmarshal([]byte(output), &planData); err == nil {
		if plan, ok := planData["plan"].(map[string]interface{}); ok {
			if title, ok := plan["title"].(string); ok {
				summary.WriteString(fmt.Sprintf("Title: %s\n", title))
			}
			// Prefer total_steps field if present, else count array
			if totalSteps, ok := plan["total_steps"].(float64); ok {
				summary.WriteString(fmt.Sprintf("Total steps: %d\n", int(totalSteps)))
			} else if steps, ok := plan["steps"].([]interface{}); ok {
				summary.WriteString(fmt.Sprintf("Total steps: %d\n", len(steps)))
			}
		}
	}

	summary.WriteString("\nUse the context tool to recall the full plan:\n")
	summary.WriteString(`{"tool": "context", "args": {"action": "recall", "key": "implementation_plan"}}`)

	return summary.String()
}

// sanitizeAnalyzeOutput removes code snippets but keeps findings
func sanitizeAnalyzeOutput(output string) string {
	lines := strings.Split(output, "\n")
	var sanitized []string
	inCodeBlock := false

	for _, line := range lines {
		// Toggle code block state
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue // Skip fence markers
		}

		// Skip lines inside code blocks
		if inCodeBlock {
			continue
		}

		// Skip lines that look like file paths
		if isFilePath(line) {
			continue
		}

		// Skip lines with JSON tool calls
		if strings.Contains(line, `{"tool":`) {
			continue
		}

		sanitized = append(sanitized, line)
	}

	return strings.Join(sanitized, "\n")
}

// sanitizeImplementOutput removes tool call examples
func sanitizeImplementOutput(output string) string {
	lines := strings.Split(output, "\n")
	var sanitized []string
	inJSONBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle JSON block state
		if trimmed == "```json" || trimmed == "```" {
			inJSONBlock = !inJSONBlock
			continue
		}

		// Skip JSON blocks
		if inJSONBlock {
			continue
		}

		// Skip standalone JSON objects
		if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"tool"`) {
			continue
		}

		// Remove inline tool calls (e.g., "Step 1: Done {"tool": "file", ...}")
		// Look for patterns like {"tool": and remove until closing }
		if strings.Contains(line, `{"tool"`) {
			// Simple approach: remove from {"tool" to end of line
			// More robust: find matching closing brace
			idx := strings.Index(line, `{"tool"`)
			if idx >= 0 {
				// Find the closing brace
				depth := 0
				start := idx
				foundOpen := false
				end := len(line)

				for i := start; i < len(line); i++ {
					if line[i] == '{' {
						depth++
						foundOpen = true
					} else if line[i] == '}' {
						depth--
						if depth == 0 && foundOpen {
							end = i + 1
							break
						}
					}
				}

				// Remove the tool call JSON from the line
				line = line[:start] + line[end:]
				line = strings.TrimSpace(line)
			}
		}

		sanitized = append(sanitized, line)
	}

	return strings.Join(sanitized, "\n")
}

// sanitizeGenericOutput applies general sanitization rules
func sanitizeGenericOutput(output string) string {
	lines := strings.Split(output, "\n")
	var sanitized []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code block state
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Skip code blocks
		if inCodeBlock {
			continue
		}

		// Skip file paths
		if isFilePath(line) {
			continue
		}

		// Skip JSON lines
		if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, ":") {
			continue
		}

		sanitized = append(sanitized, line)
	}

	return strings.Join(sanitized, "\n")
}

// isFilePath checks if a line looks like a file path
func isFilePath(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Common file extensions
	extensions := []string{".go", ".js", ".py", ".ts", ".java", ".cpp", ".h", ".md", ".json", ".yaml", ".yml"}
	for _, ext := range extensions {
		if strings.Contains(trimmed, ext) {
			return true
		}
	}

	// Path patterns (e.g., "pkg/metrics/metrics.go")
	if strings.Contains(trimmed, "/") && len(strings.Split(trimmed, "/")) > 1 {
		// Check if it's likely a path (has no spaces, reasonable length)
		if !strings.Contains(trimmed, " ") && len(trimmed) < 200 {
			return true
		}
	}

	return false
}

// buildNextPhaseInput builds the input for the next phase based on previous result
func (pe *PhasedExecutor) buildNextPhaseInput(result *PhaseResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Previous Phase: %s\n\n", result.PhaseName))
	sb.WriteString("## Output\n")

	// Sanitize output to prevent context pollution
	sanitized := sanitizePhaseOutput(result.Output, result.PhaseName)
	sb.WriteString(sanitized)

	// Note: We no longer include raw Structured Data to prevent pollution
	// Agents should use context tool to recall structured data if needed

	return sb.String()
}

// updateJobPhase updates the job's current phase in the database
func (pe *PhasedExecutor) updateJobPhase(ctx context.Context, phaseName string) {
	if pe.agent.jobManager == nil {
		return
	}

	// Log phase transition to conversation history
	entry := storage.ConversationEntry{
		Role:      "system",
		Content:   fmt.Sprintf("Starting phase: %s", phaseName),
		Timestamp: time.Now(),
	}
	if err := pe.agent.jobManager.AppendConversation(ctx, pe.jobID, entry); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠️ Failed to append conversation entry: %v\n", err)
	}
}

// savePhaseResults saves all phase results to the job
func (pe *PhasedExecutor) savePhaseResults(ctx context.Context) {
	if pe.agent.jobManager == nil {
		return
	}

	// Convert phase results to JSON for storage
	resultsJSON, err := json.Marshal(pe.phaseResults)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠️ Failed to marshal phase results: %v\n", err)
		return
	}

	// Log phase results summary
	entry := storage.ConversationEntry{
		Role:      "system",
		Content:   fmt.Sprintf("Phase results updated: %s", string(resultsJSON)),
		Timestamp: time.Now(),
	}
	if err := pe.agent.jobManager.AppendConversation(ctx, pe.jobID, entry); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠️ Failed to append conversation entry: %v\n", err)
	}
}

// SaveCheckpoint saves the current execution state for resume capability
// Saves to disk (CLI) and/or database (webapp) depending on configuration
func (pe *PhasedExecutor) SaveCheckpoint(lastInput string) error {
	// Check if checkpointing is enabled
	if !pe.agent.config.Limits.EnablePhaseCheckpoints {
		return nil // Silently skip if disabled
	}

	// Count generated phases (current total - initial count)
	generatedCount := len(pe.phases) - pe.initialPhaseCount

	checkpoint := Checkpoint{
		Version:         1,
		JobID:           pe.jobID,
		CreatedAt:       time.Now(),
		CurrentPhase:    pe.currentPhase,
		CompletedPhases: pe.getCompletedPhaseNames(),
		PhaseResults:    pe.phaseResults,
		LastInput:       lastInput,
		TotalPhases:     len(pe.phases),
		GeneratedPhases: generatedCount,
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	// Save to disk (for CLI)
	checkpointPath := filepath.Join(pe.contextMgr.GetJobDir(), "checkpoint.json")
	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠️ Failed to write checkpoint to disk: %v\n", err)
	}

	// Save to database (for webapp)
	if pe.agent.jobManager != nil {
		entry := storage.ConversationEntry{
			Role:      "system",
			Content:   fmt.Sprintf("Checkpoint saved: %s", string(data)),
			Timestamp: time.Now(),
		}
		if err := pe.agent.jobManager.AppendConversation(context.Background(), pe.jobID, entry); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️ Failed to save checkpoint to database: %v\n", err)
		}
	}

	if pe.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "   [DEBUG] Checkpoint saved: %d/%d phases completed\n",
			pe.currentPhase, len(pe.phases))
	}

	return nil
}

// LoadCheckpoint loads a checkpoint from a job directory
func LoadCheckpoint(jobDir string) (*Checkpoint, error) {
	checkpointPath := filepath.Join(jobDir, "checkpoint.json")

	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no checkpoint found: %w", err)
		}
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// CanResume checks if a checkpoint exists in the job directory
func CanResume(jobDir string) bool {
	checkpointPath := filepath.Join(jobDir, "checkpoint.json")
	_, err := os.Stat(checkpointPath)
	return err == nil
}

// ResumeFromCheckpoint resumes execution from a saved checkpoint
func (pe *PhasedExecutor) ResumeFromCheckpoint(ctx context.Context, checkpoint *Checkpoint) error {
	// Validate checkpoint
	if checkpoint.JobID != pe.jobID {
		return fmt.Errorf("checkpoint job ID mismatch: expected %s, got %s", pe.jobID, checkpoint.JobID)
	}

	if checkpoint.CurrentPhase >= len(pe.phases) {
		return fmt.Errorf("checkpoint phase index out of bounds: %d >= %d", checkpoint.CurrentPhase, len(pe.phases))
	}

	// Restore state
	pe.currentPhase = checkpoint.CurrentPhase
	pe.phaseResults = checkpoint.PhaseResults

	fmt.Fprintf(os.Stderr, "\n🔄 Resuming from checkpoint: %d/%d phases completed\n",
		checkpoint.CurrentPhase, len(pe.phases))
	fmt.Fprintf(os.Stderr, "   Completed phases: %v\n", checkpoint.CompletedPhases)

	// Continue execution from the current phase
	return pe.Execute(ctx, checkpoint.LastInput)
}

// getCompletedPhaseNames returns a list of completed phase names
func (pe *PhasedExecutor) getCompletedPhaseNames() []string {
	completed := make([]string, 0, pe.currentPhase)
	for i := 0; i < pe.currentPhase && i < len(pe.phases); i++ {
		completed = append(completed, pe.phases[i].Name)
	}
	return completed
}

// insertPhases inserts new phases at a specific position in the phase list
func insertPhases(phases []Phase, position int, newPhases []Phase) []Phase {
	if position < 0 || position > len(phases) {
		// Invalid position, append to end
		return append(phases, newPhases...)
	}

	// Create new slice with capacity for all phases
	result := make([]Phase, 0, len(phases)+len(newPhases))

	// Copy phases before insertion point
	result = append(result, phases[:position]...)

	// Insert new phases
	result = append(result, newPhases...)

	// Copy remaining phases
	result = append(result, phases[position:]...)

	return result
}

// phaseInferenceExecutor handles inference for a single phase
type phaseInferenceExecutor struct {
	agent        *BaseAgent
	contextMgr   *llmcontext.Manager
	phase        Phase
	maxRounds    int
	currentRound int
	jobID        string
	result       *PhaseResult            // Track tool calls in this phase
	callHistory  *middleware.CallHistory // Track tool calls and failures via middleware
}

// execute runs the inference loop for a phase
func (pie *phaseInferenceExecutor) execute(ctx context.Context, input string) (string, int, error) {
	currentPrompt := input

	for pie.currentRound < pie.maxRounds {
		pie.currentRound++

		// TODO(#83): Replace with Claude Code-style tree progress view
		// Current: Simple "🔄 Round 1/10" output
		// Desired: Tree structure with tool counts, tokens, collapsible sections
		// See: https://github.com/Soypete/PedroCLI/issues/83
		fmt.Fprintf(os.Stderr, "   🔄 Round %d/%d\n", pie.currentRound, pie.maxRounds)

		// Log user prompt
		pie.logConversation(ctx, "user", currentPrompt, "", nil, nil)

		// Save prompt to context files
		fullPrompt := fmt.Sprintf("System: %s\n\nUser: %s", pie.phase.SystemPrompt, currentPrompt)
		if err := pie.contextMgr.SavePrompt(fullPrompt); err != nil {
			return "", pie.currentRound, fmt.Errorf("failed to save prompt: %w", err)
		}

		// Execute inference with phase-specific system prompt
		systemPrompt := pie.phase.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = pie.agent.buildSystemPrompt()
		}

		response, err := pie.executeInference(ctx, systemPrompt, currentPrompt)
		if err != nil {
			return "", pie.currentRound, fmt.Errorf("inference failed: %w", err)
		}

		// Log assistant response
		pie.logConversation(ctx, "assistant", response.Text, "", nil, nil)

		// Save response to context files
		if err := pie.contextMgr.SaveResponse(response.Text); err != nil {
			return "", pie.currentRound, fmt.Errorf("failed to save response: %w", err)
		}

		// Get tool calls
		toolCalls := response.ToolCalls
		if toolCalls == nil {
			toolCalls = []llm.ToolCall{}
		}

		// FALLBACK: If native tool calling didn't return any calls, try parsing from text
		if len(toolCalls) == 0 && response.Text != "" {
			// Get appropriate formatter for model
			formatter := toolformat.GetFormatterForModel(pie.agent.config.Model.ModelName)

			// Parse tool calls from response text
			parsedCalls, err := formatter.ParseToolCalls(response.Text)
			if err == nil && len(parsedCalls) > 0 {
				// Convert toolformat.ToolCall to llm.ToolCall
				toolCalls = make([]llm.ToolCall, len(parsedCalls))
				for i, tc := range parsedCalls {
					toolCalls[i] = llm.ToolCall{
						Name: tc.Name,
						Args: tc.Args,
					}
				}

				if pie.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "  📝 Parsed %d tool call(s) from response text\n", len(toolCalls))
				}
			}
		}

		// Debug: Log tool call status
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "   [DEBUG] LLM returned %d tool calls\n", len(toolCalls))
			if len(toolCalls) == 0 {
				fmt.Fprintf(os.Stderr, "   [DEBUG] Response contains PHASE_COMPLETE: %v\n", pie.isPhaseComplete(response.Text))
			}
		}

		// If no tool calls, check for completion or prompt for action
		if len(toolCalls) == 0 {
			// Check if phase is complete
			if pie.isPhaseComplete(response.Text) {
				// Debug: Show completion
				if pie.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "   [DEBUG] Phase completing after %d rounds\n", pie.currentRound)
				}
				return response.Text, pie.currentRound, nil
			}

			// No tool calls and not complete - prompt for action
			currentPrompt = "Please continue with the current phase. Use tools if needed, or indicate completion with PHASE_COMPLETE or TASK_COMPLETE."
			continue
		}

		// VALIDATE tool calls before execution
		validCalls, validationErrors := pie.validateToolCalls(toolCalls)

		if len(validationErrors) > 0 {
			// Build error feedback prompt
			currentPrompt = "Tool call errors:\n\n"
			for _, err := range validationErrors {
				currentPrompt += "❌ " + err + "\n"
			}
			currentPrompt += "\nPlease retry with correct tool names and parameters."
			continue // Loop again for LLM to fix
		}

		// Filter tools if phase has tool restrictions
		if len(pie.phase.Tools) > 0 {
			validCalls = pie.filterToolCalls(validCalls)
		}

		// Use validated calls for the rest of the flow
		toolCalls = validCalls

		// Save tool calls to context files
		contextCalls := make([]llmcontext.ToolCall, len(toolCalls))
		for i, tc := range toolCalls {
			contextCalls[i] = llmcontext.ToolCall{
				Name: tc.Name,
				Args: tc.Args,
			}
		}
		if err := pie.contextMgr.SaveToolCalls(contextCalls); err != nil {
			return "", pie.currentRound, fmt.Errorf("failed to save tool calls: %w", err)
		}

		// Execute tools
		results, err := pie.executeTools(ctx, toolCalls)
		if err != nil {
			return "", pie.currentRound, fmt.Errorf("tool execution failed: %w", err)
		}

		// Save tool results to context files
		contextResults := make([]llmcontext.ToolResult, len(results))
		for i, r := range results {
			contextResults[i] = llmcontext.ToolResult{
				Name:          toolCalls[i].Name,
				Success:       r.Success,
				Output:        r.Output,
				Error:         r.Error,
				ModifiedFiles: r.ModifiedFiles,
			}
		}
		if err := pie.contextMgr.SaveToolResults(contextResults); err != nil {
			return "", pie.currentRound, fmt.Errorf("failed to save tool results: %w", err)
		}

		// Check for completion signal in response text (AFTER tools executed)
		// This handles cases where agent outputs tool calls + PHASE_COMPLETE in same response
		if pie.isPhaseComplete(response.Text) {
			return response.Text, pie.currentRound, nil
		}

		// Build feedback prompt
		currentPrompt = pie.buildFeedbackPrompt(toolCalls, results)

		// Check for completion signal in tool results
		if pie.hasCompletionSignal(results) {
			return response.Text, pie.currentRound, nil
		}
	}

	return "", pie.currentRound, fmt.Errorf("max rounds (%d) reached without phase completion", pie.maxRounds)
}

// executeInference performs a single inference call
func (pie *phaseInferenceExecutor) executeInference(ctx context.Context, systemPrompt, userPrompt string) (*llm.InferenceResponse, error) {
	// Check if we need to compact history BEFORE inference
	if pie.contextMgr.ShouldCompact() {
		if err := pie.performCompaction(); err != nil {
			return nil, fmt.Errorf("compaction failed: %w", err)
		}
	}

	budget := llm.CalculateBudget(pie.agent.config, systemPrompt, userPrompt, "")

	// Get tool definitions using the agent's conversion method
	// Note: convertToolsToDefinitions() handles both registry and tools map fallback
	var toolDefs []llm.ToolDefinition
	if pie.agent.config.Model.EnableTools {
		// Get all tool definitions from registry/tools map
		allToolDefs := pie.agent.convertToolsToDefinitions()

		// Filter to phase-allowed tools BEFORE sending to LLM
		toolDefs = pie.filterToolDefinitions(allToolDefs)

		// Debug: Show filtering results
		if pie.agent.config.Debug.Enabled {
			if len(pie.phase.Tools) == 1 && pie.phase.Tools[0] == AllTools {
				// AllTools sentinel = unrestricted
				fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s: all %d tools available (unrestricted)\n",
					pie.phase.Name, len(toolDefs))
			} else if len(pie.phase.Tools) == 0 {
				// Empty array = no tools
				fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s: 0 tools allowed (no tool use)\n",
					pie.phase.Name)
			} else {
				// Specific subset
				fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s tools: %d/%d allowed (%v)\n",
					pie.phase.Name, len(toolDefs), len(allToolDefs), pie.phase.Tools)
			}
		}
	}

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  pie.agent.config.Model.Temperature,
		MaxTokens:    budget.Available,
		Tools:        toolDefs,
	}

	// Apply anti-hallucination logit bias in Validate phase when processing tool results
	// This prevents the agent from fabricating tool outputs
	// IMPORTANT: Skip when native tool calling is enabled (conflicts with grammar)
	if pie.phase.Name == "validate" && strings.HasPrefix(userPrompt, "Tool results:") && !pie.agent.config.Model.EnableTools {
		req.LogitBias = GetToolResultValidationBias(pie.agent.tokenIDProvider)
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintln(os.Stderr, "  🎯 Applied anti-hallucination logit bias")
		}
	}

	// Apply multi-action tool bias when tools are available
	// This encourages correct "action" parameter usage in multi-action tools
	// IMPORTANT: Skip logit bias when native tool calling is enabled (EnableTools=true)
	// because llama.cpp grammar constraints conflict with logit bias manipulation
	if pie.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "   [DEBUG] Checking logit bias: toolDefs=%d, tokenIDProvider=%v, EnableTools=%v\n",
			len(toolDefs), pie.agent.tokenIDProvider != nil, pie.agent.config.Model.EnableTools)
	}

	// Only apply logit bias when NOT using native tool calling
	// Native tool calling already enforces schemas via llama.cpp grammar system
	if len(toolDefs) > 0 && pie.agent.tokenIDProvider != nil && !pie.agent.config.Model.EnableTools {
		// Check if we have multi-action tools (search, file, navigate, rss_feed, etc.)
		hasMultiActionTools := false
		for _, toolDef := range toolDefs {
			// Multi-action tools typically have "action" in their parameter names
			// Parameters is a JSON Schema stored as map[string]interface{}
			if toolDef.Parameters != nil {
				// Debug: Show parameter structure
				if pie.agent.config.Debug.Enabled {
					if props, ok := toolDef.Parameters["properties"]; ok {
						fmt.Fprintf(os.Stderr, "   [DEBUG] Tool %s properties type: %T\n", toolDef.Name, props)
						// Show what keys are in properties
						if propsMap, ok := props.(map[string]interface{}); ok {
							keys := make([]string, 0, len(propsMap))
							for k := range propsMap {
								keys = append(keys, k)
							}
							fmt.Fprintf(os.Stderr, "   [DEBUG] Tool %s properties keys: %v\n", toolDef.Name, keys)
						}
					}
				}

				// Try different type assertions for properties
				if props, ok := toolDef.Parameters["properties"].(map[string]interface{}); ok {
					if _, hasAction := props["action"]; hasAction {
						hasMultiActionTools = true
						if pie.agent.config.Debug.Enabled {
							fmt.Fprintf(os.Stderr, "   [DEBUG] Found multi-action tool: %s (via map[string]interface{})\n", toolDef.Name)
						}
						break
					}
				} else if propsMap, ok := toolDef.Parameters["properties"].(map[string]*logits.JSONSchema); ok {
					// Properties might be map[string]*logits.JSONSchema from metadata
					if _, hasAction := propsMap["action"]; hasAction {
						hasMultiActionTools = true
						if pie.agent.config.Debug.Enabled {
							fmt.Fprintf(os.Stderr, "   [DEBUG] Found multi-action tool: %s (via JSONSchema map)\n", toolDef.Name)
						}
						break
					}
				}
			}
		}

		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "   [DEBUG] hasMultiActionTools=%v\n", hasMultiActionTools)
		}

		if hasMultiActionTools {
			// Merge with existing logit bias if present, otherwise create new
			multiActionBias := GetMultiActionToolBias(pie.agent.tokenIDProvider)
			if req.LogitBias == nil {
				req.LogitBias = multiActionBias
			} else {
				// Merge biases (multi-action bias takes precedence)
				for tokenID, bias := range multiActionBias {
					req.LogitBias[tokenID] = bias
				}
			}

			if pie.agent.config.Debug.Enabled {
				fmt.Fprintf(os.Stderr, "  🎯 Applied multi-action tool logit bias (%d tokens)\n", len(multiActionBias))
			}
		}
	} else if len(toolDefs) > 0 && pie.agent.config.Model.EnableTools {
		// Native tool calling enabled - logit bias skipped to avoid grammar conflicts
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "   [DEBUG] Skipping logit bias (native tool calling uses grammar constraints)\n")
		}
	}

	return pie.agent.llm.Infer(ctx, req)
}

// filterToolCalls filters tool calls to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolCalls(calls []llm.ToolCall) []llm.ToolCall {
	// Check for AllTools sentinel (unrestricted)
	if len(pie.phase.Tools) == 1 && pie.phase.Tools[0] == AllTools {
		// AllTools = unrestricted, return all calls
		return calls
	}

	if len(pie.phase.Tools) == 0 {
		// Empty array = no tools allowed, filter out all
		return []llm.ToolCall{}
	}

	allowedSet := make(map[string]bool)
	for _, t := range pie.phase.Tools {
		allowedSet[t] = true
	}

	filtered := make([]llm.ToolCall, 0)
	for _, call := range calls {
		if allowedSet[call.Name] {
			filtered = append(filtered, call)
		} else {
			fmt.Fprintf(os.Stderr, "   ⚠️ Tool %s not allowed in phase %s, skipping\n", call.Name, pie.phase.Name)
			if pie.agent.config.Debug.Enabled {
				fmt.Fprintf(os.Stderr, "      [DEBUG] This should not happen if tool definitions were filtered correctly\n")
			}
		}
	}

	return filtered
}

// filterToolDefinitions filters tool definitions to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolDefinitions(defs []llm.ToolDefinition) []llm.ToolDefinition {
	// Check for nil vs empty slice - nil means unrestricted
	if pie.phase.Tools == nil {
		// nil = unrestricted, return all tools
		return defs
	}

	// Check for AllTools sentinel (unrestricted)
	if len(pie.phase.Tools) == 1 && pie.phase.Tools[0] == AllTools {
		// AllTools = unrestricted, return all tools
		return defs
	}

	if len(pie.phase.Tools) == 0 {
		// Empty array = explicitly no tools allowed
		return []llm.ToolDefinition{}
	}

	// Build allowed set for O(1) lookup
	allowedSet := make(map[string]bool)
	for _, toolName := range pie.phase.Tools {
		allowedSet[toolName] = true
	}

	// Prevent tool call loops by removing:
	// 1. Tools that have been successfully called
	// 2. Tools that have failed 3+ times (likely invalid arguments)
	// This forces the LLM to either use different tools or output PHASE_COMPLETE
	removedTools := make(map[string]string)

	// Remove successfully called tools
	for toolName := range pie.callHistory.GetCalledTools() {
		delete(allowedSet, toolName)
		removedTools[toolName] = "already called successfully"
	}

	// Remove tools that have failed repeatedly (3+ failures = give up)
	for toolName, failCount := range pie.callHistory.GetFailedTools() {
		if failCount >= 3 {
			delete(allowedSet, toolName)
			removedTools[toolName] = fmt.Sprintf("failed %d times", failCount)
		}
	}

	if len(removedTools) > 0 && pie.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "   [DEBUG] Removed tools from available set: %v\n", removedTools)
	}

	// Filter definitions
	filtered := make([]llm.ToolDefinition, 0, len(pie.phase.Tools))
	foundTools := make(map[string]bool)

	for _, def := range defs {
		if allowedSet[def.Name] {
			filtered = append(filtered, def)
			foundTools[def.Name] = true
		}
	}

	// Debug logging
	if pie.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "   [DEBUG] Filtered tool definitions: %d → %d (phase: %s)\n",
			len(defs), len(filtered), pie.phase.Name)
	}

	// Check for tools that genuinely don't exist (not just filtered out)
	allToolNames := pie.getAllRegisteredToolNames()
	for _, toolName := range pie.phase.Tools {
		if toolName == AllTools {
			continue
		}

		// Check if tool exists in full registered set
		if !allToolNames[toolName] {
			// Tool genuinely doesn't exist - this is an error
			fmt.Fprintf(os.Stderr, "   ⚠️  Tool %q specified in phase but not registered\n", toolName)
		} else if !foundTools[toolName] && pie.agent.config.Debug.Enabled {
			// Tool exists but was filtered - this is expected behavior
			reason := "filtered"
			calledTools := pie.callHistory.GetCalledTools()
			failedTools := pie.callHistory.GetFailedTools()
			if calledTools[toolName] {
				reason = "already called successfully"
			} else if failedTools[toolName] >= 3 {
				reason = fmt.Sprintf("failed %d times", failedTools[toolName])
			}
			fmt.Fprintf(os.Stderr, "   [DEBUG] Tool %q filtered out: %s\n", toolName, reason)
		}
	}

	return filtered
}

// performCompaction compacts the context history when approaching token limit
func (pie *phaseInferenceExecutor) performCompaction() error {
	// Compact history, keeping last 3 rounds
	_, err := pie.contextMgr.CompactHistory(3)
	if err != nil {
		return err
	}

	// Log compaction event
	if pie.agent.config.Debug.Enabled {
		stats, _ := pie.contextMgr.GetCompactionStats()
		if stats != nil {
			fmt.Fprintf(os.Stderr, "   📦 Compacted history: %d rounds → %d recent (%d/%d tokens, %.1f%%)\n",
				stats.TotalRounds,
				stats.RecentRounds,
				stats.LastPromptTokens,
				stats.ContextLimit,
				float64(stats.LastPromptTokens)/float64(stats.ContextLimit)*100)
		}
	}

	// Record compaction stats to database if available
	if pie.agent.compactionStatsStore != nil {
		stats, _ := pie.contextMgr.GetCompactionStats()
		if stats != nil {
			compactionRecord := &storage.CompactionStats{
				JobID:            pie.jobID,
				InferenceRound:   pie.currentRound,
				ModelName:        pie.agent.config.Model.ModelName,
				ContextLimit:     stats.ContextLimit,
				TokensBefore:     stats.LastPromptTokens,
				TokensAfter:      stats.LastPromptTokens, // Approximation - will improve after compaction
				RoundsCompacted:  stats.CompactedRounds,
				RoundsKept:       stats.RecentRounds,
				CompactionTimeMs: 0, // Could measure this if needed
				ThresholdHit:     stats.IsOverThreshold,
			}
			_ = pie.agent.compactionStatsStore.RecordCompaction(context.Background(), compactionRecord)
		}
	}

	return nil
}

// executeTools executes tool calls and logs results
func (pie *phaseInferenceExecutor) executeTools(ctx context.Context, calls []llm.ToolCall) ([]*tools.Result, error) {
	results := make([]*tools.Result, len(calls))

	// BEFORE executing tools: Save tool calls to context manager
	if pie.contextMgr != nil {
		contextCalls := make([]llmcontext.ToolCall, len(calls))
		for i, tc := range calls {
			contextCalls[i] = llmcontext.ToolCall{
				Name: tc.Name,
				Args: tc.Args,
			}
		}
		if err := pie.contextMgr.SaveToolCalls(contextCalls); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "   ⚠️  Failed to save tool calls: %v\n", err)
		}
	}

	for i, call := range calls {
		fmt.Fprintf(os.Stderr, "   🔧 %s", call.Name)

		// Debug: Show arguments for write/edit operations
		if pie.agent.config.Debug.Enabled {
			if call.Name == "code_edit" || call.Name == "file_write" {
				if file, ok := call.Args["file"].(string); ok {
					fmt.Fprintf(os.Stderr, " → %s", file)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "\n")

		// Log tool call
		pie.logConversation(ctx, "tool_call", "", call.Name, call.Args, nil)

		result, err := pie.agent.executeTool(ctx, call.Name, call.Args)
		if err != nil {
			result = &tools.Result{
				Success: false,
				Error:   fmt.Sprintf("tool execution error: %v", err),
			}
		}

		results[i] = result

		// Debug: Log tool result details
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "      [DEBUG] Success: %v, Modified files: %v\n", result.Success, result.ModifiedFiles)
		}

		// Log tool result
		success := result.Success
		pie.logConversationWithSuccess(ctx, call.Name, result, &success)

		if result.Success {
			fmt.Fprintf(os.Stderr, "   ✅ %s\n", call.Name)
			pie.callHistory.RecordToolCall(call.Name, true)
		} else {
			fmt.Fprintf(os.Stderr, "   ❌ %s: %s\n", call.Name, result.Error)
			pie.callHistory.RecordToolCall(call.Name, false)
			if pie.agent.config.Debug.Enabled {
				failedTools := pie.callHistory.GetFailedTools()
				fmt.Fprintf(os.Stderr, "      [DEBUG] Tool %s has failed %d time(s)\n", call.Name, failedTools[call.Name])
			}
		}
	}

	// AFTER executing tools: Save tool results to context manager
	if pie.contextMgr != nil {
		contextResults := make([]llmcontext.ToolResult, len(results))
		for i, r := range results {
			contextResults[i] = llmcontext.ToolResult{
				Name:          calls[i].Name,
				Success:       r.Success,
				Output:        r.Output,
				Error:         r.Error,
				ModifiedFiles: r.ModifiedFiles,
			}
		}
		if err := pie.contextMgr.SaveToolResults(contextResults); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Failed to save tool results: %v\n", err)
		}
	}

	return results, nil
}

// buildFeedbackPrompt builds feedback for the next round
func (pie *phaseInferenceExecutor) buildFeedbackPrompt(calls []llm.ToolCall, results []*tools.Result) string {
	var sb strings.Builder

	sb.WriteString("Tool results:\n\n")

	allSucceeded := true
	for i, call := range calls {
		result := results[i]
		if result.Success {
			sb.WriteString(fmt.Sprintf("✅ %s: %s\n", call.Name, truncateOutput(result.Output, 1000)))
		} else {
			sb.WriteString(fmt.Sprintf("❌ %s failed: %s\n", call.Name, result.Error))
			allSucceeded = false
		}
	}

	sb.WriteString("\n")

	// For phases like analyze_style that fetch data once, prevent tool call loops
	if allSucceeded && pie.phase.Name == "analyze_style" {
		sb.WriteString("The RSS feed has been fetched successfully above. DO NOT call rss_feed again.\n")
		sb.WriteString("Analyze the blog posts in the feed data and create a style guide.\n")
		sb.WriteString("When analysis is complete, output your style guide and signal PHASE_COMPLETE.\n")
	} else {
		sb.WriteString("Continue with the phase. When complete, indicate with PHASE_COMPLETE.")
	}

	return sb.String()
}

// isPhaseComplete checks if response indicates phase completion
func (pie *phaseInferenceExecutor) isPhaseComplete(text string) bool {
	text = strings.ToLower(text)
	completionSignals := []string{
		"phase_complete",
		"phase complete",
		"task_complete",
		"task complete",
	}

	for _, signal := range completionSignals {
		if strings.Contains(text, signal) {
			return true
		}
	}

	return false
}

// hasCompletionSignal checks tool results for completion indicators
func (pie *phaseInferenceExecutor) hasCompletionSignal(results []*tools.Result) bool {
	for _, result := range results {
		if result.Success {
			lower := strings.ToLower(result.Output)
			if strings.Contains(lower, "pr created") || strings.Contains(lower, "pull request created") {
				return true
			}
		}
	}
	return false
}

// logConversation logs a conversation entry
func (pie *phaseInferenceExecutor) logConversation(ctx context.Context, role, content, tool string, args map[string]interface{}, result interface{}) {
	// Log to job manager if available
	if pie.agent.jobManager != nil {
		entry := storage.ConversationEntry{
			Role:      role,
			Content:   content,
			Tool:      tool,
			Args:      args,
			Result:    result,
			Timestamp: time.Now(),
		}

		if err := pie.agent.jobManager.AppendConversation(ctx, pie.jobID, entry); err != nil {
			if pie.agent.config.Debug.Enabled {
				fmt.Fprintf(os.Stderr, "   ⚠️ Failed to log conversation: %v\n", err)
			}
		}
	}

	// Always log to context manager for debugging
	if pie.contextMgr != nil {
		if role == "user" && content != "" {
			_ = pie.contextMgr.SavePrompt(content)
		} else if role == "assistant" && content != "" {
			_ = pie.contextMgr.SaveResponse(content)
		}
	}
}

// logConversationWithSuccess logs a tool result with success status
func (pie *phaseInferenceExecutor) logConversationWithSuccess(ctx context.Context, tool string, result *tools.Result, success *bool) {
	if pie.agent.jobManager == nil {
		return
	}

	resultData := map[string]interface{}{
		"output":         result.Output,
		"error":          result.Error,
		"modified_files": result.ModifiedFiles,
	}

	entry := storage.ConversationEntry{
		Role:      "tool_result",
		Tool:      tool,
		Result:    resultData,
		Success:   success,
		Timestamp: time.Now(),
	}

	if err := pie.agent.jobManager.AppendConversation(ctx, pie.jobID, entry); err != nil {
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "   ⚠️ Failed to log tool result: %v\n", err)
		}
	}
}

// validateToolCalls validates tool calls before execution
// Returns valid calls and error messages for invalid ones
func (pie *phaseInferenceExecutor) validateToolCalls(calls []llm.ToolCall) ([]llm.ToolCall, []string) {
	var validated []llm.ToolCall
	var errors []string

	for _, call := range calls {
		// Get the actual tool instance
		var tool tools.Tool
		var toolExists bool

		if pie.agent.registry != nil {
			for _, t := range pie.agent.registry.List() {
				if t.Name() == call.Name {
					tool = t
					toolExists = true
					break
				}
			}
		} else {
			tool, toolExists = pie.agent.tools[call.Name]
		}

		if !toolExists {
			// Check if this might be an action name mistakenly used as tool name
			if isSearchAction(call.Name) {
				errors = append(errors, fmt.Sprintf(
					"Tool '%s' not found. Did you mean: {\"tool\": \"search\", \"args\": {\"action\": \"%s\", ...}}?",
					call.Name, call.Name))
			} else if isFileAction(call.Name) {
				errors = append(errors, fmt.Sprintf(
					"Tool '%s' not found. Did you mean: {\"tool\": \"file\", \"args\": {\"action\": \"%s\", ...}}?",
					call.Name, call.Name))
			} else if isNavigateAction(call.Name) {
				errors = append(errors, fmt.Sprintf(
					"Tool '%s' not found. Did you mean: {\"tool\": \"navigate\", \"args\": {\"action\": \"%s\", ...}}?",
					call.Name, call.Name))
			} else {
				errors = append(errors, fmt.Sprintf("Tool '%s' not found", call.Name))
			}
			continue
		}

		// Comprehensive schema-based validation for required parameters
		requiredParams := pie.getRequiredParameters(tool)
		var missingParams []string

		for _, paramName := range requiredParams {
			if val, ok := call.Args[paramName]; !ok || val == "" {
				missingParams = append(missingParams, paramName)
			}
		}

		if len(missingParams) > 0 {
			// Debug: Show what args were actually provided
			if pie.agent.config.Debug.Enabled {
				argsJSON, _ := json.Marshal(call.Args)
				fmt.Fprintf(os.Stderr, "   [DEBUG] Tool '%s' called with args: %s\n", call.Name, string(argsJSON))
				fmt.Fprintf(os.Stderr, "   [DEBUG] Required params: %v, Missing: %v\n", requiredParams, missingParams)
			}

			// Check for common mistakes (using 'type' instead of 'action')
			if stringSliceContains(missingParams, "action") {
				if _, hasType := call.Args["type"]; hasType {
					errors = append(errors, fmt.Sprintf(
						"Tool '%s' error: parameter is named 'action', not 'type'. Use: {\"tool\": \"%s\", \"args\": {\"action\": \"...\", ...}}",
						call.Name, call.Name))
					continue
				}
			}

			// Provide detailed error with all missing parameters
			errors = append(errors, fmt.Sprintf(
				"Tool '%s' error: missing required parameter(s): %v. Example: {\"tool\": \"%s\", \"args\": {%s}}",
				call.Name,
				missingParams,
				call.Name,
				buildExampleArgs(missingParams)))
			continue
		}

		validated = append(validated, call)
	}

	return validated, errors
}

// getRequiredParameters extracts required parameters from tool's schema
func (pie *phaseInferenceExecutor) getRequiredParameters(tool tools.Tool) []string {
	// Check if tool implements ExtendedTool interface
	extTool, ok := tool.(tools.ExtendedTool)
	if !ok {
		return nil
	}

	metadata := extTool.Metadata()
	if metadata == nil || metadata.Schema == nil {
		return nil
	}

	return metadata.Schema.Required
}

// buildExampleArgs builds an example args JSON snippet for error messages
func buildExampleArgs(paramNames []string) string {
	if len(paramNames) == 0 {
		return ""
	}

	var parts []string
	for _, name := range paramNames {
		parts = append(parts, fmt.Sprintf("\"%s\": \"...\"", name))
	}
	return strings.Join(parts, ", ")
}

// stringSliceContains checks if a string slice contains an item
func stringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getAllRegisteredToolNames returns all registered tool names
func (pie *phaseInferenceExecutor) getAllRegisteredToolNames() map[string]bool {
	names := make(map[string]bool)
	if pie.agent.registry != nil {
		for _, t := range pie.agent.registry.List() {
			names[t.Name()] = true
		}
	} else {
		for name := range pie.agent.tools {
			names[name] = true
		}
	}
	return names
}

// isNavigateAction checks if a name is a navigate action
func isNavigateAction(name string) bool {
	actions := []string{"list_directory", "get_file_outline", "get_tree", "analyze_imports"}
	for _, action := range actions {
		if name == action {
			return true
		}
	}
	return false
}

// Helper functions

// extractJSONData extracts JSON data from text
func extractJSONData(text string) (map[string]interface{}, error) {
	// Find JSON in text
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found")
	}

	jsonStr := text[start : end+1]

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, err
	}

	return data, nil
}

// truncateOutput truncates output to maxLen characters to prevent context window explosion
// Adds helpful message about accessing full output from context files
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}

	truncated := output[:maxLen]

	// Try to truncate at a newline to avoid mid-sentence cuts
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxLen/2 {
		truncated = truncated[:lastNewline]
	}

	// Count approximate tokens truncated
	truncatedChars := len(output) - len(truncated)
	truncatedTokens := truncatedChars / 4

	return fmt.Sprintf("%s\n\n[Output truncated: ~%d more tokens available. Full result saved to context files.]",
		truncated, truncatedTokens)
}
