package repl

import (
	"context"
	"fmt"
	"time"
)

// handleBackgroundAsync handles background execution asynchronously
// Jobs run in goroutines and respect the MaxConcurrentJobs limit
// TODO: Currently unused, may be needed for future async features
//
//nolint:unused
func (r *REPL) _handleBackgroundAsync(agent string, prompt string) error {
	// Check if we're at the concurrent job limit
	activeCount := r.session.JobManager.ActiveCount()
	if activeCount >= MaxConcurrentJobs {
		r.output.PrintWarning("⚠️  Already running %d jobs (max: %d)\n", activeCount, MaxConcurrentJobs)
		r.output.PrintMessage("   Use /jobs to see active jobs\n")
		r.output.PrintMessage("   Use /cancel <id> to cancel a job\n")
		return nil
	}

	// Create job ID
	jobID := fmt.Sprintf("job-%d", time.Now().Unix())

	// Create job context with cancellation
	jobCtx, cancel := context.WithCancel(r.ctx)

	// Create background job
	job := &BackgroundJob{
		ID:          jobID,
		Agent:       agent,
		Description: prompt,
		Status:      JobStatusPending,
		StartTime:   time.Now(),
		Cancel:      cancel,
	}

	// Add to job manager
	r.session.JobManager.AddJob(job)

	// Get persister for saving job state
	persister, err := NewJobStatePersister()
	if err != nil {
		r.output.PrintError("Failed to create job persister: %v\n", err)
		return nil
	}

	// Create debug logger if in debug mode
	debugLogger, err := NewDebugLogger(jobID, true, r.session.Config.Debug.Enabled)
	if err != nil {
		r.output.PrintWarning("⚠️  Failed to create debug logger: %v\n", err)
	}
	defer func() {
		if debugLogger != nil {
			debugLogger.Close()
		}
	}()

	// Show Pedro and job notification
	ShowPedro(r.output.writer)
	r.output.PrintSuccess("\n✅ Started background job: %s\n", jobID)
	r.output.PrintMessage("   Agent: %s\n", agent)
	r.output.PrintMessage("   Use /jobs to see progress\n")

	// In debug mode, notify user about log file location
	if r.session.Config.Debug.Enabled && debugLogger != nil {
		logPath := debugLogger.GetLogPath()
		if logPath != "" {
			r.output.PrintMessage("   Debug logs: %s\n", logPath)
			r.output.PrintMessage("   (Use 'tail -f %s' to follow progress)\n", logPath)
		}
	}
	r.output.PrintMessage("\n")

	// Save initial job state
	if err := persister.SaveJob(r.session.ID, job); err != nil {
		r.session.Logger.LogError(fmt.Errorf("failed to save job state: %w", err))
	}

	// Run job in background goroutine
	go func() {
		// Close debug logger when goroutine exits
		if debugLogger != nil {
			defer debugLogger.Close()
		}

		// Update status to running
		r.session.JobManager.UpdateJobStatus(jobID, JobStatusRunning)
		_ = persister.SaveJob(r.session.ID, job)

		// TODO: Redirect stderr to debug logger for this goroutine
		// This requires refactoring agents to accept a debug writer
		// For now, debug output still goes to stderr (blocking prompt)
		// See GitHub issue for proper fix

		// Execute agent
		result, err := r.session.Bridge.ExecuteAgent(jobCtx, agent, prompt)

		// Update job with result
		r.session.JobManager.SetJobResult(jobID, result, err)

		// Update final status
		if err != nil {
			r.session.JobManager.UpdateJobStatus(jobID, JobStatusFailed)
			r.output.PrintMessage("\n[%s] ❌ Failed: %s\n", jobID, err.Error())
			r.session.Logger.LogError(err)
		} else if result != nil && !result.Success {
			r.session.JobManager.UpdateJobStatus(jobID, JobStatusFailed)
			r.output.PrintMessage("\n[%s] ❌ Agent failed: %s\n", jobID, result.Error)
		} else {
			r.session.JobManager.UpdateJobStatus(jobID, JobStatusComplete)
			r.output.PrintMessage("\n[%s] ✅ Complete! %s\n", jobID, result.Output)
			ShowCompletePedro(r.output.writer)
		}

		// Save final job state
		_ = persister.SaveJob(r.session.ID, job)

		// Log completion
		if result != nil {
			r.session.Logger.LogOutput(result.Output)
		}

		// Cleanup based on config
		if !r.session.Config.Debug.KeepTempFiles {
			// If job completed successfully, delete immediately
			if job.Status == JobStatusComplete {
				_ = persister.DeleteJob(jobID)
			}

			// Check storage limits and cleanup if needed
			toDelete := r.session.JobManager.CleanupByStorageLimit(false)
			for _, id := range toDelete {
				_ = persister.DeleteJob(id)
			}
		}
	}()

	return nil
}
