package repl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JobStateDir is the directory where job state is persisted
const JobStateDir = "/tmp/pedrocode-jobs"

// PersistedJobState represents the state of a job saved to disk
type PersistedJobState struct {
	ID          string     `json:"id"`
	Agent       string     `json:"agent"`
	Description string     `json:"description"`
	Status      JobStatus  `json:"status"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`

	// Progress information
	LastEvent    string `json:"last_event,omitempty"`
	ToolCalls    int    `json:"tool_calls"`
	LLMCalls     int    `json:"llm_calls"`
	CurrentRound int    `json:"current_round"`

	// Result information
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`

	// Session information
	SessionID string `json:"session_id"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// JobStatePersister handles persisting job state to disk
type JobStatePersister struct {
	baseDir string
}

// NewJobStatePersister creates a new job state persister
func NewJobStatePersister() (*JobStatePersister, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(JobStateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create job state directory: %w", err)
	}

	return &JobStatePersister{
		baseDir: JobStateDir,
	}, nil
}

// SaveJob saves a job's state to disk
func (jsp *JobStatePersister) SaveJob(sessionID string, job *BackgroundJob) error {
	// Create job directory
	jobDir := filepath.Join(jsp.baseDir, job.ID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return fmt.Errorf("failed to create job directory: %w", err)
	}

	// Create persisted state
	state := PersistedJobState{
		ID:           job.ID,
		Agent:        job.Agent,
		Description:  job.Description,
		Status:       job.Status,
		StartTime:    job.StartTime,
		EndTime:      job.EndTime,
		LastEvent:    job.LastEvent,
		ToolCalls:    job.ToolCalls,
		LLMCalls:     job.LLMCalls,
		CurrentRound: job.CurrentRound,
		SessionID:    sessionID,
	}

	// Add result information if available
	if job.Result != nil {
		state.Success = job.Result.Success
		state.Output = job.Result.Output
		state.Error = job.Result.Error
	}
	if job.Error != nil {
		state.Error = job.Error.Error()
	}

	// Write job.json
	jobFile := filepath.Join(jobDir, "job.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job state: %w", err)
	}

	if err := os.WriteFile(jobFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write job state: %w", err)
	}

	return nil
}

// LoadJob loads a job's state from disk
func (jsp *JobStatePersister) LoadJob(jobID string) (*PersistedJobState, error) {
	jobFile := filepath.Join(jsp.baseDir, jobID, "job.json")

	data, err := os.ReadFile(jobFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read job state: %w", err)
	}

	var state PersistedJobState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job state: %w", err)
	}

	return &state, nil
}

// ListIncompleteJobs finds all jobs that were running when pedrocode exited
func (jsp *JobStatePersister) ListIncompleteJobs() ([]*PersistedJobState, error) {
	// Read all job directories
	entries, err := os.ReadDir(jsp.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read job directory: %w", err)
	}

	var incompleteJobs []*PersistedJobState

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		jobID := entry.Name()
		state, err := jsp.LoadJob(jobID)
		if err != nil {
			// Skip jobs we can't read
			continue
		}

		// Check if job was incomplete (running or pending)
		if state.Status == JobStatusRunning || state.Status == JobStatusPending {
			incompleteJobs = append(incompleteJobs, state)
		}
	}

	return incompleteJobs, nil
}

// MarkJobIncomplete marks a job as incomplete (for shutdown cleanup)
func (jsp *JobStatePersister) MarkJobIncomplete(jobID string) error {
	state, err := jsp.LoadJob(jobID)
	if err != nil {
		return err
	}

	// Only update if still running
	if state.Status == JobStatusRunning || state.Status == JobStatusPending {
		state.Status = JobStatusFailed
		state.Error = "Interrupted: pedrocode exited while job was running"
		now := time.Now()
		state.EndTime = &now

		// Save updated state
		jobDir := filepath.Join(jsp.baseDir, jobID)
		jobFile := filepath.Join(jobDir, "job.json")
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(jobFile, data, 0644)
	}

	return nil
}

// CleanupOldJobs removes job directories older than maxAge
func (jsp *JobStatePersister) CleanupOldJobs(maxAge time.Duration) error {
	entries, err := os.ReadDir(jsp.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	now := time.Now()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		jobID := entry.Name()
		state, err := jsp.LoadJob(jobID)
		if err != nil {
			// Skip jobs we can't read
			continue
		}

		// Determine age
		var age time.Duration
		if state.EndTime != nil {
			age = now.Sub(*state.EndTime)
		} else {
			age = now.Sub(state.StartTime)
		}

		// Remove if too old and complete/failed/cancelled
		if age > maxAge && state.Status != JobStatusRunning && state.Status != JobStatusPending {
			jobDir := filepath.Join(jsp.baseDir, jobID)
			if err := os.RemoveAll(jobDir); err != nil {
				// Log but don't fail
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old job %s: %v\n", jobID, err)
			}
		}
	}

	return nil
}

// DeleteJob removes a job's state from disk
func (jsp *JobStatePersister) DeleteJob(jobID string) error {
	jobDir := filepath.Join(jsp.baseDir, jobID)
	return os.RemoveAll(jobDir)
}

// GetJobDir returns the directory path for a job
func (jsp *JobStatePersister) GetJobDir(jobID string) string {
	return filepath.Join(jsp.baseDir, jobID)
}

// CreateResumeIssue creates a GitHub issue for resuming incomplete jobs
func CreateResumeIssue(incompleteJobs []*PersistedJobState) string {
	if len(incompleteJobs) == 0 {
		return ""
	}

	issue := "## Resume Interrupted Jobs\n\n"
	issue += fmt.Sprintf("Found %d incomplete job(s) from previous session:\n\n", len(incompleteJobs))

	for i, job := range incompleteJobs {
		issue += fmt.Sprintf("%d. **Job %s** (%s)\n", i+1, job.ID, job.Agent)
		issue += fmt.Sprintf("   - Description: %s\n", job.Description)
		issue += fmt.Sprintf("   - Started: %s\n", job.StartTime.Format("2006-01-02 15:04:05"))
		issue += fmt.Sprintf("   - Status: %s\n", job.Status)
		if job.LastEvent != "" {
			issue += fmt.Sprintf("   - Last event: %s\n", job.LastEvent)
		}
		issue += fmt.Sprintf("   - Progress: Round %d, %d tool calls, %d LLM calls\n",
			job.CurrentRound, job.ToolCalls, job.LLMCalls)
		issue += "\n"
	}

	issue += "### Options\n\n"
	issue += "1. Resume jobs automatically on startup\n"
	issue += "2. Prompt user to resume specific jobs\n"
	issue += "3. Archive incomplete jobs for later review\n"
	issue += "4. Provide `/resume <job-id>` command\n"

	return issue
}
