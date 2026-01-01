package toolformat

import (
	"context"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/tools"
)

// mockExtendedTool implements ExtendedTool for testing
type mockExtendedTool struct {
	name        string
	description string
	metadata    *tools.ToolMetadata
}

func (m *mockExtendedTool) Name() string        { return m.name }
func (m *mockExtendedTool) Description() string { return m.description }
func (m *mockExtendedTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	return &tools.Result{Success: true}, nil
}
func (m *mockExtendedTool) Metadata() *tools.ToolMetadata { return m.metadata }

func createTestRegistry() *tools.ToolRegistry {
	registry := tools.NewToolRegistry()

	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "file",
		description: "Read, write, and modify files",
		metadata: &tools.ToolMetadata{
			Schema: &logits.JSONSchema{
				Type: "object",
				Properties: map[string]*logits.JSONSchema{
					"action": {
						Type:        "string",
						Description: "The operation to perform",
						Enum:        []interface{}{"read", "write"},
					},
					"path": {
						Type:        "string",
						Description: "File path",
					},
				},
				Required: []string{"action", "path"},
			},
		},
	})

	_ = registry.RegisterExtended(&mockExtendedTool{
		name:        "search",
		description: "Search code with patterns",
		metadata:    &tools.ToolMetadata{},
	})

	return registry
}

func TestGenericFormatter_ParseToolCalls(t *testing.T) {
	f := &GenericFormatter{}

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "single tool call",
			input:    `{"tool": "file", "args": {"action": "read", "path": "main.go"}}`,
			expected: 1,
		},
		{
			name:     "alternative format",
			input:    `{"name": "file", "arguments": {"action": "read", "path": "main.go"}}`,
			expected: 1,
		},
		{
			name:     "array of calls",
			input:    `[{"tool": "file", "args": {"action": "read"}}, {"tool": "search", "args": {"pattern": "test"}}]`,
			expected: 2,
		},
		{
			name:     "code block",
			input:    "Let me read the file:\n```json\n{\"tool\": \"file\", \"args\": {\"action\": \"read\"}}\n```",
			expected: 1,
		},
		{
			name:     "inline JSON in text",
			input:    "I will use the file tool:\n{\"tool\": \"file\", \"args\": {\"action\": \"read\"}}\nThat should work.",
			expected: 1,
		},
		{
			name:     "no tool calls",
			input:    "I will think about this problem.",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := f.ParseToolCalls(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(calls) != tt.expected {
				t.Errorf("Expected %d calls, got %d", tt.expected, len(calls))
			}
		})
	}
}

func TestQwenFormatter_ParseToolCalls(t *testing.T) {
	f := &QwenFormatter{Version: "2.5"}

	tests := []struct {
		name     string
		input    string
		expected int
		toolName string
	}{
		{
			name:     "tool_call tag",
			input:    "Let me read the file.\n<tool_call>\n{\"name\": \"file\", \"arguments\": {\"action\": \"read\"}}\n</tool_call>",
			expected: 1,
			toolName: "file",
		},
		{
			name:     "multiple tool_call tags",
			input:    "<tool_call>{\"name\": \"file\", \"arguments\": {}}</tool_call>\n<tool_call>{\"name\": \"search\", \"arguments\": {}}</tool_call>",
			expected: 2,
		},
		{
			name:     "fallback to generic",
			input:    `{"tool": "file", "args": {"action": "read"}}`,
			expected: 1,
			toolName: "file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := f.ParseToolCalls(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(calls) != tt.expected {
				t.Errorf("Expected %d calls, got %d", tt.expected, len(calls))
			}
			if tt.toolName != "" && len(calls) > 0 && calls[0].Name != tt.toolName {
				t.Errorf("Expected tool name %q, got %q", tt.toolName, calls[0].Name)
			}
		})
	}
}

func TestLlamaFormatter_ParseToolCalls(t *testing.T) {
	f := &LlamaFormatter{Version: "3.3"}

	tests := []struct {
		name     string
		input    string
		expected int
		toolName string
	}{
		{
			name:     "python_tag",
			input:    "I'll use the file tool:\n<|python_tag|>{\"name\": \"file\", \"arguments\": {\"action\": \"read\"}}",
			expected: 1,
			toolName: "file",
		},
		{
			name:     "fallback to generic",
			input:    `{"tool": "search", "args": {"pattern": "test"}}`,
			expected: 1,
			toolName: "search",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := f.ParseToolCalls(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(calls) != tt.expected {
				t.Errorf("Expected %d calls, got %d", tt.expected, len(calls))
			}
			if tt.toolName != "" && len(calls) > 0 && calls[0].Name != tt.toolName {
				t.Errorf("Expected tool name %q, got %q", tt.toolName, calls[0].Name)
			}
		})
	}
}

func TestMistralFormatter_ParseToolCalls(t *testing.T) {
	f := &MistralFormatter{}

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "TOOL_CALLS block",
			input:    "[TOOL_CALLS]\n[{\"name\": \"file\", \"arguments\": {\"action\": \"read\"}}]",
			expected: 1,
		},
		{
			name:     "multiple tools in block",
			input:    "[TOOL_CALLS]\n[{\"name\": \"file\", \"arguments\": {}}, {\"name\": \"search\", \"arguments\": {}}]",
			expected: 2,
		},
		{
			name:     "fallback to generic",
			input:    `{"tool": "file", "args": {"action": "read"}}`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls, err := f.ParseToolCalls(tt.input)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if len(calls) != tt.expected {
				t.Errorf("Expected %d calls, got %d", tt.expected, len(calls))
			}
		})
	}
}

func TestGetFormatter(t *testing.T) {
	tests := []struct {
		modelName    string
		expectedType string
	}{
		{"qwen2.5-coder:32b", "qwen"},
		{"qwen2.5:7b", "qwen"},
		{"llama-3.3:70b", "llama"},
		{"llama3.1", "llama"},
		{"mistral-large", "mistral"},
		{"mixtral-8x7b", "mistral"},
		{"unknown-model", "generic"},
		{"phi-3", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			formatter := GetFormatter(tt.modelName)
			if formatter.Name() != tt.expectedType {
				t.Errorf("For model %q, expected formatter %q, got %q",
					tt.modelName, tt.expectedType, formatter.Name())
			}
		})
	}
}

func TestFormatToolsForPrompt(t *testing.T) {
	registry := createTestRegistry()

	formatters := []ToolFormatter{
		&GenericFormatter{},
		&QwenFormatter{Version: "2.5"},
		&LlamaFormatter{Version: "3.3"},
		&MistralFormatter{},
	}

	for _, f := range formatters {
		t.Run(f.Name(), func(t *testing.T) {
			output := f.FormatToolsForPrompt(registry)

			// All formatters should include tool names
			if !strings.Contains(output, "file") {
				t.Error("Expected output to contain 'file' tool")
			}
			if !strings.Contains(output, "search") {
				t.Error("Expected output to contain 'search' tool")
			}
		})
	}
}

func TestFormatToolResult(t *testing.T) {
	call := ToolCall{Name: "file", Args: map[string]interface{}{"action": "read"}}

	successResult := &tools.Result{
		Success: true,
		Output:  "file contents here",
	}

	failResult := &tools.Result{
		Success: false,
		Error:   "file not found",
	}

	formatters := []ToolFormatter{
		&GenericFormatter{},
		&QwenFormatter{Version: "2.5"},
		&LlamaFormatter{Version: "3.3"},
		&MistralFormatter{},
	}

	for _, f := range formatters {
		t.Run(f.Name()+"_success", func(t *testing.T) {
			output := f.FormatToolResult(call, successResult)
			if !strings.Contains(output, "Success") {
				t.Error("Expected output to contain 'Success'")
			}
			if !strings.Contains(output, "file contents here") {
				t.Error("Expected output to contain tool output")
			}
		})

		t.Run(f.Name()+"_failure", func(t *testing.T) {
			output := f.FormatToolResult(call, failResult)
			if !strings.Contains(output, "Failed") {
				t.Error("Expected output to contain 'Failed'")
			}
			if !strings.Contains(output, "file not found") {
				t.Error("Expected output to contain error message")
			}
		})
	}
}

func TestSupportsNativeToolUse(t *testing.T) {
	formatters := []ToolFormatter{
		&GenericFormatter{},
		&QwenFormatter{Version: "2.5"},
		&LlamaFormatter{Version: "3.3"},
		&MistralFormatter{},
	}

	for _, f := range formatters {
		t.Run(f.Name(), func(t *testing.T) {
			// All local model formatters should return false
			if f.SupportsNativeToolUse() {
				t.Errorf("Expected %s to not support native tool use", f.Name())
			}
		})
	}
}
