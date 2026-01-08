package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ServerClient implements Backend for OpenAI-compatible HTTP APIs
// Works with llama-server, ollama, vllm, lmstudio, etc.
type ServerClient struct {
	baseURL     string
	modelName   string
	contextSize int
	usableSize  int
	enableTools bool
	httpClient  *http.Client
	apiPath     string // e.g., "/v1/chat/completions" or "/api/generate"
}

// ServerClientConfig configures the HTTP server client
type ServerClientConfig struct {
	BaseURL     string
	ModelName   string
	ContextSize int
	EnableTools bool
	APIPath     string        // Optional, defaults to "/v1/chat/completions"
	Timeout     time.Duration // Optional, defaults to 5min
}

// NewServerClient creates a new HTTP server client
func NewServerClient(cfg ServerClientConfig) *ServerClient {
	if cfg.APIPath == "" {
		cfg.APIPath = "/v1/chat/completions" // OpenAI-compatible default
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}

	usableSize := cfg.ContextSize
	if usableSize > 0 {
		usableSize = int(float64(cfg.ContextSize) * 0.75)
	}

	return &ServerClient{
		baseURL:     cfg.BaseURL,
		modelName:   cfg.ModelName,
		contextSize: cfg.ContextSize,
		usableSize:  usableSize,
		enableTools: cfg.EnableTools,
		apiPath:     cfg.APIPath,
		httpClient:  &http.Client{Timeout: cfg.Timeout},
	}
}

// Infer performs inference using OpenAI-compatible chat completions API
func (c *ServerClient) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	// Build chat messages
	messages := []map[string]string{
		{"role": "system", "content": req.SystemPrompt},
		{"role": "user", "content": req.UserPrompt},
	}

	// Build request body
	reqBody := map[string]interface{}{
		"model":       c.modelName,
		"messages":    messages,
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      false,
	}

	// Add tools if enabled (native tool calling)
	if c.enableTools && len(req.Tools) > 0 {
		reqBody["tools"] = c.formatTools(req.Tools)
	}

	// Add logit bias if provided
	// Note: Ollama's /v1/chat/completions may not support logit_bias
	// For full logit bias support with Ollama, use /api/generate endpoint
	if len(req.LogitBias) > 0 {
		// Convert map[int]float32 to map[string]float32 for JSON
		logitBiasJSON := make(map[string]interface{})
		for tokenID, bias := range req.LogitBias {
			logitBiasJSON[fmt.Sprintf("%d", tokenID)] = bias
		}
		reqBody["logit_bias"] = logitBiasJSON
	}

	// Add grammar constraint if provided (llama-server specific)
	if req.Grammar != "" {
		reqBody["grammar"] = req.Grammar
	}

	// Marshal to JSON
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+c.apiPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Extract message
	message := chatResp.Choices[0].Message

	// Parse tool calls from API response
	var toolCalls []ToolCall
	for _, tc := range message.ToolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			continue
		}
		toolCalls = append(toolCalls, ToolCall{
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return &InferenceResponse{
		Text:       message.Content,
		ToolCalls:  toolCalls,
		NextAction: "CONTINUE",
		TokensUsed: chatResp.Usage.TotalTokens,
	}, nil
}

// formatTools converts tool definitions to OpenAI format
func (c *ServerClient) formatTools(tools []ToolDefinition) []map[string]interface{} {
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		}
	}
	return result
}

// GetContextWindow returns the context window size
func (c *ServerClient) GetContextWindow() int {
	return c.contextSize
}

// GetUsableContext returns the usable context size (75%)
func (c *ServerClient) GetUsableContext() int {
	return c.usableSize
}

// Close closes the HTTP client
func (c *ServerClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
