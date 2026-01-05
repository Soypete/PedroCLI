package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/config"
	depcheck "github.com/soypete/pedrocli/pkg/init"
)

const version = "0.2.0-dev"

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

// startBridge starts the CLI bridge and returns it
func startBridge(cfg *config.Config) (*cli.CLIBridge, error) {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}

	bridge, err := cli.NewCLIBridge(cli.CLIBridgeConfig{
		Config:  cfg,
		WorkDir: workDir,
	})
	if err != nil {
		return nil, err
	}

	return bridge, nil
}

// extractJobID extracts job ID from agent response text
func extractJobID(text string) (string, error) {
	// Look for "Job job-XXXXX started"
	re := regexp.MustCompile(`Job (job-\d+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract job ID from response: %s", text)
	}
	return matches[1], nil
}

// pollJobStatus polls for job status until completion
func pollJobStatus(ctx context.Context, bridge *cli.CLIBridge, jobID string) error {
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
			// Call get_job_status
			result, err := bridge.CallTool(ctx, "get_job_status", map[string]interface{}{
				"job_id": jobID,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to check status: %v\n", err)
				continue
			}

			// Extract status from response
			if result.Success && result.Output != "" {
				status := result.Output

				// Only print if status changed
				if status != lastStatus {
					fmt.Println(status)
					lastStatus = status
				}

				// Check if job is complete
				if strings.Contains(strings.ToLower(status), "completed") {
					fmt.Println("\n✅ Job completed successfully!")
					return nil
				}
				if strings.Contains(strings.ToLower(status), "failed") {
					fmt.Println("\n❌ Job failed!")
					return fmt.Errorf("job failed")
				}
			}
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

	// Build arguments for the tool
	arguments := map[string]interface{}{
		"description": *description,
	}
	if *issue != "" {
		arguments["issue"] = *issue
	}

	// Call builder agent and poll for completion
	callAgent(cfg, "builder", arguments)
}

// callAgent is a helper function to call an agent and poll for completion
func callAgent(cfg *config.Config, agentName string, arguments map[string]interface{}) {
	// Start bridge
	bridge, err := startBridge(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start bridge: %v\n", err)
		os.Exit(1)
	}
	defer bridge.Close()

	ctx := bridge.Context()

	// Call agent
	fmt.Printf("\nStarting %s job...\n", agentName)
	result, err := bridge.CallTool(ctx, agentName, arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to call %s: %v\n", agentName, err)
		os.Exit(1)
	}

	// Check for error
	if !result.Success {
		fmt.Printf("\n❌ Failed to start %s job:\n", agentName)
		if result.Error != "" {
			fmt.Println(result.Error)
		}
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		os.Exit(1)
	}

	// Extract job ID from response
	var jobID string
	if result.Output != "" {
		fmt.Println(result.Output)
		jobID, err = extractJobID(result.Output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	if jobID == "" {
		fmt.Println("\n⚠️  Job started but couldn't extract job ID. Check 'pedrocli list' for status.")
		return
	}

	// Poll for status
	if err := pollJobStatus(ctx, bridge, jobID); err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			fmt.Println("\n⚠️  Stopped watching job. Job continues in background.")
			fmt.Printf("Use 'pedrocli status %s' to check progress.\n", jobID)
		}
	}
}

// callTool is a helper function to call a tool and display the response
func callTool(cfg *config.Config, toolName string, arguments map[string]interface{}) {
	// Start bridge
	bridge, err := startBridge(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start bridge: %v\n", err)
		os.Exit(1)
	}
	defer bridge.Close()

	// Call tool
	fmt.Printf("\nCalling %s...\n", toolName)
	result, err := bridge.CallTool(bridge.Context(), toolName, arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to call %s: %v\n", toolName, err)
		os.Exit(1)
	}

	// Display response
	fmt.Println()
	if !result.Success {
		fmt.Println("❌ Operation failed:")
		if result.Error != "" {
			fmt.Println(result.Error)
		}
	} else {
		fmt.Println("✅ Operation completed:")
	}

	if result.Output != "" {
		fmt.Println(result.Output)
	}
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

	callAgent(cfg, "debugger", arguments)
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

	callAgent(cfg, "reviewer", arguments)
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

	callAgent(cfg, "triager", arguments)
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

		callAgent(cfg, "blog_orchestrator", arguments)
	} else if *content != "" {
		// Simple blog post (direct to writer)
		if *title == "" {
			fmt.Fprintln(os.Stderr, "Error: -title is required for simple blog posts")
			fs.Usage()
			os.Exit(1)
		}

		fmt.Printf("Creating blog post: %s\n", *title)

		arguments := map[string]interface{}{
			"title":   *title,
			"content": *content,
		}

		callTool(cfg, "blog_writer", arguments)
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

	// Build arguments
	arguments := map[string]interface{}{
		"job_id": jobID,
	}

	callTool(cfg, "get_job_status", arguments)
}

func listCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	fmt.Println("Listing all jobs...")

	callTool(cfg, "list_jobs", map[string]interface{}{})
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

	// Build arguments
	arguments := map[string]interface{}{
		"job_id": jobID,
	}

	callTool(cfg, "cancel_job", arguments)
}
