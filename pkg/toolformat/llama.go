package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/soypete/pedrocli/pkg/tools"
)

// LlamaFormatter handles tool formatting for Llama 3.x models.
// Llama 3.x uses <|python_tag|> format for tool calls.
type LlamaFormatter struct {
	Version string // e.g., "3.1", "3.2", "3.3"
}

// Name returns the formatter name
func (f *LlamaFormatter) Name() string {
	return "llama"
}

// FormatToolsForPrompt generates tool descriptions for Llama 3.x models
func (f *LlamaFormatter) FormatToolsForPrompt(registry *tools.ToolRegistry) string {
	toolList := registry.List()
	if len(toolList) == 0 {
		return "No tools available."
	}

	// Sort by name
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].Name() < toolList[j].Name()
	})

	var sb strings.Builder
	sb.WriteString("You have access to the following tools:\n\n")

	for _, tool := range toolList {
		sb.WriteString(fmt.Sprintf("### %s\n", tool.Name()))
		sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description()))

		meta := tool.Metadata()
		if meta != nil && meta.Schema != nil {
			schemaJSON, err := json.MarshalIndent(meta.Schema, "", "  ")
			if err == nil {
				sb.WriteString(fmt.Sprintf("Parameters:\n```json\n%s\n```\n\n", schemaJSON))
			}
		}
	}

	sb.WriteString(`
When you need to call a tool, use the following format:
<|python_tag|>{"name": "tool_name", "arguments": {"param": "value"}}

When all tasks are complete, respond with "TASK_COMPLETE".
`)
	return sb.String()
}

// ParseToolCalls extracts tool calls from Llama's <|python_tag|> format
func (f *LlamaFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Look for <|python_tag|> markers followed by JSON
	// The JSON can be complex with nested objects, so we need to match carefully
	// Note: The pipe characters need escaping in regex
	re := regexp.MustCompile(`(?s)<\|python_tag\|>\s*(\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\})`)
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		var call struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(match[1]), &call); err == nil && call.Name != "" {
			calls = append(calls, ToolCall{Name: call.Name, Args: call.Arguments})
		}
	}

	// Fallback to generic JSON parsing if no python_tag found
	if len(calls) == 0 {
		generic := &GenericFormatter{}
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for Llama
func (f *LlamaFormatter) FormatToolResult(call ToolCall, result *tools.Result) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<|tool_result|>\nTool: %s\n", call.Name))

	if result.Success {
		sb.WriteString("Status: Success\n")
		if result.Output != "" {
			output := result.Output
			if len(output) > 1000 {
				output = output[:1000] + "\n... (truncated)"
			}
			sb.WriteString(fmt.Sprintf("Output:\n%s\n", output))
		}
		if len(result.ModifiedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("Modified files: %v\n", result.ModifiedFiles))
		}
	} else {
		sb.WriteString("Status: Failed\n")
		sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}

	sb.WriteString("<|end_tool_result|>\n")
	return sb.String()
}

// SupportsNativeToolUse returns false - Llama uses prompt-based tools
func (f *LlamaFormatter) SupportsNativeToolUse() bool {
	return false
}
