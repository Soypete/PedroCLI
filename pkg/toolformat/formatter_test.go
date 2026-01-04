package toolformat

import (
	"testing"
)

func TestDetectModelFamily(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected ModelFamily
	}{
		{"Llama 3 lowercase", "llama3:70b", ModelFamilyLlama3},
		{"Llama 3.1", "llama-3.1-8b", ModelFamilyLlama3},
		{"Qwen 2.5", "qwen2.5-coder:32b", ModelFamilyQwen},
		{"Qwen lowercase", "qwen:7b", ModelFamilyQwen},
		{"Mistral", "mistral:7b", ModelFamilyMistral},
		{"Mixtral", "mixtral:8x7b", ModelFamilyMistral},
		{"Claude", "claude-3-opus", ModelFamilyClaude},
		{"OpenAI", "gpt-4-turbo", ModelFamilyOpenAI},
		{"Hermes", "hermes-2-yi-34b", ModelFamilyHermes},
		{"Nous", "nous-hermes-2", ModelFamilyHermes},
		{"Unknown", "unknown-model", ModelFamilyGeneric},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectModelFamily(tc.model)
			if result != tc.expected {
				t.Errorf("DetectModelFamily(%s) = %s, want %s", tc.model, result, tc.expected)
			}
		})
	}
}

func TestGenericFormatterParseToolCalls(t *testing.T) {
	formatter := NewGenericFormatter()

	tests := []struct {
		name     string
		response string
		expected int // number of tool calls
		toolName string
	}{
		{
			name:     "Single JSON object with tool field",
			response: `{"tool": "file", "args": {"action": "read", "path": "main.go"}}`,
			expected: 1,
			toolName: "file",
		},
		{
			name:     "Single JSON object with name field",
			response: `{"name": "search", "args": {"action": "grep", "pattern": "func"}}`,
			expected: 1,
			toolName: "search",
		},
		{
			name:     "JSON array of tool calls",
			response: `[{"tool": "file", "args": {"action": "read"}}, {"tool": "search", "args": {"action": "grep"}}]`,
			expected: 2,
			toolName: "file",
		},
		{
			name: "Tool call in code block",
			response: "```json\n{\"tool\": \"git\", \"args\": {\"action\": \"status\"}}\n```",
			expected: 1,
			toolName: "git",
		},
		{
			name:     "Tool call on a line",
			response: "Here's what I'll do:\n{\"tool\": \"bash\", \"args\": {\"command\": \"ls\"}}\nDone.",
			expected: 1,
			toolName: "bash",
		},
		{
			name:     "No tool calls",
			response: "I'll help you with that task. TASK_COMPLETE",
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := formatter.ParseToolCalls(tc.response)
			if err != nil {
				t.Fatalf("ParseToolCalls failed: %v", err)
			}
			if len(calls) != tc.expected {
				t.Errorf("got %d tool calls, want %d", len(calls), tc.expected)
			}
			if tc.expected > 0 && len(calls) > 0 && calls[0].Name != tc.toolName {
				t.Errorf("first tool name = %s, want %s", calls[0].Name, tc.toolName)
			}
		})
	}
}

func TestQwenFormatterParseToolCalls(t *testing.T) {
	formatter := NewQwenFormatter()

	tests := []struct {
		name     string
		response string
		expected int
		toolName string
	}{
		{
			name: "Single tool_call tag",
			response: `I'll read the file.
<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "main.go"}}
</tool_call>`,
			expected: 1,
			toolName: "file",
		},
		{
			name: "Multiple tool_call tags",
			response: `<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "a.go"}}
</tool_call>
<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "b.go"}}
</tool_call>`,
			expected: 2,
			toolName: "file",
		},
		{
			name:     "Fallback to generic parsing",
			response: `{"tool": "search", "args": {"action": "grep"}}`,
			expected: 1,
			toolName: "search",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := formatter.ParseToolCalls(tc.response)
			if err != nil {
				t.Fatalf("ParseToolCalls failed: %v", err)
			}
			if len(calls) != tc.expected {
				t.Errorf("got %d tool calls, want %d", len(calls), tc.expected)
			}
			if tc.expected > 0 && len(calls) > 0 && calls[0].Name != tc.toolName {
				t.Errorf("first tool name = %s, want %s", calls[0].Name, tc.toolName)
			}
		})
	}
}

func TestLlama3FormatterParseToolCalls(t *testing.T) {
	formatter := NewLlama3Formatter()

	tests := []struct {
		name     string
		response string
		expected int
		toolName string
	}{
		{
			name:     "Python tag format",
			response: "<|python_tag|>\n{\"name\": \"file\", \"parameters\": {\"action\": \"read\", \"path\": \"main.go\"}}<|eom_id|>",
			expected: 1,
			toolName: "file",
		},
		{
			name: "Multiple calls after python tag",
			response: `<|python_tag|>
{"name": "search", "parameters": {"action": "grep", "pattern": "func"}}
{"name": "file", "parameters": {"action": "read", "path": "main.go"}}
<|eot_id|>`,
			expected: 2,
			toolName: "search",
		},
		{
			name:     "Fallback to generic",
			response: `{"tool": "git", "args": {"action": "status"}}`,
			expected: 1,
			toolName: "git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := formatter.ParseToolCalls(tc.response)
			if err != nil {
				t.Fatalf("ParseToolCalls failed: %v", err)
			}
			if len(calls) != tc.expected {
				t.Errorf("got %d tool calls, want %d", len(calls), tc.expected)
			}
			if tc.expected > 0 && len(calls) > 0 && calls[0].Name != tc.toolName {
				t.Errorf("first tool name = %s, want %s", calls[0].Name, tc.toolName)
			}
		})
	}
}

func TestMistralFormatterParseToolCalls(t *testing.T) {
	formatter := NewMistralFormatter()

	tests := []struct {
		name     string
		response string
		expected int
		toolName string
	}{
		{
			name:     "TOOL_CALLS format",
			response: `[TOOL_CALLS] [{"name": "file", "arguments": {"action": "read", "path": "main.go"}}]`,
			expected: 1,
			toolName: "file",
		},
		{
			name:     "Multiple tool calls",
			response: `[TOOL_CALLS] [{"name": "search", "arguments": {"action": "grep"}}, {"name": "file", "arguments": {"action": "read"}}]`,
			expected: 2,
			toolName: "search",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls, err := formatter.ParseToolCalls(tc.response)
			if err != nil {
				t.Fatalf("ParseToolCalls failed: %v", err)
			}
			if len(calls) != tc.expected {
				t.Errorf("got %d tool calls, want %d", len(calls), tc.expected)
			}
			if tc.expected > 0 && len(calls) > 0 && calls[0].Name != tc.toolName {
				t.Errorf("first tool name = %s, want %s", calls[0].Name, tc.toolName)
			}
		})
	}
}

func TestFormatToolsPrompt(t *testing.T) {
	tools := []ToolDefinition{
		{
			Name:        "file",
			Description: "Read and write files",
			Category:    CategoryCode,
			Parameters:  FileToolSchema(),
		},
		{
			Name:        "search",
			Description: "Search code",
			Category:    CategoryCode,
			Parameters:  SearchToolSchema(),
		},
	}

	formatters := []ToolFormatter{
		NewGenericFormatter(),
		NewQwenFormatter(),
		NewLlama3Formatter(),
		NewMistralFormatter(),
		NewHermesFormatter(),
		NewClaudeFormatter(),
		NewOpenAIFormatter(),
	}

	for _, formatter := range formatters {
		t.Run(formatter.Name(), func(t *testing.T) {
			prompt := formatter.FormatToolsPrompt(tools)
			if prompt == "" {
				t.Error("FormatToolsPrompt returned empty string")
			}
			// Should contain tool names
			if !containsString(prompt, "file") {
				t.Error("prompt should contain 'file' tool name")
			}
			if !containsString(prompt, "search") {
				t.Error("prompt should contain 'search' tool name")
			}
		})
	}
}

func TestToolCallUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
	}{
		{
			name:     "name field",
			json:     `{"name": "file", "args": {"action": "read"}}`,
			expected: "file",
		},
		{
			name:     "tool field",
			json:     `{"tool": "search", "args": {"action": "grep"}}`,
			expected: "search",
		},
		{
			name:     "both fields (name takes precedence)",
			json:     `{"name": "file", "tool": "search", "args": {}}`,
			expected: "file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var call ToolCall
			if err := call.UnmarshalJSON([]byte(tc.json)); err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}
			if call.Name != tc.expected {
				t.Errorf("Name = %s, want %s", call.Name, tc.expected)
			}
		})
	}
}
