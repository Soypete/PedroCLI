package toolformat

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AgentAdapter wraps an agent as a tool for direct execution
type AgentAdapter struct {
	agent agents.Agent
}

// NewAgentAdapter creates an adapter for an agent
func NewAgentAdapter(agent agents.Agent) *AgentAdapter {
	return &AgentAdapter{agent: agent}
}

// Name returns the agent's name as a tool
func (a *AgentAdapter) Name() string {
	return a.agent.Name()
}

// Description returns the agent's description
func (a *AgentAdapter) Description() string {
	return a.agent.Description()
}

// Execute runs the agent and returns its output
func (a *AgentAdapter) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	// Call the agent's Execute method (runs asynchronously, returns job)
	job, err := a.agent.Execute(ctx, args)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Job %s started", job.ID),
	}, nil
}

// AgentFactory creates agents with proper tool configuration
type AgentFactory struct {
	config     *config.Config
	backend    llm.Backend
	jobManager *jobs.Manager
	workDir    string
	codeTools  []tools.Tool
	blogTools  []tools.Tool
}

// NewAgentFactory creates a new agent factory
func NewAgentFactory(cfg *config.Config, backend llm.Backend, jobManager *jobs.Manager, workDir string) *AgentFactory {
	return &AgentFactory{
		config:     cfg,
		backend:    backend,
		jobManager: jobManager,
		workDir:    workDir,
	}
}

// WithCodeTools sets the code tools available to agents
func (f *AgentFactory) WithCodeTools(codeTools []tools.Tool) *AgentFactory {
	f.codeTools = codeTools
	return f
}

// WithBlogTools sets the blog tools available to agents
func (f *AgentFactory) WithBlogTools(blogTools []tools.Tool) *AgentFactory {
	f.blogTools = blogTools
	return f
}

// CreateCodeTools creates the standard code tools
func (f *AgentFactory) CreateCodeTools() []tools.Tool {
	return []tools.Tool{
		tools.NewFileTool(),
		tools.NewCodeEditTool(),
		tools.NewSearchTool(f.workDir),
		tools.NewNavigateTool(f.workDir),
		tools.NewGitTool(f.workDir),
		tools.NewBashTool(f.config, f.workDir),
		tools.NewTestTool(f.workDir),
	}
}

// CreateBlogTools creates the blog research tools
func (f *AgentFactory) CreateBlogTools() []tools.Tool {
	blogTools := []tools.Tool{
		tools.NewRSSFeedTool(f.config),
	}

	if f.config.Blog.Enabled {
		blogTools = append(blogTools, tools.NewStaticLinksTool(f.config))
		blogTools = append(blogTools, tools.NewBlogNotionTool(f.config))
	}

	return blogTools
}

// CreateBuilder creates a builder agent
func (f *AgentFactory) CreateBuilder() agents.Agent {
	agent := agents.NewBuilderAgent(f.config, f.backend, f.jobManager)
	for _, tool := range f.codeTools {
		agent.RegisterTool(tool)
	}
	return agent
}

// CreateDebugger creates a debugger agent
func (f *AgentFactory) CreateDebugger() agents.Agent {
	agent := agents.NewDebuggerAgent(f.config, f.backend, f.jobManager)
	for _, tool := range f.codeTools {
		agent.RegisterTool(tool)
	}
	return agent
}

// CreateReviewer creates a reviewer agent
func (f *AgentFactory) CreateReviewer() agents.Agent {
	agent := agents.NewReviewerAgent(f.config, f.backend, f.jobManager)
	for _, tool := range f.codeTools {
		agent.RegisterTool(tool)
	}
	return agent
}

// CreateTriager creates a triager agent
func (f *AgentFactory) CreateTriager() agents.Agent {
	agent := agents.NewTriagerAgent(f.config, f.backend, f.jobManager)
	for _, tool := range f.codeTools {
		agent.RegisterTool(tool)
	}
	return agent
}

// CreateBlogOrchestrator creates a blog orchestrator agent
func (f *AgentFactory) CreateBlogOrchestrator() agents.Agent {
	agent := agents.NewBlogOrchestratorAgent(f.config, f.backend, f.jobManager)

	// Register research tools
	for _, tool := range f.blogTools {
		if tool.Name() == "blog_notion" {
			agent.RegisterNotionTool(tool)
		} else {
			agent.RegisterResearchTool(tool)
		}
	}

	return agent
}

// RegisterAgentsInRegistry registers all agents as tools in the registry
func (f *AgentFactory) RegisterAgentsInRegistry(registry *Registry) error {
	// Ensure tools are created
	if len(f.codeTools) == 0 {
		f.codeTools = f.CreateCodeTools()
	}
	if len(f.blogTools) == 0 {
		f.blogTools = f.CreateBlogTools()
	}

	// Create and register code agents
	agentsToRegister := []agents.Agent{
		f.CreateBuilder(),
		f.CreateDebugger(),
		f.CreateReviewer(),
		f.CreateTriager(),
	}

	// Add blog orchestrator if blog is enabled
	if f.config.Blog.Enabled {
		agentsToRegister = append(agentsToRegister, f.CreateBlogOrchestrator())
	}

	// Register each agent as a tool
	for _, agent := range agentsToRegister {
		def := agentToDefinition(agent)
		if err := registry.Register(def); err != nil {
			return fmt.Errorf("failed to register agent %s: %w", agent.Name(), err)
		}
	}

	return nil
}

// agentToDefinition converts an agent to a ToolDefinition
func agentToDefinition(agent agents.Agent) *ToolDefinition {
	// Create handler that calls agent.Execute (runs asynchronously)
	handler := func(args map[string]interface{}) (*ToolResult, error) {
		job, err := agent.Execute(context.Background(), args)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Job %s started", job.ID),
		}, nil
	}

	// Get appropriate schema based on agent type
	schema := getAgentSchema(agent.Name())

	return &ToolDefinition{
		Name:        agent.Name(),
		Description: agent.Description(),
		Category:    CategoryAgent,
		Parameters:  schema,
		Handler:     handler,
	}
}

// getAgentSchema returns the parameter schema for an agent
func getAgentSchema(name string) ParameterSchema {
	switch name {
	case "builder":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"description"},
			Properties: map[string]PropertySchema{
				"description": {Type: "string", Description: "Description of the feature to build"},
				"issue":       {Type: "string", Description: "GitHub issue number (optional)"},
			},
		}
	case "debugger":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"description"},
			Properties: map[string]PropertySchema{
				"description": {Type: "string", Description: "Description of the bug symptoms"},
				"error_log":   {Type: "string", Description: "Path to error log file (optional)"},
			},
		}
	case "reviewer":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"branch"},
			Properties: map[string]PropertySchema{
				"branch":    {Type: "string", Description: "Branch name to review"},
				"pr_number": {Type: "string", Description: "PR number (optional)"},
			},
		}
	case "triager":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"description"},
			Properties: map[string]PropertySchema{
				"description": {Type: "string", Description: "Description of the issue to triage"},
				"error_log":   {Type: "string", Description: "Path to error log file (optional)"},
			},
		}
	case "blog_orchestrator":
		return ParameterSchema{
			Type:     "object",
			Required: []string{"prompt"},
			Properties: map[string]PropertySchema{
				"prompt":  {Type: "string", Description: "Blog post prompt or dictation"},
				"title":   {Type: "string", Description: "Blog post title (optional)"},
				"publish": {Type: "boolean", Description: "Auto-publish to Notion"},
			},
		}
	default:
		return ParameterSchema{
			Type:       "object",
			Properties: map[string]PropertySchema{},
		}
	}
}
