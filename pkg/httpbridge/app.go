package httpbridge

import (
	"os"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AppContext holds all the shared dependencies for the HTTP server
type AppContext struct {
	Config     *config.Config
	Backend    llm.Backend
	JobManager *jobs.Manager
	WorkDir    string

	// Tools (used by agents)
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
}

// NewAppContext creates and initializes the application context
func NewAppContext(cfg *config.Config) (*AppContext, error) {
	// Create LLM backend
	backend, err := llm.NewBackend(cfg)
	if err != nil {
		return nil, err
	}

	// Create job manager
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		return nil, err
	}

	// Get working directory
	workDir := cfg.Project.Workdir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create tools
	ctx := &AppContext{
		Config:     cfg,
		Backend:    backend,
		JobManager: jobManager,
		WorkDir:    workDir,
	}

	// Initialize code tools
	ctx.FileTool = tools.NewFileTool()
	ctx.GitTool = tools.NewGitTool(workDir)
	ctx.BashTool = tools.NewBashTool(cfg, workDir)
	ctx.TestTool = tools.NewTestTool(workDir)
	ctx.CodeEditTool = tools.NewCodeEditTool()
	ctx.SearchTool = tools.NewSearchTool(workDir)
	ctx.NavigateTool = tools.NewNavigateTool(workDir)

	// Initialize blog tools
	ctx.RSSFeedTool = tools.NewRSSFeedTool(cfg)
	ctx.StaticLinksTool = tools.NewStaticLinksTool(cfg)
	ctx.BlogNotionTool = tools.NewBlogNotionTool(cfg)

	return ctx, nil
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

// NewBuilderAgentWithTools creates a fully configured builder agent
func (ctx *AppContext) NewBuilderAgent() *agents.BuilderAgent {
	agent := agents.NewBuilderAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	registerCodeTools(agent, ctx)
	return agent
}

// NewDebuggerAgentWithTools creates a fully configured debugger agent
func (ctx *AppContext) NewDebuggerAgent() *agents.DebuggerAgent {
	agent := agents.NewDebuggerAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	registerCodeTools(agent, ctx)
	return agent
}

// NewReviewerAgentWithTools creates a fully configured reviewer agent
func (ctx *AppContext) NewReviewerAgent() *agents.ReviewerAgent {
	agent := agents.NewReviewerAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	registerCodeTools(agent, ctx)
	return agent
}

// NewTriagerAgentWithTools creates a fully configured triager agent
func (ctx *AppContext) NewTriagerAgent() *agents.TriagerAgent {
	agent := agents.NewTriagerAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	registerCodeTools(agent, ctx)
	return agent
}

// NewBlogOrchestratorAgent creates a fully configured blog orchestrator
func (ctx *AppContext) NewBlogOrchestratorAgent() *agents.BlogOrchestratorAgent {
	agent := agents.NewBlogOrchestratorAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.RegisterResearchTool(ctx.RSSFeedTool)
	agent.RegisterResearchTool(ctx.StaticLinksTool)
	agent.RegisterNotionTool(ctx.BlogNotionTool)
	return agent
}

// NewDynamicBlogAgent creates a fully configured dynamic blog agent (ADR-003)
func (ctx *AppContext) NewDynamicBlogAgent() *agents.DynamicBlogAgent {
	agent := agents.NewDynamicBlogAgent(ctx.Config, ctx.Backend, ctx.JobManager)
	agent.RegisterResearchTool(ctx.RSSFeedTool)
	agent.RegisterResearchTool(ctx.StaticLinksTool)
	agent.RegisterNotionTool(ctx.BlogNotionTool)
	return agent
}
