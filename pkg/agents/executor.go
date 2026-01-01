package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// InferenceExecutor handles the inference loop
type InferenceExecutor struct {
	agent        *BaseAgent
	contextMgr   *llmcontext.Manager
	maxRounds    int
	currentRound int
	systemPrompt string                   // Custom system prompt (if set)
	formatter    toolformat.ToolFormatter // Model-specific tool formatter
}

// NewInferenceExecutor creates a new inference executor with default GenericFormatter
func NewInferenceExecutor(agent *BaseAgent, contextMgr *llmcontext.Manager) *InferenceExecutor {
	return &InferenceExecutor{
		agent:        agent,
		contextMgr:   contextMgr,
		maxRounds:    agent.config.Limits.MaxInferenceRuns,
		currentRound: 0,
		systemPrompt: "",                             // Will use agent's default if empty
		formatter:    &toolformat.GenericFormatter{}, // Default formatter
	}
}

// NewInferenceExecutorWithModel creates a new inference executor with model-specific formatter
func NewInferenceExecutorWithModel(agent *BaseAgent, contextMgr *llmcontext.Manager, modelName string) *InferenceExecutor {
	return &InferenceExecutor{
		agent:        agent,
		contextMgr:   contextMgr,
		maxRounds:    agent.config.Limits.MaxInferenceRuns,
		currentRound: 0,
		systemPrompt: "",
		formatter:    toolformat.GetFormatter(modelName),
	}
}

// SetFormatter sets a custom tool formatter
func (e *InferenceExecutor) SetFormatter(formatter toolformat.ToolFormatter) {
	e.formatter = formatter
}

// SetSystemPrompt sets a custom system prompt for this executor
func (e *InferenceExecutor) SetSystemPrompt(prompt string) {
	e.systemPrompt = prompt
}

// getSystemPrompt returns the system prompt to use
func (e *InferenceExecutor) getSystemPrompt() string {
	if e.systemPrompt != "" {
		return e.systemPrompt
	}
	return e.agent.buildSystemPrompt()
}

// Execute runs the inference loop until completion or max rounds
func (e *InferenceExecutor) Execute(ctx context.Context, initialPrompt string) error {
	currentPrompt := initialPrompt
	jobID := e.contextMgr.GetJobID()

	for e.currentRound < e.maxRounds {
		e.currentRound++

		fmt.Fprintf(os.Stderr, "🔄 Inference round %d/%d\n", e.currentRound, e.maxRounds)

		// Log user prompt to conversation history
		e.logConversation(ctx, jobID, "user", currentPrompt, "", nil, nil)

		// Execute one inference round (with custom system prompt if set)
		response, err := e.agent.executeInferenceWithSystemPrompt(ctx, e.contextMgr, currentPrompt, e.systemPrompt)
		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

		// Log assistant response to conversation history
		e.logConversation(ctx, jobID, "assistant", response.Text, "", nil, nil)

		// Parse tool calls from response
		toolCalls := e.parseToolCalls(response.Text)

		// Check if we're done (no more tool calls)
		if len(toolCalls) == 0 {
			if e.isDone(response.Text) {
				fmt.Fprintln(os.Stderr, "✅ Task completed!")
				return nil
			}

			// No tool calls but not explicitly done - provide feedback
			currentPrompt = "You haven't called any tools yet. Please use the available tools to complete the task. Remember to use JSON format for tool calls:\n\n{\"tool\": \"tool_name\", \"args\": {\"key\": \"value\"}}"
			continue
		}

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
			return nil
		}
	}

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

// parseToolCalls parses tool calls from the LLM response using the model-specific formatter
func (e *InferenceExecutor) parseToolCalls(text string) []llm.ToolCall {
	if e.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "  🔍 Parsing tool calls from %d bytes of text using %s formatter\n", len(text), e.formatter.Name())
	}

	// Use formatter to parse tool calls
	formatterCalls, err := e.formatter.ParseToolCalls(text)
	if err != nil {
		if e.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  ⚠️ Formatter parse error: %v, falling back to legacy parsing\n", err)
		}
		return e.parseToolCallsLegacy(text)
	}

	// Convert toolformat.ToolCall to llm.ToolCall
	calls := make([]llm.ToolCall, len(formatterCalls))
	for i, fc := range formatterCalls {
		calls[i] = llm.ToolCall{
			Name: fc.Name,
			Args: fc.Args,
		}
	}

	if e.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "  📋 Parsed %d tool call(s)\n", len(calls))
		for i, call := range calls {
			fmt.Fprintf(os.Stderr, "    %d. %s\n", i+1, call.Name)
		}
	}

	return calls
}

// parseToolCallsLegacy is the legacy parsing logic for fallback
// Supports both "tool" and "name" field names for backwards compatibility
func (e *InferenceExecutor) parseToolCallsLegacy(text string) []llm.ToolCall {
	var calls []llm.ToolCall

	// Strategy 1: Try parsing entire response as JSON array
	var arrayOfCalls []llm.ToolCall
	if err := json.Unmarshal([]byte(text), &arrayOfCalls); err == nil && len(arrayOfCalls) > 0 {
		calls = e.filterValidCalls(arrayOfCalls)
		if e.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  📋 Legacy: Parsed %d tool call(s) from JSON array\n", len(calls))
		}
		return calls
	}

	// Strategy 2: Try parsing as single JSON object
	var singleCall llm.ToolCall
	if err := json.Unmarshal([]byte(text), &singleCall); err == nil && singleCall.Name != "" {
		if e.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  📋 Legacy: Parsed 1 tool call from JSON object: %s\n", singleCall.Name)
		}
		return []llm.ToolCall{singleCall}
	}

	// Strategy 3: Extract from markdown code blocks
	calls = e.extractFromCodeBlocks(text)
	if len(calls) > 0 {
		if e.agent.config.Debug.Enabled {
			fmt.Fprintf(os.Stderr, "  📋 Legacy: Parsed %d tool call(s) from code blocks\n", len(calls))
		}
		return calls
	}

	// Strategy 4: Line-by-line JSON parsing
	calls = e.extractFromLines(text)
	if e.agent.config.Debug.Enabled {
		fmt.Fprintf(os.Stderr, "  📋 Legacy: Parsed %d tool call(s) from lines\n", len(calls))
		for i, call := range calls {
			fmt.Fprintf(os.Stderr, "    %d. %s\n", i+1, call.Name)
		}
	}

	return calls
}

// extractFromCodeBlocks extracts tool calls from markdown code blocks
func (e *InferenceExecutor) extractFromCodeBlocks(text string) []llm.ToolCall {
	var calls []llm.ToolCall

	// Match ```json...``` or ```...``` blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		// Try as array first
		var arrayCalls []llm.ToolCall
		if err := json.Unmarshal([]byte(match[1]), &arrayCalls); err == nil {
			calls = append(calls, e.filterValidCalls(arrayCalls)...)
			continue
		}

		// Try as single object
		var call llm.ToolCall
		if err := json.Unmarshal([]byte(match[1]), &call); err == nil && call.Name != "" {
			calls = append(calls, call)
		}
	}

	return calls
}

// extractFromLines extracts tool calls from individual lines
func (e *InferenceExecutor) extractFromLines(text string) []llm.ToolCall {
	var calls []llm.ToolCall
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Must start with { to be JSON
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var call llm.ToolCall
		if err := json.Unmarshal([]byte(line), &call); err == nil && call.Name != "" {
			calls = append(calls, call)
		}
	}

	return calls
}

// filterValidCalls filters out invalid tool calls (empty names)
func (e *InferenceExecutor) filterValidCalls(calls []llm.ToolCall) []llm.ToolCall {
	var valid []llm.ToolCall
	for _, call := range calls {
		if call.Name != "" {
			valid = append(valid, call)
		}
	}
	return valid
}

// buildFeedbackPrompt builds a prompt with tool results for the next round
func (e *InferenceExecutor) buildFeedbackPrompt(calls []llm.ToolCall, results []*tools.Result) string {
	var prompt strings.Builder

	prompt.WriteString("Tool execution results:\n\n")

	for i, call := range calls {
		result := results[i]

		// Convert llm.ToolCall to toolformat.ToolCall for formatter
		formatterCall := toolformat.ToolCall{
			Name: call.Name,
			Args: call.Args,
		}

		// Use formatter to format the result
		prompt.WriteString(e.formatter.FormatToolResult(formatterCall, result))
		prompt.WriteString("\n")
	}

	prompt.WriteString("Based on these results, what should we do next? If the task is complete, respond with 'TASK_COMPLETE'. Otherwise, continue with the next steps using tool calls.")

	return prompt.String()
}

// isDone checks if the response indicates task completion
func (e *InferenceExecutor) isDone(text string) bool {
	text = strings.ToLower(text)
	doneSignals := []string{
		"task_complete",
		"task complete",
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
