package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

// REPL represents the interactive REPL
type REPL struct {
	session *Session
	input   *InputHandler
	output  *ProgressOutput
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewREPL creates a new REPL instance
func NewREPL(session *Session) (*REPL, error) {
	// Create input handler
	input, err := NewInputHandler(session)
	if err != nil {
		return nil, fmt.Errorf("failed to create input handler: %w", err)
	}

	// Create output handler
	output := NewProgressOutput()
	output.SetWriter(os.Stdout)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	return &REPL{
		session: session,
		input:   input,
		output:  output,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Run starts the REPL loop
func (r *REPL) Run() error {
	defer r.Close()

	// Suppress stderr in non-debug mode
	cleanup := ConditionalStderr(r.session.DebugMode)
	defer cleanup()

	// Print welcome message
	r.printWelcome()

	// Main REPL loop
	for {
		// Read input
		line, err := r.input.ReadLine()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C - just show new prompt
				continue
			}
			if err == io.EOF {
				// Ctrl+D or EOF - exit gracefully
				r.output.PrintMessage("\nGoodbye!\n")
				return nil
			}
			return fmt.Errorf("input error: %w", err)
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Add to history
		r.session.AddToHistory(line)

		// Parse command
		cmd := ParseCommand(line)

		// Handle command
		if err := r.handleCommand(cmd); err != nil {
			if err == io.EOF {
				// Exit requested
				r.output.PrintMessage("\nGoodbye!\n")
				return nil
			}
			r.output.PrintError("Error: %v\n", err)
		}
	}
}

// handleCommand handles a parsed command
func (r *REPL) handleCommand(cmd *Command) error {
	switch cmd.Type {
	case CommandTypeREPL:
		return r.handleREPLCommand(cmd)
	case CommandTypeSlash:
		return r.handleSlashCommand(cmd)
	case CommandTypeNatural:
		return r.handleNaturalLanguage(cmd)
	default:
		r.output.PrintWarning("Unknown command type\n")
		return nil
	}
}

// handleREPLCommand handles REPL-specific commands
func (r *REPL) handleREPLCommand(cmd *Command) error {
	switch cmd.Name {
	case "help", "h", "?":
		r.output.PrintMessage("%s", GetREPLHelp(r.session.Mode))
		return nil

	case "quit", "exit", "q":
		return io.EOF

	case "history":
		r.printHistory()
		return nil

	case "clear", "cls":
		ClearScreen()
		return nil

	case "mode":
		return r.switchMode(cmd)

	case "context", "info":
		r.printContext()
		return nil

	case "logs":
		r.printLogs()
		return nil

	case "interactive":
		r.session.SetInteractiveMode(true)
		r.output.PrintSuccess("✅ Interactive mode enabled (default)\n")
		r.output.PrintMessage("   Agent will ask for approval before writing code\n")
		return nil

	case "background", "auto":
		r.session.SetInteractiveMode(false)
		r.output.PrintWarning("⚡ Background mode enabled\n")
		r.output.PrintMessage("   Agent will run autonomously without approval\n")
		return nil

	default:
		r.output.PrintWarning("Unknown REPL command: /%s\n", cmd.Name)
		r.output.PrintMessage("Type /help for available commands\n")
		return nil
	}
}

// handleSlashCommand handles slash commands (future PR #60 integration)
func (r *REPL) handleSlashCommand(cmd *Command) error {
	r.output.PrintWarning("Slash commands not yet implemented\n")
	r.output.PrintMessage("Command: /%s\n", cmd.Name)
	r.output.PrintMessage("This will be implemented when PR #60 merges\n")
	return nil
}

// handleNaturalLanguage handles natural language prompts
func (r *REPL) handleNaturalLanguage(cmd *Command) error {
	agent := r.session.GetCurrentAgent()

	// Log input
	r.session.Logger.LogInput(cmd.Text)
	r.session.Logger.LogAgent("Agent: %s\n", agent)
	r.session.Logger.LogAgent("Prompt: %s\n", cmd.Text)

	// Check if interactive mode is enabled
	if r.session.IsInteractive() {
		return r.handleInteractive(agent, cmd.Text)
	}

	// Background mode - direct execution
	return r.handleBackground(agent, cmd.Text)
}

// handleInteractive handles interactive execution with approval
func (r *REPL) handleInteractive(agent string, prompt string) error {
	r.output.PrintMessage("\n🔍 Analyzing your request (interactive mode)...\n")
	r.output.PrintMessage("   Task: %s\n\n", prompt)

	// Ask for confirmation before starting
	r.output.PrintMessage("Start this task? [y/n]: ")
	line, err := r.input.ReadLine()
	if err != nil {
		return err
	}

	response := strings.TrimSpace(strings.ToLower(line))
	if response != "y" && response != "yes" {
		r.output.PrintWarning("❌ Task cancelled\n")
		return nil
	}

	r.output.PrintMessage("\n🤖 Processing with %s agent...\n", agent)
	r.output.PrintMessage("   (Running in background - full interactive workflow coming soon!)\n\n")

	// For now, execute normally
	// TODO: Add proposal → approve → apply workflow
	return r.handleBackground(agent, prompt)
}

// handleBackground handles background execution without approval
func (r *REPL) handleBackground(agent string, prompt string) error {
	r.output.PrintMessage("\n🤖 Processing with %s agent...\n\n", agent)

	// Execute agent via bridge
	result, err := r.session.Bridge.ExecuteAgent(r.ctx, agent, prompt)

	if err != nil {
		r.session.Logger.LogError(err)
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// Log output
	r.session.Logger.LogOutput(result.Output)

	// Display result
	if !result.Success {
		r.output.PrintError("❌ Agent failed: %s\n", result.Error)
	} else {
		r.output.PrintSuccess("✅ %s\n", result.Output)
	}

	return nil
}

// switchMode switches the current agent mode
func (r *REPL) switchMode(cmd *Command) error {
	// Get new mode from args
	if len(cmd.Args) == 0 {
		r.output.PrintWarning("Usage: /mode <agent>\n")
		r.output.PrintMessage("Available agents for %s mode:\n", r.session.Mode)
		r.printAvailableAgents()
		return nil
	}

	newAgent, ok := cmd.Args["arg0"].(string)
	if !ok {
		return fmt.Errorf("invalid agent name")
	}

	// Validate agent for current mode
	if !r.isValidAgent(newAgent) {
		r.output.PrintWarning("Invalid agent: %s\n", newAgent)
		r.printAvailableAgents()
		return nil
	}

	// Switch agent
	oldAgent := r.session.GetCurrentAgent()
	r.session.SetCurrentAgent(newAgent)
	r.input.UpdatePrompt()

	r.output.PrintSuccess("Switched from %s to %s\n", oldAgent, newAgent)

	return nil
}

// isValidAgent checks if an agent is valid for the current mode
func (r *REPL) isValidAgent(agent string) bool {
	validAgents := r.getValidAgentsForMode()
	for _, valid := range validAgents {
		if agent == valid {
			return true
		}
	}
	return false
}

// getValidAgentsForMode returns valid agents for the current mode
func (r *REPL) getValidAgentsForMode() []string {
	switch r.session.Mode {
	case "code":
		return []string{"build", "debug", "review", "triage"}
	case "blog":
		return []string{"blog", "writer", "editor"}
	case "podcast":
		return []string{"podcast"}
	default:
		return []string{"build"}
	}
}

// printAvailableAgents prints available agents for the current mode
func (r *REPL) printAvailableAgents() {
	agents := r.getValidAgentsForMode()
	for _, agent := range agents {
		r.output.PrintMessage("  - %s\n", agent)
	}
}

// printHistory prints command history
func (r *REPL) printHistory() {
	history := r.session.GetHistory()
	if len(history) == 0 {
		r.output.PrintMessage("No command history\n")
		return
	}

	r.output.PrintMessage("Command History:\n")
	for i, cmd := range history {
		r.output.PrintMessage("  %d: %s\n", i+1, cmd)
	}
}

// printContext prints current session context
func (r *REPL) printContext() {
	duration := time.Since(r.session.StartTime).Round(time.Second)

	r.output.PrintMessage("Session Context:\n")
	r.output.PrintMessage("  Session ID: %s\n", r.session.ID)
	r.output.PrintMessage("  Mode: %s\n", r.session.Mode)
	r.output.PrintMessage("  Current Agent: %s\n", r.session.GetCurrentAgent())
	r.output.PrintMessage("  Duration: %s\n", duration)
	r.output.PrintMessage("  Commands: %d\n", len(r.session.GetHistory()))

	if activeJob := r.session.GetActiveJob(); activeJob != "" {
		r.output.PrintMessage("  Active Job: %s\n", activeJob)
	}
}

// printLogs shows log file status and recent entries
func (r *REPL) printLogs() {
	if !r.session.DebugMode {
		r.output.PrintWarning("Debug mode not enabled - logs will be auto-cleaned on exit\n")
		r.output.PrintMessage("Start with --debug to keep logs\n\n")
	}

	logDir := r.session.Logger.GetSessionDir()
	r.output.PrintMessage("📁 Log Directory: %s\n\n", logDir)

	// Show file sizes and quick preview
	files := []struct {
		name string
		desc string
	}{
		{"session.log", "Full transcript"},
		{"agent-calls.log", "Agent execution"},
		{"tool-calls.log", "Tool calls"},
		{"llm-requests.log", "LLM API calls (debug only)"},
	}

	for _, f := range files {
		path := fmt.Sprintf("%s/%s", logDir, f.name)
		info, err := os.Stat(path)
		if err != nil {
			r.output.PrintMessage("  %-20s : %s (not found)\n", f.name, f.desc)
			continue
		}

		size := info.Size()
		sizeStr := fmt.Sprintf("%d bytes", size)
		if size == 0 {
			sizeStr = "empty"
		} else if size > 1024 {
			sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
		}

		r.output.PrintMessage("  %-20s : %s (%s)\n", f.name, f.desc, sizeStr)
	}

	r.output.PrintMessage("\nTo view logs:\n")
	r.output.PrintMessage("  cat %s/session.log\n", logDir)
	r.output.PrintMessage("  tail -f %s/*.log\n", logDir)
}

// printWelcome prints the welcome message
func (r *REPL) printWelcome() {
	r.output.PrintMessage("\n")
	r.output.PrintMessage("╔═══════════════════════════════════════════╗\n")
	r.output.PrintMessage("║   pedrocode - Interactive Coding Agent   ║\n")
	r.output.PrintMessage("╚═══════════════════════════════════════════╝\n")
	r.output.PrintMessage("\n")
	r.output.PrintMessage("Mode: %s\n", r.session.Mode)
	r.output.PrintMessage("Agent: %s\n", r.session.GetCurrentAgent())

	// Show debug info if enabled
	if r.session.DebugMode {
		r.output.PrintMessage("\n")
		r.output.PrintMessage("🐛 Debug mode enabled\n")
		r.output.PrintMessage("📁 Logs: %s\n", r.session.Logger.GetSessionDir())
		r.output.PrintMessage("   - session.log       : Full transcript\n")
		r.output.PrintMessage("   - agent-calls.log   : Agent execution details\n")
		r.output.PrintMessage("   - tool-calls.log    : Tool execution details\n")
		r.output.PrintMessage("   - llm-requests.log  : LLM API calls\n")
		r.output.PrintMessage("\n")
		r.output.PrintMessage("Logs will be kept after exit for debugging\n")
	}

	r.output.PrintMessage("\n")
	r.output.PrintMessage("Type /help for available commands\n")
	r.output.PrintMessage("Type /quit to exit\n")
	r.output.PrintMessage("\n")
}

// Close closes the REPL and cleans up resources
func (r *REPL) Close() error {
	r.cancel()

	// Flush logs before closing
	if r.session.Logger != nil {
		r.session.Logger.Flush()
	}

	// Close session (cleans up logs if not in debug mode)
	if err := r.session.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close session: %v\n", err)
	}

	if r.input != nil {
		return r.input.Close()
	}

	return nil
}
