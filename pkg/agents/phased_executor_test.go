package agents

import (
	"strings"
	"testing"
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
