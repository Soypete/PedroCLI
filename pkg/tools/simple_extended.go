package tools

import "context"

// SimpleExtendedTool wraps a basic Tool to implement ExtendedTool
// with nil metadata. This provides backward compatibility for legacy
// tools that haven't been migrated to provide metadata.
type SimpleExtendedTool struct {
	tool Tool
}

// NewSimpleExtendedTool wraps a Tool as an ExtendedTool
func NewSimpleExtendedTool(tool Tool) *SimpleExtendedTool {
	return &SimpleExtendedTool{tool: tool}
}

// Name returns the tool name
func (s *SimpleExtendedTool) Name() string {
	return s.tool.Name()
}

// Description returns the tool description
func (s *SimpleExtendedTool) Description() string {
	return s.tool.Description()
}

// Execute runs the wrapped tool
func (s *SimpleExtendedTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	return s.tool.Execute(ctx, args)
}

// Metadata returns nil for legacy tools without metadata
func (s *SimpleExtendedTool) Metadata() *ToolMetadata {
	return nil
}

// Unwrap returns the underlying Tool
func (s *SimpleExtendedTool) Unwrap() Tool {
	return s.tool
}
