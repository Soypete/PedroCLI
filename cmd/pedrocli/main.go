package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/database"
	depcheck "github.com/soypete/pedrocli/pkg/init"
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
	case "podcast":
		podcastCommand(cfg, os.Args[2:])
	case "migrate":
		migrateCommand(cfg, os.Args[2:])
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
  podcast    Podcast production tools (create-script, review-news, add-link, add-guest, create-outline)
  migrate    Database migration commands (up, down, status, reset, redo, version)
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
  pedrocli podcast create-script -topic "Starting a Homelab" -notes "Cover hardware, OS choices"
  pedrocli migrate up
  pedrocli migrate status
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
func pollJobStatus(ctx context.Context, client *mcp.Client, jobID string) error {
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
			response, err := client.CallTool(ctx, "get_job_status", map[string]interface{}{
				"job_id": jobID,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to check status: %v\n", err)
				continue
			}

			// Extract status from response
			if len(response.Content) > 0 && response.Content[0].Type == "text" {
				status := response.Content[0].Text

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
	// Start MCP client
	client, ctx, cancel, err := startMCPClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start MCP client: %v\n", err)
		os.Exit(1)
	}
	defer cancel()
	defer client.Stop()

	// Call agent
	fmt.Printf("\nStarting %s job...\n", agentName)
	response, err := client.CallTool(ctx, agentName, arguments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to call %s: %v\n", agentName, err)
		os.Exit(1)
	}

	// Extract job ID from response
	if response.IsError {
		fmt.Printf("\n❌ Failed to start %s job:\n", agentName)
		for _, block := range response.Content {
			if block.Type == "text" {
				fmt.Println(block.Text)
			}
		}
		os.Exit(1)
	}

	var jobID string
	for _, block := range response.Content {
		if block.Type == "text" {
			fmt.Println(block.Text)
			jobID, err = extractJobID(block.Text)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		}
	}

	if jobID == "" {
		fmt.Println("\n⚠️  Job started but couldn't extract job ID. Check 'pedrocli list' for status.")
		return
	}

	// Poll for status
	if err := pollJobStatus(ctx, client, jobID); err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			fmt.Println("\n⚠️  Stopped watching job. Job continues in background.")
			fmt.Printf("Use 'pedrocli status %s' to check progress.\n", jobID)
		}
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
	defer client.Stop()

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

	callMCPTool(cfg, "get_job_status", arguments)
}

func listCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	fmt.Println("Listing all jobs...")

	callMCPTool(cfg, "list_jobs", map[string]interface{}{})
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

	callMCPTool(cfg, "cancel_job", arguments)
}

// podcastCommand handles all podcast-related subcommands
func podcastCommand(cfg *config.Config, args []string) {
	if len(args) == 0 {
		printPodcastUsage()
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "create-script":
		podcastCreateScriptCommand(cfg, args[1:])
	case "review-news":
		podcastReviewNewsCommand(cfg, args[1:])
	case "add-link":
		podcastAddLinkCommand(cfg, args[1:])
	case "add-guest":
		podcastAddGuestCommand(cfg, args[1:])
	case "create-outline":
		podcastCreateOutlineCommand(cfg, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown podcast action: %s\n\n", action)
		printPodcastUsage()
		os.Exit(1)
	}
}

func printPodcastUsage() {
	fmt.Println(`Podcast production tools

Usage:
  pedrocli podcast <action> [flags]

Actions:
  create-script    Create a podcast episode script from a topic
  review-news      Review and summarize news items for episode prep
  add-link         Add an article or news link to Notion database
  add-guest        Add guest information to the guests database
  create-outline   Create an episode outline from a topic

Examples:
  pedrocli podcast create-script -topic "Starting a Homelab" -notes "Cover hardware, OS choices, first services"
  pedrocli podcast review-news -focus "AI" -max-items 5
  pedrocli podcast add-link -url "https://example.com/article" -title "Great article" -notes "Useful for episode 42"
  pedrocli podcast add-guest -name "John Doe" -email "john@example.com" -bio "Homelab expert"
  pedrocli podcast create-outline -topic "Kubernetes Basics" -duration "45min"`)
}

func podcastCreateScriptCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("create-script", flag.ExitOnError)
	topic := fs.String("topic", "", "Episode topic (required)")
	notes := fs.String("notes", "", "Additional notes or key points (optional)")
	guests := fs.String("guests", "", "Guest names, comma-separated (optional)")
	recordingDate := fs.String("recording-date", "", "Recording date (optional)")
	fs.Parse(args)

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "Error: -topic is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Creating podcast script for: %s\n", *topic)

	// Build arguments
	arguments := map[string]interface{}{
		"topic": *topic,
	}
	if *notes != "" {
		arguments["notes"] = *notes
	}
	if *guests != "" {
		// Split comma-separated guests into array
		guestList := strings.Split(*guests, ",")
		guestArray := make([]interface{}, len(guestList))
		for i, g := range guestList {
			guestArray[i] = strings.TrimSpace(g)
		}
		arguments["guests"] = guestArray
	}
	if *recordingDate != "" {
		arguments["recording_date"] = *recordingDate
	}

	callAgent(cfg, "create_podcast_script", arguments)
}

func podcastReviewNewsCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("review-news", flag.ExitOnError)
	focus := fs.String("focus", "", "Focus topic for news filtering (optional)")
	maxItems := fs.Int("max-items", 0, "Maximum number of news items to review (optional)")
	fs.Parse(args)

	fmt.Println("Reviewing news items for podcast...")

	// Build arguments
	arguments := map[string]interface{}{}
	if *focus != "" {
		arguments["focus_topic"] = *focus
	}
	if *maxItems > 0 {
		arguments["max_items"] = float64(*maxItems)
	}

	callAgent(cfg, "review_news_summary", arguments)
}

func podcastAddLinkCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("add-link", flag.ExitOnError)
	url := fs.String("url", "", "URL to add (required)")
	title := fs.String("title", "", "Article title (optional)")
	notes := fs.String("notes", "", "Notes about the link (optional)")
	database := fs.String("database", "", "Target Notion database (optional)")
	fs.Parse(args)

	if *url == "" {
		fmt.Fprintln(os.Stderr, "Error: -url is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Adding link: %s\n", *url)

	// Build arguments
	arguments := map[string]interface{}{
		"url": *url,
	}
	if *title != "" {
		arguments["title"] = *title
	}
	if *notes != "" {
		arguments["notes"] = *notes
	}
	if *database != "" {
		arguments["database"] = *database
	}

	callAgent(cfg, "add_notion_link", arguments)
}

func podcastAddGuestCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("add-guest", flag.ExitOnError)
	name := fs.String("name", "", "Guest name (required)")
	title := fs.String("title", "", "Guest title/role (optional)")
	organization := fs.String("organization", "", "Guest organization (optional)")
	bio := fs.String("bio", "", "Guest bio (optional)")
	email := fs.String("email", "", "Guest email (optional)")
	topics := fs.String("topics", "", "Topics of expertise, comma-separated (optional)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "Error: -name is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Adding guest: %s\n", *name)

	// Build arguments
	arguments := map[string]interface{}{
		"name": *name,
	}
	if *title != "" {
		arguments["title"] = *title
	}
	if *organization != "" {
		arguments["organization"] = *organization
	}
	if *bio != "" {
		arguments["bio"] = *bio
	}
	if *email != "" {
		arguments["email"] = *email
	}
	if *topics != "" {
		// Split comma-separated topics into array
		topicList := strings.Split(*topics, ",")
		topicArray := make([]interface{}, len(topicList))
		for i, t := range topicList {
			topicArray[i] = strings.TrimSpace(t)
		}
		arguments["topics"] = topicArray
	}

	callAgent(cfg, "add_guest", arguments)
}

func podcastCreateOutlineCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("create-outline", flag.ExitOnError)
	topic := fs.String("topic", "", "Episode topic (required)")
	duration := fs.String("duration", "", "Target episode duration (optional)")
	format := fs.String("format", "", "Episode format (optional)")
	fs.Parse(args)

	if *topic == "" {
		fmt.Fprintln(os.Stderr, "Error: -topic is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("Creating episode outline for: %s\n", *topic)

	// Build arguments
	arguments := map[string]interface{}{
		"topic": *topic,
	}
	if *duration != "" {
		arguments["duration"] = *duration
	}
	if *format != "" {
		arguments["format"] = *format
	}

	callAgent(cfg, "create_episode_outline", arguments)
}

// migrateCommand handles database migration subcommands
func migrateCommand(cfg *config.Config, args []string) {
	if len(args) == 0 {
		printMigrateUsage()
		os.Exit(1)
	}

	action := args[0]

	// Get database path from config
	dbPath := cfg.RepoStorage.DatabasePath

	switch action {
	case "up":
		migrateUpCommand(dbPath)
	case "down":
		migrateDownCommand(dbPath)
	case "status":
		migrateStatusCommand(dbPath)
	case "reset":
		migrateResetCommand(dbPath)
	case "redo":
		migrateRedoCommand(dbPath)
	case "version":
		migrateVersionCommand(dbPath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown migrate action: %s\n\n", action)
		printMigrateUsage()
		os.Exit(1)
	}
}

func printMigrateUsage() {
	fmt.Println(`Database migration commands

Usage:
  pedrocli migrate <action>

Actions:
  up        Run all pending migrations
  down      Rollback the last migration
  status    Show migration status
  reset     Rollback all migrations
  redo      Rollback and re-run the last migration
  version   Show current database version

Examples:
  pedrocli migrate up
  pedrocli migrate down
  pedrocli migrate status
  pedrocli migrate reset
  pedrocli migrate redo
  pedrocli migrate version`)
}

func migrateUpCommand(dbPath string) {
	fmt.Println("Running database migrations...")

	store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Migrate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migrations complete")
}

func migrateDownCommand(dbPath string) {
	fmt.Println("Rolling back last migration...")

	store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.MigrateDown(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: rollback failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Rollback complete")
}

func migrateStatusCommand(dbPath string) {
	store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Printf("Database: %s\n\n", dbPath)
	if err := store.MigrationStatus(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get migration status: %v\n", err)
		os.Exit(1)
	}
}

func migrateResetCommand(dbPath string) {
	fmt.Println("Rolling back all migrations...")

	store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.MigrateReset(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: reset failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Reset complete")
}

func migrateRedoCommand(dbPath string) {
	fmt.Println("Re-running last migration...")

	store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.MigrateRedo(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: redo failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Redo complete")
}

func migrateVersionCommand(dbPath string) {
	store, err := database.NewSQLiteStoreWithOptions(dbPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	version, err := store.MigrationVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get database version: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Database version: %d\n", version)
}
