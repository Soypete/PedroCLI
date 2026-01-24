package repl

import (
	"fmt"
	"strings"
)

// CommandType represents different types of commands
type CommandType int

const (
	CommandTypeUnknown CommandType = iota
	CommandTypeREPL                // REPL-specific commands (/help, /quit, etc.)
	CommandTypeSlash               // Slash commands (when PR #60 merges)
	CommandTypeNatural             // Natural language prompts
)

// Command represents a parsed command
type Command struct {
	Type CommandType
	Name string                 // For slash commands
	Args map[string]interface{} // For slash commands
	Text string                 // For natural language
	Raw  string                 // Original input
}

// ParseCommand parses user input into a Command
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return &Command{
			Type: CommandTypeUnknown,
			Raw:  input,
		}
	}

	// Check for slash commands
	if strings.HasPrefix(input, "/") {
		return parseSlashCommand(input)
	}

	// Everything else is natural language
	return &Command{
		Type: CommandTypeNatural,
		Text: input,
		Raw:  input,
	}
}

// parseSlashCommand parses a slash command
func parseSlashCommand(input string) *Command {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return &Command{
			Type: CommandTypeUnknown,
			Raw:  input,
		}
	}

	cmdName := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	// Check if it's a REPL-specific command
	if isREPLCommand(cmdName) {
		return &Command{
			Type: CommandTypeREPL,
			Name: cmdName,
			Args: parseArgs(args),
			Raw:  input,
		}
	}

	// Otherwise it's a slash command (for future PR #60 integration)
	return &Command{
		Type: CommandTypeSlash,
		Name: cmdName,
		Args: parseArgs(args),
		Raw:  input,
	}
}

// isREPLCommand checks if a command is a REPL-specific command
func isREPLCommand(cmd string) bool {
	replCommands := []string{
		"help", "h", "?",
		"quit", "exit", "q",
		"history",
		"clear", "cls",
		"mode",
		"context", "info",
	}

	for _, replCmd := range replCommands {
		if cmd == replCmd {
			return true
		}
	}

	return false
}

// parseArgs parses command arguments into a map
func parseArgs(args []string) map[string]interface{} {
	result := make(map[string]interface{})

	// Simple positional argument parsing for now
	// Can be enhanced later for flag-style arguments (-key value)
	for i, arg := range args {
		result[fmt.Sprintf("arg%d", i)] = arg
	}

	return result
}

// GetREPLHelp returns help text for REPL commands
func GetREPLHelp(mode string) string {
	help := `
pedrocode - Interactive REPL for PedroCLI

REPL Commands:
  /help, /h, /?        Show this help message
  /quit, /exit, /q     Exit the REPL
  /history             Show command history
  /clear, /cls         Clear the screen
  /mode <agent>        Switch agent mode
  /context, /info      Show current session info
  /logs                Show log files and status
  /interactive         Enable interactive mode (default - asks for approval)
  /background, /auto   Enable background mode (runs without approval)

`

	// Mode-specific help
	switch mode {
	case "code":
		help += `Code Mode Agents:
  build    - Build new features from descriptions
  debug    - Debug and fix issues
  review   - Code review on branches/PRs
  triage   - Diagnose issues without fixing

Usage:
  pedro> build a rate limiter for the API
  pedro> /mode debug
  pedro> fix the authentication bug
`
	case "blog":
		help += `Blog Mode:
  Write blog posts, newsletters, and content

Usage:
  pedro> write about Go contexts and best practices
  pedro> create a newsletter from recent posts
`
	case "podcast":
		help += `Podcast Mode:
  Create podcast outlines, scripts, and episode prep

Usage:
  pedro> outline for episode about choosing an LLM
  pedro> generate script from outline.md
`
	}

	help += `
Natural Language:
  Just type your request naturally - no slash needed!
  Example: "add rate limiting to the API"

Note: Use Ctrl+D or enter an empty line to submit multi-line input.
`

	return help
}
