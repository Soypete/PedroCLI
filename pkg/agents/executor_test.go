package agents

import (
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
