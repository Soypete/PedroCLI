package llm

import (
	"github.com/soypete/pedrocli/pkg/config"
)

// LlamaCppClient implements Backend for llama-server HTTP API
type LlamaCppClient struct {
	*ServerClient
	config *config.Config
}

// NewLlamaCppClient creates a new llama-server HTTP client
func NewLlamaCppClient(cfg *config.Config) *LlamaCppClient {
	return NewLlamaCppClientFromModel(cfg, cfg.Model)
}

// NewLlamaCppClientFromModel creates a client from a specific model config
func NewLlamaCppClientFromModel(cfg *config.Config, modelCfg config.ModelConfig) *LlamaCppClient {
	// Determine server URL
	serverURL := modelCfg.ServerURL
	if serverURL == "" {
		serverURL = "http://localhost:8082" // Default llama-server port
	}

	// Determine model name
	modelName := modelCfg.ModelName
	if modelName == "" {
		modelName = "default" // llama-server doesn't require model name in request
	}

	// Create server client
	serverClient := NewServerClient(ServerClientConfig{
		BaseURL:     serverURL,
		ModelName:   modelName,
		ContextSize: modelCfg.ContextSize,
		EnableTools: modelCfg.EnableTools,
		APIPath:     "/v1/chat/completions", // OpenAI-compatible endpoint
	})

	return &LlamaCppClient{
		ServerClient: serverClient,
		config:       cfg,
	}
}
