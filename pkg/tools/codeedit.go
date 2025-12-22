package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/fileio"
)

// CodeEditTool provides precise line-based code editing
type CodeEditTool struct {
	maxFileSize int64
	fs          *fileio.FileSystem
}

// NewCodeEditTool creates a new code edit tool
func NewCodeEditTool() *CodeEditTool {
	return &CodeEditTool{
		maxFileSize: 10 * 1024 * 1024, // 10MB max
		fs:          fileio.NewFileSystem(),
	}
}

// NewCodeEditToolWithFileSystem creates a new code edit tool with a custom FileSystem
func NewCodeEditToolWithFileSystem(fs *fileio.FileSystem) *CodeEditTool {
	return &CodeEditTool{
		maxFileSize: 10 * 1024 * 1024,
		fs:          fs,
	}
}

// Name returns the tool name
func (c *CodeEditTool) Name() string {
	return "code_edit"
}

// Description returns the tool description
func (c *CodeEditTool) Description() string {
	return "Precise line-based code editing (edit ranges, insert at line, delete lines)"
}

// Execute executes the code edit tool
func (c *CodeEditTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "get_lines":
		return c.getLines(args)
	case "edit_lines":
		return c.editLines(args)
	case "insert_at_line":
		return c.insertAtLine(args)
	case "delete_lines":
		return c.deleteLines(args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// getLines reads specific line range from a file
func (c *CodeEditTool) getLines(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	startLine, ok := args["start_line"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'start_line' parameter"}, nil
	}

	endLine, ok := args["end_line"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'end_line' parameter"}, nil
	}

	start := int(startLine)
	end := int(endLine)

	// Use fileio to read lines
	content, err := c.fs.ReadLinesString(path, start, end)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success: true,
		Output:  content,
	}, nil
}

// editLines replaces a range of lines with new content
func (c *CodeEditTool) editLines(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	startLine, ok := args["start_line"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'start_line' parameter"}, nil
	}

	endLine, ok := args["end_line"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'end_line' parameter"}, nil
	}

	newContent, ok := args["new_content"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'new_content' parameter"}, nil
	}

	start := int(startLine)
	end := int(endLine)

	// Use fileio to edit lines
	if err := c.fs.EditLines(path, start, end, newContent); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	absPath, _ := filepath.Abs(path)
	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Edited lines %d-%d in %s", start, end, path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// insertAtLine inserts content at a specific line number
func (c *CodeEditTool) insertAtLine(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	lineNum, ok := args["line_number"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'line_number' parameter"}, nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'content' parameter"}, nil
	}

	line := int(lineNum)

	// Use fileio to insert at line
	if err := c.fs.InsertAtLine(path, line, content); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	absPath, _ := filepath.Abs(path)
	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Inserted content at line %d in %s", line, path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// deleteLines deletes a range of lines
func (c *CodeEditTool) deleteLines(args map[string]interface{}) (*Result, error) {
	path, ok := args["path"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'path' parameter"}, nil
	}

	startLine, ok := args["start_line"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'start_line' parameter"}, nil
	}

	endLine, ok := args["end_line"].(float64)
	if !ok {
		return &Result{Success: false, Error: "missing 'end_line' parameter"}, nil
	}

	start := int(startLine)
	end := int(endLine)

	// Use fileio to delete lines
	if err := c.fs.DeleteLines(path, start, end); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	absPath, _ := filepath.Abs(path)
	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Deleted lines %d-%d in %s", start, end, path),
		ModifiedFiles: []string{absPath},
	}, nil
}

// GetFileSystem returns the underlying FileSystem for advanced operations
func (c *CodeEditTool) GetFileSystem() *fileio.FileSystem {
	return c.fs
}
