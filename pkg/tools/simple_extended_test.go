package tools

import (
	"context"
	"testing"
)

func TestSimpleExtendedTool_Name(t *testing.T) {
	tool := &mockTool{name: "test_tool", description: "Test description"}
	wrapped := NewSimpleExtendedTool(tool)

	if wrapped.Name() != "test_tool" {
		t.Errorf("expected name 'test_tool', got %q", wrapped.Name())
	}
}

func TestSimpleExtendedTool_Description(t *testing.T) {
	tool := &mockTool{name: "test_tool", description: "Test description"}
	wrapped := NewSimpleExtendedTool(tool)

	if wrapped.Description() != "Test description" {
		t.Errorf("expected description 'Test description', got %q", wrapped.Description())
	}
}

func TestSimpleExtendedTool_Execute(t *testing.T) {
	tool := &mockTool{name: "test_tool", description: "Test"}
	wrapped := NewSimpleExtendedTool(tool)

	result, err := wrapped.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Output != "mock output" {
		t.Errorf("expected 'mock output', got %q", result.Output)
	}
}

func TestSimpleExtendedTool_Metadata(t *testing.T) {
	tool := &mockTool{name: "test_tool", description: "Test"}
	wrapped := NewSimpleExtendedTool(tool)

	if wrapped.Metadata() != nil {
		t.Error("expected nil metadata for wrapped legacy tool")
	}
}

func TestSimpleExtendedTool_Unwrap(t *testing.T) {
	tool := &mockTool{name: "test_tool", description: "Test"}
	wrapped := NewSimpleExtendedTool(tool)

	unwrapped := wrapped.Unwrap()
	if unwrapped != tool {
		t.Error("unwrap should return the original tool")
	}
}

func TestSimpleExtendedTool_ImplementsExtendedTool(t *testing.T) {
	tool := &mockTool{name: "test", description: "Test"}
	wrapped := NewSimpleExtendedTool(tool)

	// Verify it implements ExtendedTool interface
	var _ ExtendedTool = wrapped
}
