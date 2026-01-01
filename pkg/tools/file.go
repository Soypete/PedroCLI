package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/fileio"
	"github.com/soypete/pedrocli/pkg/logits"
)

// FileTool provides cross-platform file operations using pure Go (no sed/awk)
type FileTool struct {
	maxFileSize int64
	fs          *fileio.FileSystem
}

// NewFileTool creates a new file tool
func NewFileTool() *FileTool {
	return &FileTool{
		maxFileSize: 10 * 1024 * 1024, // 10MB max
		fs:          fileio.NewFileSystem(),
	}
}

// NewFileToolWithFileSystem creates a new file tool with a custom FileSystem
func NewFileToolWithFileSystem(fs *fileio.FileSystem) *FileTool {
	return &FileTool{
		maxFileSize: 10 * 1024 * 1024,
		fs:          fs,
	}
}

// Name returns the tool name
func (f *FileTool) Name() string {
	return "file"
}

// Description returns the tool description
func (f *FileTool) Description() string {
	return `Read, write, and modify files.

Actions:
- read: Read entire file content
  Args: path (string)
- write: Write content to a file (creates parent directories)
  Args: path (string), content (string)
- replace: Replace text in a file
  Args: path (string), old (string), new (string)
- append: Append content to a file
  Args: path (string), content (string)
- delete: Delete a file
  Args: path (string)

IMPORTANT:
- ALWAYS read a file before modifying it to understand its content
- Use code_edit tool for precise line-based changes
- This tool replaces sed/awk - never use those via bash

Example: {"tool": "file", "args": {"action": "read", "path": "main.go"}}`
}

// Execute executes the file tool
func (f *FileTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "read":
		return f.read(args)
	case "write":
		return f.write(args)
	case "replace":
		return f.replace(args)
	case "append":
		return f.append(args)
	case "delete":
		return f.delete(args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// read reads a file
func (f *FileTool) read(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	content, err := f.fs.ReadFileString(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  content,
	}, nil
}

// write writes content to a file
func (f *FileTool) write(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'content' parameter"}, nil
	}

	// Use fileio to write file (handles directory creation and atomic writes)
	if err := f.fs.WriteFileString(path, content); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	absPath, _ := filepath.Abs(path)
	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// replace replaces text in a file (using Go strings, not sed)
func (f *FileTool) replace(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	old, ok := args["old"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'old' parameter"}, nil
	}

	newStr, ok := args["new"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'new' parameter"}, nil
	}

	// Use fileio to replace in file
	count, err := f.fs.ReplaceInFile(path, old, newStr)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	absPath, _ := filepath.Abs(path)
	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Replaced %d occurrence(s) of '%s' with '%s' in %s", count, old, newStr, path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// append appends content to a file
func (f *FileTool) append(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'content' parameter"}, nil
	}

	// Use fileio to append to file
	if err := f.fs.AppendFileString(path, content); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	absPath, _ := filepath.Abs(path)
	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Appended %d bytes to %s", len(content), path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// delete deletes a file
func (f *FileTool) delete(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	absPath, _ := filepath.Abs(path)
	if err := f.fs.DeleteFile(path); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Deleted %s", path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// Metadata returns rich tool metadata for discovery and LLM guidance
func (f *FileTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []interface{}{"read", "write", "replace", "append", "delete"},
					Description: "The file operation to perform",
				},
				"path": {
					Type:        "string",
					Description: "The file path to operate on",
				},
				"content": {
					Type:        "string",
					Description: "Content for write/append operations",
				},
				"old": {
					Type:        "string",
					Description: "Text to find for replace operation",
				},
				"new": {
					Type:        "string",
					Description: "Replacement text for replace operation",
				},
			},
			Required: []string{"action", "path"},
		},
		Category:    CategoryCode,
		Optionality: ToolRequired,
		UsageHint:   "Always read a file before modifying it. Use code_edit for precise line-based changes.",
		Examples: []ToolExample{
			{
				Description: "Read a file",
				Input:       map[string]interface{}{"action": "read", "path": "main.go"},
			},
			{
				Description: "Write a new file",
				Input:       map[string]interface{}{"action": "write", "path": "config.json", "content": `{"key": "value"}`},
			},
			{
				Description: "Replace text in a file",
				Input:       map[string]interface{}{"action": "replace", "path": "main.go", "old": "oldFunc", "new": "newFunc"},
			},
		},
		Produces: []string{"file_content"},
	}
}

// GetFileInfo returns file metadata including language detection
func (f *FileTool) GetFileInfo(path string) (*fileio.FileInfo, error) {
	return f.fs.GetFileInfo(path)
}

// GetFileSystem returns the underlying FileSystem for advanced operations
func (f *FileTool) GetFileSystem() *fileio.FileSystem {
	return f.fs
}
