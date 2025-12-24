package llm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// LlamaCppClient implements the Backend interface for llama.cpp
type LlamaCppClient struct {
	llamacppPath string
	modelPath    string
	contextSize  int
	usableSize   int
	nGpuLayers   int
	temperature  float64
	threads      int
}

// NewLlamaCppClient creates a new llama.cpp client
func NewLlamaCppClient(cfg *config.Config) *LlamaCppClient {
	return &LlamaCppClient{
		llamacppPath: cfg.Model.LlamaCppPath,
		modelPath:    cfg.Model.ModelPath,
		contextSize:  cfg.Model.ContextSize,
		usableSize:   cfg.Model.UsableContext,
		nGpuLayers:   cfg.Model.NGpuLayers,
		temperature:  cfg.Model.Temperature,
		threads:      cfg.Model.Threads,
	}
}

// Infer performs one-shot inference using llama.cpp
func (l *LlamaCppClient) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	// Build the full prompt
	fullPrompt := l.buildPrompt(req)

	// Build llama.cpp command
	args := []string{
		"-m", l.modelPath,
		"-c", fmt.Sprintf("%d", l.contextSize),
		"-n", fmt.Sprintf("%d", req.MaxTokens),
		"--temp", fmt.Sprintf("%.2f", req.Temperature),
		"-t", fmt.Sprintf("%d", l.threads),
		"-p", fullPrompt,
		"-ngl", fmt.Sprintf("%d", l.nGpuLayers),
		"--no-display-prompt", // Don't echo the prompt
	}

	// Execute llama.cpp
	cmd := exec.CommandContext(ctx, l.llamacppPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("llama.cpp execution failed: %w (output: %s)", err, string(output))
	}

	// Parse the output
	response := &InferenceResponse{
		Text:       strings.TrimSpace(string(output)),
		ToolCalls:  []ToolCall{}, // TODO: Parse tool calls from response
		NextAction: "COMPLETE",   // TODO: Determine based on response
		TokensUsed: EstimateTokens(string(output)),
	}

	return response, nil
}

// GetContextWindow returns the context window size
func (l *LlamaCppClient) GetContextWindow() int {
	return l.contextSize
}

// GetUsableContext returns the usable context size
func (l *LlamaCppClient) GetUsableContext() int {
	return l.usableSize
}

// buildPrompt builds the full prompt from system and user prompts
func (l *LlamaCppClient) buildPrompt(req *InferenceRequest) string {
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
