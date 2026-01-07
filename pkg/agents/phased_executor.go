package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/storage"
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

// PhasedExecutor handles multi-phase workflow execution
type PhasedExecutor struct {
	agent            *BaseAgent
	contextMgr       *llmcontext.Manager
	phases           []Phase
	phaseResults     map[string]*PhaseResult
	currentPhase     int
	jobID            string
	defaultMaxRounds int
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
	}
}

// Execute runs all phases sequentially
func (pe *PhasedExecutor) Execute(ctx context.Context, initialInput string) error {
	currentInput := initialInput

	for pe.currentPhase < len(pe.phases) {
		phase := pe.phases[pe.currentPhase]

		fmt.Fprintf(os.Stderr, "\n📋 Phase %d/%d: %s\n", pe.currentPhase+1, len(pe.phases), phase.Name)
		fmt.Fprintf(os.Stderr, "   %s\n", phase.Description)

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
		agent:        pe.agent,
		contextMgr:   pe.contextMgr,
		phase:        phase,
		maxRounds:    maxRounds,
		currentRound: 0,
		jobID:        pe.jobID,
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

// buildNextPhaseInput builds the input for the next phase based on previous result
func (pe *PhasedExecutor) buildNextPhaseInput(result *PhaseResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Previous Phase: %s\n\n", result.PhaseName))
	sb.WriteString("## Output\n")
	sb.WriteString(result.Output)

	if len(result.Data) > 0 {
		sb.WriteString("\n\n## Structured Data\n```json\n")
		data, _ := json.MarshalIndent(result.Data, "", "  ")
		sb.WriteString(string(data))
		sb.WriteString("\n```")
	}

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
		// Log error but don't fail the execution
		if pe.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  ⚠️ Failed to log phase start: %v\n", err)
		}
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
		// Log error but don't fail the execution
		if pe.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  ⚠️ Failed to log phase results: %v\n", err)
		}
	}
}

// phaseInferenceExecutor handles inference for a single phase
type phaseInferenceExecutor struct {
	agent        *BaseAgent
	contextMgr   *llmcontext.Manager
	phase        Phase
	maxRounds    int
	currentRound int
	jobID        string
}

// execute runs the inference loop for a phase
func (pie *phaseInferenceExecutor) execute(ctx context.Context, input string) (string, int, error) {
	currentPrompt := input

	for pie.currentRound < pie.maxRounds {
		pie.currentRound++

		fmt.Fprintf(os.Stderr, "   🔄 Round %d/%d\n", pie.currentRound, pie.maxRounds)

		// Log user prompt
		pie.logConversation(ctx, "user", currentPrompt, "", nil, nil)

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

		// Get tool calls
		toolCalls := response.ToolCalls
		if toolCalls == nil {
			toolCalls = []llm.ToolCall{}
		}

		// Check if phase is complete (no more tool calls and completion signal)
		if len(toolCalls) == 0 {
			if pie.isPhaseComplete(response.Text) {
				return response.Text, pie.currentRound, nil
			}

			// No tool calls but not complete - prompt for action
			currentPrompt = "Please continue with the current phase. Use tools if needed, or indicate completion with PHASE_COMPLETE or TASK_COMPLETE."
			continue
		}

		// Filter tools if phase has tool restrictions
		if len(pie.phase.Tools) > 0 {
			toolCalls = pie.filterToolCalls(toolCalls)
		}

		// Execute tools
		results, err := pie.executeTools(ctx, toolCalls)
		if err != nil {
			return "", pie.currentRound, fmt.Errorf("tool execution failed: %w", err)
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
	budget := llm.CalculateBudget(pie.agent.config, systemPrompt, userPrompt, "")

	// Get tool definitions using the agent's conversion method
	var toolDefs []llm.ToolDefinition
	if pie.agent.config.Model.EnableTools && pie.agent.registry != nil {
		toolDefs = pie.agent.convertToolsToDefinitions()
	}

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  pie.agent.config.Model.Temperature,
		MaxTokens:    budget.Available,
		Tools:        toolDefs,
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
		if allowedSet[call.Name] {
			filtered = append(filtered, call)
		} else {
			fmt.Fprintf(os.Stderr, "   ⚠️ Tool %s not allowed in phase %s, skipping\n", call.Name, pie.phase.Name)
		}
	}

	return filtered
}

// executeTools executes tool calls and logs results
func (pie *phaseInferenceExecutor) executeTools(ctx context.Context, calls []llm.ToolCall) ([]*tools.Result, error) {
	results := make([]*tools.Result, len(calls))

	for i, call := range calls {
		fmt.Fprintf(os.Stderr, "   🔧 %s\n", call.Name)

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

		// Log tool result
		success := result.Success
		pie.logConversationWithSuccess(ctx, call.Name, result, &success)

		if result.Success {
			fmt.Fprintf(os.Stderr, "   ✅ %s\n", call.Name)
		} else {
			fmt.Fprintf(os.Stderr, "   ❌ %s: %s\n", call.Name, result.Error)
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
		if result.Success {
			sb.WriteString(fmt.Sprintf("✅ %s: %s\n", call.Name, truncateOutput(result.Output, 1000)))
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
	if pie.agent.jobManager == nil {
		return
	}

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

// truncateOutput truncates output to maxLen characters
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "..."
}
