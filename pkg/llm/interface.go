package llm

import (
	"context"
	"encoding/json"
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

	// Tools for native API-based tool calling
	Tools []ToolDefinition `json:"tools,omitempty"`

	// Logit bias for controlling token probabilities
	// Map of token ID -> bias value (-100 to 100)
	// Positive values increase probability, negative decrease
	LogitBias map[int]float32 `json:"logit_bias,omitempty"`

	// Grammar constraint in GBNF format (llama-server only)
	// Forces output to conform to specified grammar
	Grammar string `json:"grammar,omitempty"`
}

// ToolDefinition defines a tool for native API-based tool calling
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]interface{} // JSON Schema
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

// UnmarshalJSON implements custom unmarshaling to accept both "tool" and "name" fields
// This provides backwards compatibility with prompts that use "tool" instead of "name"
func (tc *ToolCall) UnmarshalJSON(data []byte) error {
	// Define an alias to prevent recursion
	type Alias ToolCall

	// Temporary struct that accepts both "tool" and "name"
	aux := &struct {
		Tool string `json:"tool"` // Accept "tool" as alternative to "name"
		*Alias
	}{
		Alias: (*Alias)(tc),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// If "tool" was provided but "name" wasn't, use "tool" value for Name
	if aux.Tool != "" && tc.Name == "" {
		tc.Name = aux.Tool
	}

	return nil
}
