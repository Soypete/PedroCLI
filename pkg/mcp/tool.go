package mcp

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/tools"
)

// MCPToolWrapper wraps an MCP tool to implement the tools.Tool interface
type MCPToolWrapper struct {
	client     *Client
	serverName string
	tool       Tool
}

// NewMCPToolWrapper creates a new MCP tool wrapper
func NewMCPToolWrapper(client *Client, serverName string, tool Tool) *MCPToolWrapper {
	return &MCPToolWrapper{
		client:     client,
		serverName: serverName,
		tool:       tool,
	}
}

// Name returns the tool name (prefixed with server name)
func (w *MCPToolWrapper) Name() string {
	return fmt.Sprintf("mcp_%s_%s", w.serverName, w.tool.Name)
}

// Description returns the tool description
func (w *MCPToolWrapper) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", w.serverName, w.tool.Description)
}

// Execute calls the MCP tool
func (w *MCPToolWrapper) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	result, err := w.client.CallTool(ctx, w.serverName, w.tool.Name, args)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("MCP tool call failed: %v", err),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  result,
		Data: map[string]interface{}{
			"mcp_server": w.serverName,
			"mcp_tool":   w.tool.Name,
		},
	}, nil
}

// Metadata returns the tool metadata
func (w *MCPToolWrapper) Metadata() *tools.ToolMetadata {
	return &tools.ToolMetadata{
		Category:  tools.CategoryUtility,
		UsageHint: fmt.Sprintf("This tool is provided by MCP server: %s", w.serverName),
	}
}

// RegisterMCPTools registers all MCP tools with a tool registry
func RegisterMCPTools(client *Client, registry *tools.ToolRegistry) error {
	allTools := client.ListAllTools()

	for serverName, serverTools := range allTools {
		for _, tool := range serverTools {
			wrapper := NewMCPToolWrapper(client, serverName, tool)
			if err := registry.Register(wrapper); err != nil {
				return fmt.Errorf("failed to register MCP tool %s: %w", wrapper.Name(), err)
			}
		}
	}

	return nil
}

// MCPMetaTool provides meta operations for MCP servers
type MCPMetaTool struct {
	client *Client
}

// NewMCPMetaTool creates a new MCP meta tool
func NewMCPMetaTool(client *Client) *MCPMetaTool {
	return &MCPMetaTool{
		client: client,
	}
}

// Name returns the tool name
func (t *MCPMetaTool) Name() string {
	return "mcp"
}

// Description returns the tool description
func (t *MCPMetaTool) Description() string {
	return `Manage MCP (Model Context Protocol) servers and their tools.
Actions:
- list_servers: List all connected MCP servers
- list_tools: List all available tools from MCP servers
- call: Call a specific MCP tool (requires server, tool, and args)`
}

// Execute performs MCP meta operations
func (t *MCPMetaTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		return &tools.Result{
			Success: false,
			Error:   "action is required (list_servers, list_tools, call)",
		}, nil
	}

	switch action {
	case "list_servers":
		return t.listServers()
	case "list_tools":
		return t.listTools()
	case "call":
		return t.callTool(ctx, args)
	default:
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

func (t *MCPMetaTool) listServers() (*tools.Result, error) {
	servers := t.client.ListServers()

	var output string
	output = fmt.Sprintf("Found %d MCP server(s):\n\n", len(servers))
	for _, server := range servers {
		status := "stopped"
		if server.IsRunning() {
			status = "running"
		}
		output += fmt.Sprintf("- **%s** [%s]: %s\n", server.Name, server.Type, status)
		if len(server.tools) > 0 {
			output += fmt.Sprintf("  Tools: %d available\n", len(server.tools))
		}
	}

	return &tools.Result{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"count": len(servers),
		},
	}, nil
}

func (t *MCPMetaTool) listTools() (*tools.Result, error) {
	allTools := t.client.ListAllTools()

	var output string
	totalTools := 0
	for serverName, serverTools := range allTools {
		output += fmt.Sprintf("## %s\n", serverName)
		for _, tool := range serverTools {
			output += fmt.Sprintf("- **%s**: %s\n", tool.Name, tool.Description)
			totalTools++
		}
		output += "\n"
	}

	if totalTools == 0 {
		output = "No MCP tools available. Ensure MCP servers are running."
	}

	return &tools.Result{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"total_tools": totalTools,
		},
	}, nil
}

func (t *MCPMetaTool) callTool(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	serverName, ok := args["server"].(string)
	if !ok || serverName == "" {
		return &tools.Result{
			Success: false,
			Error:   "server name is required",
		}, nil
	}

	toolName, ok := args["tool"].(string)
	if !ok || toolName == "" {
		return &tools.Result{
			Success: false,
			Error:   "tool name is required",
		}, nil
	}

	toolArgs, _ := args["args"].(map[string]interface{})
	if toolArgs == nil {
		toolArgs = make(map[string]interface{})
	}

	result, err := t.client.CallTool(ctx, serverName, toolName, toolArgs)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("MCP tool call failed: %v", err),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  result,
		Data: map[string]interface{}{
			"server": serverName,
			"tool":   toolName,
		},
	}, nil
}

// Metadata returns the tool metadata
func (t *MCPMetaTool) Metadata() *tools.ToolMetadata {
	return &tools.ToolMetadata{
		Category:  tools.CategoryUtility,
		UsageHint: "Manage MCP (Model Context Protocol) servers and their tools.",
		Examples: []tools.ToolExample{
			{
				Description: "List all MCP servers",
				Input: map[string]interface{}{
					"action": "list_servers",
				},
			},
			{
				Description: "List all available MCP tools",
				Input: map[string]interface{}{
					"action": "list_tools",
				},
			},
			{
				Description: "Call a GitHub MCP tool",
				Input: map[string]interface{}{
					"action": "call",
					"server": "github",
					"tool":   "get_issue",
					"args": map[string]interface{}{
						"repo":  "owner/repo",
						"issue": 123,
					},
				},
			},
		},
	}
}
