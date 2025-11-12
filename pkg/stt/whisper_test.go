package stt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewWhisperClient(t *testing.T) {
	tests := []struct {
		name        string
		binaryPath  string
		modelPath   string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing binary",
			binaryPath:  "/nonexistent/whisper",
			modelPath:   "/tmp/model.bin",
			wantErr:     true,
			errContains: "whisper binary not found",
		},
		{
			name:        "missing model",
			binaryPath:  "/bin/echo", // Use echo as a fake binary
			modelPath:   "/nonexistent/model.bin",
			wantErr:     true,
			errContains: "whisper model not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWhisperClient(tt.binaryPath, tt.modelPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewWhisperClient() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewWhisperClient() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("NewWhisperClient() unexpected error = %v", err)
					return
				}
				if client == nil {
					t.Error("NewWhisperClient() returned nil client")
				}
			}
		})
	}
}

func TestNewWhisperClient_ValidPaths(t *testing.T) {
	// Create temporary binary and model files
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	// Create fake files
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("fake model"), 0644); err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("NewWhisperClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewWhisperClient() returned nil client")
	}

	// Verify default values
	if client.language != "en" {
		t.Errorf("Expected default language 'en', got %q", client.language)
	}
	if client.threads != 4 {
		t.Errorf("Expected default threads 4, got %d", client.threads)
	}
}

func TestWhisperClient_SetLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	client.SetLanguage("es")
	if client.language != "es" {
		t.Errorf("SetLanguage() failed, got %q, want 'es'", client.language)
	}
}

func TestWhisperClient_SetThreads(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	client.SetThreads(8)
	if client.threads != 8 {
		t.Errorf("SetThreads() failed, got %d, want 8", client.threads)
	}
}

func TestWhisperClient_IsAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if !client.IsAvailable() {
		t.Error("IsAvailable() = false, want true when binary exists")
	}

	// Test with non-existent binary
	client.binaryPath = "/nonexistent/path"
	if client.IsAvailable() {
		t.Error("IsAvailable() = true, want false when binary doesn't exist")
	}
}

func TestWhisperClient_GetModelInfo(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	testData := []byte("test model data")
	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, testData, 0644)

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	info := client.GetModelInfo()

	if info["binary_path"] != binaryPath {
		t.Errorf("GetModelInfo() binary_path = %v, want %v", info["binary_path"], binaryPath)
	}
	if info["model_path"] != modelPath {
		t.Errorf("GetModelInfo() model_path = %v, want %v", info["model_path"], modelPath)
	}
	if info["language"] != "en" {
		t.Errorf("GetModelInfo() language = %v, want 'en'", info["language"])
	}
	if info["threads"] != 4 {
		t.Errorf("GetModelInfo() threads = %v, want 4", info["threads"])
	}

	// Check model size
	expectedSizeMB := int64(len(testData)) / (1024 * 1024)
	if sizeMB, ok := info["model_size_mb"].(int64); ok {
		if sizeMB != expectedSizeMB {
			t.Errorf("GetModelInfo() model_size_mb = %v, want %v", sizeMB, expectedSizeMB)
		}
	}
}

func TestWhisperClient_TranscribeFile_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.TranscribeFile(ctx, "/nonexistent/audio.wav")
	if err == nil {
		t.Error("TranscribeFile() expected error for non-existent file, got nil")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("TranscribeFile() error = %v, want error containing 'not found'", err)
	}
}

func TestWhisperClient_CleanTranscription(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "whisper")
	modelPath := filepath.Join(tmpDir, "model.bin")

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)

	client, err := NewWhisperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "extra whitespace",
			input: "  hello   world  ",
			want:  "hello world",
		},
		{
			name:  "multiple spaces",
			input: "hello     world",
			want:  "hello world",
		},
		{
			name:  "newlines and tabs",
			input: "hello\nworld\t\ttest",
			want:  "hello world test",
		},
		{
			name:  "already clean",
			input: "hello world",
			want:  "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.cleanTranscription(tt.input)
			if got != tt.want {
				t.Errorf("cleanTranscription() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || anyIndex(s, substr) >= 0)
}

func anyIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
