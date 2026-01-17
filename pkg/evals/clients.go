package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ModelClient is the interface for LLM backends used in evaluations.
type ModelClient interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	// StreamComplete sends a streaming completion request.
	StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
	// ListModels returns available models on the server.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// Provider returns the provider name ("ollama" or "llama_cpp").
	Provider() string
}

// CompletionRequest represents a completion request to an LLM.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse represents a completion response from an LLM.
type CompletionResponse struct {
	Content          string        `json:"content"`
	Model            string        `json:"model"`
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	FinishReason     string        `json:"finish_reason"`
	TimeToFirstToken time.Duration `json:"time_to_first_token"`
	TotalTime        time.Duration `json:"total_time"`
}

// StreamChunk represents a chunk from a streaming response.
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"error,omitempty"`
}

// ModelInfo contains information about a model.
type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

// OllamaClient implements ModelClient for Ollama servers.
type OllamaClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewOllamaClient creates a new Ollama client.
func NewOllamaClient(endpoint string) *OllamaClient {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	return &OllamaClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // Long timeout for inference
		},
	}
}

func (c *OllamaClient) Provider() string {
	return "ollama"
}

// Complete implements ModelClient.Complete for Ollama.
func (c *OllamaClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	startTime := time.Now()

	// Convert to Ollama chat format
	ollamaReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
		"options":  map[string]interface{}{},
	}

	if req.Temperature > 0 {
		ollamaReq["options"].(map[string]interface{})["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		ollamaReq["options"].(map[string]interface{})["num_predict"] = req.MaxTokens
	}
	if req.TopP > 0 {
		ollamaReq["options"].(map[string]interface{})["top_p"] = req.TopP
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Model           string `json:"model"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
		Done            bool   `json:"done"`
		DoneReason      string `json:"done_reason"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	totalTime := time.Since(startTime)

	return &CompletionResponse{
		Content:          ollamaResp.Message.Content,
		Model:            ollamaResp.Model,
		PromptTokens:     ollamaResp.PromptEvalCount,
		CompletionTokens: ollamaResp.EvalCount,
		TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		FinishReason:     ollamaResp.DoneReason,
		TotalTime:        totalTime,
	}, nil
}

// StreamComplete implements ModelClient.StreamComplete for Ollama.
func (c *OllamaClient) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	ollamaReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
		"options":  map[string]interface{}{},
	}

	if req.Temperature > 0 {
		ollamaReq["options"].(map[string]interface{})["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		ollamaReq["options"].(map[string]interface{})["num_predict"] = req.MaxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	go func() {
		defer close(ch)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := decoder.Decode(&chunk); err != nil {
				if err != io.EOF {
					ch <- StreamChunk{Error: err}
				}
				return
			}
			ch <- StreamChunk{
				Content: chunk.Message.Content,
				Done:    chunk.Done,
			}
			if chunk.Done {
				return
			}
		}
	}()

	return ch, nil
}

// ListModels implements ModelClient.ListModels for Ollama.
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, len(ollamaResp.Models))
	for i, m := range ollamaResp.Models {
		models[i] = ModelInfo{
			Name:       m.Name,
			Size:       m.Size,
			ModifiedAt: m.ModifiedAt,
		}
	}

	return models, nil
}

// LlamaCppClient implements ModelClient for llama.cpp servers (OpenAI-compatible).
type LlamaCppClient struct {
	endpoint   string
	httpClient *http.Client
	modelName  string // llama.cpp doesn't always return model name
}

// NewLlamaCppClient creates a new llama.cpp client.
func NewLlamaCppClient(endpoint string, modelName string) *LlamaCppClient {
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}
	return &LlamaCppClient{
		endpoint:  endpoint,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

func (c *LlamaCppClient) Provider() string {
	return "llama_cpp"
}

// Complete implements ModelClient.Complete for llama.cpp.
func (c *LlamaCppClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	startTime := time.Now()

	// llama.cpp uses OpenAI-compatible API
	openaiReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false,
	}

	if req.Temperature > 0 {
		openaiReq["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		openaiReq["max_tokens"] = req.MaxTokens
	}
	if req.TopP > 0 {
		openaiReq["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		openaiReq["stop"] = req.Stop
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llama.cpp error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	totalTime := time.Since(startTime)

	content := ""
	finishReason := ""
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		finishReason = openaiResp.Choices[0].FinishReason
	}

	model := openaiResp.Model
	if model == "" {
		model = c.modelName
	}

	return &CompletionResponse{
		Content:          content,
		Model:            model,
		PromptTokens:     openaiResp.Usage.PromptTokens,
		CompletionTokens: openaiResp.Usage.CompletionTokens,
		TotalTokens:      openaiResp.Usage.TotalTokens,
		FinishReason:     finishReason,
		TotalTime:        totalTime,
	}, nil
}

// StreamComplete implements ModelClient.StreamComplete for llama.cpp.
func (c *LlamaCppClient) StreamComplete(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 100)

	openaiReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	if req.Temperature > 0 {
		openaiReq["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		openaiReq["max_tokens"] = req.MaxTokens
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	go func() {
		defer close(ch)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}
		defer resp.Body.Close()

		// Parse SSE stream
		decoder := json.NewDecoder(resp.Body)
		for {
			// Read line by line for SSE format
			var line []byte
			buf := make([]byte, 1)
			for {
				n, err := resp.Body.Read(buf)
				if err != nil {
					if err != io.EOF {
						ch <- StreamChunk{Error: err}
					}
					return
				}
				if n == 0 {
					continue
				}
				if buf[0] == '\n' {
					break
				}
				line = append(line, buf[0])
			}

			lineStr := string(line)
			if lineStr == "" || lineStr == "\r" {
				continue
			}
			if lineStr == "data: [DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}
			if len(lineStr) > 6 && lineStr[:6] == "data: " {
				jsonData := lineStr[6:]
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
				}
				if err := json.Unmarshal([]byte(jsonData), &chunk); err != nil {
					// Skip malformed chunks
					_ = decoder // silence unused warning
					continue
				}
				if len(chunk.Choices) > 0 {
					ch <- StreamChunk{
						Content: chunk.Choices[0].Delta.Content,
						Done:    chunk.Choices[0].FinishReason == "stop",
					}
				}
			}
		}
	}()

	return ch, nil
}

// ListModels implements ModelClient.ListModels for llama.cpp.
func (c *LlamaCppClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// llama.cpp might not support /v1/models, return default
		if c.modelName != "" {
			return []ModelInfo{{Name: c.modelName}}, nil
		}
		return []ModelInfo{{Name: "default"}}, nil
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		// Return configured model if parse fails
		if c.modelName != "" {
			return []ModelInfo{{Name: c.modelName}}, nil
		}
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ModelInfo, len(modelsResp.Data))
	for i, m := range modelsResp.Data {
		models[i] = ModelInfo{Name: m.ID}
	}

	return models, nil
}

// NewClient creates a ModelClient based on provider type.
func NewClient(provider, endpoint, modelName string) (ModelClient, error) {
	switch provider {
	case "ollama":
		return NewOllamaClient(endpoint), nil
	case "llama_cpp", "llamacpp":
		return NewLlamaCppClient(endpoint, modelName), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: ollama, llama_cpp)", provider)
	}
}
