// Package stt provides speech-to-text functionality using whisper.cpp
package stt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WhisperClient provides speech-to-text using whisper.cpp
type WhisperClient struct {
	binaryPath string // Path to whisper.cpp main binary
	modelPath  string // Path to whisper model file
	language   string // Language code (e.g., "en", "es")
	threads    int    // Number of threads to use
}

// NewWhisperClient creates a new WhisperClient
func NewWhisperClient(binaryPath, modelPath string) (*WhisperClient, error) {
	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper binary not found at %s", binaryPath)
	}

	// Verify model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found at %s", modelPath)
	}

	return &WhisperClient{
		binaryPath: binaryPath,
		modelPath:  modelPath,
		language:   "en",
		threads:    4,
	}, nil
}

// SetLanguage sets the language for transcription
func (w *WhisperClient) SetLanguage(lang string) {
	w.language = lang
}

// SetThreads sets the number of threads to use
func (w *WhisperClient) SetThreads(threads int) {
	w.threads = threads
}

// TranscribeAudio transcribes audio data and returns the text
func (w *WhisperClient) TranscribeAudio(ctx context.Context, audioData []byte) (string, error) {
	// Create temporary file for audio
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("pedrocli-audio-%d.wav", os.Getpid()))
	defer os.Remove(tempFile)

	// Write audio data to temp file
	if err := os.WriteFile(tempFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("failed to write audio file: %w", err)
	}

	// Run whisper.cpp
	return w.TranscribeFile(ctx, tempFile)
}

// TranscribeFile transcribes an audio file and returns the text
func (w *WhisperClient) TranscribeFile(ctx context.Context, audioPath string) (string, error) {
	// Verify file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return "", fmt.Errorf("audio file not found: %s", audioPath)
	}

	// Build command
	args := []string{
		"-m", w.modelPath,
		"-f", audioPath,
		"-l", w.language,
		"-t", fmt.Sprintf("%d", w.threads),
		"-nt",   // No timestamps
		"-otxt", // Output as text
	}

	cmd := exec.CommandContext(ctx, w.binaryPath, args...)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper.cpp failed: %w\nStderr: %s", err, stderr.String())
	}

	// Parse output - whisper.cpp writes to a .txt file
	outputFile := strings.TrimSuffix(audioPath, filepath.Ext(audioPath)) + ".txt"
	defer os.Remove(outputFile)

	text, err := os.ReadFile(outputFile)
	if err != nil {
		// If file doesn't exist, try to parse stdout
		output := stdout.String()
		if output != "" {
			return w.cleanTranscription(output), nil
		}
		return "", fmt.Errorf("failed to read transcription: %w", err)
	}

	return w.cleanTranscription(string(text)), nil
}

// TranscribeStream transcribes audio from a reader
func (w *WhisperClient) TranscribeStream(ctx context.Context, reader io.Reader) (string, error) {
	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read audio stream: %w", err)
	}

	return w.TranscribeAudio(ctx, data)
}

// cleanTranscription removes extra whitespace and formatting from whisper output
func (w *WhisperClient) cleanTranscription(text string) string {
	// Remove extra whitespace
	text = strings.TrimSpace(text)

	// Remove multiple spaces
	text = strings.Join(strings.Fields(text), " ")

	return text
}

// IsAvailable checks if whisper.cpp is available
func (w *WhisperClient) IsAvailable() bool {
	_, err := os.Stat(w.binaryPath)
	return err == nil
}

// GetModelInfo returns information about the loaded model
func (w *WhisperClient) GetModelInfo() map[string]interface{} {
	info := map[string]interface{}{
		"binary_path": w.binaryPath,
		"model_path":  w.modelPath,
		"language":    w.language,
		"threads":     w.threads,
	}

	// Check if model exists and get size
	if stat, err := os.Stat(w.modelPath); err == nil {
		info["model_size_mb"] = stat.Size() / (1024 * 1024)
	}

	return info
}
