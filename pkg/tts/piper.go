// Package tts provides text-to-speech functionality using Piper
package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PiperClient provides text-to-speech using Piper
type PiperClient struct {
	binaryPath string // Path to piper binary
	modelPath  string // Path to piper model (.onnx file)
	configPath string // Path to model config (.json file)
	sampleRate int    // Audio sample rate (default: 22050)
}

// NewPiperClient creates a new PiperClient
func NewPiperClient(binaryPath, modelPath string) (*PiperClient, error) {
	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("piper binary not found at %s", binaryPath)
	}

	// Verify model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("piper model not found at %s", modelPath)
	}

	// Look for config file (same name as model but .json)
	configPath := modelPath + ".json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("piper config not found at %s", configPath)
	}

	return &PiperClient{
		binaryPath: binaryPath,
		modelPath:  modelPath,
		configPath: configPath,
		sampleRate: 22050,
	}, nil
}

// SetSampleRate sets the audio sample rate
func (p *PiperClient) SetSampleRate(rate int) {
	p.sampleRate = rate
}

// Synthesize converts text to speech and returns WAV audio data
func (p *PiperClient) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Create temporary output file
	tempDir := os.TempDir()
	outputFile := filepath.Join(tempDir, fmt.Sprintf("pedrocli-tts-%d.wav", os.Getpid()))
	defer os.Remove(outputFile)

	// Build command
	// piper --model model.onnx --config model.onnx.json --output_file output.wav
	args := []string{
		"--model", p.modelPath,
		"--config", p.configPath,
		"--output_file", outputFile,
	}

	cmd := exec.CommandContext(ctx, p.binaryPath, args...)

	// Pass text via stdin
	cmd.Stdin = bytes.NewBufferString(text)

	// Capture errors
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Run command
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper failed: %w\nStderr: %s", err, stderr.String())
	}

	// Read generated audio file
	audioData, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio file: %w", err)
	}

	return audioData, nil
}

// SynthesizeToFile converts text to speech and saves to a file
func (p *PiperClient) SynthesizeToFile(ctx context.Context, text, outputPath string) error {
	audioData, err := p.Synthesize(ctx, text)
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, audioData, 0644)
}

// SynthesizeStreaming converts text to speech and streams audio data
// This is useful for real-time playback
func (p *PiperClient) SynthesizeStreaming(ctx context.Context, text string) (*bytes.Reader, error) {
	audioData, err := p.Synthesize(ctx, text)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(audioData), nil
}

// IsAvailable checks if Piper is available
func (p *PiperClient) IsAvailable() bool {
	_, err := os.Stat(p.binaryPath)
	return err == nil
}

// GetModelInfo returns information about the loaded model
func (p *PiperClient) GetModelInfo() map[string]interface{} {
	info := map[string]interface{}{
		"binary_path": p.binaryPath,
		"model_path":  p.modelPath,
		"config_path": p.configPath,
		"sample_rate": p.sampleRate,
	}

	// Check if model exists and get size
	if stat, err := os.Stat(p.modelPath); err == nil {
		info["model_size_mb"] = stat.Size() / (1024 * 1024)
	}

	return info
}

// SupportedVoices returns a list of common Piper voice models
// Users should download these from https://github.com/rhasspy/piper/releases
func SupportedVoices() []string {
	return []string{
		"en_US-lessac-medium",
		"en_US-libritts-high",
		"en_GB-alan-medium",
		"es_ES-sharvard-medium",
		"fr_FR-siwis-medium",
		"de_DE-thorsten-medium",
		"it_IT-riccardo-x_low",
		"pt_BR-faber-medium",
		"ru_RU-dmitri-medium",
		"zh_CN-huayan-medium",
	}
}
