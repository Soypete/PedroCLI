package agents

import (
	"fmt"
	"io"
	"sync"
)

// PhaseStatus represents the status of a phase
type PhaseStatus string

const (
	PhaseStatusPending    PhaseStatus = "pending"
	PhaseStatusInProgress PhaseStatus = "in_progress"
	PhaseStatusDone       PhaseStatus = "done"
	PhaseStatusFailed     PhaseStatus = "failed"
)

// PhaseProgress represents the progress of a single phase
type PhaseProgress struct {
	Name       string      `json:"name"`
	Status     PhaseStatus `json:"status"`
	ToolUses   int         `json:"tool_uses"`
	TokenCount int         `json:"token_count"`
	Progress   string      `json:"progress,omitempty"` // e.g., "section 3/5"
	Error      string      `json:"error,omitempty"`
}

// ProgressTracker tracks progress across multiple phases
type ProgressTracker struct {
	mu       sync.RWMutex
	phases   map[string]*PhaseProgress
	order    []string // Maintain phase order for display
	writers  []io.Writer
	sseMode  bool
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		phases:  make(map[string]*PhaseProgress),
		order:   []string{},
		writers: []io.Writer{},
	}
}

// AddPhase adds a phase to track
func (pt *ProgressTracker) AddPhase(name string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, exists := pt.phases[name]; !exists {
		pt.phases[name] = &PhaseProgress{
			Name:   name,
			Status: PhaseStatusPending,
		}
		pt.order = append(pt.order, name)
	}
}

// UpdatePhase updates the status and progress of a phase
func (pt *ProgressTracker) UpdatePhase(name string, status PhaseStatus, progress string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if phase, exists := pt.phases[name]; exists {
		phase.Status = status
		phase.Progress = progress
	} else {
		// Add phase if it doesn't exist
		pt.phases[name] = &PhaseProgress{
			Name:     name,
			Status:   status,
			Progress: progress,
		}
		pt.order = append(pt.order, name)
	}

	// Broadcast update
	pt.broadcastUpdate()
}

// SetPhaseError sets an error for a phase and marks it as failed
func (pt *ProgressTracker) SetPhaseError(name string, err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if phase, exists := pt.phases[name]; exists {
		phase.Status = PhaseStatusFailed
		phase.Error = err.Error()
	}

	pt.broadcastUpdate()
}

// IncrementToolUse increments the tool use count for a phase
func (pt *ProgressTracker) IncrementToolUse(name string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if phase, exists := pt.phases[name]; exists {
		phase.ToolUses++
	}

	pt.broadcastUpdate()
}

// AddTokens adds to the token count for a phase
func (pt *ProgressTracker) AddTokens(name string, tokens int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if phase, exists := pt.phases[name]; exists {
		phase.TokenCount += tokens
	}

	pt.broadcastUpdate()
}

// GetPhases returns a snapshot of all phases in order
func (pt *ProgressTracker) GetPhases() []*PhaseProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]*PhaseProgress, 0, len(pt.order))
	for _, name := range pt.order {
		if phase, exists := pt.phases[name]; exists {
			// Create a copy to avoid race conditions
			phaseCopy := *phase
			result = append(result, &phaseCopy)
		}
	}
	return result
}

// GetPhase returns a specific phase
func (pt *ProgressTracker) GetPhase(name string) *PhaseProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if phase, exists := pt.phases[name]; exists {
		phaseCopy := *phase
		return &phaseCopy
	}
	return nil
}

// AddWriter adds an output writer for progress updates
func (pt *ProgressTracker) AddWriter(w io.Writer) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.writers = append(pt.writers, w)
}

// SetSSEMode enables Server-Sent Events mode for web UI
func (pt *ProgressTracker) SetSSEMode(enabled bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.sseMode = enabled
}

// broadcastUpdate sends progress updates to all registered writers
func (pt *ProgressTracker) broadcastUpdate() {
	// Must be called with lock held
	if len(pt.writers) == 0 {
		return
	}

	// Generate output based on mode
	var output string
	if pt.sseMode {
		output = pt.formatSSE()
	} else {
		output = pt.formatCLI()
	}

	// Write to all writers
	for _, w := range pt.writers {
		fmt.Fprint(w, output)
	}
}

// formatCLI formats progress as a CLI tree view (like Claude Code)
func (pt *ProgressTracker) formatCLI() string {
	var output string

	for i, name := range pt.order {
		phase := pt.phases[name]
		isLast := i == len(pt.order)-1

		// Tree characters
		var prefix string
		if isLast {
			prefix = "└─ "
		} else {
			prefix = "├─ "
		}

		// Status icon
		icon := pt.getStatusIcon(phase.Status)

		// Format line
		line := fmt.Sprintf("%s%s %s", prefix, icon, phase.Name)

		// Add stats if available
		if phase.ToolUses > 0 || phase.TokenCount > 0 {
			stats := fmt.Sprintf(" . %d tool uses . %s tokens",
				phase.ToolUses,
				formatTokenCount(phase.TokenCount))
			line += stats
		}

		// Add progress if available
		if phase.Progress != "" {
			line += fmt.Sprintf(" (%s)", phase.Progress)
		}

		// Add error if failed
		if phase.Status == PhaseStatusFailed && phase.Error != "" {
			line += fmt.Sprintf("\n   Error: %s", phase.Error)
		}

		output += line + "\n"

		// Add status line under phase if not last
		if !isLast {
			var childPrefix string
			if isLast {
				childPrefix = "   "
			} else {
				childPrefix = "│  "
			}

			statusText := pt.getStatusText(phase.Status)
			output += fmt.Sprintf("%s└─ %s\n", childPrefix, statusText)
		}
	}

	return output
}

// formatSSE formats progress as Server-Sent Events for web UI
func (pt *ProgressTracker) formatSSE() string {
	// This would be implemented by the HTTP bridge to send actual SSE events
	// For now, just return empty as SSE writing is handled elsewhere
	return ""
}

// getStatusIcon returns an icon for the phase status
func (pt *ProgressTracker) getStatusIcon(status PhaseStatus) string {
	switch status {
	case PhaseStatusPending:
		return "⏳"
	case PhaseStatusInProgress:
		return "▶"
	case PhaseStatusDone:
		return "✓"
	case PhaseStatusFailed:
		return "✗"
	default:
		return "?"
	}
}

// getStatusText returns text description of the status
func (pt *ProgressTracker) getStatusText(status PhaseStatus) string {
	switch status {
	case PhaseStatusPending:
		return "Pending"
	case PhaseStatusInProgress:
		return "In Progress"
	case PhaseStatusDone:
		return "Done"
	case PhaseStatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// formatTokenCount formats token count with k suffix for readability
func formatTokenCount(tokens int) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%d", tokens)
}

// PrintTree prints the current progress tree to stdout
func (pt *ProgressTracker) PrintTree() {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	fmt.Print(pt.formatCLI())
}

// Reset clears all phases
func (pt *ProgressTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.phases = make(map[string]*PhaseProgress)
	pt.order = []string{}
}
