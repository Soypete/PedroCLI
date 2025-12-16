package executor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
)

// DirectExecutor executes agents without MCP
type DirectExecutor struct {
	config     *config.Config
	backend    llm.Backend
	jobManager *jobs.Manager
	factory    *agents.AgentFactory
}

// NewDirectExecutor creates a direct executor
func NewDirectExecutor(cfg *config.Config) (*DirectExecutor, error) {
	// Create backend
	var backend llm.Backend
	switch cfg.Model.Type {
	case "llamacpp":
		backend = llm.NewLlamaCppClient(cfg)
	case "ollama":
		backend = llm.NewOllamaBackend(cfg)
	default:
		return nil, fmt.Errorf("unsupported model type: %s", cfg.Model.Type)
	}

	// Create job manager
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	// Determine work directory
	workDir := cfg.Project.Workdir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create agent factory
	factory := agents.NewAgentFactory(cfg, backend, jobManager, workDir)

	return &DirectExecutor{
		config:     cfg,
		backend:    backend,
		jobManager: jobManager,
		factory:    factory,
	}, nil
}

// Execute runs an agent directly
func (e *DirectExecutor) Execute(ctx context.Context, agentName string, input map[string]interface{}) (*jobs.Job, error) {
	// Get agent
	agent, err := e.factory.CreateAgent(agentName)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Execute agent
	job, err := agent.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	return job, nil
}

// ExecuteWithProgress runs an agent and displays real-time progress
func (e *DirectExecutor) ExecuteWithProgress(ctx context.Context, agentName string, input map[string]interface{}, verbose bool) error {
	// Start execution in goroutine (it will create job and run to completion)
	errChan := make(chan error, 1)
	go func() {
		_, err := e.Execute(ctx, agentName, input)
		if err != nil {
			errChan <- err
		}
	}()

	// Poll for job creation (agent creates job immediately, then runs inference)
	var jobID string
	pollStart := time.Now()
	for time.Since(pollStart) < 10*time.Second {
		// List all jobs and find the most recent one that matches our criteria
		// This is a simple approach - in production we'd want the agent to signal the job ID
		jobs := e.jobManager.List()
		if len(jobs) > 0 {
			// Get the most recent job
			latestJob := jobs[len(jobs)-1]
			if latestJob.Status == "running" || latestJob.Status == "completed" {
				jobID = latestJob.ID
				break
			}
		}

		// Check if execution failed early
		select {
		case err := <-errChan:
			return err
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	if jobID == "" {
		return fmt.Errorf("timeout waiting for job creation")
	}

	// Display progress
	return e.displayProgress(ctx, jobID, verbose)
}

// displayProgress shows real-time updates
func (e *DirectExecutor) displayProgress(ctx context.Context, jobID string, verbose bool) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	if verbose {
		fmt.Printf("\n📋 Job %s started...\n", jobID)
	} else {
		fmt.Printf("\nJob %s started...\n", jobID)
	}

	lastStatus := jobs.Status("")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			job, err := e.jobManager.Get(jobID)
			if err != nil {
				return fmt.Errorf("failed to get job status: %w", err)
			}

			// Display status change
			if job.Status != lastStatus {
				if verbose {
					fmt.Printf("  Status: %s\n", string(job.Status))
				} else {
					fmt.Printf("\rStatus: %s", string(job.Status))
				}
				lastStatus = job.Status
			}

			// Check if complete
			if job.Status == jobs.StatusCompleted {
				if !verbose {
					fmt.Println() // New line after status
				}
				fmt.Println("\n✅ Job completed successfully")
				displayJobOutput(job, verbose)
				return nil
			}

			if job.Status == jobs.StatusFailed {
				if !verbose {
					fmt.Println() // New line after status
				}
				fmt.Println("\n❌ Job failed")
				if job.Error != "" {
					fmt.Printf("Error: %s\n", job.Error)
				}
				return fmt.Errorf("job failed")
			}

			if job.Status == jobs.StatusCancelled {
				if !verbose {
					fmt.Println() // New line after status
				}
				fmt.Println("\n⚠️  Job cancelled")
				return fmt.Errorf("job cancelled")
			}
		}
	}
}

// displayJobOutput shows the job output based on type
func displayJobOutput(job *jobs.Job, verbose bool) {
	if job.Output == nil {
		if verbose {
			fmt.Println("  (No output)")
		}
		return
	}

	if verbose {
		fmt.Println("\n📄 Output:")
		fmt.Println("  " + repeatString("-", 50))
	}

	// Display based on output type
	if reviewText, ok := job.Output["review_text"].(string); ok {
		fmt.Println(reviewText)
	} else if response, ok := job.Output["response"].(string); ok {
		fmt.Println(response)
	} else if diagnosis, ok := job.Output["diagnosis"].(string); ok {
		fmt.Println(diagnosis)
	} else if triageReport, ok := job.Output["triage_report"].(string); ok {
		fmt.Println(triageReport)
	} else {
		// Generic output display
		fmt.Printf("%v\n", job.Output)
	}

	if verbose {
		fmt.Println("  " + repeatString("-", 50))
	}
}

// repeatString repeats a string n times
func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
