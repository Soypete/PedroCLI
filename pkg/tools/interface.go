package tools

import (
	"context"

	"github.com/soypete/pedrocli/pkg/logits"
)

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
	Success       bool                   `json:"success"`
	Output        string                 `json:"output"`
	Error         string                 `json:"error,omitempty"`
	ModifiedFiles []string               `json:"modified_files,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// ErrorResult creates an error result with the given message
func ErrorResult(msg string) *Result {
	return &Result{
		Success: false,
		Error:   msg,
	}
}

// ToolCategory represents the functional category of a tool
type ToolCategory string

const (
	CategoryCode     ToolCategory = "code"
	CategoryVCS      ToolCategory = "vcs"
	CategoryBuild    ToolCategory = "build"
	CategoryResearch ToolCategory = "research"
	CategoryPublish  ToolCategory = "publish"
	CategoryUtility  ToolCategory = "utility"
)

// ToolOptionality indicates whether a tool is required or optional
type ToolOptionality string

const (
	ToolRequired    ToolOptionality = "required"
	ToolOptional    ToolOptionality = "optional"
	ToolConditional ToolOptionality = "conditional"
)

// ToolExample represents an example usage of a tool
type ToolExample struct {
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      string                 `json:"output,omitempty"`
}

// ToolMetadata provides rich information about a tool for
// discovery, documentation, and LLM guidance
type ToolMetadata struct {
	// Schema defines the JSON schema for tool arguments
	Schema *logits.JSONSchema

	// Category is the functional category (code, vcs, build, etc.)
	Category ToolCategory

	// Optionality indicates if the tool is required or optional
	Optionality ToolOptionality

	// UsageHint provides guidance to LLMs on when to use this tool
	UsageHint string

	// Examples show example invocations
	Examples []ToolExample

	// RequiresCapabilities lists capabilities needed (e.g., "git", "network")
	RequiresCapabilities []string

	// Consumes lists artifact types this tool can use as input
	Consumes []string

	// Produces lists artifact types this tool generates
	Produces []string
}

// ExtendedTool extends Tool with metadata for dynamic discovery
type ExtendedTool interface {
	Tool
	// Metadata returns rich tool information for discovery and LLM guidance
	Metadata() *ToolMetadata
}
