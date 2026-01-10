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

// Note: Phase and PhaseResult are now defined in orchestrator.go

// PhaseCallback is called after each phase completes
// Return true to continue, false to stop execution
type PhaseCallback func(phase Phase, result *PhaseResult) (shouldContinue bool, err error)

// PhasedExecutor handles multi-phase workflow execution
type PhasedExecutor struct {
	agent            *BaseAgent
	contextMgr       *llmcontext.Manager
	phases           []Phase
	phaseResults     map[string]*PhaseResult
	currentPhase     int
	jobID            string
	defaultMaxRounds int
	phaseCallback    PhaseCallback // Optional callback after each phase
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
		PhaseName:     phase.Name,
		StartedAt:     time.Now(),
		Data:          make(map[string]interface{}),
		ToolCalls:     []ToolCallSummary{},
		ModifiedFiles: []string{},
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
		result:       result, // Pass result so executor can track tool calls
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
	agent        *BaseAgent
	contextMgr   *llmcontext.Manager
	phase        Phase
	maxRounds    int
	currentRound int
	jobID        string
	result       *PhaseResult // Track tool calls in this phase
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

		// Debug: Log tool call status
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "   [DEBUG] LLM returned %d tool calls\n", len(toolCalls))
			if len(toolCalls) == 0 {
				fmt.Fprintf(os.Stderr, "   [DEBUG] Response contains PHASE_COMPLETE: %v\n", pie.isPhaseComplete(response.Text))
			}
		}

		// Check if phase is complete (no more tool calls and completion signal)
		if len(toolCalls) == 0 {
			if pie.isPhaseComplete(response.Text) {
				// Debug: Show what was accomplished
				if pie.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "   [DEBUG] Phase completing with %d tool calls made, %d files modified\n",
						len(pie.result.ToolCalls), len(pie.result.ModifiedFiles))
				}
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
	// Note: convertToolsToDefinitions() handles both registry and tools map fallback
	var toolDefs []llm.ToolDefinition
	if pie.agent.config.Model.EnableTools {
		toolDefs = pie.agent.convertToolsToDefinitions()

		// Debug: Show how many tools are being sent
		if pie.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "   [DEBUG] Converting tools for inference: %d tool definitions\n", len(toolDefs))
		}
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

		// Track tool call in phase result
		if pie.result != nil {
			summary := ToolCallSummary{
				ToolName:      call.Name,
				Success:       result.Success,
				Output:        result.Output,
				Error:         result.Error,
				ModifiedFiles: result.ModifiedFiles,
			}
			pie.result.ToolCalls = append(pie.result.ToolCalls, summary)

			// Track modified files at phase level
			for _, file := range result.ModifiedFiles {
				// Check if already in list
				found := false
				for _, existing := range pie.result.ModifiedFiles {
					if existing == file {
						found = true
						break
					}
				}
				if !found {
					pie.result.ModifiedFiles = append(pie.result.ModifiedFiles, file)
				}
			}
		}

		// Log tool result
		success := result.Success
		pie.logConversationWithSuccess(ctx, call.Name, result, &success)

		if result.Success {
			fmt.Fprintf(os.Stderr, "   ✅ %s\n", call.Name)
		} else {
			fmt.Fprintf(os.Stderr, "   ❌ %s: %s\n", call.Name, result.Error)
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

// truncateOutput truncates output to maxLen characters
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "..."
}
