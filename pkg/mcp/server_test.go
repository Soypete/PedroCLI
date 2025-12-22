package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/tools"
)

// mockTool is a simple mock tool for testing
type mockTool struct {
	name        string
	description string
	result      *tools.Result
	executeErr  error
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.result, nil
}

func TestNewServer(t *testing.T) {
	server := NewServer()
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
	if server.tools == nil {
		t.Error("Server tools map is nil")
	}
}

func TestRegisterTool(t *testing.T) {
	server := NewServer()
	tool := &mockTool{name: "test-tool", description: "A test tool"}

	server.RegisterTool(tool)

	if len(server.tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(server.tools))
	}

	registeredTool, ok := server.tools["test-tool"]
	if !ok {
		t.Error("Tool not registered with correct name")
	}
	if registeredTool.Name() != "test-tool" {
		t.Errorf("Expected tool name 'test-tool', got '%s'", registeredTool.Name())
	}
}

func TestHandleInitialize(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}
	// ID comes back as float64 from JSON unmarshaling
	if id, ok := resp.ID.(float64); !ok || id != 1.0 {
		t.Errorf("Expected ID 1, got %v (type %T)", resp.ID, resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	// Check result structure
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	if result["protocolVersion"] != "1.0" {
		t.Errorf("Expected protocolVersion '1.0', got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("serverInfo is not a map")
	}
	if serverInfo["name"] != "pedroceli" {
		t.Errorf("Expected server name 'pedroceli', got %v", serverInfo["name"])
	}
}

func TestHandleToolsList(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	// Register test tools
	server.RegisterTool(&mockTool{name: "tool1", description: "First tool"})
	server.RegisterTool(&mockTool{name: "tool2", description: "Second tool"})

	req := &Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	toolsList, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("tools is not an array")
	}

	if len(toolsList) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(toolsList))
	}
}

func TestHandleToolCallSuccess(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	// Register mock tool that returns success
	server.RegisterTool(&mockTool{
		name:        "echo",
		description: "Echo tool",
		result: &tools.Result{
			Success: true,
			Output:  "Hello World",
		},
	})

	req := &Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "Hello",
			},
		},
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	if result["isError"] != false {
		t.Error("Expected isError to be false")
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("content is not an array or is empty")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("content[0] is not a map")
	}

	if firstContent["text"] != "Hello World" {
		t.Errorf("Expected text 'Hello World', got %v", firstContent["text"])
	}
}

func TestHandleToolCallToolNotFound(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	req := &Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "nonexistent",
			"arguments": map[string]interface{}{},
		},
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error, got none")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Tool not found") {
		t.Errorf("Expected 'Tool not found' error, got: %s", resp.Error.Message)
	}
}

func TestHandleToolCallMissingName(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	req := &Request{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"arguments": map[string]interface{}{},
		},
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error, got none")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestHandleToolCallMissingArguments(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	server.RegisterTool(&mockTool{name: "test", description: "Test"})

	req := &Request{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test",
		},
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error, got none")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	req := &Request{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "unknown/method",
	}

	server.handleRequest(context.Background(), req)

	// Parse response
	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error, got none")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Method not found") {
		t.Errorf("Expected 'Method not found' error, got: %s", resp.Error.Message)
	}
}

func TestSendResponse(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	server.sendResponse(123, map[string]string{"result": "success"})

	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}
	// ID comes back as float64 from JSON unmarshaling
	if id, ok := resp.ID.(float64); !ok || id != 123.0 {
		t.Errorf("Expected ID 123, got %v (type %T)", resp.ID, resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
}

func TestSendError(t *testing.T) {
	var stdout bytes.Buffer
	server := NewServer()
	server.stdout = &stdout

	server.sendError(456, -32600, "Invalid Request")

	var resp Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}
	// ID comes back as float64 from JSON unmarshaling
	if id, ok := resp.ID.(float64); !ok || id != 456.0 {
		t.Errorf("Expected ID 456, got %v (type %T)", resp.ID, resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("Expected error, got none")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("Expected error code -32600, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Invalid Request" {
		t.Errorf("Expected message 'Invalid Request', got '%s'", resp.Error.Message)
	}
}

func TestRunWithInvalidJSON(t *testing.T) {
	stdin := strings.NewReader("invalid json\n")
	var stdout bytes.Buffer

	server := NewServer()
	server.stdin = stdin
	server.stdout = &stdout

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run in goroutine since it will block on scanner
	done := make(chan error)
	go func() {
		done <- server.Run(ctx)
	}()

	// Cancel to stop the server
	cancel()

	// Wait for completion
	<-done

	// Should have sent a parse error
	output := stdout.String()
	if !strings.Contains(output, "Parse error") {
		t.Errorf("Expected 'Parse error' in output, got: %s", output)
	}
}
