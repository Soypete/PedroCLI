package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// HermesFormatter handles Hermes/Nous model tool format
// Hermes uses XML-style function calling format
type HermesFormatter struct{}

// NewHermesFormatter creates a new Hermes formatter
func NewHermesFormatter() *HermesFormatter {
	return &HermesFormatter{}
}

// Name returns the formatter name
func (f *HermesFormatter) Name() string {
	return "hermes"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
func (f *HermesFormatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("You are a function calling AI model. You are provided with function signatures within <tools></tools> XML tags.\n")
	sb.WriteString("You may call one or more functions to assist with the user query. Don't make assumptions about what values to plug into functions.\n\n")

	sb.WriteString("<tools>\n")

	for _, tool := range tools {
		sb.WriteString(f.formatToolXML(tool))
	}

	sb.WriteString("</tools>\n\n")

	sb.WriteString("For each function call return a json object with function name and arguments within <tool_call></tool_call> XML tags as follows:\n")
	sb.WriteString("<tool_call>\n")
	sb.WriteString("{\"name\": \"<function-name>\", \"arguments\": <args-dict>}\n")
	sb.WriteString("</tool_call>\n")

	return sb.String()
}

// formatToolXML formats a single tool definition as XML
func (f *HermesFormatter) formatToolXML(tool ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("<tool>\n")
	sb.WriteString(fmt.Sprintf("  <name>%s</name>\n", tool.Name))
	sb.WriteString(fmt.Sprintf("  <description>%s</description>\n", tool.Description))

	if len(tool.Parameters.Properties) > 0 {
		sb.WriteString("  <parameters>\n")
		for name, prop := range tool.Parameters.Properties {
			required := "false"
			for _, req := range tool.Parameters.Required {
				if req == name {
					required = "true"
					break
				}
			}
			sb.WriteString(fmt.Sprintf("    <parameter name=\"%s\" type=\"%s\" required=\"%s\">\n", name, prop.Type, required))
			sb.WriteString(fmt.Sprintf("      <description>%s</description>\n", prop.Description))
			if len(prop.Enum) > 0 {
				sb.WriteString(fmt.Sprintf("      <enum>%s</enum>\n", strings.Join(prop.Enum, ", ")))
			}
			sb.WriteString("    </parameter>\n")
		}
		sb.WriteString("  </parameters>\n")
	}

	sb.WriteString("</tool>\n")

	return sb.String()
}

// FormatToolsAPI returns tool definitions in OpenAI-compatible format
func (f *HermesFormatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	var result []map[string]interface{}

	for _, tool := range tools {
		properties := make(map[string]interface{})
		for name, prop := range tool.Parameters.Properties {
			propDef := map[string]interface{}{
				"type":        prop.Type,
				"description": prop.Description,
			}
			if len(prop.Enum) > 0 {
				propDef["enum"] = prop.Enum
			}
			properties[name] = propDef
		}

		def := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": properties,
					"required":   tool.Parameters.Required,
				},
			},
		}
		result = append(result, def)
	}

	return result
}

// ParseToolCalls extracts tool calls from a Hermes response
// Looks for <tool_call>...</tool_call> XML tags
func (f *HermesFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Extract from <tool_call> tags
	re := regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*</tool_call>`)
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		content := strings.TrimSpace(match[1])

		// Parse the JSON content
		var rawCall struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}

		if err := json.Unmarshal([]byte(content), &rawCall); err != nil {
			continue
		}

		if rawCall.Name != "" {
			calls = append(calls, ToolCall{
				Name: rawCall.Name,
				Args: rawCall.Arguments,
			})
		}
	}

	// Fall back to generic parsing if no Hermes-format calls found
	if len(calls) == 0 {
		generic := NewGenericFormatter()
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for feeding back to the LLM
func (f *HermesFormatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString("<tool_response>\n")
	sb.WriteString(fmt.Sprintf("  <name>%s</name>\n", call.Name))

	if result.Success {
		sb.WriteString("  <status>success</status>\n")
		output := result.Output
		if len(output) > 2000 {
			output = output[:2000] + "\n... (truncated)"
		}
		sb.WriteString(fmt.Sprintf("  <output>%s</output>\n", output))
	} else {
		sb.WriteString("  <status>error</status>\n")
		sb.WriteString(fmt.Sprintf("  <error>%s</error>\n", result.Error))
	}

	sb.WriteString("</tool_response>\n")

	return sb.String()
}
