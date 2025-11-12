package mcp

import (
	"context"

	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/jobs"
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

// InputSchema returns the JSON Schema for agent arguments
func (at *AgentTool) InputSchema() map[string]interface{} {
	// Agent-specific schemas based on agent name
	agentName := at.agent.Name()

	switch agentName {
	case "builder":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Detailed description of the feature to build",
				},
				"issue_reference": map[string]interface{}{
					"type":        "string",
					"description": "Optional GitHub issue reference (e.g., #123)",
				},
			},
			"required": []string{"description"},
		}
	case "debugger":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"symptoms": map[string]interface{}{
					"type":        "string",
					"description": "Description of the bug symptoms and behavior",
				},
				"log_files": map[string]interface{}{
					"type":        "string",
					"description": "Optional paths to relevant log files",
				},
			},
			"required": []string{"symptoms"},
		}
	case "reviewer":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Git branch name to review",
				},
				"pr_number": map[string]interface{}{
					"type":        "string",
					"description": "Optional PR number for context",
				},
			},
			"required": []string{"branch"},
		}
	case "triager":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the issue to triage and diagnose",
				},
			},
			"required": []string{"description"},
		}
	default:
		// Generic agent schema
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input for the agent",
				},
			},
			"required": []string{"input"},
		}
	}
}

// Execute executes the agent and returns the result
func (at *AgentTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	// Execute the agent
	job, err := at.agent.Execute(ctx, args)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert job to tool result
	output := ""
	if job.Output != nil {
		if reviewText, ok := job.Output["review_text"].(string); ok {
			output = reviewText
		} else if response, ok := job.Output["response"].(string); ok {
			output = response
		} else if diagnosis, ok := job.Output["diagnosis"].(string); ok {
			output = diagnosis
		}
	}

	success := job.Status == jobs.StatusCompleted

	return &tools.Result{
		Success: success,
		Output:  output,
		Error:   job.Error,
	}, nil
}
