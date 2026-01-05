package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServerClient_Infer_BasicResponse(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json")
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Verify request fields
		if reqBody["model"] != "test-model" {
			t.Errorf("Expected model 'test-model', got %v", reqBody["model"])
		}
		if reqBody["temperature"] != 0.2 {
			t.Errorf("Expected temperature 0.2, got %v", reqBody["temperature"])
		}

		// Send mock response
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "Hello, world!",
					},
				},
			},
			"usage": map[string]interface{}{
				"total_tokens": 100,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		EnableTools: false,
	})

	// Make inference request
	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Say hello",
		Temperature:  0.2,
		MaxTokens:    100,
	}

	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	// Verify response
	if resp.Text != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got '%s'", resp.Text)
	}
	if resp.TokensUsed != 100 {
		t.Errorf("Expected 100 tokens, got %d", resp.TokensUsed)
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("Expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestServerClient_Infer_WithToolCalls(t *testing.T) {
	// Create mock server that returns tool calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to verify tools are included
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Verify tools are in request
		if tools, ok := reqBody["tools"].([]interface{}); !ok || len(tools) == 0 {
			t.Errorf("Expected tools in request, got %v", reqBody["tools"])
		}

		// Send response with tool call
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"function": map[string]interface{}{
									"name":      "search",
									"arguments": `{"query":"test"}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]interface{}{
				"total_tokens": 50,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with tools enabled
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		EnableTools: true,
	})

	// Make inference request with tools
	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Search for test",
		Temperature:  0.2,
		MaxTokens:    100,
		Tools: []ToolDefinition{
			{
				Name:        "search",
				Description: "Search for information",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}

	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Infer failed: %v", err)
	}

	// Verify tool call
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search" {
		t.Errorf("Expected tool 'search', got '%s'", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Args["query"] != "test" {
		t.Errorf("Expected query 'test', got '%v'", resp.ToolCalls[0].Args["query"])
	}
}

func TestServerClient_ContextWindow(t *testing.T) {
	client := NewServerClient(ServerClientConfig{
		BaseURL:     "http://localhost:8081",
		ModelName:   "test-model",
		ContextSize: 32768,
	})

	if client.GetContextWindow() != 32768 {
		t.Errorf("Expected context window 32768, got %d", client.GetContextWindow())
	}

	// Usable context should be 75%
	expectedUsable := int(float64(32768) * 0.75)
	if client.GetUsableContext() != expectedUsable {
		t.Errorf("Expected usable context %d, got %d", expectedUsable, client.GetUsableContext())
	}
}

func TestServerClient_Timeout(t *testing.T) {
	// Create slow mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with short timeout
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		Timeout:     10 * time.Millisecond,
	})

	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "Test",
		UserPrompt:   "Test",
		Temperature:  0.2,
		MaxTokens:    100,
	}

	_, err := client.Infer(ctx, req)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestServerClient_ErrorResponse(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
	})

	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "Test",
		UserPrompt:   "Test",
		Temperature:  0.2,
		MaxTokens:    100,
	}

	_, err := client.Infer(ctx, req)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}
