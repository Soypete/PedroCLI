package llm

import (
	"context"
)

// Backend represents an LLM inference backend
type Backend interface {
	// Infer performs one-shot inference
	Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)

	// GetContextWindow returns the context window size
	GetContextWindow() int

	// GetUsableContext returns the usable context size (75% of window)
	GetUsableContext() int
}

// InferenceRequest represents a request for inference
type InferenceRequest struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  float64
	MaxTokens    int
}

// InferenceResponse represents a response from inference
type InferenceResponse struct {
	Text       string
	ToolCalls  []ToolCall
	NextAction string // "CONTINUE", "COMPLETE", "ERROR"
	TokensUsed int
}

// ToolCall represents a tool call from the model
type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}
