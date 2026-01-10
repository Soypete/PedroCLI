package agents

import (
	"context"
	"time"
)

// AgentOrchestrator defines the common interface for all multi-phase agents.
// All agents (blog, podcast, coding) implement this interface to ensure
// consistent workflow patterns across CLI and Web UI.
type AgentOrchestrator interface {
	// Execute runs the complete workflow
	Execute(ctx context.Context) error

	// GetPhases returns the list of workflow phases
	GetPhases() []Phase

	// GetCurrentPhase returns the currently executing phase
	GetCurrentPhase() string

	// GetProgress returns the progress tracker
	GetProgress() *ProgressTracker

	// GetOutput returns the final output (blog post, podcast, code changes, etc.)
	GetOutput() interface{}
}

// Phase represents a single step in an agent workflow.
// This combines the phased executor pattern with the unified orchestrator pattern.
type Phase struct {
	Name        string                      // Phase identifier (e.g., "analyze", "plan", "implement")
	Description string                      // Human-readable description
	Execute     func(context.Context) error // Direct execution function (for unified agents)
	Required    bool                        // Can this phase be skipped?

	// Fields for phased executor compatibility
	SystemPrompt string                          // Custom system prompt for this phase
	Tools        []string                        // Subset of tools available in this phase (empty = all)
	MaxRounds    int                             // Max inference rounds for this phase (0 = use default)
	ExpectsJSON  bool                            // Allow the phase to produce structured output
	Validator    func(result *PhaseResult) error // Validates the phase output
}

// ToolCallSummary captures what a tool did during a phase
type ToolCallSummary struct {
	ToolName      string   `json:"tool_name"`
	Success       bool     `json:"success"`
	Output        string   `json:"output,omitempty"`
	Error         string   `json:"error,omitempty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
}

// PhaseResult contains the result of executing a phase (for phased executor)
type PhaseResult struct {
	PhaseName     string                 `json:"phase_name"`
	Success       bool                   `json:"success"`
	Output        string                 `json:"output"`         // Full LLM response text
	Data          map[string]interface{} `json:"data,omitempty"` // Structured data (JSON)
	Error         string                 `json:"error,omitempty"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   time.Time              `json:"completed_at"`
	RoundsUsed    int                    `json:"rounds_used"`
	ToolCalls     []ToolCallSummary      `json:"tool_calls,omitempty"`     // Tools called during this phase
	ModifiedFiles []string               `json:"modified_files,omitempty"` // All files modified
}

// ExecutionMode determines how the agent runs
type ExecutionMode int

const (
	ExecutionModeSync  ExecutionMode = iota // CLI - blocking execution
	ExecutionModeAsync                      // Web UI - background job
)

func (m ExecutionMode) String() string {
	switch m {
	case ExecutionModeSync:
		return "sync"
	case ExecutionModeAsync:
		return "async"
	default:
		return "unknown"
	}
}

// Note: PhaseStatus, PhaseProgress, and ProgressTracker are defined in progress.go
// to avoid duplication. This file only defines the AgentOrchestrator interface
// and the unified Phase struct that combines both orchestrator and phased executor patterns.
