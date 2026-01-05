package toolformat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAIFormatter handles OpenAI-compatible API tool format
// Used for OpenAI, vLLM, llama.cpp server, and other compatible APIs
type OpenAIFormatter struct{}

// NewOpenAIFormatter creates a new OpenAI formatter
func NewOpenAIFormatter() *OpenAIFormatter {
	return &OpenAIFormatter{}
}

// Name returns the formatter name
func (f *OpenAIFormatter) Name() string {
	return "openai"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
// For OpenAI API, tools are typically passed via the API, but this provides a fallback
func (f *OpenAIFormatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("# Available Functions\n\n")
	sb.WriteString("You have access to the following functions:\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("## %s\n\n", tool.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))

		if len(tool.Parameters.Properties) > 0 {
			sb.WriteString("**Parameters:**\n")
			for name, prop := range tool.Parameters.Properties {
				required := ""
				for _, req := range tool.Parameters.Required {
					if req == name {
						required = " (required)"
						break
					}
				}
				sb.WriteString(fmt.Sprintf("- `%s` (%s)%s: %s\n", name, prop.Type, required, prop.Description))
				if len(prop.Enum) > 0 {
					sb.WriteString(fmt.Sprintf("  - Allowed values: %s\n", strings.Join(prop.Enum, ", ")))
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("To call a function, output a JSON object in this format:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\"name\": \"function_name\", \"arguments\": {\"param1\": \"value1\"}}\n")
	sb.WriteString("```\n")

	return sb.String()
}

// OpenAITool represents a tool in OpenAI API format
type OpenAITool struct {
	Type     string          `json:"type"`
	Function OpenAIFunction  `json:"function"`
}

// OpenAIFunction represents a function in OpenAI API format
type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// FormatToolsAPI returns tool definitions in OpenAI API format
func (f *OpenAIFormatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	var result []OpenAITool

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
			if prop.Items != nil {
				propDef["items"] = map[string]interface{}{
					"type": prop.Items.Type,
				}
			}
			if len(prop.Properties) > 0 {
				nestedProps := make(map[string]interface{})
				for nestedName, nestedProp := range prop.Properties {
					nestedProps[nestedName] = map[string]interface{}{
						"type":        nestedProp.Type,
						"description": nestedProp.Description,
					}
				}
				propDef["properties"] = nestedProps
			}
			properties[name] = propDef
		}

		openaiTool := OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": properties,
					"required":   tool.Parameters.Required,
				},
			},
		}
		result = append(result, openaiTool)
	}

	return result
}

// OpenAIToolCall represents a tool call from OpenAI API response
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// ParseToolCalls extracts tool calls from an OpenAI API response
func (f *OpenAIFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Try to parse as OpenAI API tool_calls format
	var toolCalls []OpenAIToolCall
	if err := json.Unmarshal([]byte(response), &toolCalls); err == nil && len(toolCalls) > 0 {
		for _, tc := range toolCalls {
			if tc.Function.Name != "" {
				var args map[string]interface{}
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				calls = append(calls, ToolCall{
					Name: tc.Function.Name,
					Args: args,
				})
			}
		}
		if len(calls) > 0 {
			return calls, nil
		}
	}

	// Try single tool call
	var singleTC OpenAIToolCall
	if err := json.Unmarshal([]byte(response), &singleTC); err == nil && singleTC.Function.Name != "" {
		var args map[string]interface{}
		if singleTC.Function.Arguments != "" {
			json.Unmarshal([]byte(singleTC.Function.Arguments), &args)
		}
		return []ToolCall{{Name: singleTC.Function.Name, Args: args}}, nil
	}

	// Try simple format with name and arguments
	var simpleCall struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(response), &simpleCall); err == nil && simpleCall.Name != "" {
		return []ToolCall{{Name: simpleCall.Name, Args: simpleCall.Arguments}}, nil
	}

	// Fall back to generic parsing
	generic := NewGenericFormatter()
	return generic.ParseToolCalls(response)
}

// FormatToolResult formats a tool result for feeding back to the LLM
func (f *OpenAIFormatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Function: %s\n", call.Name))

	if result.Success {
		sb.WriteString("Status: Success\n")
		if result.Output != "" {
			output := result.Output
			if len(output) > 2000 {
				output = output[:2000] + "\n... (truncated)"
			}
			sb.WriteString(fmt.Sprintf("Result:\n%s\n", output))
		}
	} else {
		sb.WriteString("Status: Error\n")
		sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}

	return sb.String()
}

// OpenAIToolResult represents a tool result for OpenAI API
type OpenAIToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
}

// FormatToolResultAPI formats a tool result for the OpenAI API
func (f *OpenAIFormatter) FormatToolResultAPI(toolCallID string, result *ToolResult) OpenAIToolResult {
	content := result.Output
	if !result.Success {
		content = fmt.Sprintf("Error: %s", result.Error)
	}

	return OpenAIToolResult{
		ToolCallID: toolCallID,
		Role:       "tool",
		Content:    content,
	}
}
