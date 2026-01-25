package repl

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/tools"
)

// executePhasedAgentSync executes a phased agent SYNCHRONOUSLY for interactive mode
// This bypasses the async job system so interactive prompts work in the REPL
func (r *REPL) executePhasedAgentSync(ctx context.Context, agentName string, prompt string, callback agents.PhaseCallback) error {
	// Create a temporary job ID for context
	jobID := fmt.Sprintf("interactive-%d", r.session.StartTime.Unix())

	// Create context manager
	contextMgr, err := llmcontext.NewManager(jobID, r.session.Config.Debug.Enabled, r.session.Config.Model.ContextSize)
	if err != nil {
		return fmt.Errorf("failed to create context manager: %w", err)
	}

	// Create LLM backend
	backend, err := llm.NewBackend(r.session.Config)
	if err != nil {
		return fmt.Errorf("failed to create LLM backend: %w", err)
	}

	// Create the appropriate agent
	var baseAgent *agents.CodingBaseAgent
	var phases []agents.Phase

	switch agentName {
	case "build":
		builderAgent := agents.NewBuilderPhasedAgent(r.session.Config, backend, nil)
		// Register code tools
		builderAgent.RegisterTool(tools.NewFileTool())
		builderAgent.RegisterTool(tools.NewCodeEditTool())
		builderAgent.RegisterTool(tools.NewSearchTool(r.session.Config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewNavigateTool(r.session.Config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewGitTool(r.session.Config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewBashTool(r.session.Config, r.session.Config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewTestTool(r.session.Config.Project.Workdir))
		builderAgent.RegisterTool(tools.NewGitHubTool(""))
		baseAgent = builderAgent.CodingBaseAgent
		phases = builderAgent.GetPhases()

	case "debug":
		debuggerAgent := agents.NewDebuggerPhasedAgent(r.session.Config, backend, nil)
		// Register code tools
		debuggerAgent.RegisterTool(tools.NewFileTool())
		debuggerAgent.RegisterTool(tools.NewCodeEditTool())
		debuggerAgent.RegisterTool(tools.NewSearchTool(r.session.Config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewNavigateTool(r.session.Config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewGitTool(r.session.Config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewBashTool(r.session.Config, r.session.Config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewTestTool(r.session.Config.Project.Workdir))
		debuggerAgent.RegisterTool(tools.NewGitHubTool(""))
		baseAgent = debuggerAgent.CodingBaseAgent
		phases = debuggerAgent.GetPhases()

	case "review":
		reviewerAgent := agents.NewReviewerPhasedAgent(r.session.Config, backend, nil)
		// Register code tools
		reviewerAgent.RegisterTool(tools.NewFileTool())
		reviewerAgent.RegisterTool(tools.NewCodeEditTool())
		reviewerAgent.RegisterTool(tools.NewSearchTool(r.session.Config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewNavigateTool(r.session.Config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewGitTool(r.session.Config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewBashTool(r.session.Config, r.session.Config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewTestTool(r.session.Config.Project.Workdir))
		reviewerAgent.RegisterTool(tools.NewGitHubTool(""))
		baseAgent = reviewerAgent.CodingBaseAgent
		phases = reviewerAgent.GetPhases()

	default:
		return fmt.Errorf("unknown agent: %s (only build/debug/review support interactive mode)", agentName)
	}

	// Create phased executor
	executor := agents.NewPhasedExecutor(baseAgent.BaseAgent, contextMgr, phases)
	executor.SetPhaseCallback(callback)

	// Execute synchronously
	if err := executor.Execute(ctx, prompt); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}
