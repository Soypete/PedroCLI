package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileTool provides cross-platform file operations using pure Go (no sed/awk)
type FileTool struct {
	maxFileSize int64
}

// NewFileTool creates a new file tool
func NewFileTool() *FileTool {
	return &FileTool{
		maxFileSize: 10 * 1024 * 1024, // 10MB max
	}
}

// Name returns the tool name
func (f *FileTool) Name() string {
	return "file"
}

// Description returns the tool description
func (f *FileTool) Description() string {
	return "Read, write, and modify files using pure Go (cross-platform)"
}

// InputSchema returns the JSON Schema for tool arguments
func (f *FileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type": "string",
				"enum": []string{"read", "write", "replace", "append", "delete"},
				"description": "Action: read (read file), write (overwrite), replace (find/replace text), append (add to end), delete (remove file)",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path (relative or absolute)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content for write/append actions",
			},
			"find": map[string]interface{}{
				"type":        "string",
				"description": "Text to find (for replace action)",
			},
			"replace_with": map[string]interface{}{
				"type":        "string",
				"description": "Replacement text (for replace action)",
			},
		},
		"required": []string{"action", "path"},
	}
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

	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("file not found: %s", path)}, nil
	}

	if info.Size() > f.maxFileSize {
		return &Result{Success: false, Error: fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), f.maxFileSize)}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  string(content),
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

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to create directory: %s", err)}, nil
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Wrote %d bytes to %s", len(content), path),
		ModifiedFiles: []string{path},
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

	new, ok := args["new"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'new' parameter"}, nil
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Replace using Go strings (cross-platform, no sed)
	newContent := strings.ReplaceAll(string(content), old, new)

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Replaced '%s' with '%s' in %s", old, new, path),
		ModifiedFiles: []string{path},
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

	// Open file in append mode
	f_ptr, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}
	defer f_ptr.Close()

	// Append content
	if _, err := f_ptr.WriteString(content); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Appended %d bytes to %s", len(content), path),
		ModifiedFiles: []string{path},
	}, nil
}

// delete deletes a file
func (f *FileTool) delete(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	if err := os.Remove(path); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Deleted %s", path),
		ModifiedFiles: []string{path},
	}, nil
}
