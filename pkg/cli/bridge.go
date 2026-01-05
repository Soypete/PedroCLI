package cli

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// CLIBridge provides a unified interface for the CLI to call tools using direct execution
type CLIBridge struct {
	bridge toolformat.ToolBridge
	ctx    context.Context
	cancel context.CancelFunc
}

// CLIBridgeConfig configures the CLI bridge
type CLIBridgeConfig struct {
	Config  *config.Config // App config
	WorkDir string         // Working directory for tools
}

// NewCLIBridge creates a new CLI bridge using direct tool execution
func NewCLIBridge(cfg CLIBridgeConfig) (*CLIBridge, error) {
	// Create tool factory and registry
	factory := toolformat.NewToolFactory(cfg.Config, cfg.WorkDir)
	registry, err := factory.CreateRegistryForMode(toolformat.ModeAll)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}

	// Register job management tools
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	// Register job tools
	jobTools := []tools.Tool{
		tools.NewGetJobStatusTool(jobManager),
		tools.NewListJobsTool(jobManager),
		tools.NewCancelJobTool(jobManager),
	}

	for _, tool := range jobTools {
		def := &toolformat.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Category:    toolformat.CategoryJob,
			Parameters:  toolformat.GetSchemaForTool(tool.Name()),
			Handler: func(t tools.Tool) toolformat.ToolHandler {
				return func(args map[string]interface{}) (*toolformat.ToolResult, error) {
					result, err := t.Execute(context.Background(), args)
					if err != nil {
						return &toolformat.ToolResult{Success: false, Error: err.Error()}, nil
					}
					return &toolformat.ToolResult{
						Success: result.Success,
						Output:  result.Output,
						Error:   result.Error,
					}, nil
				}
			}(tool),
		}
		registry.Register(def)
	}

	// Get formatter for configured model
	modelName := "generic"
	if cfg.Config != nil && cfg.Config.Model.ModelName != "" {
		modelName = cfg.Config.Model.ModelName
	}
	formatter := toolformat.GetFormatterForModel(modelName)

	// Create direct bridge
	bridge := toolformat.NewDirectBridge(registry, formatter)
	ctx := context.Background()

	return &CLIBridge{
		bridge: bridge,
		ctx:    ctx,
		cancel: func() {},
	}, nil
}

// CallTool calls a tool through the bridge
func (b *CLIBridge) CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolformat.BridgeResult, error) {
	return b.bridge.CallTool(ctx, name, args)
}

// IsHealthy returns whether the bridge is healthy
func (b *CLIBridge) IsHealthy() bool {
	return b.bridge.IsHealthy()
}

// GetToolNames returns available tool names
func (b *CLIBridge) GetToolNames() []string {
	return b.bridge.GetToolNames()
}

// Context returns the bridge's context
func (b *CLIBridge) Context() context.Context {
	return b.ctx
}

// Close shuts down the bridge
func (b *CLIBridge) Close() {
	if b.cancel != nil {
		b.cancel()
	}
}
