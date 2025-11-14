package llm

import (
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "empty string",
			text: "",
			want: 0,
		},
		{
			name: "simple text",
			text: "hello world",
			want: 2, // 11 chars / 4 = 2
		},
		{
			name: "longer text",
			text: "The quick brown fox jumps over the lazy dog",
			want: 10, // 44 chars / 4 = 11 (integer division gives 10)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.want {
				t.Errorf("EstimateTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimateTokensAccurate(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "empty string",
			text: "",
			want: 0,
		},
		{
			name: "simple text",
			text: "hello world",
			want: 2, // 2 words * 1.3 = 2.6 = 2
		},
		{
			name: "multiple words",
			text: "The quick brown fox",
			want: 5, // 4 words * 1.3 = 5.2 = 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokensAccurate(tt.text)
			if got != tt.want {
				t.Errorf("EstimateTokensAccurate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateBudget(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			ContextSize: 32768,
		},
	}

	systemPrompt := "You are a helpful assistant."
	taskPrompt := "Help me write code."
	toolDefs := "tool1, tool2, tool3"

	budget := CalculateBudget(cfg, systemPrompt, taskPrompt, toolDefs)

	// Check that total matches config
	if budget.Total != 32768 {
		t.Errorf("Total = %v, want %v", budget.Total, 32768)
	}

	// Check that usable is 75% of total
	expectedUsable := 32768 * 3 / 4
	if budget.Usable != expectedUsable {
		t.Errorf("Usable = %v, want %v", budget.Usable, expectedUsable)
	}

	// Check that available is calculated
	if budget.Available <= 0 {
		t.Errorf("Available should be positive, got %v", budget.Available)
	}

	// Check that available is less than usable
	if budget.Available >= budget.Usable {
		t.Errorf("Available (%v) should be less than Usable (%v)", budget.Available, budget.Usable)
	}
}

func TestContextBudgetCanFitHistory(t *testing.T) {
	budget := &ContextBudget{
		Available: 1000,
	}

	tests := []struct {
		name          string
		historyTokens int
		want          bool
	}{
		{
			name:          "fits within budget",
			historyTokens: 500,
			want:          true,
		},
		{
			name:          "exactly at budget",
			historyTokens: 1000,
			want:          true,
		},
		{
			name:          "exceeds budget",
			historyTokens: 1500,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := budget.CanFitHistory(tt.historyTokens)
			if got != tt.want {
				t.Errorf("CanFitHistory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOllamaContextSize(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		want      int
	}{
		{
			name:      "known model 32k",
			modelName: "qwen2.5-coder:32b",
			want:      32768,
		},
		{
			name:      "known model 128k",
			modelName: "qwen2.5-coder:72b",
			want:      131072,
		},
		{
			name:      "unknown model",
			modelName: "unknown-model",
			want:      8192, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetOllamaContextSize(tt.modelName)
			if got != tt.want {
				t.Errorf("GetOllamaContextSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetUsableContext(t *testing.T) {
	tests := []struct {
		name        string
		contextSize int
		want        int
	}{
		{
			name:        "32k context",
			contextSize: 32768,
			want:        24576, // 75%
		},
		{
			name:        "128k context",
			contextSize: 131072,
			want:        98304, // 75%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetUsableContext(tt.contextSize)
			if got != tt.want {
				t.Errorf("GetUsableContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
