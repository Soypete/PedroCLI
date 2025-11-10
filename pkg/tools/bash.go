package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

// BashTool executes safe bash commands (no sed/grep/find - use Go instead)
type BashTool struct {
	allowedCommands   map[string]bool
	forbiddenCommands map[string]bool
	workDir           string
}

// NewBashTool creates a new bash tool
func NewBashTool(cfg *config.Config, workDir string) *BashTool {
	allowed := make(map[string]bool)
	for _, cmd := range cfg.Tools.AllowedBashCommands {
		allowed[cmd] = true
	}

	forbidden := make(map[string]bool)
	for _, cmd := range cfg.Tools.ForbiddenCommands {
		forbidden[cmd] = true
	}

	return &BashTool{
		allowedCommands:   allowed,
		forbiddenCommands: forbidden,
		workDir:           workDir,
	}
}

// Name returns the tool name
func (b *BashTool) Name() string {
	return "bash"
}

// Description returns the tool description
func (b *BashTool) Description() string {
	return "Execute safe bash commands (no sed/grep/find - use File tool instead)"
}

// InputSchema returns the JSON Schema for tool arguments
func (b *BashTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Command to execute (must be in allowed list)",
			},
			"args": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Command arguments",
			},
		},
		"required": []string{"command"},
	}
}

// Execute executes the bash tool
func (b *BashTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	command, ok := args["command"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'command' parameter"}, nil
	}

	// Parse command to check first word
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return &Result{Success: false, Error: "empty command"}, nil
	}

	baseCmd := fields[0]

	// Check if forbidden
	if b.forbiddenCommands[baseCmd] {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("command forbidden: %s (use File tool for file operations)", baseCmd),
		}, nil
	}

	// Check if allowed
	if !b.isAllowed(baseCmd) {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("command not allowed: %s", baseCmd),
		}, nil
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = b.workDir

	output, err := cmd.CombinedOutput()

	if err != nil {
		return &Result{
			Success: false,
			Output:  string(output),
			Error:   err.Error(),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// isAllowed checks if a command is allowed
func (b *BashTool) isAllowed(cmd string) bool {
	// If allowedCommands is empty, allow all non-forbidden
	if len(b.allowedCommands) == 0 {
		return !b.forbiddenCommands[cmd]
	}

	return b.allowedCommands[cmd]
}
