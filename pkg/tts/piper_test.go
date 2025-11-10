package tts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewPiperClient(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  func(tmpDir string) (binaryPath, modelPath string)
		wantErr     bool
		errContains string
	}{
		{
			name: "missing binary",
			setupFiles: func(tmpDir string) (string, string) {
				modelPath := filepath.Join(tmpDir, "model.onnx")
				configPath := modelPath + ".json"
				os.WriteFile(modelPath, []byte("test"), 0644)
				os.WriteFile(configPath, []byte("{}"), 0644)
				return "/nonexistent/piper", modelPath
			},
			wantErr:     true,
			errContains: "piper binary not found",
		},
		{
			name: "missing model",
			setupFiles: func(tmpDir string) (string, string) {
				binaryPath := filepath.Join(tmpDir, "piper")
				os.WriteFile(binaryPath, []byte("test"), 0755)
				return binaryPath, "/nonexistent/model.onnx"
			},
			wantErr:     true,
			errContains: "piper model not found",
		},
		{
			name: "missing config",
			setupFiles: func(tmpDir string) (string, string) {
				binaryPath := filepath.Join(tmpDir, "piper")
				modelPath := filepath.Join(tmpDir, "model.onnx")
				os.WriteFile(binaryPath, []byte("test"), 0755)
				os.WriteFile(modelPath, []byte("test"), 0644)
				// Don't create config file
				return binaryPath, modelPath
			},
			wantErr:     true,
			errContains: "piper config not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			binaryPath, modelPath := tt.setupFiles(tmpDir)

			client, err := NewPiperClient(binaryPath, modelPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPiperClient() expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewPiperClient() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("NewPiperClient() unexpected error = %v", err)
					return
				}
				if client == nil {
					t.Error("NewPiperClient() returned nil client")
				}
			}
		})
	}
}

func TestNewPiperClient_ValidPaths(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "piper")
	modelPath := filepath.Join(tmpDir, "model.onnx")
	configPath := modelPath + ".json"

	// Create fake files
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("fake model"), 0644); err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	client, err := NewPiperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("NewPiperClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewPiperClient() returned nil client")
	}

	// Verify default values
	if client.sampleRate != 22050 {
		t.Errorf("Expected default sample rate 22050, got %d", client.sampleRate)
	}
	if client.binaryPath != binaryPath {
		t.Errorf("Expected binaryPath %q, got %q", binaryPath, client.binaryPath)
	}
	if client.modelPath != modelPath {
		t.Errorf("Expected modelPath %q, got %q", modelPath, client.modelPath)
	}
	if client.configPath != configPath {
		t.Errorf("Expected configPath %q, got %q", configPath, client.configPath)
	}
}

func TestPiperClient_SetSampleRate(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "piper")
	modelPath := filepath.Join(tmpDir, "model.onnx")
	configPath := modelPath + ".json"

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)
	os.WriteFile(configPath, []byte("{}"), 0644)

	client, err := NewPiperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	client.SetSampleRate(44100)
	if client.sampleRate != 44100 {
		t.Errorf("SetSampleRate() failed, got %d, want 44100", client.sampleRate)
	}
}

func TestPiperClient_IsAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "piper")
	modelPath := filepath.Join(tmpDir, "model.onnx")
	configPath := modelPath + ".json"

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)
	os.WriteFile(configPath, []byte("{}"), 0644)

	client, err := NewPiperClient(binaryPath, modelPath)
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

func TestPiperClient_GetModelInfo(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "piper")
	modelPath := filepath.Join(tmpDir, "model.onnx")
	configPath := modelPath + ".json"

	testData := []byte("test model data with some content")
	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, testData, 0644)
	os.WriteFile(configPath, []byte("{}"), 0644)

	client, err := NewPiperClient(binaryPath, modelPath)
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
	if info["config_path"] != configPath {
		t.Errorf("GetModelInfo() config_path = %v, want %v", info["config_path"], configPath)
	}
	if info["sample_rate"] != 22050 {
		t.Errorf("GetModelInfo() sample_rate = %v, want 22050", info["sample_rate"])
	}

	// Check model size
	expectedSizeMB := int64(len(testData)) / (1024 * 1024)
	if sizeMB, ok := info["model_size_mb"].(int64); ok {
		if sizeMB != expectedSizeMB {
			t.Errorf("GetModelInfo() model_size_mb = %v, want %v", sizeMB, expectedSizeMB)
		}
	}
}

func TestPiperClient_Synthesize_EmptyText(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "piper")
	modelPath := filepath.Join(tmpDir, "model.onnx")
	configPath := modelPath + ".json"

	os.WriteFile(binaryPath, []byte("test"), 0755)
	os.WriteFile(modelPath, []byte("test"), 0644)
	os.WriteFile(configPath, []byte("{}"), 0644)

	client, err := NewPiperClient(binaryPath, modelPath)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.Synthesize(ctx, "")
	if err == nil {
		t.Error("Synthesize() expected error for empty text, got nil")
	}
	if !contains(err.Error(), "cannot be empty") {
		t.Errorf("Synthesize() error = %v, want error containing 'cannot be empty'", err)
	}
}

func TestSupportedVoices(t *testing.T) {
	voices := SupportedVoices()

	if len(voices) == 0 {
		t.Error("SupportedVoices() returned empty list")
	}

	// Check that common voices are included
	expectedVoices := []string{
		"en_US-lessac-medium",
		"en_US-libritts-high",
		"en_GB-alan-medium",
	}

	for _, expected := range expectedVoices {
		found := false
		for _, voice := range voices {
			if voice == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SupportedVoices() missing expected voice %q", expected)
		}
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
