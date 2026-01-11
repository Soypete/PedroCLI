package httpbridge

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
	"github.com/soypete/pedrocli/pkg/storage/blog"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AppContext holds all the shared dependencies for the HTTP server
type AppContext struct {
	Config     *config.Config
	Backend    llm.Backend
	JobManager jobs.JobManager
	Database   *database.DB
	WorkDir    string

	// Workspace manager for HTTP bridge jobs
	WorkspaceManager *WorkspaceManager

	// Stores
	CompactionStatsStore storage.CompactionStatsStore
	BlogStore            *blog.PostStore
	VersionStore         *blog.VersionStore

	// Tools (used by agents)
	FileTool     tools.Tool
	GitTool      tools.Tool
	BashTool     tools.Tool
	TestTool     tools.Tool
	CodeEditTool tools.Tool
	SearchTool   tools.Tool
	NavigateTool tools.Tool
	LSPTool      *tools.LSPTool

	// Blog tools
	RSSFeedTool     tools.Tool
	StaticLinksTool tools.Tool
	BlogNotionTool  tools.Tool

	// Scheduling tools
	CalComTool tools.Tool
}

// NewAppContext creates and initializes the application context with database-backed job manager.
func NewAppContext(cfg *config.Config) (*AppContext, error) {
	return NewAppContextWithDB(cfg, nil)
}

// NewAppContextWithDB creates and initializes the application context with an optional database.
// If db is nil, it will create a new database connection using default config.
func NewAppContextWithDB(cfg *config.Config, db *database.DB) (*AppContext, error) {
	// Create LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		return nil, err
	}

	// Create or use provided database
	if db == nil {
		dbCfg := database.DefaultConfig()
		db, err = database.New(dbCfg)
		if err != nil {
			return nil, err
		}
	}

	// Run migrations
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		return nil, err
	}

	// Create job store and manager
	jobStore := storage.NewJobStore(db.DB)
	jobManager := jobs.NewDBManager(jobStore)

	// Create compaction stats store
	compactionStatsStore := database.NewCompactionStatsStore(db.DB)

	// Create blog stores
	blogStore := blog.NewPostStore(db.DB)
	versionStore := blog.NewVersionStore(db.DB)

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

	// Create workspace manager for HTTP bridge jobs
	workspaceManager := NewWorkspaceManager(cfg.HTTPBridge.WorkspacePath)

	// Create tools
	appCtx := &AppContext{
		Config:               cfg,
		Backend:              backend,
		JobManager:           jobManager,
		Database:             db,
		WorkDir:              workDir,
		WorkspaceManager:     workspaceManager,
		CompactionStatsStore: compactionStatsStore,
		BlogStore:            blogStore,
		VersionStore:         versionStore,
	}

	// Initialize code tools
	appCtx.FileTool = tools.NewFileTool()
	appCtx.GitTool = tools.NewGitTool(workDir)
	appCtx.BashTool = tools.NewBashTool(cfg, workDir)
	appCtx.TestTool = tools.NewTestTool(workDir)
	appCtx.CodeEditTool = tools.NewCodeEditTool()
	appCtx.SearchTool = tools.NewSearchTool(workDir)
	appCtx.NavigateTool = tools.NewNavigateTool(workDir)

	// Initialize LSP tool if enabled
	if cfg.LSP.Enabled {
		appCtx.LSPTool = tools.NewLSPTool(cfg, workDir)
	}

	// Initialize blog tools
	appCtx.RSSFeedTool = tools.NewRSSFeedTool(cfg)
	appCtx.StaticLinksTool = tools.NewStaticLinksTool(cfg)
	appCtx.BlogNotionTool = tools.NewBlogNotionTool(cfg)

	// Initialize scheduling tools
	if cfg.CalCom.Enabled {
		appCtx.CalComTool = tools.NewCalComTool(cfg, nil) // nil tokenManager for now
	}

	return appCtx, nil
}

// Close closes all resources including database and LSP servers.
func (ctx *AppContext) Close() error {
	var errs []error

	// Shutdown LSP servers
	if ctx.LSPTool != nil {
		if err := ctx.LSPTool.Shutdown(context.Background()); err != nil {
			errs = append(errs, err)
		}
	}

	// Close database
	if ctx.Database != nil {
		if err := ctx.Database.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
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
	// Register LSP tool if enabled
	if ctx.LSPTool != nil {
		agent.RegisterTool(ctx.LSPTool)
	}
}

// registerSchedulingTools registers scheduling tools (Cal.com) with an agent
func registerSchedulingTools(agent interface{ RegisterTool(tools.Tool) }, ctx *AppContext) {
	if ctx.CalComTool != nil {
		agent.RegisterTool(ctx.CalComTool)
	}
}

// NewBuilderAgent creates a fully configured phased builder agent
func (ctx *AppContext) NewBuilderAgent() *agents.BuilderPhasedAgent {
	agent := agents.NewBuilderPhasedAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.SetCompactionStatsStore(ctx.CompactionStatsStore)
	registerCodeTools(agent, ctx)
	return agent
}

// NewDebuggerAgent creates a fully configured phased debugger agent
func (ctx *AppContext) NewDebuggerAgent() *agents.DebuggerPhasedAgent {
	agent := agents.NewDebuggerPhasedAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.SetCompactionStatsStore(ctx.CompactionStatsStore)
	registerCodeTools(agent, ctx)
	return agent
}

// NewReviewerAgent creates a fully configured phased reviewer agent
func (ctx *AppContext) NewReviewerAgent() *agents.ReviewerPhasedAgent {
	agent := agents.NewReviewerPhasedAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.SetCompactionStatsStore(ctx.CompactionStatsStore)
	registerCodeTools(agent, ctx)
	return agent
}

// NewTriagerAgentWithTools creates a fully configured triager agent
func (ctx *AppContext) NewTriagerAgent() *agents.TriagerAgent {
	agent := agents.NewTriagerAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.SetCompactionStatsStore(ctx.CompactionStatsStore)
	registerCodeTools(agent, ctx)
	return agent
}

// NewBlogOrchestratorAgent creates a fully configured blog orchestrator
func (ctx *AppContext) NewBlogOrchestratorAgent() *agents.BlogOrchestratorAgent {
	agent := agents.NewBlogOrchestratorAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.SetCompactionStatsStore(ctx.CompactionStatsStore)
	agent.RegisterResearchTool(ctx.RSSFeedTool)
	agent.RegisterResearchTool(ctx.StaticLinksTool)
	agent.RegisterNotionTool(ctx.BlogNotionTool)
	registerSchedulingTools(agent, ctx)
	return agent
}

// NewDynamicBlogAgent creates a fully configured dynamic blog agent (ADR-003)
func (ctx *AppContext) NewDynamicBlogAgent() *agents.DynamicBlogAgent {
	agent := agents.NewDynamicBlogAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.SetCompactionStatsStore(ctx.CompactionStatsStore)
	agent.RegisterResearchTool(ctx.RSSFeedTool)
	agent.RegisterResearchTool(ctx.StaticLinksTool)
	agent.RegisterNotionTool(ctx.BlogNotionTool)
	registerSchedulingTools(agent, ctx)
	return agent
}
