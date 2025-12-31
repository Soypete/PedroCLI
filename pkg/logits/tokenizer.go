package logits

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Tokenizer provides vocabulary access for logit manipulation.
// This interface abstracts tokenizer operations needed for grammar
// and safety filters.
type Tokenizer interface {
	// VocabSize returns the size of the vocabulary.
	VocabSize() int

	// TokenToString converts a token ID to its string representation.
	TokenToString(tokenID int) string

	// StringToTokens tokenizes a string into token IDs.
	// This is a greedy tokenization, not necessarily optimal.
	StringToTokens(s string) []int

	// GetVocabulary returns the full vocabulary as a slice.
	// Index is token ID, value is token string.
	GetVocabulary() []string

	// IsSpecialToken returns true if the token is a special token
	// (BOS, EOS, PAD, etc.)
	IsSpecialToken(tokenID int) bool

	// EOSToken returns the end-of-sequence token ID.
	EOSToken() int

	// BOSToken returns the beginning-of-sequence token ID.
	BOSToken() int
}

// VocabTokenizer is a simple vocabulary-based tokenizer.
// It loads vocabulary from a file and provides basic tokenization.
type VocabTokenizer struct {
	vocab      []string
	tokenToID  map[string]int
	specialIDs map[int]bool
	eosTokenID int
	bosTokenID int
}

// NewVocabTokenizer creates a tokenizer with the given vocabulary.
func NewVocabTokenizer(vocab []string) *VocabTokenizer {
	tokenToID := make(map[string]int, len(vocab))
	for i, token := range vocab {
		tokenToID[token] = i
	}

	t := &VocabTokenizer{
		vocab:      vocab,
		tokenToID:  tokenToID,
		specialIDs: make(map[int]bool),
		eosTokenID: -1,
		bosTokenID: -1,
	}

	// Try to detect special tokens
	t.detectSpecialTokens()

	return t
}

// LoadVocabFromFile loads vocabulary from a text file (one token per line).
func LoadVocabFromFile(path string) (*VocabTokenizer, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vocab file: %w", err)
	}
	defer file.Close()

	var vocab []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		vocab = append(vocab, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read vocab file: %w", err)
	}

	return NewVocabTokenizer(vocab), nil
}

// LoadVocabFromJSON loads vocabulary from a JSON file.
// Expects either an array of strings or an object with token->id mapping.
func LoadVocabFromJSON(path string) (*VocabTokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read vocab file: %w", err)
	}

	// Try array format first
	var vocabArray []string
	if err := json.Unmarshal(data, &vocabArray); err == nil {
		return NewVocabTokenizer(vocabArray), nil
	}

	// Try object format
	var vocabMap map[string]int
	if err := json.Unmarshal(data, &vocabMap); err == nil {
		// Convert to array
		vocab := make([]string, len(vocabMap))
		for token, id := range vocabMap {
			if id >= 0 && id < len(vocab) {
				vocab[id] = token
			}
		}
		return NewVocabTokenizer(vocab), nil
	}

	return nil, fmt.Errorf("vocab file must be JSON array or object")
}

func (t *VocabTokenizer) detectSpecialTokens() {
	specialPatterns := []string{
		"<s>", "</s>", "<eos>", "<bos>", "<pad>", "<unk>",
		"[CLS]", "[SEP]", "[PAD]", "[UNK]", "[MASK]",
		"<|endoftext|>", "<|im_start|>", "<|im_end|>",
	}

	for i, token := range t.vocab {
		lower := strings.ToLower(token)
		for _, pattern := range specialPatterns {
			if lower == strings.ToLower(pattern) {
				t.specialIDs[i] = true

				// Try to identify EOS/BOS
				if strings.Contains(lower, "eos") || lower == "</s>" || lower == "<|endoftext|>" {
					t.eosTokenID = i
				}
				if strings.Contains(lower, "bos") || lower == "<s>" {
					t.bosTokenID = i
				}
				break
			}
		}
	}
}

// VocabSize returns the vocabulary size.
func (t *VocabTokenizer) VocabSize() int {
	return len(t.vocab)
}

// TokenToString converts a token ID to string.
func (t *VocabTokenizer) TokenToString(tokenID int) string {
	if tokenID < 0 || tokenID >= len(t.vocab) {
		return ""
	}
	return t.vocab[tokenID]
}

// StringToTokens performs greedy tokenization.
// This is a simple implementation - not optimal like BPE.
func (t *VocabTokenizer) StringToTokens(s string) []int {
	var tokens []int

	for len(s) > 0 {
		// Find longest matching token
		bestLen := 0
		bestID := -1

		for token, id := range t.tokenToID {
			if len(token) > bestLen && strings.HasPrefix(s, token) {
				bestLen = len(token)
				bestID = id
			}
		}

		if bestID == -1 {
			// No match, skip character
			s = s[1:]
			continue
		}

		tokens = append(tokens, bestID)
		s = s[bestLen:]
	}

	return tokens
}

// GetVocabulary returns the full vocabulary.
func (t *VocabTokenizer) GetVocabulary() []string {
	return t.vocab
}

// IsSpecialToken checks if token is special.
func (t *VocabTokenizer) IsSpecialToken(tokenID int) bool {
	return t.specialIDs[tokenID]
}

// EOSToken returns the EOS token ID.
func (t *VocabTokenizer) EOSToken() int {
	return t.eosTokenID
}

// BOSToken returns the BOS token ID.
func (t *VocabTokenizer) BOSToken() int {
	return t.bosTokenID
}

// SetSpecialToken marks a token as special.
func (t *VocabTokenizer) SetSpecialToken(tokenID int) {
	t.specialIDs[tokenID] = true
}

// SetEOSToken sets the EOS token ID.
func (t *VocabTokenizer) SetEOSToken(tokenID int) {
	t.eosTokenID = tokenID
	t.specialIDs[tokenID] = true
}

// SetBOSToken sets the BOS token ID.
func (t *VocabTokenizer) SetBOSToken(tokenID int) {
	t.bosTokenID = tokenID
	t.specialIDs[tokenID] = true
}

// FindTokensMatching returns token IDs whose strings match the predicate.
func (t *VocabTokenizer) FindTokensMatching(pred func(string) bool) []int {
	var matches []int
	for i, token := range t.vocab {
		if pred(token) {
			matches = append(matches, i)
		}
	}
	return matches
}

// FindTokensContaining returns token IDs containing the substring.
func (t *VocabTokenizer) FindTokensContaining(substr string) []int {
	return t.FindTokensMatching(func(s string) bool {
		return strings.Contains(s, substr)
	})
}

// FindTokensStartingWith returns token IDs starting with the prefix.
func (t *VocabTokenizer) FindTokensStartingWith(prefix string) []int {
	return t.FindTokensMatching(func(s string) bool {
		return strings.HasPrefix(s, prefix)
	})
}

// TokenAnalysis provides analysis of a token for debugging.
type TokenAnalysis struct {
	TokenID   int
	Text      string
	IsSpecial bool
	ByteLen   int
	RuneCount int
}

// AnalyzeToken returns detailed information about a token.
func (t *VocabTokenizer) AnalyzeToken(tokenID int) *TokenAnalysis {
	if tokenID < 0 || tokenID >= len(t.vocab) {
		return nil
	}
	text := t.vocab[tokenID]
	return &TokenAnalysis{
		TokenID:   tokenID,
		Text:      text,
		IsSpecial: t.specialIDs[tokenID],
		ByteLen:   len(text),
		RuneCount: len([]rune(text)),
	}
}
