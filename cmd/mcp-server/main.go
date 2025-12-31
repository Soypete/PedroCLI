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
	"github.com/soypete/pedrocli/pkg/tokens"
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

	// Create web scraping tool (only if enabled)
	var webScrapeTool *tools.WebScrapeTool
	if cfg.WebScraping.Enabled {
		webScrapeTool = tools.NewWebScrapeTool(cfg, nil) // Token manager will be set later if available
	}

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
	if webScrapeTool != nil {
		server.RegisterTool(webScrapeTool)
	}

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
	if webScrapeTool != nil {
		builderAgent.RegisterTool(webScrapeTool)
	}

	reviewerAgent := agents.NewReviewerAgent(cfg, backend, jobManager)
	reviewerAgent.RegisterTool(fileTool)
	reviewerAgent.RegisterTool(codeEditTool)
	reviewerAgent.RegisterTool(searchTool)
	reviewerAgent.RegisterTool(navigateTool)
	reviewerAgent.RegisterTool(gitTool)
	reviewerAgent.RegisterTool(bashTool)
	reviewerAgent.RegisterTool(testTool)
	reviewerAgent.RegisterTool(repoTool)
	if webScrapeTool != nil {
		reviewerAgent.RegisterTool(webScrapeTool)
	}

	debuggerAgent := agents.NewDebuggerAgent(cfg, backend, jobManager)
	debuggerAgent.RegisterTool(fileTool)
	debuggerAgent.RegisterTool(codeEditTool)
	debuggerAgent.RegisterTool(searchTool)
	debuggerAgent.RegisterTool(navigateTool)
	debuggerAgent.RegisterTool(gitTool)
	debuggerAgent.RegisterTool(bashTool)
	debuggerAgent.RegisterTool(testTool)
	debuggerAgent.RegisterTool(repoTool)
	if webScrapeTool != nil {
		debuggerAgent.RegisterTool(webScrapeTool)
	}

	triagerAgent := agents.NewTriagerAgent(cfg, backend, jobManager)
	triagerAgent.RegisterTool(fileTool)
	triagerAgent.RegisterTool(codeEditTool)
	triagerAgent.RegisterTool(searchTool)
	triagerAgent.RegisterTool(navigateTool)
	triagerAgent.RegisterTool(gitTool)
	triagerAgent.RegisterTool(bashTool)
	triagerAgent.RegisterTool(testTool)
	triagerAgent.RegisterTool(repoTool)
	if webScrapeTool != nil {
		triagerAgent.RegisterTool(webScrapeTool)
	}

	// Create token manager for podcast tools (only if podcast mode is enabled)
	var tokenManager *tokens.Manager
	var toolTokenManager tools.TokenManager
	if cfg.Podcast.Enabled {
		// Create token store using the same database as repo store
		var tokenStore tokens.Store
		if repoStore != nil {
			// Use the existing database connection
			if sqliteStore, ok := repoStore.(*database.SQLiteStore); ok {
				tokenStore = tokens.NewSQLiteTokenStore(sqliteStore.DB())
			}
		}

		// If no database store, create in-memory store (tokens won't persist)
		if tokenStore == nil {
			fmt.Fprintf(os.Stderr, "Warning: No database configured, podcast tokens will not persist\n")
		} else {
			// Create token manager with refresh handlers
			tokenManager = tokens.NewManager(tokenStore)

			// Register Google OAuth refresh handler (if configured)
			if cfg.OAuth.Google.ClientID != "" && cfg.OAuth.Google.ClientSecret != "" {
				tokenManager.RegisterRefreshHandler(
					tokens.NewGoogleRefreshHandler(cfg.OAuth.Google.ClientID, cfg.OAuth.Google.ClientSecret),
				)
			}

			// Register Notion refresh handler (no-op, API keys don't expire)
			tokenManager.RegisterRefreshHandler(tokens.NewNotionRefreshHandler())

			// Wrap token manager for tools (only exposes access tokens, never full token objects)
			toolTokenManager = &tokenManagerAdapter{manager: tokenManager}
		}
	}

	// Create podcast tools (only if podcast mode is enabled)
	var notionTool *tools.NotionTool
	var calendarTool *tools.CalendarTool
	if cfg.Podcast.Enabled {
		notionTool = tools.NewNotionTool(cfg, toolTokenManager)
		calendarTool = tools.NewCalendarTool(cfg, toolTokenManager)
	}

	// Create and register podcast agents (only if podcast mode is enabled)
	if cfg.Podcast.Enabled {
		scriptCreatorAgent := agents.NewScriptCreatorAgent(cfg, backend, jobManager)
		scriptCreatorAgent.RegisterTool(fileTool)
		scriptCreatorAgent.RegisterTool(notionTool)
		scriptCreatorAgent.RegisterTool(calendarTool)

		newsReviewerAgent := agents.NewNewsReviewerAgent(cfg, backend, jobManager)
		newsReviewerAgent.RegisterTool(fileTool)
		newsReviewerAgent.RegisterTool(notionTool)

		linkAdderAgent := agents.NewLinkAdderAgent(cfg, backend, jobManager)
		linkAdderAgent.RegisterTool(fileTool)
		linkAdderAgent.RegisterTool(notionTool)

		guestAdderAgent := agents.NewGuestAdderAgent(cfg, backend, jobManager)
		guestAdderAgent.RegisterTool(fileTool)
		guestAdderAgent.RegisterTool(notionTool)

		episodeOutlinerAgent := agents.NewEpisodeOutlinerAgent(cfg, backend, jobManager)
		episodeOutlinerAgent.RegisterTool(fileTool)
		episodeOutlinerAgent.RegisterTool(notionTool)
		episodeOutlinerAgent.RegisterTool(calendarTool)

		// Wrap podcast agents as tools for MCP
		server.RegisterTool(mcp.NewAgentTool(scriptCreatorAgent))
		server.RegisterTool(mcp.NewAgentTool(newsReviewerAgent))
		server.RegisterTool(mcp.NewAgentTool(linkAdderAgent))
		server.RegisterTool(mcp.NewAgentTool(guestAdderAgent))
		server.RegisterTool(mcp.NewAgentTool(episodeOutlinerAgent))
	}

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

// tokenManagerAdapter wraps tokens.Manager to implement tools.TokenManager interface
// SECURITY: This adapter ensures that only access tokens are exposed to tools, never the full token object
// This prevents tokens from being logged or accidentally exposed to the LLM
type tokenManagerAdapter struct {
	manager *tokens.Manager
}

// GetToken retrieves only the access token string, hiding all other token details
func (a *tokenManagerAdapter) GetToken(ctx context.Context, provider, service string) (string, error) {
	if a.manager == nil {
		return "", fmt.Errorf("token manager not initialized")
	}

	token, err := a.manager.GetToken(ctx, provider, service)
	if err != nil {
		return "", err
	}

	// Return ONLY the access token string - never expose the full token object
	return token.AccessToken, nil
}
