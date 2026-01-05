package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Llama3Formatter handles Llama 3.x model tool format
// Llama 3 uses <|python_tag|> for code/function output and a specific JSON format
type Llama3Formatter struct{}

// NewLlama3Formatter creates a new Llama 3 formatter
func NewLlama3Formatter() *Llama3Formatter {
	return &Llama3Formatter{}
}

// Name returns the formatter name
func (f *Llama3Formatter) Name() string {
	return "llama3"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
// Llama 3.1+ uses a specific format for tool definitions
func (f *Llama3Formatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("Environment: ipython\n")
	sb.WriteString("Tools: ")

	// List tool names
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	sb.WriteString(strings.Join(names, ", "))
	sb.WriteString("\n\n")

	sb.WriteString("Cutting Knowledge Date: December 2023\n")
	sb.WriteString("Today Date: 2024\n\n")

	sb.WriteString("# Tool Definitions\n\n")
	sb.WriteString("You have access to the following tools:\n\n")

	for _, tool := range tools {
		sb.WriteString(f.formatToolDefinition(tool))
		sb.WriteString("\n\n")
	}

	sb.WriteString("# Tool Call Format\n\n")
	sb.WriteString("When you need to call a tool, use the following format:\n\n")
	sb.WriteString("<|python_tag|>\n")
	sb.WriteString("{\"name\": \"tool_name\", \"parameters\": {\"param1\": \"value1\"}}\n")
	sb.WriteString("\n")
	sb.WriteString("You can call multiple tools by outputting multiple JSON objects.\n")
	sb.WriteString("After receiving tool results, continue with your analysis.\n")

	return sb.String()
}

// formatToolDefinition formats a single tool definition
func (f *Llama3Formatter) formatToolDefinition(tool ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s\n\n", tool.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))

	if len(tool.Parameters.Properties) > 0 {
		sb.WriteString("Parameters:\n")
		for name, prop := range tool.Parameters.Properties {
			required := ""
			for _, req := range tool.Parameters.Required {
				if req == name {
					required = " [required]"
					break
				}
			}
			sb.WriteString(fmt.Sprintf("  - %s (%s)%s: %s\n", name, prop.Type, required, prop.Description))
			if len(prop.Enum) > 0 {
				sb.WriteString(fmt.Sprintf("    Allowed values: %s\n", strings.Join(prop.Enum, ", ")))
			}
		}
	}

	return sb.String()
}

// FormatToolsAPI returns tool definitions for Llama 3 API format
func (f *Llama3Formatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	// Llama 3 via APIs like Together.ai use OpenAI-compatible format
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

// ParseToolCalls extracts tool calls from a Llama 3 response
// Looks for JSON after <|python_tag|> or in code blocks
func (f *Llama3Formatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Strategy 1: Extract from <|python_tag|> sections
	pythonTagRe := regexp.MustCompile(`(?s)<\|python_tag\|>\s*(.*?)(?:<\|eom_id\|>|<\|eot_id\|>|$)`)
	matches := pythonTagRe.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		content := strings.TrimSpace(match[1])
		parsedCalls := f.parseToolJSON(content)
		calls = append(calls, parsedCalls...)
	}

	// Strategy 2: Look for JSON objects with "name" and "parameters" fields
	if len(calls) == 0 {
		calls = f.extractLlama3FormatCalls(response)
	}

	// Strategy 3: Fall back to generic parsing
	if len(calls) == 0 {
		generic := NewGenericFormatter()
		return generic.ParseToolCalls(response)
	}

	return calls, nil
}

// parseToolJSON parses tool calls from JSON content
func (f *Llama3Formatter) parseToolJSON(content string) []ToolCall {
	var calls []ToolCall

	// Try as array
	var arrayCalls []struct {
		Name       string                 `json:"name"`
		Parameters map[string]interface{} `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(content), &arrayCalls); err == nil {
		for _, c := range arrayCalls {
			if c.Name != "" {
				calls = append(calls, ToolCall{Name: c.Name, Args: c.Parameters})
			}
		}
		return calls
	}

	// Try as single object
	var singleCall struct {
		Name       string                 `json:"name"`
		Parameters map[string]interface{} `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(content), &singleCall); err == nil && singleCall.Name != "" {
		return []ToolCall{{Name: singleCall.Name, Args: singleCall.Parameters}}
	}

	// Try line by line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var call struct {
			Name       string                 `json:"name"`
			Parameters map[string]interface{} `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(line), &call); err == nil && call.Name != "" {
			calls = append(calls, ToolCall{Name: call.Name, Args: call.Parameters})
		}
	}

	return calls
}

// extractLlama3FormatCalls extracts calls in Llama 3 format from text
func (f *Llama3Formatter) extractLlama3FormatCalls(text string) []ToolCall {
	var calls []ToolCall

	// Find JSON objects that look like Llama 3 tool calls
	re := regexp.MustCompile(`\{[^{}]*"name"\s*:\s*"[^"]+"\s*,[^{}]*"parameters"\s*:\s*\{[^{}]*\}[^{}]*\}`)
	matches := re.FindAllString(text, -1)

	for _, match := range matches {
		var call struct {
			Name       string                 `json:"name"`
			Parameters map[string]interface{} `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(match), &call); err == nil && call.Name != "" {
			calls = append(calls, ToolCall{Name: call.Name, Args: call.Parameters})
		}
	}

	return calls
}

// FormatToolResult formats a tool result for feeding back to the LLM
func (f *Llama3Formatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<|start_header_id|>ipython<|end_header_id|>\n\n"))
	sb.WriteString(fmt.Sprintf("Tool: %s\n", call.Name))

	if result.Success {
		sb.WriteString("Status: Success\n")
		if result.Output != "" {
			output := result.Output
			if len(output) > 2000 {
				output = output[:2000] + "\n... (truncated)"
			}
			sb.WriteString(fmt.Sprintf("Output:\n%s\n", output))
		}
	} else {
		sb.WriteString("Status: Failed\n")
		sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}

	return sb.String()
}
