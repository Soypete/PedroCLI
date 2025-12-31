package logits

import (
	"fmt"
	"sync"
)

// FilterChain applies multiple filters in sequence.
// Filters are applied in the order they were added.
type FilterChain struct {
	mu      sync.RWMutex
	filters []LogitFilter
}

// NewFilterChain creates an empty filter chain.
func NewFilterChain() *FilterChain {
	return &FilterChain{
		filters: make([]LogitFilter, 0),
	}
}

// NewFilterChainWithFilters creates a chain with the given filters.
func NewFilterChainWithFilters(filters ...LogitFilter) *FilterChain {
	return &FilterChain{
		filters: filters,
	}
}

// Add appends a filter to the chain.
func (c *FilterChain) Add(filter LogitFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filters = append(c.filters, filter)
}

// Insert inserts a filter at the specified position.
func (c *FilterChain) Insert(index int, filter LogitFilter) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index > len(c.filters) {
		return fmt.Errorf("index %d out of range [0, %d]", index, len(c.filters))
	}

	c.filters = append(c.filters[:index], append([]LogitFilter{filter}, c.filters[index:]...)...)
	return nil
}

// Remove removes a filter by name.
func (c *FilterChain) Remove(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, f := range c.filters {
		if f.Name() == name {
			c.filters = append(c.filters[:i], c.filters[i+1:]...)
			return true
		}
	}
	return false
}

// Get returns a filter by name, or nil if not found.
func (c *FilterChain) Get(name string) LogitFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, f := range c.filters {
		if f.Name() == name {
			return f
		}
	}
	return nil
}

// Apply runs all enabled filters in sequence.
func (c *FilterChain) Apply(logits []float32, ctx *GenerationContext) []float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		if filter.Enabled() {
			logits = filter.Apply(logits, ctx)
		}
	}
	return logits
}

// ApplyWithResult applies filters and returns detailed results.
func (c *FilterChain) ApplyWithResult(logits []float32, ctx *GenerationContext) *FilterResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	activeFilters := make([]string, 0)

	for _, filter := range c.filters {
		if filter.Enabled() {
			logits = filter.Apply(logits, ctx)
			activeFilters = append(activeFilters, filter.Name())
		}
	}

	// Count banned tokens
	bannedCount := 0
	for _, logit := range logits {
		if logit == NegativeInfinity {
			bannedCount++
		}
	}

	return &FilterResult{
		ModifiedLogits:   logits,
		BannedTokenCount: bannedCount,
		ActiveFilters:    activeFilters,
	}
}

// OnTokenGenerated notifies all filters that a token was generated.
func (c *FilterChain) OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		if filter.Enabled() {
			filter.OnTokenGenerated(tokenID, tokenText, ctx)
		}
	}
}

// Reset resets all filters in the chain.
func (c *FilterChain) Reset() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		filter.Reset()
	}
}

// EnableAll enables all filters in the chain.
func (c *FilterChain) EnableAll() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		filter.SetEnabled(true)
	}
}

// DisableAll disables all filters in the chain.
func (c *FilterChain) DisableAll() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		filter.SetEnabled(false)
	}
}

// Enable enables a specific filter by name.
func (c *FilterChain) Enable(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		if filter.Name() == name {
			filter.SetEnabled(true)
			return true
		}
	}
	return false
}

// Disable disables a specific filter by name.
func (c *FilterChain) Disable(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, filter := range c.filters {
		if filter.Name() == name {
			filter.SetEnabled(false)
			return true
		}
	}
	return false
}

// Filters returns a copy of the filter list.
func (c *FilterChain) Filters() []LogitFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]LogitFilter, len(c.filters))
	copy(result, c.filters)
	return result
}

// ActiveFilters returns only the enabled filters.
func (c *FilterChain) ActiveFilters() []LogitFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]LogitFilter, 0)
	for _, f := range c.filters {
		if f.Enabled() {
			result = append(result, f)
		}
	}
	return result
}

// Len returns the number of filters in the chain.
func (c *FilterChain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.filters)
}

// Clear removes all filters from the chain.
func (c *FilterChain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filters = c.filters[:0]
}

// Clone creates a copy of the filter chain.
// Note: This creates a shallow copy - filters are shared.
func (c *FilterChain) Clone() *FilterChain {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filters := make([]LogitFilter, len(c.filters))
	copy(filters, c.filters)
	return &FilterChain{filters: filters}
}

// ChainBuilder provides a fluent interface for building filter chains.
type ChainBuilder struct {
	chain *FilterChain
}

// NewChainBuilder creates a new builder.
func NewChainBuilder() *ChainBuilder {
	return &ChainBuilder{
		chain: NewFilterChain(),
	}
}

// With adds a filter to the chain.
func (b *ChainBuilder) With(filter LogitFilter) *ChainBuilder {
	b.chain.Add(filter)
	return b
}

// WithBannedTokens adds a token ban filter.
func (b *ChainBuilder) WithBannedTokens(name string, tokenIDs []int) *ChainBuilder {
	b.chain.Add(NewTokenBanFilter(name, "Bans specific tokens", tokenIDs))
	return b
}

// WithLogitBias adds a logit bias filter.
func (b *ChainBuilder) WithLogitBias(biases map[int]float32) *ChainBuilder {
	b.chain.Add(NewLogitBiasFilter(biases))
	return b
}

// Build returns the constructed filter chain.
func (b *ChainBuilder) Build() *FilterChain {
	return b.chain
}
