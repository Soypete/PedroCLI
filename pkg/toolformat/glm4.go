package toolformat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GLM4Formatter handles GLM-4 model tool format
// GLM-4 uses OpenAI-compatible tool calling with native API support
// It also supports a "reasoning_content" field in responses (CoT-style reasoning)
type GLM4Formatter struct{}

// NewGLM4Formatter creates a new GLM-4 formatter
func NewGLM4Formatter() *GLM4Formatter {
	return &GLM4Formatter{}
}

// Name returns the formatter name
func (f *GLM4Formatter) Name() string {
	return "glm4"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
// GLM-4 uses OpenAI-style tool calling, but we still provide prompt-based format as fallback
func (f *GLM4Formatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("# Available Tools\n\n")
	sb.WriteString("You have access to the following tools. Use them by calling the tool with appropriate arguments.\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("## %s\n", tool.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))

		if len(tool.Parameters.Properties) > 0 {
			sb.WriteString("Parameters:\n")
			for name, prop := range tool.Parameters.Properties {
				required := ""
				for _, req := range tool.Parameters.Required {
					if req == name {
						required = " (required)"
						break
					}
				}
				sb.WriteString(fmt.Sprintf("- %s (%s)%s: %s\n", name, prop.Type, required, prop.Description))
				if len(prop.Enum) > 0 {
					sb.WriteString(fmt.Sprintf("  Allowed values: %s\n", strings.Join(prop.Enum, ", ")))
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("# Tool Call Format\n\n")
	sb.WriteString("To call a tool, output a JSON object:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\"name\": \"tool_name\", \"args\": {\"param1\": \"value1\"}}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("You can call multiple tools by using the native tool calling API.\n")

	return sb.String()
}

// FormatToolsAPI returns tool definitions in OpenAI-compatible format for API use
// GLM-4 supports native tool calling via the API
func (f *GLM4Formatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	var result []map[string]interface{}

	for _, tool := range tools {
		// Convert parameters to proper JSON schema format
		params := f.convertParametersToSchema(tool.Parameters)

		def := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  params,
			},
		}
		result = append(result, def)
	}

	return result
}

// convertParametersToSchema converts ParameterSchema to JSON Schema format
func (f *GLM4Formatter) convertParametersToSchema(params ParameterSchema) map[string]interface{} {
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
			if prop.Items.Description != "" {
				propDef["items"].(map[string]interface{})["description"] = prop.Items.Description
			}
		}
		if prop.Properties != nil {
			// Nested object properties
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

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(params.Required) > 0 {
		schema["required"] = params.Required
	}

	return schema
}

// ParseToolCalls extracts tool calls from a GLM-4 response
// Supports multiple formats:
// 1. Native API tool calls (already parsed by LLM backend)
// 2. JSON objects in text
// 3. Markdown code blocks with JSON
func (f *GLM4Formatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Strategy 1: Try parsing entire response as JSON array
	var arrayOfCalls []ToolCall
	if err := json.Unmarshal([]byte(response), &arrayOfCalls); err == nil && len(arrayOfCalls) > 0 {
		return filterValidCalls(arrayOfCalls), nil
	}

	// Strategy 2: Try parsing as single JSON object
	var singleCall ToolCall
	if err := json.Unmarshal([]byte(response), &singleCall); err == nil && singleCall.Name != "" {
		return []ToolCall{singleCall}, nil
	}

	// Strategy 3: Extract from markdown code blocks
	calls = extractFromCodeBlocks(response)
	if len(calls) > 0 {
		return calls, nil
	}

	// Strategy 4: Line-by-line JSON parsing
	calls = extractFromLines(response)
	if len(calls) > 0 {
		return calls, nil
	}

	// Strategy 5: Find JSON objects in text
	calls = extractJSONObjects(response)
	return calls, nil
}

// FormatToolResult formats a tool result for feeding back to the LLM
func (f *GLM4Formatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tool: %s\n", call.Name))

	if result.Success {
		sb.WriteString("Status: Success\n")
		if result.Output != "" {
			sb.WriteString(fmt.Sprintf("Output:\n%s\n", result.Output))
		}
		if len(result.ModifiedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("Modified files: %s\n", strings.Join(result.ModifiedFiles, ", ")))
		}
	} else {
		sb.WriteString("Status: Failed\n")
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		}
	}

	return sb.String()
}
