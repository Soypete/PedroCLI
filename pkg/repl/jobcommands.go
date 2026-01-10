package repl

import (
	"fmt"
	"strings"
)

// printJobs prints job status
func (r *REPL) printJobs(cmd *Command) {
	// If no args, list all jobs
	if len(cmd.Args) == 0 {
		output := r.session.JobManager.FormatJobList()
		r.output.PrintMessage("\n%s\n", output)
		return
	}

	// Otherwise show job details
	jobID, ok := cmd.Args["arg0"].(string)
	if !ok {
		r.output.PrintError("Invalid job ID\n")
		return
	}

	output := r.session.JobManager.FormatJobDetails(jobID)
	r.output.PrintMessage("\n%s\n", output)
}

// cancelJob cancels a running job
func (r *REPL) cancelJob(cmd *Command) error {
	if len(cmd.Args) == 0 {
		r.output.PrintError("Usage: /cancel <job-id>\n")
		r.output.PrintMessage("Use /jobs to see active jobs\n")
		return nil
	}

	jobID, ok := cmd.Args["arg0"].(string)
	if !ok {
		return fmt.Errorf("invalid job ID")
	}

	if err := r.session.JobManager.CancelJob(jobID); err != nil {
		r.output.PrintError("Failed to cancel job: %v\n", err)
		return nil
	}

	r.output.PrintSuccess("🚫 Cancelled job %s\n", jobID)
	return nil
}

// getActiveJobsIndicator returns a string showing active job count for the prompt
// TODO: Currently unused, reserved for future prompt customization
//
//nolint:unused
func (r *REPL) _getActiveJobsIndicator() string {
	count := r.session.JobManager.ActiveCount()
	if count == 0 {
		return ""
	}
	if count == 1 {
		return " [1 job]"
	}
	return fmt.Sprintf(" [%d jobs]", count)
}

// CheckForIncompleteJobs checks for incomplete jobs from previous sessions
func CheckForIncompleteJobs(persister *JobStatePersister) {
	incompleteJobs, err := persister.ListIncompleteJobs()
	if err != nil || len(incompleteJobs) == 0 {
		return
	}

	fmt.Printf("\n⚠️  Found %d incomplete job(s) from previous session:\n\n", len(incompleteJobs))

	for i, job := range incompleteJobs {
		fmt.Printf("%d. %s (%s) - %s\n", i+1, job.ID, job.Agent, job.Status)
		fmt.Printf("   Description: %s\n", job.Description)
		fmt.Printf("   Started: %s\n", job.StartTime.Format("2006-01-02 15:04:05"))
		if job.LastEvent != "" {
			// Truncate long events
			event := job.LastEvent
			if len(event) > 60 {
				event = event[:57] + "..."
			}
			fmt.Printf("   Last: %s\n", event)
		}
		fmt.Printf("   Progress: Round %d, %d tool calls\n", job.CurrentRound, job.ToolCalls)
		fmt.Println()
	}

	fmt.Println("These jobs were interrupted when pedrocode exited.")
	fmt.Println("Job files are saved in: /tmp/pedrocode-jobs/")
	fmt.Println()
	fmt.Print("Options: [v] View details  [d] Delete all  [k] Keep  [Enter to continue]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "v":
		// Show details
		for _, job := range incompleteJobs {
			fmt.Printf("\n--- %s ---\n", job.ID)
			fmt.Printf("Agent: %s\n", job.Agent)
			fmt.Printf("Description: %s\n", job.Description)
			fmt.Printf("Status: %s\n", job.Status)
			fmt.Printf("Started: %s\n", job.StartTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("Progress: Round %d, %d tools, %d LLM\n", job.CurrentRound, job.ToolCalls, job.LLMCalls)
			if job.LastEvent != "" {
				fmt.Printf("Last event: %s\n", job.LastEvent)
			}
			if job.Error != "" {
				fmt.Printf("Error: %s\n", job.Error)
			}
			fmt.Printf("Location: %s\n", persister.GetJobDir(job.ID))
		}
		fmt.Println()

	case "d":
		// Delete all
		for _, job := range incompleteJobs {
			if err := persister.DeleteJob(job.ID); err != nil {
				fmt.Printf("Warning: failed to delete %s: %v\n", job.ID, err)
			}
		}
		fmt.Printf("✅ Deleted %d incomplete job(s)\n\n", len(incompleteJobs))

	case "k", "":
		// Keep - do nothing
		fmt.Println("Keeping incomplete jobs. Use /jobs to see them.")
		fmt.Println()

	default:
		fmt.Println("Invalid option. Keeping incomplete jobs.")
		fmt.Println()
	}

	// TODO: Add /resume command in future (create GitHub issue)
	fmt.Println("💡 Tip: Future releases will support '/resume <job-id>' to continue interrupted work")
	fmt.Println()
}
