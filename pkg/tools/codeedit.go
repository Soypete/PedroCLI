package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/fileio"
	"github.com/soypete/pedrocli/pkg/logits"
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
	return `Precise line-based code editing. Preferred for targeted changes.

Actions:
- get_lines: Read specific line range
  Args: path (string), start_line (int), end_line (int)
- edit_lines: Replace a range of lines with new content
  Args: path (string), start_line (int), end_line (int), new_content (string)
- insert_at_line: Insert content at a specific line
  Args: path (string), line_number (int), content (string)
- delete_lines: Delete a range of lines
  Args: path (string), start_line (int), end_line (int)

Usage Tips:
- Use get_lines first to see current content before editing
- Line numbers are 1-indexed (first line is 1)
- Preserve exact indentation when providing new_content
- Prefer this tool over file tool for surgical changes

Example: {"tool": "code_edit", "args": {"action": "edit_lines", "path": "main.go", "start_line": 10, "end_line": 15, "new_content": "// new code here"}}`
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

// Metadata returns rich tool metadata for discovery and LLM guidance
func (c *CodeEditTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []interface{}{"get_lines", "edit_lines", "insert_at_line", "delete_lines"},
					Description: "The code edit operation to perform",
				},
				"path": {
					Type:        "string",
					Description: "File path to edit",
				},
				"start_line": {
					Type:        "integer",
					Description: "Starting line number (1-indexed)",
				},
				"end_line": {
					Type:        "integer",
					Description: "Ending line number (1-indexed)",
				},
				"line_number": {
					Type:        "integer",
					Description: "Line number for insert_at_line (1-indexed)",
				},
				"new_content": {
					Type:        "string",
					Description: "New content for edit_lines",
				},
				"content": {
					Type:        "string",
					Description: "Content to insert for insert_at_line",
				},
			},
			Required: []string{"action", "path"},
		},
		Category:    CategoryCode,
		Optionality: ToolRequired,
		UsageHint:   "Use get_lines first to see current content before editing. Preserve exact indentation.",
		Examples: []ToolExample{
			{
				Description: "Read lines 10-20 from a file",
				Input:       map[string]interface{}{"action": "get_lines", "path": "main.go", "start_line": 10, "end_line": 20},
			},
			{
				Description: "Replace lines 10-15 with new content",
				Input:       map[string]interface{}{"action": "edit_lines", "path": "main.go", "start_line": 10, "end_line": 15, "new_content": "// new code"},
			},
		},
		Consumes: []string{"file_content"},
		Produces: []string{"file_content"},
	}
}

// GetFileSystem returns the underlying FileSystem for advanced operations
func (c *CodeEditTool) GetFileSystem() *fileio.FileSystem {
	return c.fs
}
