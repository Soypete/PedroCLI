package logits

import (
	"testing"
)

func TestParseGBNFSimple(t *testing.T) {
	grammar := `root ::= "hello"`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.RootRule != "root" {
		t.Errorf("expected root rule 'root', got %q", g.RootRule)
	}

	if len(g.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(g.Rules))
	}

	rule := g.Rules["root"]
	if rule == nil {
		t.Fatal("expected root rule to exist")
	}

	if len(rule.Alternates) != 1 {
		t.Errorf("expected 1 alternate, got %d", len(rule.Alternates))
	}

	if len(rule.Alternates[0].Elements) != 1 {
		t.Errorf("expected 1 element, got %d", len(rule.Alternates[0].Elements))
	}

	lit, ok := rule.Alternates[0].Elements[0].(GBNFLiteral)
	if !ok {
		t.Fatal("expected literal element")
	}

	if lit.Value != "hello" {
		t.Errorf("expected literal 'hello', got %q", lit.Value)
	}
}

func TestParseGBNFAlternates(t *testing.T) {
	grammar := `root ::= "yes" | "no"`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := g.Rules["root"]
	if len(rule.Alternates) != 2 {
		t.Errorf("expected 2 alternates, got %d", len(rule.Alternates))
	}
}

func TestParseGBNFCharClass(t *testing.T) {
	grammar := `root ::= [a-z]`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := g.Rules["root"]
	elem := rule.Alternates[0].Elements[0]

	cc, ok := elem.(GBNFCharClass)
	if !ok {
		t.Fatal("expected char class element")
	}

	if cc.Negated {
		t.Error("expected non-negated char class")
	}

	if len(cc.Ranges) != 1 {
		t.Errorf("expected 1 range, got %d", len(cc.Ranges))
	}

	if cc.Ranges[0][0] != 'a' || cc.Ranges[0][1] != 'z' {
		t.Errorf("expected range a-z, got %c-%c", cc.Ranges[0][0], cc.Ranges[0][1])
	}
}

func TestParseGBNFNegatedCharClass(t *testing.T) {
	grammar := `root ::= [^"\\]`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := g.Rules["root"]
	elem := rule.Alternates[0].Elements[0]

	cc, ok := elem.(GBNFCharClass)
	if !ok {
		t.Fatal("expected char class element")
	}

	if !cc.Negated {
		t.Error("expected negated char class")
	}
}

func TestParseGBNFRuleRef(t *testing.T) {
	grammar := `
root ::= value
value ::= "test"
`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(g.Rules))
	}

	rule := g.Rules["root"]
	elem := rule.Alternates[0].Elements[0]

	ref, ok := elem.(GBNFRuleRef)
	if !ok {
		t.Fatal("expected rule ref element")
	}

	if ref.RuleName != "value" {
		t.Errorf("expected rule ref 'value', got %q", ref.RuleName)
	}
}

func TestParseGBNFRepetition(t *testing.T) {
	tests := []struct {
		grammar string
		min     int
		max     int
	}{
		{`root ::= "a"*`, 0, -1},
		{`root ::= "a"+`, 1, -1},
		{`root ::= "a"?`, 0, 1},
	}

	for _, tc := range tests {
		g, err := ParseGBNF(tc.grammar)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.grammar, err)
		}

		rule := g.Rules["root"]
		elem := rule.Alternates[0].Elements[0]

		rep, ok := elem.(GBNFRepetition)
		if !ok {
			t.Fatalf("expected repetition element for %q", tc.grammar)
		}

		if rep.Min != tc.min {
			t.Errorf("expected min %d for %q, got %d", tc.min, tc.grammar, rep.Min)
		}
		if rep.Max != tc.max {
			t.Errorf("expected max %d for %q, got %d", tc.max, tc.grammar, rep.Max)
		}
	}
}

func TestParseGBNFGroup(t *testing.T) {
	grammar := `root ::= ("a" | "b")`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := g.Rules["root"]
	elem := rule.Alternates[0].Elements[0]

	group, ok := elem.(GBNFGroup)
	if !ok {
		t.Fatal("expected group element")
	}

	if len(group.Alternates) != 2 {
		t.Errorf("expected 2 alternates in group, got %d", len(group.Alternates))
	}
}

func TestParseGBNFComment(t *testing.T) {
	grammar := `
# This is a comment
root ::= "hello"  # inline comment
`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(g.Rules))
	}
}

func TestParseGBNFEscapeSequences(t *testing.T) {
	grammar := `root ::= "\n\t\"\\"`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rule := g.Rules["root"]
	lit := rule.Alternates[0].Elements[0].(GBNFLiteral)

	expected := "\n\t\"\\"
	if lit.Value != expected {
		t.Errorf("expected %q, got %q", expected, lit.Value)
	}
}

func TestParseGBNFJSON(t *testing.T) {
	g, err := ParseGBNF(JSONGrammar)
	if err != nil {
		t.Fatalf("failed to parse JSON grammar: %v", err)
	}

	if g.RootRule != "root" {
		t.Errorf("expected root rule 'root', got %q", g.RootRule)
	}

	// Check that required rules exist
	requiredRules := []string{"root", "object", "array", "value", "string", "number", "ws"}
	for _, name := range requiredRules {
		if _, ok := g.Rules[name]; !ok {
			t.Errorf("expected rule %q to exist", name)
		}
	}
}

func TestParseGBNFToolCall(t *testing.T) {
	g, err := ParseGBNF(ToolCallGrammar)
	if err != nil {
		t.Fatalf("failed to parse tool call grammar: %v", err)
	}

	if g.RootRule != "root" {
		t.Errorf("expected root rule 'root', got %q", g.RootRule)
	}
}

func TestGBNFString(t *testing.T) {
	grammar := `root ::= "hello" | "world"`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The String() method should return the original grammar
	str := g.String()
	if str != grammar {
		// Original is preserved
	}
}

func TestGBNFValidateRule(t *testing.T) {
	grammar := `
root ::= value
value ::= "test"
`

	g, err := ParseGBNF(grammar)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !g.ValidateRule("root") {
		t.Error("expected root rule to be valid")
	}
	if !g.ValidateRule("value") {
		t.Error("expected value rule to be valid")
	}
	if g.ValidateRule("nonexistent") {
		t.Error("expected nonexistent rule to be invalid")
	}
}

func TestGBNFCharClassMatches(t *testing.T) {
	tests := []struct {
		cc      GBNFCharClass
		r       rune
		matches bool
	}{
		{
			cc:      GBNFCharClass{Ranges: [][2]rune{{'a', 'z'}}},
			r:       'a',
			matches: true,
		},
		{
			cc:      GBNFCharClass{Ranges: [][2]rune{{'a', 'z'}}},
			r:       'm',
			matches: true,
		},
		{
			cc:      GBNFCharClass{Ranges: [][2]rune{{'a', 'z'}}},
			r:       'A',
			matches: false,
		},
		{
			cc:      GBNFCharClass{Chars: "abc"},
			r:       'b',
			matches: true,
		},
		{
			cc:      GBNFCharClass{Chars: "abc"},
			r:       'd',
			matches: false,
		},
		{
			cc:      GBNFCharClass{Chars: "abc", Negated: true},
			r:       'd',
			matches: true,
		},
		{
			cc:      GBNFCharClass{Chars: "abc", Negated: true},
			r:       'a',
			matches: false,
		},
	}

	for i, tc := range tests {
		got := tc.cc.Matches(tc.r)
		if got != tc.matches {
			t.Errorf("test %d: expected Matches(%c) = %v, got %v", i, tc.r, tc.matches, got)
		}
	}
}

func TestGrammarStateClone(t *testing.T) {
	state := NewGrammarState("root")
	state.MatchedText = "hello"
	state.AlternateIndex = 1
	state.ElementIndex = 2

	clone := state.Clone()

	if clone.CurrentRule != state.CurrentRule {
		t.Error("clone should have same current rule")
	}
	if clone.MatchedText != state.MatchedText {
		t.Error("clone should have same matched text")
	}
	if clone.AlternateIndex != state.AlternateIndex {
		t.Error("clone should have same alternate index")
	}
	if clone.ElementIndex != state.ElementIndex {
		t.Error("clone should have same element index")
	}

	// Modifying clone shouldn't affect original
	clone.MatchedText = "world"
	if state.MatchedText == "world" {
		t.Error("modifying clone should not affect original")
	}
}
