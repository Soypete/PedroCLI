package main

import (
	"flag"
	"fmt"
	"os"

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

	// TODO: Spawn MCP server and call build_feature tool
	fmt.Println("\n🚧 MCP integration coming soon...")
	fmt.Println("This will:")
	fmt.Println("  1. Start MCP server")
	fmt.Println("  2. Call build_feature tool")
	fmt.Println("  3. Return job ID for monitoring")
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

	// TODO: Spawn MCP server and call debug_issue tool
	fmt.Println("\n🚧 MCP integration coming soon...")
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

	// TODO: Spawn MCP server and call review_pr tool
	fmt.Println("\n🚧 MCP integration coming soon...")
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

	// TODO: Spawn MCP server and call triage_issue tool
	fmt.Println("\n🚧 MCP integration coming soon...")
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

	// TODO: Spawn MCP server and call get_job_status tool
	fmt.Println("\n🚧 MCP integration coming soon...")
}

func listCommand(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)

	fmt.Println("Listing all jobs...")

	// TODO: Spawn MCP server and call list_jobs tool
	fmt.Println("\n🚧 MCP integration coming soon...")
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

	// TODO: Spawn MCP server and call cancel_job tool
	fmt.Println("\n🚧 MCP integration coming soon...")
}
