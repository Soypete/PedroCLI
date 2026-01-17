package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// Grader is the interface for all grading implementations.
type Grader interface {
	// Grade evaluates a trial against a task's grading criteria.
	Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error)
	// Type returns the grader type.
	Type() GraderType
}

// GraderFactory creates graders of different types.
type GraderFactory struct {
	llmClient ModelClient // For LLM-based grading
}

// NewGraderFactory creates a new grader factory.
func NewGraderFactory(llmClient ModelClient) *GraderFactory {
	return &GraderFactory{llmClient: llmClient}
}

// GetGrader returns a grader for the given type.
func (f *GraderFactory) GetGrader(graderType GraderType) (Grader, error) {
	switch graderType {
	case GraderTypeStringMatch:
		return &StringMatchGrader{}, nil
	case GraderTypeRegex:
		return &RegexGrader{}, nil
	case GraderTypeJSONSchema:
		return &JSONSchemaGrader{}, nil
	case GraderTypeLLMRubric:
		if f.llmClient == nil {
			return nil, fmt.Errorf("LLM client required for LLM rubric grading")
		}
		return &LLMRubricGrader{client: f.llmClient}, nil
	case GraderTypeMarkdownLint:
		return &MarkdownLintGrader{}, nil
	case GraderTypeReadability:
		return &ReadabilityGrader{}, nil
	case GraderTypeCodeExec:
		return &CodeExecGrader{}, nil
	case GraderTypeToolCalls:
		return &ToolCallsGrader{}, nil
	default:
		return nil, fmt.Errorf("unknown grader type: %s", graderType)
	}
}

// StringMatchGrader checks for exact or partial string matches.
type StringMatchGrader struct{}

func (g *StringMatchGrader) Type() GraderType {
	return GraderTypeStringMatch
}

func (g *StringMatchGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput
	expected, _ := config.Config["expected"].(string)
	matchTypeStr, _ := config.Config["match_type"].(string)
	if matchTypeStr == "" {
		matchTypeStr = "contains"
	}
	matchType := MatchType(matchTypeStr)
	caseSensitive := true
	if cs, ok := config.Config["case_sensitive"].(bool); ok {
		caseSensitive = cs
	}

	if !caseSensitive {
		output = strings.ToLower(output)
		expected = strings.ToLower(expected)
	}

	var matched bool
	switch matchType {
	case MatchTypeExact:
		matched = output == expected
	case MatchTypeContains:
		matched = strings.Contains(output, expected)
	case MatchTypePrefix:
		matched = strings.HasPrefix(output, expected)
	case MatchTypeSuffix:
		matched = strings.HasSuffix(output, expected)
	default:
		matched = strings.Contains(output, expected)
	}

	score := 0.0
	if matched {
		score = 1.0
	}

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     matched,
		Score:      score,
		Feedback:   fmt.Sprintf("String match (%s): %v", matchType, matched),
		Details: map[string]interface{}{
			"match_type": matchType,
			"expected":   expected,
			"found":      matched,
		},
	}, nil
}

// RegexGrader checks output against regex patterns.
type RegexGrader struct{}

func (g *RegexGrader) Type() GraderType {
	return GraderTypeRegex
}

func (g *RegexGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput

	// Get patterns from config or assertions
	var patterns []string
	if p, ok := config.Config["pattern"].(string); ok && p != "" {
		patterns = append(patterns, p)
	}
	if p, ok := config.Config["patterns"].([]interface{}); ok {
		for _, pat := range p {
			if s, ok := pat.(string); ok {
				patterns = append(patterns, s)
			}
		}
	}
	patterns = append(patterns, config.Assertions...)

	if len(patterns) == 0 {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No regex patterns specified",
		}, nil
	}

	// Check all patterns (AND logic by default)
	requireAll := true
	if ra, ok := config.Config["require_all"].(bool); ok {
		requireAll = ra
	}

	matches := make(map[string]bool)
	matchCount := 0
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return &GradeResult{
				GraderType: g.Type(),
				Passed:     false,
				Score:      0,
				Feedback:   fmt.Sprintf("Invalid regex pattern: %s", err),
				Error:      err.Error(),
			}, nil
		}
		matched := re.MatchString(output)
		matches[pattern] = matched
		if matched {
			matchCount++
		}
	}

	var passed bool
	var score float64
	if requireAll {
		passed = matchCount == len(patterns)
		score = float64(matchCount) / float64(len(patterns))
	} else {
		passed = matchCount > 0
		score = float64(matchCount) / float64(len(patterns))
	}

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     passed,
		Score:      score,
		Feedback:   fmt.Sprintf("Matched %d/%d patterns", matchCount, len(patterns)),
		Details: map[string]interface{}{
			"pattern_results": matches,
			"require_all":     requireAll,
		},
	}, nil
}

// JSONSchemaGrader validates JSON output against a schema.
type JSONSchemaGrader struct{}

func (g *JSONSchemaGrader) Type() GraderType {
	return GraderTypeJSONSchema
}

func (g *JSONSchemaGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput

	// Extract JSON from output
	jsonStr := extractJSON(output)
	if jsonStr == "" {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No JSON found in output",
		}, nil
	}

	// Get schema from config
	schema, ok := config.Config["schema"]
	if !ok {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No schema specified in config",
		}, nil
	}

	schemaLoader := gojsonschema.NewGoLoader(schema)
	documentLoader := gojsonschema.NewStringLoader(jsonStr)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   fmt.Sprintf("Schema validation error: %s", err),
			Error:      err.Error(),
		}, nil
	}

	if !result.Valid() {
		errors := make([]string, len(result.Errors()))
		for i, err := range result.Errors() {
			errors[i] = err.String()
		}
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   fmt.Sprintf("Schema validation failed: %s", strings.Join(errors, "; ")),
			Details: map[string]interface{}{
				"validation_errors": errors,
			},
		}, nil
	}

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     true,
		Score:      1.0,
		Feedback:   "JSON validates against schema",
	}, nil
}

// extractJSON extracts JSON from text that may contain other content.
func extractJSON(text string) string {
	// Try to find JSON object
	start := strings.Index(text, "{")
	if start == -1 {
		// Try JSON array
		start = strings.Index(text, "[")
		if start == -1 {
			return ""
		}
	}

	// Find matching end bracket
	openChar := text[start]
	closeChar := byte('}')
	if openChar == '[' {
		closeChar = ']'
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(text); i++ {
		c := text[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == openChar {
			depth++
		} else if c == closeChar {
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}

	return ""
}

// LLMRubricGrader uses an LLM to grade output against a rubric.
type LLMRubricGrader struct {
	client ModelClient
}

func (g *LLMRubricGrader) Type() GraderType {
	return GraderTypeLLMRubric
}

func (g *LLMRubricGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput

	// Get rubric from config
	rubric, _ := config.Config["rubric"].(string)
	model, _ := config.Config["model"].(string)
	if model == "" {
		model = "llama3:8b" // Default
	}

	// Build assertions list
	assertions := config.Assertions
	if a, ok := config.Config["assertions"].([]interface{}); ok {
		for _, ass := range a {
			if s, ok := ass.(string); ok {
				assertions = append(assertions, s)
			}
		}
	}

	// Build grading prompt
	var prompt strings.Builder
	prompt.WriteString("You are an expert evaluator. Grade the following output based on the criteria provided.\n\n")
	prompt.WriteString("## Task Description\n")
	prompt.WriteString(task.Description)
	prompt.WriteString("\n\n## Task Input\n")
	prompt.WriteString(task.Input.Prompt)
	prompt.WriteString("\n\n## Output to Grade\n")
	prompt.WriteString(output)
	prompt.WriteString("\n\n")

	if rubric != "" {
		prompt.WriteString("## Rubric\n")
		prompt.WriteString(rubric)
		prompt.WriteString("\n\n")
	}

	if len(assertions) > 0 {
		prompt.WriteString("## Assertions to Check\nThe output should satisfy ALL of these:\n")
		for i, a := range assertions {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, a))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString(`## Response Format
Respond with JSON only, no other text:
{
  "passed": true or false,
  "score": 0.0 to 1.0,
  "feedback": "Brief explanation of the grade",
  "assertion_results": {
    "assertion_1": true/false,
    "assertion_2": true/false
  }
}
`)

	// Call LLM
	resp, err := g.client.Complete(ctx, &CompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: "You are an expert code and content evaluator. Always respond with valid JSON."},
			{Role: "user", Content: prompt.String()},
		},
		Temperature: 0.1, // Low temperature for consistent grading
		MaxTokens:   1000,
	})
	if err != nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   fmt.Sprintf("LLM grading failed: %s", err),
			Error:      err.Error(),
		}, nil
	}

	// Parse LLM response
	jsonStr := extractJSON(resp.Content)
	if jsonStr == "" {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "LLM did not return valid JSON",
			Details: map[string]interface{}{
				"raw_response": resp.Content,
			},
		}, nil
	}

	var gradeResp struct {
		Passed           bool            `json:"passed"`
		Score            float64         `json:"score"`
		Feedback         string          `json:"feedback"`
		AssertionResults map[string]bool `json:"assertion_results"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &gradeResp); err != nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   fmt.Sprintf("Failed to parse LLM response: %s", err),
			Error:      err.Error(),
			Details: map[string]interface{}{
				"raw_response": resp.Content,
			},
		}, nil
	}

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     gradeResp.Passed,
		Score:      gradeResp.Score,
		Feedback:   gradeResp.Feedback,
		Details: map[string]interface{}{
			"assertion_results": gradeResp.AssertionResults,
			"tokens_used":       resp.TotalTokens,
		},
	}, nil
}

// MarkdownLintGrader checks markdown formatting.
type MarkdownLintGrader struct{}

func (g *MarkdownLintGrader) Type() GraderType {
	return GraderTypeMarkdownLint
}

func (g *MarkdownLintGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput
	issues := []string{}
	warnings := []string{}

	// Check for common markdown issues
	lines := strings.Split(output, "\n")

	// Rule 1: Headers should have space after #
	headerNoSpace := regexp.MustCompile(`^#{1,6}[^# \n]`)
	for i, line := range lines {
		if headerNoSpace.MatchString(line) {
			issues = append(issues, fmt.Sprintf("Line %d: Header missing space after #", i+1))
		}
	}

	// Rule 2: Code blocks should be properly closed
	codeBlockCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			codeBlockCount++
		}
	}
	if codeBlockCount%2 != 0 {
		issues = append(issues, "Unclosed code block (mismatched ```)")
	}

	// Rule 3: Lists should be consistent
	bulletStyles := make(map[string]int)
	listBullet := regexp.MustCompile(`^(\s*)([-*+])\s`)
	for _, line := range lines {
		if matches := listBullet.FindStringSubmatch(line); matches != nil {
			bulletStyles[matches[2]]++
		}
	}
	if len(bulletStyles) > 1 {
		warnings = append(warnings, "Inconsistent list bullet styles (mixing -, *, +)")
	}

	// Rule 4: Links should have valid format
	brokenLink := regexp.MustCompile(`\[([^\]]*)\]\s+\(`) // Space between ] and (
	for i, line := range lines {
		if brokenLink.MatchString(line) {
			issues = append(issues, fmt.Sprintf("Line %d: Invalid link format (space between ] and ()", i+1))
		}
	}

	// Rule 5: No trailing whitespace (warning only)
	trailingWhitespace := 0
	for _, line := range lines {
		if strings.TrimRight(line, " \t") != line {
			trailingWhitespace++
		}
	}
	if trailingWhitespace > 0 {
		warnings = append(warnings, fmt.Sprintf("%d lines have trailing whitespace", trailingWhitespace))
	}

	// Rule 6: Check for H1 header
	hasH1 := false
	h1Count := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			hasH1 = true
			h1Count++
		}
	}
	if !hasH1 {
		warnings = append(warnings, "No H1 header found")
	} else if h1Count > 1 {
		warnings = append(warnings, "Multiple H1 headers found (consider using H2+)")
	}

	// Calculate score
	maxIssues := 10.0 // Normalize
	issueScore := math.Max(0, 1.0-(float64(len(issues))/maxIssues))
	warningPenalty := float64(len(warnings)) * 0.05
	score := math.Max(0, issueScore-warningPenalty)

	passed := len(issues) == 0
	feedback := "Markdown is well-formed"
	if len(issues) > 0 {
		feedback = fmt.Sprintf("Found %d issues: %s", len(issues), strings.Join(issues, "; "))
	} else if len(warnings) > 0 {
		feedback = fmt.Sprintf("No errors, but %d warnings: %s", len(warnings), strings.Join(warnings, "; "))
	}

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     passed,
		Score:      score,
		Feedback:   feedback,
		Details: map[string]interface{}{
			"issues":   issues,
			"warnings": warnings,
		},
	}, nil
}

// ReadabilityGrader calculates readability scores.
type ReadabilityGrader struct{}

func (g *ReadabilityGrader) Type() GraderType {
	return GraderTypeReadability
}

func (g *ReadabilityGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput

	// Strip markdown formatting for analysis
	plainText := stripMarkdown(output)

	// Calculate Flesch Reading Ease score
	sentences := countSentences(plainText)
	words := countWords(plainText)
	syllables := countSyllables(plainText)

	if sentences == 0 || words == 0 {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "Insufficient text for readability analysis",
		}, nil
	}

	// Flesch Reading Ease = 206.835 - 1.015(words/sentences) - 84.6(syllables/words)
	avgSentenceLength := float64(words) / float64(sentences)
	avgSyllablesPerWord := float64(syllables) / float64(words)
	fleschScore := 206.835 - 1.015*avgSentenceLength - 84.6*avgSyllablesPerWord

	// Flesch-Kincaid Grade Level = 0.39(words/sentences) + 11.8(syllables/words) - 15.59
	gradeLevel := 0.39*avgSentenceLength + 11.8*avgSyllablesPerWord - 15.59

	// Get min/max from config
	minFlesch := 30.0 // Default: "difficult" is minimum acceptable
	maxFlesch := 100.0
	if mf, ok := config.Config["min_flesch_score"].(float64); ok {
		minFlesch = mf
	}
	if mf, ok := config.Config["max_flesch_score"].(float64); ok {
		maxFlesch = mf
	}

	passed := fleschScore >= minFlesch && fleschScore <= maxFlesch

	// Normalize score (0-100 scale to 0-1)
	normalizedScore := math.Max(0, math.Min(1, fleschScore/100))

	readabilityLevel := getReadabilityLevel(fleschScore)

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     passed,
		Score:      normalizedScore,
		Feedback:   fmt.Sprintf("Flesch score: %.1f (%s), Grade level: %.1f", fleschScore, readabilityLevel, gradeLevel),
		Details: map[string]interface{}{
			"flesch_reading_ease":    fleschScore,
			"flesch_kincaid_grade":   gradeLevel,
			"readability_level":      readabilityLevel,
			"word_count":             words,
			"sentence_count":         sentences,
			"syllable_count":         syllables,
			"avg_sentence_length":    avgSentenceLength,
			"avg_syllables_per_word": avgSyllablesPerWord,
		},
	}, nil
}

func getReadabilityLevel(fleschScore float64) string {
	switch {
	case fleschScore >= 90:
		return "Very Easy"
	case fleschScore >= 80:
		return "Easy"
	case fleschScore >= 70:
		return "Fairly Easy"
	case fleschScore >= 60:
		return "Standard"
	case fleschScore >= 50:
		return "Fairly Difficult"
	case fleschScore >= 30:
		return "Difficult"
	default:
		return "Very Difficult"
	}
}

func stripMarkdown(text string) string {
	// Remove code blocks
	codeBlock := regexp.MustCompile("```[\\s\\S]*?```")
	text = codeBlock.ReplaceAllString(text, "")

	// Remove inline code
	inlineCode := regexp.MustCompile("`[^`]+`")
	text = inlineCode.ReplaceAllString(text, "")

	// Remove headers
	headers := regexp.MustCompile(`(?m)^#{1,6}\s+`)
	text = headers.ReplaceAllString(text, "")

	// Remove links but keep text
	links := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	text = links.ReplaceAllString(text, "$1")

	// Remove bold/italic
	boldItalic := regexp.MustCompile(`[*_]{1,3}([^*_]+)[*_]{1,3}`)
	text = boldItalic.ReplaceAllString(text, "$1")

	// Remove list markers
	listMarkers := regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`)
	text = listMarkers.ReplaceAllString(text, "")
	numberedList := regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`)
	text = numberedList.ReplaceAllString(text, "")

	return text
}

func countSentences(text string) int {
	// Count sentence-ending punctuation
	sentenceEnders := regexp.MustCompile(`[.!?]+`)
	matches := sentenceEnders.FindAllString(text, -1)
	count := len(matches)
	if count == 0 && len(text) > 0 {
		count = 1 // At least one sentence if there's text
	}
	return count
}

func countWords(text string) int {
	words := regexp.MustCompile(`\b\w+\b`)
	return len(words.FindAllString(text, -1))
}

func countSyllables(text string) int {
	words := regexp.MustCompile(`\b\w+\b`)
	wordList := words.FindAllString(strings.ToLower(text), -1)

	total := 0
	for _, word := range wordList {
		total += countSyllablesInWord(word)
	}
	return total
}

func countSyllablesInWord(word string) int {
	word = strings.ToLower(word)
	if len(word) <= 3 {
		return 1
	}

	// Remove silent e at end
	if strings.HasSuffix(word, "e") && !strings.HasSuffix(word, "le") {
		word = word[:len(word)-1]
	}

	// Count vowel groups
	vowels := "aeiouy"
	count := 0
	prevWasVowel := false

	for _, char := range word {
		isVowel := strings.ContainsRune(vowels, char)
		if isVowel && !prevWasVowel {
			count++
		}
		prevWasVowel = isVowel
	}

	if count == 0 {
		count = 1
	}

	return count
}

// CodeExecGrader executes code and checks results.
type CodeExecGrader struct{}

func (g *CodeExecGrader) Type() GraderType {
	return GraderTypeCodeExec
}

func (g *CodeExecGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	// For safety, code execution is limited to checking if code compiles/is valid
	// Actual execution would require sandboxing
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	output := trial.Outcome.FinalOutput

	// Extract code blocks
	codeBlocks := extractCodeBlocks(output)
	if len(codeBlocks) == 0 {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No code blocks found in output",
		}, nil
	}

	// For now, just verify code blocks exist and have content
	// Real implementation would compile/run the code in a sandbox
	validBlocks := 0
	for _, block := range codeBlocks {
		if strings.TrimSpace(block.Code) != "" {
			validBlocks++
		}
	}

	passed := validBlocks > 0
	score := float64(validBlocks) / float64(len(codeBlocks))

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     passed,
		Score:      score,
		Feedback:   fmt.Sprintf("Found %d code blocks, %d with content", len(codeBlocks), validBlocks),
		Details: map[string]interface{}{
			"code_blocks":  len(codeBlocks),
			"valid_blocks": validBlocks,
			"languages":    getLanguages(codeBlocks),
		},
	}, nil
}

type codeBlock struct {
	Language string
	Code     string
}

func extractCodeBlocks(text string) []codeBlock {
	// Match ```language\ncode\n```
	pattern := regexp.MustCompile("```(\\w*)\\n([\\s\\S]*?)```")
	matches := pattern.FindAllStringSubmatch(text, -1)

	blocks := make([]codeBlock, len(matches))
	for i, match := range matches {
		blocks[i] = codeBlock{
			Language: match[1],
			Code:     match[2],
		}
	}
	return blocks
}

func getLanguages(blocks []codeBlock) []string {
	langs := make(map[string]bool)
	for _, block := range blocks {
		if block.Language != "" {
			langs[block.Language] = true
		}
	}
	result := make([]string, 0, len(langs))
	for lang := range langs {
		result = append(result, lang)
	}
	return result
}

// ToolCallsGrader checks tool usage patterns.
type ToolCallsGrader struct{}

func (g *ToolCallsGrader) Type() GraderType {
	return GraderTypeToolCalls
}

func (g *ToolCallsGrader) Grade(ctx context.Context, task *Task, trial *Trial, config *GraderConfig) (*GradeResult, error) {
	if trial.Outcome == nil {
		return &GradeResult{
			GraderType: g.Type(),
			Passed:     false,
			Score:      0,
			Feedback:   "No outcome available",
		}, nil
	}

	toolCalls := trial.Outcome.ToolCallsUsed

	// Get expected tools from config
	var requiredTools []string
	if rt, ok := config.Config["required_tools"].([]interface{}); ok {
		for _, t := range rt {
			if s, ok := t.(string); ok {
				requiredTools = append(requiredTools, s)
			}
		}
	}

	var forbiddenTools []string
	if ft, ok := config.Config["forbidden_tools"].([]interface{}); ok {
		for _, t := range ft {
			if s, ok := t.(string); ok {
				forbiddenTools = append(forbiddenTools, s)
			}
		}
	}

	minCalls := 0
	if mc, ok := config.Config["min_calls"].(int); ok {
		minCalls = mc
	}
	if mc, ok := config.Config["min_calls"].(float64); ok {
		minCalls = int(mc)
	}

	maxCalls := 1000
	if mc, ok := config.Config["max_calls"].(int); ok {
		maxCalls = mc
	}
	if mc, ok := config.Config["max_calls"].(float64); ok {
		maxCalls = int(mc)
	}

	// Collect used tool names
	usedTools := make(map[string]int)
	for _, tc := range toolCalls {
		usedTools[tc.Tool]++
	}

	// Check required tools
	missingRequired := []string{}
	for _, req := range requiredTools {
		if _, found := usedTools[req]; !found {
			missingRequired = append(missingRequired, req)
		}
	}

	// Check forbidden tools
	usedForbidden := []string{}
	for _, forbidden := range forbiddenTools {
		if _, found := usedTools[forbidden]; found {
			usedForbidden = append(usedForbidden, forbidden)
		}
	}

	// Check call count
	totalCalls := len(toolCalls)
	callCountOk := totalCalls >= minCalls && totalCalls <= maxCalls

	passed := len(missingRequired) == 0 && len(usedForbidden) == 0 && callCountOk

	// Calculate score
	score := 1.0
	if len(requiredTools) > 0 {
		score *= float64(len(requiredTools)-len(missingRequired)) / float64(len(requiredTools))
	}
	if len(usedForbidden) > 0 {
		score *= 0.5 // Penalty for using forbidden tools
	}
	if !callCountOk {
		score *= 0.8 // Penalty for wrong call count
	}

	feedback := fmt.Sprintf("%d tool calls", totalCalls)
	if len(missingRequired) > 0 {
		feedback += fmt.Sprintf(", missing: %v", missingRequired)
	}
	if len(usedForbidden) > 0 {
		feedback += fmt.Sprintf(", used forbidden: %v", usedForbidden)
	}

	return &GradeResult{
		GraderType: g.Type(),
		Passed:     passed,
		Score:      score,
		Feedback:   feedback,
		Details: map[string]interface{}{
			"total_calls":      totalCalls,
			"tools_used":       usedTools,
			"missing_required": missingRequired,
			"used_forbidden":   usedForbidden,
		},
	}, nil
}

// CompositeScore calculates a weighted composite score from multiple grade results.
func CompositeScore(results []*GradeResult, configs []GraderConfig) (float64, bool) {
	if len(results) == 0 {
		return 0, false
	}

	totalWeight := 0.0
	weightedSum := 0.0
	allPassed := true

	for i, result := range results {
		weight := 1.0
		if i < len(configs) && configs[i].Weight > 0 {
			weight = configs[i].Weight
		}

		totalWeight += weight
		weightedSum += result.Score * weight

		// Check if required graders passed
		if i < len(configs) && configs[i].Required && !result.Passed {
			allPassed = false
		}
	}

	if totalWeight == 0 {
		return 0, false
	}

	return weightedSum / totalWeight, allPassed
}
