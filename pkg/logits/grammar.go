package logits

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// GBNF represents a parsed GBNF grammar.
// GBNF (GGML BNF) is a variant of BNF used by llama.cpp for constrained generation.
type GBNF struct {
	// Rules maps rule names to their definitions
	Rules map[string]*GBNFRule

	// RootRule is the starting rule (typically "root")
	RootRule string

	// rawGrammar is the original grammar string
	rawGrammar string
}

// GBNFRule represents a single grammar rule.
type GBNFRule struct {
	Name        string
	Alternates  []GBNFAlternate
	IsRecursive bool
}

// GBNFAlternate represents one alternative in a rule (separated by |).
type GBNFAlternate struct {
	Elements []GBNFElement
}

// GBNFElement represents an element in a rule.
type GBNFElement interface {
	isGBNFElement()
	String() string
}

// GBNFLiteral is a literal string match.
type GBNFLiteral struct {
	Value string
}

func (GBNFLiteral) isGBNFElement() {}
func (l GBNFLiteral) String() string {
	return fmt.Sprintf(`"%s"`, l.Value)
}

// GBNFCharClass is a character class like [a-z] or [^"].
type GBNFCharClass struct {
	Chars    string
	Negated  bool
	Ranges   [][2]rune // Start-end pairs for ranges
}

func (GBNFCharClass) isGBNFElement() {}
func (c GBNFCharClass) String() string {
	if c.Negated {
		return fmt.Sprintf("[^%s]", c.Chars)
	}
	return fmt.Sprintf("[%s]", c.Chars)
}

// Matches checks if a rune matches this character class.
func (c GBNFCharClass) Matches(r rune) bool {
	// Check ranges
	for _, rang := range c.Ranges {
		if r >= rang[0] && r <= rang[1] {
			return !c.Negated
		}
	}
	// Check individual chars
	for _, ch := range c.Chars {
		if r == ch {
			return !c.Negated
		}
	}
	return c.Negated
}

// GBNFRuleRef is a reference to another rule.
type GBNFRuleRef struct {
	RuleName string
}

func (GBNFRuleRef) isGBNFElement() {}
func (r GBNFRuleRef) String() string {
	return r.RuleName
}

// GBNFRepetition wraps an element with repetition (* + ?).
type GBNFRepetition struct {
	Element GBNFElement
	Min     int // 0 for *, 1 for +
	Max     int // -1 for unlimited
}

func (GBNFRepetition) isGBNFElement() {}
func (r GBNFRepetition) String() string {
	suffix := ""
	if r.Min == 0 && r.Max == -1 {
		suffix = "*"
	} else if r.Min == 1 && r.Max == -1 {
		suffix = "+"
	} else if r.Min == 0 && r.Max == 1 {
		suffix = "?"
	}
	return fmt.Sprintf("(%s)%s", r.Element.String(), suffix)
}

// GBNFGroup is a grouped expression.
type GBNFGroup struct {
	Alternates []GBNFAlternate
}

func (GBNFGroup) isGBNFElement() {}
func (g GBNFGroup) String() string {
	parts := make([]string, len(g.Alternates))
	for i, alt := range g.Alternates {
		elems := make([]string, len(alt.Elements))
		for j, e := range alt.Elements {
			elems[j] = e.String()
		}
		parts[i] = strings.Join(elems, " ")
	}
	return "(" + strings.Join(parts, " | ") + ")"
}

// GrammarState tracks the current parse position in the grammar.
type GrammarState struct {
	// CurrentRule is the rule currently being matched
	CurrentRule string

	// Position in the current alternate
	AlternateIndex int

	// Position in the current alternate's elements
	ElementIndex int

	// Stack of parent states for nested rules
	Stack []*GrammarState

	// MatchedText is text matched so far in current state
	MatchedText string
}

// NewGrammarState creates a new state starting at the root rule.
func NewGrammarState(rootRule string) *GrammarState {
	return &GrammarState{
		CurrentRule:    rootRule,
		AlternateIndex: 0,
		ElementIndex:   0,
		Stack:          make([]*GrammarState, 0),
		MatchedText:    "",
	}
}

// Clone creates a copy of the state.
func (s *GrammarState) Clone() *GrammarState {
	stack := make([]*GrammarState, len(s.Stack))
	for i, state := range s.Stack {
		stack[i] = state.Clone()
	}
	return &GrammarState{
		CurrentRule:    s.CurrentRule,
		AlternateIndex: s.AlternateIndex,
		ElementIndex:   s.ElementIndex,
		Stack:          stack,
		MatchedText:    s.MatchedText,
	}
}

// ParseGBNF parses a GBNF grammar string.
func ParseGBNF(grammar string) (*GBNF, error) {
	p := &gbnfParser{
		input: grammar,
		pos:   0,
		rules: make(map[string]*GBNFRule),
	}

	if err := p.parse(); err != nil {
		return nil, fmt.Errorf("parse GBNF: %w", err)
	}

	// Determine root rule (first rule defined, or "root" if exists)
	rootRule := ""
	if _, ok := p.rules["root"]; ok {
		rootRule = "root"
	} else if p.firstRule != "" {
		rootRule = p.firstRule
	}

	return &GBNF{
		Rules:      p.rules,
		RootRule:   rootRule,
		rawGrammar: grammar,
	}, nil
}

// String returns the GBNF grammar as a string.
func (g *GBNF) String() string {
	if g.rawGrammar != "" {
		return g.rawGrammar
	}

	var sb strings.Builder
	for name, rule := range g.Rules {
		sb.WriteString(name)
		sb.WriteString(" ::= ")

		for i, alt := range rule.Alternates {
			if i > 0 {
				sb.WriteString(" | ")
			}
			for j, elem := range alt.Elements {
				if j > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(elem.String())
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// ValidateRule checks if a rule exists in the grammar.
func (g *GBNF) ValidateRule(name string) bool {
	_, ok := g.Rules[name]
	return ok
}

// gbnfParser is the internal parser state.
type gbnfParser struct {
	input     string
	pos       int
	rules     map[string]*GBNFRule
	firstRule string
}

func (p *gbnfParser) parse() error {
	for p.pos < len(p.input) {
		p.skipWhitespaceAndComments()
		if p.pos >= len(p.input) {
			break
		}

		rule, err := p.parseRule()
		if err != nil {
			return err
		}
		if rule != nil {
			if p.firstRule == "" {
				p.firstRule = rule.Name
			}
			p.rules[rule.Name] = rule
		}
	}
	return nil
}

func (p *gbnfParser) skipWhitespaceAndComments() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '#' {
			// Skip comment until end of line
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *gbnfParser) parseRule() (*GBNFRule, error) {
	// Parse rule name
	name := p.parseIdentifier()
	if name == "" {
		return nil, nil
	}

	p.skipWhitespace()

	// Expect ::=
	if !p.consume("::=") {
		return nil, fmt.Errorf("expected '::=' after rule name '%s' at position %d", name, p.pos)
	}

	p.skipWhitespace()

	// Parse alternates
	alternates, err := p.parseAlternates()
	if err != nil {
		return nil, fmt.Errorf("parsing rule '%s': %w", name, err)
	}

	// Check for recursion
	isRecursive := false
	for _, alt := range alternates {
		for _, elem := range alt.Elements {
			if ref, ok := elem.(GBNFRuleRef); ok && ref.RuleName == name {
				isRecursive = true
				break
			}
		}
	}

	return &GBNFRule{
		Name:        name,
		Alternates:  alternates,
		IsRecursive: isRecursive,
	}, nil
}

func (p *gbnfParser) parseAlternates() ([]GBNFAlternate, error) {
	var alternates []GBNFAlternate

	for {
		elements, err := p.parseElements()
		if err != nil {
			return nil, err
		}
		alternates = append(alternates, GBNFAlternate{Elements: elements})

		p.skipWhitespace()
		if !p.consume("|") {
			break
		}
		p.skipWhitespace()
	}

	return alternates, nil
}

func (p *gbnfParser) parseElements() ([]GBNFElement, error) {
	var elements []GBNFElement

	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			break
		}

		ch := p.input[p.pos]
		if ch == '|' || ch == '\n' || ch == '\r' || ch == '#' {
			break
		}

		elem, err := p.parseElement()
		if err != nil {
			return nil, err
		}
		if elem == nil {
			break
		}

		// Check for repetition suffix
		elem = p.parseRepetition(elem)
		elements = append(elements, elem)
	}

	return elements, nil
}

func (p *gbnfParser) parseElement() (GBNFElement, error) {
	if p.pos >= len(p.input) {
		return nil, nil
	}

	ch := p.input[p.pos]

	switch ch {
	case '"':
		return p.parseLiteral()
	case '[':
		return p.parseCharClass()
	case '(':
		return p.parseGroup()
	default:
		if isIdentStart(ch) {
			name := p.parseIdentifier()
			if name != "" {
				return GBNFRuleRef{RuleName: name}, nil
			}
		}
	}

	return nil, nil
}

func (p *gbnfParser) parseLiteral() (GBNFElement, error) {
	if !p.consume("\"") {
		return nil, fmt.Errorf("expected '\"' at position %d", p.pos)
	}

	var sb strings.Builder
	for p.pos < len(p.input) && p.input[p.pos] != '"' {
		if p.input[p.pos] == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			switch p.input[p.pos] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteByte(p.input[p.pos])
			}
		} else {
			sb.WriteByte(p.input[p.pos])
		}
		p.pos++
	}

	if !p.consume("\"") {
		return nil, fmt.Errorf("unterminated string literal at position %d", p.pos)
	}

	return GBNFLiteral{Value: sb.String()}, nil
}

func (p *gbnfParser) parseCharClass() (GBNFElement, error) {
	if !p.consume("[") {
		return nil, fmt.Errorf("expected '[' at position %d", p.pos)
	}

	negated := false
	if p.consume("^") {
		negated = true
	}

	var chars strings.Builder
	var ranges [][2]rune

	for p.pos < len(p.input) && p.input[p.pos] != ']' {
		ch, size := utf8.DecodeRuneInString(p.input[p.pos:])
		p.pos += size

		// Check for escape
		if ch == '\\' && p.pos < len(p.input) {
			nextCh, nextSize := utf8.DecodeRuneInString(p.input[p.pos:])
			p.pos += nextSize
			switch nextCh {
			case 'n':
				ch = '\n'
			case 't':
				ch = '\t'
			case 'r':
				ch = '\r'
			default:
				ch = nextCh
			}
		}

		// Check for range
		if p.pos < len(p.input) && p.input[p.pos] == '-' && p.pos+1 < len(p.input) && p.input[p.pos+1] != ']' {
			p.pos++ // consume -
			endCh, endSize := utf8.DecodeRuneInString(p.input[p.pos:])
			p.pos += endSize
			if endCh == '\\' && p.pos < len(p.input) {
				nextCh, nextSize := utf8.DecodeRuneInString(p.input[p.pos:])
				p.pos += nextSize
				switch nextCh {
				case 'n':
					endCh = '\n'
				case 't':
					endCh = '\t'
				case 'r':
					endCh = '\r'
				default:
					endCh = nextCh
				}
			}
			ranges = append(ranges, [2]rune{ch, endCh})
		} else {
			chars.WriteRune(ch)
		}
	}

	if !p.consume("]") {
		return nil, fmt.Errorf("unterminated character class at position %d", p.pos)
	}

	return GBNFCharClass{
		Chars:   chars.String(),
		Negated: negated,
		Ranges:  ranges,
	}, nil
}

func (p *gbnfParser) parseGroup() (GBNFElement, error) {
	if !p.consume("(") {
		return nil, fmt.Errorf("expected '(' at position %d", p.pos)
	}

	p.skipWhitespace()
	alternates, err := p.parseAlternates()
	if err != nil {
		return nil, err
	}
	p.skipWhitespace()

	if !p.consume(")") {
		return nil, fmt.Errorf("expected ')' at position %d", p.pos)
	}

	return GBNFGroup{Alternates: alternates}, nil
}

func (p *gbnfParser) parseRepetition(elem GBNFElement) GBNFElement {
	if p.pos >= len(p.input) {
		return elem
	}

	switch p.input[p.pos] {
	case '*':
		p.pos++
		return GBNFRepetition{Element: elem, Min: 0, Max: -1}
	case '+':
		p.pos++
		return GBNFRepetition{Element: elem, Min: 1, Max: -1}
	case '?':
		p.pos++
		return GBNFRepetition{Element: elem, Min: 0, Max: 1}
	}

	return elem
}

func (p *gbnfParser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) && isIdentChar(p.input[p.pos]) {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *gbnfParser) skipWhitespace() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *gbnfParser) consume(s string) bool {
	if strings.HasPrefix(p.input[p.pos:], s) {
		p.pos += len(s)
		return true
	}
	return false
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '-'
}

// Common GBNF grammars for convenience

// JSONGrammar is a GBNF grammar for JSON.
const JSONGrammar = `
root   ::= object | array
object ::= "{" ws (string ":" ws value ("," ws string ":" ws value)*)? "}"
array  ::= "[" ws (value ("," ws value)*)? "]"
value  ::= object | array | string | number | "true" | "false" | "null"
string ::= "\"" ([^"\\] | "\\" ["\\/bfnrt] | "\\u" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F])* "\""
number ::= "-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?
ws     ::= [ \t\n\r]*
`

// JSONObjectGrammar restricts JSON to only objects (not arrays).
const JSONObjectGrammar = `
root   ::= object
object ::= "{" ws (string ":" ws value ("," ws string ":" ws value)*)? "}"
array  ::= "[" ws (value ("," ws value)*)? "]"
value  ::= object | array | string | number | "true" | "false" | "null"
string ::= "\"" ([^"\\] | "\\" ["\\/bfnrt] | "\\u" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F])* "\""
number ::= "-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?
ws     ::= [ \t\n\r]*
`

// ToolCallGrammar is a GBNF grammar for tool call output.
const ToolCallGrammar = `
root       ::= "{" ws "\"name\"" ws ":" ws string ws "," ws "\"args\"" ws ":" ws object ws "}"
object     ::= "{" ws (string ws ":" ws value (ws "," ws string ws ":" ws value)*)? ws "}"
array      ::= "[" ws (value (ws "," ws value)*)? ws "]"
value      ::= object | array | string | number | "true" | "false" | "null"
string     ::= "\"" ([^"\\] | "\\" ["\\/bfnrt] | "\\u" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F])* "\""
number     ::= "-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)? ([eE] [+-]? [0-9]+)?
ws         ::= [ \t\n\r]*
`
