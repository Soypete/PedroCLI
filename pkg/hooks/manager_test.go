package hooks

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectProjectType(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "test-hooks-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		files    []string
		expected ProjectType
	}{
		{
			name:     "go project",
			files:    []string{"go.mod"},
			expected: ProjectTypeGo,
		},
		{
			name:     "node project",
			files:    []string{"package.json"},
			expected: ProjectTypeNode,
		},
		{
			name:     "python project",
			files:    []string{"requirements.txt"},
			expected: ProjectTypePython,
		},
		{
			name:     "rust project",
			files:    []string{"Cargo.toml"},
			expected: ProjectTypeRust,
		},
		{
			name:     "unknown project",
			files:    []string{"README.md"},
			expected: ProjectTypeUnknown,
		},
	}

	manager := NewManager()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Create test files
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(testDir, f), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result, err := manager.DetectProjectType(testDir)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDefaultChecks(t *testing.T) {
	tests := []struct {
		projectType ProjectType
		hasPreCommit bool
		hasPrePush   bool
	}{
		{ProjectTypeGo, true, true},
		{ProjectTypeNode, true, true},
		{ProjectTypePython, true, true},
		{ProjectTypeRust, true, true},
		{ProjectTypeUnknown, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.projectType), func(t *testing.T) {
			config := DefaultChecks(tt.projectType)

			if tt.hasPreCommit && len(config.PreCommit) == 0 {
				t.Error("expected pre-commit checks")
			}
			if tt.hasPrePush && len(config.PrePush) == 0 {
				t.Error("expected pre-push checks")
			}
		})
	}
}

func TestHooksConfig_JSON(t *testing.T) {
	config := &HooksConfig{
		ProjectType: ProjectTypeGo,
		PreCommit: []Check{
			{
				Name:     "gofmt",
				Command:  "gofmt",
				Args:     []string{"-l", "."},
				Required: true,
			},
		},
		PrePush: []Check{
			{
				Name:     "go_test",
				Command:  "go",
				Args:     []string{"test", "./..."},
				Required: true,
				Timeout:  5 * time.Minute,
			},
		},
		PreCommitTimeout: 30 * time.Second,
		PrePushTimeout:   5 * time.Minute,
		Source:           "test",
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-config-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager()

	// Save config
	if err := manager.SetHooksConfig(tmpDir, config); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Load config
	loaded, err := manager.GetHooksConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.ProjectType != config.ProjectType {
		t.Errorf("expected project type %s, got %s", config.ProjectType, loaded.ProjectType)
	}

	if len(loaded.PreCommit) != len(config.PreCommit) {
		t.Errorf("expected %d pre-commit checks, got %d", len(config.PreCommit), len(loaded.PreCommit))
	}

	if len(loaded.PrePush) != len(config.PrePush) {
		t.Errorf("expected %d pre-push checks, got %d", len(config.PrePush), len(loaded.PrePush))
	}
}

func TestFormatAgentFeedback_AllPassed(t *testing.T) {
	manager := NewManager()

	result := &ValidationResult{
		AllPassed: true,
		Results: []HookResult{
			{HookName: "pre-commit", CheckName: "gofmt", Passed: true, Duration: time.Second},
			{HookName: "pre-push", CheckName: "go_test", Passed: true, Duration: 2 * time.Second},
		},
		Duration: 3 * time.Second,
	}

	feedback := manager.FormatAgentFeedback(result)

	if !feedback.Success {
		t.Error("expected success")
	}

	if len(feedback.AllResults) != 2 {
		t.Errorf("expected 2 results, got %d", len(feedback.AllResults))
	}
}

func TestFormatAgentFeedback_WithFailure(t *testing.T) {
	manager := NewManager()

	result := &ValidationResult{
		AllPassed: false,
		Results: []HookResult{
			{HookName: "pre-commit", CheckName: "gofmt", Passed: false, Output: "main.go\n", ErrorMsg: "files need formatting"},
			{HookName: "pre-push", CheckName: "go_test", Passed: true, Duration: 2 * time.Second},
		},
		Duration: 3 * time.Second,
	}

	feedback := manager.FormatAgentFeedback(result)

	if feedback.Success {
		t.Error("expected failure")
	}

	if feedback.FailedCheck != "gofmt" {
		t.Errorf("expected failed check 'gofmt', got '%s'", feedback.FailedCheck)
	}

	if feedback.Suggestion == "" {
		t.Error("expected suggestion")
	}
}

func TestCheck_Validation(t *testing.T) {
	check := Check{
		Name:     "test",
		Command:  "echo",
		Args:     []string{"hello"},
		Required: true,
		Timeout:  time.Second,
	}

	if check.Name == "" {
		t.Error("check should have a name")
	}

	if check.Command == "" {
		t.Error("check should have a command")
	}
}

func TestHookResult_Fields(t *testing.T) {
	result := HookResult{
		HookName:  "pre-commit",
		CheckName: "gofmt",
		Passed:    true,
		Output:    "all good",
		Duration:  time.Second,
	}

	if result.HookName != "pre-commit" {
		t.Errorf("expected 'pre-commit', got '%s'", result.HookName)
	}

	if !result.Passed {
		t.Error("expected passed")
	}
}

func TestValidationResult_Summary(t *testing.T) {
	manager := NewManager()

	result := &ValidationResult{
		AllPassed: true,
		Results: []HookResult{
			{CheckName: "check1", Passed: true},
			{CheckName: "check2", Passed: true},
		},
	}

	summary := manager.buildSummary(result)

	if summary == "" {
		t.Error("expected non-empty summary")
	}

	if len(summary) < 10 {
		t.Error("summary too short")
	}
}
