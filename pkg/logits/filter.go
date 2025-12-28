// Package logits provides token-level control for LLM generation.
// It implements logit manipulation filters for structured outputs,
// safety controls, and grammar-based constraints.
//
// This package is designed for use with llama.cpp backend where
// direct logit control is available. It is NOT used with Ollama
// which handles sampling internally.
package logits

import (
	"context"
	"math"
)

// NegativeInfinity is used to ban tokens by setting their logit to -inf
var NegativeInfinity = float32(math.Inf(-1))

// GenerationContext holds state during token generation.
// It provides information about the current generation state
// that filters can use to make decisions.
type GenerationContext struct {
	// Prompt is the original input prompt
	Prompt string

	// GeneratedTokens are the token IDs generated so far
	GeneratedTokens []int

	// GeneratedText is the decoded text generated so far
	GeneratedText string

	// CurrentPosition is the current token position in generation
	CurrentPosition int

	// Metadata holds additional context-specific data
	// Filters can store and retrieve state here
	Metadata map[string]interface{}
}

// NewGenerationContext creates a new GenerationContext with the given prompt.
func NewGenerationContext(prompt string) *GenerationContext {
	return &GenerationContext{
		Prompt:          prompt,
		GeneratedTokens: make([]int, 0),
		GeneratedText:   "",
		CurrentPosition: 0,
		Metadata:        make(map[string]interface{}),
	}
}

// AppendToken adds a generated token to the context.
func (c *GenerationContext) AppendToken(tokenID int, tokenText string) {
	c.GeneratedTokens = append(c.GeneratedTokens, tokenID)
	c.GeneratedText += tokenText
	c.CurrentPosition++
}

// Reset clears the generation state for a new generation.
func (c *GenerationContext) Reset() {
	c.GeneratedTokens = c.GeneratedTokens[:0]
	c.GeneratedText = ""
	c.CurrentPosition = 0
	c.Metadata = make(map[string]interface{})
}

// LogitFilter modifies logits before sampling.
// Implementations can ban tokens, boost/penalize tokens,
// or enforce grammatical constraints.
type LogitFilter interface {
	// Name returns the filter's identifier
	Name() string

	// Description returns a human-readable description
	Description() string

	// Apply modifies the logits array based on the current context.
	// Returns the modified logits array.
	// To ban a token, set its logit to NegativeInfinity.
	// To boost a token, add a positive bias.
	// To penalize a token, add a negative bias.
	Apply(logits []float32, ctx *GenerationContext) []float32

	// OnTokenGenerated is called after a token is sampled.
	// Filters can update their internal state here.
	OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext)

	// Reset resets the filter's internal state for a new generation.
	Reset()

	// Enabled returns whether the filter is currently active.
	Enabled() bool

	// SetEnabled enables or disables the filter.
	SetEnabled(enabled bool)
}

// StatefulFilter provides a base implementation for stateful filters.
type StatefulFilter struct {
	enabled bool
}

// Enabled returns whether the filter is active.
func (f *StatefulFilter) Enabled() bool {
	return f.enabled
}

// SetEnabled enables or disables the filter.
func (f *StatefulFilter) SetEnabled(enabled bool) {
	f.enabled = enabled
}

// TokenBanFilter is a simple filter that bans specific tokens.
type TokenBanFilter struct {
	StatefulFilter
	name          string
	description   string
	bannedTokens  map[int]bool
}

// NewTokenBanFilter creates a filter that bans the specified token IDs.
func NewTokenBanFilter(name, description string, bannedTokenIDs []int) *TokenBanFilter {
	banned := make(map[int]bool, len(bannedTokenIDs))
	for _, id := range bannedTokenIDs {
		banned[id] = true
	}
	return &TokenBanFilter{
		StatefulFilter: StatefulFilter{enabled: true},
		name:           name,
		description:    description,
		bannedTokens:   banned,
	}
}

// Name returns the filter name.
func (f *TokenBanFilter) Name() string { return f.name }

// Description returns the filter description.
func (f *TokenBanFilter) Description() string { return f.description }

// Apply bans the configured tokens.
func (f *TokenBanFilter) Apply(logits []float32, ctx *GenerationContext) []float32 {
	if !f.enabled {
		return logits
	}
	for tokenID := range f.bannedTokens {
		if tokenID >= 0 && tokenID < len(logits) {
			logits[tokenID] = NegativeInfinity
		}
	}
	return logits
}

// OnTokenGenerated does nothing for token ban filter.
func (f *TokenBanFilter) OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext) {}

// Reset does nothing for token ban filter.
func (f *TokenBanFilter) Reset() {}

// AddBannedToken adds a token to the ban list.
func (f *TokenBanFilter) AddBannedToken(tokenID int) {
	f.bannedTokens[tokenID] = true
}

// RemoveBannedToken removes a token from the ban list.
func (f *TokenBanFilter) RemoveBannedToken(tokenID int) {
	delete(f.bannedTokens, tokenID)
}

// LogitBiasFilter applies biases to specific tokens.
type LogitBiasFilter struct {
	StatefulFilter
	biases map[int]float32
}

// NewLogitBiasFilter creates a filter with the specified token biases.
func NewLogitBiasFilter(biases map[int]float32) *LogitBiasFilter {
	return &LogitBiasFilter{
		StatefulFilter: StatefulFilter{enabled: true},
		biases:         biases,
	}
}

// Name returns the filter name.
func (f *LogitBiasFilter) Name() string { return "logit_bias" }

// Description returns the filter description.
func (f *LogitBiasFilter) Description() string { return "Applies fixed biases to token logits" }

// Apply adds biases to the configured tokens.
func (f *LogitBiasFilter) Apply(logits []float32, ctx *GenerationContext) []float32 {
	if !f.enabled {
		return logits
	}
	for tokenID, bias := range f.biases {
		if tokenID >= 0 && tokenID < len(logits) {
			logits[tokenID] += bias
		}
	}
	return logits
}

// OnTokenGenerated does nothing for logit bias filter.
func (f *LogitBiasFilter) OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext) {}

// Reset does nothing for logit bias filter.
func (f *LogitBiasFilter) Reset() {}

// SetBias sets or updates a bias for a token.
func (f *LogitBiasFilter) SetBias(tokenID int, bias float32) {
	f.biases[tokenID] = bias
}

// FilterResult holds the result of applying filters.
type FilterResult struct {
	// ModifiedLogits is the logits array after all filters applied
	ModifiedLogits []float32

	// BannedTokenCount is how many tokens were banned (set to -inf)
	BannedTokenCount int

	// ActiveFilters lists which filters were applied
	ActiveFilters []string
}

// Generation represents a completed generation with metadata.
type Generation struct {
	// Text is the generated text
	Text string

	// TokenIDs are the generated token IDs
	TokenIDs []int

	// TokenCount is the number of tokens generated
	TokenCount int

	// FiltersApplied lists filters that were active
	FiltersApplied []string

	// Metadata holds additional generation info
	Metadata map[string]interface{}
}

// FilterFactory creates filters based on configuration.
type FilterFactory interface {
	// CreateFilter creates a filter by name with the given config.
	CreateFilter(ctx context.Context, name string, config map[string]interface{}) (LogitFilter, error)

	// AvailableFilters returns the list of filter types that can be created.
	AvailableFilters() []string
}
