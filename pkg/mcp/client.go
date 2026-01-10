// Package mcp provides Model Context Protocol client for external tool integration.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ServerType represents the type of MCP server connection
type ServerType string

const (
	ServerTypeLocal  ServerType = "local"  // stdio-based
	ServerTypeRemote ServerType = "remote" // HTTP-based
)

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Server represents an MCP server connection
type Server struct {
	Name        string
	Type        ServerType
	Command     []string
	URL         string
	Headers     map[string]string
	Environment map[string]string
	Enabled     bool

	// Runtime state
	process   *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	tools     []Tool
	running   bool
	requestID int64
	mu        sync.Mutex
	responses map[int64]chan *JSONRPCResponse
}

// Client manages multiple MCP server connections
type Client struct {
	servers map[string]*Server
	mu      sync.RWMutex
}

// NewClient creates a new MCP client
func NewClient() *Client {
	return &Client{
		servers: make(map[string]*Server),
	}
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Type        ServerType
	Command     []string
	URL         string
	Headers     map[string]string
	Environment map[string]string
	Enabled     bool
}

// AddServer adds an MCP server to the client
func (c *Client) AddServer(name string, cfg ServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	server := &Server{
		Name:        name,
		Type:        cfg.Type,
		Command:     cfg.Command,
		URL:         cfg.URL,
		Headers:     cfg.Headers,
		Environment: cfg.Environment,
		Enabled:     cfg.Enabled,
		responses:   make(map[int64]chan *JSONRPCResponse),
	}

	c.servers[name] = server
	return nil
}

// Initialize starts all enabled servers
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, server := range c.servers {
		if !server.Enabled {
			continue
		}

		if err := server.Start(ctx); err != nil {
			// Log error but continue with other servers
			fmt.Printf("Warning: failed to start MCP server %s: %v\n", name, err)
			continue
		}
	}

	return nil
}

// Start starts an MCP server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	switch s.Type {
	case ServerTypeLocal:
		return s.startLocal(ctx)
	case ServerTypeRemote:
		return s.connectRemote(ctx)
	default:
		return fmt.Errorf("unknown server type: %s", s.Type)
	}
}

// startLocal starts a local stdio-based MCP server
func (s *Server) startLocal(ctx context.Context) error {
	if len(s.Command) == 0 {
		return fmt.Errorf("no command specified for local server")
	}

	// Expand environment variables in command
	expandedCmd := make([]string, len(s.Command))
	for i, arg := range s.Command {
		expandedCmd[i] = os.ExpandEnv(arg)
	}

	cmd := exec.CommandContext(ctx, expandedCmd[0], expandedCmd[1:]...)

	// Set up environment
	cmd.Env = os.Environ()
	for k, v := range s.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, os.ExpandEnv(v)))
	}

	// Get stdin and stdout pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	s.process = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.running = true

	// Start reading responses
	go s.readResponses()

	// Initialize the server
	if err := s.initialize(ctx); err != nil {
		s.Stop()
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Discover tools
	if err := s.discoverTools(ctx); err != nil {
		s.Stop()
		return fmt.Errorf("failed to discover tools: %w", err)
	}

	return nil
}

// connectRemote establishes connection to a remote HTTP MCP server
func (s *Server) connectRemote(ctx context.Context) error {
	if s.URL == "" {
		return fmt.Errorf("no URL specified for remote server")
	}

	s.running = true

	// Initialize the server
	if err := s.initialize(ctx); err != nil {
		s.running = false
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Discover tools
	if err := s.discoverTools(ctx); err != nil {
		s.running = false
		return fmt.Errorf("failed to discover tools: %w", err)
	}

	return nil
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// readResponses reads and dispatches responses from the server
func (s *Server) readResponses() {
	scanner := bufio.NewScanner(s.stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var response JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue
		}

		s.mu.Lock()
		if ch, ok := s.responses[response.ID]; ok {
			ch <- &response
			delete(s.responses, response.ID)
		}
		s.mu.Unlock()
	}
}

// sendRequest sends a JSON-RPC request and waits for response
func (s *Server) sendRequest(ctx context.Context, method string, params interface{}) (*JSONRPCResponse, error) {
	id := atomic.AddInt64(&s.requestID, 1)

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	responseCh := make(chan *JSONRPCResponse, 1)
	s.mu.Lock()
	s.responses[id] = responseCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.responses, id)
		s.mu.Unlock()
	}()

	switch s.Type {
	case ServerTypeLocal:
		return s.sendLocalRequest(ctx, request, responseCh)
	case ServerTypeRemote:
		return s.sendRemoteRequest(ctx, request)
	default:
		return nil, fmt.Errorf("unknown server type: %s", s.Type)
	}
}

// sendLocalRequest sends a request via stdio
func (s *Server) sendLocalRequest(ctx context.Context, request JSONRPCRequest, responseCh chan *JSONRPCResponse) (*JSONRPCResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	s.mu.Lock()
	_, err = s.stdin.Write(append(data, '\n'))
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case response := <-responseCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

// sendRemoteRequest sends a request via HTTP
func (s *Server) sendRemoteRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.URL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.Headers {
		req.Header.Set(k, os.ExpandEnv(v))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// initialize sends the initialize request to the server
func (s *Server) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"clientInfo": map[string]interface{}{
			"name":    "pedrocli",
			"version": "1.0.0",
		},
	}

	response, err := s.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("initialize error: %s", response.Error.Message)
	}

	return nil
}

// discoverTools fetches available tools from the server
func (s *Server) discoverTools(ctx context.Context) error {
	response, err := s.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("tools/list error: %s", response.Error.Message)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return fmt.Errorf("failed to parse tools: %w", err)
	}

	s.tools = result.Tools
	return nil
}

// ListTools returns the available tools from the server
func (s *Server) ListTools() []Tool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tools
}

// CallTool calls a tool on the server
func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	response, err := s.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}

	if response.Error != nil {
		return "", fmt.Errorf("tool call error: %s", response.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse result: %w", err)
	}

	if result.IsError {
		var texts []string
		for _, c := range result.Content {
			if c.Type == "text" {
				texts = append(texts, c.Text)
			}
		}
		return "", fmt.Errorf("tool error: %s", strings.Join(texts, "\n"))
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	if s.Type == ServerTypeLocal && s.process != nil {
		if s.stdin != nil {
			s.stdin.Close()
		}
		if s.stdout != nil {
			s.stdout.Close()
		}
		return s.process.Process.Kill()
	}

	return nil
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetServer returns a server by name
func (c *Client) GetServer(name string) (*Server, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	server, ok := c.servers[name]
	return server, ok
}

// ListServers returns all servers
func (c *Client) ListServers() []*Server {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Server, 0, len(c.servers))
	for _, server := range c.servers {
		result = append(result, server)
	}
	return result
}

// ListAllTools returns all tools from all servers
func (c *Client) ListAllTools() map[string][]Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]Tool)
	for name, server := range c.servers {
		if server.running {
			result[name] = server.ListTools()
		}
	}
	return result
}

// CallTool calls a tool on a specific server
func (c *Client) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	c.mu.RLock()
	server, ok := c.servers[serverName]
	c.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("server not found: %s", serverName)
	}

	if !server.IsRunning() {
		return "", fmt.Errorf("server not running: %s", serverName)
	}

	return server.CallTool(ctx, toolName, args)
}

// Close stops all servers and cleans up
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, server := range c.servers {
		server.Stop()
	}

	return nil
}

// LoadFromConfig loads MCP servers from configuration
func (c *Client) LoadFromConfig(configs map[string]ServerConfig) error {
	for name, cfg := range configs {
		if err := c.AddServer(name, cfg); err != nil {
			return fmt.Errorf("failed to add server %s: %w", name, err)
		}
	}
	return nil
}
