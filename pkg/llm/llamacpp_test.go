package llm

import (
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

// TestNewLlamaCppClient tests client creation
func TestNewLlamaCppClient(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:        "llamacpp",
			ServerURL:   "http://localhost:8082",
			ModelName:   "test-model",
			ContextSize: 4096,
			EnableTools: true,
		},
	}

	client := NewLlamaCppClient(cfg)
	if client == nil {
		t.Fatal("NewLlamaCppClient returned nil")
	}

	if client.GetContextWindow() != 4096 {
		t.Errorf("GetContextWindow() = %d, want 4096", client.GetContextWindow())
	}

	expectedUsable := 4096 * 3 / 4 // 75%
	if client.GetUsableContext() != expectedUsable {
		t.Errorf("GetUsableContext() = %d, want %d", client.GetUsableContext(), expectedUsable)
	}
}

// Note: Integration tests with real llama-server require the server to be running
// and are better suited for manual testing or separate integration test suite.
// See docs/MIGRATION-LLAMA-SERVER.md for testing guide.
