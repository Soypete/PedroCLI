package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/executor"
	depcheck "github.com/soypete/pedrocli/pkg/init"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/mcp"
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

	// Commands that don't need config
	if subcommand == "status" || subcommand == "list" || subcommand == "cancel" {
		switch subcommand {
		case "status":
			statusCommand(nil, os.Args[2:])
		case "list":
			listCommand(nil, os.Args[2:])
		case "cancel":
			cancelCommand(nil, os.Args[2:])
		}
		return
	}

	// Global flags (for commands that need config)
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

	// Handle subcommands that need config
	switch subcommand {
	case "build":
		buildCommand(cfg, os.Args[2:])
	case "debug":
		debugCommand(cfg, os.Args[2:])
	case "review":
		reviewCommand(cfg, os.Args[2:])
	case "triage":
		triageCommand(cfg, os.Args[2:])
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
  pedrocli status job-1234567890
  pedrocli list
  pedrocli cancel job-1234567890

For more information: https://github.com/soypete/pedrocli`)
}

// startMCPClient starts the MCP server and returns a client
func startMCPClient(cfg *config.Config) (*mcp.Client, context.Context, context.CancelFunc, error) {
	// Find the MCP server binary
	serverPath, err := findMCPServer()
	if err != nil {
		return nil, nil, nil, err
	}

	// Create client
	client := mcp.NewClient(serverPath, []string{})

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	// Start server
	if err := client.Start(ctx); err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	return client, ctx, cancel, nil
}

// findMCPServer finds the MCP server binary
func findMCPServer() (string, error) {
	// Try current directory first
	localPath := "./pedrocli-server"
	if _, err := os.Stat(localPath); err == nil {
		abs, _ := filepath.Abs(localPath)
		return abs, nil
	}

	// Try in same directory as the CLI binary
	exePath, err := os.Executable()
	if err == nil {
		serverPath := filepath.Join(filepath.Dir(exePath), "pedrocli-server")
		if _, err := os.Stat(serverPath); err == nil {
			return serverPath, nil
		}
	}

	// Try $PATH
	serverPath, err := exec.LookPath("pedrocli-server")
	if err == nil {
		return serverPath, nil
	}

	return "", fmt.Errorf("pedrocli-server not found. Please build it with 'make build-server'")
}

// Global executor cache (lazily initialized)
var (
	directExecutor     *executor.DirectExecutor
	directExecutorOnce sync.Once
	directExecutorErr  error
)

// newDirectExecutor creates or returns cached direct executor
func newDirectExecutor(cfg *config.Config) (*executor.DirectExecutor, error) {
	directExecutorOnce.Do(func() {
		directExecutor, directExecutorErr = executor.NewDirectExecutor(cfg)
	})
	return directExecutor, directExecutorErr
}

// executeDirectMode executes an agent directly without MCP
func executeDirectMode(cfg *config.Config, agentName string, arguments map[string]interface{}, verbose bool) error {
	exec, err := newDirectExecutor(cfg)
	if err != nil {
		return fmt.Errorf("failed to create direct executor: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := exec.ExecuteWithProgress(ctx, agentName, arguments, verbose); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}

// executeMCPMode executes an agent via MCP client
func executeMCPMode(cfg *config.Config, agentName string, arguments map[string]interface{}) {
	// Start MCP client
	client, ctx, cancel, err := startMCPClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start MCP client: %v\n", err)
		os.Exit(1)
	}
	defer cancel()
	defer func() {
		_ = client.Stop() // Ignore stop errors in defer
	}()

	// Call agent tool
	fmt.Printf("\nStarting %s job via MCP...\n", agentName)
	response, err := client.CallTool(ctx, agentName, arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to call %s: %v\n", agentName, err)
		os.Exit(1)
	}

	// Display response
	if response.IsError {
		fmt.Printf("\n❌ %s failed:\n", agentName)
	} else {
		fmt.Printf("\n✅ %s job started:\n", agentName)
	}

	for _, block := range response.Content {
		if block.Type == "text" {
			fmt.Println(block.Text)
		}
	}
}

func buildCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	description := fs.String("description", "", "Feature description (required)")
	issue := fs.String("issue", "", "GitHub issue number (optional)")
	direct := fs.Bool("direct", false, "Execute directly without MCP (faster, shows progress)")
	verbose := fs.Bool("verbose", false, "Show verbose output in direct mode")
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

	if *description == "" {
		fmt.Fprintln(os.Stderr, "Error: -description is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Building feature: %s\n", *description)
	if *issue != "" {
		fmt.Printf("Issue: %s\n", *issue)
	}

	// Build arguments
	arguments := map[string]interface{}{
		"description": *description,
	}
	if *issue != "" {
		arguments["issue"] = *issue
	}

	// Execute based on mode
	if *direct {
		// Direct execution mode
		if err := executeDirectMode(cfg, "builder", arguments, *verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to execute builder: %v\n", err)
			os.Exit(1)
		}
	} else {
		// MCP mode
		executeMCPMode(cfg, "builder", arguments)
	}
}

// callMCPTool is a helper function to call an MCP tool and display the response
func callMCPTool(cfg *config.Config, toolName string, arguments map[string]interface{}) {
	// Start MCP client
	client, ctx, cancel, err := startMCPClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start MCP client: %v\n", err)
		os.Exit(1)
	}
	defer cancel()
	defer func() {
		_ = client.Stop() // Ignore stop errors in defer
	}()

	// Call tool
	fmt.Printf("\nCalling %s...\n", toolName)
	response, err := client.CallTool(ctx, toolName, arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to call %s: %v\n", toolName, err)
		os.Exit(1)
	}

	// Display response
	fmt.Println()
	if response.IsError {
		fmt.Println("❌ Operation failed:")
	} else {
		fmt.Println("✅ Operation completed:")
	}

	for _, block := range response.Content {
		if block.Type == "text" {
			fmt.Println(block.Text)
		}
	}
}

func debugCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("debug", flag.ExitOnError)
	symptoms := fs.String("symptoms", "", "Problem symptoms (required)")
	logs := fs.String("logs", "", "Path to log file (optional)")
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

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
		"symptoms": *symptoms,
	}
	if *logs != "" {
		arguments["logs"] = *logs
	}

	callMCPTool(cfg, "debugger", arguments)
}

func reviewCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	branch := fs.String("branch", "", "Branch name (required)")
	prNumber := fs.String("pr", "", "PR number (optional)")
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

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

	callMCPTool(cfg, "reviewer", arguments)
}

func triageCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("triage", flag.ExitOnError)
	description := fs.String("description", "", "Issue description (required)")
	errorLogs := fs.String("error-logs", "", "Error logs (optional)")
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

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
		arguments["error_logs"] = *errorLogs
	}

	callMCPTool(cfg, "triager", arguments)
}

func statusCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "Error: job ID required")
		fmt.Fprintln(os.Stderr, "Usage: pedrocli status <job-id>")
		os.Exit(1)
	}

	jobID := fs.Args()[0]

	// Access job manager directly
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create job manager: %v\n", err)
		os.Exit(1)
	}

	job, err := jobManager.Get(jobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: job not found: %v\n", err)
		os.Exit(1)
	}

	// Display job status
	fmt.Printf("Job ID: %s\n", job.ID)
	fmt.Printf("Type: %s\n", job.Type)
	fmt.Printf("Status: %s\n", job.Status)
	fmt.Printf("Description: %s\n", job.Description)
	fmt.Printf("Created: %s\n", job.CreatedAt.Format("2006-01-02 15:04:05"))
	if job.StartedAt != nil {
		fmt.Printf("Started: %s\n", job.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if job.CompletedAt != nil {
		fmt.Printf("Completed: %s\n", job.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if job.Error != "" {
		fmt.Printf("Error: %s\n", job.Error)
	}
}

func listCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

	// Access job manager directly
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create job manager: %v\n", err)
		os.Exit(1)
	}

	jobList := jobManager.List()
	if len(jobList) == 0 {
		fmt.Println("No jobs found")
		return
	}

	fmt.Printf("Found %d job(s):\n\n", len(jobList))
	for _, job := range jobList {
		fmt.Printf("  %s  %-10s  %-12s  %s\n",
			job.ID,
			job.Type,
			job.Status,
			job.Description)
	}
}

func cancelCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("cancel", flag.ExitOnError)
	_ = fs.Parse(args) // Error handling done by flag.ExitOnError

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "Error: job ID required")
		fmt.Fprintln(os.Stderr, "Usage: pedrocli cancel <job-id>")
		os.Exit(1)
	}

	jobID := fs.Args()[0]

	// Access job manager directly
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create job manager: %v\n", err)
		os.Exit(1)
	}

	// Update job to cancelled status
	err = jobManager.Update(jobID, jobs.StatusCancelled, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to cancel job: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Job %s cancelled\n", jobID)
}
