// Package cli provides command-line interface utilities for PedroCLI.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/soypete/pedrocli/pkg/commands"
	"github.com/soypete/pedrocli/pkg/config"
)

// CommandRunner handles slash command execution from the CLI
type CommandRunner struct {
	registry *commands.CommandRegistry
	config   *config.Config
	workDir  string
}

// NewCommandRunner creates a new command runner
func NewCommandRunner(cfg *config.Config, workDir string) *CommandRunner {
	registry := commands.NewCommandRegistry(workDir)

	// Load commands from standard paths
	cmdPaths := []string{
		".pedro/command",
	}

	if home, err := os.UserHomeDir(); err == nil {
		cmdPaths = append(cmdPaths, home+"/.config/pedrocli/command")
	}

	if err := registry.LoadMarkdownCommands(cmdPaths...); err != nil {
		// Log but don't fail if markdown commands can't be loaded
		fmt.Fprintf(os.Stderr, "Warning: failed to load markdown commands: %v\n", err)
	}

	return &CommandRunner{
		registry: registry,
		config:   cfg,
		workDir:  workDir,
	}
}

// ListCommands returns all available commands
func (r *CommandRunner) ListCommands() []*commands.Command {
	return r.registry.List()
}

// GetCommand returns a command by name
func (r *CommandRunner) GetCommand(name string) (*commands.Command, bool) {
	return r.registry.Get(name)
}

// ExpandCommand expands a command template with arguments
func (r *CommandRunner) ExpandCommand(name string, args []string) (string, error) {
	cmd, ok := r.registry.Get(name)
	if !ok {
		return "", fmt.Errorf("command not found: %s", name)
	}

	// Handle builtins
	if r.registry.IsBuiltin(name) {
		ctx := &commands.ExecutionContext{
			WorkDir: r.workDir,
			Model:   r.config.Model.ModelName,
		}
		return r.registry.ExecuteBuiltin(name, args, ctx)
	}

	// Expand template
	ctx := &commands.ExecutionContext{
		WorkDir: r.workDir,
		Model:   r.config.Model.ModelName,
		FileRead: func(path string) (string, error) {
			data, err := os.ReadFile(path)
			return string(data), err
		},
	}

	return r.registry.ExpandTemplate(cmd.Template, args, ctx)
}

// ParseSlashCommand parses a slash command string into name and arguments
// e.g., "/blog-outline My topic here" -> ("blog-outline", ["My", "topic", "here"])
func ParseSlashCommand(input string) (name string, args []string, isCommand bool) {
	input = strings.TrimSpace(input)

	// Must start with /
	if !strings.HasPrefix(input, "/") {
		return "", nil, false
	}

	// Remove leading /
	input = strings.TrimPrefix(input, "/")

	// Split into parts
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil, false
	}

	return parts[0], parts[1:], true
}

// RunCommand executes a slash command and returns the expanded prompt
func (r *CommandRunner) RunCommand(ctx context.Context, input string) (string, error) {
	name, args, isCommand := ParseSlashCommand(input)
	if !isCommand {
		return "", fmt.Errorf("not a slash command: %s", input)
	}

	return r.ExpandCommand(name, args)
}

// PrintHelp prints help for all available commands
func (r *CommandRunner) PrintHelp() {
	cmds := r.ListCommands()

	fmt.Println("Available slash commands:")
	fmt.Println()

	// Group by source
	builtins := make([]*commands.Command, 0)
	custom := make([]*commands.Command, 0)

	for _, cmd := range cmds {
		if cmd.Source == "builtin" {
			builtins = append(builtins, cmd)
		} else {
			custom = append(custom, cmd)
		}
	}

	if len(builtins) > 0 {
		fmt.Println("Built-in commands:")
		for _, cmd := range builtins {
			fmt.Printf("  /%s - %s\n", cmd.Name, cmd.Description)
		}
		fmt.Println()
	}

	if len(custom) > 0 {
		fmt.Println("Custom commands (from .pedro/command/):")
		for _, cmd := range custom {
			desc := cmd.Description
			if desc == "" {
				desc = "(no description)"
			}
			agent := ""
			if cmd.Agent != "" {
				agent = fmt.Sprintf(" [agent: %s]", cmd.Agent)
			}
			fmt.Printf("  /%s - %s%s\n", cmd.Name, desc, agent)
		}
		fmt.Println()
	}

	fmt.Println("Usage:")
	fmt.Println("  pedrocli run /command-name [arguments...]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  pedrocli run /blog-outline \"Building CLI tools in Go\"")
	fmt.Println("  pedrocli run /test")
	fmt.Println("  pedrocli run /lint")
}
