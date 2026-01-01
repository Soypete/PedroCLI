package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	depcheck "github.com/soypete/pedrocli/pkg/init"
	"github.com/soypete/pedrocli/pkg/jobs"
)

const version = "0.3.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle help and version before config loading
	subcommand := os.Args[1]
	if subcommand == "help" || subcommand == "-h" || subcommand == "--help" {
		printUsage()
		os.Exit(0)
	}
	if subcommand == "version" || subcommand == "-v" || subcommand == "--version" {
		fmt.Printf("pedrocli version %s\n", version)
		os.Exit(0)
	}

	// Global flags
	verbosePtr := flag.Bool("verbose", false, "Enable verbose output")
	skipChecksPtr := flag.Bool("skip-checks", false, "Skip dependency checks")

	// Parse global flags first
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override config with flags
	if *verbosePtr {
		cfg.Init.Verbose = true
	}
	if *skipChecksPtr {
		cfg.Init.SkipChecks = true
	}

	// Check dependencies (unless skipped)
	if !cfg.Init.SkipChecks {
		checker := depcheck.NewChecker(cfg)
		results, err := checker.CheckAll()

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Print successful checks in verbose mode
		if cfg.Init.Verbose {
			fmt.Println("✓ All dependencies OK")
			for _, result := range results {
				if result.Found {
					fmt.Printf("  ✓ %s: %s\n", result.Name, result.Version)
				}
			}
		}
	}

	// Handle subcommands
	switch subcommand {
	case "build":
		buildCommand(cfg, os.Args[2:])
	case "debug":
		debugCommand(cfg, os.Args[2:])
	case "review":
		reviewCommand(cfg, os.Args[2:])
	case "triage":
		triageCommand(cfg, os.Args[2:])
	case "blog":
		blogCommand(cfg, os.Args[2:])
	case "status":
		statusCommand(cfg, os.Args[2:])
	case "list":
		listCommand(cfg, os.Args[2:])
	case "cancel":
		cancelCommand(cfg, os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`pedrocli - AI-powered coding agent

Usage:
  pedrocli <command> [flags]

Commands:
  build      Build a new feature autonomously
  debug      Debug and fix an issue
  review     Review a pull request or branch
  triage     Diagnose and triage an issue (no fix)
  blog       Create a blog post (writes to Notion)
  status     Get status of a job
  list       List all jobs
  cancel     Cancel a running job

Global Flags:
  -verbose         Enable verbose output
  -skip-checks     Skip dependency checks
  -version         Print version and exit

Examples:
  pedrocli build -description "Add rate limiting" -issue GH-123
  pedrocli debug -symptoms "Bot crashes on startup" -logs error.log
  pedrocli review -branch feature/rate-limiting
  pedrocli triage -description "Memory leak in handler"
  pedrocli blog -title "My Post" -content "Raw thoughts here..."
  pedrocli blog -prompt "Write a 2025 recap with calendar events..." -publish
  pedrocli status job-1234567890
  pedrocli list
  pedrocli cancel job-1234567890

For more information: https://github.com/soypete/pedrocli`)
}

// pollJobStatus polls for job status until completion
func pollJobStatus(ctx context.Context, jobMgr *jobs.Manager, jobID string) error {
	fmt.Printf("\n⏳ Job %s is running...\n", jobID)
	fmt.Println("Checking status every 5 seconds. Press Ctrl+C to stop watching (job will continue in background).")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastStatus := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			job, err := jobMgr.Get(ctx, jobID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to check status: %v\n", err)
				continue
			}

			// Build status string
			status := fmt.Sprintf("Status: %s", job.Status)

			// Only print if status changed
			if status != lastStatus {
				fmt.Println(status)
				lastStatus = status
			}

			// Check if job is complete
			if job.Status == jobs.StatusCompleted {
				fmt.Println("\n✅ Job completed successfully!")
				return nil
			}
			if job.Status == jobs.StatusFailed {
				fmt.Printf("\n❌ Job failed: %s\n", job.Error)
				return fmt.Errorf("job failed: %s", job.Error)
			}
		}
	}
}

// executeAgent executes an agent and polls for completion
func executeAgent(cfg *config.Config, agent agents.Agent, arguments map[string]interface{}) {
	// Initialize app context for job manager
	appCtx, err := NewAppContext(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Limits.MaxTaskDurationMinutes)*time.Minute)
	defer cancel()

	// Execute the agent
	fmt.Printf("\nStarting %s job...\n", agent.Name())
	job, err := agent.Execute(ctx, arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start %s: %v\n", agent.Name(), err)
		os.Exit(1)
	}

	fmt.Printf("Job %s started\n", job.ID)

	// Poll for status
	if err := pollJobStatus(ctx, appCtx.JobManager, job.ID); err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			fmt.Println("\n⚠️  Stopped watching job. Job continues in background.")
			fmt.Printf("Use 'pedrocli status %s' to check progress.\n", job.ID)
		}
	}
}

func buildCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	description := fs.String("description", "", "Feature description (required)")
	issue := fs.String("issue", "", "GitHub issue number (optional)")
	fs.Parse(args)

	if *description == "" {
		fmt.Fprintln(os.Stderr, "Error: -description is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Building feature: %s\n", *description)
	if *issue != "" {
		fmt.Printf("Issue: %s\n", *issue)
	}

	// Build arguments for the agent
	arguments := map[string]interface{}{
		"description": *description,
	}
	if *issue != "" {
		arguments["issue"] = *issue
	}

	// Create and execute agent
	appCtx, err := NewAppContext(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	agent := NewBuilderAgentWithTools(appCtx)
	executeAgent(cfg, agent, arguments)
}

func debugCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	symptoms := fs.String("symptoms", "", "Problem symptoms (required)")
	logs := fs.String("logs", "", "Path to log file (optional)")
	fs.Parse(args)

	if *symptoms == "" {
		fmt.Fprintln(os.Stderr, "Error: -symptoms is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Debugging issue: %s\n", *symptoms)
	if *logs != "" {
		fmt.Printf("Logs: %s\n", *logs)
	}

	// Build arguments
	arguments := map[string]interface{}{
		"description": *symptoms, // Agent expects "description"
	}
	if *logs != "" {
		arguments["error_log"] = *logs // Agent expects "error_log"
	}

	// Create and execute agent
	appCtx, err := NewAppContext(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	agent := NewDebuggerAgentWithTools(appCtx)
	executeAgent(cfg, agent, arguments)
}

func reviewCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	branch := fs.String("branch", "", "Branch name (required)")
	prNumber := fs.String("pr", "", "PR number (optional)")
	fs.Parse(args)

	if *branch == "" {
		fmt.Fprintln(os.Stderr, "Error: -branch is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Reviewing branch: %s\n", *branch)
	if *prNumber != "" {
		fmt.Printf("PR: %s\n", *prNumber)
	}

	// Build arguments
	arguments := map[string]interface{}{
		"branch": *branch,
	}
	if *prNumber != "" {
		arguments["pr_number"] = *prNumber
	}

	// Create and execute agent
	appCtx, err := NewAppContext(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	agent := NewReviewerAgentWithTools(appCtx)
	executeAgent(cfg, agent, arguments)
}

func triageCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("triage", flag.ExitOnError)
	description := fs.String("description", "", "Issue description (required)")
	errorLogs := fs.String("error-logs", "", "Error logs (optional)")
	fs.Parse(args)

	if *description == "" {
		fmt.Fprintln(os.Stderr, "Error: -description is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Triaging issue: %s\n", *description)
	if *errorLogs != "" {
		fmt.Printf("Error logs: %s\n", *errorLogs)
	}

	// Build arguments
	arguments := map[string]interface{}{
		"description": *description,
	}
	if *errorLogs != "" {
		arguments["error_log"] = *errorLogs // Agent expects "error_log"
	}

	// Create and execute agent
	appCtx, err := NewAppContext(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	agent := NewTriagerAgentWithTools(appCtx)
	executeAgent(cfg, agent, arguments)
}

func blogCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("blog", flag.ExitOnError)
	title := fs.String("title", "", "Blog post title (optional for orchestrate)")
	content := fs.String("content", "", "Blog post content/dictation (for simple posts)")
	prompt := fs.String("prompt", "", "Complex blog prompt for orchestration (use this for multi-step posts)")
	publish := fs.Bool("publish", false, "Auto-publish to Notion after generation")
	fs.Parse(args)

	// Check which mode we're in
	if *prompt != "" {
		// Orchestrated blog post (complex, multi-phase)
		fmt.Println("Starting blog orchestrator...")
		if *title != "" {
			fmt.Printf("Title hint: %s\n", *title)
		}

		arguments := map[string]interface{}{
			"prompt":  *prompt,
			"publish": *publish,
		}
		if *title != "" {
			arguments["title"] = *title
		}

		// Create and execute blog orchestrator
		appCtx, err := NewAppContext(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
			os.Exit(1)
		}

		agent := NewBlogOrchestratorAgentWithTools(appCtx)
		executeAgent(cfg, agent, arguments)
	} else if *content != "" {
		// Simple blog post - use blog_notion tool directly
		if *title == "" {
			fmt.Fprintln(os.Stderr, "Error: -title is required for simple blog posts")
			fs.Usage()
			os.Exit(1)
		}

		fmt.Printf("Creating blog post: %s\n", *title)

		// For simple posts, use the blog notion tool directly
		appCtx, err := NewAppContext(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
			os.Exit(1)
		}

		ctx := context.Background()
		result, err := appCtx.BlogNotionTool.Execute(ctx, map[string]interface{}{
			"action":  "create",
			"title":   *title,
			"content": *content,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create blog post: %v\n", err)
			os.Exit(1)
		}

		if result.Success {
			fmt.Println("\n✅ Blog post created successfully!")
			fmt.Println(result.Output)
		} else {
			fmt.Printf("\n❌ Failed to create blog post: %s\n", result.Error)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Error: either -prompt (for orchestrated posts) or -content (for simple posts) is required")
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  Simple post:       pedrocli blog -title \"My Post\" -content \"Raw thoughts...\"")
		fmt.Fprintln(os.Stderr, "  Orchestrated post: pedrocli blog -prompt \"Write a 2025 recap with...\" -publish")
		os.Exit(1)
	}
}

func statusCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "Error: job ID required")
		fmt.Fprintln(os.Stderr, "Usage: pedrocli status <job-id>")
		os.Exit(1)
	}

	jobID := fs.Args()[0]
	fmt.Printf("Getting status for job: %s\n", jobID)

	// Get job status directly from job manager
	jobMgr, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize job manager: %v\n", err)
		os.Exit(1)
	}

	job, err := jobMgr.Get(context.Background(), jobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get job status: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nJob: %s\n", job.ID)
	fmt.Printf("Type: %s\n", job.Type)
	fmt.Printf("Status: %s\n", job.Status)
	fmt.Printf("Created: %s\n", job.CreatedAt.Format(time.RFC3339))
	if job.StartedAt != nil {
		fmt.Printf("Started: %s\n", job.StartedAt.Format(time.RFC3339))
	}
	if job.CompletedAt != nil {
		fmt.Printf("Completed: %s\n", job.CompletedAt.Format(time.RFC3339))
	}
	if job.Error != "" {
		fmt.Printf("Error: %s\n", job.Error)
	}
	if job.Output != nil {
		fmt.Println("Output:", job.Output)
	}
}

func listCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	fmt.Println("Listing all jobs...")

	// List jobs directly from job manager
	jobMgr, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize job manager: %v\n", err)
		os.Exit(1)
	}

	jobList, err := jobMgr.List(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list jobs: %v\n", err)
		os.Exit(1)
	}
	if len(jobList) == 0 {
		fmt.Println("\nNo jobs found.")
		return
	}

	fmt.Printf("\nFound %d job(s):\n\n", len(jobList))
	for _, job := range jobList {
		status := string(job.Status)
		switch job.Status {
		case jobs.StatusCompleted:
			status = "✅ " + status
		case jobs.StatusFailed:
			status = "❌ " + status
		case jobs.StatusRunning:
			status = "🔄 " + status
		case jobs.StatusPending:
			status = "⏳ " + status
		}

		fmt.Printf("%s [%s] %s\n", job.ID, status, truncate(job.Description, 50))
	}
}

func cancelCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("cancel", flag.ExitOnError)
	fs.Parse(args)

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "Error: job ID required")
		fmt.Fprintln(os.Stderr, "Usage: pedrocli cancel <job-id>")
		os.Exit(1)
	}

	jobID := fs.Args()[0]
	fmt.Printf("Canceling job: %s\n", jobID)

	// Cancel job directly using job manager
	jobMgr, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize job manager: %v\n", err)
		os.Exit(1)
	}

	if err := jobMgr.Cancel(context.Background(), jobID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cancel job: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Job cancelled successfully")
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
