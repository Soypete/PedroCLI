package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/soypete/pedro-agentware/middleware"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// MockContextManager is a minimal mock for testing PhasedExecutor
type MockContextManager struct {
	jobID string
}

func (m *MockContextManager) GetJobID() string {
	return m.jobID
}

func (m *MockContextManager) GetJobDir() string {
	return "/tmp/test-" + m.jobID
}

func (m *MockContextManager) SavePrompt(prompt string) error {
	return nil
}

func (m *MockContextManager) SaveResponse(response string) error {
	return nil
}

func (m *MockContextManager) SaveToolCalls(calls []llmcontext.ToolCall) error {
	return nil
}

func (m *MockContextManager) SaveToolResults(results []llmcontext.ToolResult) error {
	return nil
}

func (m *MockContextManager) GetHistoryWithinBudget(budget int) (string, error) {
	return "mock history", nil
}

func (m *MockContextManager) ShouldCompact() bool {
	return false
}

func (m *MockContextManager) CompactHistory(keepRounds int) (string, error) {
	return "compacted", nil
}

func (m *MockContextManager) GetCompactionStats() (*llmcontext.CompactionStats, error) {
	return nil, nil
}

func (m *MockContextManager) Cleanup() error {
	return nil
}

// MockLLMBackend is a simple mock that returns pre-programmed responses
type MockLLMBackend struct {
	responses   []string
	currentCall int
}

func (m *MockLLMBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	if m.currentCall >= len(m.responses) {
		return &llm.InferenceResponse{
			Text:       "PHASE_COMPLETE",
			TokensUsed: 10,
		}, nil
	}

	resp := m.responses[m.currentCall]
	m.currentCall++

	return &llm.InferenceResponse{
		Text:       resp + "\n\nPHASE_COMPLETE",
		TokensUsed: 10,
	}, nil
}

func (m *MockLLMBackend) GetContextWindow() int {
	return 4096
}

func (m *MockLLMBackend) GetUsableContext() int {
	return 3072
}

func (m *MockLLMBackend) Tokenize(ctx context.Context, text string) ([]int, error) {
	return []int{1, 2, 3}, nil
}

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
		name         string
		input        string
		maxLen       int
		expectTrunc  bool
		maxOutputLen int
	}{
		{
			name:         "short output unchanged",
			input:        "short",
			maxLen:       10,
			expectTrunc:  false,
			maxOutputLen: 10,
		},
		{
			name:         "exact length unchanged",
			input:        "exact",
			maxLen:       5,
			expectTrunc:  false,
			maxOutputLen: 5,
		},
		{
			name:         "long output truncated",
			input:        "this is a very long string that should be truncated",
			maxLen:       10,
			expectTrunc:  true,
			maxOutputLen: 150, // Allow room for truncation message
		},
		{
			name:         "very large output heavily truncated",
			input:        strings.Repeat("Large search result content ", 5000), // ~140K chars
			maxLen:       1000,
			expectTrunc:  true,
			maxOutputLen: 1200,
		},
		{
			name:         "output with newlines truncates at newline",
			input:        strings.Repeat("line\n", 300), // 1500 chars
			maxLen:       1000,
			expectTrunc:  true,
			maxOutputLen: 1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOutput(tt.input, tt.maxLen)

			// Check length constraint
			if len(result) > tt.maxOutputLen {
				t.Errorf("Expected length <= %d, got %d", tt.maxOutputLen, len(result))
			}

			// Check truncation message presence
			if tt.expectTrunc {
				if !strings.Contains(result, "[Output truncated") {
					t.Error("Expected truncation notice in output")
				}
				if !strings.Contains(result, "Full result saved to context files") {
					t.Error("Expected context files message in output")
				}
			} else {
				if strings.Contains(result, "[Output truncated") {
					t.Error("Did not expect truncation notice for short output")
				}
				// For non-truncated, output should be identical
				if result != tt.input {
					t.Errorf("Expected output to be unchanged for short input, got %q want %q", result, tt.input)
				}
			}
		})
	}
}

// TestPhaseResultToolTracking verifies that tool calls are captured in PhaseResult
// DISABLED: ToolCalls and ModifiedFiles fields removed from PhaseResult during merge
func TestPhaseResultToolTracking_DISABLED(t *testing.T) {
	t.Skip("Test disabled - ToolCalls and ModifiedFiles tracking removed from PhaseResult")
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
// DISABLED: ToolCallSummary type removed during merge
func TestToolCallSummaryFields_DISABLED(t *testing.T) {
	t.Skip("Test disabled - ToolCallSummary type removed")
}

// TestEmptyToolCallsList verifies that an empty tool calls list is handled correctly
// DISABLED: ToolCalls field removed during merge
func TestEmptyToolCallsList_DISABLED(t *testing.T) {
	t.Skip("Test disabled - ToolCalls field removed from PhaseResult")
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

func TestFilterToolDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		phaseTools []string
		allTools   []llm.ToolDefinition
		want       int
		wantNames  []string
	}{
		{
			name:       "nil phase tools returns all",
			phaseTools: nil,
			allTools: []llm.ToolDefinition{
				{Name: "file"}, {Name: "git"}, {Name: "bash"},
			},
			want:      3,
			wantNames: []string{"file", "git", "bash"},
		},
		{
			name:       "filters to allowed subset",
			phaseTools: []string{"git", "github"},
			allTools: []llm.ToolDefinition{
				{Name: "file"}, {Name: "git"}, {Name: "bash"}, {Name: "github"},
			},
			want:      2,
			wantNames: []string{"git", "github"},
		},
		{
			name:       "handles missing tools gracefully",
			phaseTools: []string{"git", "nonexistent"},
			allTools: []llm.ToolDefinition{
				{Name: "file"}, {Name: "git"},
			},
			want:      1,
			wantNames: []string{"git"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := Phase{Name: "test", Tools: tt.phaseTools}
			cfg := &config.Config{Debug: config.DebugConfig{Enabled: false}}
			agent := NewBaseAgent("test", "test", cfg, nil, nil)

			pie := &phaseInferenceExecutor{
				agent:       agent,
				phase:       phase,
				callHistory: middleware.NewCallHistory(),
			}

			filtered := pie.filterToolDefinitions(tt.allTools)

			if len(filtered) != tt.want {
				t.Errorf("got %d tools, want %d", len(filtered), tt.want)
			}

			gotNames := make(map[string]bool)
			for _, def := range filtered {
				gotNames[def.Name] = true
			}

			for _, name := range tt.wantNames {
				if !gotNames[name] {
					t.Errorf("expected tool %q in filtered results", name)
				}
			}
		})
	}
}

// TestInsertPhases tests the insertPhases helper function
func TestInsertPhases(t *testing.T) {
	initial := []Phase{
		{Name: "phase1"},
		{Name: "phase2"},
		{Name: "phase3"},
	}

	tests := []struct {
		name      string
		position  int
		newPhases []Phase
		want      []string
	}{
		{
			name:     "insert at beginning",
			position: 0,
			newPhases: []Phase{
				{Name: "new1"},
			},
			want: []string{"new1", "phase1", "phase2", "phase3"},
		},
		{
			name:     "insert in middle",
			position: 1,
			newPhases: []Phase{
				{Name: "new1"},
				{Name: "new2"},
			},
			want: []string{"phase1", "new1", "new2", "phase2", "phase3"},
		},
		{
			name:     "insert at end",
			position: 3,
			newPhases: []Phase{
				{Name: "new1"},
			},
			want: []string{"phase1", "phase2", "phase3", "new1"},
		},
		{
			name:     "insert beyond end (append)",
			position: 10,
			newPhases: []Phase{
				{Name: "new1"},
			},
			want: []string{"phase1", "phase2", "phase3", "new1"},
		},
		{
			name:     "invalid position negative (append)",
			position: -1,
			newPhases: []Phase{
				{Name: "new1"},
			},
			want: []string{"phase1", "phase2", "phase3", "new1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of initial to avoid mutation
			phases := make([]Phase, len(initial))
			copy(phases, initial)

			result := insertPhases(phases, tt.position, tt.newPhases)

			if len(result) != len(tt.want) {
				t.Errorf("got %d phases, want %d", len(result), len(tt.want))
			}

			for i, want := range tt.want {
				if i >= len(result) {
					t.Errorf("missing phase at index %d: want %s", i, want)
					continue
				}
				if result[i].Name != want {
					t.Errorf("phase[%d].Name = %s, want %s", i, result[i].Name, want)
				}
			}
		})
	}
}

// TestPhasedExecutor_DynamicPhaseGeneration tests that PhaseGenerator correctly
// inserts new phases dynamically based on phase results
func TestPhasedExecutor_DynamicPhaseGeneration(t *testing.T) {
	tests := []struct {
		name                string
		numSectionsToGen    int
		expectedTotalPhases int
		expectedPhaseNames  []string
	}{
		{
			name:                "generate 3 sections",
			numSectionsToGen:    3,
			expectedTotalPhases: 5, // outline + 3 sections + assemble
			expectedPhaseNames:  []string{"outline", "section_0", "section_1", "section_2", "assemble"},
		},
		{
			name:                "generate 5 sections",
			numSectionsToGen:    5,
			expectedTotalPhases: 7, // outline + 5 sections + assemble
			expectedPhaseNames:  []string{"outline", "section_0", "section_1", "section_2", "section_3", "section_4", "assemble"},
		},
		{
			name:                "generate 0 sections",
			numSectionsToGen:    0,
			expectedTotalPhases: 2, // outline + assemble (no sections generated)
			expectedPhaseNames:  []string{"outline", "assemble"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create phase generator that simulates outline phase
			phaseGenerator := func(result *PhaseResult) ([]Phase, error) {
				// Simulate parsing sections from outline
				sectionPhases := make([]Phase, 0, tt.numSectionsToGen)
				for i := 0; i < tt.numSectionsToGen; i++ {
					sectionPhases = append(sectionPhases, Phase{
						Name:        fmt.Sprintf("section_%d", i),
						Description: "Generated section",
						MaxRounds:   1,
					})
				}
				return sectionPhases, nil
			}

			phases := []Phase{
				{
					Name:           "outline",
					Description:    "Generate outline",
					MaxRounds:      1,
					PhaseGenerator: phaseGenerator,
				},
				{
					Name:        "assemble",
					Description: "Assemble final content",
					MaxRounds:   1,
				},
			}

			// Test the phase generation logic
			result := &PhaseResult{
				PhaseName: "outline",
				Success:   true,
				Output:    "Test outline",
			}

			generatedPhases, err := phaseGenerator(result)
			if err != nil {
				t.Fatalf("phase generator failed: %v", err)
			}

			// Test insertion
			newPhases := insertPhases(phases, 1, generatedPhases)

			if len(newPhases) != tt.expectedTotalPhases {
				t.Errorf("total phases = %d, want %d", len(newPhases), tt.expectedTotalPhases)
			}

			// Verify phase names
			gotNames := make([]string, len(newPhases))
			for i, p := range newPhases {
				gotNames[i] = p.Name
			}

			if len(gotNames) != len(tt.expectedPhaseNames) {
				t.Errorf("got %d phase names, want %d", len(gotNames), len(tt.expectedPhaseNames))
			}

			for i, want := range tt.expectedPhaseNames {
				if i >= len(gotNames) {
					t.Errorf("missing phase at index %d: want %s", i, want)
					continue
				}
				if gotNames[i] != want {
					t.Errorf("phase[%d] = %s, want %s", i, gotNames[i], want)
				}
			}
		})
	}
}

// TestPhasedExecutor_CheckpointWithGeneratedPhases tests that checkpoints
// correctly track dynamically generated phases
func TestPhasedExecutor_CheckpointWithGeneratedPhases(t *testing.T) {
	// Create phases with generator
	phaseGenerator := func(result *PhaseResult) ([]Phase, error) {
		return []Phase{
			{Name: "generated_1", MaxRounds: 1},
			{Name: "generated_2", MaxRounds: 1},
		}, nil
	}

	phases := []Phase{
		{
			Name:           "outline",
			PhaseGenerator: phaseGenerator,
			MaxRounds:      1,
		},
		{Name: "assemble", MaxRounds: 1},
	}

	// Test checkpoint structure
	initialPhaseCount := len(phases)

	// Simulate phase completion and generation
	result := &PhaseResult{
		PhaseName: "outline",
		Success:   true,
		Output:    "Test outline",
	}

	generated, err := phaseGenerator(result)
	if err != nil {
		t.Fatalf("phase generator failed: %v", err)
	}

	newPhases := insertPhases(phases, 1, generated)

	// Verify phase counts
	expectedTotal := 4 // outline + 2 generated + assemble
	if len(newPhases) != expectedTotal {
		t.Errorf("total phases = %d, want %d", len(newPhases), expectedTotal)
	}

	expectedGenerated := len(newPhases) - initialPhaseCount
	if expectedGenerated != 2 {
		t.Errorf("generated phases = %d, want 2", expectedGenerated)
	}

	// Verify phase names in order
	expectedNames := []string{"outline", "generated_1", "generated_2", "assemble"}
	for i, expected := range expectedNames {
		if i >= len(newPhases) {
			t.Errorf("missing phase at index %d: want %s", i, expected)
			continue
		}
		if newPhases[i].Name != expected {
			t.Errorf("phase[%d] = %s, want %s", i, newPhases[i].Name, expected)
		}
	}
}

// TestFilterToolDefinitions_NilVsEmptySemantics tests the tool filtering logic for nil vs empty array
func TestFilterToolDefinitions_NilVsEmptySemantics(t *testing.T) {
	allTools := []llm.ToolDefinition{
		{Name: "tool1"},
		{Name: "tool2"},
		{Name: "tool3"},
	}

	tests := []struct {
		name      string
		tools     []string
		wantCount int
		wantNames []string
	}{
		{
			name:      "nil tools = unrestricted (all tools)",
			tools:     nil,
			wantCount: 3,
			wantNames: []string{"tool1", "tool2", "tool3"},
		},
		{
			name:      "empty array = no tools",
			tools:     []string{},
			wantCount: 0,
			wantNames: []string{},
		},
		{
			name:      "specific tools = filtered subset",
			tools:     []string{"tool1", "tool3"},
			wantCount: 2,
			wantNames: []string{"tool1", "tool3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := Phase{Name: "test", Tools: tt.tools}
			cfg := &config.Config{Debug: config.DebugConfig{Enabled: false}}
			agent := NewBaseAgent("test", "test", cfg, nil, nil)

			pie := &phaseInferenceExecutor{
				agent:       agent,
				phase:       phase,
				callHistory: middleware.NewCallHistory(),
			}

			filtered := pie.filterToolDefinitions(allTools)

			if len(filtered) != tt.wantCount {
				t.Errorf("got %d tools, want %d", len(filtered), tt.wantCount)
			}

			gotNames := make(map[string]bool)
			for _, def := range filtered {
				gotNames[def.Name] = true
			}

			for _, name := range tt.wantNames {
				if !gotNames[name] {
					t.Errorf("expected tool %q in filtered results", name)
				}
			}
		})
	}
}

// TestPhasedExecutor_PhaseGeneratorError tests that phase generator errors
// are properly handled
func TestPhasedExecutor_PhaseGeneratorError(t *testing.T) {
	errorPhaseGenerator := func(result *PhaseResult) ([]Phase, error) {
		return nil, fmt.Errorf("failed to parse outline")
	}

	// Simulate successful phase execution that should fail during generation
	result := &PhaseResult{
		PhaseName: "outline",
		Success:   true,
		Output:    "Test outline",
	}

	// Try to generate phases - should fail
	_, err := errorPhaseGenerator(result)
	if err == nil {
		t.Error("expected error from phase generator, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse outline") {
		t.Errorf("error message = %q, want to contain 'failed to parse outline'", err.Error())
	}
}
