package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tickInterval = 150 * time.Millisecond // ~7fps spinner animation
	maxOutput    = 1000                   // max output buffer lines
)

// Model is the top-level Bubble Tea model for PedroCLI's TUI.
// It follows the Elm Architecture: Init -> Update -> View loop.
type Model struct {
	// Layout
	width  int
	height int

	// Components
	progress *ProgressPanel
	output   *OutputPanel
	input    textinput.Model

	// State
	agent       string // current agent mode
	mode        string // code, blog, podcast
	ready       bool   // terminal size received
	agentBusy   bool   // agent is running
	quitting    bool
	history     []string
	historyIdx  int

	// Callbacks (set by the REPL integration layer)
	OnSubmit func(input string) tea.Cmd
}

// New creates a new TUI model with default settings.
func New(mode, agent string) Model {
	// Text input
	ti := textinput.New()
	ti.Prompt = promptString(agent)
	ti.Focus()
	ti.CharLimit = 4096

	return Model{
		progress:   NewProgressPanel(),
		output:     NewOutputPanel(maxOutput),
		input:      ti,
		agent:      agent,
		mode:       mode,
		history:    []string{},
		historyIdx: -1,
	}
}

// Init implements tea.Model. Starts the tick loop.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tickCmd(),
	)
}

// Update implements tea.Model. Processes messages and returns updated model + commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.SetWidth(msg.Width)
		m.output.SetWidth(msg.Width)
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case TickMsg:
		m.progress.Tick()
		return m, tickCmd()

	// --- Agent lifecycle ---

	case AgentStartMsg:
		m.agentBusy = true
		m.progress.StartAgent(msg.Agent, msg.Prompt, msg.Phases)
		m.output.Append(fmt.Sprintf("Running %s agent...", msg.Agent), "info")
		return m, nil

	case AgentDoneMsg:
		m.agentBusy = false
		if msg.Success {
			m.output.Append("Agent completed successfully.", "success")
			if msg.Output != "" {
				m.output.Append(msg.Output, "info")
			}
		} else {
			m.output.Append(fmt.Sprintf("Agent failed: %s", msg.Error), "error")
		}
		return m, nil

	// --- Progress updates ---

	case ProgressMsg:
		m.progress.UpdatePhase(msg)
		return m, nil

	// --- Tool / LLM events ---

	case ToolCallMsg:
		args := msg.Args
		if len(args) > 60 {
			args = args[:57] + "..."
		}
		m.output.Append(fmt.Sprintf("  > %s(%s)", msg.Name, args), "tool")
		return m, nil

	case ToolResultMsg:
		style := "success"
		prefix := "    ok"
		if !msg.Success {
			style = "error"
			prefix = "    fail"
		}
		text := msg.Output
		if len(text) > 80 {
			text = text[:77] + "..."
		}
		m.output.Append(fmt.Sprintf("%s: %s", prefix, text), style)
		return m, nil

	case LLMResponseMsg:
		m.output.Append(msg.Text, "llm")
		return m, nil

	case OutputLineMsg:
		m.output.Append(msg.Text, msg.Style)
		return m, nil

	// --- Mode change ---

	case ModeChangeMsg:
		m.agent = msg.Agent
		m.input.Prompt = promptString(msg.Agent)
		m.output.Append(fmt.Sprintf("Switched to %s agent.", msg.Agent), "info")
		return m, nil

	case QuitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	// Forward remaining messages to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey processes keyboard events.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyCtrlD:
		if m.input.Value() == "" {
			m.quitting = true
			return m, tea.Quit
		}

	case tea.KeyEnter:
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return m, nil
		}

		// Add to history
		m.history = append(m.history, text)
		m.historyIdx = len(m.history)

		// Clear input
		m.input.SetValue("")

		// Handle built-in commands
		if cmd := m.handleBuiltinCommand(text); cmd != nil {
			return m, cmd
		}

		// Delegate to OnSubmit callback
		if m.OnSubmit != nil {
			return m, m.OnSubmit(text)
		}

		m.output.Append(fmt.Sprintf("> %s", text), "info")
		return m, nil

	case tea.KeyUp:
		// History navigation
		if len(m.history) > 0 && m.historyIdx > 0 {
			m.historyIdx--
			m.input.SetValue(m.history[m.historyIdx])
			m.input.CursorEnd()
		}
		return m, nil

	case tea.KeyDown:
		// History navigation
		if m.historyIdx < len(m.history)-1 {
			m.historyIdx++
			m.input.SetValue(m.history[m.historyIdx])
			m.input.CursorEnd()
		} else {
			m.historyIdx = len(m.history)
			m.input.SetValue("")
		}
		return m, nil

	case tea.KeyPgUp:
		m.output.ScrollUp(10)
		return m, nil

	case tea.KeyPgDown:
		m.output.ScrollDown(10)
		return m, nil
	}

	// Forward to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// handleBuiltinCommand handles /commands that don't need agent execution.
func (m Model) handleBuiltinCommand(text string) tea.Cmd {
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	parts := strings.Fields(text)
	cmd := strings.TrimPrefix(parts[0], "/")

	switch cmd {
	case "quit", "exit", "q":
		return func() tea.Msg { return QuitMsg{} }

	case "clear", "cls":
		m.output.Clear()
		return nil

	case "help", "h", "?":
		m.output.Append(helpText(m.mode), "info")
		return nil

	case "mode":
		if len(parts) > 1 {
			return func() tea.Msg { return ModeChangeMsg{Agent: parts[1]} }
		}
		m.output.Append("Usage: /mode <agent>", "warning")
		return nil // Return nil since we handled it via Append
	}

	return nil
}

// View implements tea.Model. Renders the full TUI frame.
// This is called on every update -- Bubble Tea diffs the output.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	if !m.ready {
		return "Initializing..."
	}

	var sections []string

	// 1. Header bar
	header := m.renderHeader()
	sections = append(sections, header)

	// 2. Progress panel (only when agent is active or has phases)
	progressView := m.progress.View()
	if progressView != "" {
		sections = append(sections, progressView)
	}

	// 3. Separator
	sep := DimStyle.Render(strings.Repeat("─", m.width))
	sections = append(sections, sep)

	// 4. Output area (fills remaining space)
	outputHeight := m.outputHeight(len(sections))
	outputView := m.output.View(outputHeight)
	if outputView != "" {
		sections = append(sections, outputView)
	}

	// 5. Input area (pinned to bottom)
	inputView := m.input.View()
	sections = append(sections, inputView)

	return strings.Join(sections, "\n")
}

// renderHeader renders the top status bar.
func (m Model) renderHeader() string {
	title := HeaderStyle.Render("pedrocode")

	// Status indicators
	var statusParts []string
	statusParts = append(statusParts, DimStyle.Render("mode:")+m.mode)
	statusParts = append(statusParts, DimStyle.Render("agent:")+m.agent)

	if m.agentBusy {
		spinner := SpinnerFrames[m.progress.frame%len(SpinnerFrames)]
		statusParts = append(statusParts, WarningStyle.Render(spinner+" working"))
	}

	status := DimStyle.Render(" | ") + strings.Join(statusParts, DimStyle.Render(" | "))

	// Right-align help hint
	helpHint := DimStyle.Render("/help  ctrl+c quit")
	gap := m.width - lipgloss.Width(title+status) - lipgloss.Width(helpHint) - 2
	if gap < 1 {
		gap = 1
	}

	return title + status + strings.Repeat(" ", gap) + helpHint
}

// outputHeight calculates how many lines the output area gets.
func (m Model) outputHeight(usedSections int) int {
	// Each section is roughly 1 line; progress panel can be more.
	progressLines := strings.Count(m.progress.View(), "\n")
	if progressLines > 0 {
		progressLines++ // Account for the view itself
	}

	// Header (1) + progress + separator (1) + input (1) + padding
	overhead := 1 + progressLines + 1 + 1 + 1
	remaining := m.height - overhead
	if remaining < 3 {
		remaining = 3
	}
	return remaining
}

// tickCmd returns a command that sends a TickMsg after the tick interval.
func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// promptString generates the prompt for a given agent.
func promptString(agent string) string {
	return PromptStyle.Render(fmt.Sprintf("pedro:%s> ", agent))
}

// helpText returns the help text for the current mode.
func helpText(mode string) string {
	var b strings.Builder
	b.WriteString("Commands:\n")
	b.WriteString("  /help          Show this help\n")
	b.WriteString("  /mode <agent>  Switch agent\n")
	b.WriteString("  /clear         Clear output\n")
	b.WriteString("  /quit          Exit\n")
	b.WriteString("\n")
	b.WriteString("Keys:\n")
	b.WriteString("  Enter          Submit prompt\n")
	b.WriteString("  Up/Down        History navigation\n")
	b.WriteString("  PgUp/PgDown    Scroll output\n")
	b.WriteString("  Ctrl+C         Quit\n")
	b.WriteString("\n")

	switch mode {
	case "code":
		b.WriteString("Agents: build, debug, review, triage\n")
	case "blog":
		b.WriteString("Agents: blog, writer, editor\n")
	case "podcast":
		b.WriteString("Agents: podcast\n")
	}

	return b.String()
}
