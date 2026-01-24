package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
)

// ProgressOutput handles streaming progress output to the terminal
type ProgressOutput struct {
	tracker *agents.ProgressTracker
	writer  io.Writer
}

// NewProgressOutput creates a new progress output handler
func NewProgressOutput() *ProgressOutput {
	return &ProgressOutput{
		tracker: agents.NewProgressTracker(),
		writer:  os.Stdout,
	}
}

// SetWriter sets the output writer
func (o *ProgressOutput) SetWriter(w io.Writer) {
	o.writer = w
	o.tracker.AddWriter(w)
}

// StartJob initializes progress tracking for a new job
func (o *ProgressOutput) StartJob(phases []string) {
	o.tracker.Reset()

	// Add all phases
	for _, phase := range phases {
		o.tracker.AddPhase(phase)
	}

	// Print initial tree
	o.PrintTree()
}

// UpdatePhase updates a phase's status
func (o *ProgressOutput) UpdatePhase(name string, status agents.PhaseStatus, progress string) {
	o.tracker.UpdatePhase(name, status, progress)
	o.PrintTree()
}

// SetPhaseError sets an error for a phase
func (o *ProgressOutput) SetPhaseError(name string, err error) {
	o.tracker.SetPhaseError(name, err)
	o.PrintTree()
}

// IncrementToolUse increments tool use count for a phase
func (o *ProgressOutput) IncrementToolUse(name string) {
	o.tracker.IncrementToolUse(name)
	o.PrintTree()
}

// AddTokens adds to the token count for a phase
func (o *ProgressOutput) AddTokens(name string, tokens int) {
	o.tracker.AddTokens(name, tokens)
	o.PrintTree()
}

// PrintTree prints the current progress tree
func (o *ProgressOutput) PrintTree() {
	// Clear screen and reprint tree for live updates
	// For now, just print (can enhance with terminal manipulation later)
	o.tracker.PrintTree()
}

// PrintMessage prints a message to the output
func (o *ProgressOutput) PrintMessage(format string, args ...interface{}) {
	fmt.Fprintf(o.writer, format, args...)
}

// PrintError prints an error message
func (o *ProgressOutput) PrintError(format string, args ...interface{}) {
	fmt.Fprintf(o.writer, "❌ "+format, args...)
}

// PrintSuccess prints a success message
func (o *ProgressOutput) PrintSuccess(format string, args ...interface{}) {
	fmt.Fprintf(o.writer, "✅ "+format, args...)
}

// PrintWarning prints a warning message
func (o *ProgressOutput) PrintWarning(format string, args ...interface{}) {
	fmt.Fprintf(o.writer, "⚠️  "+format, args...)
}

// WaitForJobCompletion waits for a job to complete by polling
// This is a fallback mechanism until we have progress callbacks
func (o *ProgressOutput) WaitForJobCompletion(ctx context.Context, jobID string) error {
	// TODO: Implement job polling via JobManager
	// For now, just return
	fmt.Fprintf(o.writer, "⏳ Waiting for job %s to complete...\n", jobID)

	// Simple polling loop (will be replaced with callbacks in Phase 2)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("job timeout after 30 minutes")
		case <-ticker.C:
			// TODO: Check job status via JobManager
			// For now, just log
			fmt.Fprintf(o.writer, ".")
		}
	}
}

// GetTracker returns the underlying progress tracker
func (o *ProgressOutput) GetTracker() *agents.ProgressTracker {
	return o.tracker
}
