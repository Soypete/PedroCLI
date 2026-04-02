package toolformat

import (
	"context"
	"fmt"
	"testing"

	"github.com/soypete/pedro-agentware/middleware"
	"github.com/soypete/pedro-agentware/middleware/types"
)

type mockExecutor struct {
	callToolFunc func(ctx context.Context, name string, args map[string]interface{}) (*middleware.ToolResult, error)
	tools        []types.ToolDefinition
}

func (m *mockExecutor) CallTool(ctx context.Context, name string, args map[string]interface{}) (*middleware.ToolResult, error) {
	if m.callToolFunc != nil {
		return m.callToolFunc(ctx, name, args)
	}
	return &middleware.ToolResult{Content: "ok"}, nil
}

func (m *mockExecutor) ListTools() []types.ToolDefinition {
	return m.tools
}

func TestMiddlewareBridge_CallTool_Success(t *testing.T) {
	exec := &mockExecutor{
		callToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*middleware.ToolResult, error) {
			return &middleware.ToolResult{
				Content: "tool executed successfully",
				Error:   nil,
			}, nil
		},
		tools: []types.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}

	policy := middleware.Policy{
		Rules: []types.Rule{},
	}
	mw := middleware.New(exec, policy)

	bridge := NewMiddlewareBridge(mw)

	result, err := bridge.CallTool(context.Background(), "test_tool", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success = true, got false")
	}

	if result.Output != "tool executed successfully" {
		t.Errorf("Expected output = 'tool executed successfully', got %q", result.Output)
	}
}

func TestMiddlewareBridge_CallTool_Error(t *testing.T) {
	exec := &mockExecutor{
		callToolFunc: func(ctx context.Context, name string, args map[string]interface{}) (*middleware.ToolResult, error) {
			return &middleware.ToolResult{
				Content: nil,
				Error:   fmt.Errorf("tool execution failed"),
			}, nil
		},
		tools: []types.ToolDefinition{
			{Name: "test_tool", Description: "Test tool"},
		},
	}

	policy := middleware.Policy{
		Rules: []types.Rule{},
	}
	mw := middleware.New(exec, policy)

	bridge := NewMiddlewareBridge(mw)

	result, err := bridge.CallTool(context.Background(), "test_tool", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}

	if result.Success {
		t.Errorf("Expected success = false, got true")
	}

	if result.Error != "tool execution failed" {
		t.Errorf("Expected error = 'tool execution failed', got %q", result.Error)
	}
}

func TestMiddlewareBridge_CallTool_NilMiddleware(t *testing.T) {
	bridge := NewMiddlewareBridge(nil)

	result, err := bridge.CallTool(context.Background(), "test_tool", nil)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}

	if result.Success {
		t.Errorf("Expected success = false for nil middleware")
	}

	if result.Error != "middleware not configured" {
		t.Errorf("Expected error = 'middleware not configured', got %q", result.Error)
	}
}

func TestMiddlewareBridge_IsHealthy(t *testing.T) {
	exec := &mockExecutor{}
	policy := middleware.Policy{Rules: []types.Rule{}}
	mw := middleware.New(exec, policy)

	bridge := NewMiddlewareBridge(mw)
	if !bridge.IsHealthy() {
		t.Error("Expected IsHealthy() = true for non-nil middleware")
	}

	nilBridge := NewMiddlewareBridge(nil)
	if nilBridge.IsHealthy() {
		t.Error("Expected IsHealthy() = false for nil middleware")
	}
}

func TestMiddlewareBridge_GetToolNames(t *testing.T) {
	exec := &mockExecutor{
		tools: []types.ToolDefinition{
			{Name: "tool_a", Description: "Tool A"},
			{Name: "tool_b", Description: "Tool B"},
		},
	}

	policy := middleware.Policy{
		Rules: []types.Rule{},
	}
	mw := middleware.New(exec, policy)

	bridge := NewMiddlewareBridge(mw)

	names := bridge.GetToolNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 tool names, got %d", len(names))
	}

	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}

	if !found["tool_a"] || !found["tool_b"] {
		t.Errorf("Expected tool_a and tool_b, got %v", names)
	}
}

func TestMiddlewareBridge_GetMiddleware(t *testing.T) {
	exec := &mockExecutor{}
	policy := middleware.Policy{Rules: []types.Rule{}}
	mw := middleware.New(exec, policy)

	bridge := NewMiddlewareBridge(mw)

	if bridge.GetMiddleware() != mw {
		t.Error("GetMiddleware() should return the original middleware")
	}
}
