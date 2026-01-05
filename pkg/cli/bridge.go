package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/mcp"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// CLIBridge provides a unified interface for the CLI to call tools
// It can use either MCP client (subprocess) or direct tool execution
type CLIBridge struct {
	bridge    toolformat.ToolBridge
	mcpClient *mcp.Client
	ctx       context.Context
	cancel    context.CancelFunc
}

// CLIBridgeConfig configures the CLI bridge
type CLIBridgeConfig struct {
	UseDirect bool           // Default mode (true = direct, false = MCP subprocess)
	Config    *config.Config // App config - can override UseDirect via Execution.DirectMode
	WorkDir   string         // Working directory for tools
}

// shouldUseDirect determines if direct mode should be used
// Priority: config.Execution.DirectMode (if explicitly set) > UseDirect default
func (cfg CLIBridgeConfig) shouldUseDirect() bool {
	// If config explicitly sets direct_mode to false, use MCP subprocess
	// (This allows opting out of the new default)
	if cfg.Config != nil {
		// Config setting takes precedence when explicitly configured
		return cfg.Config.Execution.DirectMode || cfg.UseDirect
	}
	// Fall back to UseDirect default
	return cfg.UseDirect
}

// NewCLIBridge creates a new CLI bridge
// Default is direct execution (in-process). Set config.Execution.DirectMode=false for MCP subprocess.
func NewCLIBridge(cfg CLIBridgeConfig) (*CLIBridge, error) {
	if cfg.shouldUseDirect() {
		return newDirectBridge(cfg)
	}
	return newMCPBridge(cfg)
}

// newMCPBridge creates a bridge using MCP subprocess
func newMCPBridge(cfg CLIBridgeConfig) (*CLIBridge, error) {
	serverPath, err := FindMCPServer()
	if err != nil {
		return nil, err
	}

	client := mcp.NewClient(serverPath, []string{})
	ctx, cancel := context.WithCancel(context.Background())

	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Create adapter bridge
	adapter := &toolformat.MCPClientAdapter{
		MCPCaller: func(ctx context.Context, name string, args map[string]interface{}) (string, bool, error) {
			result, err := client.CallTool(ctx, name, args)
			if err != nil {
				return "", true, err
			}
			if len(result.Content) == 0 {
				return "", result.IsError, nil
			}
			return result.Content[0].Text, result.IsError, nil
		},
		MCPHealthy: func() bool {
			return client.IsRunning()
		},
	}

	return &CLIBridge{
		bridge:    adapter,
		mcpClient: client,
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// newDirectBridge creates a bridge for direct tool execution
func newDirectBridge(cfg CLIBridgeConfig) (*CLIBridge, error) {
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

	// Create LLM backend for agents
	backend, err := llm.NewBackend(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM backend: %w", err)
	}

	// Create agent factory and register agents
	agentFactory := toolformat.NewAgentFactory(cfg.Config, backend, jobManager, cfg.WorkDir)
	agentFactory.WithCodeTools(agentFactory.CreateCodeTools())
	agentFactory.WithBlogTools(agentFactory.CreateBlogTools())

	if err := agentFactory.RegisterAgentsInRegistry(registry); err != nil {
		return nil, fmt.Errorf("failed to register agents: %w", err)
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
	if b.mcpClient != nil {
		b.mcpClient.Stop()
	}
}

// FindMCPServer finds the MCP server binary
func FindMCPServer() (string, error) {
	// Try current directory first
	localPath := "./pedrocli-server"
	if _, err := os.Stat(localPath); err == nil {
		abs, _ := filepath.Abs(localPath)
		return abs, nil
	}

	// Try in same directory as the CLI binary
	exePath, err := os.Executable()
	if err == nil {
		serverPath := filepath.Join(filepath.Dir(exePath), "pedrocli-server")
		if _, err := os.Stat(serverPath); err == nil {
			return serverPath, nil
		}
	}

	// Try $PATH
	serverPath, err := exec.LookPath("pedrocli-server")
	if err == nil {
		return serverPath, nil
	}

	return "", fmt.Errorf("pedrocli-server not found. Please build it with 'make build-server'")
}
