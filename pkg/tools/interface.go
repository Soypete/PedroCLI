package tools

import "context"

// Tool represents an executable tool
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns the tool description
	Description() string

	// Execute executes the tool with given arguments
	Execute(ctx context.Context, args map[string]interface{}) (*Result, error)
}

// Result represents a tool execution result
type Result struct {
	Success       bool     `json:"success"`
	Output        string   `json:"output"`
	Error         string   `json:"error,omitempty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
}
