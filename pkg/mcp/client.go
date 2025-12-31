package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Client represents an MCP client that communicates with an MCP server via stdio
type Client struct {
	serverPath string
	serverArgs []string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	scanner    *bufio.Scanner
	mu         sync.Mutex
	nextID     int
}

// NewClient creates a new MCP client
func NewClient(serverPath string, args []string) *Client {
	return &Client{
		serverPath: serverPath,
		serverArgs: args,
		nextID:     1,
	}
}

// Start starts the MCP server subprocess
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create command
	c.cmd = exec.CommandContext(ctx, c.serverPath, c.serverArgs...)

	// Set up pipes
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Set up scanner for reading responses
	c.scanner = bufio.NewScanner(c.stdout)

	return nil
}

// Stop stops the MCP server subprocess
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Close stdin to signal shutdown
	if c.stdin != nil {
		c.stdin.Close()
	}

	// Wait for process to exit
	return c.cmd.Wait()
}

// CallTool calls an MCP tool and returns the result
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResponse, error) {
	// Hold lock for entire request-response cycle to prevent race conditions
	// between concurrent goroutines (SSE broadcaster + HTTP handlers)
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	// Build JSON-RPC request
	request := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	// Serialize request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	_, err = c.stdin.Write(append(requestBytes, '\n'))
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response (must be in same lock to ensure we get OUR response)
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("no response from server")
	}
	responseBytes := c.scanner.Bytes()

	// Parse response
	var response JSONRPCResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for error
	if response.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", response.Error.Message)
	}

	// Parse tool response
	var toolResponse ToolResponse
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := json.Unmarshal(resultBytes, &toolResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool response: %w", err)
	}

	return &toolResponse, nil
}

// ListTools lists available MCP tools
func (c *Client) ListTools(ctx context.Context) ([]ToolInfo, error) {
	// Hold lock for entire request-response cycle
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	// Build JSON-RPC request
	request := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	// Serialize request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	_, err = c.stdin.Write(append(requestBytes, '\n'))
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("no response from server")
	}
	responseBytes := c.scanner.Bytes()

	// Parse response
	var response JSONRPCResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for error
	if response.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", response.Error.Message)
	}

	// Parse tools list
	var result struct {
		Tools []ToolInfo `json:"tools"`
	}
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools list: %w", err)
	}

	return result.Tools, nil
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolResponse represents the response from calling an MCP tool
type ToolResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in the tool response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ToolInfo represents information about an MCP tool
type ToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}
