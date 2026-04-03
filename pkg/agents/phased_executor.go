package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/soypete/pedro-agentware/middleware"
	"github.com/soypete/pedrocli/pkg/artifacts"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/prompts"
	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// Phase represents a single phase in a phased workflow
type Phase struct {
	Name         string   // Phase identifier (e.g., "analyze", "plan", "implement")
	Description  string   // Human-readable description
	SystemPrompt string   // Custom system prompt for this phase
	Tools        []string // Subset of tools available in this phase (empty = all)
	MaxRounds    int      // Max inference rounds for this phase (0 = use default)
	// Validator validates the phase output and returns error if invalid
	Validator func(result *PhaseResult) error
	// Optional: allow the phase to produce structured output
	ExpectsJSON bool
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

// PhaseCallback is called after each phase completes
// Return true to continue, false to stop execution
type PhaseCallback func(phase Phase, result *PhaseResult) (shouldContinue bool, err error)

// SubagentSpawner is an interface for spawning subagents (avoids import cycle)
type SubagentSpawner interface {
	Spawn(ctx context.Context, taskID, agentType, goal string, tools []string) (string, error)
	Wait(ctx context.Context, handleID string) (string, bool, error)
}

// PhasedExecutor handles multi-phase workflow execution
type PhasedExecutor struct {
	agent            *BaseAgent
	contextMgr       *llmcontext.Manager
	phases           []Phase
	phaseResults     map[string]*PhaseResult
	currentPhase     int
	jobID            string
	defaultMaxRounds int
	phaseCallback    PhaseCallback              // Optional callback after each phase
	policyEvaluator  middleware.PolicyEvaluator // Middleware for tool result filtering
	subagentManager  SubagentSpawner
	artifactStore    artifacts.ArtifactStore // M6: Artifact store for passing data between phases

	// M8: Layered prompts
	mode              string // Current execution mode (chat, plan, build, review)
	useLayeredPrompts bool   // Whether to use layered prompt composition
}

// NewPhasedExecutor creates a new phased executor
func NewPhasedExecutor(agent *BaseAgent, contextMgr *llmcontext.Manager, phases []Phase) *PhasedExecutor {
	return &PhasedExecutor{
		agent:            agent,
		contextMgr:       contextMgr,
		phases:           phases,
		phaseResults:     make(map[string]*PhaseResult),
		currentPhase:     0,
		jobID:            contextMgr.GetJobID(),
		defaultMaxRounds: agent.config.Limits.MaxInferenceRuns,
		phaseCallback:    nil,
	}
}

// SetPhaseCallback sets a callback to be called after each phase completes
func (pe *PhasedExecutor) SetPhaseCallback(callback PhaseCallback) {
	pe.phaseCallback = callback
}

// SetMode sets the execution mode for layered prompts (M8)
func (pe *PhasedExecutor) SetMode(mode string) {
	pe.mode = mode
	pe.useLayeredPrompts = mode != ""
}

// EnableLayeredPrompts enables layered prompt composition
func (pe *PhasedExecutor) EnableLayeredPrompts(enabled bool) {
	pe.useLayeredPrompts = enabled
}

// GetMode returns the current execution mode
func (pe *PhasedExecutor) GetMode() string {
	return pe.mode
}

// SystemPromptProvider is implemented by types that can provide system prompt info
type SystemPromptProvider interface {
	GetPhaseName() string
	GetTools() []string
}

// buildLayeredPrompt builds a layered prompt using PromptBuilder (M8)
func (pe *PhasedExecutor) buildLayeredPrompt(phaseName string, tools []string) string {
	pb := prompts.NewPromptBuilder()

	// Layer 1: Identity
	pb.SetIdentity(prompts.DefaultIdentityPrompt)

	// Layer 2: Mode constraints
	if pe.mode != "" {
		if constraints, ok := prompts.DefaultModeConstraints[pe.mode]; ok {
			pb.SetMode(pe.mode, constraints.String())
		}
	}

	// Layer 3: Phase
	if phaseName != "" {
		pb.SetPhase(
			phaseName,
			prompts.GetPhaseGoal(phaseName),
			strings.Join(tools, ", "),
		)
	}

	// Layer 6: Output schema (based on phase)
	pb.SetOutputSchema(prompts.GetDefaultOutputSchema(phaseName))

	return pb.Build()
}

// SetPolicyEvaluator sets the middleware policy evaluator for tool result filtering
func (pe *PhasedExecutor) SetPolicyEvaluator(eval middleware.PolicyEvaluator) {
	pe.policyEvaluator = eval
}

// SetSubagentManager sets the subagent manager for spawning child agents
func (pe *PhasedExecutor) SetSubagentManager(mgr SubagentSpawner) {
	pe.subagentManager = mgr
}

// HasSubagentManager returns true if a subagent manager is configured
func (pe *PhasedExecutor) HasSubagentManager() bool {
	return pe.subagentManager != nil
}

// SpawnSubagent spawns a subagent to handle parallel tasks within a phase
func (pe *PhasedExecutor) SpawnSubagent(ctx context.Context, taskID, agentType, goal string, tools []string) (string, error) {
	if pe.subagentManager == nil {
		return "", fmt.Errorf("subagent manager not configured")
	}
	return pe.subagentManager.Spawn(ctx, taskID, agentType, goal, tools)
}

// SetArtifactStore sets the artifact store for passing data between phases (M6)
func (pe *PhasedExecutor) SetArtifactStore(store artifacts.ArtifactStore) {
	pe.artifactStore = store
}

// HasArtifactStore returns true if an artifact store is configured
func (pe *PhasedExecutor) HasArtifactStore() bool {
	return pe.artifactStore != nil
}

// PutArtifact stores an artifact for access by later phases
func (pe *PhasedExecutor) PutArtifact(ctx context.Context, artifact *artifacts.Artifact) error {
	if pe.artifactStore == nil {
		return fmt.Errorf("artifact store not configured")
	}
	return pe.artifactStore.Put(ctx, artifact)
}

// GetArtifact retrieves an artifact by ID
func (pe *PhasedExecutor) GetArtifact(ctx context.Context, id string) (*artifacts.Artifact, error) {
	if pe.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}
	return pe.artifactStore.Get(ctx, id)
}

// GetArtifactByName retrieves an artifact by name
func (pe *PhasedExecutor) GetArtifactByName(ctx context.Context, name string) (*artifacts.Artifact, error) {
	if pe.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}
	return pe.artifactStore.GetByName(ctx, name)
}

// ListArtifacts lists artifacts matching the given filter
func (pe *PhasedExecutor) ListArtifacts(ctx context.Context, filter *artifacts.ArtifactFilter) ([]*artifacts.Artifact, error) {
	if pe.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}
	return pe.artifactStore.List(ctx, filter)
}

// storePhaseArtifact stores the phase result as an artifact for later phases to consume
func (pe *PhasedExecutor) storePhaseArtifact(ctx context.Context, phaseName string, result *PhaseResult) {
	if pe.artifactStore == nil || result == nil {
		return
	}

	// Determine artifact type based on phase name
	artifactType := artifacts.ArtifactType(phaseName)
	artifactName := fmt.Sprintf("%s_output", phaseName)

	// Extract content from phase result
	content := result.Output
	if content == "" && result.Data != nil {
		// Try to serialize Data as content
		if dataBytes, err := json.Marshal(result.Data); err == nil {
			content = string(dataBytes)
		}
	}

	artifact := &artifacts.Artifact{
		ID:        fmt.Sprintf("%s-%s", pe.jobID, phaseName),
		Type:      artifactType,
		Name:      artifactName,
		Content:   content,
		Size:      int64(len(content)),
		CreatedBy: phaseName,
		CreatedAt: time.Now(),
	}

	if err := pe.artifactStore.Put(ctx, artifact); err != nil {
		// Log but don't fail - artifact storage is best-effort
		fmt.Fprintf(os.Stderr, "   [WARN] Failed to store artifact for phase %s: %v\n", phaseName, err)
	}
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

		// Store phase output as artifact for later phases (M6)
		pe.storePhaseArtifact(ctx, phase.Name, result)

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

		// Use phase output as input for next phase
		currentInput = pe.buildNextPhaseInput(result)
		pe.currentPhase++
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
		agent:             pe.agent,
		contextMgr:        pe.contextMgr,
		phase:             phase,
		maxRounds:         maxRounds,
		currentRound:      0,
		jobID:             pe.jobID,
		result:            result, // Pass result so executor can track tool calls
		policyEvaluator:   pe.policyEvaluator,
		callHistory:       middleware.NewCallHistory(),
		mode:              pe.mode,
		useLayeredPrompts: pe.useLayeredPrompts,
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

// phaseInferenceExecutor handles inference for a single phase
type phaseInferenceExecutor struct {
	agent           *BaseAgent
	contextMgr      *llmcontext.Manager
	phase           Phase
	maxRounds       int
	currentRound    int
	jobID           string
	result          *PhaseResult // Track tool calls in this phase
	policyEvaluator middleware.PolicyEvaluator
	callHistory     *middleware.CallHistory

	// M8: Layered prompts
	mode              string
	useLayeredPrompts bool
}

func (pie *phaseInferenceExecutor) SetPolicyEvaluator(evaluator middleware.PolicyEvaluator) {
	pie.policyEvaluator = evaluator
}

func (pie *phaseInferenceExecutor) SetCallHistory(callHistory *middleware.CallHistory) {
	pie.callHistory = callHistory
}

// buildLayeredPrompt builds a layered prompt (M8)
func (pie *phaseInferenceExecutor) buildLayeredPrompt() string {
	pb := prompts.NewPromptBuilder()

	// Layer 1: Identity
	pb.SetIdentity(prompts.DefaultIdentityPrompt)

	// Layer 2: Mode constraints
	if pie.mode != "" {
		if constraints, ok := prompts.DefaultModeConstraints[pie.mode]; ok {
			pb.SetMode(pie.mode, constraints.String())
		}
	}

	// Layer 3: Phase
	if pie.phase.Name != "" {
		pb.SetPhase(
			pie.phase.Name,
			prompts.GetPhaseGoal(pie.phase.Name),
			strings.Join(pie.phase.Tools, ", "),
		)
	}

	// Layer 6: Output schema
	pb.SetOutputSchema(prompts.GetDefaultOutputSchema(pie.phase.Name))

	return pb.Build()
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

		// M8: Use layered prompts if enabled and no explicit system prompt
		if systemPrompt == "" && pie.useLayeredPrompts {
			systemPrompt = pie.buildLayeredPrompt()
		}

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

		// Filter tools if phase has tool restrictions
		if len(pie.phase.Tools) > 0 {
			toolCalls = pie.filterToolCalls(toolCalls)
		}

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
			if len(pie.phase.Tools) > 0 {
				fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s tools: %d/%d allowed (%v)\n",
					pie.phase.Name, len(toolDefs), len(allToolDefs), pie.phase.Tools)
			} else {
				fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s: all %d tools available (unrestricted)\n",
					pie.phase.Name, len(toolDefs))
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
	if pie.phase.Name == "validate" && strings.HasPrefix(userPrompt, "Tool results:") {
		req.LogitBias = GetToolResultValidationBias(pie.agent.tokenIDProvider)
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintln(os.Stderr, "  🎯 Applied anti-hallucination logit bias")
		}
	}

	return pie.agent.llm.Infer(ctx, req)
}

// filterToolCalls filters tool calls to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolCalls(calls []llm.ToolCall) []llm.ToolCall {
	if len(pie.phase.Tools) == 0 {
		return calls
	}

	allowedSet := make(map[string]bool)
	for _, t := range pie.phase.Tools {
		allowedSet[t] = true
	}

	filtered := make([]llm.ToolCall, 0)
	for _, call := range calls {
		if !allowedSet[call.Name] {
			fmt.Fprintf(os.Stderr, "   ⚠️ Tool %s not allowed in phase %s, skipping\n", call.Name, pie.phase.Name)
			if pie.agent.config.Debug.Enabled {
				fmt.Fprintf(os.Stderr, "      [DEBUG] This should not happen if tool definitions were filtered correctly\n")
			}
			continue
		}

		// Skip tools already called this phase (prevents loops)
		if pie.callHistory != nil {
			toolKey := pie.phase.Name + ":" + call.Name
			if pie.callHistory.WasToolCalled(toolKey) {
				fmt.Fprintf(os.Stderr, "   ⚠️ Tool %s already called in phase %s, skipping\n", call.Name, pie.phase.Name)
				continue
			}
			// Skip tools that have failed 3+ times in this phase
			if pie.callHistory.HasToolFailedTooManyTimes(toolKey) {
				fmt.Fprintf(os.Stderr, "   ⚠️ Tool %s failed too many times in phase %s, skipping\n", call.Name, pie.phase.Name)
				continue
			}
		}

		filtered = append(filtered, call)
	}

	return filtered
}

// filterToolDefinitions filters tool definitions to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolDefinitions(defs []llm.ToolDefinition) []llm.ToolDefinition {
	// No restrictions if Tools list is empty
	if len(pie.phase.Tools) == 0 {
		return defs
	}

	// Build allowed set for O(1) lookup
	allowedSet := make(map[string]bool)
	for _, toolName := range pie.phase.Tools {
		allowedSet[toolName] = true
	}

	// Filter definitions
	filtered := make([]llm.ToolDefinition, 0, len(pie.phase.Tools))
	foundTools := make(map[string]bool)

	for _, def := range defs {
		if !allowedSet[def.Name] {
			continue
		}

		// Skip tools already called this phase (prevents loops)
		if pie.callHistory != nil {
			toolKey := pie.phase.Name + ":" + def.Name
			if pie.callHistory.WasToolCalled(toolKey) {
				if pie.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "   [DEBUG] Tool %s already called in phase %s, excluding\n", def.Name, pie.phase.Name)
				}
				continue
			}
			// Skip tools that have failed 3+ times in this phase
			if pie.callHistory.HasToolFailedTooManyTimes(toolKey) {
				if pie.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "   [DEBUG] Tool %s failed too many times in phase %s, excluding\n", def.Name, pie.phase.Name)
				}
				continue
			}
		}

		filtered = append(filtered, def)
		foundTools[def.Name] = true
	}

	// Debug logging
	if pie.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "   [DEBUG] Filtered tool definitions: %d → %d (phase: %s)\n",
			len(defs), len(filtered), pie.phase.Name)

		// Warn about tools in phase spec that don't exist
		for _, toolName := range pie.phase.Tools {
			if !foundTools[toolName] {
				fmt.Fprintf(os.Stderr, "   ⚠️  Tool %q specified in phase but not registered\n", toolName)
			}
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
		} else {
			fmt.Fprintf(os.Stderr, "   ❌ %s: %s\n", call.Name, result.Error)
		}

		// Record tool call in middleware's call history (phase-specific tracking)
		if pie.callHistory != nil {
			toolKey := pie.phase.Name + ":" + call.Name
			pie.callHistory.RecordToolCall(toolKey, result.Success)
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

	for i, call := range calls {
		result := results[i]
		if pie.policyEvaluator != nil {
			callerCtx := middleware.CallerContext{Trusted: true}
			mwResult := &middleware.ToolResult{Content: result.Output}
			if result.Error != "" {
				mwResult.Error = fmt.Errorf("%s", result.Error)
			}
			filtered := pie.policyEvaluator.FilterResult(callerCtx, call.Name, mwResult)
			result.Output = fmt.Sprintf("%v", filtered.Content)
			if filtered.Error != nil {
				result.Error = filtered.Error.Error()
			}
		}
		if result.Success {
			sb.WriteString(fmt.Sprintf("✅ %s: %s\n", call.Name, result.Output))
		} else {
			sb.WriteString(fmt.Sprintf("❌ %s failed: %s\n", call.Name, result.Error))
		}
	}

	sb.WriteString("\nContinue with the phase. When complete, indicate with PHASE_COMPLETE.")

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
