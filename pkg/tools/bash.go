package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/logits"
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
	return `Execute shell commands for build and system operations.

Args:
- command (string): The shell command to execute

IMPORTANT RESTRICTIONS:
This tool is for terminal operations ONLY. DO NOT use for file operations:
- DON'T use grep/rg - use the search tool instead
- DON'T use sed/awk - use the file or code_edit tool instead
- DON'T use cat/head/tail - use the file tool instead
- DON'T use find - use the search tool instead

ALLOWED uses:
- Build commands: go build, npm run build, make, etc.
- Test commands: go test, npm test, pytest, etc.
- Package management: go mod tidy, npm install, pip install, etc.
- System info: pwd, ls (simple), which, echo, etc.
- Git operations (if not using git tool)

Commands are checked against allow/deny lists in config.

Example: {"tool": "bash", "args": {"command": "go build ./..."}}`
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

// Metadata returns rich tool metadata for discovery and LLM guidance
func (b *BashTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"command": {
					Type:        "string",
					Description: "The shell command to execute",
				},
			},
			Required: []string{"command"},
		},
		Category:             CategoryBuild,
		Optionality:          ToolRequired,
		UsageHint:            "Use for build, test, and package commands. DON'T use grep/sed/cat - use file and search tools instead.",
		RequiresCapabilities: []string{"bash"},
		Examples: []ToolExample{
			{
				Description: "Build a Go project",
				Input:       map[string]interface{}{"command": "go build ./..."},
			},
			{
				Description: "Install npm dependencies",
				Input:       map[string]interface{}{"command": "npm install"},
			},
			{
				Description: "Run make target",
				Input:       map[string]interface{}{"command": "make build"},
			},
		},
		Produces: []string{"command_output"},
	}
}

// BashExploreTool is a variant of BashTool for code exploration (Plan/Analyze phases).
// Allows grep/find for searching but forbids file editing commands like sed/awk.
type BashExploreTool struct {
	*BashTool
}

// NewBashExploreTool creates a bash tool for exploration with grep/find enabled
func NewBashExploreTool(cfg *config.Config, workDir string) *BashExploreTool {
	// Override allowed commands for exploration
	allowed := map[string]bool{
		"git": true, "gh": true, "go": true, "cat": true, "ls": true,
		"head": true, "tail": true, "wc": true, "sort": true, "uniq": true,
		"grep": true, "find": true, // Allowed for exploration
	}

	// Override forbidden commands - no file editing
	forbidden := map[string]bool{
		"sed": true, "awk": true, // File editing forbidden in explore mode
		"rm": true, "mv": true, "dd": true, "sudo": true, "xargs": true,
	}

	return &BashExploreTool{
		BashTool: &BashTool{
			allowedCommands:   allowed,
			forbiddenCommands: forbidden,
			workDir:           workDir,
		},
	}
}

// Name returns the tool name
func (b *BashExploreTool) Name() string {
	return "bash"
}

// Description returns the tool description
func (b *BashExploreTool) Description() string {
	return `Execute shell commands for code exploration and search.

Args:
- command (string): The shell command to execute

This variant is for EXPLORATION/SEARCH operations (Plan/Analyze phases):
- grep/find: ALLOWED for searching code
- sed/awk: FORBIDDEN (use file/code_edit tools in Implement phase)

ALLOWED commands: git, gh, go, cat, ls, head, tail, wc, sort, uniq, grep, find
FORBIDDEN commands: sed, awk, rm, mv, dd, sudo, xargs

Examples:
- grep -r "function.*Error" pkg/
- find . -name "*.go" -type f
- git log --oneline | head -10`
}

// BashEditTool is a variant of BashTool for file editing (Implement phase).
// Allows sed/awk for file manipulation but forbids grep/find (use SearchTool instead).
type BashEditTool struct {
	*BashTool
}

// NewBashEditTool creates a bash tool for file editing with sed/awk enabled
func NewBashEditTool(cfg *config.Config, workDir string) *BashEditTool {
	// Override allowed commands for editing
	allowed := map[string]bool{
		"git": true, "gh": true, "go": true, "cat": true, "ls": true,
		"head": true, "tail": true, "wc": true, "sort": true, "uniq": true,
		"sed": true, "awk": true, "tee": true, "cut": true, "tr": true, // File editing allowed
	}

	// Override forbidden commands - no search (use SearchTool)
	forbidden := map[string]bool{
		"grep": true, "find": true, // Search forbidden in edit mode (use SearchTool)
		"rm": true, "mv": true, "dd": true, "sudo": true, "xargs": true,
	}

	return &BashEditTool{
		BashTool: &BashTool{
			allowedCommands:   allowed,
			forbiddenCommands: forbidden,
			workDir:           workDir,
		},
	}
}

// Name returns the tool name
func (b *BashEditTool) Name() string {
	return "bash"
}

// Description returns the tool description
func (b *BashEditTool) Description() string {
	return `Execute shell commands for file editing and system operations.

Args:
- command (string): The shell command to execute

This variant is for FILE EDITING operations (Implement phase):
- sed/awk: ALLOWED for regex-based file editing
- grep/find: FORBIDDEN (use SearchTool for searching)

FILE EDITING TOOL SELECTION:
- Use 'sed' for regex-based stream editing (complex find/replace patterns)
- Use 'awk' for field/column-based text processing
- Use 'file' tool for simple string replacements (cross-platform)
- Use 'code_edit' tool for precise line-based changes (preserves indentation)

ALLOWED commands: git, gh, go, cat, ls, head, tail, wc, sort, uniq, sed, awk, tee, cut, tr
FORBIDDEN commands: grep, find, rm, mv, dd, sudo, xargs

Examples:
- sed 's/old/new/g' file.txt                    # Global text replacement
- sed -i 's/fmt\\.Print/slog.Info/g' *.go       # Multi-file regex replace
- awk '{print $1}' file.txt                     # Extract first column
- go build ./...                                # Build project
- git add . && git commit -m "message"          # Git operations`
}
