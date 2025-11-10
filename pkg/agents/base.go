package agents

import (
	"context"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

// Agent represents a coding agent
type Agent interface {
	// Name returns the agent name
	Name() string

	// Description returns the agent description
	Description() string

	// Execute executes the agent's task
	Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error)
}

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	name        string
	description string
	config      *config.Config
	llm         llm.Backend
	tools       map[string]tools.Tool
	jobManager  *jobs.Manager
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(name, description string, cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *BaseAgent {
	return &BaseAgent{
		name:        name,
		description: description,
		config:      cfg,
		llm:         backend,
		tools:       make(map[string]tools.Tool),
		jobManager:  jobMgr,
	}
}

// Name returns the agent name
func (a *BaseAgent) Name() string {
	return a.name
}

// Description returns the agent description
func (a *BaseAgent) Description() string {
	return a.description
}

// RegisterTool registers a tool with the agent
func (a *BaseAgent) RegisterTool(tool tools.Tool) {
	a.tools[tool.Name()] = tool
}

// buildSystemPrompt builds the system prompt for the agent
func (a *BaseAgent) buildSystemPrompt() string {
	return `You are an autonomous coding agent. You can execute tools to interact with code, run tests, and make changes.

Available tools:
- file: Read, write, and modify files
- git: Execute git commands
- bash: Run safe shell commands
- test: Run tests and parse results

Always think step-by-step and verify your changes with tests before committing.`
}

// executeInference performs one-shot inference
func (a *BaseAgent) executeInference(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string) (*llm.InferenceResponse, error) {
	// Build system prompt
	systemPrompt := a.buildSystemPrompt()

	// Calculate context budget
	budget := llm.CalculateBudget(a.config, systemPrompt, userPrompt, "")

	// Get history within budget
	history, err := contextMgr.GetHistoryWithinBudget(budget.Available)
	if err != nil {
		return nil, err
	}

	// Build full prompt with history
	fullPrompt := userPrompt
	if history != "" {
		fullPrompt = history + "\n\n" + userPrompt
	}

	// Save prompt
	if err := contextMgr.SavePrompt(fullPrompt); err != nil {
		return nil, err
	}

	// Perform inference
	response, err := a.llm.Infer(ctx, &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   fullPrompt,
		Temperature:  a.config.Model.Temperature,
		MaxTokens:    8192, // Reserve for response
	})

	if err != nil {
		return nil, err
	}

	// Save response
	if err := contextMgr.SaveResponse(response.Text); err != nil {
		return nil, err
	}

	return response, nil
}

// executeTool executes a tool
func (a *BaseAgent) executeTool(ctx context.Context, name string, args map[string]interface{}) (*tools.Result, error) {
	tool, ok := a.tools[name]
	if !ok {
		return &tools.Result{
			Success: false,
			Error:   "tool not found: " + name,
		}, nil
	}

	return tool.Execute(ctx, args)
}
