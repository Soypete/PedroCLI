package evals

import (
	"context"
	"testing"
)

func TestStringMatchGrader_ExactMatch(t *testing.T) {
	grader := &StringMatchGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "hello world",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeStringMatch,
		Config: map[string]interface{}{
			"expected":   "hello world",
			"match_type": "exact",
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail")
	}
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score)
	}
}

func TestStringMatchGrader_Contains(t *testing.T) {
	grader := &StringMatchGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "The answer is hello world!",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeStringMatch,
		Config: map[string]interface{}{
			"expected":   "hello world",
			"match_type": "contains",
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail")
	}
}

func TestStringMatchGrader_CaseInsensitive(t *testing.T) {
	grader := &StringMatchGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "HELLO WORLD",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeStringMatch,
		Config: map[string]interface{}{
			"expected":       "hello world",
			"match_type":     "contains",
			"case_sensitive": false,
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail")
	}
}

func TestStringMatchGrader_NoMatch(t *testing.T) {
	grader := &StringMatchGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "goodbye world",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeStringMatch,
		Config: map[string]interface{}{
			"expected":   "hello",
			"match_type": "contains",
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail, got pass")
	}
	if result.Score != 0.0 {
		t.Errorf("expected score 0.0, got %f", result.Score)
	}
}

func TestRegexGrader_SinglePattern(t *testing.T) {
	grader := &RegexGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "func ReverseString(s string) string { return s }",
		},
	}
	config := &GraderConfig{
		Type:       GraderTypeRegex,
		Assertions: []string{"func\\s+\\w+\\s*\\("},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Feedback)
	}
}

func TestRegexGrader_MultiplePatterns(t *testing.T) {
	grader := &RegexGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "func Add(a, b int) int { return a + b }",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeRegex,
		Assertions: []string{
			"func\\s+Add",
			"int",
			"return",
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Feedback)
	}
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", result.Score)
	}
}

func TestRegexGrader_PartialMatch(t *testing.T) {
	grader := &RegexGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "func Add(a, b int) int { return a + b }",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeRegex,
		Config: map[string]interface{}{
			"require_all": true,
		},
		Assertions: []string{
			"func\\s+Add",
			"string", // This won't match
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail, got pass")
	}
	if result.Score != 0.5 {
		t.Errorf("expected score 0.5, got %f", result.Score)
	}
}

func TestRegexGrader_InvalidPattern(t *testing.T) {
	grader := &RegexGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "some output",
		},
	}
	config := &GraderConfig{
		Type:       GraderTypeRegex,
		Assertions: []string{"[invalid("},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for invalid pattern")
	}
	if result.Error == "" {
		t.Errorf("expected error message for invalid pattern")
	}
}

func TestJSONSchemaGrader_ValidJSON(t *testing.T) {
	grader := &JSONSchemaGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: `{"name": "John", "age": 30}`,
		},
	}
	config := &GraderConfig{
		Type: GraderTypeJSONSchema,
		Config: map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "integer"},
				},
				"required": []string{"name", "age"},
			},
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Feedback)
	}
}

func TestJSONSchemaGrader_InvalidJSON(t *testing.T) {
	grader := &JSONSchemaGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: `{"name": "John", "age": "thirty"}`,
		},
	}
	config := &GraderConfig{
		Type: GraderTypeJSONSchema,
		Config: map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "integer"},
				},
			},
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for invalid type")
	}
}

func TestJSONSchemaGrader_ExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain json object",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json in text",
			input:    `Here is the response: {"name": "test"} end`,
			expected: `{"name": "test"}`,
		},
		{
			name:     "nested json",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "json array",
			input:    `[1, 2, 3]`,
			expected: `[1, 2, 3]`,
		},
		{
			name:     "no json",
			input:    `no json here`,
			expected: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSON(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMarkdownLintGrader_ValidMarkdown(t *testing.T) {
	grader := &MarkdownLintGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: `# Title

This is a paragraph.

## Section

- Item 1
- Item 2

` + "```go\nfunc main() {}\n```",
		},
	}
	config := &GraderConfig{Type: GraderTypeMarkdownLint}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Feedback)
	}
}

func TestMarkdownLintGrader_HeaderNoSpace(t *testing.T) {
	grader := &MarkdownLintGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: `#No space after hash

This is text.`,
		},
	}
	config := &GraderConfig{Type: GraderTypeMarkdownLint}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for header without space")
	}
}

func TestMarkdownLintGrader_UnclosedCodeBlock(t *testing.T) {
	grader := &MarkdownLintGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "# Title\n\n```go\nfunc main() {}\n",
		},
	}
	config := &GraderConfig{Type: GraderTypeMarkdownLint}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for unclosed code block")
	}
}

func TestReadabilityGrader_SimpleText(t *testing.T) {
	grader := &ReadabilityGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "The cat sat on the mat. The dog ran in the park. Birds fly in the sky. Fish swim in the sea.",
		},
	}
	config := &GraderConfig{
		Type: GraderTypeReadability,
		Config: map[string]interface{}{
			"min_flesch_score": 50.0,
			"max_flesch_score": 150.0, // Allow high scores for very simple text
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass for simple text, got: %s", result.Feedback)
	}

	details := result.Details
	if details == nil {
		t.Fatal("expected details in result")
	}
	if _, ok := details["flesch_reading_ease"]; !ok {
		t.Error("expected flesch_reading_ease in details")
	}
}

func TestReadabilityGrader_EmptyText(t *testing.T) {
	grader := &ReadabilityGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "",
		},
	}
	config := &GraderConfig{Type: GraderTypeReadability}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for empty text")
	}
}

func TestToolCallsGrader_WithRequiredTools(t *testing.T) {
	grader := &ToolCallsGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			ToolCallsUsed: []ToolCallRecord{
				{Tool: "file", Success: true},
				{Tool: "search", Success: true},
				{Tool: "git", Success: true},
			},
		},
	}
	config := &GraderConfig{
		Type: GraderTypeToolCalls,
		Config: map[string]interface{}{
			"required_tools": []interface{}{"file", "search"},
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Feedback)
	}
}

func TestToolCallsGrader_MissingRequired(t *testing.T) {
	grader := &ToolCallsGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			ToolCallsUsed: []ToolCallRecord{
				{Tool: "file", Success: true},
			},
		},
	}
	config := &GraderConfig{
		Type: GraderTypeToolCalls,
		Config: map[string]interface{}{
			"required_tools": []interface{}{"file", "search", "git"},
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for missing required tools")
	}
}

func TestToolCallsGrader_ForbiddenTools(t *testing.T) {
	grader := &ToolCallsGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			ToolCallsUsed: []ToolCallRecord{
				{Tool: "file", Success: true},
				{Tool: "bash", Success: true}, // forbidden
			},
		},
	}
	config := &GraderConfig{
		Type: GraderTypeToolCalls,
		Config: map[string]interface{}{
			"forbidden_tools": []interface{}{"bash", "rm"},
		},
	}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for using forbidden tool")
	}
}

func TestCodeExecGrader_WithCodeBlocks(t *testing.T) {
	grader := &CodeExecGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "Here is the code:\n```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```\n",
		},
	}
	config := &GraderConfig{Type: GraderTypeCodeExec}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected pass, got fail: %s", result.Feedback)
	}
}

func TestCodeExecGrader_NoCodeBlocks(t *testing.T) {
	grader := &CodeExecGrader{}
	task := &Task{ID: "test"}
	trial := &Trial{
		Outcome: &Outcome{
			FinalOutput: "This is just text without any code.",
		},
	}
	config := &GraderConfig{Type: GraderTypeCodeExec}

	result, err := grader.Grade(context.Background(), task, trial, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Errorf("expected fail for no code blocks")
	}
}

func TestCompositeScore(t *testing.T) {
	tests := []struct {
		name          string
		results       []*GradeResult
		configs       []GraderConfig
		expectedScore float64
		expectedPass  bool
	}{
		{
			name: "all pass equal weight",
			results: []*GradeResult{
				{Passed: true, Score: 1.0},
				{Passed: true, Score: 1.0},
			},
			configs: []GraderConfig{
				{Weight: 1.0},
				{Weight: 1.0},
			},
			expectedScore: 1.0,
			expectedPass:  true,
		},
		{
			name: "mixed results",
			results: []*GradeResult{
				{Passed: true, Score: 1.0},
				{Passed: false, Score: 0.0},
			},
			configs: []GraderConfig{
				{Weight: 1.0},
				{Weight: 1.0},
			},
			expectedScore: 0.5,
			expectedPass:  true, // No required graders
		},
		{
			name: "required fails",
			results: []*GradeResult{
				{Passed: true, Score: 1.0},
				{Passed: false, Score: 0.0},
			},
			configs: []GraderConfig{
				{Weight: 1.0},
				{Weight: 1.0, Required: true},
			},
			expectedScore: 0.5,
			expectedPass:  false, // Required grader failed
		},
		{
			name: "weighted scores",
			results: []*GradeResult{
				{Passed: true, Score: 1.0},
				{Passed: true, Score: 0.5},
			},
			configs: []GraderConfig{
				{Weight: 2.0},
				{Weight: 1.0},
			},
			expectedScore: (1.0*2.0 + 0.5*1.0) / 3.0, // ~0.833
			expectedPass:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, passed := CompositeScore(tt.results, tt.configs)
			if passed != tt.expectedPass {
				t.Errorf("CompositeScore() passed = %v, want %v", passed, tt.expectedPass)
			}
			// Allow small floating point differences
			if score < tt.expectedScore-0.01 || score > tt.expectedScore+0.01 {
				t.Errorf("CompositeScore() score = %v, want %v", score, tt.expectedScore)
			}
		})
	}
}

func TestGraderFactory(t *testing.T) {
	factory := NewGraderFactory(nil)

	tests := []struct {
		graderType GraderType
		expectErr  bool
	}{
		{GraderTypeStringMatch, false},
		{GraderTypeRegex, false},
		{GraderTypeJSONSchema, false},
		{GraderTypeMarkdownLint, false},
		{GraderTypeReadability, false},
		{GraderTypeCodeExec, false},
		{GraderTypeToolCalls, false},
		{GraderTypeLLMRubric, true}, // Requires client
		{GraderType("unknown"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.graderType), func(t *testing.T) {
			grader, err := factory.GetGrader(tt.graderType)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for grader type %s", tt.graderType)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if grader == nil {
					t.Error("expected non-nil grader")
				}
			}
		})
	}
}

func TestCountSyllables(t *testing.T) {
	tests := []struct {
		word     string
		expected int
	}{
		{"the", 1},
		{"hello", 2},
		{"beautiful", 3},
		{"programming", 3},
		{"a", 1},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			result := countSyllablesInWord(tt.word)
			// Allow +/- 1 for syllable counting variations
			if result < tt.expected-1 || result > tt.expected+1 {
				t.Errorf("countSyllablesInWord(%q) = %d, want approximately %d", tt.word, result, tt.expected)
			}
		})
	}
}

func TestStripMarkdown(t *testing.T) {
	input := `# Header

This is **bold** and *italic* text.

` + "```go\nfunc main() {}\n```" + `

[Link text](http://example.com)

- List item
`

	result := stripMarkdown(input)

	// Should not contain markdown syntax
	if contains(result, "```") {
		t.Error("result should not contain code fence")
	}
	if contains(result, "**") {
		t.Error("result should not contain bold markers")
	}
	if contains(result, "[Link") {
		t.Error("result should not contain link markdown")
	}

	// Should contain plain text
	if !contains(result, "bold") {
		t.Error("result should contain 'bold'")
	}
	if !contains(result, "Link text") {
		t.Error("result should contain link text")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
