package tools

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/jobs"
)

// GetJobStatusTool implements get_job_status
type GetJobStatusTool struct {
	jobManager *jobs.Manager
}

// NewGetJobStatusTool creates a new get job status tool
func NewGetJobStatusTool(jobMgr *jobs.Manager) *GetJobStatusTool {
	return &GetJobStatusTool{
		jobManager: jobMgr,
	}
}

func (t *GetJobStatusTool) Name() string {
	return "get_job_status"
}

func (t *GetJobStatusTool) Description() string {
	return "Get the status of a running or completed job"
}

func (t *GetJobStatusTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	jobID, ok := args["job_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing 'job_id' parameter",
		}, nil
	}

	job, err := t.jobManager.Get(jobID)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("Job not found: %s", jobID),
		}, nil
	}

	// Format status message
	statusMsg := fmt.Sprintf("Job %s (%s):\nStatus: %s\nDescription: %s",
		job.ID, job.Type, job.Status, job.Description)

	if job.Error != "" {
		statusMsg += fmt.Sprintf("\nError: %s", job.Error)
	}

	if job.Output != nil {
		statusMsg += fmt.Sprintf("\nOutput: %v", job.Output)
	}

	return &Result{
		Success: true,
		Output:  statusMsg,
	}, nil
}

// ListJobsTool implements list_jobs
type ListJobsTool struct {
	jobManager *jobs.Manager
}

// NewListJobsTool creates a new list jobs tool
func NewListJobsTool(jobMgr *jobs.Manager) *ListJobsTool {
	return &ListJobsTool{
		jobManager: jobMgr,
	}
}

func (t *ListJobsTool) Name() string {
	return "list_jobs"
}

func (t *ListJobsTool) Description() string {
	return "List all jobs"
}

func (t *ListJobsTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	jobsList := t.jobManager.List()

	if len(jobsList) == 0 {
		return &Result{
			Success: true,
			Output:  "No jobs found",
		}, nil
	}

	output := fmt.Sprintf("Found %d jobs:\n\n", len(jobsList))
	for _, job := range jobsList {
		output += fmt.Sprintf("- %s (%s): %s - %s\n",
			job.ID, job.Type, job.Status, job.Description)
	}

	return &Result{
		Success: true,
		Output:  output,
	}, nil
}

// CancelJobTool implements cancel_job
type CancelJobTool struct {
	jobManager *jobs.Manager
}

// NewCancelJobTool creates a new cancel job tool
func NewCancelJobTool(jobMgr *jobs.Manager) *CancelJobTool {
	return &CancelJobTool{
		jobManager: jobMgr,
	}
}

func (t *CancelJobTool) Name() string {
	return "cancel_job"
}

func (t *CancelJobTool) Description() string {
	return "Cancel a running job"
}

func (t *CancelJobTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	jobID, ok := args["job_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "missing 'job_id' parameter",
		}, nil
	}

	err := t.jobManager.Cancel(jobID)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("Failed to cancel job: %v", err),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Job %s cancelled successfully", jobID),
	}, nil
}
