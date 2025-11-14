package llm

import (
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// EstimateTokens provides a rough token estimation (1 token ≈ 4 characters)
func EstimateTokens(text string) int {
	return len(text) / 4
}

// EstimateTokensAccurate provides a more accurate estimation based on word count
// Average: 1.3 tokens per word
func EstimateTokensAccurate(text string) int {
	words := len(strings.Fields(text))
	return int(float64(words) * 1.3)
}

// ContextBudget represents token allocation across different components
type ContextBudget struct {
	Total           int // Total context window size
	Usable          int // 75% of total (leave room for response)
	SystemPrompt    int // Tokens for system prompt
	TaskPrompt      int // Tokens for task description
	ToolDefinitions int // Tokens for tool definitions
	History         int // Tokens for history
	Available       int // Tokens available for code/files
}

// CalculateBudget calculates the context budget based on config and prompts
func CalculateBudget(cfg *config.Config, systemPrompt, taskPrompt, toolDefs string) *ContextBudget {
	total := cfg.Model.ContextSize
	usable := total * 3 / 4 // 75% usable

	systemTokens := EstimateTokens(systemPrompt)
	taskTokens := EstimateTokens(taskPrompt)
	toolTokens := EstimateTokens(toolDefs)

	// Reserve some for history and responses
	reserved := 2000

	available := usable - systemTokens - taskTokens - toolTokens - reserved

	return &ContextBudget{
		Total:           total,
		Usable:          usable,
		SystemPrompt:    systemTokens,
		TaskPrompt:      taskTokens,
		ToolDefinitions: toolTokens,
		Available:       available,
	}
}

// CanFitHistory checks if history fits within available budget
func (cb *ContextBudget) CanFitHistory(historyTokens int) bool {
	return historyTokens <= cb.Available
}

// MaxFilesSize returns the maximum size for files in tokens
func (cb *ContextBudget) MaxFilesSize() int {
	// Leave room for history and responses
	return cb.Available - 2000
}

// OllamaModelContexts maps known Ollama models to their context windows
var OllamaModelContexts = map[string]int{
	"qwen2.5-coder:7b":     32768,
	"qwen2.5-coder:32b":    32768,
	"qwen2.5-coder:72b":    131072,
	"deepseek-coder:33b":   16384,
	"codellama:34b":        16384,
	"llama3.1:70b":         131072,
	"qwen2.5-coder:latest": 32768,
}

// GetOllamaContextSize returns the context window size for a given Ollama model
func GetOllamaContextSize(modelName string) int {
	if ctx, ok := OllamaModelContexts[modelName]; ok {
		return ctx
	}
	// Default conservative estimate
	return 8192
}

// GetUsableContext returns 75% of the context window
func GetUsableContext(contextSize int) int {
	return contextSize * 3 / 4
}
