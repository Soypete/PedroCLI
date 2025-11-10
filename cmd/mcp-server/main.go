package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/init"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/mcp"
	"github.com/soypete/pedrocli/pkg/tools"
)

func main() {
	// Load configuration
	cfg, err := config.LoadDefault()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check dependencies (unless skipped)
	if !cfg.Init.SkipChecks {
		checker := init.NewChecker(cfg)
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

	// Create LLM backend
	var backend llm.Backend
	if cfg.Model.Type == "llamacpp" {
		backend = llm.NewLlamaCppClient(cfg)
	} else {
		log.Fatalf("Unsupported model type: %s", cfg.Model.Type)
	}

	// Create job manager
	jobManager, err := jobs.NewManager("/tmp/pedroceli-jobs")
	if err != nil {
		log.Fatalf("Failed to create job manager: %v", err)
	}

	// Create MCP server
	server := mcp.NewServer()

	// Register tools
	workDir := cfg.Project.Workdir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	server.RegisterTool(tools.NewFileTool())
	server.RegisterTool(tools.NewGitTool(workDir))
	server.RegisterTool(tools.NewBashTool(cfg, workDir))
	server.RegisterTool(tools.NewTestTool(workDir))

	// Start server
	log.Println("Pedroceli MCP server starting...")
	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
