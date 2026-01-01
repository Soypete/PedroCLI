// Package jobs provides job management for PedroCLI agents.
//
// Deprecated: The file-based Manager is deprecated. Use DBManager instead
// for persistent storage across nodes and better debugging capabilities.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/storage"
)

// Status represents job status
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job represents a coding job
type Job struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "build", "debug", "review", "triage"
	Status      Status                 `json:"status"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	WorkDir     string                 `json:"work_dir"`
	ContextDir  string                 `json:"context_dir"`
}

// Manager manages jobs using file-based storage.
//
// Deprecated: Use DBManager for production use.
type Manager struct {
	mu            sync.RWMutex
	jobs          map[string]*Job
	conversations map[string][]storage.ConversationEntry
	stateDir      string
}

// NewManager creates a new file-based job manager.
//
// Deprecated: Use NewDBManager for production use.
func NewManager(stateDir string) (*Manager, error) {
	if stateDir == "" {
		stateDir = "/tmp/pedroceli-jobs"
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	m := &Manager{
		jobs:          make(map[string]*Job),
		conversations: make(map[string][]storage.ConversationEntry),
		stateDir:      stateDir,
	}

	// Load existing jobs
	if err := m.loadJobs(); err != nil {
		return nil, err
	}

	return m, nil
}

// Create creates a new job.
func (m *Manager) Create(ctx context.Context, jobType, description string, input map[string]interface{}) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	job := &Job{
		ID:          fmt.Sprintf("job-%d", now.Unix()),
		Type:        jobType,
		Status:      StatusPending,
		Description: description,
		Input:       input,
		CreatedAt:   now,
		WorkDir:     "", // Set by agent
		ContextDir:  "", // Set by context manager
	}

	m.jobs[job.ID] = job
	m.conversations[job.ID] = []storage.ConversationEntry{}

	// Save to disk
	if err := m.saveJob(job); err != nil {
		return nil, err
	}

	return job, nil
}

// Get retrieves a job by ID.
func (m *Manager) Get(ctx context.Context, id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return job, nil
}

// List returns all jobs.
func (m *Manager) List(ctx context.Context) ([]*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Update updates a job's status and details.
func (m *Manager) Update(ctx context.Context, id string, status Status, output map[string]interface{}, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = status
	if output != nil {
		job.Output = output
	}
	if err != nil {
		job.Error = err.Error()
	}

	now := time.Now()
	if status == StatusRunning && job.StartedAt == nil {
		job.StartedAt = &now
	}
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		job.CompletedAt = &now
	}

	// Save to disk
	return m.saveJob(job)
}

// Cancel cancels a job.
func (m *Manager) Cancel(ctx context.Context, id string) error {
	return m.Update(ctx, id, StatusCancelled, nil, nil)
}

// saveJob saves a job to disk
func (m *Manager) saveJob(job *Job) error {
	filename := filepath.Join(m.stateDir, fmt.Sprintf("%s.json", job.ID))

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// loadJobs loads all jobs from disk
func (m *Manager) loadJobs() error {
	files, err := filepath.Glob(filepath.Join(m.stateDir, "job-*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}

		m.jobs[job.ID] = &job
	}

	return nil
}

// CleanupOldJobs removes completed jobs older than the specified duration.
func (m *Manager) CleanupOldJobs(ctx context.Context, olderThan time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)

	for id, job := range m.jobs {
		if job.Status == StatusCompleted || job.Status == StatusFailed || job.Status == StatusCancelled {
			if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
				// Remove from memory
				delete(m.jobs, id)
				delete(m.conversations, id)

				// Remove from disk
				filename := filepath.Join(m.stateDir, fmt.Sprintf("%s.json", id))
				os.Remove(filename)
			}
		}
	}

	return nil
}

// SetWorkDir sets the working directory for a job.
func (m *Manager) SetWorkDir(ctx context.Context, id string, workDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.WorkDir = workDir
	return m.saveJob(job)
}

// SetContextDir sets the context directory for a job.
func (m *Manager) SetContextDir(ctx context.Context, id string, contextDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	job.ContextDir = contextDir
	return m.saveJob(job)
}

// AppendConversation appends an entry to the job's conversation history.
func (m *Manager) AppendConversation(ctx context.Context, id string, entry storage.ConversationEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.jobs[id]; !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	m.conversations[id] = append(m.conversations[id], entry)
	return nil
}

// GetConversation retrieves the conversation history for a job.
func (m *Manager) GetConversation(ctx context.Context, id string) ([]storage.ConversationEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.jobs[id]; !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return m.conversations[id], nil
}
