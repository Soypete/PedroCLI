package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/logits"
)

// TestTool runs tests and parses results
type TestTool struct {
	workDir string
}

// NewTestTool creates a new test tool
func NewTestTool(workDir string) *TestTool {
	return &TestTool{
		workDir: workDir,
	}
}

// Name returns the tool name
func (t *TestTool) Name() string {
	return "test"
}

// Description returns the tool description
func (t *TestTool) Description() string {
	return `Run tests and parse results for Go, npm, and Python projects.

Args:
- type (string): Test framework - "go", "npm", or "python" (default: "go")

Go-specific args:
- package (string): Package to test (default: "./...")
- verbose (bool): Verbose output (default: false)
- run (string): Regex pattern to run specific tests
- count (int): Run tests n times

npm-specific args:
- script (string): npm script to run (default: "test")

Python-specific args:
- module (string): Module/directory to test
- verbose (bool): Verbose output

Usage Tips:
- ALWAYS run tests after making changes to verify correctness
- Use specific package/module paths to run relevant tests faster
- Use "run" to target specific test functions
- Analyze test output carefully when tests fail

Examples:
{"tool": "test", "args": {"type": "go", "package": "./pkg/tools/..."}}
{"tool": "test", "args": {"type": "go", "run": "TestFileTool"}}
{"tool": "test", "args": {"type": "npm", "script": "test:unit"}}
{"tool": "test", "args": {"type": "python", "module": "tests/", "verbose": true}}`
}

// Execute executes the test tool
func (t *TestTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	testType, ok := args["type"].(string)
	if !ok {
		testType = "go" // default to Go tests
	}

	switch testType {
	case "go":
		return t.runGoTests(ctx, args)
	case "npm":
		return t.runNpmTests(ctx, args)
	case "python":
		return t.runPythonTests(ctx, args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown test type: %s", testType)}, nil
	}
}

// runGoTests runs Go tests
func (t *TestTool) runGoTests(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Optional: specific package
	pkg := "./..."
	if p, ok := args["package"].(string); ok {
		pkg = p
	}

	// Optional: verbose
	verbose := false
	if v, ok := args["verbose"].(bool); ok {
		verbose = v
	}

	cmdArgs := []string{"test"}
	if verbose {
		cmdArgs = append(cmdArgs, "-v")
	}
	cmdArgs = append(cmdArgs, pkg)

	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = t.workDir

	output, err := cmd.CombinedOutput()

	// Parse results
	result := t.parseGoTestOutput(string(output))
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	}

	return result, nil
}

// runNpmTests runs npm tests
func (t *TestTool) runNpmTests(ctx context.Context, args map[string]interface{}) (*Result, error) {
	cmd := exec.CommandContext(ctx, "npm", "test")
	cmd.Dir = t.workDir

	output, err := cmd.CombinedOutput()

	result := &Result{
		Success: err == nil,
		Output:  string(output),
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}

// runPythonTests runs Python tests
func (t *TestTool) runPythonTests(ctx context.Context, args map[string]interface{}) (*Result, error) {
	cmd := exec.CommandContext(ctx, "python", "-m", "pytest")
	cmd.Dir = t.workDir

	output, err := cmd.CombinedOutput()

	result := &Result{
		Success: err == nil,
		Output:  string(output),
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}

// parseGoTestOutput parses Go test output to extract pass/fail counts
func (t *TestTool) parseGoTestOutput(output string) *Result {
	lines := strings.Split(output, "\n")

	passed := 0
	failed := 0

	for _, line := range lines {
		if strings.Contains(line, "PASS") {
			passed++
		} else if strings.Contains(line, "FAIL") {
			failed++
		}
	}

	success := failed == 0

	summary := fmt.Sprintf("Tests: %d passed, %d failed", passed, failed)

	return &Result{
		Success: success,
		Output:  summary,
	}
}

// Metadata returns rich tool metadata for discovery and LLM guidance
func (t *TestTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"type": {
					Type:        "string",
					Enum:        []interface{}{"go", "npm", "python"},
					Description: "Test framework type (default: go)",
				},
				"package": {
					Type:        "string",
					Description: "Go package to test (default: ./...)",
				},
				"verbose": {
					Type:        "boolean",
					Description: "Enable verbose output",
				},
				"run": {
					Type:        "string",
					Description: "Regex pattern to run specific tests (Go)",
				},
				"count": {
					Type:        "integer",
					Description: "Run tests n times (Go)",
				},
				"script": {
					Type:        "string",
					Description: "npm script to run (default: test)",
				},
				"module": {
					Type:        "string",
					Description: "Python module/directory to test",
				},
			},
		},
		Category:    CategoryBuild,
		Optionality: ToolRequired,
		UsageHint:   "ALWAYS run tests after making changes to verify correctness. Use specific packages for faster feedback.",
		Examples: []ToolExample{
			{
				Description: "Run all Go tests",
				Input:       map[string]interface{}{"type": "go"},
			},
			{
				Description: "Run specific Go test",
				Input:       map[string]interface{}{"type": "go", "run": "TestFileTool", "verbose": true},
			},
			{
				Description: "Run npm tests",
				Input:       map[string]interface{}{"type": "npm"},
			},
		},
		Produces: []string{"test_results"},
	}
}
