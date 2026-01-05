package main

import (
	"context"
	"log"
	"os"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/database"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AppContext holds all the shared dependencies
type AppContext struct {
	Config     *config.Config
	Backend    llm.Backend
	JobManager jobs.JobManager
	Database   *database.DB
	WorkDir    string

	// Tools
	FileTool     tools.Tool
	GitTool      tools.Tool
	BashTool     tools.Tool
	TestTool     tools.Tool
	CodeEditTool tools.Tool
	SearchTool   tools.Tool
	NavigateTool tools.Tool

	// Blog tools
	RSSFeedTool     tools.Tool
	StaticLinksTool tools.Tool
	BlogNotionTool  tools.Tool

	// Tool Registry for grammar generation
	ToolRegistry *tools.ToolRegistry
}

// NewAppContext creates and initializes the application context with database-backed job manager.
func NewAppContext(cfg *config.Config) (*AppContext, error) {
	// Create LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		return nil, err
	}

	// Create database connection
	dbCfg := &database.Config{
		Driver:   cfg.Database.Driver,
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Database: cfg.Database.Database,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		SSLMode:  cfg.Database.SSLMode,
	}
	// If no driver specified in config, use defaults
	if dbCfg.Driver == "" {
		dbCfg = database.DefaultConfig()
	}
	db, err := database.New(dbCfg)
	if err != nil {
		return nil, err
	}

	// Run migrations
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// Create job store and manager
	jobStore := storage.NewJobStore(db.DB)
	jobManager := jobs.NewDBManager(jobStore)

	// Migrate existing file-based jobs to database
	migrated, err := jobManager.MigrateFromFiles(ctx, "/tmp/pedrocli-jobs")
	if err != nil {
		log.Printf("Warning: failed to migrate jobs from files: %v", err)
	} else if migrated > 0 {
		log.Printf("Migrated %d jobs from files to database", migrated)
	}

	// Get working directory
	workDir := cfg.Project.Workdir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create tools
	appCtx := &AppContext{
		Config:     cfg,
		Backend:    backend,
		JobManager: jobManager,
		Database:   db,
		WorkDir:    workDir,
	}

	// Initialize code tools
	appCtx.FileTool = tools.NewFileTool()
	appCtx.GitTool = tools.NewGitTool(workDir)
	appCtx.BashTool = tools.NewBashTool(cfg, workDir)
	appCtx.TestTool = tools.NewTestTool(workDir)
	appCtx.CodeEditTool = tools.NewCodeEditTool()
	appCtx.SearchTool = tools.NewSearchTool(workDir)
	appCtx.NavigateTool = tools.NewNavigateTool(workDir)

	// Initialize blog tools
	appCtx.RSSFeedTool = tools.NewRSSFeedTool(cfg)
	appCtx.StaticLinksTool = tools.NewStaticLinksTool(cfg)
	appCtx.BlogNotionTool = tools.NewBlogNotionTool(cfg)

	// Create and populate tool registry for grammar generation
	registry := tools.NewToolRegistry()
	// Register code tools as ExtendedTools (they implement Metadata())
	registry.Register(appCtx.FileTool)
	registry.Register(appCtx.CodeEditTool)
	registry.Register(appCtx.SearchTool)
	registry.Register(appCtx.NavigateTool)
	registry.Register(appCtx.GitTool)
	registry.Register(appCtx.BashTool)
	registry.Register(appCtx.TestTool)
	appCtx.ToolRegistry = registry

	return appCtx, nil
}

// Close closes the database connection.
func (ctx *AppContext) Close() error {
	if ctx.Database != nil {
		return ctx.Database.Close()
	}
	return nil
}

// registerCodeTools registers standard code tools with an agent
func registerCodeTools(agent interface{ RegisterTool(tools.Tool) }, ctx *AppContext) {
	agent.RegisterTool(ctx.FileTool)
	agent.RegisterTool(ctx.CodeEditTool)
	agent.RegisterTool(ctx.SearchTool)
	agent.RegisterTool(ctx.NavigateTool)
	agent.RegisterTool(ctx.GitTool)
	agent.RegisterTool(ctx.BashTool)
	agent.RegisterTool(ctx.TestTool)
}

// NewBuilderAgentWithTools creates a fully configured builder agent with registry
func NewBuilderAgentWithTools(ctx *AppContext) *agents.BuilderAgent {
	// Create base with registry for grammar generation
	base := agents.NewCodingBaseAgentWithRegistry(
		"builder",
		"Build new features autonomously and create draft PRs",
		ctx.Config,
		ctx.Backend,
		ctx.JobManager,
		ctx.ToolRegistry,
	)

	// Still register tools to agent's tool map for execution
	agent := &agents.BuilderAgent{CodingBaseAgent: base}
	registerCodeTools(agent, ctx)
	return agent
}

// NewDebuggerAgentWithTools creates a fully configured debugger agent with registry
func NewDebuggerAgentWithTools(ctx *AppContext) *agents.DebuggerAgent {
	base := agents.NewCodingBaseAgentWithRegistry(
		"debugger",
		"Debug and fix issues autonomously",
		ctx.Config,
		ctx.Backend,
		ctx.JobManager,
		ctx.ToolRegistry,
	)

	agent := &agents.DebuggerAgent{CodingBaseAgent: base}
	registerCodeTools(agent, ctx)
	return agent
}

// NewReviewerAgentWithTools creates a fully configured reviewer agent with registry
func NewReviewerAgentWithTools(ctx *AppContext) *agents.ReviewerAgent {
	base := agents.NewCodingBaseAgentWithRegistry(
		"reviewer",
		"Review code and provide feedback",
		ctx.Config,
		ctx.Backend,
		ctx.JobManager,
		ctx.ToolRegistry,
	)

	agent := &agents.ReviewerAgent{CodingBaseAgent: base}
	registerCodeTools(agent, ctx)
	return agent
}

// NewTriagerAgentWithTools creates a fully configured triager agent with registry
func NewTriagerAgentWithTools(ctx *AppContext) *agents.TriagerAgent {
	base := agents.NewCodingBaseAgentWithRegistry(
		"triager",
		"Diagnose issues without fixing them",
		ctx.Config,
		ctx.Backend,
		ctx.JobManager,
		ctx.ToolRegistry,
	)

	agent := &agents.TriagerAgent{CodingBaseAgent: base}
	registerCodeTools(agent, ctx)
	return agent
}

// NewBlogOrchestratorAgentWithTools creates a fully configured blog orchestrator
func NewBlogOrchestratorAgentWithTools(ctx *AppContext) *agents.BlogOrchestratorAgent {
	agent := agents.NewBlogOrchestratorAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.RegisterResearchTool(ctx.RSSFeedTool)
	agent.RegisterResearchTool(ctx.StaticLinksTool)
	agent.RegisterNotionTool(ctx.BlogNotionTool)
	return agent
}
