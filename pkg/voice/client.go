package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client represents a whisper.cpp HTTP client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new whisper.cpp client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Transcription can take time
		},
	}
}

// Transcribe sends audio to whisper.cpp for transcription
func (c *Client) Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error) {
	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio file
	part, err := writer.CreateFormFile("file", "audio."+req.Format)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(req.Audio); err != nil {
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// Add optional fields
	if req.Language != "" && req.Language != "auto" {
		writer.WriteField("language", req.Language)
	}

	if req.Prompt != "" {
		writer.WriteField("prompt", req.Prompt)
	}

	// Close multipart writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + "/inference"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &TranscribeResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to send request: %v", err),
		}, err
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TranscribeResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}, err
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return &TranscribeResponse{
			Success: false,
			Error:   fmt.Sprintf("whisper.cpp returned status %d: %s", resp.StatusCode, string(respBody)),
		}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	// whisper.cpp returns: {"text": "transcribed text"}
	var whisperResp struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal(respBody, &whisperResp); err != nil {
		return &TranscribeResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v", err),
		}, err
	}

	processingTime := time.Since(startTime).Milliseconds()

	return &TranscribeResponse{
		Text:             whisperResp.Text,
		ProcessingTimeMs: processingTime,
		Success:          true,
	}, nil
}

// Status checks if whisper.cpp server is running
func (c *Client) Status(ctx context.Context) (*StatusResponse, error) {
	// Create HTTP request
	url := c.baseURL + "/health"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &StatusResponse{
			Running: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, err
	}

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &StatusResponse{
			Running: false,
			Error:   fmt.Sprintf("failed to connect: %v", err),
		}, err
	}
	defer resp.Body.Close()

	// If we got a response, server is running
	if resp.StatusCode == http.StatusOK {
		return &StatusResponse{
			Running: true,
		}, nil
	}

	return &StatusResponse{
		Running: false,
		Error:   fmt.Sprintf("unexpected status: %d", resp.StatusCode),
	}, nil
}
