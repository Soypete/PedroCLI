package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWebSearchTool_AcceptsStatus202 tests that web_search handles 202 Accepted responses
func TestWebSearchTool_AcceptsStatus202(t *testing.T) {
	// Create a test server that returns 202
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
		w.Write([]byte(`{"results": [{"title": "Test", "url": "https://example.com"}]}`))
	}))
	defer server.Close()

	tool := NewWebSearchTool()

	// This currently fails but should succeed
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":       "test query",
		"max_results": 3,
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Currently this fails with "search returned status 202"
	// After fix, this should succeed
	if !result.Success {
		t.Errorf("Expected success with 202 status, got error: %s", result.Error)
	}
}

// TestWebSearchTool_AcceptsStatus200 tests normal 200 OK responses still work
func TestWebSearchTool_AcceptsStatus200(t *testing.T) {
	// Create a test server that returns 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 200
		w.Write([]byte(`{"results": [{"title": "Test", "url": "https://example.com"}]}`))
	}))
	defer server.Close()

	tool := NewWebSearchTool()

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":       "test query",
		"max_results": 3,
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success with 200 status, got error: %s", result.Error)
	}
}

// TestWebSearchTool_CalledByAgent simulates how an agent calls the tool
func TestWebSearchTool_CalledByAgent(t *testing.T) {
	tool := NewWebSearchTool()

	// Simulate agent tool call format
	args := map[string]interface{}{
		"query":       "kubernetes API design",
		"max_results": 3,
	}

	result, err := tool.Execute(context.Background(), args)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// May fail due to network/API issues, but should not panic
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	t.Logf("Result: success=%v, output_len=%d, error=%s",
		result.Success, len(result.Output), result.Error)
}
