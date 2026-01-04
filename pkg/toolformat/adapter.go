package toolformat

import (
	"context"

	"github.com/soypete/pedrocli/pkg/tools"
)

// SchemaProvider is an optional interface that tools can implement
// to provide their parameter schema for the new tool format system
type SchemaProvider interface {
	Schema() ParameterSchema
}

// CategoryProvider is an optional interface that tools can implement
// to specify their category
type CategoryProvider interface {
	Category() ToolCategory
}

// ToolAdapter wraps a legacy tools.Tool to work with the new toolformat system
type ToolAdapter struct {
	tool     tools.Tool
	category ToolCategory
	schema   ParameterSchema
}

// NewToolAdapter creates an adapter for a legacy tool
func NewToolAdapter(tool tools.Tool) *ToolAdapter {
	adapter := &ToolAdapter{
		tool:     tool,
		category: CategoryCode, // Default category
	}

	// Check if tool implements SchemaProvider
	if sp, ok := tool.(SchemaProvider); ok {
		adapter.schema = sp.Schema()
	} else {
		// Generate basic schema from description
		adapter.schema = adapter.inferSchemaFromDescription()
	}

	// Check if tool implements CategoryProvider
	if cp, ok := tool.(CategoryProvider); ok {
		adapter.category = cp.Category()
	}

	return adapter
}

// NewToolAdapterWithCategory creates an adapter with a specific category
func NewToolAdapterWithCategory(tool tools.Tool, category ToolCategory) *ToolAdapter {
	adapter := NewToolAdapter(tool)
	adapter.category = category
	return adapter
}

// ToDefinition converts the adapted tool to a ToolDefinition
func (a *ToolAdapter) ToDefinition() *ToolDefinition {
	return &ToolDefinition{
		Name:        a.tool.Name(),
		Description: a.tool.Description(),
		Category:    a.category,
		Parameters:  a.schema,
		Handler:     a.createHandler(),
	}
}

// createHandler creates a ToolHandler from the legacy tool
func (a *ToolAdapter) createHandler() ToolHandler {
	return func(args map[string]interface{}) (*ToolResult, error) {
		result, err := a.tool.Execute(context.Background(), args)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &ToolResult{
			Success:       result.Success,
			Output:        result.Output,
			Error:         result.Error,
			ModifiedFiles: result.ModifiedFiles,
			Data:          result.Data,
		}, nil
	}
}

// inferSchemaFromDescription attempts to infer a basic schema from the tool description
// This is a fallback for tools that don't implement SchemaProvider
func (a *ToolAdapter) inferSchemaFromDescription() ParameterSchema {
	schema := NewParameterSchema()

	// Add a generic action parameter that most tools use
	schema.AddProperty("action", StringProperty("The action to perform"), true)

	return schema
}

// RegisterLegacyTools registers multiple legacy tools with a registry
func RegisterLegacyTools(registry *Registry, toolList []tools.Tool, category ToolCategory) error {
	for _, tool := range toolList {
		adapter := NewToolAdapterWithCategory(tool, category)
		if err := registry.Register(adapter.ToDefinition()); err != nil {
			return err
		}
	}
	return nil
}

// AdaptLegacyTool is a convenience function to convert a single legacy tool
func AdaptLegacyTool(tool tools.Tool, category ToolCategory) *ToolDefinition {
	return NewToolAdapterWithCategory(tool, category).ToDefinition()
}

// CreateContextHandler creates a handler that includes context
func CreateContextHandler(tool tools.Tool) func(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return func(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
		result, err := tool.Execute(ctx, args)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &ToolResult{
			Success:       result.Success,
			Output:        result.Output,
			Error:         result.Error,
			ModifiedFiles: result.ModifiedFiles,
			Data:          result.Data,
		}, nil
	}
}
