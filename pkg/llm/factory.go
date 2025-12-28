package llm

import (
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
)

// NewBackend creates a new LLM backend based on the configuration
func NewBackend(cfg *config.Config) (Backend, error) {
	return NewBackendFromModel(cfg, cfg.Model)
}

// NewBackendForProfile creates a new LLM backend for a specific model profile
func NewBackendForProfile(cfg *config.Config, profileName string) (Backend, error) {
	modelCfg := cfg.GetModelConfig(profileName)
	return NewBackendFromModel(cfg, modelCfg)
}

// NewBackendForPodcast creates a new LLM backend optimized for podcast/content tasks
func NewBackendForPodcast(cfg *config.Config) (Backend, error) {
	modelCfg := cfg.GetPodcastModelConfig()
	return NewBackendFromModel(cfg, modelCfg)
}

// NewBackendFromModel creates a new LLM backend from a specific model configuration
func NewBackendFromModel(cfg *config.Config, modelCfg config.ModelConfig) (Backend, error) {
	switch modelCfg.Type {
	case "llamacpp":
		return NewLlamaCppClientFromModel(cfg, modelCfg), nil
	case "ollama":
		return NewOllamaClientFromModel(cfg, modelCfg), nil
	default:
		return nil, fmt.Errorf("unknown backend type: %s (supported: llamacpp, ollama)", modelCfg.Type)
	}
}
