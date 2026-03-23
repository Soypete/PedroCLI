// Package tts provides a TTS client and audio file storage.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TTSClient is an HTTP client for a llama.cpp instance running Qwen2.5-Omni TTS.
type TTSClient struct {
	BaseURL string
	Voice   string
	Speed   float64
	client  *http.Client
}

// NewTTSClient creates a TTSClient.
func NewTTSClient(baseURL, voice string, speed float64) *TTSClient {
	return &TTSClient{
		BaseURL: baseURL,
		Voice:   voice,
		Speed:   speed,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Synthesize sends text to the TTS server and returns raw MP3 audio bytes.
func (tc *TTSClient) Synthesize(ctx context.Context, text string) ([]byte, error) {
	body := map[string]any{
		"model": "qwen-tts",
		"input": text,
		"voice": tc.Voice,
		"speed": tc.Speed,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tc.BaseURL+"/v1/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts returned %d: %s", resp.StatusCode, b)
	}

	return io.ReadAll(resp.Body)
}
