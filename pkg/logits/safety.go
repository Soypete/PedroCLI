package logits

import (
	"strings"
	"sync"
)

// SafetyCategory represents a category of content that can be blocked.
type SafetyCategory string

const (
	// CategoryProfanity blocks profane and vulgar language
	CategoryProfanity SafetyCategory = "profanity"

	// CategoryViolence blocks violent and harmful content
	CategoryViolence SafetyCategory = "violence"

	// CategoryPII blocks potential personally identifiable information patterns
	CategoryPII SafetyCategory = "pii"

	// CategoryCodeInjection blocks potential code injection patterns
	CategoryCodeInjection SafetyCategory = "code_injection"

	// CategoryCredentials blocks credential and secret patterns
	CategoryCredentials SafetyCategory = "credentials"

	// CategoryHate blocks hate speech patterns
	CategoryHate SafetyCategory = "hate"

	// CategorySexual blocks sexual content
	CategorySexual SafetyCategory = "sexual"

	// CategorySelfHarm blocks self-harm related content
	CategorySelfHarm SafetyCategory = "self_harm"

	// CategoryDangerous blocks dangerous instructions
	CategoryDangerous SafetyCategory = "dangerous"
)

// AllSafetyCategories returns all defined safety categories.
func AllSafetyCategories() []SafetyCategory {
	return []SafetyCategory{
		CategoryProfanity,
		CategoryViolence,
		CategoryPII,
		CategoryCodeInjection,
		CategoryCredentials,
		CategoryHate,
		CategorySexual,
		CategorySelfHarm,
		CategoryDangerous,
	}
}

// SafetyFilter blocks unsafe content at the token level.
// It maintains lists of banned tokens and sequences organized by category.
type SafetyFilter struct {
	StatefulFilter

	mu sync.RWMutex

	// bannedTokenIDs maps token IDs to ban
	bannedTokenIDs map[int]bool

	// bannedSequences are multi-token sequences to block
	bannedSequences []bannedSequence

	// sequenceBuffer tracks recent tokens for sequence detection
	sequenceBuffer []int
	maxSeqLen      int

	// enabledCategories tracks which categories are active
	enabledCategories map[SafetyCategory]bool

	// categoryTokens maps categories to their banned tokens
	categoryTokens map[SafetyCategory][]int

	// categorySequences maps categories to their banned sequences
	categorySequences map[SafetyCategory][][]int

	// tokenizer for analyzing vocabulary
	tokenizer Tokenizer

	// customPatterns are user-defined patterns to block
	customPatterns []string

	// customTokens are user-defined token IDs to block
	customTokens []int
}

type bannedSequence struct {
	tokens   []int
	category SafetyCategory
}

// NewSafetyFilter creates a new safety filter with the given tokenizer.
func NewSafetyFilter(tokenizer Tokenizer) *SafetyFilter {
	sf := &SafetyFilter{
		StatefulFilter:    StatefulFilter{enabled: true},
		bannedTokenIDs:    make(map[int]bool),
		bannedSequences:   make([]bannedSequence, 0),
		sequenceBuffer:    make([]int, 0),
		maxSeqLen:         10, // Track last 10 tokens for sequence matching
		enabledCategories: make(map[SafetyCategory]bool),
		categoryTokens:    make(map[SafetyCategory][]int),
		categorySequences: make(map[SafetyCategory][][]int),
		tokenizer:         tokenizer,
	}

	// Build banned lists from vocabulary
	if tokenizer != nil {
		sf.buildBannedLists()
	}

	return sf
}

// buildBannedLists analyzes the vocabulary to find tokens to ban per category.
// TODO: This needs to be populated with actual safety word lists.
// For now, we provide a framework that can be extended.
func (s *SafetyFilter) buildBannedLists() {
	if s.tokenizer == nil {
		return
	}

	vocab := s.tokenizer.GetVocabulary()

	// Build lists for each category based on token content
	for tokenID, tokenStr := range vocab {
		lower := strings.ToLower(tokenStr)

		// Code injection patterns
		if s.matchesCodeInjectionPattern(lower) {
			s.categoryTokens[CategoryCodeInjection] = append(
				s.categoryTokens[CategoryCodeInjection], tokenID)
		}

		// Credential patterns
		if s.matchesCredentialPattern(lower) {
			s.categoryTokens[CategoryCredentials] = append(
				s.categoryTokens[CategoryCredentials], tokenID)
		}

		// PII patterns (emails, SSN-like, etc.)
		if s.matchesPIIPattern(lower) {
			s.categoryTokens[CategoryPII] = append(
				s.categoryTokens[CategoryPII], tokenID)
		}

		// TODO: Add other categories with appropriate word lists
		// These should be loaded from configuration or external files
		// to avoid hardcoding potentially offensive content
	}
}

// matchesCodeInjectionPattern checks for code injection related tokens.
func (s *SafetyFilter) matchesCodeInjectionPattern(token string) bool {
	// Look for shell injection patterns
	injectionPatterns := []string{
		"`", "$(", "${", "eval(", "exec(", "system(",
		"; rm ", "| rm ", "&& rm ", "|| rm ",
		"; sudo", "| sudo", "&& sudo",
		"chmod 777", "chmod +x",
		"curl |", "wget |",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(token, pattern) {
			return true
		}
	}

	return false
}

// matchesCredentialPattern checks for credential-like tokens.
func (s *SafetyFilter) matchesCredentialPattern(token string) bool {
	credentialPatterns := []string{
		"password=", "passwd=", "secret=", "api_key=",
		"apikey=", "access_token=", "auth_token=",
		"private_key", "-----begin rsa",
		"aws_access_key", "aws_secret",
	}

	for _, pattern := range credentialPatterns {
		if strings.Contains(token, pattern) {
			return true
		}
	}

	return false
}

// matchesPIIPattern checks for PII-like tokens.
func (s *SafetyFilter) matchesPIIPattern(token string) bool {
	// Look for SSN, credit card patterns, etc.
	// This is simplified - real implementation would use regex
	piiPatterns := []string{
		"ssn:", "social security",
		"credit card", "card number",
		"cvv:", "expiry:",
	}

	for _, pattern := range piiPatterns {
		if strings.Contains(token, pattern) {
			return true
		}
	}

	return false
}

// Name returns the filter name.
func (s *SafetyFilter) Name() string {
	return "safety"
}

// Description returns the filter description.
func (s *SafetyFilter) Description() string {
	return "Blocks unsafe content by category"
}

// Apply masks logits to ban unsafe tokens.
func (s *SafetyFilter) Apply(logits []float32, ctx *GenerationContext) []float32 {
	if !s.enabled {
		return logits
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ban individual tokens
	for tokenID := range s.bannedTokenIDs {
		if tokenID >= 0 && tokenID < len(logits) {
			logits[tokenID] = NegativeInfinity
		}
	}

	// Check if current generation is approaching a banned sequence
	for _, seq := range s.bannedSequences {
		if s.isApproachingSequence(seq.tokens) {
			// Ban the next token in the sequence
			nextIdx := len(s.sequenceBuffer)
			if nextIdx < len(seq.tokens) {
				nextToken := seq.tokens[nextIdx]
				if nextToken >= 0 && nextToken < len(logits) {
					logits[nextToken] = NegativeInfinity
				}
			}
		}
	}

	return logits
}

// isApproachingSequence checks if recent tokens match the start of a sequence.
func (s *SafetyFilter) isApproachingSequence(seq []int) bool {
	if len(s.sequenceBuffer) == 0 || len(seq) == 0 {
		return false
	}

	// Check if the sequence buffer ends with the start of this sequence
	bufLen := len(s.sequenceBuffer)
	for checkLen := 1; checkLen <= bufLen && checkLen < len(seq); checkLen++ {
		match := true
		for i := 0; i < checkLen; i++ {
			if s.sequenceBuffer[bufLen-checkLen+i] != seq[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}

// OnTokenGenerated updates the sequence buffer.
func (s *SafetyFilter) OnTokenGenerated(tokenID int, tokenText string, ctx *GenerationContext) {
	if !s.enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Add to sequence buffer
	s.sequenceBuffer = append(s.sequenceBuffer, tokenID)

	// Trim buffer if too long
	if len(s.sequenceBuffer) > s.maxSeqLen {
		s.sequenceBuffer = s.sequenceBuffer[1:]
	}
}

// Reset resets the filter state.
func (s *SafetyFilter) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequenceBuffer = s.sequenceBuffer[:0]
}

// EnableCategory enables blocking for a category.
func (s *SafetyFilter) EnableCategory(cat SafetyCategory) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.enabledCategories[cat] {
		return
	}

	s.enabledCategories[cat] = true

	// Add category tokens to banned list
	for _, tokenID := range s.categoryTokens[cat] {
		s.bannedTokenIDs[tokenID] = true
	}

	// Add category sequences to banned list
	for _, seq := range s.categorySequences[cat] {
		s.bannedSequences = append(s.bannedSequences, bannedSequence{
			tokens:   seq,
			category: cat,
		})
	}
}

// DisableCategory disables blocking for a category.
func (s *SafetyFilter) DisableCategory(cat SafetyCategory) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabledCategories[cat] {
		return
	}

	s.enabledCategories[cat] = false

	// Remove category tokens from banned list
	// (only if not banned by other enabled categories)
	tokensToKeep := make(map[int]bool)
	for enabledCat := range s.enabledCategories {
		if enabledCat != cat && s.enabledCategories[enabledCat] {
			for _, tokenID := range s.categoryTokens[enabledCat] {
				tokensToKeep[tokenID] = true
			}
		}
	}

	// Also keep custom tokens
	for _, tokenID := range s.customTokens {
		tokensToKeep[tokenID] = true
	}

	// Rebuild banned tokens
	s.bannedTokenIDs = tokensToKeep

	// Rebuild banned sequences (remove this category's)
	newSeqs := make([]bannedSequence, 0)
	for _, seq := range s.bannedSequences {
		if seq.category != cat {
			newSeqs = append(newSeqs, seq)
		}
	}
	s.bannedSequences = newSeqs
}

// EnableCategories enables multiple categories at once.
func (s *SafetyFilter) EnableCategories(cats ...SafetyCategory) {
	for _, cat := range cats {
		s.EnableCategory(cat)
	}
}

// DisableAllCategories disables all safety categories.
func (s *SafetyFilter) DisableAllCategories() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabledCategories = make(map[SafetyCategory]bool)

	// Keep only custom tokens
	s.bannedTokenIDs = make(map[int]bool)
	for _, tokenID := range s.customTokens {
		s.bannedTokenIDs[tokenID] = true
	}

	// Clear category sequences
	s.bannedSequences = s.bannedSequences[:0]
}

// EnabledCategories returns the list of enabled categories.
func (s *SafetyFilter) EnabledCategories() []SafetyCategory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cats := make([]SafetyCategory, 0, len(s.enabledCategories))
	for cat, enabled := range s.enabledCategories {
		if enabled {
			cats = append(cats, cat)
		}
	}
	return cats
}

// AddCustomBannedToken adds a custom token to the ban list.
func (s *SafetyFilter) AddCustomBannedToken(tokenID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.customTokens = append(s.customTokens, tokenID)
	s.bannedTokenIDs[tokenID] = true
}

// AddCustomBannedSequence adds a custom sequence to the ban list.
func (s *SafetyFilter) AddCustomBannedSequence(tokenIDs []int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.bannedSequences = append(s.bannedSequences, bannedSequence{
		tokens:   tokenIDs,
		category: "", // No category for custom sequences
	})
}

// AddCustomBannedPattern adds a pattern that will be tokenized and banned.
func (s *SafetyFilter) AddCustomBannedPattern(pattern string) {
	if s.tokenizer == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.customPatterns = append(s.customPatterns, pattern)

	// Tokenize and add as sequence
	tokens := s.tokenizer.StringToTokens(pattern)
	if len(tokens) > 0 {
		if len(tokens) == 1 {
			s.customTokens = append(s.customTokens, tokens[0])
			s.bannedTokenIDs[tokens[0]] = true
		} else {
			s.bannedSequences = append(s.bannedSequences, bannedSequence{
				tokens:   tokens,
				category: "",
			})
		}
	}
}

// BannedTokenCount returns the number of banned tokens.
func (s *SafetyFilter) BannedTokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.bannedTokenIDs)
}

// BannedSequenceCount returns the number of banned sequences.
func (s *SafetyFilter) BannedSequenceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.bannedSequences)
}

// GetLogitBiases returns the banned tokens as a logit bias map.
// Useful for passing to backends that support logit_bias parameter.
func (s *SafetyFilter) GetLogitBiases() map[int]float32 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	biases := make(map[int]float32, len(s.bannedTokenIDs))
	for tokenID := range s.bannedTokenIDs {
		biases[tokenID] = -100 // Strong negative bias
	}
	return biases
}

// SafetyPreset is a predefined safety configuration.
type SafetyPreset struct {
	Name        string
	Description string
	Categories  []SafetyCategory
}

// SafetyPresets provides common safety configurations.
var SafetyPresets = map[string]*SafetyPreset{
	"minimal": {
		Name:        "minimal",
		Description: "Minimal safety - only blocks code injection",
		Categories: []SafetyCategory{
			CategoryCodeInjection,
		},
	},
	"standard": {
		Name:        "standard",
		Description: "Standard safety - blocks code injection and credentials",
		Categories: []SafetyCategory{
			CategoryCodeInjection,
			CategoryCredentials,
		},
	},
	"strict": {
		Name:        "strict",
		Description: "Strict safety - blocks most harmful content categories",
		Categories: []SafetyCategory{
			CategoryCodeInjection,
			CategoryCredentials,
			CategoryPII,
			CategoryViolence,
			CategoryDangerous,
		},
	},
	"maximum": {
		Name:        "maximum",
		Description: "Maximum safety - blocks all content categories",
		Categories:  AllSafetyCategories(),
	},
}

// ApplyPreset enables categories from a predefined preset.
func (s *SafetyFilter) ApplyPreset(presetName string) error {
	preset, ok := SafetyPresets[presetName]
	if !ok {
		return nil // Unknown preset, silently ignore
	}

	s.DisableAllCategories()
	s.EnableCategories(preset.Categories...)
	return nil
}

// LoadWordList loads additional banned words from a list.
// Each word is tokenized and added as a banned pattern.
func (s *SafetyFilter) LoadWordList(category SafetyCategory, words []string) {
	if s.tokenizer == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, word := range words {
		tokens := s.tokenizer.StringToTokens(word)
		if len(tokens) == 0 {
			continue
		}

		if len(tokens) == 1 {
			s.categoryTokens[category] = append(s.categoryTokens[category], tokens[0])
		} else {
			s.categorySequences[category] = append(s.categorySequences[category], tokens)
		}
	}
}
