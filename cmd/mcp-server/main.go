package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/database"
	"github.com/soypete/pedrocli/pkg/hooks"
	depcheck "github.com/soypete/pedrocli/pkg/init"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/mcp"
	"github.com/soypete/pedrocli/pkg/repos"
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
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		log.Fatalf("Failed to create LLM backend: %v", err)
	}

	// Create job manager
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
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

	// Register advanced code editing and navigation tools
	codeEditTool := tools.NewCodeEditTool()
	searchTool := tools.NewSearchTool(workDir)
	navigateTool := tools.NewNavigateTool(workDir)

	// Register job management tools
	getJobStatusTool := tools.NewGetJobStatusTool(jobManager)
	listJobsTool := tools.NewListJobsTool(jobManager)
	cancelJobTool := tools.NewCancelJobTool(jobManager)

	// Initialize repo management system
	var repoStore repos.Store
	if cfg.RepoStorage.DatabasePath != "" {
		store, err := database.NewSQLiteStore(cfg.RepoStorage.DatabasePath)
		if err != nil {
			// Log but don't fail - repo management is optional
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize repo store: %v\n", err)
		} else {
			repoStore = store
			defer store.Close()
		}
	}

	// Create hooks manager
	hooksManager := hooks.NewManager()

	// Create repo manager with GOPATH-style storage
	repoManager := repos.NewManager(
		repos.WithBasePath(cfg.RepoStorage.BasePath),
		repos.WithStore(repoStore),
	)

	// Create git operations and executor
	gitOps := repos.NewGitOps()
	repoExecutor := repos.NewExecutor()

	// Create repo management tool
	repoTool := tools.NewRepoTool(repoManager, gitOps, hooksManager, repoExecutor)

	server.RegisterTool(fileTool)
	server.RegisterTool(gitTool)
	server.RegisterTool(bashTool)
	server.RegisterTool(testTool)
	server.RegisterTool(codeEditTool)
	server.RegisterTool(searchTool)
	server.RegisterTool(navigateTool)
	server.RegisterTool(getJobStatusTool)
	server.RegisterTool(listJobsTool)
	server.RegisterTool(cancelJobTool)
	server.RegisterTool(repoTool)

	// Create and register agents with all tools
	builderAgent := agents.NewBuilderAgent(cfg, backend, jobManager)
	builderAgent.RegisterTool(fileTool)
	builderAgent.RegisterTool(codeEditTool)
	builderAgent.RegisterTool(searchTool)
	builderAgent.RegisterTool(navigateTool)
	builderAgent.RegisterTool(gitTool)
	builderAgent.RegisterTool(bashTool)
	builderAgent.RegisterTool(testTool)
	builderAgent.RegisterTool(repoTool)

	reviewerAgent := agents.NewReviewerAgent(cfg, backend, jobManager)
	reviewerAgent.RegisterTool(fileTool)
	reviewerAgent.RegisterTool(codeEditTool)
	reviewerAgent.RegisterTool(searchTool)
	reviewerAgent.RegisterTool(navigateTool)
	reviewerAgent.RegisterTool(gitTool)
	reviewerAgent.RegisterTool(bashTool)
	reviewerAgent.RegisterTool(testTool)
	reviewerAgent.RegisterTool(repoTool)

	debuggerAgent := agents.NewDebuggerAgent(cfg, backend, jobManager)
	debuggerAgent.RegisterTool(fileTool)
	debuggerAgent.RegisterTool(codeEditTool)
	debuggerAgent.RegisterTool(searchTool)
	debuggerAgent.RegisterTool(navigateTool)
	debuggerAgent.RegisterTool(gitTool)
	debuggerAgent.RegisterTool(bashTool)
	debuggerAgent.RegisterTool(testTool)
	debuggerAgent.RegisterTool(repoTool)

	triagerAgent := agents.NewTriagerAgent(cfg, backend, jobManager)
	triagerAgent.RegisterTool(fileTool)
	triagerAgent.RegisterTool(codeEditTool)
	triagerAgent.RegisterTool(searchTool)
	triagerAgent.RegisterTool(navigateTool)
	triagerAgent.RegisterTool(gitTool)
	triagerAgent.RegisterTool(bashTool)
	triagerAgent.RegisterTool(testTool)
	triagerAgent.RegisterTool(repoTool)

	// Wrap agents as tools for MCP
	server.RegisterTool(mcp.NewAgentTool(builderAgent))
	server.RegisterTool(mcp.NewAgentTool(reviewerAgent))
	server.RegisterTool(mcp.NewAgentTool(debuggerAgent))
	server.RegisterTool(mcp.NewAgentTool(triagerAgent))

	// Start server (no logging to avoid corrupting JSON-RPC on stdout)
	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
