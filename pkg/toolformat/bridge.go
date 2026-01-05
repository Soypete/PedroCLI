package toolformat

import (
	"context"
	"fmt"
	"strings"
)

// ToolBridge provides a unified interface for tool execution
// that can be backed by either the MCP client or direct toolformat execution.
// This enables gradual migration from MCP to the new toolformat system.
type ToolBridge interface {
	// CallTool executes a tool and returns the result
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*BridgeResult, error)

	// IsHealthy checks if the bridge is operational
	IsHealthy() bool

	// GetToolNames returns the list of available tool names
	GetToolNames() []string
}

// BridgeResult represents a tool execution result in a format
// compatible with both MCP responses and toolformat results
type BridgeResult struct {
	Success       bool                   `json:"success"`
	Output        string                 `json:"output"`
	Error         string                 `json:"error,omitempty"`
	ModifiedFiles []string               `json:"modified_files,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// DirectBridge implements ToolBridge using the toolformat package directly
type DirectBridge struct {
	registry *Registry
	executor *ToolExecutor
}

// NewDirectBridge creates a bridge that executes tools directly via toolformat
func NewDirectBridge(registry *Registry, formatter ToolFormatter) *DirectBridge {
	executor := NewToolExecutor(ExecutorConfig{
		Registry:  registry,
		Formatter: formatter,
	})

	return &DirectBridge{
		registry: registry,
		executor: executor,
	}
}

// CallTool executes a tool directly using the registry
func (b *DirectBridge) CallTool(ctx context.Context, name string, args map[string]interface{}) (*BridgeResult, error) {
	result, err := b.registry.Execute(ctx, name, args)
	if err != nil {
		return &BridgeResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &BridgeResult{
		Success:       result.Success,
		Output:        result.Output,
		Error:         result.Error,
		ModifiedFiles: result.ModifiedFiles,
		Data:          result.Data,
	}, nil
}

// IsHealthy always returns true for direct execution
func (b *DirectBridge) IsHealthy() bool {
	return true
}

// GetToolNames returns all registered tool names
func (b *DirectBridge) GetToolNames() []string {
	tools := b.registry.List()
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

// GetRegistry returns the underlying registry
func (b *DirectBridge) GetRegistry() *Registry {
	return b.registry
}

// GetExecutor returns the underlying executor
func (b *DirectBridge) GetExecutor() *ToolExecutor {
	return b.executor
}

// HybridBridge implements ToolBridge with fallback to MCP for unsupported tools
type HybridBridge struct {
	directTools map[string]bool
	registry    *Registry
	mcpFallback func(ctx context.Context, name string, args map[string]interface{}) (*BridgeResult, error)
}

// NewHybridBridge creates a bridge that uses direct execution for some tools
// and falls back to MCP for others
func NewHybridBridge(
	registry *Registry,
	directToolNames []string,
	mcpFallback func(ctx context.Context, name string, args map[string]interface{}) (*BridgeResult, error),
) *HybridBridge {
	directTools := make(map[string]bool)
	for _, name := range directToolNames {
		directTools[name] = true
	}

	return &HybridBridge{
		directTools: directTools,
		registry:    registry,
		mcpFallback: mcpFallback,
	}
}

// CallTool routes tool calls to direct execution or MCP fallback
func (b *HybridBridge) CallTool(ctx context.Context, name string, args map[string]interface{}) (*BridgeResult, error) {
	// Check if this tool should be executed directly
	if b.directTools[name] {
		result, err := b.registry.Execute(ctx, name, args)
		if err != nil {
			return &BridgeResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
		return &BridgeResult{
			Success:       result.Success,
			Output:        result.Output,
			Error:         result.Error,
			ModifiedFiles: result.ModifiedFiles,
			Data:          result.Data,
		}, nil
	}

	// Fall back to MCP
	if b.mcpFallback != nil {
		return b.mcpFallback(ctx, name, args)
	}

	return &BridgeResult{
		Success: false,
		Error:   fmt.Sprintf("tool %q not found and no fallback configured", name),
	}, nil
}

// IsHealthy checks both direct and MCP health
func (b *HybridBridge) IsHealthy() bool {
	// Direct execution is always healthy
	// For full health, could check MCP via fallback call
	return true
}

// GetToolNames returns all available tool names
func (b *HybridBridge) GetToolNames() []string {
	var names []string
	for name := range b.directTools {
		names = append(names, name)
	}
	// Could also query MCP for additional tools
	return names
}

// MCPClientAdapter wraps an MCP client to implement ToolBridge
// This allows gradual migration by using the existing MCP infrastructure
type MCPClientAdapter struct {
	// MCPCaller is the function that calls the MCP client
	// This is a function pointer to avoid circular imports with pkg/mcp
	MCPCaller  func(ctx context.Context, name string, args map[string]interface{}) (string, bool, error)
	MCPHealthy func() bool
}

// CallTool calls the MCP client through the adapter
func (a *MCPClientAdapter) CallTool(ctx context.Context, name string, args map[string]interface{}) (*BridgeResult, error) {
	if a.MCPCaller == nil {
		return &BridgeResult{
			Success: false,
			Error:   "MCP caller not configured",
		}, nil
	}

	output, isError, err := a.MCPCaller(ctx, name, args)
	if err != nil {
		return &BridgeResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &BridgeResult{
		Success: !isError,
		Output:  output,
	}, nil
}

// IsHealthy checks MCP client health
func (a *MCPClientAdapter) IsHealthy() bool {
	if a.MCPHealthy != nil {
		return a.MCPHealthy()
	}
	return false
}

// GetToolNames returns empty list (MCP tools are dynamic)
func (a *MCPClientAdapter) GetToolNames() []string {
	return nil
}

// ExtractJobID extracts a job ID from tool output (common pattern in job-related tools)
func ExtractJobID(output string) string {
	// Look for patterns like "Job job-1234567890" or "job-1234567890"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Job ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
		if strings.HasPrefix(line, "job-") {
			return strings.Fields(line)[0]
		}
	}
	return ""
}

// ExtractJobStatus extracts job status from tool output
func ExtractJobStatus(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	return ""
}
