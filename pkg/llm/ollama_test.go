package llm

import (
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestNewOllamaBackend(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:          "ollama",
			ModelName:     "qwen2.5-coder:32b",
			ContextSize:   32768,
			UsableContext: 24576,
			Temperature:   0.2,
		},
	}

	backend := NewOllamaBackend(cfg)

	if backend == nil {
		t.Fatal("NewOllamaBackend returned nil")
	}

	if backend.modelName != "qwen2.5-coder:32b" {
		t.Errorf("Expected model name 'qwen2.5-coder:32b', got %s", backend.modelName)
	}

	if backend.contextSize != 32768 {
		t.Errorf("Expected context size 32768, got %d", backend.contextSize)
	}

	if backend.usableSize != 24576 {
		t.Errorf("Expected usable size 24576, got %d", backend.usableSize)
	}

	if backend.temperature != 0.2 {
		t.Errorf("Expected temperature 0.2, got %f", backend.temperature)
	}
}

func TestOllamaBackend_GetContextWindow(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:          "ollama",
			ModelName:     "qwen2.5-coder:32b",
			ContextSize:   32768,
			UsableContext: 24576,
		},
	}

	backend := NewOllamaBackend(cfg)

	if backend.GetContextWindow() != 32768 {
		t.Errorf("Expected context window 32768, got %d", backend.GetContextWindow())
	}
}

func TestOllamaBackend_GetUsableContext(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:          "ollama",
			ModelName:     "qwen2.5-coder:32b",
			ContextSize:   32768,
			UsableContext: 24576,
		},
	}

	backend := NewOllamaBackend(cfg)

	if backend.GetUsableContext() != 24576 {
		t.Errorf("Expected usable context 24576, got %d", backend.GetUsableContext())
	}
}

func TestOllamaBackend_buildPrompt(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:      "ollama",
			ModelName: "qwen2.5-coder:32b",
		},
	}

	backend := NewOllamaBackend(cfg)

	tests := []struct {
		name         string
		req          *InferenceRequest
		wantContains []string
	}{
		{
			name: "with system prompt",
			req: &InferenceRequest{
				SystemPrompt: "You are a helpful assistant",
				UserPrompt:   "Hello",
			},
			wantContains: []string{"System:", "You are a helpful assistant", "User:", "Hello", "Assistant:"},
		},
		{
			name: "without system prompt",
			req: &InferenceRequest{
				UserPrompt: "Hello",
			},
			wantContains: []string{"User:", "Hello", "Assistant:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := backend.buildPrompt(tt.req)
			for _, want := range tt.wantContains {
				if !contains(prompt, want) {
					t.Errorf("buildPrompt() result missing %q, got: %s", want, prompt)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
