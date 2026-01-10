package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestServerClient_RetryOnTimeout(t *testing.T) {
	var attempts atomic.Int32

	// Create server that fails first 2 times, succeeds on 3rd
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			// Simulate timeout by sleeping longer than client timeout
			time.Sleep(200 * time.Millisecond)
			return
		}
		// Success on 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"success"}}],"usage":{"total_tokens":10}}`))
	}))
	defer server.Close()

	// Create client with short timeout and 3 retries
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		Timeout:     100 * time.Millisecond,
		MaxRetries:  3,
	})

	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "test",
		UserPrompt:   "test",
		Temperature:  0.7,
		MaxTokens:    100,
	}

	// Should succeed after retries
	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if resp.Text != "success" {
		t.Errorf("Expected 'success', got: %s", resp.Text)
	}

	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts.Load())
	}
}

func TestServerClient_RetryOn5xxError(t *testing.T) {
	var attempts atomic.Int32

	// Create server that returns 500 first 2 times, succeeds on 3rd
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
			return
		}
		// Success on 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"success"}}],"usage":{"total_tokens":10}}`))
	}))
	defer server.Close()

	// Create client with retries
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		MaxRetries:  3,
	})

	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "test",
		UserPrompt:   "test",
		Temperature:  0.7,
		MaxTokens:    100,
	}

	// Should succeed after retries
	resp, err := client.Infer(ctx, req)
	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	if resp.Text != "success" {
		t.Errorf("Expected 'success', got: %s", resp.Text)
	}

	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts, got: %d", attempts.Load())
	}
}

func TestServerClient_NoRetryOn4xxError(t *testing.T) {
	var attempts atomic.Int32

	// Create server that always returns 400
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	// Create client with retries
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		MaxRetries:  3,
	})

	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "test",
		UserPrompt:   "test",
		Temperature:  0.7,
		MaxTokens:    100,
	}

	// Should fail immediately without retries
	_, err := client.Infer(ctx, req)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Should only try once (no retries for 4xx)
	if attempts.Load() != 1 {
		t.Errorf("Expected 1 attempt (no retries for 4xx), got: %d", attempts.Load())
	}
}

func TestServerClient_ExhaustedRetries(t *testing.T) {
	var attempts atomic.Int32

	// Create server that always times out
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	// Create client with short timeout and 2 retries
	client := NewServerClient(ServerClientConfig{
		BaseURL:     server.URL,
		ModelName:   "test-model",
		ContextSize: 4096,
		Timeout:     100 * time.Millisecond,
		MaxRetries:  2,
	})

	ctx := context.Background()
	req := &InferenceRequest{
		SystemPrompt: "test",
		UserPrompt:   "test",
		Temperature:  0.7,
		MaxTokens:    100,
	}

	// Should fail after exhausting retries
	_, err := client.Infer(ctx, req)
	if err == nil {
		t.Fatal("Expected error after exhausting retries, got nil")
	}

	// Should try 3 times total (initial + 2 retries)
	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts (initial + 2 retries), got: %d", attempts.Load())
	}
}
