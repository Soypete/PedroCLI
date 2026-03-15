// Package artifacts provides LLM-based artifact generation for study materials.
package artifacts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Usage tracks token consumption from a completion.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// LlamaClient is an OpenAI-compatible HTTP client for a llama.cpp server.
type LlamaClient struct {
	BaseURL string
	Model   string
	client  *http.Client
}

// NewLlamaClient creates a LlamaClient.
func NewLlamaClient(baseURL, model string) *LlamaClient {
	return &LlamaClient{
		BaseURL: baseURL,
		Model:   model,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

// Complete sends a chat completion request and returns the response text and usage.
// Appends /no_think to the system prompt to disable chain-of-thought for artifact generation.
func (lc *LlamaClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, *Usage, error) {
	body := map[string]any{
		"model": lc.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt + "\n/no_think"},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, lc.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := lc.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("llama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("llama returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", nil, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices in response")
	}
	return result.Choices[0].Message.Content, &result.Usage, nil
}
