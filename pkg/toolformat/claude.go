package toolformat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ClaudeFormatter handles Claude API native tool format
// Claude uses structured tool_use blocks in responses
type ClaudeFormatter struct{}

// NewClaudeFormatter creates a new Claude formatter
func NewClaudeFormatter() *ClaudeFormatter {
	return &ClaudeFormatter{}
}

// Name returns the formatter name
func (f *ClaudeFormatter) Name() string {
	return "claude"
}

// FormatToolsPrompt generates the tool definitions portion of the system prompt
// For Claude API, tools are passed via the API, not in the prompt
// This returns a minimal prompt for cases where prompt-based tools are needed
func (f *ClaudeFormatter) FormatToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("You have access to the following tools:\n\n")

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
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("To use a tool, respond with a tool_use block:\n")
	sb.WriteString("```\n")
	sb.WriteString("<tool_use>\n")
	sb.WriteString("name: tool_name\n")
	sb.WriteString("arguments: {\"param\": \"value\"}\n")
	sb.WriteString("</tool_use>\n")
	sb.WriteString("```\n")

	return sb.String()
}

// ClaudeToolDefinition represents a tool in Claude API format
type ClaudeToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// FormatToolsAPI returns tool definitions in Claude API native format
func (f *ClaudeFormatter) FormatToolsAPI(tools []ToolDefinition) interface{} {
	var result []ClaudeToolDefinition

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
			properties[name] = propDef
		}

		def := ClaudeToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   tool.Parameters.Required,
			},
		}
		result = append(result, def)
	}

	return result
}

// ClaudeToolUse represents a tool use from Claude API response
type ClaudeToolUse struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ParseToolCalls extracts tool calls from a Claude response
// Claude API returns structured tool_use blocks
func (f *ClaudeFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
	var calls []ToolCall

	// Try to parse as Claude API response content
	// Claude API responses have content blocks with type "tool_use"
	var contentBlocks []ClaudeToolUse
	if err := json.Unmarshal([]byte(response), &contentBlocks); err == nil {
		for _, block := range contentBlocks {
			if block.Type == "tool_use" && block.Name != "" {
				calls = append(calls, ToolCall{
					Name: block.Name,
					Args: block.Input,
				})
			}
		}
		if len(calls) > 0 {
			return calls, nil
		}
	}

	// Try single tool_use block
	var singleBlock ClaudeToolUse
	if err := json.Unmarshal([]byte(response), &singleBlock); err == nil {
		if singleBlock.Type == "tool_use" && singleBlock.Name != "" {
			return []ToolCall{{Name: singleBlock.Name, Args: singleBlock.Input}}, nil
		}
	}

	// Fall back to generic parsing for text-based responses
	generic := NewGenericFormatter()
	return generic.ParseToolCalls(response)
}

// FormatToolResult formats a tool result for feeding back to Claude
func (f *ClaudeFormatter) FormatToolResult(call ToolCall, result *ToolResult) string {
	// Claude expects tool_result content blocks
	// For text format, we use a structured response
	var sb strings.Builder

	sb.WriteString("<tool_result>\n")
	sb.WriteString(fmt.Sprintf("tool_use_id: %s\n", call.Name)) // Use name as ID for text format

	if result.Success {
		content := result.Output
		if len(content) > 2000 {
			content = content[:2000] + "\n... (truncated)"
		}
		sb.WriteString(fmt.Sprintf("content: %s\n", content))
	} else {
		sb.WriteString("is_error: true\n")
		sb.WriteString(fmt.Sprintf("content: %s\n", result.Error))
	}

	sb.WriteString("</tool_result>\n")

	return sb.String()
}

// ClaudeToolResult represents a tool result for Claude API
type ClaudeToolResult struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// FormatToolResultAPI formats a tool result for the Claude API
func (f *ClaudeFormatter) FormatToolResultAPI(toolUseID string, result *ToolResult) ClaudeToolResult {
	if result.Success {
		return ClaudeToolResult{
			Type:      "tool_result",
			ToolUseID: toolUseID,
			Content:   result.Output,
		}
	}
	return ClaudeToolResult{
		Type:      "tool_result",
		ToolUseID: toolUseID,
		Content:   result.Error,
		IsError:   true,
	}
}
