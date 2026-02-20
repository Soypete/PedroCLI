package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/soypete/pedrocli/pkg/agents"
)

// AgentAdapter bridges the existing agent/progress system into Bubble Tea messages.
// It implements an io.Writer-like interface that the ProgressTracker can write to,
// converting progress updates into tea.Msg values sent via a program reference.
type AgentAdapter struct {
	program *tea.Program
}

// NewAgentAdapter creates an adapter attached to a running Bubble Tea program.
func NewAgentAdapter(p *tea.Program) *AgentAdapter {
	return &AgentAdapter{program: p}
}

// Send dispatches a message into the Bubble Tea event loop.
func (a *AgentAdapter) Send(msg tea.Msg) {
	if a.program != nil {
		a.program.Send(msg)
	}
}

// SendOutput sends a styled output line to the TUI.
func (a *AgentAdapter) SendOutput(text, style string) {
	a.Send(OutputLineMsg{Text: text, Style: style})
}

// OnPhaseUpdate converts a ProgressTracker phase update to a ProgressMsg.
func (a *AgentAdapter) OnPhaseUpdate(phase *agents.PhaseProgress) {
	status := string(phase.Status)
	a.Send(ProgressMsg{
		Phase:    phase.Name,
		Status:   status,
		ToolUses: phase.ToolUses,
		Tokens:   phase.TokenCount,
		Progress: phase.Progress,
		Error:    phase.Error,
	})
}

// OnToolCall sends a tool call event to the TUI.
func (a *AgentAdapter) OnToolCall(name string, args string) {
	a.Send(ToolCallMsg{Name: name, Args: args})
}

// OnToolResult sends a tool result event to the TUI.
func (a *AgentAdapter) OnToolResult(name string, success bool, output string) {
	a.Send(ToolResultMsg{Name: name, Success: success, Output: output})
}

// OnLLMResponse sends an LLM response event to the TUI.
func (a *AgentAdapter) OnLLMResponse(text string) {
	a.Send(LLMResponseMsg{Text: text})
}

// OnAgentStart signals the start of an agent run.
func (a *AgentAdapter) OnAgentStart(agent, prompt string, phases []string) {
	a.Send(AgentStartMsg{Agent: agent, Prompt: prompt, Phases: phases})
}

// OnAgentDone signals agent completion.
func (a *AgentAdapter) OnAgentDone(success bool, output, errMsg string) {
	a.Send(AgentDoneMsg{Success: success, Output: output, Error: errMsg})
}

// ProgressWriter implements io.Writer so it can be used with ProgressTracker.AddWriter().
// It converts written text into OutputLineMsg messages.
type ProgressWriter struct {
	adapter *AgentAdapter
	style   string
}

// NewProgressWriter creates a writer that forwards to the TUI.
func NewProgressWriter(adapter *AgentAdapter, style string) *ProgressWriter {
	return &ProgressWriter{adapter: adapter, style: style}
}

// Write implements io.Writer.
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	pw.adapter.SendOutput(string(p), pw.style)
	return len(p), nil
}

// RunAgent executes an agent in a background goroutine and streams events to the TUI.
// This is the main integration point between the old agent system and the new TUI.
func RunAgent(ctx context.Context, adapter *AgentAdapter, agentName, prompt string, executeFn func(ctx context.Context, agent string, prompt string) (bool, string, error)) tea.Cmd {
	return func() tea.Msg {
		// Determine default phases based on agent type
		phases := defaultPhases(agentName)
		adapter.OnAgentStart(agentName, prompt, phases)

		// Mark first phase as in progress
		if len(phases) > 0 {
			adapter.Send(ProgressMsg{Phase: phases[0], Status: "in_progress"})
		}

		// Execute the agent
		success, output, err := executeFn(ctx, agentName, prompt)

		// Mark all phases as done or failed
		finalStatus := "done"
		if !success || err != nil {
			finalStatus = "failed"
		}
		for _, phase := range phases {
			adapter.Send(ProgressMsg{Phase: phase, Status: finalStatus})
		}

		// Return completion message
		if err != nil {
			return AgentDoneMsg{Success: false, Error: err.Error()}
		}
		return AgentDoneMsg{Success: success, Output: output}
	}
}

// defaultPhases returns reasonable default phase names for each agent type.
func defaultPhases(agent string) []string {
	switch agent {
	case "build":
		return []string{"Analyze", "Plan", "Implement", "Test", "Review"}
	case "debug":
		return []string{"Diagnose", "Locate", "Fix", "Verify"}
	case "review":
		return []string{"Read", "Analyze", "Report"}
	case "triage":
		return []string{"Gather Info", "Diagnose", "Summarize"}
	case "blog":
		return []string{"Research", "Outline", "Draft", "Edit", "Publish"}
	case "podcast":
		return []string{"Research", "Outline", "Script"}
	default:
		return []string{fmt.Sprintf("Execute %s", agent)}
	}
}
