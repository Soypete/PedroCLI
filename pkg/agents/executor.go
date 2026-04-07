package agents

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/soypete/pedro-agentware/middleware"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/telemetry"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// ProgressEventType represents the type of progress event
type ProgressEventType string

const (
	ProgressEventRoundStart  ProgressEventType = "round_start"
	ProgressEventRoundEnd    ProgressEventType = "round_end"
	ProgressEventToolCall    ProgressEventType = "tool_call"
	ProgressEventToolResult  ProgressEventType = "tool_result"
	ProgressEventLLMResponse ProgressEventType = "llm_response"
	ProgressEventMessage     ProgressEventType = "message"
	ProgressEventError       ProgressEventType = "error"
	ProgressEventComplete    ProgressEventType = "complete"
)

// ProgressEvent represents a progress event during execution
type ProgressEvent struct {
	Type    ProgressEventType
	Message string
	Data    interface{}
}

// ProgressCallback is called when progress events occur
type ProgressCallback func(event ProgressEvent)

// InferenceExecutor handles the inference loop
type InferenceExecutor struct {
	agent        *BaseAgent
	contextMgr   *llmcontext.Manager
	maxRounds    int
	currentRound int
	systemPrompt string // Custom system prompt (if set)

	// Mode constraints (M2)
	allowedTools []string
	deniedTools  []string
	allowWrites  bool

	// Policy evaluator for tool call validation (middleware)
	policyEvaluator middleware.PolicyEvaluator

	// Progress callback for streaming updates
	progressCallback ProgressCallback

	// M9: Telemetry collector
	telemetryCollector telemetry.TelemetryCollector
	jobID              string
}

// NewInferenceExecutor creates a new inference executor
func NewInferenceExecutor(agent *BaseAgent, contextMgr *llmcontext.Manager) *InferenceExecutor {
	return &InferenceExecutor{
		agent:           agent,
		contextMgr:      contextMgr,
		maxRounds:       agent.config.Limits.MaxInferenceRuns,
		currentRound:    0,
		systemPrompt:    "",  // Will use agent's default if empty
		policyEvaluator: nil, // Will be set via SetPolicyEvaluator if needed
	}
}

// SetPolicyEvaluator sets the middleware policy evaluator for tool call validation
func (e *InferenceExecutor) SetPolicyEvaluator(eval middleware.PolicyEvaluator) {
	e.policyEvaluator = eval
}

// SetSystemPrompt sets a custom system prompt for this executor
func (e *InferenceExecutor) SetSystemPrompt(prompt string) {
	e.systemPrompt = prompt
}

// SetProgressCallback sets a callback for progress events
func (e *InferenceExecutor) SetProgressCallback(callback ProgressCallback) {
	e.progressCallback = callback
}

// SetTelemetryCollector sets the telemetry collector for M9
func (e *InferenceExecutor) SetTelemetryCollector(collector telemetry.TelemetryCollector, jobID string) {
	e.telemetryCollector = collector
	e.jobID = jobID
}

// recordInferenceTelemetry records inference metrics to telemetry collector (M9)
func (e *InferenceExecutor) recordInferenceTelemetry(response *llm.InferenceResponse, latency time.Duration, success bool) {
	if e.telemetryCollector == nil || e.jobID == "" {
		return
	}

	e.telemetryCollector.Record(telemetry.TelemetryEvent{
		JobID:     e.jobID,
		AgentID:   e.agent.name,
		Round:     e.currentRound,
		EventType: telemetry.EventInference,
		Data: map[string]interface{}{
			"prompt_tokens":     response.TokensUsed / 2, // Approximate
			"completion_tokens": response.TokensUsed / 2, // Approximate
			"total_tokens":      response.TokensUsed,
			"llm_latency":       latency.String(),
			"model":             e.agent.config.Model.ModelName,
			"success":           success,
		},
	})
}

// SetModeConstraints sets tool constraints based on execution mode (M2)
func (e *InferenceExecutor) SetModeConstraints(allowedTools, deniedTools []string, allowWrites bool) {
	e.allowedTools = allowedTools
	e.deniedTools = deniedTools
	e.allowWrites = allowWrites
}

// SetMaxRounds sets the maximum number of inference rounds
func (e *InferenceExecutor) SetMaxRounds(maxRounds int) {
	if maxRounds > 0 {
		e.maxRounds = maxRounds
	}
}

// isToolAllowed checks if a tool is allowed to execute based on mode constraints
func (e *InferenceExecutor) isToolAllowed(toolName string) bool {
	// Check denied tools first
	for _, denied := range e.deniedTools {
		if denied == toolName {
			return false
		}
	}

	// If allowedTools is specified, tool must be in that list
	if len(e.allowedTools) > 0 {
		for _, allowed := range e.allowedTools {
			if allowed == toolName {
				return true
			}
		}
		return false
	}

	// No constraints, allow all
	return true
}

// checkWritePermission checks if a write operation is allowed
func (e *InferenceExecutor) checkWritePermission(toolName string) error {
	if !e.allowWrites {
		writeTools := []string{"file", "code_edit", "write", "create", "edit"}
		for _, wt := range writeTools {
			if toolName == wt {
				return fmt.Errorf("write operations not allowed in this mode")
			}
		}
	}
	return nil
}

// emitProgress emits a progress event if callback is set
func (e *InferenceExecutor) emitProgress(eventType ProgressEventType, message string, data interface{}) {
	if e.progressCallback != nil {
		e.progressCallback(ProgressEvent{
			Type:    eventType,
			Message: message,
			Data:    data,
		})
	}
}

// Execute runs the inference loop until completion or max rounds
func (e *InferenceExecutor) Execute(ctx context.Context, initialPrompt string) error {
	currentPrompt := initialPrompt
	jobID := e.contextMgr.GetJobID()

	for e.currentRound < e.maxRounds {
		e.currentRound++

		fmt.Fprintf(os.Stderr, "🔄 Inference round %d/%d\n", e.currentRound, e.maxRounds)

		// Check context budget and warn if nearing limit
		if e.currentRound > 1 {
			stats, err := e.contextMgr.GetCompactionStats()
			if err == nil && stats.IsOverThreshold {
				fmt.Fprintf(os.Stderr, "⚠️  Context near limit: %d/%d tokens (%.0f%%)\n",
					stats.LastPromptTokens, stats.ContextLimit,
					float64(stats.LastPromptTokens)/float64(stats.ContextLimit)*100)
			}
		}

		// Force compaction every N rounds to prevent context explosion (configurable)
		compactEvery := e.agent.config.Context.CompactEveryNRounds
		if compactEvery == 0 {
			compactEvery = 3 // Default to 3 if not configured
		}
		if e.currentRound%compactEvery == 0 && e.currentRound > 0 {
			fmt.Fprintf(os.Stderr, "  🗜️  Compacting context (round %d)...\n", e.currentRound)
			_, err := e.contextMgr.CompactHistory(2) // Keep last 2 rounds
			if err != nil {
				// Log warning but don't fail - compaction is best-effort
				fmt.Fprintf(os.Stderr, "  ⚠️  Compaction failed: %v\n", err)
			}
		}

		// Emit round start event
		e.emitProgress(ProgressEventRoundStart, fmt.Sprintf("Round %d/%d", e.currentRound, e.maxRounds), map[string]interface{}{
			"round":      e.currentRound,
			"max_rounds": e.maxRounds,
		})

		// Log user prompt to conversation history
		e.logConversation(ctx, jobID, "user", currentPrompt, "", nil, nil)

		// Execute one inference round (with custom system prompt if set)
		startTime := time.Now()
		response, err := e.agent.executeInferenceWithSystemPrompt(ctx, e.contextMgr, currentPrompt, e.systemPrompt)
		llmLatency := time.Since(startTime)

		if err != nil {
			e.emitProgress(ProgressEventError, "Inference failed", err)
			// M9: Record failed inference
			e.recordInferenceTelemetry(response, llmLatency, false)
			return fmt.Errorf("inference failed: %w", err)
		}

		// M9: Record successful inference
		e.recordInferenceTelemetry(response, llmLatency, true)

		// Emit LLM response event
		e.emitProgress(ProgressEventLLMResponse, "Received LLM response", map[string]interface{}{
			"text_length": len(response.Text),
			"tool_calls":  len(response.ToolCalls),
		})

		// Log assistant response to conversation history
		e.logConversation(ctx, jobID, "assistant", response.Text, "", nil, nil)

		// Get tool calls from response (native API tool calling)
		toolCalls := response.ToolCalls
		if toolCalls == nil {
			toolCalls = []llm.ToolCall{}
		}

		// FALLBACK: If native tool calling didn't return any calls, try parsing from text
		if len(toolCalls) == 0 && response.Text != "" {
			// Get appropriate formatter for model
			formatter := toolformat.GetFormatterForModel(e.agent.config.Model.ModelName)

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

				if e.agent.config.Debug.Enabled {
					fmt.Fprintf(os.Stderr, "  📝 Parsed %d tool call(s) from response text\n", len(toolCalls))
				}
			}
		}

		// Check if we're done (no more tool calls)
		if len(toolCalls) == 0 {
			if e.isDone(response.Text) {
				fmt.Fprintln(os.Stderr, "✅ Task completed!")
				e.emitProgress(ProgressEventComplete, "Task completed", nil)
				return nil
			}

			// No tool calls but not explicitly done - provide feedback
			currentPrompt = "You haven't called any tools yet. Please use the available tools to complete the task. Remember to use JSON format for tool calls:\n\n{\"tool\": \"tool_name\", \"args\": {\"key\": \"value\"}}"
			continue
		}

		// Validate tool calls before execution
		validCalls, validationErrors := e.validateToolCalls(toolCalls)
		if len(validationErrors) > 0 {
			// Build helpful feedback prompt with examples
			currentPrompt = "Tool call errors:\n\n"
			for _, err := range validationErrors {
				currentPrompt += "❌ " + err + "\n"
			}
			currentPrompt += "\nIMPORTANT: You MUST provide the required parameters in JSON format."
			currentPrompt += "\nExample: {\"tool\": \"web_search\", \"args\": {\"query\": \"search term here\"}}"
			currentPrompt += "\nYou must include actual values, not empty objects {}. Each parameter is required."
			continue
		}
		toolCalls = validCalls

		// Save tool calls
		contextCalls := make([]llmcontext.ToolCall, len(toolCalls))
		for i, tc := range toolCalls {
			contextCalls[i] = llmcontext.ToolCall{
				Name: tc.Name,
				Args: tc.Args,
			}
		}
		if err := e.contextMgr.SaveToolCalls(contextCalls); err != nil {
			return fmt.Errorf("failed to save tool calls: %w", err)
		}

		// Execute tools and log each call/result
		results, err := e.executeToolsWithLogging(ctx, toolCalls, jobID)
		if err != nil {
			return fmt.Errorf("tool execution failed: %w", err)
		}

		// Save tool results
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
		if err := e.contextMgr.SaveToolResults(contextResults); err != nil {
			return fmt.Errorf("failed to save tool results: %w", err)
		}

		// Build next prompt with tool results
		currentPrompt = e.buildFeedbackPrompt(toolCalls, results)

		// Check if any tool indicated completion
		if e.hasCompletionSignal(results) {
			fmt.Fprintln(os.Stderr, "✅ Task completed (indicated by tool result)")
			e.emitProgress(ProgressEventComplete, "Task completed", nil)
			return nil
		}

		// Emit round end event
		e.emitProgress(ProgressEventRoundEnd, fmt.Sprintf("Round %d complete", e.currentRound), nil)
	}

	e.emitProgress(ProgressEventError, "Max rounds reached", nil)
	return fmt.Errorf("max inference rounds (%d) reached without completion", e.maxRounds)
}

// logConversation logs a conversation entry to the database
func (e *InferenceExecutor) logConversation(ctx context.Context, jobID, role, content, tool string, args map[string]interface{}, result interface{}) {
	if e.agent.jobManager == nil {
		return // No job manager available
	}

	entry := storage.ConversationEntry{
		Role:      role,
		Content:   content,
		Tool:      tool,
		Args:      args,
		Result:    result,
		Timestamp: time.Now(),
	}

	if err := e.agent.jobManager.AppendConversation(ctx, jobID, entry); err != nil {
		// Log error but don't fail the execution
		if e.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  ⚠️ Failed to log conversation: %v\n", err)
		}
	}
}

// executeToolsWithLogging executes tools and logs each call/result to conversation history
func (e *InferenceExecutor) executeToolsWithLogging(ctx context.Context, calls []llm.ToolCall, jobID string) ([]*tools.Result, error) {
	results := make([]*tools.Result, len(calls))

	for i, call := range calls {
		fmt.Fprintf(os.Stderr, "  🔧 Executing tool: %s\n", call.Name)

		// Check mode-based tool constraints (M2)
		if !e.isToolAllowed(call.Name) {
			result := &tools.Result{
				Success: false,
				Error:   fmt.Sprintf("tool '%s' is not allowed in this execution mode", call.Name),
			}
			results[i] = result
			e.emitProgress(ProgressEventToolResult, fmt.Sprintf("Tool %s blocked by mode constraints", call.Name), map[string]interface{}{
				"tool":    call.Name,
				"success": false,
				"error":   result.Error,
			})
			continue
		}

		// Check write permission
		if err := e.checkWritePermission(call.Name); err != nil {
			result := &tools.Result{
				Success: false,
				Error:   err.Error(),
			}
			results[i] = result
			e.emitProgress(ProgressEventToolResult, fmt.Sprintf("Tool %s blocked by write permission", call.Name), map[string]interface{}{
				"tool":    call.Name,
				"success": false,
				"error":   result.Error,
			})
			continue
		}

		// Emit tool call event
		e.emitProgress(ProgressEventToolCall, fmt.Sprintf("Calling tool: %s", call.Name), map[string]interface{}{
			"tool": call.Name,
			"args": call.Args,
		})

		// Log tool call
		e.logConversation(ctx, jobID, "tool_call", "", call.Name, call.Args, nil)

		result, err := e.agent.executeTool(ctx, call.Name, call.Args)
		if err != nil {
			result = &tools.Result{
				Success: false,
				Error:   fmt.Sprintf("tool execution error: %v", err),
			}
		}

		results[i] = result

		// Emit tool result event
		e.emitProgress(ProgressEventToolResult, fmt.Sprintf("Tool %s result", call.Name), map[string]interface{}{
			"tool":    call.Name,
			"success": result.Success,
			"output":  result.Output,
			"error":   result.Error,
		})

		// Log tool result
		success := result.Success
		e.logConversationWithSuccess(ctx, jobID, call.Name, result, &success)

		if result.Success {
			fmt.Fprintf(os.Stderr, "  ✅ Tool %s succeeded\n", call.Name)
		} else {
			fmt.Fprintf(os.Stderr, "  ❌ Tool %s failed: %s\n", call.Name, result.Error)
		}
	}

	return results, nil
}

// validateToolCalls checks if tool names exist and have required parameters
// Returns validated calls and any errors
func (e *InferenceExecutor) validateToolCalls(calls []llm.ToolCall) ([]llm.ToolCall, []string) {
	var validated []llm.ToolCall
	var errors []string

	// Create CallerContext for middleware evaluation
	callerCtx := middleware.CallerContext{
		Trusted:   true,
		Role:      "agent",
		UserID:    "",
		SessionID: "",
		Source:    "inference",
		Phase:     "validation",
		Metadata:  nil,
	}

	for _, call := range calls {
		// Check if tool exists
		if _, exists := e.agent.tools[call.Name]; !exists {
			// Check if this might be an action name mistakenly used as tool name
			if isSearchAction(call.Name) || isFileAction(call.Name) {
				errors = append(errors, fmt.Sprintf(
					"Tool '%s' not found. Did you mean: {\"tool\": \"search\", \"args\": {\"action\": \"%s\", ...}}?",
					call.Name, call.Name))
			} else {
				errors = append(errors, fmt.Sprintf("Tool '%s' not found", call.Name))
			}
			continue
		}

		// For multi-action tools, check if 'action' parameter is present
		if call.Name == "search" || call.Name == "file" {
			if action, ok := call.Args["action"]; !ok || action == "" {
				// Check if they used 'type' instead of 'action'
				if _, hasType := call.Args["type"]; hasType {
					errors = append(errors, fmt.Sprintf(
						"Tool '%s' error: parameter is named 'action', not 'type'. Use: {\"tool\": \"%s\", \"args\": {\"action\": \"...\", ...}}",
						call.Name, call.Name))
				} else {
					errors = append(errors, fmt.Sprintf(
						"Tool '%s' error: missing required 'action' parameter. Use: {\"tool\": \"%s\", \"args\": {\"action\": \"...\", ...}}",
						call.Name, call.Name))
				}
				continue
			}
		}

		// If policy evaluator is set, use middleware for additional validation
		if e.policyEvaluator != nil {
			decision := e.policyEvaluator.Evaluate(callerCtx, call.Name, call.Args)
			if decision.IsDenied() {
				reason := decision.Reason
				if reason == "" {
					reason = fmt.Sprintf("policy denied tool '%s'", call.Name)
				}
				errors = append(errors, reason)
				continue
			}

			// For bash tool, apply allowlist/denylist if not handled by middleware
			if call.Name == "bash" {
				if !e.validateBashAllowlistDenylist(call.Args, decision) {
					errors = append(errors, fmt.Sprintf("bash command not allowed by allowlist/denylist policy"))
					continue
				}
			}
		}

		validated = append(validated, call)
	}

	return validated, errors
}

// validateBashAllowlistDenylist validates bash commands against allowlist/denylist
// This preserves the existing bash command allowlist/denylist logic from pkg/tools/bash.go
func (e *InferenceExecutor) validateBashAllowlistDenylist(args map[string]interface{}, decision middleware.Decision) bool {
	// Check if middleware already handled bash validation (includes allowlist/denylist info)
	if reason := decision.Reason; reason != "" {
		// If middleware provided a reason, it has handled the validation
		return decision.IsAllowed()
	}

	// Fall back to checking bash tool's own allowlist/denylist
	// Get the bash tool from agent's tools
	bashTool, exists := e.agent.tools["bash"]
	if !exists {
		// No bash tool available, allow (will fail later if needed)
		return true
	}

	// Access the bash tool's allowed/forbidden commands
	// We need to type assert to get the internal fields
	bash, ok := bashTool.(*tools.BashTool)
	if !ok {
		// Not the expected type, allow through
		return true
	}

	// Get command from args
	cmd, ok := args["command"].(string)
	if !ok || cmd == "" {
		// No command to validate, allow
		return true
	}

	// Use bash tool's built-in isAllowed method
	return bash.IsAllowed(cmd)
}

// isSearchAction checks if a name is a search action
func isSearchAction(name string) bool {
	actions := []string{"grep", "find_files", "find_in_file", "find_definition"}
	for _, action := range actions {
		if name == action {
			return true
		}
	}
	return false
}

// isFileAction checks if a name is a file action
func isFileAction(name string) bool {
	actions := []string{"read", "write", "replace", "append", "delete"}
	for _, action := range actions {
		if name == action {
			return true
		}
	}
	return false
}

// logConversationWithSuccess logs a tool result with success status
func (e *InferenceExecutor) logConversationWithSuccess(ctx context.Context, jobID, tool string, result *tools.Result, success *bool) {
	if e.agent.jobManager == nil {
		return
	}

	// Build result object for storage
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

	if err := e.agent.jobManager.AppendConversation(ctx, jobID, entry); err != nil {
		if e.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  ⚠️ Failed to log tool result: %v\n", err)
		}
	}
}

// truncateOutputWithLimit truncates tool output based on per-tool limits from config
func truncateOutputWithLimit(toolName string, output string, cfg *config.Config) string {
	if output == "" {
		return output
	}

	limits := map[string]int{}
	if cfg != nil && cfg.Context.ToolResultLimits != nil {
		limits = cfg.Context.ToolResultLimits
	}
	if len(limits) == 0 {
		limits = map[string]int{
			"web_search":   500,
			"web_scraper":  800,
			"search":       600,
			"grep":         600,
			"file":         1200,
			"read":         1200,
			"rss":          600,
			"static_links": 400,
			"default":      500,
		}
	}

	maxChars := limits[toolName]
	if maxChars == 0 {
		maxChars = limits["default"]
	}
	if maxChars == 0 {
		maxChars = 500
	}

	return truncateOutput(output, maxChars)
}

// buildFeedbackPrompt builds a prompt with tool results for the next round
func (e *InferenceExecutor) buildFeedbackPrompt(calls []llm.ToolCall, results []*tools.Result) string {
	var prompt strings.Builder

	prompt.WriteString("Tool execution results:\n\n")

	for i, call := range calls {
		result := results[i]

		// Apply middleware result filtering if policy evaluator is set
		if e.policyEvaluator != nil {
			callerCtx := middleware.CallerContext{Trusted: true}
			mwResult := &middleware.ToolResult{Content: result.Output}
			if result.Error != "" {
				mwResult.Error = fmt.Errorf("%s", result.Error)
			}
			filtered := e.policyEvaluator.FilterResult(callerCtx, call.Name, mwResult)
			result.Output = fmt.Sprintf("%v", filtered.Content)
			if filtered.Error != nil {
				result.Error = filtered.Error.Error()
			}
		} else {
			// Fallback truncation to prevent context window explosion when middleware not configured
			result.Output = truncateOutputWithLimit(call.Name, result.Output, e.agent.config)
			if result.Error != "" {
				result.Error = truncateOutput(result.Error, 500)
			}
		}

		// Format tool result
		if result.Success {
			prompt.WriteString(fmt.Sprintf("✅ %s: %s\n", call.Name, result.Output))
		} else {
			prompt.WriteString(fmt.Sprintf("❌ %s failed: %s\n", call.Name, result.Error))
		}
	}

	prompt.WriteString("\nBased on these results, what should we do next? If the task is complete, respond with 'TASK_COMPLETE'. Otherwise, continue with the next steps using JSON tool calls:")
	prompt.WriteString("\nFormat: {\"tool\": \"tool_name\", \"args\": {\"param\": \"value\"}}")
	prompt.WriteString("\nExample: {\"tool\": \"web_search\", \"args\": {\"query\": \"search term\"}}")

	return prompt.String()
}

// isDone checks if the response indicates task completion
func (e *InferenceExecutor) isDone(text string) bool {
	text = strings.ToLower(text)
	doneSignals := []string{
		"task_complete",
		"task complete",
		"research_complete", // For blog research phase
		"work is complete",
		"i'm done",
		"all done",
		"finished",
	}

	for _, signal := range doneSignals {
		if strings.Contains(text, signal) {
			return true
		}
	}

	return false
}

// hasCompletionSignal checks if any tool result indicates completion
func (e *InferenceExecutor) hasCompletionSignal(results []*tools.Result) bool {
	for _, result := range results {
		if result.Success && strings.Contains(strings.ToLower(result.Output), "pr created") {
			return true
		}
	}
	return false
}
