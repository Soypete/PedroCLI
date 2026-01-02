package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/storage"
)

// DBManager implements JobManager using PostgreSQL for persistence
// with an in-memory cache for running jobs.
type DBManager struct {
	store *storage.JobStore

	// In-memory cache for running jobs only (cleared on completion)
	mu           sync.RWMutex
	runningJobs  map[string]*Job
	migrated     bool
	migratedFrom string
}

// NewDBManager creates a new database-backed job manager.
func NewDBManager(store *storage.JobStore) *DBManager {
	return &DBManager{
		store:       store,
		runningJobs: make(map[string]*Job),
	}
}

// Create creates a new job with a UUID and stores it in the database.
func (m *DBManager) Create(ctx context.Context, jobType, description string, input map[string]interface{}) (*Job, error) {
	now := time.Now()
	id := uuid.New().String()

	// Marshal input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Create the job in the database
	dbJob := &storage.Job{
		ID:           id,
		JobType:      storage.JobType(jobType),
		Status:       storage.JobStatusPending,
		Description:  description,
		InputPayload: inputJSON,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := m.store.Create(ctx, dbJob); err != nil {
		return nil, fmt.Errorf("failed to create job in database: %w", err)
	}

	// Create our local Job struct
	job := &Job{
		ID:          id,
		Type:        jobType,
		Status:      StatusPending,
		Description: description,
		Input:       input,
		CreatedAt:   now,
	}

	// Add to running jobs cache (will be removed on completion)
	m.mu.Lock()
	m.runningJobs[id] = job
	m.mu.Unlock()

	return job, nil
}

// Get retrieves a job by ID from the database.
func (m *DBManager) Get(ctx context.Context, id string) (*Job, error) {
	dbJob, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return convertFromDBJob(dbJob), nil
}

// List returns all jobs from the database.
func (m *DBManager) List(ctx context.Context) ([]*Job, error) {
	dbJobs, err := m.store.List(ctx, storage.ListJobsOptions{})
	if err != nil {
		return nil, err
	}

	jobs := make([]*Job, len(dbJobs))
	for i, dbJob := range dbJobs {
		jobs[i] = convertFromDBJob(dbJob)
	}

	return jobs, nil
}

// Update updates a job's status, output, and error in the database.
func (m *DBManager) Update(ctx context.Context, id string, status Status, output map[string]interface{}, err error) error {
	// Get current job from database
	dbJob, getErr := m.store.Get(ctx, id)
	if getErr != nil {
		return getErr
	}

	// Update status
	dbJob.Status = storage.JobStatus(status)

	// Update output if provided
	if output != nil {
		outputJSON, marshalErr := json.Marshal(output)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal output: %w", marshalErr)
		}
		dbJob.OutputPayload = outputJSON
	}

	// Update error if provided
	if err != nil {
		dbJob.ErrorMessage = err.Error()
	}

	// Update timestamps
	now := time.Now()
	if status == StatusRunning && dbJob.StartedAt == nil {
		dbJob.StartedAt = &now
	}
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		dbJob.CompletedAt = &now
	}

	// Save to database
	if updateErr := m.store.Update(ctx, dbJob); updateErr != nil {
		return updateErr
	}

	// Remove from running jobs cache if completed
	if status == StatusCompleted || status == StatusFailed || status == StatusCancelled {
		m.mu.Lock()
		delete(m.runningJobs, id)
		m.mu.Unlock()
	}

	return nil
}

// Cancel cancels a job.
func (m *DBManager) Cancel(ctx context.Context, id string) error {
	return m.Update(ctx, id, StatusCancelled, nil, nil)
}

// CleanupOldJobs removes completed/failed/cancelled jobs older than the specified duration.
func (m *DBManager) CleanupOldJobs(ctx context.Context, olderThan time.Duration) error {
	deleted, err := m.store.CleanupOldJobs(ctx, olderThan)
	if err != nil {
		return err
	}

	if deleted > 0 {
		log.Printf("Cleaned up %d old jobs", deleted)
	}

	return nil
}

// SetWorkDir sets the working directory for a job.
func (m *DBManager) SetWorkDir(ctx context.Context, id string, workDir string) error {
	dbJob, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}

	dbJob.WorkDir = workDir
	return m.store.Update(ctx, dbJob)
}

// SetContextDir sets the context directory for a job.
func (m *DBManager) SetContextDir(ctx context.Context, id string, contextDir string) error {
	dbJob, err := m.store.Get(ctx, id)
	if err != nil {
		return err
	}

	dbJob.ContextDir = contextDir
	return m.store.Update(ctx, dbJob)
}

// AppendConversation appends an entry to the job's conversation history.
func (m *DBManager) AppendConversation(ctx context.Context, id string, entry storage.ConversationEntry) error {
	return m.store.AppendConversation(ctx, id, entry)
}

// GetConversation retrieves the conversation history for a job.
func (m *DBManager) GetConversation(ctx context.Context, id string) ([]storage.ConversationEntry, error) {
	return m.store.GetConversation(ctx, id)
}

// MigrateFromFiles migrates existing JSON files from the file-based manager to the database.
// Returns the number of jobs migrated and any error encountered.
func (m *DBManager) MigrateFromFiles(ctx context.Context, stateDir string) (int, error) {
	if m.migrated {
		return 0, nil
	}

	if stateDir == "" {
		stateDir = "/tmp/pedrocli-jobs"
	}

	// Check if directory exists
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		m.migrated = true
		return 0, nil
	}

	// Find all job files
	files, err := filepath.Glob(filepath.Join(stateDir, "job-*.json"))
	if err != nil {
		return 0, fmt.Errorf("failed to list job files: %w", err)
	}

	if len(files) == 0 {
		m.migrated = true
		return 0, nil
	}

	log.Printf("Migrating %d jobs from %s to database", len(files), stateDir)

	migrated := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Warning: failed to read job file %s: %v", file, err)
			continue
		}

		var oldJob Job
		if err := json.Unmarshal(data, &oldJob); err != nil {
			log.Printf("Warning: failed to parse job file %s: %v", file, err)
			continue
		}

		// Convert to database job
		inputJSON, _ := json.Marshal(oldJob.Input)
		outputJSON, _ := json.Marshal(oldJob.Output)

		dbJob := &storage.Job{
			ID:            uuid.New().String(), // Generate new UUID
			JobType:       storage.JobType(oldJob.Type),
			Status:        storage.JobStatus(oldJob.Status),
			Description:   oldJob.Description,
			InputPayload:  inputJSON,
			OutputPayload: outputJSON,
			ErrorMessage:  oldJob.Error,
			WorkDir:       oldJob.WorkDir,
			ContextDir:    oldJob.ContextDir,
			StartedAt:     oldJob.StartedAt,
			CompletedAt:   oldJob.CompletedAt,
			CreatedAt:     oldJob.CreatedAt,
			UpdatedAt:     time.Now(),
		}

		if err := m.store.Create(ctx, dbJob); err != nil {
			log.Printf("Warning: failed to migrate job %s: %v", oldJob.ID, err)
			continue
		}

		migrated++
		log.Printf("Migrated job %s -> %s", oldJob.ID, dbJob.ID)
	}

	m.migrated = true
	m.migratedFrom = stateDir
	log.Printf("Migration complete: %d/%d jobs migrated", migrated, len(files))

	return migrated, nil
}

// CleanupMigratedFiles removes the old job files after successful migration.
// Should only be called after verifying the migration was successful.
func (m *DBManager) CleanupMigratedFiles() error {
	if !m.migrated || m.migratedFrom == "" {
		return nil
	}

	files, err := filepath.Glob(filepath.Join(m.migratedFrom, "job-*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			log.Printf("Warning: failed to remove migrated file %s: %v", file, err)
		}
	}

	log.Printf("Cleaned up %d migrated job files from %s", len(files), m.migratedFrom)
	return nil
}

// convertFromDBJob converts a storage.Job to a jobs.Job.
func convertFromDBJob(dbJob *storage.Job) *Job {
	job := &Job{
		ID:          dbJob.ID,
		Type:        string(dbJob.JobType),
		Status:      Status(dbJob.Status),
		Description: dbJob.Description,
		Error:       dbJob.ErrorMessage,
		WorkDir:     dbJob.WorkDir,
		ContextDir:  dbJob.ContextDir,
		CreatedAt:   dbJob.CreatedAt,
		StartedAt:   dbJob.StartedAt,
		CompletedAt: dbJob.CompletedAt,
	}

	// Unmarshal input payload
	if len(dbJob.InputPayload) > 0 {
		json.Unmarshal(dbJob.InputPayload, &job.Input)
	}

	// Unmarshal output payload
	if len(dbJob.OutputPayload) > 0 {
		json.Unmarshal(dbJob.OutputPayload, &job.Output)
	}

	return job
}

// Verify that DBManager implements JobManager (compile-time check).
var _ JobManager = (*DBManager)(nil)
