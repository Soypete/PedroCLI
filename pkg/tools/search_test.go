package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchToolName(t *testing.T) {
	tool := NewSearchTool("/tmp")
	if tool.Name() != "search" {
		t.Errorf("Name() = %v, want search", tool.Name())
	}
}

func TestSearchToolDescription(t *testing.T) {
	tool := NewSearchTool("/tmp")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestGrep(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSearchTool(tmpDir)
	ctx := context.Background()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	os.WriteFile(file1, []byte(`package main

func TestFunction() {
	// test code
}
`), 0644)
	os.WriteFile(file2, []byte(`package main

func TestAnotherFunction() {
	// more test code
}
`), 0644)

	tests := []struct {
		name     string
		args     map[string]interface{}
		validate func(*testing.T, *Result)
	}{
		{
			name: "find pattern",
			args: map[string]interface{}{
				"action":  "grep",
				"pattern": "TestFunction",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "TestFunction") {
					t.Errorf("Output should contain 'TestFunction', got: %q", r.Output)
				}
			},
		},
		{
			name: "case insensitive search",
			args: map[string]interface{}{
				"action":           "grep",
				"pattern":          "testfunction",
				"case_insensitive": true,
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "TestFunction") {
					t.Errorf("Should find TestFunction with case insensitive search")
				}
			},
		},
		{
			name: "file pattern filter",
			args: map[string]interface{}{
				"action":       "grep",
				"pattern":      "func",
				"file_pattern": "*.go",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
			},
		},
		{
			name: "no matches",
			args: map[string]interface{}{
				"action":  "grep",
				"pattern": "NONEXISTENT",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true for no matches")
				}
				if !strings.Contains(r.Output, "No matches found") {
					t.Errorf("Should indicate no matches found")
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

func TestFindFiles(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSearchTool(tmpDir)
	ctx := context.Background()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test_test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# README"), 0644)

	tests := []struct {
		name     string
		pattern  string
		validate func(*testing.T, *Result)
	}{
		{
			name:    "find go files",
			pattern: "*.go",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "test.go") {
					t.Errorf("Should find test.go")
				}
			},
		},
		{
			name:    "find test files",
			pattern: "*_test.go",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "test_test.go") {
					t.Errorf("Should find test_test.go")
				}
			},
		},
		{
			name:    "find markdown",
			pattern: "*.md",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "README.md") {
					t.Errorf("Should find README.md")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"action":  "find_files",
				"pattern": tt.pattern,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestFindInFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSearchTool(tmpDir)
	ctx := context.Background()

	testFile := filepath.Join(tmpDir, "search_test.go")
	content := `package main

import "fmt"

func TestFunc() {
	fmt.Println("test")
}

func TestAnotherFunc() {
	fmt.Println("another test")
}
`
	os.WriteFile(testFile, []byte(content), 0644)

	tests := []struct {
		name     string
		pattern  string
		validate func(*testing.T, *Result)
	}{
		{
			name:    "find function definitions",
			pattern: "^func Test",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Should find both TestFunc and TestAnotherFunc
				if !strings.Contains(r.Output, "TestFunc") {
					t.Errorf("Should find TestFunc")
				}
			},
		},
		{
			name:    "find imports",
			pattern: `^import`,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "import") {
					t.Errorf("Should find import statement")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"action":  "find_in_file",
				"path":    testFile,
				"pattern": tt.pattern,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestFindDefinition(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewSearchTool(tmpDir)
	ctx := context.Background()

	// Create test Go file
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

func HelloWorld() {
	println("hello")
}

type MyStruct struct {
	field string
}
`
	os.WriteFile(goFile, []byte(goContent), 0644)

	// Create test Python file
	pyFile := filepath.Join(tmpDir, "main.py")
	pyContent := `def hello_world():
    print("hello")

class MyClass:
    pass
`
	os.WriteFile(pyFile, []byte(pyContent), 0644)

	tests := []struct {
		name     string
		defName  string
		language string
		validate func(*testing.T, *Result)
	}{
		{
			name:     "find Go function",
			defName:  "HelloWorld",
			language: "go",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "HelloWorld") {
					t.Errorf("Should find HelloWorld function")
				}
			},
		},
		{
			name:     "find Go type",
			defName:  "MyStruct",
			language: "go",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "MyStruct") {
					t.Errorf("Should find MyStruct type")
				}
			},
		},
		{
			name:     "find Python function",
			defName:  "hello_world",
			language: "python",
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "hello_world") {
					t.Errorf("Should find hello_world function")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"action":   "find_definition",
				"name":     tt.defName,
				"language": tt.language,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestSearchToolUnknownAction(t *testing.T) {
	tool := NewSearchTool("/tmp")
	ctx := context.Background()

	args := map[string]interface{}{
		"action": "unknown",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for unknown action")
	}
}
