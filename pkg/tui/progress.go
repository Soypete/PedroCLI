package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Phase represents a single tracked phase in the progress tree.
type Phase struct {
	Name     string
	Status   string // pending, in_progress, done, failed
	ToolUses int
	Tokens   int
	Progress string
	Error    string
}

// ProgressPanel renders a live-updating tree view of agent phases.
type ProgressPanel struct {
	phases    []Phase
	width     int
	frame     int
	startTime time.Time
	agent     string
	prompt    string
}

// NewProgressPanel creates a new progress panel.
func NewProgressPanel() *ProgressPanel {
	return &ProgressPanel{
		phases:    []Phase{},
		startTime: time.Now(),
	}
}

// SetWidth sets the panel width for rendering.
func (p *ProgressPanel) SetWidth(w int) {
	p.width = w
}

// StartAgent resets the panel for a new agent run.
func (p *ProgressPanel) StartAgent(agent, prompt string, phaseNames []string) {
	p.agent = agent
	p.prompt = prompt
	p.startTime = time.Now()
	p.phases = make([]Phase, len(phaseNames))
	for i, name := range phaseNames {
		p.phases[i] = Phase{Name: name, Status: "pending"}
	}
}

// UpdatePhase updates a phase by name.
func (p *ProgressPanel) UpdatePhase(msg ProgressMsg) {
	for i := range p.phases {
		if p.phases[i].Name == msg.Phase {
			if msg.Status != "" {
				p.phases[i].Status = msg.Status
			}
			if msg.Progress != "" {
				p.phases[i].Progress = msg.Progress
			}
			if msg.ToolUses > 0 {
				p.phases[i].ToolUses = msg.ToolUses
			}
			if msg.Tokens > 0 {
				p.phases[i].Tokens = msg.Tokens
			}
			if msg.Error != "" {
				p.phases[i].Error = msg.Error
			}
			return
		}
	}
	// Phase not found - add it dynamically
	p.phases = append(p.phases, Phase{
		Name:     msg.Phase,
		Status:   msg.Status,
		ToolUses: msg.ToolUses,
		Tokens:   msg.Tokens,
		Progress: msg.Progress,
		Error:    msg.Error,
	})
}

// Tick advances the spinner frame.
func (p *ProgressPanel) Tick() {
	p.frame++
}

// IsActive returns true if any phase is in progress.
func (p *ProgressPanel) IsActive() bool {
	for _, ph := range p.phases {
		if ph.Status == "in_progress" {
			return true
		}
	}
	return false
}

// View renders the progress panel.
func (p *ProgressPanel) View() string {
	if len(p.phases) == 0 {
		return ""
	}

	// Build the phase tree (right side)
	tree := p.renderTree()

	// Build the Pedro animation (left side)
	pedro := p.renderPedro()

	// Join Pedro and tree side-by-side using Lip Gloss
	combined := lipgloss.JoinHorizontal(lipgloss.Top, pedro, "  ", tree)

	return combined + "\n"
}

// renderTree builds the text progress tree.
func (p *ProgressPanel) renderTree() string {
	var b strings.Builder

	// Header line: agent name + elapsed time
	elapsed := time.Since(p.startTime).Round(time.Second)
	header := fmt.Sprintf(" %s agent", p.agent)
	timer := DimStyle.Render(fmt.Sprintf("%v", elapsed))
	b.WriteString(HeaderStyle.Render(header) + "  " + timer + "\n")

	// Prompt summary (truncated)
	if p.prompt != "" {
		summary := p.prompt
		maxLen := p.width - 20 // account for Pedro width
		if maxLen < 20 {
			maxLen = 40
		}
		if len(summary) > maxLen {
			summary = summary[:maxLen-3] + "..."
		}
		b.WriteString(DimStyle.Render("  "+summary) + "\n")
	}

	b.WriteString("\n")

	// Phase tree
	for i, ph := range p.phases {
		isLast := i == len(p.phases)-1

		// Tree connector
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		// Status icon with spinner for in-progress
		icon := p.statusIcon(ph.Status)

		// Phase name
		nameStyle := lipgloss.NewStyle()
		switch ph.Status {
		case "done":
			nameStyle = nameStyle.Foreground(Theme.Success)
		case "failed":
			nameStyle = nameStyle.Foreground(Theme.Error)
		case "in_progress":
			nameStyle = nameStyle.Bold(true)
		default:
			nameStyle = nameStyle.Foreground(Theme.Muted)
		}

		line := fmt.Sprintf("  %s %s %s", DimStyle.Render(connector), icon, nameStyle.Render(ph.Name))

		// Stats
		var stats []string
		if ph.ToolUses > 0 {
			stats = append(stats, fmt.Sprintf("%d tools", ph.ToolUses))
		}
		if ph.Tokens > 0 {
			stats = append(stats, formatTokens(ph.Tokens))
		}
		if ph.Progress != "" {
			stats = append(stats, ph.Progress)
		}
		if len(stats) > 0 {
			line += DimStyle.Render("  " + strings.Join(stats, " . "))
		}

		b.WriteString(line + "\n")

		// Error detail under failed phases
		if ph.Status == "failed" && ph.Error != "" {
			childPrefix := "  │  "
			if isLast {
				childPrefix = "     "
			}
			b.WriteString(childPrefix + ErrorStyle.Render("Error: "+ph.Error) + "\n")
		}
	}

	return b.String()
}

// renderPedro returns the current Pedro animation frame.
func (p *ProgressPanel) renderPedro() string {
	var frame string

	if p.IsActive() {
		// Dancing Pedro while agent is working
		idx := (p.frame / 2) % len(PedroFrames) // slower: advance every 2 ticks
		frame = PedroFrames[idx]
	} else if p.allDone() {
		// Celebration pose when finished
		frame = PedroDoneFrame
	} else {
		// Idle blink
		idx := (p.frame / 4) % len(PedroIdleFrames) // very slow blink
		frame = PedroIdleFrames[idx]
	}

	return PedroStyle.Render(frame)
}

// allDone returns true if all phases are done (none pending or in_progress).
func (p *ProgressPanel) allDone() bool {
	if len(p.phases) == 0 {
		return false
	}
	for _, ph := range p.phases {
		if ph.Status == "pending" || ph.Status == "in_progress" {
			return false
		}
	}
	return true
}

// statusIcon returns the icon for a given status, with animation for in_progress.
func (p *ProgressPanel) statusIcon(status string) string {
	switch status {
	case "pending":
		return DimStyle.Render("○")
	case "in_progress":
		frame := SpinnerFrames[p.frame%len(SpinnerFrames)]
		return WarningStyle.Render(frame)
	case "done":
		return SuccessStyle.Render("●")
	case "failed":
		return ErrorStyle.Render("✗")
	default:
		return DimStyle.Render("?")
	}
}

// formatTokens formats a token count with k suffix.
func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk tok", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d tok", n)
}
