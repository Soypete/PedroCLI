package repl

import (
	"context"
	"fmt"
	"time"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

// executePhasedAgentSync executes a phased agent SYNCHRONOUSLY for interactive mode
// This bypasses the async job system so interactive prompts work in the REPL
func (r *REPL) executePhasedAgentSync(ctx context.Context, agentName string, prompt string, callback agents.PhaseCallback) error {
	// Create a unique job ID for each interactive run
	jobID := fmt.Sprintf("interactive-%d", time.Now().Unix())

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

	// Create code tools setup (registry + all tools)
	codeTools := tools.NewCodeToolsSetup(r.session.Config, r.session.Config.Project.Workdir)

	// Create the appropriate agent
	var baseAgent *agents.CodingBaseAgent
	var phases []agents.Phase

	switch agentName {
	case "build":
		builderAgent := agents.NewBuilderPhasedAgent(r.session.Config, backend, nil)
		codeTools.RegisterWithAgent(builderAgent)
		baseAgent = builderAgent.CodingBaseAgent
		phases = builderAgent.GetPhases()

	case "debug":
		debuggerAgent := agents.NewDebuggerPhasedAgent(r.session.Config, backend, nil)
		codeTools.RegisterWithAgent(debuggerAgent)
		baseAgent = debuggerAgent.CodingBaseAgent
		phases = debuggerAgent.GetPhases()

	case "review":
		reviewerAgent := agents.NewReviewerPhasedAgent(r.session.Config, backend, nil)
		codeTools.RegisterWithAgent(reviewerAgent)
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
