package logits

import (
	"fmt"
	"strings"
)

// GrammarFilter enforces GBNF grammar constraints at the logit level.
// It tracks the grammar parse state and only allows tokens that
// could continue a valid parse.
//
// NOTE: Full grammar-constrained generation is best done server-side
// in llama.cpp. This filter is for cases where we need client-side
// control or want to pre-compute valid token masks.
type GrammarFilter struct {
	StatefulFilter

	grammar      *GBNF
	state        *GrammarState
	tokenizer    Tokenizer

	// validTokenCache caches valid tokens for each grammar state
	// Key is a state fingerprint, value is set of valid token IDs
	validTokenCache map[string]map[int]bool

	// prefixTokens maps string prefixes to tokens that could produce them
	prefixTokens map[string][]int
}

// NewGrammarFilter creates a filter that enforces the given grammar.
func NewGrammarFilter(grammar *GBNF, tokenizer Tokenizer) *GrammarFilter {
	gf := &GrammarFilter{
		StatefulFilter:  StatefulFilter{enabled: true},
		grammar:         grammar,
		tokenizer:       tokenizer,
		validTokenCache: make(map[string]map[int]bool),
		prefixTokens:    make(map[string][]int),
	}
	gf.Reset()
	gf.buildPrefixIndex()
	return gf
}

// NewGrammarFilterFromString parses a GBNF grammar string and creates a filter.
func NewGrammarFilterFromString(grammarStr string, tokenizer Tokenizer) (*GrammarFilter, error) {
	grammar, err := ParseGBNF(grammarStr)
	if err != nil {
		return nil, fmt.Errorf("parse grammar: %w", err)
	}
	return NewGrammarFilter(grammar, tokenizer), nil
}

// buildPrefixIndex builds an index of tokens by their prefixes.
// This enables efficient lookup of tokens that could match a partial string.
func (g *GrammarFilter) buildPrefixIndex() {
	vocab := g.tokenizer.GetVocabulary()
	for tokenID, tokenStr := range vocab {
		if tokenStr == "" {
			continue
		}
		// Index by first few characters
		for prefixLen := 1; prefixLen <= len(tokenStr) && prefixLen <= 4; prefixLen++ {
			prefix := tokenStr[:prefixLen]
			g.prefixTokens[prefix] = append(g.prefixTokens[prefix], tokenID)
		}
	}
}

// Name returns the filter name.
func (g *GrammarFilter) Name() string {
	return "grammar"
}

// Description returns the filter description.
func (g *GrammarFilter) Description() string {
	return "Enforces GBNF grammar constraints on generation"
}

// Apply masks logits to only allow grammar-valid tokens.
func (g *GrammarFilter) Apply(logits []float32, ctx *GenerationContext) []float32 {
	if !g.enabled {
		return logits
	}

	validTokens := g.getValidTokens(ctx)

	// If we couldn't determine valid tokens, allow all
	if len(validTokens) == 0 {
		return logits
	}

	// Ban all tokens not in valid set
	for i := range logits {
		if !validTokens[i] {
			logits[i] = NegativeInfinity
		}
	}

	return logits
}

// getValidTokens returns the set of tokens valid in the current grammar state.
func (g *GrammarFilter) getValidTokens(ctx *GenerationContext) map[int]bool {
	// Check cache first
	stateKey := g.stateFingerprint()
	if cached, ok := g.validTokenCache[stateKey]; ok {
		return cached
	}

	// Compute valid tokens
	validTokens := g.computeValidTokens(ctx)

	// Cache result (limit cache size)
	if len(g.validTokenCache) < 10000 {
		g.validTokenCache[stateKey] = validTokens
	}

	return validTokens
}

// computeValidTokens computes which tokens are valid given current state.
func (g *GrammarFilter) computeValidTokens(ctx *GenerationContext) map[int]bool {
	validTokens := make(map[int]bool)

	if g.state == nil || g.grammar == nil {
		return validTokens
	}

	// Get current rule
	rule, ok := g.grammar.Rules[g.state.CurrentRule]
	if !ok {
		return validTokens
	}

	// Get valid next strings from grammar
	validPrefixes := g.getValidPrefixes(rule, g.state)

	// Find tokens that could produce these prefixes
	vocab := g.tokenizer.GetVocabulary()
	for tokenID, tokenStr := range vocab {
		if g.tokenMatchesValidPrefix(tokenStr, validPrefixes) {
			validTokens[tokenID] = true
		}
	}

	// Always allow EOS if grammar could be complete
	if g.couldBeComplete() {
		eosID := g.tokenizer.EOSToken()
		if eosID >= 0 {
			validTokens[eosID] = true
		}
	}

	return validTokens
}

// getValidPrefixes returns strings that could validly come next in the grammar.
func (g *GrammarFilter) getValidPrefixes(rule *GBNFRule, state *GrammarState) []string {
	var prefixes []string

	if state.AlternateIndex >= len(rule.Alternates) {
		return prefixes
	}

	alt := rule.Alternates[state.AlternateIndex]
	if state.ElementIndex >= len(alt.Elements) {
		return prefixes
	}

	elem := alt.Elements[state.ElementIndex]
	prefixes = append(prefixes, g.getPrefixesFromElement(elem)...)

	return prefixes
}

// getPrefixesFromElement extracts valid string prefixes from a grammar element.
func (g *GrammarFilter) getPrefixesFromElement(elem GBNFElement) []string {
	switch e := elem.(type) {
	case GBNFLiteral:
		return []string{e.Value}

	case GBNFCharClass:
		// Return all single characters that match the class
		var prefixes []string
		// Check ASCII printable range
		for r := rune(32); r < 127; r++ {
			if e.Matches(r) {
				prefixes = append(prefixes, string(r))
			}
		}
		// Add common unicode if not negated
		if !e.Negated {
			for _, ch := range e.Chars {
				prefixes = append(prefixes, string(ch))
			}
		}
		return prefixes

	case GBNFRuleRef:
		// Recursively get prefixes from referenced rule
		if refRule, ok := g.grammar.Rules[e.RuleName]; ok {
			var prefixes []string
			for _, alt := range refRule.Alternates {
				if len(alt.Elements) > 0 {
					prefixes = append(prefixes, g.getPrefixesFromElement(alt.Elements[0])...)
				}
			}
			return prefixes
		}

	case GBNFGroup:
		var prefixes []string
		for _, alt := range e.Alternates {
			if len(alt.Elements) > 0 {
				prefixes = append(prefixes, g.getPrefixesFromElement(alt.Elements[0])...)
			}
		}
		return prefixes

	case GBNFRepetition:
		// Get prefixes from inner element
		prefixes := g.getPrefixesFromElement(e.Element)
		// If min is 0, also get prefixes from next element (skip this one)
		// This is simplified - full implementation would track position
		return prefixes
	}

	return nil
}

// tokenMatchesValidPrefix checks if a token could produce any valid prefix.
func (g *GrammarFilter) tokenMatchesValidPrefix(tokenStr string, validPrefixes []string) bool {
	for _, prefix := range validPrefixes {
		// Token matches if it's a prefix of the valid string
		// or if the valid string is a prefix of the token
		if strings.HasPrefix(prefix, tokenStr) || strings.HasPrefix(tokenStr, prefix) {
			return true
		}
	}
	return false
}

// couldBeComplete checks if the current generation could be a complete parse.
func (g *GrammarFilter) couldBeComplete() bool {
	if g.state == nil {
		return true
	}

	rule, ok := g.grammar.Rules[g.state.CurrentRule]
	if !ok {
		return true
	}

	// Check if we're past the end of all elements in current alternate
	if g.state.AlternateIndex >= len(rule.Alternates) {
		return len(g.state.Stack) == 0
	}

	alt := rule.Alternates[g.state.AlternateIndex]

	// If we're past all elements and stack is empty, we're done
	if g.state.ElementIndex >= len(alt.Elements) {
		return len(g.state.Stack) == 0
	}

	// Check if remaining elements are optional
	for i := g.state.ElementIndex; i < len(alt.Elements); i++ {
		if !g.isOptionalElement(alt.Elements[i]) {
			return false
		}
	}

	return len(g.state.Stack) == 0
}

// isOptionalElement checks if an element can be skipped.
func (g *GrammarFilter) isOptionalElement(elem GBNFElement) bool {
	if rep, ok := elem.(GBNFRepetition); ok {
		return rep.Min == 0
	}
	return false
}

// stateFingerprint returns a string identifying the current state.
func (g *GrammarFilter) stateFingerprint() string {
	if g.state == nil {
		return "nil"
	}
	return fmt.Sprintf("%s:%d:%d:%s",
		g.state.CurrentRule,
		g.state.AlternateIndex,
		g.state.ElementIndex,
		g.state.MatchedText)
}

// OnTokenGenerated updates grammar state after a token is sampled.
func (g *GrammarFilter) OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext) {
	if !g.enabled || g.state == nil {
		return
	}

	// Advance the grammar state based on the token text
	g.advanceState(tokenText)
}

// advanceState updates the grammar state based on matched text.
func (g *GrammarFilter) advanceState(text string) {
	if g.state == nil {
		return
	}

	g.state.MatchedText += text

	// Try to consume the text through the grammar
	// This is a simplified implementation
	rule, ok := g.grammar.Rules[g.state.CurrentRule]
	if !ok {
		return
	}

	if g.state.AlternateIndex >= len(rule.Alternates) {
		return
	}

	alt := rule.Alternates[g.state.AlternateIndex]

	// Try to match against current element
	for g.state.ElementIndex < len(alt.Elements) {
		elem := alt.Elements[g.state.ElementIndex]
		consumed := g.tryConsume(elem, g.state.MatchedText)
		if consumed > 0 {
			g.state.MatchedText = g.state.MatchedText[consumed:]
			g.state.ElementIndex++
		} else if consumed == 0 && g.isOptionalElement(elem) {
			// Optional element, skip it
			g.state.ElementIndex++
		} else {
			// Couldn't consume, stop
			break
		}
	}
}

// tryConsume tries to consume text with the given grammar element.
// Returns number of characters consumed, or -1 if no match possible.
func (g *GrammarFilter) tryConsume(elem GBNFElement, text string) int {
	switch e := elem.(type) {
	case GBNFLiteral:
		if strings.HasPrefix(text, e.Value) {
			return len(e.Value)
		}
		// Partial match - keep trying
		if strings.HasPrefix(e.Value, text) {
			return 0
		}
		return -1

	case GBNFCharClass:
		if len(text) == 0 {
			return 0
		}
		if e.Matches(rune(text[0])) {
			return 1
		}
		return -1

	case GBNFRepetition:
		consumed := 0
		count := 0
		for {
			n := g.tryConsume(e.Element, text[consumed:])
			if n <= 0 {
				break
			}
			consumed += n
			count++
			if e.Max >= 0 && count >= e.Max {
				break
			}
		}
		if count < e.Min {
			return -1
		}
		return consumed

	case GBNFRuleRef:
		// Would need to recursively check referenced rule
		// Simplified: just return 0 (partial match possible)
		return 0

	case GBNFGroup:
		// Try each alternate
		for _, alt := range e.Alternates {
			consumed := 0
			matched := true
			for _, subElem := range alt.Elements {
				n := g.tryConsume(subElem, text[consumed:])
				if n < 0 {
					matched = false
					break
				}
				consumed += n
			}
			if matched {
				return consumed
			}
		}
		return 0 // Partial match possible
	}

	return 0
}

// Reset resets the grammar state for a new generation.
func (g *GrammarFilter) Reset() {
	if g.grammar != nil {
		g.state = NewGrammarState(g.grammar.RootRule)
	}
}

// Grammar returns the underlying GBNF grammar.
func (g *GrammarFilter) Grammar() *GBNF {
	return g.grammar
}

// SetGrammar updates the grammar and resets state.
func (g *GrammarFilter) SetGrammar(grammar *GBNF) {
	g.grammar = grammar
	g.validTokenCache = make(map[string]map[int]bool)
	g.Reset()
}

// GrammarString returns the grammar as a GBNF string.
func (g *GrammarFilter) GrammarString() string {
	if g.grammar == nil {
		return ""
	}
	return g.grammar.String()
}

// JSONGrammarFilter is a convenience filter for JSON output.
type JSONGrammarFilter struct {
	*GrammarFilter
}

// NewJSONGrammarFilter creates a filter that enforces JSON output.
func NewJSONGrammarFilter(tokenizer Tokenizer) (*JSONGrammarFilter, error) {
	grammar, err := ParseGBNF(JSONGrammar)
	if err != nil {
		return nil, fmt.Errorf("parse JSON grammar: %w", err)
	}
	return &JSONGrammarFilter{
		GrammarFilter: NewGrammarFilter(grammar, tokenizer),
	}, nil
}

// JSONObjectGrammarFilter is a convenience filter for JSON object output.
type JSONObjectGrammarFilter struct {
	*GrammarFilter
}

// NewJSONObjectGrammarFilter creates a filter that enforces JSON object output.
func NewJSONObjectGrammarFilter(tokenizer Tokenizer) (*JSONObjectGrammarFilter, error) {
	grammar, err := ParseGBNF(JSONObjectGrammar)
	if err != nil {
		return nil, fmt.Errorf("parse JSON object grammar: %w", err)
	}
	return &JSONObjectGrammarFilter{
		GrammarFilter: NewGrammarFilter(grammar, tokenizer),
	}, nil
}
