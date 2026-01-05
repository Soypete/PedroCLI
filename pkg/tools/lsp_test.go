package tools

import (
	"context"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestNewLSPTool(t *testing.T) {
	cfg := &config.Config{
		LSP: config.LSPConfig{
			Enabled: true,
			Timeout: 30,
		},
	}

	tool := NewLSPTool(cfg, "/tmp/workspace")

	if tool == nil {
		t.Fatal("NewLSPTool returned nil")
	}

	if tool.Name() != "lsp" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "lsp")
	}

	if tool.Description() == "" {
		t.Error("Description() returned empty string")
	}
}

func TestLSPToolMissingOperation(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"file": "main.go",
	})

	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Execute should have failed without operation")
	}

	if result.Error == "" {
		t.Error("Error should contain message about missing operation")
	}
}

func TestLSPToolMissingFile(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"operation": "definition",
	})

	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Execute should have failed without file")
	}

	if result.Error == "" {
		t.Error("Error should contain message about missing file")
	}
}

func TestLSPToolUnknownOperation(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"operation": "unknown_operation",
		"file":      "main.go",
	})

	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Execute should have failed with unknown operation")
	}

	if result.Error == "" {
		t.Error("Error should contain message about unknown operation")
	}
}

func TestLSPToolDefinitionMissingPosition(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	// Missing line and column
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"operation": "definition",
		"file":      "main.go",
	})

	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Execute should have failed without line/column")
	}
}

func TestLSPToolExtractPosition(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	tests := []struct {
		name     string
		args     map[string]interface{}
		wantLine int
		wantCol  int
		wantErr  bool
	}{
		{
			name:     "valid float64",
			args:     map[string]interface{}{"line": float64(10), "column": float64(5)},
			wantLine: 10,
			wantCol:  5,
			wantErr:  false,
		},
		{
			name:     "valid int",
			args:     map[string]interface{}{"line": 10, "column": 5},
			wantLine: 10,
			wantCol:  5,
			wantErr:  false,
		},
		{
			name:    "missing line",
			args:    map[string]interface{}{"column": float64(5)},
			wantErr: true,
		},
		{
			name:    "missing column",
			args:    map[string]interface{}{"line": float64(10)},
			wantErr: true,
		},
		{
			name:    "invalid line type",
			args:    map[string]interface{}{"line": "10", "column": float64(5)},
			wantErr: true,
		},
		{
			name:    "line less than 1",
			args:    map[string]interface{}{"line": float64(0), "column": float64(5)},
			wantErr: true,
		},
		{
			name:    "column less than 1",
			args:    map[string]interface{}{"line": float64(10), "column": float64(0)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col, err := tool.extractPosition(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if line != tt.wantLine {
				t.Errorf("line = %d, want %d", line, tt.wantLine)
			}

			if col != tt.wantCol {
				t.Errorf("col = %d, want %d", col, tt.wantCol)
			}
		})
	}
}

func TestLSPToolRelPath(t *testing.T) {
	tool := NewLSPTool(nil, "/home/user/project")

	tests := []struct {
		absPath  string
		expected string
	}{
		{"/home/user/project/main.go", "main.go"},
		{"/home/user/project/pkg/server/handler.go", "pkg/server/handler.go"},
		{"/other/path/file.go", "/other/path/file.go"}, // Outside workspace
	}

	for _, tt := range tests {
		t.Run(tt.absPath, func(t *testing.T) {
			result := tool.relPath(tt.absPath)
			if result != tt.expected {
				t.Errorf("relPath(%q) = %q, want %q", tt.absPath, result, tt.expected)
			}
		})
	}
}

func TestLSPToolMetadata(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	meta := tool.Metadata()

	if meta == nil {
		t.Fatal("Metadata returned nil")
	}

	if meta.Schema == nil {
		t.Error("Schema is nil")
	}

	if meta.Category != CategoryCode {
		t.Errorf("Category = %v, want %v", meta.Category, CategoryCode)
	}

	if meta.Optionality != ToolOptional {
		t.Errorf("Optionality = %v, want %v", meta.Optionality, ToolOptional)
	}

	if len(meta.Examples) == 0 {
		t.Error("Examples should not be empty")
	}

	// Check schema properties
	props := meta.Schema.Properties
	if props == nil {
		t.Fatal("Schema properties are nil")
	}

	requiredProps := []string{"operation", "file"}
	for _, prop := range requiredProps {
		if _, ok := props[prop]; !ok {
			t.Errorf("Schema missing property: %s", prop)
		}
	}
}

func TestLSPToolShutdown(t *testing.T) {
	tool := NewLSPTool(nil, "/tmp/workspace")

	// Shutdown should not error even without any active clients
	err := tool.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}
