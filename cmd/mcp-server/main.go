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

	// Register blog tools
	blogNotionTool := tools.NewBlogNotionTool(cfg)

	// Register blog research tools
	rssFeedTool := tools.NewRSSFeedTool(cfg)
	staticLinksTool := tools.NewStaticLinksTool(cfg)
	// CalendarTool requires TokenManager - skip for now if not available

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
	server.RegisterTool(blogNotionTool)
	server.RegisterTool(rssFeedTool)
	server.RegisterTool(staticLinksTool)

	// Create and register agents with all tools
	builderAgent := agents.NewBuilderAgent(cfg, backend, jobManager)
	builderAgent.RegisterTool(fileTool)
	builderAgent.RegisterTool(codeEditTool)
	builderAgent.RegisterTool(searchTool)
	builderAgent.RegisterTool(navigateTool)
	builderAgent.RegisterTool(gitTool)
	builderAgent.RegisterTool(bashTool)
	builderAgent.RegisterTool(testTool)

	reviewerAgent := agents.NewReviewerAgent(cfg, backend, jobManager)
	reviewerAgent.RegisterTool(fileTool)
	reviewerAgent.RegisterTool(codeEditTool)
	reviewerAgent.RegisterTool(searchTool)
	reviewerAgent.RegisterTool(navigateTool)
	reviewerAgent.RegisterTool(gitTool)
	reviewerAgent.RegisterTool(bashTool)
	reviewerAgent.RegisterTool(testTool)

	debuggerAgent := agents.NewDebuggerAgent(cfg, backend, jobManager)
	debuggerAgent.RegisterTool(fileTool)
	debuggerAgent.RegisterTool(codeEditTool)
	debuggerAgent.RegisterTool(searchTool)
	debuggerAgent.RegisterTool(navigateTool)
	debuggerAgent.RegisterTool(gitTool)
	debuggerAgent.RegisterTool(bashTool)
	debuggerAgent.RegisterTool(testTool)

	triagerAgent := agents.NewTriagerAgent(cfg, backend, jobManager)
	triagerAgent.RegisterTool(fileTool)
	triagerAgent.RegisterTool(codeEditTool)
	triagerAgent.RegisterTool(searchTool)
	triagerAgent.RegisterTool(navigateTool)
	triagerAgent.RegisterTool(gitTool)
	triagerAgent.RegisterTool(bashTool)
	triagerAgent.RegisterTool(testTool)

	// Create blog orchestrator agent with research tools and Notion publishing
	// (Replaces the old writer/editor agents with a unified orchestrator)
	blogOrchestratorAgent := agents.NewBlogOrchestratorAgent(cfg, backend, jobManager)
	blogOrchestratorAgent.RegisterResearchTool(rssFeedTool)
	blogOrchestratorAgent.RegisterResearchTool(staticLinksTool)
	blogOrchestratorAgent.RegisterNotionTool(blogNotionTool)
	// Note: CalendarTool would be registered here if TokenManager is available

	// Wrap agents as tools for MCP
	server.RegisterTool(mcp.NewAgentTool(builderAgent))
	server.RegisterTool(mcp.NewAgentTool(reviewerAgent))
	server.RegisterTool(mcp.NewAgentTool(debuggerAgent))
	server.RegisterTool(mcp.NewAgentTool(triagerAgent))
	server.RegisterTool(mcp.NewAgentTool(blogOrchestratorAgent))

	// Start server (no logging to avoid corrupting JSON-RPC on stdout)
	if err := server.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
