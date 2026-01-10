package repl

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/toolformat"
)

// JobStatus represents the status of a background job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusComplete  JobStatus = "complete"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// BackgroundJob represents a job running in the background
type BackgroundJob struct {
	ID          string
	Agent       string
	Description string
	Status      JobStatus
	StartTime   time.Time
	EndTime     *time.Time
	Result      *toolformat.ToolResult
	Error       error
	Cancel      context.CancelFunc

	// Progress tracking
	LastEvent    string
	ToolCalls    int
	LLMCalls     int
	CurrentRound int
}

// JobManager manages background jobs in the REPL
type JobManager struct {
	mu    sync.RWMutex
	jobs  map[string]*BackgroundJob
	order []string // Track order for display
}

// NewJobManager creates a new job manager
func NewJobManager() *JobManager {
	return &JobManager{
		jobs:  make(map[string]*BackgroundJob),
		order: make([]string, 0),
	}
}

// AddJob adds a new job to the manager
func (jm *JobManager) AddJob(job *BackgroundJob) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jm.jobs[job.ID] = job
	jm.order = append(jm.order, job.ID)
}

// GetJob retrieves a job by ID
func (jm *JobManager) GetJob(id string) (*BackgroundJob, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	job, ok := jm.jobs[id]
	return job, ok
}

// ListJobs returns all jobs in order
func (jm *JobManager) ListJobs() []*BackgroundJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	jobs := make([]*BackgroundJob, 0, len(jm.order))
	for _, id := range jm.order {
		if job, ok := jm.jobs[id]; ok {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// ActiveJobs returns only active (running/pending) jobs
func (jm *JobManager) ActiveJobs() []*BackgroundJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	jobs := make([]*BackgroundJob, 0)
	for _, id := range jm.order {
		if job, ok := jm.jobs[id]; ok {
			if job.Status == JobStatusRunning || job.Status == JobStatusPending {
				jobs = append(jobs, job)
			}
		}
	}
	return jobs
}

// UpdateJobStatus updates a job's status
func (jm *JobManager) UpdateJobStatus(id string, status JobStatus) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if job, ok := jm.jobs[id]; ok {
		job.Status = status
		if status == JobStatusComplete || status == JobStatusFailed || status == JobStatusCancelled {
			now := time.Now()
			job.EndTime = &now
		}
	}
}

// UpdateJobProgress updates a job's progress information
func (jm *JobManager) UpdateJobProgress(id string, event string, toolCalls, llmCalls, round int) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if job, ok := jm.jobs[id]; ok {
		job.LastEvent = event
		job.ToolCalls = toolCalls
		job.LLMCalls = llmCalls
		job.CurrentRound = round
	}
}

// SetJobResult sets the result of a completed job
func (jm *JobManager) SetJobResult(id string, result *toolformat.ToolResult, err error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if job, ok := jm.jobs[id]; ok {
		job.Result = result
		job.Error = err
	}
}

// CancelJob cancels a running job
func (jm *JobManager) CancelJob(id string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[id]
	if !ok {
		return fmt.Errorf("job not found: %s", id)
	}

	if job.Status != JobStatusRunning && job.Status != JobStatusPending {
		return fmt.Errorf("job is not running (status: %s)", job.Status)
	}

	if job.Cancel != nil {
		job.Cancel()
	}

	job.Status = JobStatusCancelled
	now := time.Now()
	job.EndTime = &now

	return nil
}

// ActiveCount returns the number of active jobs
func (jm *JobManager) ActiveCount() int {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	count := 0
	for _, job := range jm.jobs {
		if job.Status == JobStatusRunning || job.Status == JobStatusPending {
			count++
		}
	}
	return count
}

// CleanupByStorageLimit removes jobs based on storage limits
// - Completed jobs are removed immediately (unless debug mode)
// - Keep last MaxUnfinishedJobs unfinished jobs
func (jm *JobManager) CleanupByStorageLimit(keepCompleted bool) []string {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	var toDelete []string
	var unfinished []*BackgroundJob

	// Separate completed from unfinished
	for _, id := range jm.order {
		job, ok := jm.jobs[id]
		if !ok {
			continue
		}

		if job.Status == JobStatusComplete {
			// Delete completed jobs unless debug mode
			if !keepCompleted {
				toDelete = append(toDelete, id)
				delete(jm.jobs, id)
			}
		} else {
			// Track unfinished jobs
			unfinished = append(unfinished, job)
		}
	}

	// If too many unfinished jobs, delete oldest
	if len(unfinished) > MaxUnfinishedJobs {
		// Sort by start time (oldest first)
		sort.Slice(unfinished, func(i, j int) bool {
			return unfinished[i].StartTime.Before(unfinished[j].StartTime)
		})

		// Delete excess jobs
		excess := len(unfinished) - MaxUnfinishedJobs
		for i := 0; i < excess; i++ {
			toDelete = append(toDelete, unfinished[i].ID)
			delete(jm.jobs, unfinished[i].ID)
		}
	}

	// Rebuild order list
	newOrder := make([]string, 0)
	for _, id := range jm.order {
		if _, ok := jm.jobs[id]; ok {
			newOrder = append(newOrder, id)
		}
	}
	jm.order = newOrder

	return toDelete
}

// FormatJobList returns a formatted string of all jobs
func (jm *JobManager) FormatJobList() string {
	jobs := jm.ListJobs()
	if len(jobs) == 0 {
		return "No jobs found"
	}

	output := fmt.Sprintf("Jobs (%d total, %d active):\n\n", len(jobs), jm.ActiveCount())

	for i, job := range jobs {
		statusIcon := "⏳"
		switch job.Status {
		case JobStatusRunning:
			statusIcon = "🔄"
		case JobStatusComplete:
			statusIcon = "✅"
		case JobStatusFailed:
			statusIcon = "❌"
		case JobStatusCancelled:
			statusIcon = "🚫"
		}

		elapsed := time.Since(job.StartTime).Round(time.Second)
		if job.EndTime != nil {
			elapsed = job.EndTime.Sub(job.StartTime).Round(time.Second)
		}

		// Show progress for running jobs
		progress := ""
		if job.Status == JobStatusRunning {
			progress = fmt.Sprintf(" - Round %d (%d tools, %d LLM)",
				job.CurrentRound, job.ToolCalls, job.LLMCalls)
		}

		output += fmt.Sprintf("%s %s (%s) - %s%s [%v]\n",
			statusIcon, job.ID, job.Agent, job.Status, progress, elapsed)

		if job.LastEvent != "" && job.Status == JobStatusRunning {
			output += fmt.Sprintf("   └─ %s\n", job.LastEvent)
		}

		if i < len(jobs)-1 {
			output += "\n"
		}
	}

	return output
}

// FormatJobDetails returns detailed information about a specific job
func (jm *JobManager) FormatJobDetails(id string) string {
	job, ok := jm.GetJob(id)
	if !ok {
		return fmt.Sprintf("Job not found: %s", id)
	}

	output := fmt.Sprintf("Job: %s\n", job.ID)
	output += fmt.Sprintf("Agent: %s\n", job.Agent)
	output += fmt.Sprintf("Status: %s\n", job.Status)
	output += fmt.Sprintf("Description: %s\n", job.Description)
	output += fmt.Sprintf("Started: %s\n", job.StartTime.Format("2006-01-02 15:04:05"))

	if job.EndTime != nil {
		output += fmt.Sprintf("Ended: %s\n", job.EndTime.Format("2006-01-02 15:04:05"))
		duration := job.EndTime.Sub(job.StartTime).Round(time.Second)
		output += fmt.Sprintf("Duration: %v\n", duration)
	} else {
		elapsed := time.Since(job.StartTime).Round(time.Second)
		output += fmt.Sprintf("Elapsed: %v\n", elapsed)
	}

	output += fmt.Sprintf("\nProgress:\n")
	output += fmt.Sprintf("  Round: %d\n", job.CurrentRound)
	output += fmt.Sprintf("  Tool calls: %d\n", job.ToolCalls)
	output += fmt.Sprintf("  LLM calls: %d\n", job.LLMCalls)

	if job.LastEvent != "" {
		output += fmt.Sprintf("  Last event: %s\n", job.LastEvent)
	}

	if job.Result != nil {
		output += fmt.Sprintf("\nResult:\n")
		output += fmt.Sprintf("  Success: %v\n", job.Result.Success)
		if job.Result.Output != "" {
			output += fmt.Sprintf("  Output: %s\n", job.Result.Output)
		}
		if job.Result.Error != "" {
			output += fmt.Sprintf("  Error: %s\n", job.Result.Error)
		}
	}

	if job.Error != nil {
		output += fmt.Sprintf("\nError: %s\n", job.Error.Error())
	}

	return output
}
