package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/soypete/pedrocli/pkg/tools"
)

// QwenFormatter handles tool formatting for Qwen 2.5 models.
// Qwen uses XML-style <tool_call> tags for tool invocations.
type QwenFormatter struct {
	Version string // e.g., "2.5", "2.5-coder"
}

// Name returns the formatter name
func (f *QwenFormatter) Name() string {
	return "qwen"
}

// FormatToolsForPrompt generates tool descriptions for Qwen models
func (f *QwenFormatter) FormatToolsForPrompt(registry *tools.ToolRegistry) string {
	toolList := registry.List()
	if len(toolList) == 0 {
		return "No tools available."
	}

	// Sort by name
	sort.Slice(toolList, func(i, j int) bool {
		return toolList[i].Name() < toolList[j].Name()
	})

	var sb strings.Builder
	sb.WriteString("# Tools\n\n")
	sb.WriteString("You have access to the following tools:\n\n")

	for _, tool := range toolList {
		sb.WriteString(fmt.Sprintf("## %s\n", tool.Name()))
		sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description()))

		// Qwen works well with inline parameter descriptions
		meta := tool.Metadata()
		if meta != nil && meta.Schema != nil && meta.Schema.Properties != nil {
			sb.WriteString("**Parameters:**\n")

			// Create required set
			requiredSet := make(map[string]bool)
			for _, r := range meta.Schema.Required {
				requiredSet[r] = true
			}

			// Sort property names
			propNames := make([]string, 0, len(meta.Schema.Properties))
			for name := range meta.Schema.Properties {
				propNames = append(propNames, name)
			}
			sort.Strings(propNames)

			for _, name := range propNames {
				prop := meta.Schema.Properties[name]
				req := ""
				if requiredSet[name] {
					req = " (required)"
				}
				desc := prop.Description
				if desc == "" {
					desc = "No description"
				}
				sb.WriteString(fmt.Sprintf("- `%s`%s: %s\n", name, req, desc))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(`
To call a tool, use the following XML format:
<tool_call>
{"name": "tool_name", "arguments": {"param": "value"}}
</tool_call>

When all tasks are complete, respond with "TASK_COMPLETE".
`)
	return sb.String()
}

// ParseToolCalls extracts tool calls from Qwen's <tool_call> format
func (f *QwenFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Look for <tool_call>...</tool_call> blocks
	re := regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)
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

	// Fallback to generic JSON parsing if no tool_call tags found
	if len(calls) == 0 {
		generic := &GenericFormatter{}
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for Qwen
func (f *QwenFormatter) FormatToolResult(call ToolCall, result *tools.Result) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<tool_result name=\"%s\">\n", call.Name))

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

	sb.WriteString("</tool_result>\n")
	return sb.String()
}

// SupportsNativeToolUse returns false - Qwen uses prompt-based tools
func (f *QwenFormatter) SupportsNativeToolUse() bool {
	return false
}
