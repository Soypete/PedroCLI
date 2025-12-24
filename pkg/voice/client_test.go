package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Status_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("Expected path /health, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Check status
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !status.Running {
		t.Error("Expected status.Running to be true")
	}
}

func TestClient_Status_Failure(t *testing.T) {
	// Create mock server that's down
	client := NewClient("http://localhost:99999") // Invalid port

	// Check status
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if status.Running {
		t.Error("Expected status.Running to be false")
	}

	if status.Error == "" {
		t.Error("Expected status.Error to be set")
	}
}

func TestClient_Transcribe_Success(t *testing.T) {
	// Create mock whisper.cpp server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/inference" {
			t.Errorf("Expected path /inference, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("Failed to parse multipart form: %v", err)
		}

		// Check file exists
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("Expected file in form, got error: %v", err)
		}
		file.Close()

		// Return mock transcription
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"text": "Hello world",
		})
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Transcribe
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := TranscribeRequest{
		Audio:    []byte("fake audio data"),
		Format:   "wav",
		Language: "en",
	}

	resp, err := client.Transcribe(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}

	if resp.Text != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", resp.Text)
	}

	// Processing time should be >= 0 (could be 0 for very fast operations)
	if resp.ProcessingTimeMs < 0 {
		t.Error("Expected processing time to be non-negative")
	}
}

func TestClient_Transcribe_ServerError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL)

	// Transcribe
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := TranscribeRequest{
		Audio:  []byte("fake audio data"),
		Format: "wav",
	}

	resp, err := client.Transcribe(ctx, req)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if resp.Success {
		t.Error("Expected failure")
	}

	if resp.Error == "" {
		t.Error("Expected error message to be set")
	}
}

func TestTranscribeRequest_Validation(t *testing.T) {
	req := TranscribeRequest{
		Audio:    []byte("test"),
		Format:   "wav",
		Language: "en",
		Prompt:   "test prompt",
	}

	if len(req.Audio) == 0 {
		t.Error("Expected audio to be set")
	}

	if req.Format != "wav" {
		t.Errorf("Expected format 'wav', got '%s'", req.Format)
	}
}

func TestTranscribeResponse_Success(t *testing.T) {
	resp := TranscribeResponse{
		Text:             "transcribed text",
		Language:         "en",
		ProcessingTimeMs: 100,
		Confidence:       0.95,
		Success:          true,
	}

	if !resp.Success {
		t.Error("Expected success to be true")
	}

	if resp.Text == "" {
		t.Error("Expected text to be set")
	}
}
