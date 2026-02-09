package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/tools"
)

func TestBuildFeedbackPrompt_Truncates(t *testing.T) {
	// Create a minimal InferenceExecutor with config for testing
	cfg := &config.Config{
		Context: config.ContextConfig{
			ToolResultLimits: map[string]int{
				"web_search": 500,
				"default":    500,
			},
		},
	}
	executor := &InferenceExecutor{
		agent: &BaseAgent{
			config: cfg,
		},
	}

	tests := []struct {
		name          string
		calls         []llm.ToolCall
		results       []*tools.Result
		expectedSize  int
		shouldContain []string
	}{
		{
			name: "small output not truncated",
			calls: []llm.ToolCall{
				{Name: "test_tool", Args: map[string]interface{}{}},
			},
			results: []*tools.Result{
				{
					Success: true,
					Output:  "Small result",
				},
			},
			expectedSize: 500, // Should be small
			shouldContain: []string{
				"✅ test_tool",
				"Small result",
			},
		},
		{
			name: "large output truncated",
			calls: []llm.ToolCall{
				{Name: "web_search", Args: map[string]interface{}{}},
			},
			results: []*tools.Result{
				{
					Success: true,
					Output:  strings.Repeat("Large search result content ", 5000), // ~140K chars
				},
			},
			expectedSize: 2000, // Should be much smaller than input
			shouldContain: []string{
				"✅ web_search",
				"[Output truncated",
				"Full result saved to context files",
			},
		},
		{
			name: "error message truncated",
			calls: []llm.ToolCall{
				{Name: "failing_tool", Args: map[string]interface{}{}},
			},
			results: []*tools.Result{
				{
					Success: false,
					Error:   strings.Repeat("Error details ", 200), // ~2800 chars
				},
			},
			expectedSize: 1500,
			shouldContain: []string{
				"❌ failing_tool failed",
				"[Output truncated",
			},
		},
		{
			name: "multiple tools with mixed sizes",
			calls: []llm.ToolCall{
				{Name: "tool1", Args: map[string]interface{}{}},
				{Name: "tool2", Args: map[string]interface{}{}},
				{Name: "tool3", Args: map[string]interface{}{}},
			},
			results: []*tools.Result{
				{Success: true, Output: "Small"},
				{Success: true, Output: strings.Repeat("Big ", 5000)},
				{Success: false, Error: "Error"},
			},
			expectedSize: 3000,
			shouldContain: []string{
				"✅ tool1",
				"✅ tool2",
				"❌ tool3",
				"[Output truncated",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := executor.buildFeedbackPrompt(tt.calls, tt.results)

			// Check size constraint
			if len(prompt) > tt.expectedSize {
				t.Errorf("Expected prompt size <= %d, got %d", tt.expectedSize, len(prompt))
			}

			// Check required content
			for _, required := range tt.shouldContain {
				if !strings.Contains(prompt, required) {
					t.Errorf("Expected prompt to contain %q", required)
				}
			}

			// All prompts should contain the continuation instruction
			if !strings.Contains(prompt, "Based on these results") {
				t.Error("Expected continuation instruction in prompt")
			}
		})
	}
}

func TestBuildFeedbackPrompt_ContextExplosionPrevention(t *testing.T) {
	// Create a minimal InferenceExecutor with config for testing
	cfg := &config.Config{
		Context: config.ContextConfig{
			ToolResultLimits: map[string]int{
				"web_search": 500,
				"default":    500,
			},
		},
	}
	executor := &InferenceExecutor{
		agent: &BaseAgent{
			config: cfg,
		},
	}

	// Simulate 20 tool calls with large outputs (like a blog research workflow)
	calls := make([]llm.ToolCall, 20)
	results := make([]*tools.Result, 20)

	for i := 0; i < 20; i++ {
		calls[i] = llm.ToolCall{
			Name: "web_search",
			Args: map[string]interface{}{"query": "test"},
		}
		// Each result is ~40K chars (realistic for web search)
		results[i] = &tools.Result{
			Success: true,
			Output:  strings.Repeat("Search result content with lots of detail ", 1000),
		}
	}

	prompt := executor.buildFeedbackPrompt(calls, results)

	// Before fix: 20 tools × 40K = 800K chars
	// After fix: 20 tools × 1K = ~20K chars + overhead
	maxExpectedSize := 30000 // 30K chars should be plenty

	if len(prompt) > maxExpectedSize {
		t.Errorf("Context explosion detected! Prompt is %d chars, expected <= %d", len(prompt), maxExpectedSize)
	}

	// Should have truncation messages
	truncationCount := strings.Count(prompt, "[Output truncated")
	if truncationCount != 20 {
		t.Errorf("Expected 20 truncation messages, got %d", truncationCount)
	}

	t.Logf("✅ Prompt with 20 large tool results: %d chars (under %d limit)", len(prompt), maxExpectedSize)
}

// TestValidateToolCalls_ValidCalls tests that valid tool calls pass through validation
func TestValidateToolCalls_ValidCalls(t *testing.T) {
	// Create executor with test tools
	executor := &InferenceExecutor{
		agent: &BaseAgent{
			tools: map[string]tools.Tool{
				"search": &mockTool{name: "search"},
				"file":   &mockTool{name: "file"},
			},
		},
	}

	tests := []struct {
		name  string
		calls []llm.ToolCall
	}{
		{
			name: "search with action",
			calls: []llm.ToolCall{
				{Name: "search", Args: map[string]interface{}{"action": "grep", "pattern": "test"}},
			},
		},
		{
			name: "file with action",
			calls: []llm.ToolCall{
				{Name: "file", Args: map[string]interface{}{"action": "read", "path": "test.go"}},
			},
		},
		{
			name: "multiple valid calls",
			calls: []llm.ToolCall{
				{Name: "search", Args: map[string]interface{}{"action": "find_files", "pattern": "*.go"}},
				{Name: "file", Args: map[string]interface{}{"action": "write", "path": "test.go", "content": "..."}},
				{Name: "search", Args: map[string]interface{}{"action": "find_definition", "name": "HandleRequest"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errors := executor.validateToolCalls(tt.calls)

			if len(errors) != 0 {
				t.Errorf("Expected no errors, got: %v", errors)
			}

			if len(valid) != len(tt.calls) {
				t.Errorf("Expected %d valid calls, got %d", len(tt.calls), len(valid))
			}
		})
	}
}

// TestValidateToolCalls_InvalidToolNames tests detection of invalid tool names
func TestValidateToolCalls_InvalidToolNames(t *testing.T) {
	executor := &InferenceExecutor{
		agent: &BaseAgent{
			tools: map[string]tools.Tool{
				"search": &mockTool{name: "search"},
				"file":   &mockTool{name: "file"},
			},
		},
	}

	tests := []struct {
		name          string
		calls         []llm.ToolCall
		expectedError string
	}{
		{
			name: "action name used as tool - grep",
			calls: []llm.ToolCall{
				{Name: "grep", Args: map[string]interface{}{"pattern": "test"}},
			},
			expectedError: "Did you mean: {\"tool\": \"search\", \"args\": {\"action\": \"grep\"",
		},
		{
			name: "action name used as tool - find_files",
			calls: []llm.ToolCall{
				{Name: "find_files", Args: map[string]interface{}{"pattern": "*.go"}},
			},
			expectedError: "Did you mean: {\"tool\": \"search\", \"args\": {\"action\": \"find_files\"",
		},
		{
			name: "action name used as tool - read",
			calls: []llm.ToolCall{
				{Name: "read", Args: map[string]interface{}{"path": "test.go"}},
			},
			expectedError: "Did you mean: {\"tool\": \"search\", \"args\": {\"action\": \"read\"",
		},
		{
			name: "unknown tool",
			calls: []llm.ToolCall{
				{Name: "unknown_tool", Args: map[string]interface{}{}},
			},
			expectedError: "Tool 'unknown_tool' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errors := executor.validateToolCalls(tt.calls)

			if len(errors) == 0 {
				t.Error("Expected validation errors, got none")
			}

			if len(valid) != 0 {
				t.Errorf("Expected no valid calls, got %d", len(valid))
			}

			if !strings.Contains(errors[0], tt.expectedError) {
				t.Errorf("Expected error to contain %q, got: %s", tt.expectedError, errors[0])
			}
		})
	}
}

// TestValidateToolCalls_MissingAction tests detection of missing 'action' parameter
func TestValidateToolCalls_MissingAction(t *testing.T) {
	executor := &InferenceExecutor{
		agent: &BaseAgent{
			tools: map[string]tools.Tool{
				"search": &mockTool{name: "search"},
				"file":   &mockTool{name: "file"},
			},
		},
	}

	tests := []struct {
		name          string
		calls         []llm.ToolCall
		expectedError string
	}{
		{
			name: "search missing action",
			calls: []llm.ToolCall{
				{Name: "search", Args: map[string]interface{}{"pattern": "test"}},
			},
			expectedError: "missing required 'action' parameter",
		},
		{
			name: "file missing action",
			calls: []llm.ToolCall{
				{Name: "file", Args: map[string]interface{}{"path": "test.go"}},
			},
			expectedError: "missing required 'action' parameter",
		},
		{
			name: "search with type instead of action",
			calls: []llm.ToolCall{
				{Name: "search", Args: map[string]interface{}{"type": "grep", "pattern": "test"}},
			},
			expectedError: "parameter is named 'action', not 'type'",
		},
		{
			name: "file with type instead of action",
			calls: []llm.ToolCall{
				{Name: "file", Args: map[string]interface{}{"type": "read", "path": "test.go"}},
			},
			expectedError: "parameter is named 'action', not 'type'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errors := executor.validateToolCalls(tt.calls)

			if len(errors) == 0 {
				t.Error("Expected validation errors, got none")
			}

			if len(valid) != 0 {
				t.Errorf("Expected no valid calls, got %d", len(valid))
			}

			if !strings.Contains(errors[0], tt.expectedError) {
				t.Errorf("Expected error to contain %q, got: %s", tt.expectedError, errors[0])
			}
		})
	}
}

// TestValidateToolCalls_MixedValidAndInvalid tests validation with mix of good and bad calls
func TestValidateToolCalls_MixedValidAndInvalid(t *testing.T) {
	executor := &InferenceExecutor{
		agent: &BaseAgent{
			tools: map[string]tools.Tool{
				"search": &mockTool{name: "search"},
				"file":   &mockTool{name: "file"},
			},
		},
	}

	calls := []llm.ToolCall{
		{Name: "search", Args: map[string]interface{}{"action": "grep", "pattern": "test"}},       // ✅ valid
		{Name: "grep", Args: map[string]interface{}{"pattern": "test"}},                           // ❌ invalid tool name
		{Name: "file", Args: map[string]interface{}{"action": "read", "path": "test.go"}},         // ✅ valid
		{Name: "search", Args: map[string]interface{}{"pattern": "test"}},                         // ❌ missing action
		{Name: "file", Args: map[string]interface{}{"type": "write", "path": "test.go"}},          // ❌ type instead of action
		{Name: "search", Args: map[string]interface{}{"action": "find_files", "pattern": "*.go"}}, // ✅ valid
	}

	valid, errors := executor.validateToolCalls(calls)

	// Should have 3 valid calls
	if len(valid) != 3 {
		t.Errorf("Expected 3 valid calls, got %d", len(valid))
	}

	// Should have 3 error messages
	if len(errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(errors))
	}

	// Check valid calls are the right ones
	expectedValid := []string{"search", "file", "search"}
	for i, call := range valid {
		if call.Name != expectedValid[i] {
			t.Errorf("Valid call %d: expected %s, got %s", i, expectedValid[i], call.Name)
		}
	}

	// Check errors contain expected strings
	errorChecks := []string{
		"Did you mean",         // grep as tool name
		"missing required",     // missing action
		"'action', not 'type'", // type instead of action
	}

	for i, expectedStr := range errorChecks {
		if !strings.Contains(errors[i], expectedStr) {
			t.Errorf("Error %d should contain %q, got: %s", i, expectedStr, errors[i])
		}
	}
}

// TestIsSearchAction tests search action name detection
func TestIsSearchAction(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"grep", true},
		{"find_files", true},
		{"find_in_file", true},
		{"find_definition", true},
		{"search", false},
		{"read", false},
		{"write", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSearchAction(tt.name)
			if result != tt.expected {
				t.Errorf("isSearchAction(%q) = %v, expected %v", tt.name, result, tt.expected)
			}
		})
	}
}

// TestIsFileAction tests file action name detection
func TestIsFileAction(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"read", true},
		{"write", true},
		{"replace", true},
		{"append", true},
		{"delete", true},
		{"file", false},
		{"grep", false},
		{"search", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFileAction(tt.name)
			if result != tt.expected {
				t.Errorf("isFileAction(%q) = %v, expected %v", tt.name, result, tt.expected)
			}
		})
	}
}

// mockTool is a minimal tool implementation for testing
type mockTool struct {
	name string
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return "Mock tool for testing"
}

func (m *mockTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	return &tools.Result{Success: true}, nil
}
