package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// Segment represents a timestamped segment from whisper.cpp transcription.
type Segment struct {
	Start float64 `json:"t0"`
	End   float64 `json:"t1"`
	Text  string  `json:"text"`
}

// TranscriptResult holds the full transcription and individual segments.
type TranscriptResult struct {
	Text     string
	Segments []Segment
}

// WhisperClient is an HTTP client for a whisper.cpp server.
type WhisperClient struct {
	BaseURL string
	client  *http.Client
}

// NewWhisperClient creates a WhisperClient targeting the given base URL
// (e.g. "http://localhost:8081").
func NewWhisperClient(baseURL string) *WhisperClient {
	return &WhisperClient{
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Transcribe sends an audio file to the whisper.cpp server and returns
// the transcription with timestamped segments.
func (wc *WhisperClient) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptResult, error) {
	f, err := os.Open(audioFilePath)
	if err != nil {
		return nil, fmt.Errorf("open audio file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", audioFilePath)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy audio data: %w", err)
	}
	// Request JSON response with timestamps.
	_ = writer.WriteField("response_format", "verbose_json")
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wc.BaseURL+"/inference", &buf)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := wc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("whisper returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Text     string    `json:"text"`
		Segments []Segment `json:"segments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &TranscriptResult{
		Text:     result.Text,
		Segments: result.Segments,
	}, nil
}
