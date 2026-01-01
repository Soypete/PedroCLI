package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/soypete/pedrocli/pkg/tools"
)

// MistralFormatter handles tool formatting for Mistral/Mixtral models.
// Mistral uses [AVAILABLE_TOOLS] and [TOOL_CALLS] format.
type MistralFormatter struct{}

// Name returns the formatter name
func (f *MistralFormatter) Name() string {
	return "mistral"
}

// FormatToolsForPrompt generates tool descriptions for Mistral models
func (f *MistralFormatter) FormatToolsForPrompt(registry *tools.ToolRegistry) string {
	toolList := registry.List()
	if len(toolList) == 0 {
		return "No tools available."
	}

	// Sort by name
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].Name() < toolList[j].Name()
	})

	// Build Mistral function calling format
	var sb strings.Builder
	sb.WriteString("[AVAILABLE_TOOLS]\n")

	toolDefs := make([]map[string]interface{}, 0, len(toolList))
	for _, tool := range toolList {
		meta := tool.Metadata()
		var schema interface{}
		if meta != nil && meta.Schema != nil {
			schema = meta.Schema
		}

		toolDefs = append(toolDefs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  schema,
			},
		})
	}

	toolJSON, _ := json.MarshalIndent(toolDefs, "", "  ")
	sb.Write(toolJSON)
	sb.WriteString("\n[/AVAILABLE_TOOLS]\n")

	sb.WriteString(`
To call a tool, use the following format:
[TOOL_CALLS]
[{"name": "tool_name", "arguments": {"param": "value"}}]

When all tasks are complete, respond with "TASK_COMPLETE".
`)
	return sb.String()
}

// ParseToolCalls extracts tool calls from Mistral's [TOOL_CALLS] format
func (f *MistralFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Look for [TOOL_CALLS] blocks
	re := regexp.MustCompile(`(?s)\[TOOL_CALLS\]\s*(\[.*?\])`)
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		var toolCalls []struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(match[1]), &toolCalls); err == nil {
			for _, tc := range toolCalls {
				if tc.Name != "" {
					calls = append(calls, ToolCall{Name: tc.Name, Args: tc.Arguments})
				}
			}
		}
	}

	// Fallback to generic JSON parsing if no TOOL_CALLS block found
	if len(calls) == 0 {
		generic := &GenericFormatter{}
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for Mistral
func (f *MistralFormatter) FormatToolResult(call ToolCall, result *tools.Result) string {
	var sb strings.Builder

	sb.WriteString("[TOOL_RESULTS]\n")
	sb.WriteString(fmt.Sprintf("Tool: %s\n", call.Name))

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

	sb.WriteString("[/TOOL_RESULTS]\n")
	return sb.String()
}

// SupportsNativeToolUse returns false - Mistral uses prompt-based tools
func (f *MistralFormatter) SupportsNativeToolUse() bool {
	return false
}
