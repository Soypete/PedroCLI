package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/config"
)

// RunnerConfig configures the TUI runner.
type RunnerConfig struct {
	Config    *config.Config
	Bridge    *cli.CLIBridge
	Mode      string // code, blog, podcast
	DebugMode bool
}

// Run starts the Bubble Tea TUI. This replaces the old readline-based REPL.
// It runs the game-engine-style render loop: every frame, the View() is
// called, diffed against the previous frame, and minimal ANSI is emitted.
func Run(cfg RunnerConfig) error {
	agent := defaultAgentForMode(cfg.Mode)
	m := New(cfg.Mode, agent)

	// Wire up the OnSubmit callback to route user input
	bridge := cfg.Bridge
	m.OnSubmit = func(input string) tea.Cmd {
		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			return handleSlashInput(input, cfg.Mode)
		}

		// Natural language prompt -> run agent
		return func() tea.Msg {
			ctx := context.Background()
			result, err := bridge.ExecuteAgent(ctx, agent, input)
			if err != nil {
				return AgentDoneMsg{Success: false, Error: err.Error()}
			}
			return AgentDoneMsg{Success: result.Success, Output: result.Output, Error: result.Error}
		}
	}

	// Create Bubble Tea program with alt screen for full-window mode
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
	}

	if cfg.DebugMode {
		f, err := tea.LogToFile("/tmp/pedrocode-tui.log", "tui")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create TUI log: %v\n", err)
		} else {
			defer f.Close()
		}
	}

	p := tea.NewProgram(m, opts...)

	// Store program reference for the adapter
	// (the adapter can be created later when an agent starts)
	_ = NewAgentAdapter(p)

	// Run the program (blocks until quit)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// handleSlashInput handles /commands entered by the user.
func handleSlashInput(input, mode string) tea.Cmd {
	parts := strings.Fields(input)
	cmd := strings.TrimPrefix(parts[0], "/")

	switch cmd {
	case "quit", "exit", "q":
		return func() tea.Msg { return QuitMsg{} }

	case "mode":
		if len(parts) > 1 {
			newAgent := parts[1]
			if isValidAgent(newAgent, mode) {
				return func() tea.Msg { return ModeChangeMsg{Agent: newAgent} }
			}
			return func() tea.Msg {
				return OutputLineMsg{
					Text:  fmt.Sprintf("Invalid agent: %s. Valid agents for %s mode: %s", newAgent, mode, strings.Join(validAgents(mode), ", ")),
					Style: "warning",
				}
			}
		}
		return func() tea.Msg {
			return OutputLineMsg{
				Text:  fmt.Sprintf("Usage: /mode <agent>. Available: %s", strings.Join(validAgents(mode), ", ")),
				Style: "warning",
			}
		}

	case "help", "h", "?":
		return func() tea.Msg {
			return OutputLineMsg{Text: helpText(mode), Style: "info"}
		}

	case "clear", "cls":
		// Handled directly in model.handleBuiltinCommand
		return nil

	default:
		return func() tea.Msg {
			return OutputLineMsg{
				Text:  fmt.Sprintf("Unknown command: /%s. Type /help for available commands.", cmd),
				Style: "warning",
			}
		}
	}
}

// defaultAgentForMode returns the default agent name for a mode.
func defaultAgentForMode(mode string) string {
	switch mode {
	case "code":
		return "build"
	case "blog":
		return "blog"
	case "podcast":
		return "podcast"
	default:
		return "build"
	}
}

// validAgents returns valid agent names for a mode.
func validAgents(mode string) []string {
	switch mode {
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

// isValidAgent checks if an agent is valid for the given mode.
func isValidAgent(agent, mode string) bool {
	for _, valid := range validAgents(mode) {
		if agent == valid {
			return true
		}
	}
	return false
}
