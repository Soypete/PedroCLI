package mcp

import (
	"context"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/tools"
)

// AgentTool wraps an agent to make it available as an MCP tool
type AgentTool struct {
	agent agents.Agent
}

// NewAgentTool creates a new agent tool wrapper
func NewAgentTool(agent agents.Agent) *AgentTool {
	return &AgentTool{
		agent: agent,
	}
}

// Name returns the tool name (same as agent name)
func (at *AgentTool) Name() string {
	return at.agent.Name()
}

// Description returns the tool description (same as agent description)
func (at *AgentTool) Description() string {
	return at.agent.Description()
}

// Execute executes the agent and returns the job ID immediately (agent runs async)
func (at *AgentTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	// Execute the agent (now runs asynchronously)
	job, err := at.agent.Execute(ctx, args)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Return job ID immediately so client can poll for status
	output := "Job " + job.ID + " started and running in background. Use get_job_status to check progress."

	return &tools.Result{
		Success: true,
		Output:  output,
		Error:   "",
	}, nil
}
