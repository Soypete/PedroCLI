// Package commands provides OpenCode-inspired command management with template substitution.
package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Command represents a slash command with template substitution
type Command struct {
	Name        string
	Description string
	Template    string // Prompt template with $ARGUMENTS, $1, $2, @file, !`cmd`
	Agent       string // Optional agent override
	Model       string // Optional model override
	Subtask     bool   // Run as subagent/subtask
	Source      string // "builtin", "config", "markdown"
}

// CommandRegistry manages command registration and execution
type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]*Command
	builtins map[string]BuiltinCommand
	workDir  string
}

// BuiltinCommand is a command implemented in code
type BuiltinCommand interface {
	Name() string
	Description() string
	Execute(args []string, ctx *ExecutionContext) (string, error)
}

// ExecutionContext provides context for command execution
type ExecutionContext struct {
	WorkDir  string
	Agent    string
	Model    string
	FileRead func(path string) (string, error)
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry(workDir string) *CommandRegistry {
	r := &CommandRegistry{
		commands: make(map[string]*Command),
		builtins: make(map[string]BuiltinCommand),
		workDir:  workDir,
	}
	r.registerBuiltins()
	return r
}

// registerBuiltins registers built-in commands
func (r *CommandRegistry) registerBuiltins() {
	builtins := []BuiltinCommand{
		&HelpCommand{},
		&ClearCommand{},
		&UndoCommand{},
		&RedoCommand{},
		&CompactCommand{},
		&StatusCommand{},
	}

	for _, cmd := range builtins {
		r.builtins[cmd.Name()] = cmd
	}
}

// Register adds or updates a command in the registry
func (r *CommandRegistry) Register(name string, cmd *Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cmd.Name = name
	r.commands[name] = cmd
}

// Get returns a command by name
func (r *CommandRegistry) Get(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check user-defined commands first
	if cmd, ok := r.commands[name]; ok {
		return cmd, true
	}

	// Check builtins
	if builtin, ok := r.builtins[name]; ok {
		return &Command{
			Name:        builtin.Name(),
			Description: builtin.Description(),
			Source:      "builtin",
		}, true
	}

	return nil, false
}

// List returns all registered commands
func (r *CommandRegistry) List() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Command, 0, len(r.commands)+len(r.builtins))

	// Add user commands
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}

	// Add builtins
	for _, builtin := range r.builtins {
		result = append(result, &Command{
			Name:        builtin.Name(),
			Description: builtin.Description(),
			Source:      "builtin",
		})
	}

	return result
}

// IsBuiltin checks if a command name is a builtin
func (r *CommandRegistry) IsBuiltin(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.builtins[name]
	return ok
}

// ExecuteBuiltin executes a builtin command
func (r *CommandRegistry) ExecuteBuiltin(name string, args []string, ctx *ExecutionContext) (string, error) {
	r.mu.RLock()
	builtin, ok := r.builtins[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("builtin command not found: %s", name)
	}

	return builtin.Execute(args, ctx)
}

// ExpandTemplate expands a command template with arguments and substitutions
func (r *CommandRegistry) ExpandTemplate(template string, args []string, ctx *ExecutionContext) (string, error) {
	result := template

	// Replace $ARGUMENTS with all args joined
	result = strings.ReplaceAll(result, "$ARGUMENTS", strings.Join(args, " "))

	// Replace positional arguments $1, $2, etc.
	for i, arg := range args {
		placeholder := fmt.Sprintf("$%d", i+1)
		result = strings.ReplaceAll(result, placeholder, arg)
	}

	// Execute shell commands !`cmd`
	result, err := r.executeShellCommands(result)
	if err != nil {
		return "", err
	}

	// Expand file references @filepath
	result, err = r.expandFileReferences(result, ctx)
	if err != nil {
		return "", err
	}

	return result, nil
}

// executeShellCommands expands !`cmd` patterns in template
func (r *CommandRegistry) executeShellCommands(template string) (string, error) {
	re := regexp.MustCompile("!`([^`]+)`")

	var lastErr error
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract command from !`cmd`
		cmd := match[2 : len(match)-1]

		// Execute command
		execCmd := exec.Command("sh", "-c", cmd)
		execCmd.Dir = r.workDir

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		execCmd.Stdout = &stdout
		execCmd.Stderr = &stderr

		if err := execCmd.Run(); err != nil {
			lastErr = fmt.Errorf("command '%s' failed: %w\nstderr: %s", cmd, err, stderr.String())
			return fmt.Sprintf("[ERROR: %s]", err)
		}

		return strings.TrimSpace(stdout.String())
	})

	return result, lastErr
}

// expandFileReferences expands @filepath patterns in template
func (r *CommandRegistry) expandFileReferences(template string, ctx *ExecutionContext) (string, error) {
	re := regexp.MustCompile(`@([^\s,\]]+)`)

	var lastErr error
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract path from @path
		path := match[1:]

		// Make path absolute if relative
		if !filepath.IsAbs(path) {
			path = filepath.Join(r.workDir, path)
		}

		// Read file content
		var content string
		var err error

		if ctx != nil && ctx.FileRead != nil {
			content, err = ctx.FileRead(path)
		} else {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				err = readErr
			} else {
				content = string(data)
			}
		}

		if err != nil {
			lastErr = fmt.Errorf("failed to read file '%s': %w", path, err)
			return fmt.Sprintf("[ERROR: cannot read %s]", path)
		}

		return content
	})

	return result, lastErr
}

// LoadFromConfig loads commands from JSON configuration
func (r *CommandRegistry) LoadFromConfig(commands map[string]CommandConfig) error {
	for name, cfg := range commands {
		cmd := cfg.ToCommand()
		cmd.Name = name
		cmd.Source = "config"
		r.Register(name, cmd)
	}
	return nil
}

// LoadMarkdownCommands loads commands from markdown files
func (r *CommandRegistry) LoadMarkdownCommands(basePaths ...string) error {
	for _, basePath := range basePaths {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			continue // Skip non-existent directories
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				path := filepath.Join(basePath, entry.Name())
				cmd, err := r.loadMarkdownCommand(path)
				if err != nil {
					continue // Skip invalid files
				}
				cmd.Source = "markdown"
				r.Register(cmd.Name, cmd)
			}
		}
	}
	return nil
}

// CommandFrontmatter represents the YAML frontmatter in command markdown files
type CommandFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Agent       string `yaml:"agent"`
	Model       string `yaml:"model"`
	Subtask     bool   `yaml:"subtask"`
}

// loadMarkdownCommand loads a single command from a markdown file
func (r *CommandRegistry) loadMarkdownCommand(path string) (*Command, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	frontmatter, body := parseFrontmatter(string(content))

	var fm CommandFrontmatter
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	// Default name from filename
	if fm.Name == "" {
		fm.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}

	return &Command{
		Name:        fm.Name,
		Description: fm.Description,
		Template:    body,
		Agent:       fm.Agent,
		Model:       fm.Model,
		Subtask:     fm.Subtask,
	}, nil
}

// parseFrontmatter extracts YAML frontmatter and body from markdown content
func parseFrontmatter(content string) (frontmatter, body string) {
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n(.*)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return "", content
}

// CommandConfig represents the JSON configuration for a command
type CommandConfig struct {
	Template    string `json:"template"`
	Description string `json:"description,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Model       string `json:"model,omitempty"`
	Subtask     bool   `json:"subtask,omitempty"`
}

// ToCommand converts a CommandConfig to a Command
func (c *CommandConfig) ToCommand() *Command {
	return &Command{
		Description: c.Description,
		Template:    c.Template,
		Agent:       c.Agent,
		Model:       c.Model,
		Subtask:     c.Subtask,
	}
}

// ---- Built-in commands ----

// HelpCommand shows help information
type HelpCommand struct{}

func (c *HelpCommand) Name() string        { return "help" }
func (c *HelpCommand) Description() string { return "Show help information" }
func (c *HelpCommand) Execute(args []string, ctx *ExecutionContext) (string, error) {
	return "Use Tab to cycle agents, /command for commands. Type your prompt and press Enter.", nil
}

// ClearCommand clears the conversation
type ClearCommand struct{}

func (c *ClearCommand) Name() string        { return "clear" }
func (c *ClearCommand) Description() string { return "Clear the conversation" }
func (c *ClearCommand) Execute(args []string, ctx *ExecutionContext) (string, error) {
	return "__CLEAR__", nil
}

// UndoCommand undoes the last change
type UndoCommand struct{}

func (c *UndoCommand) Name() string        { return "undo" }
func (c *UndoCommand) Description() string { return "Undo the last change" }
func (c *UndoCommand) Execute(args []string, ctx *ExecutionContext) (string, error) {
	return "__UNDO__", nil
}

// RedoCommand redoes the last undone change
type RedoCommand struct{}

func (c *RedoCommand) Name() string        { return "redo" }
func (c *RedoCommand) Description() string { return "Redo the last undone change" }
func (c *RedoCommand) Execute(args []string, ctx *ExecutionContext) (string, error) {
	return "__REDO__", nil
}

// CompactCommand compacts the conversation context
type CompactCommand struct{}

func (c *CompactCommand) Name() string        { return "compact" }
func (c *CompactCommand) Description() string { return "Compact the conversation context" }
func (c *CompactCommand) Execute(args []string, ctx *ExecutionContext) (string, error) {
	return "__COMPACT__", nil
}

// StatusCommand shows current status
type StatusCommand struct{}

func (c *StatusCommand) Name() string        { return "status" }
func (c *StatusCommand) Description() string { return "Show current agent and model status" }
func (c *StatusCommand) Execute(args []string, ctx *ExecutionContext) (string, error) {
	if ctx == nil {
		return "No context available", nil
	}
	return fmt.Sprintf("Agent: %s\nModel: %s\nWork Dir: %s", ctx.Agent, ctx.Model, ctx.WorkDir), nil
}
