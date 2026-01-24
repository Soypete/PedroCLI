package cli

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/toolformat"
	"github.com/soypete/pedrocli/pkg/tools"
)

// CLIBridge provides a unified interface for the CLI to call tools using direct execution
type CLIBridge struct {
	bridge     toolformat.ToolBridge
	ctx        context.Context
	cancel     context.CancelFunc
	jobManager *jobs.Manager
	config     *config.Config
	backend    llm.Backend
}

// CLIBridgeConfig configures the CLI bridge
type CLIBridgeConfig struct {
	Config  *config.Config // App config
	WorkDir string         // Working directory for tools
}

// NewCLIBridge creates a new CLI bridge using direct tool execution
func NewCLIBridge(cfg CLIBridgeConfig) (*CLIBridge, error) {
	// Create tool factory and registry
	factory := toolformat.NewToolFactory(cfg.Config, cfg.WorkDir)
	registry, err := factory.CreateRegistryForMode(toolformat.ModeAll)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}

	// Register job management tools
	jobManager, err := jobs.NewManager("/tmp/pedrocli-jobs")
	if err != nil {
		return nil, fmt.Errorf("failed to create job manager: %w", err)
	}

	// Register job tools
	jobTools := []tools.Tool{
		tools.NewGetJobStatusTool(jobManager),
		tools.NewListJobsTool(jobManager),
		tools.NewCancelJobTool(jobManager),
	}

	for _, tool := range jobTools {
		def := &toolformat.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			Category:    toolformat.CategoryJob,
			Parameters:  toolformat.GetSchemaForTool(tool.Name()),
			Handler: func(t tools.Tool) toolformat.ToolHandler {
				return func(args map[string]interface{}) (*toolformat.ToolResult, error) {
					result, err := t.Execute(context.Background(), args)
					if err != nil {
						return &toolformat.ToolResult{Success: false, Error: err.Error()}, nil
					}
					return &toolformat.ToolResult{
						Success: result.Success,
						Output:  result.Output,
						Error:   result.Error,
					}, nil
				}
			}(tool),
		}
		if err := registry.Register(def); err != nil {
			return nil, fmt.Errorf("failed to register tool %s: %w", tool.Name(), err)
		}
	}

	// Create LLM backend for agents
	backend, err := llm.NewBackend(cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM backend: %w", err)
	}

	// Create code tools for agents
	fileTool := tools.NewFileTool()
	gitTool := tools.NewGitTool(cfg.WorkDir)
	bashTool := tools.NewBashTool(cfg.Config, cfg.WorkDir)
	testTool := tools.NewTestTool(cfg.WorkDir)
	codeEditTool := tools.NewCodeEditTool()
	searchTool := tools.NewSearchTool(cfg.WorkDir)
	navigateTool := tools.NewNavigateTool(cfg.WorkDir)

	// Helper function to register code tools with an agent
	registerCodeTools := func(agent interface{ RegisterTool(tools.Tool) }) {
		agent.RegisterTool(fileTool)
		agent.RegisterTool(codeEditTool)
		agent.RegisterTool(searchTool)
		agent.RegisterTool(navigateTool)
		agent.RegisterTool(gitTool)
		agent.RegisterTool(bashTool)
		agent.RegisterTool(testTool)
	}

	// Register coding agents (using phased implementations)
	builderAgent := agents.NewBuilderPhasedAgent(cfg.Config, backend, jobManager)
	registerCodeTools(builderAgent)

	debuggerAgent := agents.NewDebuggerPhasedAgent(cfg.Config, backend, jobManager)
	registerCodeTools(debuggerAgent)

	reviewerAgent := agents.NewReviewerPhasedAgent(cfg.Config, backend, jobManager)
	registerCodeTools(reviewerAgent)

	// Note: TriagerAgent doesn't have a phased version yet
	triagerAgent := agents.NewTriagerAgent(cfg.Config, backend, jobManager)
	registerCodeTools(triagerAgent)

	codingAgents := []agents.Agent{
		builderAgent,
		debuggerAgent,
		reviewerAgent,
		triagerAgent,
	}

	for _, agent := range codingAgents {
		agentCopy := agent // Capture for closure
		def := &toolformat.ToolDefinition{
			Name:        agentCopy.Name(),
			Description: agentCopy.Description(),
			Category:    toolformat.CategoryAgent,
			Parameters:  toolformat.GetSchemaForTool(agentCopy.Name()),
			Handler: func(args map[string]interface{}) (*toolformat.ToolResult, error) {
				// Agents return a job, not immediate results
				job, err := agentCopy.Execute(context.Background(), args)
				if err != nil {
					return &toolformat.ToolResult{Success: false, Error: err.Error()}, nil
				}
				return &toolformat.ToolResult{
					Success: true,
					Output:  fmt.Sprintf("Job %s started", job.ID),
				}, nil
			},
		}
		if err := registry.Register(def); err != nil {
			return nil, fmt.Errorf("failed to register agent %s: %w", agentCopy.Name(), err)
		}
	}

	// Get formatter for configured model
	modelName := "generic"
	if cfg.Config != nil && cfg.Config.Model.ModelName != "" {
		modelName = cfg.Config.Model.ModelName
	}
	formatter := toolformat.GetFormatterForModel(modelName)

	// Create direct bridge
	bridge := toolformat.NewDirectBridge(registry, formatter)
	ctx := context.Background()

	return &CLIBridge{
		bridge:     bridge,
		ctx:        ctx,
		cancel:     func() {},
		jobManager: jobManager,
		config:     cfg.Config,
		backend:    backend,
	}, nil
}

// CallTool calls a tool through the bridge
func (b *CLIBridge) CallTool(ctx context.Context, name string, args map[string]interface{}) (*toolformat.BridgeResult, error) {
	return b.bridge.CallTool(ctx, name, args)
}

// IsHealthy returns whether the bridge is healthy
func (b *CLIBridge) IsHealthy() bool {
	return b.bridge.IsHealthy()
}

// GetToolNames returns available tool names
func (b *CLIBridge) GetToolNames() []string {
	return b.bridge.GetToolNames()
}

// Context returns the bridge's context
func (b *CLIBridge) Context() context.Context {
	return b.ctx
}

// GetJobManager returns the job manager for monitoring jobs
func (b *CLIBridge) GetJobManager() *jobs.Manager {
	return b.jobManager
}

// ExecuteAgent creates and executes an agent with the given name and description
func (b *CLIBridge) ExecuteAgent(ctx context.Context, agentName string, description string) (*toolformat.ToolResult, error) {
	// Get config and create agent based on name
	var agent interface {
		Execute(ctx context.Context, args map[string]interface{}) (*jobs.Job, error)
	}

	switch agentName {
	case "build":
		builderAgent := agents.NewBuilderPhasedAgent(b.config, b.backend, b.jobManager)
		// Register code tools
		builderAgent.RegisterTool(tools.NewFileTool())
		builderAgent.RegisterTool(tools.NewCodeEditTool())
		builderAgent.RegisterTool(tools.NewSearchTool(b.config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewNavigateTool(b.config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewGitTool(b.config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewBashTool(b.config, b.config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewTestTool(b.config.Project.Workdir))
		agent = builderAgent
	case "debug":
		debuggerAgent := agents.NewDebuggerPhasedAgent(b.config, b.backend, b.jobManager)
		// Register code tools
		debuggerAgent.RegisterTool(tools.NewFileTool())
		debuggerAgent.RegisterTool(tools.NewCodeEditTool())
		debuggerAgent.RegisterTool(tools.NewSearchTool(b.config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewNavigateTool(b.config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewGitTool(b.config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewBashTool(b.config, b.config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewTestTool(b.config.Project.Workdir))
		agent = debuggerAgent
	case "review":
		reviewerAgent := agents.NewReviewerPhasedAgent(b.config, b.backend, b.jobManager)
		// Register code tools
		reviewerAgent.RegisterTool(tools.NewFileTool())
		reviewerAgent.RegisterTool(tools.NewCodeEditTool())
		reviewerAgent.RegisterTool(tools.NewSearchTool(b.config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewNavigateTool(b.config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewGitTool(b.config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewBashTool(b.config, b.config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewTestTool(b.config.Project.Workdir))
		agent = reviewerAgent
	case "triage":
		triagerAgent := agents.NewTriagerAgent(b.config, b.backend, b.jobManager)
		// Register code tools
		triagerAgent.RegisterTool(tools.NewFileTool())
		triagerAgent.RegisterTool(tools.NewCodeEditTool())
		triagerAgent.RegisterTool(tools.NewSearchTool(b.config.Project.Workdir))
		triagerAgent.RegisterTool(tools.NewNavigateTool(b.config.Project.Workdir))
		triagerAgent.RegisterTool(tools.NewGitTool(b.config.Project.Workdir))
		triagerAgent.RegisterTool(tools.NewBashTool(b.config, b.config.Project.Workdir))
		triagerAgent.RegisterTool(tools.NewTestTool(b.config.Project.Workdir))
		agent = triagerAgent
	default:
		return &toolformat.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown agent: %s", agentName),
		}, nil
	}

	// Prepare arguments with workspace directory
	args := map[string]interface{}{
		"description":   description,
		"workspace_dir": b.config.Project.Workdir,
	}

	// Execute agent
	job, err := agent.Execute(ctx, args)
	if err != nil {
		return &toolformat.ToolResult{
			Success: false,
			Error:   err.Error(),
			Output:  "",
		}, nil
	}

	// Return job result
	return &toolformat.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Job %s started successfully", job.ID),
	}, nil
}

// Close shuts down the bridge
func (b *CLIBridge) Close() {
	if b.cancel != nil {
		b.cancel()
	}
}
