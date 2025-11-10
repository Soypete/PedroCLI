package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestBashToolName(t *testing.T) {
	cfg := getTestConfig()
	tool := NewBashTool(cfg, "/tmp")

	if tool.Name() != "bash" {
		t.Errorf("Name() = %v, want bash", tool.Name())
	}
}

func TestBashToolDescription(t *testing.T) {
	cfg := getTestConfig()
	tool := NewBashTool(cfg, "/tmp")

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestBashToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := getTestConfig()
	tool := NewBashTool(cfg, tmpDir)
	ctx := context.Background()

	tests := []struct {
		name     string
		args     map[string]interface{}
		validate func(*testing.T, *Result)
	}{
		{
			name: "execute allowed command",
			args: map[string]interface{}{
				"command": "echo hello",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
				if !strings.Contains(r.Output, "hello") {
					t.Errorf("Output = %q, want output containing 'hello'", r.Output)
				}
			},
		},
		{
			name: "execute git command",
			args: map[string]interface{}{
				"command": "git --version",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
			},
		},
		{
			name: "forbidden command - sed",
			args: map[string]interface{}{
				"command": "sed 's/old/new/' file.txt",
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for forbidden command")
				}
				if !strings.Contains(r.Error, "command forbidden") {
					t.Errorf("Error = %q, want error containing 'command forbidden'", r.Error)
				}
			},
		},
		{
			name: "forbidden command - grep",
			args: map[string]interface{}{
				"command": "grep pattern file.txt",
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for forbidden command")
				}
				if !strings.Contains(r.Error, "command forbidden") {
					t.Errorf("Error = %q, want error containing 'command forbidden'", r.Error)
				}
			},
		},
		{
			name: "forbidden command - rm",
			args: map[string]interface{}{
				"command": "rm -rf /",
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for forbidden command")
				}
				if !strings.Contains(r.Error, "command forbidden") {
					t.Errorf("Error = %q, want error containing 'command forbidden'", r.Error)
				}
			},
		},
		{
			name: "forbidden command - sudo",
			args: map[string]interface{}{
				"command": "sudo apt-get install foo",
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for forbidden command")
				}
			},
		},
		{
			name: "empty command",
			args: map[string]interface{}{
				"command": "",
			},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for empty command")
				}
				if !strings.Contains(r.Error, "empty command") {
					t.Errorf("Error = %q, want error containing 'empty command'", r.Error)
				}
			},
		},
		{
			name: "missing command parameter",
			args: map[string]interface{}{},
			validate: func(t *testing.T, r *Result) {
				if r.Success {
					t.Error("Success = true, want false for missing command")
				}
				if !strings.Contains(r.Error, "missing 'command' parameter") {
					t.Errorf("Error = %q, want error containing 'missing 'command' parameter'", r.Error)
				}
			},
		},
		{
			name: "command with arguments",
			args: map[string]interface{}{
				"command": "ls -la",
			},
			validate: func(t *testing.T, r *Result) {
				if !r.Success {
					t.Errorf("Success = false, want true. Error: %s", r.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
			}

			tt.validate(t, result)
		})
	}
}

func TestBashToolNotAllowedCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with restricted allowed commands
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"git", "go"},
			ForbiddenCommands:   []string{"sed", "grep", "rm", "sudo"},
		},
	}

	tool := NewBashTool(cfg, tmpDir)
	ctx := context.Background()

	// Try to execute a command that's not in the allowed list
	args := map[string]interface{}{
		"command": "ls -la",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result.Success {
		t.Error("Success = true, want false for not allowed command")
	}

	if !strings.Contains(result.Error, "command not allowed") {
		t.Errorf("Error = %q, want error containing 'command not allowed'", result.Error)
	}
}

func TestBashToolEmptyAllowedList(t *testing.T) {
	tmpDir := t.TempDir()

	// Config with empty allowed list (allows all non-forbidden)
	cfg := &config.Config{
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{},
			ForbiddenCommands:   []string{"sed", "grep"},
		},
	}

	tool := NewBashTool(cfg, tmpDir)
	ctx := context.Background()

	// Should allow ls since allowed list is empty and ls is not forbidden
	args := map[string]interface{}{
		"command": "ls",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true for command with empty allowed list. Error: %s", result.Error)
	}

	// Should still block forbidden commands
	args2 := map[string]interface{}{
		"command": "sed 's/a/b/' file.txt",
	}

	result2, err := tool.Execute(ctx, args2)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if result2.Success {
		t.Error("Success = true, want false for forbidden command even with empty allowed list")
	}
}

func TestBashToolWorkDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := getTestConfig()
	tool := NewBashTool(cfg, tmpDir)
	ctx := context.Background()

	// Execute pwd to verify working directory
	args := map[string]interface{}{
		"command": "pwd",
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Errorf("Execute() unexpected error = %v", err)
	}

	if !result.Success {
		t.Errorf("Success = false, want true. Error: %s", result.Error)
	}

	if !strings.Contains(result.Output, tmpDir) {
		t.Errorf("Output = %q, want output containing %q", result.Output, tmpDir)
	}
}

// Helper function to get test config
func getTestConfig() *config.Config {
	return &config.Config{
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{
				"git", "gh", "go", "cat", "ls", "head", "tail",
				"wc", "sort", "uniq", "echo", "pwd",
			},
			ForbiddenCommands: []string{
				"sed", "grep", "find", "xargs", "rm", "mv", "dd", "sudo",
			},
		},
	}
}
