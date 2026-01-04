package toolformat

import (
	"context"
	"fmt"
	"strings"
)

// ExecutorConfig holds configuration for the tool executor
type ExecutorConfig struct {
	Formatter   ToolFormatter
	Registry    *Registry
	MaxRounds   int
	Debug       bool
	OnToolCall  func(call ToolCall)
	OnToolResult func(call ToolCall, result *ToolResult)
}

// ToolExecutor handles tool execution with model-specific formatting
type ToolExecutor struct {
	config ExecutorConfig
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(config ExecutorConfig) *ToolExecutor {
	if config.MaxRounds == 0 {
		config.MaxRounds = 20
	}
	if config.Registry == nil {
		config.Registry = DefaultRegistry
	}
	if config.Formatter == nil {
		config.Formatter = NewGenericFormatter()
	}

	return &ToolExecutor{
		config: config,
	}
}

// BuildSystemPrompt generates the system prompt with tool definitions
func (e *ToolExecutor) BuildSystemPrompt(basePrompt string, mode ToolMode) string {
	var sb strings.Builder

	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")

	// Get tools for the specified mode
	tools := e.config.Registry.GetToolsForMode(mode)

	// Format tools using the model-specific formatter
	toolsPrompt := e.config.Formatter.FormatToolsPrompt(tools)
	sb.WriteString(toolsPrompt)

	return sb.String()
}

// BuildSystemPromptWithTools generates the system prompt with specific tools
func (e *ToolExecutor) BuildSystemPromptWithTools(basePrompt string, toolNames ...string) string {
	var sb strings.Builder

	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")

	// Get specific tools
	tools := e.config.Registry.GetDefinitions(toolNames...)

	// Format tools using the model-specific formatter
	toolsPrompt := e.config.Formatter.FormatToolsPrompt(tools)
	sb.WriteString(toolsPrompt)

	return sb.String()
}

// ParseResponse parses an LLM response and extracts tool calls
func (e *ToolExecutor) ParseResponse(response string) ([]ToolCall, error) {
	return e.config.Formatter.ParseToolCalls(response)
}

// ExecuteToolCall executes a single tool call
func (e *ToolExecutor) ExecuteToolCall(ctx context.Context, call ToolCall) *ToolResult {
	if e.config.OnToolCall != nil {
		e.config.OnToolCall(call)
	}

	result, err := e.config.Registry.Execute(ctx, call.Name, call.Args)
	if err != nil {
		result = &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool execution error: %v", err),
		}
	}

	if e.config.OnToolResult != nil {
		e.config.OnToolResult(call, result)
	}

	return result
}

// ExecuteToolCalls executes multiple tool calls and returns results
func (e *ToolExecutor) ExecuteToolCalls(ctx context.Context, calls []ToolCall) []*ToolResult {
	results := make([]*ToolResult, len(calls))

	for i, call := range calls {
		results[i] = e.ExecuteToolCall(ctx, call)
	}

	return results
}

// FormatToolResults formats tool results for feeding back to the LLM
func (e *ToolExecutor) FormatToolResults(calls []ToolCall, results []*ToolResult) string {
	var sb strings.Builder

	for i, call := range calls {
		if i < len(results) {
			sb.WriteString(e.config.Formatter.FormatToolResult(call, results[i]))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// BuildFeedbackPrompt builds a complete feedback prompt with tool results
func (e *ToolExecutor) BuildFeedbackPrompt(calls []ToolCall, results []*ToolResult) string {
	var sb strings.Builder

	sb.WriteString("Tool execution results:\n\n")
	sb.WriteString(e.FormatToolResults(calls, results))
	sb.WriteString("\n")
	sb.WriteString("Based on these results, what should we do next? ")
	sb.WriteString("If the task is complete, respond with 'TASK_COMPLETE'. ")
	sb.WriteString("Otherwise, continue with the next steps using tool calls.")

	return sb.String()
}

// IsDone checks if the response indicates task completion
func (e *ToolExecutor) IsDone(text string) bool {
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

// HasCompletionSignal checks if any tool result indicates completion
func (e *ToolExecutor) HasCompletionSignal(results []*ToolResult) bool {
	for _, result := range results {
		if result.Success && strings.Contains(strings.ToLower(result.Output), "pr created") {
			return true
		}
	}
	return false
}

// GetToolsAPI returns tool definitions in API format for native tool use
func (e *ToolExecutor) GetToolsAPI(mode ToolMode) interface{} {
	tools := e.config.Registry.GetToolsForMode(mode)
	return e.config.Formatter.FormatToolsAPI(tools)
}

// GetToolsAPIByNames returns tool definitions in API format for specific tools
func (e *ToolExecutor) GetToolsAPIByNames(toolNames ...string) interface{} {
	tools := e.config.Registry.GetDefinitions(toolNames...)
	return e.config.Formatter.FormatToolsAPI(tools)
}

// ExecutorBuilder provides a fluent interface for building executors
type ExecutorBuilder struct {
	config ExecutorConfig
}

// NewExecutorBuilder creates a new executor builder
func NewExecutorBuilder() *ExecutorBuilder {
	return &ExecutorBuilder{
		config: ExecutorConfig{
			Registry:  NewRegistry(),
			Formatter: NewGenericFormatter(),
			MaxRounds: 20,
		},
	}
}

// WithRegistry sets the tool registry
func (b *ExecutorBuilder) WithRegistry(registry *Registry) *ExecutorBuilder {
	b.config.Registry = registry
	return b
}

// WithFormatter sets the tool formatter
func (b *ExecutorBuilder) WithFormatter(formatter ToolFormatter) *ExecutorBuilder {
	b.config.Formatter = formatter
	return b
}

// WithFormatterForModel sets the formatter based on model name
func (b *ExecutorBuilder) WithFormatterForModel(modelName string) *ExecutorBuilder {
	b.config.Formatter = GetFormatterForModel(modelName)
	return b
}

// WithMaxRounds sets the maximum inference rounds
func (b *ExecutorBuilder) WithMaxRounds(maxRounds int) *ExecutorBuilder {
	b.config.MaxRounds = maxRounds
	return b
}

// WithDebug enables debug mode
func (b *ExecutorBuilder) WithDebug(debug bool) *ExecutorBuilder {
	b.config.Debug = debug
	return b
}

// WithToolCallHandler sets the tool call callback
func (b *ExecutorBuilder) WithToolCallHandler(handler func(call ToolCall)) *ExecutorBuilder {
	b.config.OnToolCall = handler
	return b
}

// WithToolResultHandler sets the tool result callback
func (b *ExecutorBuilder) WithToolResultHandler(handler func(call ToolCall, result *ToolResult)) *ExecutorBuilder {
	b.config.OnToolResult = handler
	return b
}

// Build creates the executor
func (b *ExecutorBuilder) Build() *ToolExecutor {
	return NewToolExecutor(b.config)
}
