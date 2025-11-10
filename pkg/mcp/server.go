package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/soypete/pedrocli/pkg/tools"
)

// Server implements the MCP server protocol
type Server struct {
	tools  map[string]tools.Tool
	stdin  io.Reader
	stdout io.Writer
}

// Request represents an MCP request
type Request struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// Response represents an MCP response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents an MCP error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer creates a new MCP server
func NewServer() *Server {
	return &Server{
		tools:  make(map[string]tools.Tool),
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}
}

// RegisterTool registers a tool with the server
func (s *Server) RegisterTool(tool tools.Tool) {
	s.tools[tool.Name()] = tool
}

// Run starts the MCP server loop (stdio transport)
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.stdin)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		// Handle request
		s.handleRequest(ctx, &req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleRequest handles an MCP request
func (s *Server) handleRequest(ctx context.Context, req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolCall(ctx, req)
	default:
		s.sendError(req.ID, -32601, "Method not found")
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req *Request) {
	result := map[string]interface{}{
		"protocolVersion": "1.0",
		"serverInfo": map[string]interface{}{
			"name":    "pedroceli",
			"version": "0.1.0",
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}

	s.sendResponse(req.ID, result)
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(req *Request) {
	toolsList := make([]map[string]interface{}, 0, len(s.tools))

	for _, tool := range s.tools {
		toolsList = append(toolsList, map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
		})
	}

	s.sendResponse(req.ID, map[string]interface{}{
		"tools": toolsList,
	})
}

// handleToolCall handles the tools/call request
func (s *Server) handleToolCall(ctx context.Context, req *Request) {
	params := req.Params

	name, ok := params["name"].(string)
	if !ok {
		s.sendError(req.ID, -32602, "Invalid params: missing 'name'")
		return
	}

	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		s.sendError(req.ID, -32602, "Invalid params: missing 'arguments'")
		return
	}

	// Find tool
	tool, ok := s.tools[name]
	if !ok {
		s.sendError(req.ID, -32602, fmt.Sprintf("Tool not found: %s", name))
		return
	}

	// Execute tool
	result, err := tool.Execute(ctx, args)
	if err != nil {
		s.sendError(req.ID, -32603, fmt.Sprintf("Tool execution error: %s", err))
		return
	}

	// Send result
	s.sendResponse(req.ID, map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": result.Output,
			},
		},
		"isError": !result.Success,
	})
}

// sendResponse sends a success response
func (s *Server) sendResponse(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, _ := json.Marshal(resp)
	fmt.Fprintln(s.stdout, string(data))
}

// sendError sends an error response
func (s *Server) sendError(id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}

	data, _ := json.Marshal(resp)
	fmt.Fprintln(s.stdout, string(data))
}
