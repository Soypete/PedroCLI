// Package toolformat provides model-specific tool call formatting.
// Different LLM models expect and produce tool calls in different formats.
// This package abstracts the formatting and parsing of tool calls.
package toolformat

import (
	"github.com/soypete/pedrocli/pkg/tools"
)

// ToolCall represents a parsed tool call from an LLM response
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// ToolFormatter handles model-specific tool call formatting and parsing.
// Each model family (Qwen, Llama, Mistral, etc.) has different expectations
// for how tools are presented and how tool calls are formatted.
type ToolFormatter interface {
	// Name returns the name of this formatter (e.g., "qwen", "llama", "generic")
	Name() string

	// FormatToolsForPrompt generates the tool description section for the system prompt.
	// This is used when embedding tool information directly in the prompt.
	FormatToolsForPrompt(registry *tools.ToolRegistry) string

	// ParseToolCalls extracts tool calls from an LLM response.
	// Returns empty slice if no tool calls are found.
	ParseToolCalls(response string) ([]ToolCall, error)

	// FormatToolResult formats a tool result for the next prompt.
	// This creates the feedback that tells the LLM what happened.
	FormatToolResult(call ToolCall, result *tools.Result) string

	// SupportsNativeToolUse returns true if the model has native tool API support
	// (like Claude API's tool_use blocks). For most local models this is false.
	SupportsNativeToolUse() bool
}
