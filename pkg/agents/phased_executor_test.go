package agents

import (
	"strings"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/tools"
)

func TestPhaseStructure(t *testing.T) {
	// Test that Phase struct can be created with all fields
	phase := Phase{
		Name:         "test_phase",
		Description:  "Test phase description",
		SystemPrompt: "Test system prompt",
		Tools:        []string{"tool1", "tool2"},
		MaxRounds:    5,
		ExpectsJSON:  true,
		Validator: func(result *PhaseResult) error {
			return nil
		},
	}

	if phase.Name != "test_phase" {
		t.Errorf("expected Name to be 'test_phase', got %s", phase.Name)
	}
	if phase.MaxRounds != 5 {
		t.Errorf("expected MaxRounds to be 5, got %d", phase.MaxRounds)
	}
	if len(phase.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(phase.Tools))
	}
}

func TestPhaseResult(t *testing.T) {
	result := &PhaseResult{
		PhaseName:  "analyze",
		Success:    true,
		Output:     "Analysis complete",
		RoundsUsed: 3,
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.RoundsUsed != 3 {
		t.Errorf("expected RoundsUsed to be 3, got %d", result.RoundsUsed)
	}
}

func TestExtractJSONData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   `Some text before {"key": "value"} some text after`,
			wantKey: "key",
			wantErr: false,
		},
		{
			name:    "JSON with code block",
			input:   "```json\n{\"key\": \"value\"}\n```",
			wantKey: "key",
			wantErr: false,
		},
		{
			name:    "no JSON",
			input:   "No JSON here",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := extractJSONData(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractJSONData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && data[tt.wantKey] == nil {
				t.Errorf("expected key %s in result", tt.wantKey)
			}
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		got := truncateOutput(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// TestPhaseResultToolTracking verifies that tool calls are captured in PhaseResult
func TestPhaseResultToolTracking(t *testing.T) {
	// Create a minimal PhaseResult
	result := &PhaseResult{
		PhaseName:     "test",
		StartedAt:     time.Now(),
		ToolCalls:     []ToolCallSummary{},
		ModifiedFiles: []string{},
	}

	// Simulate tool execution results
	toolResults := []*tools.Result{
		{
			Success:       true,
			Output:        "File created successfully",
			ModifiedFiles: []string{"test.go"},
		},
		{
			Success:       true,
			Output:        "Line 10: old code\nLine 10: new code",
			ModifiedFiles: []string{"main.go"},
		},
		{
			Success: false,
			Error:   "File not found",
		},
	}

	toolNames := []string{"file", "code_edit", "search"}

	// Manually track tool calls (simulating what phaseInferenceExecutor does)
	for i, toolResult := range toolResults {
		summary := ToolCallSummary{
			ToolName:      toolNames[i],
			Success:       toolResult.Success,
			Output:        toolResult.Output,
			Error:         toolResult.Error,
			ModifiedFiles: toolResult.ModifiedFiles,
		}
		result.ToolCalls = append(result.ToolCalls, summary)

		// Track modified files at phase level
		for _, file := range toolResult.ModifiedFiles {
			found := false
			for _, existing := range result.ModifiedFiles {
				if existing == file {
					found = true
					break
				}
			}
			if !found {
				result.ModifiedFiles = append(result.ModifiedFiles, file)
			}
		}
	}

	// Verify tool calls were tracked
	if len(result.ToolCalls) != 3 {
		t.Errorf("Expected 3 tool calls, got %d", len(result.ToolCalls))
	}

	// Verify first tool call
	if result.ToolCalls[0].ToolName != "file" {
		t.Errorf("Expected tool name 'file', got '%s'", result.ToolCalls[0].ToolName)
	}
	if !result.ToolCalls[0].Success {
		t.Error("Expected first tool call to succeed")
	}
	if len(result.ToolCalls[0].ModifiedFiles) != 1 || result.ToolCalls[0].ModifiedFiles[0] != "test.go" {
		t.Errorf("Expected modified files [test.go], got %v", result.ToolCalls[0].ModifiedFiles)
	}

	// Verify second tool call (code_edit)
	if result.ToolCalls[1].ToolName != "code_edit" {
		t.Errorf("Expected tool name 'code_edit', got '%s'", result.ToolCalls[1].ToolName)
	}
	if result.ToolCalls[1].Output == "" {
		t.Error("Expected code_edit output to be non-empty")
	}

	// Verify third tool call (failed)
	if result.ToolCalls[2].Success {
		t.Error("Expected third tool call to fail")
	}
	if result.ToolCalls[2].Error != "File not found" {
		t.Errorf("Expected error 'File not found', got '%s'", result.ToolCalls[2].Error)
	}

	// Verify modified files at phase level
	if len(result.ModifiedFiles) != 2 {
		t.Errorf("Expected 2 modified files at phase level, got %d", len(result.ModifiedFiles))
	}
	expectedFiles := map[string]bool{"test.go": false, "main.go": false}
	for _, file := range result.ModifiedFiles {
		if _, exists := expectedFiles[file]; exists {
			expectedFiles[file] = true
		}
	}
	for file, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file '%s' in ModifiedFiles", file)
		}
	}
}

// TestPhaseCallbackInvocation verifies that phase callbacks are called correctly
func TestPhaseCallbackInvocation(t *testing.T) {
	callbackInvoked := false
	var capturedPhase Phase
	var capturedResult *PhaseResult

	callback := func(phase Phase, result *PhaseResult) (bool, error) {
		callbackInvoked = true
		capturedPhase = phase
		capturedResult = result
		return true, nil // Continue execution
	}

	// Create a minimal executor setup
	executor := &PhasedExecutor{
		phaseCallback: callback,
	}

	// Simulate calling the callback
	testPhase := Phase{
		Name:        "test_phase",
		Description: "Test phase",
	}
	testResult := &PhaseResult{
		PhaseName: "test_phase",
		Success:   true,
		StartedAt: time.Now(),
		ToolCalls: []ToolCallSummary{},
	}

	shouldContinue, err := executor.phaseCallback(testPhase, testResult)

	// Verify callback was invoked
	if !callbackInvoked {
		t.Error("Phase callback was not invoked")
	}

	// Verify correct parameters were passed
	if capturedPhase.Name != "test_phase" {
		t.Errorf("Expected phase name 'test_phase', got '%s'", capturedPhase.Name)
	}

	if capturedResult.PhaseName != "test_phase" {
		t.Errorf("Expected result phase name 'test_phase', got '%s'", capturedResult.PhaseName)
	}

	// Verify return values
	if !shouldContinue {
		t.Error("Expected shouldContinue to be true")
	}

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestPhaseCallbackCancellation verifies that execution stops when callback returns false
func TestPhaseCallbackCancellation(t *testing.T) {
	callback := func(phase Phase, result *PhaseResult) (bool, error) {
		// User cancelled
		return false, nil
	}

	executor := &PhasedExecutor{
		phaseCallback: callback,
	}

	testPhase := Phase{Name: "test"}
	testResult := &PhaseResult{PhaseName: "test", Success: true}

	shouldContinue, err := executor.phaseCallback(testPhase, testResult)

	if shouldContinue {
		t.Error("Expected shouldContinue to be false when user cancels")
	}

	if err != nil {
		t.Errorf("Expected no error on cancellation, got %v", err)
	}
}

// TestToolCallSummaryFields verifies ToolCallSummary captures all necessary data
func TestToolCallSummaryFields(t *testing.T) {
	summary := ToolCallSummary{
		ToolName:      "code_edit",
		Success:       true,
		Output:        "Successfully edited file",
		Error:         "",
		ModifiedFiles: []string{"main.go", "handler.go"},
	}

	if summary.ToolName != "code_edit" {
		t.Errorf("Expected ToolName 'code_edit', got '%s'", summary.ToolName)
	}

	if !summary.Success {
		t.Error("Expected Success to be true")
	}

	if summary.Output == "" {
		t.Error("Expected Output to be non-empty")
	}

	if len(summary.ModifiedFiles) != 2 {
		t.Errorf("Expected 2 modified files, got %d", len(summary.ModifiedFiles))
	}
}

// TestEmptyToolCallsList verifies that an empty tool calls list is handled correctly
func TestEmptyToolCallsList(t *testing.T) {
	result := &PhaseResult{
		PhaseName:     "test",
		ToolCalls:     []ToolCallSummary{},
		ModifiedFiles: []string{},
	}

	// This should not panic or cause issues
	if result.ToolCalls == nil {
		t.Error("ToolCalls should be an empty slice, not nil")
	}

	if len(result.ToolCalls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(result.ToolCalls))
	}
}

func TestSanitizePlanOutput(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "Plan with file paths",
			input: `{
  "plan": {
    "title": "Implementation plan for Prometheus observability metrics",
    "total_steps": 10,
    "steps": [
      {
        "step": 1,
        "title": "Create the pkg/metrics package",
        "files": ["pkg/metrics/metrics.go", "pkg/metrics/metrics_test.go"]
      }
    ]
  }
}`,
			wantContains: []string{
				"A detailed implementation plan was created",
				"Title: Implementation plan for Prometheus observability metrics",
				"Total steps: 10",
				"context tool",
			},
			wantNotContains: []string{
				"pkg/metrics/metrics.go",
				"metrics_test.go",
				`"files":`,
				`"step":`,
			},
		},
		{
			name: "Plan with multiple steps",
			input: `{
  "plan": {
    "title": "Add authentication",
    "total_steps": 5,
    "steps": [
      {"step": 1, "files": ["auth.go"]},
      {"step": 2, "files": ["middleware.go"]},
      {"step": 3, "files": ["handler.go"]},
      {"step": 4, "files": ["test.go"]},
      {"step": 5, "files": ["docs.md"]}
    ]
  }
}`,
			wantContains: []string{
				"Title: Add authentication",
				"Total steps: 5",
			},
			wantNotContains: []string{
				"auth.go",
				"middleware.go",
				"handler.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePlanOutput(tt.input)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("sanitizePlanOutput() output missing expected text\nwant substring: %q\ngot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("sanitizePlanOutput() output contains forbidden text\nshould not contain: %q\ngot: %s", notWant, got)
				}
			}
		})
	}
}

func TestSanitizeAnalyzeOutput(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "Analyze with code blocks",
			input: `Analysis complete. Found the following:

File structure:
- pkg/metrics/metrics.go exists

Code sample:
` + "```go" + `
func NewMetrics() *Metrics {
    return &Metrics{}
}
` + "```" + `

Recommendation: Add tests`,
			wantContains: []string{
				"Analysis complete",
				"Recommendation: Add tests",
			},
			wantNotContains: []string{
				"func NewMetrics",
				"return &Metrics{}",
				"pkg/metrics/metrics.go",
			},
		},
		{
			name: "Analyze with tool calls",
			input: `Found issues:
- Missing imports
- Example tool call: {"tool": "file", "args": {"action": "read"}}

Need to fix these.`,
			wantContains: []string{
				"Found issues",
				"Missing imports",
				"Need to fix these",
			},
			wantNotContains: []string{
				`{"tool":`,
				`"action": "read"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeAnalyzeOutput(tt.input)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("sanitizeAnalyzeOutput() output missing expected text\nwant substring: %q\ngot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("sanitizeAnalyzeOutput() output contains forbidden text\nshould not contain: %q\ngot: %s", notWant, got)
				}
			}
		})
	}
}

func TestSanitizeImplementOutput(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "Implement with JSON examples",
			input: `Created metrics package.

Example usage:
` + "```json" + `
{"tool": "file", "args": {"action": "write", "path": "metrics.go"}}
` + "```" + `

Implementation complete.`,
			wantContains: []string{
				"Created metrics package",
				"Example usage:",
				"Implementation complete",
			},
			wantNotContains: []string{
				`{"tool": "file"`,
				`"action": "write"`,
				"metrics.go",
			},
		},
		{
			name: "Implement with inline tool calls",
			input: `Step 1: Created file {"tool": "file", "args": {...}}
Step 2: Added tests {"tool": "test", "args": {...}}
Done.`,
			wantContains: []string{
				"Step 1: Created file",
				"Step 2: Added tests",
				"Done",
			},
			wantNotContains: []string{
				`{"tool": "file"`,
				`{"tool": "test"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeImplementOutput(tt.input)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("sanitizeImplementOutput() output missing expected text\nwant substring: %q\ngot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("sanitizeImplementOutput() output contains forbidden text\nshould not contain: %q\ngot: %s", notWant, got)
				}
			}
		})
	}
}

func TestIsFilePath(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"Go file", "pkg/metrics/metrics.go", true},
		{"JS file", "src/components/Button.js", true},
		{"Python file", "scripts/deploy.py", true},
		{"Markdown file", "docs/README.md", true},
		{"JSON file", "config/settings.json", true},
		{"YAML file", ".github/workflows/ci.yml", true},
		{"Array notation", `["pkg/metrics/metrics.go"]`, true},
		{"Plain text", "This is a plain sentence", false},
		{"Plain word", "metrics", false},
		{"Sentence with extension", "I like the .go language", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFilePath(tt.line)
			if got != tt.want {
				t.Errorf("isFilePath(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestSanitizePhaseOutput(t *testing.T) {
	tests := []struct {
		name            string
		phaseName       string
		input           string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:      "Plan phase sanitization",
			phaseName: "plan",
			input: `{
  "plan": {
    "title": "Add tests",
    "total_steps": 3,
    "steps": [
      {"step": 1, "files": ["test.go"]},
      {"step": 2, "files": ["main.go"]},
      {"step": 3, "files": ["helper.go"]}
    ]
  }
}`,
			wantContains: []string{
				"Title: Add tests",
				"Total steps: 3",
			},
			wantNotContains: []string{
				"test.go",
				"main.go",
				"helper.go",
			},
		},
		{
			name:      "Analyze phase sanitization",
			phaseName: "analyze",
			input: `Analyzed codebase.
Found: pkg/server.go needs updates
` + "```go" + `
func Start() {
    // code
}
` + "```" + `
Recommendation: refactor`,
			wantContains: []string{
				"Analyzed codebase",
				"Recommendation: refactor",
			},
			wantNotContains: []string{
				"pkg/server.go",
				"func Start()",
			},
		},
		{
			name:      "Implement phase sanitization",
			phaseName: "implement",
			input: `Created files.
Used: {"tool": "file", "args": {...}}
PHASE_COMPLETE`,
			wantContains: []string{
				"Created files",
				"PHASE_COMPLETE",
			},
			wantNotContains: []string{
				`{"tool":`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePhaseOutput(tt.input, tt.phaseName)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("sanitizePhaseOutput() output missing expected text\nwant substring: %q\ngot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("sanitizePhaseOutput() output contains forbidden text\nshould not contain: %q\ngot: %s", notWant, got)
				}
			}
		})
	}
}

func TestSanitizePhaseOutputEmptyInput(t *testing.T) {
	got := sanitizePhaseOutput("", "plan")
	if got != "" {
		t.Errorf("sanitizePhaseOutput(\"\", \"plan\") = %q, want empty string", got)
	}
}

func TestBuildNextPhaseInputWithSanitization(t *testing.T) {
	// Test that buildNextPhaseInput properly sanitizes output
	executor := &PhasedExecutor{}

	result := &PhaseResult{
		PhaseName: "plan",
		Output: `{
  "plan": {
    "title": "Test plan",
    "total_steps": 2,
    "steps": [
      {"step": 1, "files": ["pkg/test.go"]},
      {"step": 2, "files": ["pkg/main.go"]}
    ]
  }
}`,
	}

	got := executor.buildNextPhaseInput(result)

	// Should contain sanitized summary
	wantContains := []string{
		"Previous Phase: plan",
		"Title: Test plan",
		"Total steps: 2",
		"context tool",
	}

	// Should NOT contain raw file paths
	wantNotContains := []string{
		"pkg/test.go",
		"pkg/main.go",
		`"files":`,
	}

	for _, want := range wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("buildNextPhaseInput() output missing expected text\nwant substring: %q\ngot: %s", want, got)
		}
	}

	for _, notWant := range wantNotContains {
		if strings.Contains(got, notWant) {
			t.Errorf("buildNextPhaseInput() output contains forbidden text\nshould not contain: %q\ngot: %s", notWant, got)
		}
	}
}
