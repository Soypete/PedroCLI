package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// MistralFormatter handles Mistral/Mixtral model tool format
// Mistral uses [TOOL_CALLS] format for tool calls
type MistralFormatter struct{}

// NewMistralFormatter creates a new Mistral formatter
func NewMistralFormatter() *MistralFormatter {
	return &MistralFormatter{}
}

// Name returns the formatter name
func (f *MistralFormatter) Name() string {
	return "mistral"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
func (f *MistralFormatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("[AVAILABLE_TOOLS]\n")

	for _, tool := range tools {
		toolJSON := f.formatToolDefinitionJSON(tool)
		sb.WriteString(toolJSON)
		sb.WriteString("\n")
	}

	sb.WriteString("[/AVAILABLE_TOOLS]\n\n")

	sb.WriteString("To call a tool, use the following format:\n")
	sb.WriteString("[TOOL_CALLS] [{\"name\": \"tool_name\", \"arguments\": {\"param\": \"value\"}}]\n\n")
	sb.WriteString("After receiving tool results, continue with your response.\n")

	return sb.String()
}

// formatToolDefinitionJSON formats a single tool definition as JSON
func (f *MistralFormatter) formatToolDefinitionJSON(tool ToolDefinition) string {
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

	data, _ := json.Marshal(def)
	return string(data)
}

// FormatToolsAPI returns tool definitions in OpenAI-compatible format
func (f *MistralFormatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
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

// ParseToolCalls extracts tool calls from a Mistral response
// Looks for [TOOL_CALLS] [...] format
func (f *MistralFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Strategy 1: Extract from [TOOL_CALLS] format
	re := regexp.MustCompile(`\[TOOL_CALLS\]\s*(\[.*?\])`)
	matches := re.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		// Parse the JSON array
		var rawCalls []struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}

		if err := json.Unmarshal([]byte(match[1]), &rawCalls); err != nil {
			continue
		}

		for _, c := range rawCalls {
			if c.Name != "" {
				calls = append(calls, ToolCall{
					Name: c.Name,
					Args: c.Arguments,
				})
			}
		}
	}

	// Fall back to generic parsing if no Mistral-format calls found
	if len(calls) == 0 {
		generic := NewGenericFormatter()
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// FormatToolResult formats a tool result for feeding back to the LLM
func (f *MistralFormatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString("[TOOL_RESULTS]\n")
	sb.WriteString(fmt.Sprintf("{\"name\": \"%s\", ", call.Name))

	if result.Success {
		contentJSON, _ := json.Marshal(result.Output)
		sb.WriteString(fmt.Sprintf("\"content\": %s}", string(contentJSON)))
	} else {
		errorJSON, _ := json.Marshal(result.Error)
		sb.WriteString(fmt.Sprintf("\"error\": %s}", string(errorJSON)))
	}

	sb.WriteString("\n[/TOOL_RESULTS]\n")

	return sb.String()
}
