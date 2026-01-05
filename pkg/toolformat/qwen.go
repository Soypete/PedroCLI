package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// QwenFormatter handles Qwen 2.5 model tool format
// Qwen uses <tool_call> XML tags for tool calls
type QwenFormatter struct{}

// NewQwenFormatter creates a new Qwen formatter
func NewQwenFormatter() *QwenFormatter {
	return &QwenFormatter{}
}

// Name returns the formatter name
func (f *QwenFormatter) Name() string {
	return "qwen"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
// Qwen expects tools in a specific format within the system prompt
func (f *QwenFormatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("# Tools\n\n")
	sb.WriteString("You may call one or more functions to assist with the user query.\n\n")
	sb.WriteString("You are provided with function signatures within <tools></tools> XML tags:\n")
	sb.WriteString("<tools>\n")

	for _, tool := range tools {
		toolJSON := f.formatToolDefinitionJSON(tool)
		sb.WriteString(toolJSON)
		sb.WriteString("\n")
	}

	sb.WriteString("</tools>\n\n")

	sb.WriteString("For each function call, return a json object with function name and arguments within <tool_call></tool_call> XML tags:\n")
	sb.WriteString("<tool_call>\n")
	sb.WriteString("{\"name\": \"<function-name>\", \"arguments\": <args-json-object>}\n")
	sb.WriteString("</tool_call>\n")

	return sb.String()
}

// formatToolDefinitionJSON formats a single tool definition as JSON
func (f *QwenFormatter) formatToolDefinitionJSON(tool ToolDefinition) string {
	// Build the function definition in OpenAI-style format
	def := map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  f.formatParameters(tool.Parameters),
		},
	}

	data, _ := json.Marshal(def)
	return string(data)
}

// formatParameters converts ParameterSchema to JSON Schema format
func (f *QwenFormatter) formatParameters(params ParameterSchema) map[string]interface{} {
	properties := make(map[string]interface{})
	for name, prop := range params.Properties {
		propDef := map[string]interface{}{
			"type":        prop.Type,
			"description": prop.Description,
		}
		if len(prop.Enum) > 0 {
			propDef["enum"] = prop.Enum
		}
		if prop.Items != nil {
			propDef["items"] = map[string]interface{}{
				"type": prop.Items.Type,
			}
		}
		properties[name] = propDef
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   params.Required,
	}
}

// FormatToolsAPI returns tool definitions in OpenAI-compatible format for API use
func (f *QwenFormatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	var result []map[string]interface{}

	for _, tool := range tools {
		def := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  f.formatParameters(tool.Parameters),
			},
		}
		result = append(result, def)
	}

	return result
}

// ParseToolCalls extracts tool calls from a Qwen response
// Looks for <tool_call>...</tool_call> XML tags
func (f *QwenFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Strategy 1: Extract from <tool_call> tags
	re := regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*</tool_call>`)
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		content := strings.TrimSpace(match[1])

		// Parse the JSON content
		// Qwen format: {"name": "func_name", "arguments": {...}}
		var rawCall struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}

		if err := json.Unmarshal([]byte(content), &rawCall); err != nil {
			// Try alternative format with "tool" instead of "name"
			var altCall ToolCall
			if err2 := json.Unmarshal([]byte(content), &altCall); err2 == nil && altCall.Name != "" {
				calls = append(calls, altCall)
			}
			continue
		}

		if rawCall.Name != "" {
			calls = append(calls, ToolCall{
				Name: rawCall.Name,
				Args: rawCall.Arguments,
			})
		}
	}

	// If no <tool_call> tags found, fall back to generic parsing
	if len(calls) == 0 {
		generic := NewGenericFormatter()
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for feeding back to the LLM
func (f *QwenFormatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString("<tool_response>\n")
	sb.WriteString(fmt.Sprintf("{\"name\": \"%s\", ", call.Name))

	if result.Success {
		// Escape the output for JSON
		outputJSON, _ := json.Marshal(result.Output)
		sb.WriteString(fmt.Sprintf("\"content\": %s", string(outputJSON)))
	} else {
		errorJSON, _ := json.Marshal(result.Error)
		sb.WriteString(fmt.Sprintf("\"error\": %s", string(errorJSON)))
	}

	sb.WriteString("}\n")
	sb.WriteString("</tool_response>\n")

	return sb.String()
}
