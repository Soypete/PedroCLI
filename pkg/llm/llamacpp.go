package llm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// LlamaCppClient implements the Backend interface for llama.cpp
// It uses one-shot subprocess execution (not llama-server HTTP API)
type LlamaCppClient struct {
	llamacppPath string
	modelPath    string
	contextSize  int
	usableSize   int
	nGpuLayers   int
	temperature  float64
	threads      int

	// Grammar configuration
	enableGrammar  bool   // Enable GBNF grammar for tool calling
	grammarLogging bool   // Enable debug logging for grammar

	// Logit control options (applied via CLI flags)
	grammar       string  // GBNF grammar string for constrained generation
	grammarFile   string  // Path to grammar file (alternative to inline)
	repeatPenalty float64 // Repetition penalty (default 1.1)
	repeatLastN   int     // How many tokens to check for repetition
	topK          int     // Top-K sampling
	topP          float64 // Top-P (nucleus) sampling
	minP          float64 // Min-P sampling
}

// NewLlamaCppClient creates a new llama.cpp client
func NewLlamaCppClient(cfg *config.Config) *LlamaCppClient {
	return NewLlamaCppClientFromModel(cfg, cfg.Model)
}

// NewLlamaCppClientFromModel creates a new llama.cpp client from a specific model config
func NewLlamaCppClientFromModel(cfg *config.Config, modelCfg config.ModelConfig) *LlamaCppClient {
	return &LlamaCppClient{
		llamacppPath:   modelCfg.LlamaCppPath,
		modelPath:      modelCfg.ModelPath,
		contextSize:    modelCfg.ContextSize,
		usableSize:     modelCfg.UsableContext,
		nGpuLayers:     modelCfg.NGpuLayers,
		temperature:    modelCfg.Temperature,
		threads:        modelCfg.Threads,
		enableGrammar:  modelCfg.EnableGrammar,
		grammarLogging: modelCfg.GrammarLogging,
	}
}

// Infer performs one-shot inference using llama.cpp
func (l *LlamaCppClient) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	// Build the full prompt
	fullPrompt := l.buildPrompt(req)

	// Build llama.cpp command with base arguments
	args := []string{
		"-m", l.modelPath,
		"-c", fmt.Sprintf("%d", l.contextSize),
		"-n", fmt.Sprintf("%d", req.MaxTokens),
		"--temp", fmt.Sprintf("%.2f", req.Temperature),
		"-t", fmt.Sprintf("%d", l.threads),
		"-p", fullPrompt,
		"-ngl", fmt.Sprintf("%d", l.nGpuLayers),
		"--no-display-prompt", // Don't echo the prompt
		"--jinja",             // Enable jinja templates for tool calling
		"-no-cnv",             // Disable interactive conversation mode (one-shot)
	}

	// Add grammar constraints if configured and enabled
	// Grammar is enforced directly by llama.cpp at the logit level
	if l.enableGrammar {
		if l.grammarLogging {
			fmt.Fprintf(os.Stderr, "[DEBUG] Grammar check: l.grammar=%d bytes, l.grammarFile=%s\n", len(l.grammar), l.grammarFile)
		}
		grammarFile := l.grammarFile
		if l.grammar != "" && grammarFile == "" {
			if l.grammarLogging {
				fmt.Fprintf(os.Stderr, "[DEBUG] Writing inline grammar to temp file\n")
			}
			// Write inline grammar to temp file
			tmpFile, err := l.writeGrammarToTempFile(l.grammar)
			if err != nil {
				return nil, fmt.Errorf("failed to write grammar: %w", err)
			}
			defer os.Remove(tmpFile)
			grammarFile = tmpFile
			if l.grammarLogging {
				fmt.Fprintf(os.Stderr, "[DEBUG] Grammar temp file: %s\n", grammarFile)
			}
		}
		if grammarFile != "" {
			if l.grammarLogging {
				fmt.Fprintf(os.Stderr, "[DEBUG] Adding --grammar-file %s to args\n", grammarFile)
			}
			args = append(args, "--grammar-file", grammarFile)
		} else if l.grammarLogging {
			fmt.Fprintf(os.Stderr, "[DEBUG] NO GRAMMAR FILE - skipping grammar constraint\n")
		}
	} else if l.grammarLogging {
		fmt.Fprintf(os.Stderr, "[DEBUG] Grammar disabled via config (enable_grammar=false)\n")
	}

	// Add sampling parameters for logit control
	if l.repeatPenalty > 0 {
		args = append(args, "--repeat-penalty", fmt.Sprintf("%.2f", l.repeatPenalty))
	}
	if l.repeatLastN > 0 {
		args = append(args, "--repeat-last-n", fmt.Sprintf("%d", l.repeatLastN))
	}
	if l.topK > 0 {
		args = append(args, "--top-k", fmt.Sprintf("%d", l.topK))
	}
	if l.topP > 0 {
		args = append(args, "--top-p", fmt.Sprintf("%.2f", l.topP))
	}
	if l.minP > 0 {
		args = append(args, "--min-p", fmt.Sprintf("%.2f", l.minP))
	}

	// Execute llama.cpp
	fmt.Fprintf(os.Stderr, "[DEBUG] Executing llama.cpp: %s %s\n", l.llamacppPath, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, l.llamacppPath, args...)
	output, err := cmd.CombinedOutput()

	// Always save llama.cpp output to debug file for troubleshooting
	debugOutputFile := "/tmp/pedrocli-llamacpp-output.txt"
	if writeErr := os.WriteFile(debugOutputFile, output, 0644); writeErr != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] Failed to write llama.cpp output to %s: %v\n", debugOutputFile, writeErr)
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG] Saved llama.cpp output to: %s (%d bytes)\n", debugOutputFile, len(output))
	}

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

// writeGrammarToTempFile writes a GBNF grammar string to a temporary file
func (l *LlamaCppClient) writeGrammarToTempFile(grammar string) (string, error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "pedrocli-grammar.gbnf")

	// Write to debug file if grammar logging is enabled
	if l.grammarLogging {
		debugFile := "/tmp/pedrocli-grammar-debug.gbnf"
		if err := os.WriteFile(debugFile, []byte(grammar), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] Failed to write debug grammar file: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] Grammar written to: %s\n", debugFile)
		}
	}

	if err := os.WriteFile(tmpFile, []byte(grammar), 0600); err != nil {
		return "", err
	}
	return tmpFile, nil
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

// SetGrammar sets a GBNF grammar string for constrained generation.
// The grammar is enforced at the logit level by llama.cpp.
func (l *LlamaCppClient) SetGrammar(grammar string) {
	l.grammar = grammar
	l.grammarFile = "" // Clear file if setting inline grammar
}

// SetGrammarFile sets a path to a GBNF grammar file.
func (l *LlamaCppClient) SetGrammarFile(path string) {
	l.grammarFile = path
	l.grammar = "" // Clear inline if setting file
}

// ClearGrammar removes any grammar constraint.
func (l *LlamaCppClient) ClearGrammar() {
	l.grammar = ""
	l.grammarFile = ""
}

// SetRepeatPenalty sets the repetition penalty (1.0 = no penalty).
func (l *LlamaCppClient) SetRepeatPenalty(penalty float64) {
	l.repeatPenalty = penalty
}

// SetRepeatLastN sets how many tokens to check for repetition.
func (l *LlamaCppClient) SetRepeatLastN(n int) {
	l.repeatLastN = n
}

// SetTopK sets top-k sampling (0 = disabled).
func (l *LlamaCppClient) SetTopK(k int) {
	l.topK = k
}

// SetTopP sets top-p (nucleus) sampling (0.0 = disabled).
func (l *LlamaCppClient) SetTopP(p float64) {
	l.topP = p
}

// SetMinP sets min-p sampling (0.0 = disabled).
func (l *LlamaCppClient) SetMinP(p float64) {
	l.minP = p
}

// ConfigureForStructuredOutput sets optimal parameters for structured output.
// Low temperature, tight sampling, grammar enforcement.
func (l *LlamaCppClient) ConfigureForStructuredOutput() {
	l.temperature = 0.1
	l.topK = 40
	l.topP = 0.9
	l.repeatPenalty = 1.0
}

// ConfigureForToolCalls sets optimal parameters for tool call generation.
// Deterministic settings with tool call grammar.
func (l *LlamaCppClient) ConfigureForToolCalls() {
	l.temperature = 0.0
	l.topK = 1
	l.topP = 1.0
	l.repeatPenalty = 1.0
}
