package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeEditToolName(t *testing.T) {
	tool := NewCodeEditTool()
	if tool.Name() != "code_edit" {
		t.Errorf("Name() = %v, want code_edit", tool.Name())
	}
}

func TestCodeEditToolDescription(t *testing.T) {
	tool := NewCodeEditTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestGetLines(t *testing.T) {
	tool := NewCodeEditTool()
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	os.WriteFile(testFile, []byte(content), 0644)

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantError bool
		validate  func(*testing.T, *Result)
	}{
		{
			name: "get single line",
			args: map[string]interface{}{
				"action":     "get_lines",
				"path":       testFile,
				"start_line": float64(1),
				"end_line":   float64(1),
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "package main") {
					t.Errorf("Output = %q, want line containing 'package main'", r.Output)
				}
			},
		},
		{
			name: "get multiple lines",
			args: map[string]interface{}{
				"action":     "get_lines",
				"path":       testFile,
				"start_line": float64(5),
				"end_line":   float64(7),
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "func main") {
					t.Errorf("Output should contain 'func main', got: %q", r.Output)
				}
			},
		},
		{
			name: "invalid start line",
			args: map[string]interface{}{
				"action":     "get_lines",
				"path":       testFile,
				"start_line": float64(0),
				"end_line":   float64(5),
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for invalid start_line")
				}
			},
		},
		{
			name: "end before start",
			args: map[string]interface{}{
				"action":     "get_lines",
				"path":       testFile,
				"start_line": float64(5),
				"end_line":   float64(3),
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false when end < start")
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

func TestEditLines(t *testing.T) {
	tool := NewCodeEditTool()
	ctx := context.Background()
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		initialContent  string
		args            map[string]interface{}
		expectedContent string
		wantError       bool
	}{
		{
			name: "edit single line",
			initialContent: `line 1
line 2
line 3`,
			args: map[string]interface{}{
				"action":      "edit_lines",
				"path":        "", // Will be set
				"start_line":  float64(2),
				"end_line":    float64(2),
				"new_content": "EDITED LINE",
			},
			expectedContent: `line 1
EDITED LINE
line 3`,
		},
		{
			name: "edit multiple lines",
			initialContent: `line 1
line 2
line 3
line 4`,
			args: map[string]interface{}{
				"action":      "edit_lines",
				"path":        "",
				"start_line":  float64(2),
				"end_line":    float64(3),
				"new_content": "NEW LINE",
			},
			expectedContent: `line 1
NEW LINE
line 4`,
		},
		{
			name: "edit with multiline content",
			initialContent: `line 1
line 2
line 3`,
			args: map[string]interface{}{
				"action":      "edit_lines",
				"path":        "",
				"start_line":  float64(2),
				"end_line":    float64(2),
				"new_content": "new line 1\nnew line 2",
			},
			expectedContent: `line 1
new line 1
new line 2
line 3`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "edit_test.txt")
			os.WriteFile(testFile, []byte(tt.initialContent), 0644)

			tt.args["path"] = testFile

			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			if !result.Success {
				t.Errorf("Success = false, want true. Error: %s", result.Error)
			}

			// Verify file content
			content, _ := os.ReadFile(testFile)
			if string(content) != tt.expectedContent {
				t.Errorf("File content = %q, want %q", string(content), tt.expectedContent)
			}
		})
	}
}

func TestInsertAtLine(t *testing.T) {
	tool := NewCodeEditTool()
	ctx := context.Background()
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		initialContent  string
		lineNumber      float64
		insertContent   string
		expectedContent string
	}{
		{
			name: "insert at beginning",
			initialContent: `line 1
line 2`,
			lineNumber:    1,
			insertContent: "NEW FIRST LINE",
			expectedContent: `NEW FIRST LINE
line 1
line 2`,
		},
		{
			name: "insert in middle",
			initialContent: `line 1
line 2
line 3`,
			lineNumber:    2,
			insertContent: "INSERTED",
			expectedContent: `line 1
INSERTED
line 2
line 3`,
		},
		{
			name: "insert at end",
			initialContent: `line 1
line 2`,
			lineNumber:    100, // Beyond end
			insertContent: "APPENDED",
			expectedContent: `line 1
line 2
APPENDED`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "insert_test.txt")
			os.WriteFile(testFile, []byte(tt.initialContent), 0644)

			args := map[string]interface{}{
				"action":      "insert_at_line",
				"path":        testFile,
				"line_number": tt.lineNumber,
				"content":     tt.insertContent,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			if !result.Success {
				t.Errorf("Success = false, want true. Error: %s", result.Error)
			}

			// Verify file content
			content, _ := os.ReadFile(testFile)
			if string(content) != tt.expectedContent {
				t.Errorf("File content = %q, want %q", string(content), tt.expectedContent)
			}
		})
	}
}

func TestDeleteLines(t *testing.T) {
	tool := NewCodeEditTool()
	ctx := context.Background()
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		initialContent  string
		startLine       float64
		endLine         float64
		expectedContent string
	}{
		{
			name: "delete single line",
			initialContent: `line 1
line 2
line 3`,
			startLine: 2,
			endLine:   2,
			expectedContent: `line 1
line 3`,
		},
		{
			name: "delete multiple lines",
			initialContent: `line 1
line 2
line 3
line 4`,
			startLine: 2,
			endLine:   3,
			expectedContent: `line 1
line 4`,
		},
		{
			name: "delete from beginning",
			initialContent: `line 1
line 2
line 3`,
			startLine:       1,
			endLine:         2,
			expectedContent: `line 3`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "delete_test.txt")
			os.WriteFile(testFile, []byte(tt.initialContent), 0644)

			args := map[string]interface{}{
				"action":     "delete_lines",
				"path":       testFile,
				"start_line": tt.startLine,
				"end_line":   tt.endLine,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			if !result.Success {
				t.Errorf("Success = false, want true. Error: %s", result.Error)
			}

			// Verify file content
			content, _ := os.ReadFile(testFile)
			if string(content) != tt.expectedContent {
				t.Errorf("File content = %q, want %q", string(content), tt.expectedContent)
			}
		})
	}
}

func TestCodeEditToolUnknownAction(t *testing.T) {
	tool := NewCodeEditTool()
	ctx := context.Background()

	args := map[string]interface{}{
		"action": "unknown_action",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for unknown action")
	}
}
