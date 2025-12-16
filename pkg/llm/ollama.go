package llm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// OllamaBackend implements the Backend interface for Ollama
type OllamaBackend struct {
	modelName   string
	contextSize int
	usableSize  int
	temperature float64
}

// NewOllamaBackend creates a new Ollama backend
func NewOllamaBackend(cfg *config.Config) *OllamaBackend {
	usableSize := cfg.Model.UsableContext
	if usableSize == 0 {
		usableSize = int(float64(cfg.Model.ContextSize) * 0.75)
	}

	return &OllamaBackend{
		modelName:   cfg.Model.ModelName,
		contextSize: cfg.Model.ContextSize,
		usableSize:  usableSize,
		temperature: cfg.Model.Temperature,
	}
}

// Infer performs one-shot inference using Ollama CLI
func (o *OllamaBackend) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	// Build the full prompt
	fullPrompt := o.buildPrompt(req)

	// Build ollama command
	// Note: ollama run doesn't support all flags, so we keep it simple
	args := []string{
		"run",
		o.modelName,
		fullPrompt,
	}

	// Execute ollama
	cmd := exec.CommandContext(ctx, "ollama", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ollama execution failed: %w (output: %s)", err, string(output))
	}

	// Strip ANSI escape codes from output (ollama cli adds progress spinner)
	cleanOutput := stripANSI(string(output))

	// Parse the output
	response := &InferenceResponse{
		Text:       strings.TrimSpace(cleanOutput),
		ToolCalls:  []ToolCall{}, // TODO: Parse tool calls from response
		NextAction: "COMPLETE",    // TODO: Determine based on response
		TokensUsed: EstimateTokens(cleanOutput),
	}

	return response, nil
}

// stripANSI removes ANSI escape sequences from a string
func stripANSI(str string) string {
	// Simple ANSI strip - removes CSI sequences and other control codes
	var result strings.Builder
	inEscape := false
	inCSI := false

	for i := 0; i < len(str); i++ {
		ch := str[i]

		// Check for ESC character
		if ch == '\x1b' || ch == '\x9b' {
			inEscape = true
			inCSI = false
			continue
		}

		if inEscape {
			// Check for CSI sequence start (ESC [)
			if ch == '[' {
				inCSI = true
				continue
			}

			// End of escape sequence
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
				inEscape = false
				inCSI = false
				continue
			}

			// Continue skipping escape sequence characters
			if inCSI && (ch >= '0' && ch <= '9' || ch == ';' || ch == '?') {
				continue
			}

			// Unknown escape sequence, end it
			inEscape = false
			inCSI = false
			continue
		}

		// Skip other control characters except newlines, tabs
		if ch < 32 && ch != '\n' && ch != '\t' && ch != '\r' {
			continue
		}

		result.WriteByte(ch)
	}

	return result.String()
}

// GetContextWindow returns the context window size
func (o *OllamaBackend) GetContextWindow() int {
	return o.contextSize
}

// GetUsableContext returns the usable context size
func (o *OllamaBackend) GetUsableContext() int {
	return o.usableSize
}

// buildPrompt builds the full prompt from system and user prompts
func (o *OllamaBackend) buildPrompt(req *InferenceRequest) string {
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
