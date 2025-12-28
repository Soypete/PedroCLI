package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

// InferenceExecutor handles the inference loop
type InferenceExecutor struct {
	agent        *BaseAgent
	contextMgr   *llmcontext.Manager
	maxRounds    int
	currentRound int
	systemPrompt string // Custom system prompt (if set)
}

// NewInferenceExecutor creates a new inference executor
func NewInferenceExecutor(agent *BaseAgent, contextMgr *llmcontext.Manager) *InferenceExecutor {
	return &InferenceExecutor{
		agent:        agent,
		contextMgr:   contextMgr,
		maxRounds:    agent.config.Limits.MaxInferenceRuns,
		currentRound: 0,
		systemPrompt: "", // Will use agent's default if empty
	}
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

	for e.currentRound < e.maxRounds {
		e.currentRound++

		fmt.Fprintf(os.Stderr, "🔄 Inference round %d/%d\n", e.currentRound, e.maxRounds)

		// Execute one inference round (with custom system prompt if set)
		response, err := e.agent.executeInferenceWithSystemPrompt(ctx, e.contextMgr, currentPrompt, e.systemPrompt)
		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

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

		// Execute tools
		results, err := e.executeTools(ctx, toolCalls)
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

// parseToolCalls parses tool calls from the LLM response
// Expected format: JSON objects or JSON code blocks
func (e *InferenceExecutor) parseToolCalls(text string) []llm.ToolCall {
	var calls []llm.ToolCall

	// Try to find JSON code blocks first (```json ... ```)
	jsonBlockRegex := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")
	matches := jsonBlockRegex.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 1 {
			var call llm.ToolCall
			if err := json.Unmarshal([]byte(match[1]), &call); err == nil {
				if call.Name != "" {
					calls = append(calls, call)
				}
			}
		}
	}

	// If no code blocks, try to find inline JSON objects
	if len(calls) == 0 {
		// Look for {\"tool\": \"...\", \"args\": {...}} pattern
		inlineRegex := regexp.MustCompile(`\{[^}]*"tool"[^}]*"args"[^}]*\}`)
		matches := inlineRegex.FindAllString(text, -1)

		for _, match := range matches {
			var call llm.ToolCall
			if err := json.Unmarshal([]byte(match), &call); err == nil {
				if call.Name != "" {
					calls = append(calls, call)
				}
			}
		}
	}

	return calls
}

// executeTools executes a list of tool calls
func (e *InferenceExecutor) executeTools(ctx context.Context, calls []llm.ToolCall) ([]*tools.Result, error) {
	results := make([]*tools.Result, len(calls))

	for i, call := range calls {
		fmt.Fprintf(os.Stderr, "  🔧 Executing tool: %s\n", call.Name)

		result, err := e.agent.executeTool(ctx, call.Name, call.Args)
		if err != nil {
			result = &tools.Result{
				Success: false,
				Error:   fmt.Sprintf("tool execution error: %v", err),
			}
		}

		results[i] = result

		if result.Success {
			fmt.Fprintf(os.Stderr, "  ✅ Tool %s succeeded\n", call.Name)
		} else {
			fmt.Fprintf(os.Stderr, "  ❌ Tool %s failed: %s\n", call.Name, result.Error)
		}
	}

	return results, nil
}

// buildFeedbackPrompt builds a prompt with tool results for the next round
func (e *InferenceExecutor) buildFeedbackPrompt(calls []llm.ToolCall, results []*tools.Result) string {
	var prompt strings.Builder

	prompt.WriteString("Tool execution results:\n\n")

	for i, call := range calls {
		result := results[i]

		prompt.WriteString(fmt.Sprintf("Tool: %s\n", call.Name))

		if result.Success {
			prompt.WriteString("Status: ✅ Success\n")
			if result.Output != "" {
				// Truncate long output
				output := result.Output
				if len(output) > 1000 {
					output = output[:1000] + "\n... (truncated)"
				}
				prompt.WriteString(fmt.Sprintf("Output:\n%s\n", output))
			}
			if len(result.ModifiedFiles) > 0 {
				prompt.WriteString(fmt.Sprintf("Modified files: %v\n", result.ModifiedFiles))
			}
		} else {
			prompt.WriteString("Status: ❌ Failed\n")
			prompt.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		}

		prompt.WriteString("\n")
	}

	prompt.WriteString("Based on these results, what should we do next? If the task is complete, respond with 'TASK_COMPLETE'. Otherwise, continue with the next steps using tool calls in JSON format.")

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
