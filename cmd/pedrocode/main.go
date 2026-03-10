package main

import (
	"fmt"
	"os"
)

func main() {
	// Parse flags
	debugMode := false
	configPath := ""
	mode := "code" // default mode

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-d", "--debug":
			debugMode = true
		case "-c", "--config":
			if i+1 < len(args) {
				i++
				configPath = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "Error: --config requires a path argument\n")
				os.Exit(1)
			}
		case "-h", "--help", "help":
			printHelp()
			return
		case "-v", "--version", "version":
			printVersion()
			return
		default:
			mode = args[i]
		}
	}

	// Route based on mode
	var err error
	switch mode {
	case "code":
		err = runCodeMode(debugMode, configPath)
	case "blog":
		err = runBlogMode(debugMode)
	case "podcast":
		err = runPodcastMode(debugMode)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", mode)
		fmt.Fprintf(os.Stderr, "Use 'pedrocode --help' for usage information\n")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	help := `pedrocode - Interactive REPL for PedroCLI

Usage:
  pedrocode [options] [mode]

Modes:
  code      Interactive coding assistant (default)
            Agents: build, debug, review, triage

  blog      Interactive blog writing assistant
            Agents: blog, writer, editor

  podcast   Interactive podcast preparation assistant
            Agents: podcast

Options:
  -c, --config   Path to config file (default: .pedrocli.json)
  -d, --debug    Enable debug mode (verbose logging + keep logs)
  -h, --help     Show this help message
  -v, --version  Show version information

Examples:
  pedrocode                  # Start in code mode (default)
  pedrocode code             # Explicitly start in code mode
  pedrocode blog             # Start in blog mode
  pedrocode --debug          # Start with debug logging
  pedrocode --debug podcast  # Debug mode + podcast mode
  pedrocode --config .pedrocli.example.exo.json  # Use Exo cluster config

Debug Mode:
  When enabled with --debug:
  - Logs saved to /tmp/pedrocode-sessions/<session-id>/
  - Includes: session.log, agent-calls.log, tool-calls.log, llm-requests.log
  - Logs are kept after exit (otherwise auto-cleaned)
  - Shows log directory path on startup

REPL Commands:
  /help         Show REPL help
  /quit         Exit REPL
  /mode <name>  Switch agent within current mode
  /history      Show command history
  /context      Show session information

See also:
  pedrocli      - Background job execution
  pedroweb      - Web UI (HTTP server)
`
	fmt.Print(help)
}

func printVersion() {
	// TODO: Get version from build-time variable
	fmt.Println("pedrocode version 0.1.0")
	fmt.Println("Interactive REPL for PedroCLI")
}
