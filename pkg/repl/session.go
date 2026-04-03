package repl

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/memory"
	"github.com/soypete/pedrocli/pkg/orchestration"
)

// Session represents a REPL session state
type Session struct {
	mu sync.RWMutex

	ID              string                    // Unique session ID
	Config          *config.Config            // Application config
	Bridge          *cli.CLIBridge            // CLI bridge for tool/agent access
	QueryEngine     orchestration.QueryEngine // Query engine for intent classification and routing
	CurrentAgent    string                    // Current agent mode (build/debug/review/triage/blog/podcast)
	History         []string                  // Command history
	ActiveJobID     string                    // Currently running job ID
	StartTime       time.Time                 // Session start time
	Mode            string                    // Session mode (code/blog/podcast)
	Logger          *Logger                   // Session logger
	DebugMode       bool                      // Debug mode enabled (also keeps logs)
	InteractiveMode bool                      // Interactive mode - ask for approval before writing code
	JobManager      *JobManager               // Background job manager

	MemoryStore   memory.MemoryStore    // Memory store for session persistence
	ResumePacket  *memory.ResumePacket  // Resume packet from previous session (if any)
	DreamerWorker *memory.DreamerWorker // Post-session consolidation worker
}

// NewSession creates a new REPL session
func NewSession(id string, cfg *config.Config, bridge *cli.CLIBridge, mode string, debugMode bool) (*Session, error) {
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
		InteractiveMode: true,
		JobManager:      NewJobManager(),
	}, nil
}

// SetMemoryStore sets the memory store for session persistence
func (s *Session) SetMemoryStore(store memory.MemoryStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MemoryStore = store
}

// GetMemoryStore returns the memory store
func (s *Session) GetMemoryStore() memory.MemoryStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MemoryStore
}

// SetDreamerWorker sets the dreamer worker for post-session consolidation
func (s *Session) SetDreamerWorker(worker *memory.DreamerWorker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DreamerWorker = worker
}

// GetDreamerWorker returns the dreamer worker
func (s *Session) GetDreamerWorker() *memory.DreamerWorker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DreamerWorker
}

// LoadResume loads the resume packet from previous session
func (s *Session) LoadResume(ctx context.Context, repoID string) error {
	store := s.GetMemoryStore()
	if store == nil {
		return nil
	}

	loader := memory.NewResumeLoader(store, s.Config.Project.Workdir)
	resume, err := loader.LoadResume(ctx, repoID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.ResumePacket = resume
	s.mu.Unlock()

	return nil
}

// GetResumePacket returns the current resume packet
func (s *Session) GetResumePacket() *memory.ResumePacket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ResumePacket
}

// Close closes the session, runs dreamer consolidation, and cleans up resources
func (s *Session) Close() error {
	s.mu.RLock()
	dreamer := s.DreamerWorker
	sessionID := s.ID
	s.mu.RUnlock()

	if dreamer != nil && sessionID != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := dreamer.Run(ctx, sessionID); err != nil {
				log.Printf("[Session] Dreamer consolidation failed: %v", err)
			} else {
				log.Printf("[Session] Dreamer consolidation completed for session %s", sessionID)
			}
		}()
	}

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

// SetQueryEngine sets the query engine for intent classification
func (s *Session) SetQueryEngine(qe orchestration.QueryEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.QueryEngine = qe
}

// GetQueryEngine returns the query engine
func (s *Session) GetQueryEngine() orchestration.QueryEngine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.QueryEngine
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
