package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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

// Manager manages jobs
type Manager struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	stateDir string
}

// NewManager creates a new job manager
func NewManager(stateDir string) (*Manager, error) {
	if stateDir == "" {
		stateDir = "/tmp/pedrocli-jobs"
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	m := &Manager{
		jobs:     make(map[string]*Job),
		stateDir: stateDir,
	}

	// Load existing jobs
	if err := m.loadJobs(); err != nil {
		return nil, err
	}

	return m, nil
}

// Create creates a new job
func (m *Manager) Create(jobType, description string, input map[string]interface{}) (*Job, error) {
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

	// Save to disk
	if err := m.saveJob(job); err != nil {
		return nil, err
	}

	return job, nil
}

// Get retrieves a job by ID
func (m *Manager) Get(id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return job, nil
}

// List returns all jobs
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}

	return jobs
}

// Update updates a job's status and details
func (m *Manager) Update(id string, status Status, output map[string]interface{}, err error) error {
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

// Cancel cancels a job
func (m *Manager) Cancel(id string) error {
	return m.Update(id, StatusCancelled, nil, nil)
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

// CleanupOldJobs removes completed jobs older than the specified duration
func (m *Manager) CleanupOldJobs(olderThan time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)

	for id, job := range m.jobs {
		if job.Status == StatusCompleted || job.Status == StatusFailed || job.Status == StatusCancelled {
			if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
				// Remove from memory
				delete(m.jobs, id)

				// Remove from disk
				filename := filepath.Join(m.stateDir, fmt.Sprintf("%s.json", id))
				_ = os.Remove(filename) // Ignore error on cleanup
			}
		}
	}

	return nil
}
