package voice

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ConvertToWav converts audio from any format to WAV using ffmpeg
// This is needed because whisper.cpp only accepts WAV format
func ConvertToWav(ctx context.Context, audioData []byte, inputFormat string) ([]byte, error) {
	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w (install with: brew install ffmpeg)", err)
	}

	// Create temp directory for conversion
	tmpDir, err := os.MkdirTemp("", "voice-convert-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write input file
	inputPath := filepath.Join(tmpDir, "input."+inputFormat)
	if err := os.WriteFile(inputPath, audioData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}

	// Output path
	outputPath := filepath.Join(tmpDir, "output.wav")

	// Run ffmpeg to convert to wav
	// -y: overwrite output
	// -i: input file
	// -ar 16000: sample rate (whisper expects 16kHz)
	// -ac 1: mono channel
	// -c:a pcm_s16le: 16-bit PCM encoding
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", inputPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		outputPath,
	)

	// Capture stderr for error messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg conversion failed: %w\nstderr: %s", err, stderr.String())
	}

	// Read converted file
	wavData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted file: %w", err)
	}

	return wavData, nil
}

// NeedsConversion checks if the audio format needs conversion to wav
func NeedsConversion(format string) bool {
	switch format {
	case "wav", "wave":
		return false
	default:
		// webm, mp3, ogg, etc. all need conversion
		return true
	}
}
