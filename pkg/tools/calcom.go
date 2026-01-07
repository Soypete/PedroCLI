package tools

import (
	"context"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/tools/calcom"
)

// calComToolWrapper wraps calcom.CalComTool to implement tools.Tool interface
type calComToolWrapper struct {
	tool *calcom.CalComTool
}

// NewCalComTool creates a new Cal.com scheduling tool
func NewCalComTool(cfg *config.Config, tokenMgr TokenManager) Tool {
	return &calComToolWrapper{
		tool: calcom.NewCalComTool(cfg, tokenMgr),
	}
}

func (w *calComToolWrapper) Name() string {
	return w.tool.Name()
}

func (w *calComToolWrapper) Description() string {
	return w.tool.Description()
}

func (w *calComToolWrapper) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Call the calcom tool's Execute method
	result, err := w.tool.Execute(ctx, args)
	if err != nil {
		return nil, err
	}

	// Convert calcom.Result to tools.Result
	return &Result{
		Success:       result.Success,
		Output:        result.Output,
		Error:         result.Error,
		ModifiedFiles: result.ModifiedFiles,
		Data:          result.Data,
	}, nil
}

func (w *calComToolWrapper) Metadata() *ToolMetadata {
	// Get metadata from calcom tool
	calcomMeta := w.tool.Metadata()
	if calcomMeta == nil {
		return nil
	}

	// Convert calcom.ToolMetadata to tools.ToolMetadata
	examples := make([]ToolExample, len(calcomMeta.Examples))
	for i, ex := range calcomMeta.Examples {
		examples[i] = ToolExample{
			Description: ex.Description,
			Input:       ex.Arguments,
		}
	}

	return &ToolMetadata{
		Schema:               calcomMeta.Schema,
		Category:             ToolCategory(calcomMeta.Category),
		Optionality:          ToolOptionality(calcomMeta.Optionality),
		UsageHint:            calcomMeta.UsageHint,
		Examples:             examples,
		RequiresCapabilities: calcomMeta.RequiresCapabilities,
		Consumes:             calcomMeta.Consumes,
		Produces:             calcomMeta.Produces,
	}
}
