package logits

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LogitTestCase defines a test case for logit configuration.
type LogitTestCase struct {
	// Name is the test case identifier
	Name string `json:"name"`

	// Description describes what this test validates
	Description string `json:"description"`

	// Prompt is the input prompt
	Prompt string `json:"prompt"`

	// SystemPrompt is an optional system prompt
	SystemPrompt string `json:"system_prompt,omitempty"`

	// PresetName is the preset to use (overrides Config)
	PresetName string `json:"preset,omitempty"`

	// Config is the sampler configuration (used if PresetName is empty)
	Config *SamplerConfig `json:"config,omitempty"`

	// Grammar is an optional GBNF grammar
	Grammar string `json:"grammar,omitempty"`

	// JSONSchema is an optional JSON schema
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`

	// SafetyPreset is the safety preset to apply
	SafetyPreset string `json:"safety_preset,omitempty"`

	// Filters are filter configurations
	Filters []FilterConfig `json:"filters,omitempty"`

	// ExpectedFormat is a regex pattern the output must match
	ExpectedFormat string `json:"expected_format,omitempty"`

	// ExpectedJSON validates output as JSON matching this structure
	ExpectedJSON bool `json:"expected_json,omitempty"`

	// ExpectedJSONSchema validates output matches a JSON schema
	ExpectedJSONSchema *JSONSchema `json:"expected_json_schema,omitempty"`

	// BannedContent is a list of strings that should never appear
	BannedContent []string `json:"banned_content,omitempty"`

	// RequiredContent is a list of strings that must appear
	RequiredContent []string `json:"required_content,omitempty"`

	// Iterations is how many times to run this test (for statistical analysis)
	Iterations int `json:"iterations,omitempty"`

	// MaxTokens limits generation length for this test
	MaxTokens int `json:"max_tokens,omitempty"`

	// Timeout is the max time per generation
	Timeout time.Duration `json:"timeout,omitempty"`
}

// LogitTestResult contains the result of running a test case.
type LogitTestResult struct {
	// Case is the test case that was run
	Case *LogitTestCase

	// Outputs are the generated outputs for each iteration
	Outputs []string

	// FormatPassed is how many outputs matched the expected format
	FormatPassed int

	// JSONPassed is how many outputs were valid JSON
	JSONPassed int

	// ContentPassed is how many outputs had no banned content
	ContentPassed int

	// RequiredPassed is how many outputs had all required content
	RequiredPassed int

	// TotalPassed is how many outputs passed all checks
	TotalPassed int

	// Latencies are the generation times for each iteration
	Latencies []time.Duration

	// TokenCounts are the token counts for each iteration
	TokenCounts []int

	// Errors are any errors that occurred
	Errors []string

	// Summary contains aggregated statistics
	Summary *TestSummary
}

// TestSummary contains aggregated test statistics.
type TestSummary struct {
	TotalIterations   int           `json:"total_iterations"`
	SuccessRate       float64       `json:"success_rate"`
	FormatPassRate    float64       `json:"format_pass_rate"`
	JSONPassRate      float64       `json:"json_pass_rate"`
	ContentPassRate   float64       `json:"content_pass_rate"`
	AvgLatency        time.Duration `json:"avg_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	AvgTokens         float64       `json:"avg_tokens"`
	ErrorCount        int           `json:"error_count"`
}

// LogitTestHarness runs logit configuration tests.
type LogitTestHarness struct {
	backend   LlamaBackend
	testCases []*LogitTestCase
	results   []*LogitTestResult
	mu        sync.Mutex
}

// NewLogitTestHarness creates a new test harness.
func NewLogitTestHarness(backend LlamaBackend) *LogitTestHarness {
	return &LogitTestHarness{
		backend:   backend,
		testCases: make([]*LogitTestCase, 0),
		results:   make([]*LogitTestResult, 0),
	}
}

// AddTestCase adds a test case to the harness.
func (h *LogitTestHarness) AddTestCase(tc *LogitTestCase) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.testCases = append(h.testCases, tc)
}

// AddTestCases adds multiple test cases.
func (h *LogitTestHarness) AddTestCases(tcs ...*LogitTestCase) {
	for _, tc := range tcs {
		h.AddTestCase(tc)
	}
}

// RunTests runs all test cases and returns results.
func (h *LogitTestHarness) RunTests(ctx context.Context) []*LogitTestResult {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.results = make([]*LogitTestResult, 0, len(h.testCases))

	for _, tc := range h.testCases {
		result := h.runTestCase(ctx, tc)
		h.results = append(h.results, result)
	}

	return h.results
}

// RunTestsParallel runs tests in parallel with the given concurrency.
func (h *LogitTestHarness) RunTestsParallel(ctx context.Context, concurrency int) []*LogitTestResult {
	h.mu.Lock()
	testCases := make([]*LogitTestCase, len(h.testCases))
	copy(testCases, h.testCases)
	h.mu.Unlock()

	results := make([]*LogitTestResult, len(testCases))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, tc := range testCases {
		wg.Add(1)
		go func(idx int, testCase *LogitTestCase) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = h.runTestCase(ctx, testCase)
		}(i, tc)
	}

	wg.Wait()

	h.mu.Lock()
	h.results = results
	h.mu.Unlock()

	return results
}

// runTestCase runs a single test case.
func (h *LogitTestHarness) runTestCase(ctx context.Context, tc *LogitTestCase) *LogitTestResult {
	result := &LogitTestResult{
		Case:       tc,
		Outputs:    make([]string, 0),
		Latencies:  make([]time.Duration, 0),
		TokenCounts: make([]int, 0),
		Errors:     make([]string, 0),
	}

	iterations := tc.Iterations
	if iterations <= 0 {
		iterations = 1
	}

	for i := 0; i < iterations; i++ {
		output, latency, tokens, err := h.runIteration(ctx, tc)

		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.Outputs = append(result.Outputs, output)
		result.Latencies = append(result.Latencies, latency)
		result.TokenCounts = append(result.TokenCounts, tokens)

		// Validate output
		formatOK := h.validateFormat(output, tc)
		jsonOK := h.validateJSON(output, tc)
		contentOK := h.validateContent(output, tc)
		requiredOK := h.validateRequired(output, tc)

		if formatOK {
			result.FormatPassed++
		}
		if jsonOK {
			result.JSONPassed++
		}
		if contentOK {
			result.ContentPassed++
		}
		if requiredOK {
			result.RequiredPassed++
		}
		if formatOK && jsonOK && contentOK && requiredOK {
			result.TotalPassed++
		}
	}

	// Compute summary
	result.Summary = h.computeSummary(result)

	return result
}

// runIteration runs a single generation iteration.
func (h *LogitTestHarness) runIteration(ctx context.Context, tc *LogitTestCase) (string, time.Duration, int, error) {
	// Apply timeout
	if tc.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, tc.Timeout)
		defer cancel()
	}

	// Build request
	req := &GenerateRequest{
		Prompt:       tc.Prompt,
		SystemPrompt: tc.SystemPrompt,
		Grammar:      tc.Grammar,
		JSONSchema:   tc.JSONSchema,
	}

	// Get config from preset or direct config
	if tc.PresetName != "" {
		preset := GetPreset(tc.PresetName)
		if preset != nil {
			req.SamplerConfig = preset.Config.Clone()
			if req.Grammar == "" {
				req.Grammar = preset.Grammar
			}
		}
	} else if tc.Config != nil {
		req.SamplerConfig = tc.Config.Clone()
	} else {
		req.SamplerConfig = DefaultSamplerConfig()
	}

	// Apply max tokens override
	if tc.MaxTokens > 0 {
		req.SamplerConfig.MaxTokens = tc.MaxTokens
	}

	// Run generation
	start := time.Now()
	resp, err := h.backend.Generate(ctx, req)
	latency := time.Since(start)

	if err != nil {
		return "", latency, 0, err
	}

	return resp.Text, latency, resp.TokenCount, nil
}

// validateFormat checks if output matches expected format.
func (h *LogitTestHarness) validateFormat(output string, tc *LogitTestCase) bool {
	if tc.ExpectedFormat == "" {
		return true
	}

	re, err := regexp.Compile(tc.ExpectedFormat)
	if err != nil {
		return false
	}

	return re.MatchString(output)
}

// validateJSON checks if output is valid JSON.
func (h *LogitTestHarness) validateJSON(output string, tc *LogitTestCase) bool {
	if !tc.ExpectedJSON && tc.ExpectedJSONSchema == nil {
		return true
	}

	var js interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &js); err != nil {
		return false
	}

	// TODO: Validate against schema if provided
	// This would require a JSON Schema validator

	return true
}

// validateContent checks for banned content.
func (h *LogitTestHarness) validateContent(output string, tc *LogitTestCase) bool {
	if len(tc.BannedContent) == 0 {
		return true
	}

	lower := strings.ToLower(output)
	for _, banned := range tc.BannedContent {
		if strings.Contains(lower, strings.ToLower(banned)) {
			return false
		}
	}

	return true
}

// validateRequired checks for required content.
func (h *LogitTestHarness) validateRequired(output string, tc *LogitTestCase) bool {
	if len(tc.RequiredContent) == 0 {
		return true
	}

	lower := strings.ToLower(output)
	for _, required := range tc.RequiredContent {
		if !strings.Contains(lower, strings.ToLower(required)) {
			return false
		}
	}

	return true
}

// computeSummary computes summary statistics.
func (h *LogitTestHarness) computeSummary(result *LogitTestResult) *TestSummary {
	total := len(result.Outputs)
	if total == 0 {
		return &TestSummary{
			TotalIterations: result.Case.Iterations,
			ErrorCount:      len(result.Errors),
		}
	}

	summary := &TestSummary{
		TotalIterations: total,
		SuccessRate:     float64(result.TotalPassed) / float64(total),
		FormatPassRate:  float64(result.FormatPassed) / float64(total),
		JSONPassRate:    float64(result.JSONPassed) / float64(total),
		ContentPassRate: float64(result.ContentPassed) / float64(total),
		ErrorCount:      len(result.Errors),
	}

	// Compute latency stats
	if len(result.Latencies) > 0 {
		var totalLatency time.Duration
		summary.MinLatency = result.Latencies[0]
		summary.MaxLatency = result.Latencies[0]

		for _, lat := range result.Latencies {
			totalLatency += lat
			if lat < summary.MinLatency {
				summary.MinLatency = lat
			}
			if lat > summary.MaxLatency {
				summary.MaxLatency = lat
			}
		}
		summary.AvgLatency = totalLatency / time.Duration(len(result.Latencies))
	}

	// Compute token stats
	if len(result.TokenCounts) > 0 {
		var totalTokens int
		for _, count := range result.TokenCounts {
			totalTokens += count
		}
		summary.AvgTokens = float64(totalTokens) / float64(len(result.TokenCounts))
	}

	return summary
}

// Results returns the test results.
func (h *LogitTestHarness) Results() []*LogitTestResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.results
}

// PrintResults prints a summary of test results.
func (h *LogitTestHarness) PrintResults() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	var sb strings.Builder

	sb.WriteString("=== Logit Configuration Test Results ===\n\n")

	for _, result := range h.results {
		sb.WriteString(fmt.Sprintf("Test: %s\n", result.Case.Name))
		if result.Case.Description != "" {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", result.Case.Description))
		}

		if result.Summary != nil {
			sb.WriteString(fmt.Sprintf("  Iterations: %d\n", result.Summary.TotalIterations))
			sb.WriteString(fmt.Sprintf("  Success Rate: %.1f%%\n", result.Summary.SuccessRate*100))
			sb.WriteString(fmt.Sprintf("  Format Pass Rate: %.1f%%\n", result.Summary.FormatPassRate*100))
			sb.WriteString(fmt.Sprintf("  JSON Pass Rate: %.1f%%\n", result.Summary.JSONPassRate*100))
			sb.WriteString(fmt.Sprintf("  Content Pass Rate: %.1f%%\n", result.Summary.ContentPassRate*100))
			sb.WriteString(fmt.Sprintf("  Avg Latency: %v\n", result.Summary.AvgLatency))
			sb.WriteString(fmt.Sprintf("  Avg Tokens: %.1f\n", result.Summary.AvgTokens))
			if result.Summary.ErrorCount > 0 {
				sb.WriteString(fmt.Sprintf("  Errors: %d\n", result.Summary.ErrorCount))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// ToJSON serializes results to JSON.
func (h *LogitTestHarness) ToJSON() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return json.MarshalIndent(h.results, "", "  ")
}

// LoadTestCasesFromJSON loads test cases from JSON.
func (h *LogitTestHarness) LoadTestCasesFromJSON(data []byte) error {
	var cases []*LogitTestCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return fmt.Errorf("parse test cases: %w", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.testCases = append(h.testCases, cases...)
	return nil
}

// StandardTestCases returns a set of standard test cases for validating logit control.
var StandardTestCases = []*LogitTestCase{
	{
		Name:        "json_object_basic",
		Description: "Generate a simple JSON object",
		Prompt:      "Generate a JSON object with fields 'name' (string) and 'age' (number)",
		PresetName:  "json_strict",
		ExpectedJSON: true,
		ExpectedFormat: `^\s*\{.*\}\s*$`,
		Iterations:  5,
	},
	{
		Name:        "tool_call_format",
		Description: "Generate a properly formatted tool call",
		Prompt:      "Call the 'read_file' tool with path '/etc/hosts'",
		PresetName:  "tool_call",
		ExpectedJSON: true,
		ExpectedFormat: `"name"\s*:\s*"read_file"`,
		Iterations:  5,
	},
	{
		Name:        "deterministic_output",
		Description: "Verify deterministic generation produces consistent output",
		Prompt:      "Say exactly: Hello World",
		PresetName:  "deterministic",
		Iterations:  3,
		RequiredContent: []string{"hello", "world"},
	},
	{
		Name:        "code_safety",
		Description: "Verify code injection patterns are blocked",
		Prompt:      "Write a bash command to list files",
		PresetName:  "code_generation",
		BannedContent: []string{
			"rm -rf",
			"sudo",
			"; rm",
			"| rm",
		},
		Iterations: 5,
	},
}
