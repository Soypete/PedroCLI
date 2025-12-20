package mcp

import (
	"encoding/json"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("/path/to/server", []string{"arg1", "arg2"})
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.serverPath != "/path/to/server" {
		t.Errorf("Expected serverPath '/path/to/server', got '%s'", client.serverPath)
	}
	if len(client.serverArgs) != 2 {
		t.Errorf("Expected 2 args, got %d", len(client.serverArgs))
	}
	if client.nextID != 1 {
		t.Errorf("Expected nextID 1, got %d", client.nextID)
	}
}

func TestJSONRPCRequestSerialization(t *testing.T) {
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "test-tool",
			"arguments": map[string]interface{}{
				"arg1": "value1",
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var parsed JSONRPCRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if parsed.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", parsed.JSONRPC)
	}
	if parsed.ID != 1 {
		t.Errorf("Expected ID 1, got %d", parsed.ID)
	}
	if parsed.Method != "tools/call" {
		t.Errorf("Expected method 'tools/call', got '%s'", parsed.Method)
	}
}

func TestJSONRPCResponseDeserialization(t *testing.T) {
	responseJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"content": [
				{
					"type": "text",
					"text": "Hello World"
				}
			],
			"isError": false
		}
	}`

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("Expected result, got nil")
	}
}

func TestJSONRPCErrorResponseDeserialization(t *testing.T) {
	responseJSON := `{
		"jsonrpc": "2.0",
		"id": 1,
		"error": {
			"code": -32602,
			"message": "Invalid params"
		}
	}`

	var resp JSONRPCResponse
	if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error, got nil")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("Expected error code -32602, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Invalid params" {
		t.Errorf("Expected message 'Invalid params', got '%s'", resp.Error.Message)
	}
}

func TestToolResponseDeserialization(t *testing.T) {
	responseJSON := `{
		"content": [
			{
				"type": "text",
				"text": "Test output"
			}
		],
		"isError": false
	}`

	var toolResp ToolResponse
	if err := json.Unmarshal([]byte(responseJSON), &toolResp); err != nil {
		t.Fatalf("Failed to unmarshal tool response: %v", err)
	}

	if len(toolResp.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(toolResp.Content))
	}
	if toolResp.Content[0].Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", toolResp.Content[0].Type)
	}
	if toolResp.Content[0].Text != "Test output" {
		t.Errorf("Expected text 'Test output', got '%s'", toolResp.Content[0].Text)
	}
	if toolResp.IsError {
		t.Error("Expected isError to be false")
	}
}

func TestToolInfoDeserialization(t *testing.T) {
	toolsJSON := `{
		"tools": [
			{
				"name": "file",
				"description": "File operations tool"
			},
			{
				"name": "bash",
				"description": "Execute bash commands"
			}
		]
	}`

	var result struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal([]byte(toolsJSON), &result); err != nil {
		t.Fatalf("Failed to unmarshal tools list: %v", err)
	}

	if len(result.Tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "file" {
		t.Errorf("Expected tool name 'file', got '%s'", result.Tools[0].Name)
	}
	if result.Tools[0].Description != "File operations tool" {
		t.Errorf("Expected description 'File operations tool', got '%s'", result.Tools[0].Description)
	}
	if result.Tools[1].Name != "bash" {
		t.Errorf("Expected tool name 'bash', got '%s'", result.Tools[1].Name)
	}
}

func TestContentBlockSerialization(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Text: "Sample text",
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Failed to marshal content block: %v", err)
	}

	var parsed ContentBlock
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal content block: %v", err)
	}

	if parsed.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", parsed.Type)
	}
	if parsed.Text != "Sample text" {
		t.Errorf("Expected text 'Sample text', got '%s'", parsed.Text)
	}
}

func TestRPCErrorSerialization(t *testing.T) {
	rpcErr := &RPCError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]string{"detail": "Missing method"},
	}

	data, err := json.Marshal(rpcErr)
	if err != nil {
		t.Fatalf("Failed to marshal RPC error: %v", err)
	}

	var parsed RPCError
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal RPC error: %v", err)
	}

	if parsed.Code != -32600 {
		t.Errorf("Expected code -32600, got %d", parsed.Code)
	}
	if parsed.Message != "Invalid Request" {
		t.Errorf("Expected message 'Invalid Request', got '%s'", parsed.Message)
	}
	if parsed.Data == nil {
		t.Error("Expected data to be present")
	}
}
