package llm

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
)

// TestLlamaCppInfer tests the llama.cpp inference with a real model
// DEPRECATED: This test is for the old CLI-based llama.cpp client.
// The project has migrated to llama-server HTTP API. Skip this test.
func TestLlamaCppInfer(t *testing.T) {
	t.Skip("Skipping deprecated CLI-based llama.cpp test - migrated to llama-server HTTP API")

	llamaPath := os.Getenv("LLAMA_CPP_PATH")
	modelPath := os.Getenv("LLAMA_MODEL_PATH")

	if llamaPath == "" || modelPath == "" {
		t.Skip("Skipping llama.cpp test: LLAMA_CPP_PATH or LLAMA_MODEL_PATH not set")
	}

	// Verify the binary exists
	if _, err := os.Stat(llamaPath); os.IsNotExist(err) {
		t.Skipf("Skipping llama.cpp test: binary not found at %s", llamaPath)
	}

	// Verify the model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skipf("Skipping llama.cpp test: model not found at %s", modelPath)
	}

	// Create a test config
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:        "llamacpp",
			ServerURL:   "http://localhost:8082",
			ModelName:   "test-model",
			ContextSize: 4096,
			Temperature: 0.2,
		},
	}

	// Create client
	client := NewLlamaCppClient(cfg)

	// Create a simple inference request
	req := &InferenceRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "What is 2+2? Answer with just the number.",
		MaxTokens:    50,
		Temperature:  0.1,
	}

	// Run inference with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Inference failed: %v", err)
	}

	// Verify we got a response
	if resp == nil {
		t.Fatal("Response is nil")
	}

	if resp.Text == "" {
		t.Fatal("Response text is empty")
	}

	t.Logf("LLM Response: %s", resp.Text)

	// Check that debug output was written
	debugFile := "/tmp/pedrocli-llamacpp-output.txt"
	if _, err := os.Stat(debugFile); os.IsNotExist(err) {
		t.Errorf("Debug output file was not created at %s", debugFile)
	} else {
		t.Logf("Debug output saved to: %s", debugFile)
	}
}

// TestLlamaCppToolCalling tests tool call generation with model-specific formatting
// DEPRECATED: This test is for the old CLI-based llama.cpp client.
// The project has migrated to llama-server HTTP API with native tool calling.
func TestLlamaCppToolCalling(t *testing.T) {
	t.Skip("Skipping deprecated CLI-based llama.cpp test - migrated to llama-server HTTP API with native tool calling")

	llamaPath := os.Getenv("LLAMA_CPP_PATH")
	modelPath := os.Getenv("LLAMA_MODEL_PATH")

	if llamaPath == "" || modelPath == "" {
		t.Skip("Skipping llama.cpp tool calling test: LLAMA_CPP_PATH or LLAMA_MODEL_PATH not set")
	}

	// Create a test config
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:        "llamacpp",
			ServerURL:   "http://localhost:8082",
			ModelName:   "test-model",
			ContextSize: 4096,
			Temperature: 0.0,
			EnableTools: true,
		},
	}

	client := NewLlamaCppClient(cfg)

	// Create a tool calling request
	systemPrompt := `You are an autonomous coding agent.

# Tools

## search
Search for code using grep.

Example:
{"tool": "search", "args": {"pattern": "func main", "file_type": "go"}}

When you need to find code, use the search tool.`

	req := &InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   "Find all Go files that contain 'func main'",
		MaxTokens:    200,
		Temperature:  0.0,
	}

	// Run inference with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Tool calling inference failed: %v", err)
	}

	// Log the response for manual inspection
	t.Logf("LLM Response with tools:\n%s", resp.Text)

	// Check debug output
	debugFile := "/tmp/pedrocli-llamacpp-output.txt"
	if data, err := os.ReadFile(debugFile); err == nil {
		t.Logf("Full llama.cpp output:\n%s", string(data))
	}

	// Note: We're not asserting tool call parsing here because that's
	// handled by the executor. This test just verifies the LLM generates
	// something when given tool descriptions.
}
