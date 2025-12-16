package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// CodeEditTool provides precise line-based code editing
type CodeEditTool struct {
	maxFileSize int64
}

// NewCodeEditTool creates a new code edit tool
func NewCodeEditTool() *CodeEditTool {
	return &CodeEditTool{
		maxFileSize: 10 * 1024 * 1024, // 10MB max
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

	if start < 1 {
		return &Result{Success: false, Error: "start_line must be >= 1"}, nil
	}

	if end < start {
		return &Result{Success: false, Error: "end_line must be >= start_line"}, nil
	}

	// Read file
	file, err := os.Open(path)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to open file: %s", err)}, nil
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var lines []string
	lineNum := 1

	for scanner.Scan() {
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output := strings.Join(lines, "\n")
	return &Result{
		Success: true,
		Output:  output,
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

	if start < 1 {
		return &Result{Success: false, Error: "start_line must be >= 1"}, nil
	}

	if end < start {
		return &Result{Success: false, Error: "end_line must be >= start_line"}, nil
	}

	// Read all lines
	content, err := os.ReadFile(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	lines := strings.Split(string(content), "\n")

	// Validate line numbers
	if start > len(lines) {
		return &Result{Success: false, Error: fmt.Sprintf("start_line %d exceeds file length %d", start, len(lines))}, nil
	}

	if end > len(lines) {
		end = len(lines)
	}

	// Build new file content
	var newLines []string
	newLines = append(newLines, lines[:start-1]...)        // Lines before edit
	newLines = append(newLines, strings.Split(newContent, "\n")...) // New content
	newLines = append(newLines, lines[end:]...)            // Lines after edit

	// Write back
	newFileContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(path, []byte(newFileContent), 0644); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Edited lines %d-%d in %s", start, end, path),
		ModifiedFiles: []string{path},
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

	if line < 1 {
		return &Result{Success: false, Error: "line_number must be >= 1"}, nil
	}

	// Read all lines
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	lines := strings.Split(string(fileContent), "\n")

	// Insert content
	var newLines []string
	if line > len(lines) {
		// Append at end
		newLines = append(newLines, lines...)
		newLines = append(newLines, strings.Split(content, "\n")...)
	} else {
		// Insert at position
		newLines = append(newLines, lines[:line-1]...)
		newLines = append(newLines, strings.Split(content, "\n")...)
		newLines = append(newLines, lines[line-1:]...)
	}

	// Write back
	newFileContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(path, []byte(newFileContent), 0644); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Inserted content at line %d in %s", line, path),
		ModifiedFiles: []string{path},
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

	if start < 1 {
		return &Result{Success: false, Error: "start_line must be >= 1"}, nil
	}

	if end < start {
		return &Result{Success: false, Error: "end_line must be >= start_line"}, nil
	}

	// Read all lines
	content, err := os.ReadFile(path)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	lines := strings.Split(string(content), "\n")

	// Validate
	if start > len(lines) {
		return &Result{Success: false, Error: fmt.Sprintf("start_line %d exceeds file length %d", start, len(lines))}, nil
	}

	if end > len(lines) {
		end = len(lines)
	}

	// Build new content
	var newLines []string
	newLines = append(newLines, lines[:start-1]...) // Lines before deletion
	newLines = append(newLines, lines[end:]...)     // Lines after deletion

	// Write back
	newFileContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(path, []byte(newFileContent), 0644); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	return &Result{
		Success:       true,
		Output:        fmt.Sprintf("Deleted lines %d-%d in %s", start, end, path),
		ModifiedFiles: []string{path},
	}, nil
}
