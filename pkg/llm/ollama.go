package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// OllamaClient implements the Backend interface for Ollama
type OllamaClient struct {
	baseURL     string
	modelName   string
	temperature float64
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(cfg *config.Config) *OllamaClient {
	baseURL := cfg.Model.OllamaURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaClient{
		baseURL:     baseURL,
		modelName:   cfg.Model.ModelName,
		temperature: cfg.Model.Temperature,
	}
}

// Infer performs one-shot inference using Ollama
func (o *OllamaClient) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	// Build the full prompt
	fullPrompt := o.buildPrompt(req)

	// Build Ollama API request
	ollamaReq := map[string]interface{}{
		"model":  o.modelName,
		"prompt": fullPrompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": req.Temperature,
		},
	}

	// Marshal request
	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp struct {
		Model           string `json:"model"`
		Response        string `json:"response"`
		Done            bool   `json:"done"`
		Context         []int  `json:"context"`
		TotalDuration   int64  `json:"total_duration"`
		LoadDuration    int64  `json:"load_duration"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Build response
	response := &InferenceResponse{
		Text:       strings.TrimSpace(ollamaResp.Response),
		ToolCalls:  []ToolCall{}, // TODO: Parse tool calls from response
		NextAction: "COMPLETE",   // TODO: Determine based on response
		TokensUsed: ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}

	return response, nil
}

// GetContextWindow returns the context window size for the model
func (o *OllamaClient) GetContextWindow() int {
	// Known model context windows
	modelContexts := map[string]int{
		"qwen2.5-coder:7b":   32768,
		"qwen2.5-coder:14b":  32768,
		"qwen2.5-coder:32b":  32768,
		"qwen2.5-coder:72b":  131072,
		"deepseek-coder:33b": 16384,
		"codellama:7b":       16384,
		"codellama:13b":      16384,
		"codellama:34b":      16384,
		"llama3.1:8b":        131072,
		"llama3.1:70b":       131072,
		"llama3.1:405b":      131072,
		"llama3.2:1b":        131072,
		"llama3.2:3b":        131072,
		"mistral:7b":         32768,
		"mixtral:8x7b":       32768,
		"mixtral:8x22b":      65536,
		"phi3:mini":          131072,
		"phi3:medium":        131072,
		"gemma:2b":           8192,
		"gemma:7b":           8192,
		"gemma2:9b":          8192,
		"gemma2:27b":         8192,
	}

	if ctx, ok := modelContexts[o.modelName]; ok {
		return ctx
	}

	// Default conservative estimate
	return 8192
}

// GetUsableContext returns the usable context size (75% of total)
func (o *OllamaClient) GetUsableContext() int {
	return o.GetContextWindow() * 3 / 4
}

// buildPrompt builds the full prompt from system and user prompts
func (o *OllamaClient) buildPrompt(req *InferenceRequest) string {
	var prompt strings.Builder

	// System prompt
	if req.SystemPrompt != "" {
		prompt.WriteString("System: ")
		prompt.WriteString(req.SystemPrompt)
		prompt.WriteString("\n\n")
	}

	// User prompt
	prompt.WriteString("User: ")
	prompt.WriteString(req.UserPrompt)
	prompt.WriteString("\n\nAssistant: ")

	return prompt.String()
}
