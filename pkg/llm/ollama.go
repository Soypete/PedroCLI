package llm

import (
	"github.com/soypete/pedrocli/pkg/config"
)

// OllamaClient implements Backend for Ollama HTTP API
type OllamaClient struct {
	*ServerClient
	config *config.Config
}

// NewOllamaClient creates a new Ollama HTTP client
func NewOllamaClient(cfg *config.Config) *OllamaClient {
	return NewOllamaClientFromModel(cfg, cfg.Model)
}

// NewOllamaClientFromModel creates a client from a specific model config
func NewOllamaClientFromModel(cfg *config.Config, modelCfg config.ModelConfig) *OllamaClient {
	// Determine server URL
	serverURL := modelCfg.ServerURL
	if serverURL == "" {
		serverURL = "http://localhost:11434" // Default Ollama port
	}

	// Determine model name
	modelName := modelCfg.ModelName
	if modelName == "" {
		modelName = "qwen2.5-coder:32b" // Default model
	}

	// Determine context size (use auto-detection if not specified)
	contextSize := modelCfg.ContextSize
	if contextSize == 0 {
		contextSize = getOllamaContextSize(modelName)
	}

	// Create server client using OpenAI-compatible endpoint
	serverClient := NewServerClient(ServerClientConfig{
		BaseURL:     serverURL,
		ModelName:   modelName,
		ContextSize: contextSize,
		EnableTools: modelCfg.EnableTools,
		APIPath:     "/v1/chat/completions", // Ollama supports OpenAI-compatible API
	})

	return &OllamaClient{
		ServerClient: serverClient,
		config:       cfg,
	}
}

// getOllamaContextSize returns the context window size for known Ollama models
func getOllamaContextSize(modelName string) int {
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

	if ctx, ok := modelContexts[modelName]; ok {
		return ctx
	}

	// Default conservative estimate
	return 8192
}
