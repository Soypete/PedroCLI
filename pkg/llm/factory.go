package llm

import (
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
)

// NewBackend creates a new LLM backend based on the configuration
func NewBackend(cfg *config.Config) (Backend, error) {
	switch cfg.Model.Type {
	case "llamacpp":
		return NewLlamaCppClient(cfg), nil
	case "ollama":
		return NewOllamaClient(cfg), nil
	default:
		return nil, fmt.Errorf("unknown backend type: %s (supported: llamacpp, ollama)", cfg.Model.Type)
	}
}
