package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestToolName(t *testing.T) {
	tool := NewTestTool("/tmp")

	if tool.Name() != "test" {
		t.Errorf("Name() = %v, want test", tool.Name())
	}
}

func TestTestToolDescription(t *testing.T) {
	tool := NewTestTool("/tmp")

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestTestToolGoTests(t *testing.T) {
	// Create a temp directory with a simple Go test
	tmpDir := t.TempDir()

	// Create a simple Go file
	goFile := `package testpkg

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "math.go"), []byte(goFile), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a test file
	testFile := `package testpkg

import "testing"

func TestAdd(t *testing.T) {
	result := Add(2, 3)
	if result != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", result)
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "math_test.go"), []byte(testFile), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize go module
	modFile := `module testpkg

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	tool := NewTestTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name     string
		args     map[string]interface{}
		validate func(*testing.T, *Result)
	}{
		{
			name: "run go tests with default type",
			args: map[string]interface{}{},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s, Output: %s", r.Error, r.Output)
				}
			},
		},
		{
			name: "run go tests with explicit type",
			args: map[string]interface{}{
				"type": "go",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s, Output: %s", r.Error, r.Output)
				}
			},
		},
		{
			name: "run go tests verbose",
			args: map[string]interface{}{
				"type":    "go",
				"verbose": true,
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s, Output: %s", r.Error, r.Output)
				}
			},
		},
		{
			name: "run go tests with specific package",
			args: map[string]interface{}{
				"type":    "go",
				"package": ".",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s, Output: %s", r.Error, r.Output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			tt.validate(t, result)
		})
	}
}

func TestTestToolGoTestsFailing(t *testing.T) {
	// Create a temp directory with a failing test
	tmpDir := t.TempDir()

	// Create a simple Go file
	goFile := `package testpkg

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "math.go"), []byte(goFile), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a failing test
	testFile := `package testpkg

import "testing"

func TestAddFail(t *testing.T) {
	result := Add(2, 3)
	if result != 10 { // Intentionally wrong
		t.Errorf("Add(2, 3) = %d, want 10", result)
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "math_test.go"), []byte(testFile), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize go module
	modFile := `module testpkg

go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	tool := NewTestTool(tmpDir)
	ctx := context.Background()

	args := map[string]interface{}{
		"type": "go",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	// Should fail because the test fails
	if result.Success {
		t.Error("Success = true, want false for failing tests")
	}

	if result.Error == "" {
		t.Error("Error should not be empty for failing tests")
	}
}

func TestTestToolUnknownType(t *testing.T) {
	tool := NewTestTool("/tmp")
	ctx := context.Background()

	args := map[string]interface{}{
		"type": "unknown",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for unknown test type")
	}

	if !strings.Contains(result.Error, "unknown test type") {
		t.Errorf("Error = %q, want error containing 'unknown test type'", result.Error)
	}
}

func TestParseGoTestOutput(t *testing.T) {
	tool := NewTestTool("/tmp")

	tests := []struct {
		name          string
		output        string
		wantSuccess   bool
		checkContains string
	}{
		{
			name: "all tests pass",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
PASS
ok  	testpkg	0.001s`,
			wantSuccess:   true,
			checkContains: "passed",
		},
		{
			name: "some tests fail",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSubtract
--- FAIL: TestSubtract (0.00s)
FAIL
exit status 1
FAIL	testpkg	0.002s`,
			wantSuccess:   false,
			checkContains: "failed",
		},
		{
			name:          "no tests run",
			output:        `?   	testpkg	[no test files]`,
			wantSuccess:   true,
			checkContains: "0 passed, 0 failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.parseGoTestOutput(tt.output)

			if result.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if !strings.Contains(result.Output, tt.checkContains) {
				t.Errorf("Output = %q, want output containing %q", result.Output, tt.checkContains)
			}
		})
	}
}

func TestTestToolNpmTests(t *testing.T) {
	// Skip if npm is not available
	tmpDir := t.TempDir()
	tool := NewTestTool(tmpDir)
	ctx := context.Background()

	// Create a minimal package.json
	packageJSON := `{
  "name": "test-package",
  "version": "1.0.0",
  "scripts": {
    "test": "echo 'No tests specified'"
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	args := map[string]interface{}{
		"type": "npm",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	// Note: This test will fail if npm is not installed, but that's expected
	// We're mainly testing the interface
	if result == nil {
		t.Error("Result should not be nil")
	}
}

func TestTestToolPythonTests(t *testing.T) {
	// This is just testing the interface, not actual Python execution
	tmpDir := t.TempDir()
	tool := NewTestTool(tmpDir)
	ctx := context.Background()

	args := map[string]interface{}{
		"type": "python",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	// Note: This test will likely fail if pytest is not installed
	// We're mainly testing the interface
	if result == nil {
		t.Error("Result should not be nil")
	}
}
