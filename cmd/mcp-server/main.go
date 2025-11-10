package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	depcheck "github.com/soypete/pedrocli/pkg/init"
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

	// Register basic tools
	fileTool := tools.NewFileTool()
	gitTool := tools.NewGitTool(workDir)
	bashTool := tools.NewBashTool(cfg, workDir)
	testTool := tools.NewTestTool(workDir)

	server.RegisterTool(fileTool)
	server.RegisterTool(gitTool)
	server.RegisterTool(bashTool)
	server.RegisterTool(testTool)

	// Create and register agents
	builderAgent := agents.NewBuilderAgent(cfg, backend, jobManager)
	builderAgent.RegisterTool(fileTool)
	builderAgent.RegisterTool(gitTool)
	builderAgent.RegisterTool(bashTool)
	builderAgent.RegisterTool(testTool)

	reviewerAgent := agents.NewReviewerAgent(cfg, backend, jobManager)
	reviewerAgent.RegisterTool(fileTool)
	reviewerAgent.RegisterTool(gitTool)
	reviewerAgent.RegisterTool(bashTool)
	reviewerAgent.RegisterTool(testTool)

	debuggerAgent := agents.NewDebuggerAgent(cfg, backend, jobManager)
	debuggerAgent.RegisterTool(fileTool)
	debuggerAgent.RegisterTool(gitTool)
	debuggerAgent.RegisterTool(bashTool)
	debuggerAgent.RegisterTool(testTool)

	triagerAgent := agents.NewTriagerAgent(cfg, backend, jobManager)
	triagerAgent.RegisterTool(fileTool)
	triagerAgent.RegisterTool(gitTool)
	triagerAgent.RegisterTool(bashTool)
	triagerAgent.RegisterTool(testTool)

	// Wrap agents as tools for MCP
	server.RegisterTool(mcp.NewAgentTool(builderAgent))
	server.RegisterTool(mcp.NewAgentTool(reviewerAgent))
	server.RegisterTool(mcp.NewAgentTool(debuggerAgent))
	server.RegisterTool(mcp.NewAgentTool(triagerAgent))

	// Start server
	log.Println("Pedroceli MCP server starting...")
	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
