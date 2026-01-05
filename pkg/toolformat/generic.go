package toolformat

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// GenericFormatter is a fallback formatter that uses simple JSON format
type GenericFormatter struct{}

// NewGenericFormatter creates a new generic formatter
func NewGenericFormatter() *GenericFormatter {
	return &GenericFormatter{}
}

// Name returns the formatter name
func (f *GenericFormatter) Name() string {
	return "generic"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
func (f *GenericFormatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("# Available Tools\n\n")
	sb.WriteString("You have access to the following tools. Use them by outputting a JSON object with \"tool\" and \"args\" fields.\n\n")

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
	sb.WriteString("{\"tool\": \"tool_name\", \"args\": {\"param1\": \"value1\"}}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("You can call multiple tools by outputting multiple JSON objects on separate lines.\n")

	return sb.String()
}

// FormatToolsAPI returns nil as generic format doesn't support native API tool use
func (f *GenericFormatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	return nil
}

// ParseToolCalls extracts tool calls from an LLM response
func (f *GenericFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
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
func (f *GenericFormatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	var sb strings.Builder

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
		if len(result.ModifiedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("Modified files: %v\n", result.ModifiedFiles))
		}
	} else {
		sb.WriteString("Status: Failed\n")
		sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}

	return sb.String()
}

// Helper functions for parsing

func filterValidCalls(calls []ToolCall) []ToolCall {
	var valid []ToolCall
	for _, call := range calls {
		if call.Name != "" {
			valid = append(valid, call)
		}
	}
	return valid
}

func extractFromCodeBlocks(text string) []ToolCall {
	var calls []ToolCall

	// Match ```json...``` or ```...``` blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		// Try as array first
		var arrayCalls []ToolCall
		if err := json.Unmarshal([]byte(match[1]), &arrayCalls); err == nil {
			calls = append(calls, filterValidCalls(arrayCalls)...)
			continue
		}

		// Try as single object
		var call ToolCall
		if err := json.Unmarshal([]byte(match[1]), &call); err == nil && call.Name != "" {
			calls = append(calls, call)
		}
	}

	return calls
}

func extractFromLines(text string) []ToolCall {
	var calls []ToolCall
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Must start with { to be JSON
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var call ToolCall
		if err := json.Unmarshal([]byte(line), &call); err == nil && call.Name != "" {
			calls = append(calls, call)
		}
	}

	return calls
}

func extractJSONObjects(text string) []ToolCall {
	var calls []ToolCall

	// Find all potential JSON objects
	depth := 0
	start := -1

	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				jsonStr := text[start : i+1]
				var call ToolCall
				if err := json.Unmarshal([]byte(jsonStr), &call); err == nil && call.Name != "" {
					calls = append(calls, call)
				}
				start = -1
			}
		}
	}

	return calls
}
