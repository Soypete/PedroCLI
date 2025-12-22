package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileToolName(t *testing.T) {
	tool := NewFileTool()
	if tool.Name() != "file" {
		t.Errorf("Name() = %v, want file", tool.Name())
	}
}

func TestFileToolDescription(t *testing.T) {
	tool := NewFileTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFileToolRead(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(string) string // Returns file path
		args     map[string]interface{}
		wantErr  bool
		wantMsg  string
		validate func(*testing.T, *Result)
	}{
		{
			name: "read valid file",
			setup: func(dir string) string {
				path := filepath.Join(dir, "test.txt")
				os.WriteFile(path, []byte("hello world"), 0644)
				return path
			},
			args:    nil, // will be set in test
			wantErr: false,
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true")
				}
				if r.Output != "hello world" {
					t.Errorf("Output = %q, want %q", r.Output, "hello world")
				}
			},
		},
		{
			name: "read nonexistent file",
			setup: func(dir string) string {
				return filepath.Join(dir, "nonexistent.txt")
			},
			args:    nil,
			wantErr: false,
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for nonexistent file")
				}
				if !strings.Contains(r.Error, "file not found") {
					t.Errorf("Error = %q, want error containing 'file not found'", r.Error)
				}
			},
		},
		{
			name: "missing path parameter",
			setup: func(dir string) string {
				return ""
			},
			args: map[string]interface{}{
				"action": "read",
			},
			wantErr: false,
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for missing path")
				}
				if !strings.Contains(r.Error, "missing 'path' parameter") {
					t.Errorf("Error = %q, want error containing 'missing 'path' parameter'", r.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := tt.setup(tmpDir)

			args := tt.args
			if args == nil && path != "" {
				args = map[string]interface{}{
					"action": "read",
					"path":   path,
				}
			}

			result, err := tool.Execute(ctx, args)

			if tt.wantErr && err == nil {
				t.Error("Execute() error = nil, want error")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFileToolWrite(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name     string
		args     func(string) map[string]interface{}
		validate func(*testing.T, string, *Result)
	}{
		{
			name: "write new file",
			args: func(dir string) map[string]interface{} {
				return map[string]interface{}{
					"action":  "write",
					"path":    filepath.Join(dir, "new.txt"),
					"content": "test content",
				}
			},
			validate: func(t *testing.T, dir string, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Verify file was created
				content, err := os.ReadFile(filepath.Join(dir, "new.txt"))
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
				}
				if string(content) != "test content" {
					t.Errorf("File content = %q, want %q", string(content), "test content")
				}
			},
		},
		{
			name: "write to nested directory",
			args: func(dir string) map[string]interface{} {
				return map[string]interface{}{
					"action":  "write",
					"path":    filepath.Join(dir, "nested", "dir", "file.txt"),
					"content": "nested content",
				}
			},
			validate: func(t *testing.T, dir string, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Verify nested directory was created
				content, err := os.ReadFile(filepath.Join(dir, "nested", "dir", "file.txt"))
				if err != nil {
					t.Errorf("Failed to read nested file: %v", err)
				}
				if string(content) != "nested content" {
					t.Errorf("File content = %q, want %q", string(content), "nested content")
				}
			},
		},
		{
			name: "overwrite existing file",
			args: func(dir string) map[string]interface{} {
				path := filepath.Join(dir, "existing.txt")
				os.WriteFile(path, []byte("old content"), 0644)
				return map[string]interface{}{
					"action":  "write",
					"path":    path,
					"content": "new content",
				}
			},
			validate: func(t *testing.T, dir string, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				content, err := os.ReadFile(filepath.Join(dir, "existing.txt"))
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
				}
				if string(content) != "new content" {
					t.Errorf("File content = %q, want %q", string(content), "new content")
				}
			},
		},
		{
			name: "missing content parameter",
			args: func(dir string) map[string]interface{} {
				return map[string]interface{}{
					"action": "write",
					"path":   filepath.Join(dir, "test.txt"),
				}
			},
			validate: func(t *testing.T, dir string, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for missing content")
				}
				if !strings.Contains(r.Error, "missing 'content' parameter") {
					t.Errorf("Error = %q, want error containing 'missing 'content' parameter'", r.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			args := tt.args(tmpDir)

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			tt.validate(t, tmpDir, result)
		})
	}
}

func TestFileToolReplace(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name     string
		initial  string
		old      string
		new      string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple replacement",
			initial:  "hello world",
			old:      "world",
			new:      "universe",
			expected: "hello universe",
			wantErr:  false,
		},
		{
			name:     "multiple occurrences",
			initial:  "foo bar foo baz foo",
			old:      "foo",
			new:      "qux",
			expected: "qux bar qux baz qux",
			wantErr:  false,
		},
		{
			name:     "no match",
			initial:  "hello world",
			old:      "xyz",
			new:      "abc",
			expected: "hello world",
			wantErr:  false,
		},
		{
			name:     "multiline replacement",
			initial:  "line1\nline2\nline3",
			old:      "line2",
			new:      "LINE2",
			expected: "line1\nLINE2\nline3",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.txt")

			// Write initial content
			if err := os.WriteFile(path, []byte(tt.initial), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			args := map[string]interface{}{
				"action": "replace",
				"path":   path,
				"old":    tt.old,
				"new":    tt.new,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			if !result.Success {
				t.Errorf("Success = false, want true. Error: %s", result.Error)
			}

			// Verify content
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read file: %v", err)
			}

			if string(content) != tt.expected {
				t.Errorf("File content = %q, want %q", string(content), tt.expected)
			}
		})
	}
}

func TestFileToolAppend(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name     string
		initial  string
		append   string
		expected string
	}{
		{
			name:     "append to existing file",
			initial:  "hello",
			append:   " world",
			expected: "hello world",
		},
		{
			name:     "append to empty file",
			initial:  "",
			append:   "content",
			expected: "content",
		},
		{
			name:     "append newline",
			initial:  "line1",
			append:   "\nline2",
			expected: "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.txt")

			// Write initial content
			if err := os.WriteFile(path, []byte(tt.initial), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			args := map[string]interface{}{
				"action":  "append",
				"path":    path,
				"content": tt.append,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			if !result.Success {
				t.Errorf("Success = false, want true. Error: %s", result.Error)
			}

			// Verify content
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read file: %v", err)
			}

			if string(content) != tt.expected {
				t.Errorf("File content = %q, want %q", string(content), tt.expected)
			}
		})
	}
}

func TestFileToolAppendToNewFile(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "newfile.txt")

	args := map[string]interface{}{
		"action":  "append",
		"path":    path,
		"content": "new content",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true. Error: %s", result.Error)
	}

	// Verify file was created
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}

	if string(content) != "new content" {
		t.Errorf("File content = %q, want %q", string(content), "new content")
	}
}

func TestFileToolDelete(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(string) string
		wantErr  bool
		validate func(*testing.T, string, *Result)
	}{
		{
			name: "delete existing file",
			setup: func(dir string) string {
				path := filepath.Join(dir, "todelete.txt")
				os.WriteFile(path, []byte("content"), 0644)
				return path
			},
			wantErr: false,
			validate: func(t *testing.T, path string, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				// Verify file was deleted
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Error("File still exists after delete")
				}
			},
		},
		{
			name: "delete nonexistent file",
			setup: func(dir string) string {
				return filepath.Join(dir, "nonexistent.txt")
			},
			wantErr: false,
			validate: func(t *testing.T, path string, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for nonexistent file")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := tt.setup(tmpDir)

			args := map[string]interface{}{
				"action": "delete",
				"path":   path,
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			tt.validate(t, path, result)
		})
	}
}

func TestFileToolUnknownAction(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	args := map[string]interface{}{
		"action": "invalid_action",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for unknown action")
	}

	if !strings.Contains(result.Error, "unknown action") {
		t.Errorf("Error = %q, want error containing 'unknown action'", result.Error)
	}
}

func TestFileToolMissingAction(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	args := map[string]interface{}{
		"path": "/tmp/test.txt",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for missing action")
	}

	if !strings.Contains(result.Error, "missing 'action' parameter") {
		t.Errorf("Error = %q, want error containing 'missing 'action' parameter'", result.Error)
	}
}

func TestFileToolMaxFileSize(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.txt")

	// Create a file larger than 10MB
	largeContent := make([]byte, 11*1024*1024) // 11MB
	if err := os.WriteFile(path, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	args := map[string]interface{}{
		"action": "read",
		"path":   path,
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for file exceeding max size")
	}

	if !strings.Contains(result.Error, "file too large") {
		t.Errorf("Error = %q, want error containing 'file too large'", result.Error)
	}
}
