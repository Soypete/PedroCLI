package evals

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		provider  string
		expectErr bool
	}{
		{"ollama", false},
		{"llama_cpp", false},
		{"llamacpp", false},
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			client, err := NewClient(tt.provider, "http://localhost:8080", "test-model")
			if tt.expectErr {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected non-nil client")
				}
			}
		})
	}
}

func TestOllamaClient_Complete(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify request body
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req["model"] != "test-model" {
			t.Errorf("unexpected model: %v", req["model"])
		}

		// Send response
		resp := map[string]interface{}{
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Hello! I'm a test response.",
			},
			"model":             "test-model",
			"prompt_eval_count": 10,
			"eval_count":        20,
			"done":              true,
			"done_reason":       "stop",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, &CompletionRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: 0.7,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello! I'm a test response." {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.Model != "test-model" {
		t.Errorf("unexpected model: %s", resp.Model)
	}
	if resp.PromptTokens != 10 {
		t.Errorf("unexpected prompt tokens: %d", resp.PromptTokens)
	}
	if resp.CompletionTokens != 20 {
		t.Errorf("unexpected completion tokens: %d", resp.CompletionTokens)
	}
}

func TestOllamaClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "llama3:8b", "size": 4000000000},
				{"name": "codellama:13b", "size": 7000000000},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
	if models[0].Name != "llama3:8b" {
		t.Errorf("unexpected model name: %s", models[0].Name)
	}
}

func TestOllamaClient_Provider(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434")
	if client.Provider() != "ollama" {
		t.Errorf("unexpected provider: %s", client.Provider())
	}
}

func TestLlamaCppClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"id":    "test-id",
			"model": "test-model",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test response from llama.cpp",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     15,
				"completion_tokens": 25,
				"total_tokens":      40,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewLlamaCppClient(server.URL, "test-model")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Complete(ctx, &CompletionRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Test response from llama.cpp" {
		t.Errorf("unexpected content: %s", resp.Content)
	}
	if resp.TotalTokens != 40 {
		t.Errorf("unexpected total tokens: %d", resp.TotalTokens)
	}
}

func TestLlamaCppClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "model-1"},
				{"id": "model-2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewLlamaCppClient(server.URL, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestLlamaCppClient_ListModels_Fallback(t *testing.T) {
	// Server returns 404 (doesn't support /v1/models)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewLlamaCppClient(server.URL, "configured-model")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return configured model as fallback
	if len(models) != 1 {
		t.Errorf("expected 1 model (fallback), got %d", len(models))
	}
	if models[0].Name != "configured-model" {
		t.Errorf("expected configured model name, got %s", models[0].Name)
	}
}

func TestLlamaCppClient_Provider(t *testing.T) {
	client := NewLlamaCppClient("http://localhost:8080", "model")
	if client.Provider() != "llama_cpp" {
		t.Errorf("unexpected provider: %s", client.Provider())
	}
}

func TestOllamaClient_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Complete(ctx, &CompletionRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "test"}},
	})

	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestLlamaCppClient_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewLlamaCppClient(server.URL, "model")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Complete(ctx, &CompletionRequest{
		Model:    "test",
		Messages: []Message{{Role: "user", Content: "test"}},
	})

	if err == nil {
		t.Error("expected error for 400 response")
	}
}

func TestOllamaClient_DefaultEndpoint(t *testing.T) {
	client := NewOllamaClient("")
	if client.endpoint != "http://localhost:11434" {
		t.Errorf("unexpected default endpoint: %s", client.endpoint)
	}
}

func TestLlamaCppClient_DefaultEndpoint(t *testing.T) {
	client := NewLlamaCppClient("", "model")
	if client.endpoint != "http://localhost:8080" {
		t.Errorf("unexpected default endpoint: %s", client.endpoint)
	}
}
