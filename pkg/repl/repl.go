package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/soypete/pedrocli/pkg/cli"
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

	case "jobs":
		r.printJobs(cmd)
		return nil

	case "cancel":
		return r.cancelJob(cmd)

	case "interactive":
		r.session.SetInteractiveMode(true)
		r.output.PrintSuccess("✅ Interactive mode enabled (default)\n")
		r.output.PrintMessage("   Agent will pause after EACH phase for your review\n")
		r.output.PrintMessage("   You can continue, retry, or cancel at each step\n")
		ShowCompletePedro(r.output.writer)
		return nil

	case "background", "auto":
		r.session.SetInteractiveMode(false)
		r.output.PrintWarning("⚡ Background mode enabled\n")
		r.output.PrintMessage("   Jobs will run autonomously without approval\n")
		return nil

	default:
		r.output.PrintWarning("Unknown REPL command: /%s\n", cmd.Name)
		r.output.PrintMessage("Type /help for available commands\n")
		return nil
	}
}

// handleSlashCommand handles slash commands with template expansion
func (r *REPL) handleSlashCommand(cmd *Command) error {
	// Create command runner
	workDir := r.session.Config.Project.Workdir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			workDir = "."
		}
	}

	runner := cli.NewCommandRunner(r.session.Config, workDir)

	// Build input string from command name and args
	input := "/" + cmd.Name
	if len(cmd.Args) > 0 {
		args := make([]string, 0, len(cmd.Args))
		for i := 0; ; i++ {
			key := fmt.Sprintf("arg%d", i)
			if val, ok := cmd.Args[key].(string); ok {
				args = append(args, val)
			} else {
				break
			}
		}
		if len(args) > 0 {
			input = input + " " + strings.Join(args, " ")
		}
	}

	// Check if command exists
	name, _, _ := cli.ParseSlashCommand(input)
	command, ok := runner.GetCommand(name)
	if !ok {
		r.output.PrintError("❌ Command not found: /%s\n", name)
		r.output.PrintMessage("\nAvailable commands:\n")
		for _, c := range runner.ListCommands() {
			desc := c.Description
			if desc == "" {
				desc = "(no description)"
			}
			r.output.PrintMessage("  /%s - %s\n", c.Name, desc)
		}
		r.output.PrintMessage("\nType '/help' for REPL commands\n")
		return nil
	}

	// Expand command template
	expanded, err := runner.RunCommand(r.ctx, input)
	if err != nil {
		r.output.PrintError("❌ Command expansion failed: %v\n", err)
		return nil
	}

	// Show expanded prompt
	r.output.PrintMessage("\n📝 Expanded prompt:\n")
	r.output.PrintMessage("─────────────────────────────────────────\n")
	r.output.PrintMessage("%s\n", expanded)
	r.output.PrintMessage("─────────────────────────────────────────\n\n")

	// Log the expansion
	r.session.Logger.LogInput(fmt.Sprintf("Slash command: %s", input))
	r.session.Logger.LogOutput(fmt.Sprintf("Expanded to:\n%s", expanded))

	// Check if command has an associated agent
	if command.Agent == "" {
		r.output.PrintMessage("ℹ️  No agent configured for this command\n")
		r.output.PrintMessage("   Copy the expanded prompt to use manually, or type your own prompt\n")
		return nil
	}

	// Ask for confirmation before running agent
	r.output.PrintMessage("Run with %s agent? [y/n]: ", command.Agent)
	line, err := r.input.ReadLine()
	if err != nil {
		return err
	}

	response := strings.TrimSpace(strings.ToLower(line))
	if response != "y" && response != "yes" {
		r.output.PrintWarning("❌ Cancelled\n")
		return nil
	}

	// Execute with agent
	r.output.PrintMessage("\n🤖 Running %s agent...\n", command.Agent)

	// Map agent name to what the bridge expects
	agentName := command.Agent
	if agentName == "build" || agentName == "debug" || agentName == "review" || agentName == "triage" {
		// Code agents
		return r.handleBackground(agentName, expanded)
	} else if agentName == "blog" {
		// Blog agent
		return r.handleBackground("blog", expanded)
	} else if agentName == "podcast" {
		// Podcast agent
		return r.handleBackground("podcast", expanded)
	} else {
		r.output.PrintWarning("⚠️  Unknown agent: %s\n", agentName)
		r.output.PrintMessage("   Using expanded prompt as-is\n")
		return r.handleBackground("build", expanded)
	}
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

// handleInteractive handles phase-by-phase interactive execution
// Shows results after each phase and asks for user approval
func (r *REPL) handleInteractive(agent string, prompt string) error {
	return r.handleInteractivePhased(agent, prompt)
}

// handleBackground handles background execution without approval
func (r *REPL) handleBackground(agent string, prompt string) error {
	// Show Pedro working
	ShowPedro(r.output.writer)
	r.output.PrintMessage("\n")

	// Start progress spinner
	spinner := NewProgressSpinner(r.output.writer)
	spinner.Start(fmt.Sprintf("🤖 Running %s agent", agent))

	// Execute agent via bridge
	result, err := r.session.Bridge.ExecuteAgent(r.ctx, agent, prompt)

	// Stop spinner
	spinner.Stop()

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
