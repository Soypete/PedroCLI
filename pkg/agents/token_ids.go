package agents

import (
	"context"
	"fmt"
	"sync"

	"github.com/soypete/pedrocli/pkg/llm"
)

// TokenIDProvider provides token IDs for bias phrases
// This allows dynamic token ID lookup instead of hardcoding model-specific IDs
type TokenIDProvider interface {
	// GetTokenIDs returns token IDs for multiple phrases
	// Returns map of phrase -> token IDs (a phrase may tokenize to multiple tokens)
	GetTokenIDs(phrases []string) (map[string][]int, error)

	// GetSingleTokenID returns the first token ID for a phrase
	// Useful when you know a phrase tokenizes to a single token
	GetSingleTokenID(phrase string) (int, error)
}

// HTTPTokenIDProvider fetches token IDs from the LLM backend's /tokenize endpoint
// Results are cached for performance (no need to call /tokenize repeatedly)
type HTTPTokenIDProvider struct {
	backend llm.Backend
	cache   map[string][]int
	mu      sync.RWMutex
}

// NewHTTPTokenIDProvider creates a new HTTP-based token ID provider
func NewHTTPTokenIDProvider(backend llm.Backend) *HTTPTokenIDProvider {
	return &HTTPTokenIDProvider{
		backend: backend,
		cache:   make(map[string][]int),
	}
}

// GetTokenIDs fetches token IDs for multiple phrases (with caching)
func (p *HTTPTokenIDProvider) GetTokenIDs(phrases []string) (map[string][]int, error) {
	result := make(map[string][]int)

	for _, phrase := range phrases {
		// Check cache first
		p.mu.RLock()
		if ids, ok := p.cache[phrase]; ok {
			p.mu.RUnlock()
			result[phrase] = ids
			continue
		}
		p.mu.RUnlock()

		// Check if backend supports tokenization
		tokenizer, ok := p.backend.(interface {
			Tokenize(ctx context.Context, text string) ([]int, error)
		})
		if !ok {
			// Backend doesn't support tokenization - return empty
			// This is safe: empty bias is better than wrong bias
			continue
		}

		// Fetch from backend
		ids, err := tokenizer.Tokenize(context.Background(), phrase)
		if err != nil {
			// Skip on error, don't break entire bias
			// Empty bias for this phrase is safer than wrong bias
			continue
		}

		// Cache result
		p.mu.Lock()
		p.cache[phrase] = ids
		p.mu.Unlock()

		result[phrase] = ids
	}

	return result, nil
}

// GetSingleTokenID returns the first token ID for a phrase
func (p *HTTPTokenIDProvider) GetSingleTokenID(phrase string) (int, error) {
	ids, err := p.GetTokenIDs([]string{phrase})
	if err != nil || len(ids[phrase]) == 0 {
		return 0, fmt.Errorf("no token ID found for: %s", phrase)
	}
	return ids[phrase][0], nil
}

// StaticTokenIDProvider provides hardcoded token IDs as fallback
// Used when HTTP tokenization is unavailable or for testing
type StaticTokenIDProvider struct {
	tokenMap map[string][]int
}

// NewStaticTokenIDProvider creates a new static provider with model-specific token maps
func NewStaticTokenIDProvider(modelFamily string) *StaticTokenIDProvider {
	var tokenMap map[string][]int

	switch modelFamily {
	case "llama3":
		tokenMap = llama3TokenMap
	case "qwen":
		tokenMap = qwenTokenMap
	case "glm4":
		tokenMap = glm4TokenMap
	default:
		// Empty map for unknown models - safer than wrong IDs
		tokenMap = make(map[string][]int)
	}

	return &StaticTokenIDProvider{tokenMap: tokenMap}
}

// GetTokenIDs returns static token IDs for phrases
func (p *StaticTokenIDProvider) GetTokenIDs(phrases []string) (map[string][]int, error) {
	result := make(map[string][]int)
	for _, phrase := range phrases {
		if ids, ok := p.tokenMap[phrase]; ok {
			result[phrase] = ids
		}
	}
	return result, nil
}

// GetSingleTokenID returns the first token ID for a phrase from static map
func (p *StaticTokenIDProvider) GetSingleTokenID(phrase string) (int, error) {
	ids, ok := p.tokenMap[phrase]
	if !ok || len(ids) == 0 {
		return 0, fmt.Errorf("no token ID found for: %s", phrase)
	}
	return ids[0], nil
}

// Hardcoded token maps for different model families
// These are used as fallback when HTTP tokenization is unavailable

// llama3TokenMap contains Llama 3.x token IDs (from existing hardcoded values)
var llama3TokenMap = map[string][]int{
	"```":       {13249},
	"json":      {2285},
	"Tool":      {7575},
	"Result":    {2122},
	"Output":    {5207},
	"Let's":     {5562},
	"should":    {8005},
	"would":     {1053},
	"will":      {3685},
	"expected":  {3685}, // Note: Same ID as "will" in original
	"✓":         {2375},
	"✗":         {2377},
	"PASS":      {12950},
	"FAIL":      {8755},
	"returned":  {5263},
	"received":  {22217},
	"actual":    {37373},
	"shows":     {2427},
	"indicates": {5039},
	"The":       {791},
	"failed":    {6052},
	"succeeded": {23130},
}

// qwenTokenMap contains Qwen 2.5 token IDs
// TODO: These need to be determined from actual Qwen tokenizer
var qwenTokenMap = map[string][]int{
	// Placeholder - to be filled with actual Qwen token IDs
	// For now, leave empty to use HTTP tokenization
}

// glm4TokenMap contains GLM-4 token IDs
// TODO: These need to be determined from actual GLM-4 tokenizer
var glm4TokenMap = map[string][]int{
	// Placeholder - to be filled with actual GLM-4 token IDs
	// For now, leave empty to use HTTP tokenization
}

// NullTokenIDProvider provides no token IDs (always returns empty)
// Used for models that don't support logit bias or when tokenization is unavailable
type NullTokenIDProvider struct{}

// NewNullTokenIDProvider creates a provider that returns no token IDs
func NewNullTokenIDProvider() *NullTokenIDProvider {
	return &NullTokenIDProvider{}
}

// GetTokenIDs returns empty map (no bias)
func (p *NullTokenIDProvider) GetTokenIDs(phrases []string) (map[string][]int, error) {
	return make(map[string][]int), nil
}

// GetSingleTokenID returns error (no token ID available)
func (p *NullTokenIDProvider) GetSingleTokenID(phrase string) (int, error) {
	return 0, fmt.Errorf("null provider has no token IDs")
}
