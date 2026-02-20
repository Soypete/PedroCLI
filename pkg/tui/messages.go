package tui

import "time"

// --- Bubble Tea message types ---
// These flow through the Update() loop to drive state changes.

// ProgressMsg updates a phase in the progress tree.
type ProgressMsg struct {
	Phase    string
	Status   string // pending, in_progress, done, failed
	Progress string // e.g. "section 3/5"
	ToolUses int
	Tokens   int
	Error    string
}

// ToolCallMsg is sent when an agent invokes a tool.
type ToolCallMsg struct {
	Name string
	Args string
}

// ToolResultMsg is sent when a tool returns.
type ToolResultMsg struct {
	Name    string
	Success bool
	Output  string
}

// LLMResponseMsg is sent when the LLM produces text.
type LLMResponseMsg struct {
	Text string
}

// AgentStartMsg signals that an agent has started working.
type AgentStartMsg struct {
	Agent  string
	Prompt string
	Phases []string
}

// AgentDoneMsg signals that an agent has finished.
type AgentDoneMsg struct {
	Success bool
	Output  string
	Error   string
}

// OutputLineMsg appends a line to the scrolling output area.
type OutputLineMsg struct {
	Text  string
	Style string // "info", "success", "error", "warning", "tool", "llm"
}

// TickMsg drives the spinner animation.
type TickMsg time.Time

// InputSubmitMsg is sent when the user presses Enter.
type InputSubmitMsg struct {
	Text string
}

// ModeChangeMsg switches the current agent.
type ModeChangeMsg struct {
	Agent string
}

// QuitMsg signals the TUI should exit.
type QuitMsg struct{}
