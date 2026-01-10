package repl

import (
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/config"
)

// Session represents a REPL session state
type Session struct {
	mu sync.RWMutex

	ID              string         // Unique session ID
	Config          *config.Config // Application config
	Bridge          *cli.CLIBridge // CLI bridge for tool/agent access
	CurrentAgent    string         // Current agent mode (build/debug/review/triage/blog/podcast)
	History         []string       // Command history
	ActiveJobID     string         // Currently running job ID
	StartTime       time.Time      // Session start time
	Mode            string         // Session mode (code/blog/podcast)
	Logger          *Logger        // Session logger
	DebugMode       bool           // Debug mode enabled (also keeps logs)
	InteractiveMode bool           // Interactive mode - ask for approval before writing code
	JobManager      *JobManager    // Background job manager
}

// NewSession creates a new REPL session
func NewSession(id string, cfg *config.Config, bridge *cli.CLIBridge, mode string, debugMode bool) (*Session, error) {
	// Create logger (debug mode also keeps logs)
	logger, err := NewLogger(id, debugMode, debugMode)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:              id,
		Config:          cfg,
		Bridge:          bridge,
		CurrentAgent:    getDefaultAgentForMode(mode),
		History:         []string{},
		ActiveJobID:     "",
		StartTime:       time.Now(),
		Mode:            mode,
		Logger:          logger,
		DebugMode:       debugMode,
		InteractiveMode: true,            // DEFAULT: Interactive mode on
		JobManager:      NewJobManager(), // Background job manager
	}, nil
}

// Close closes the session and cleans up resources
func (s *Session) Close() error {
	if s.Logger != nil {
		return s.Logger.Close()
	}
	return nil
}

// AddToHistory adds a command to the history
func (s *Session) AddToHistory(cmd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, cmd)
}

// GetHistory returns the command history
func (s *Session) GetHistory() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.History...)
}

// SetCurrentAgent sets the current agent
func (s *Session) SetCurrentAgent(agent string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentAgent = agent
}

// GetCurrentAgent returns the current agent
func (s *Session) GetCurrentAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentAgent
}

// SetActiveJob sets the active job ID
func (s *Session) SetActiveJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveJobID = jobID
}

// GetActiveJob returns the active job ID
func (s *Session) GetActiveJob() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ActiveJobID
}

// SetInteractiveMode sets interactive mode
func (s *Session) SetInteractiveMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.InteractiveMode = enabled
}

// IsInteractive returns whether interactive mode is enabled
func (s *Session) IsInteractive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.InteractiveMode
}

// getDefaultAgentForMode returns the default agent for a given mode
func getDefaultAgentForMode(mode string) string {
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
