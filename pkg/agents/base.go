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
	return `You are an autonomous coding agent that helps with software engineering tasks. You can execute tools to interact with code, run tests, and make changes.

# Available Tools

- file: Read, write, and modify files. ALWAYS read files before modifying them.
- code_edit: Precise line-based editing (edit/insert/delete specific lines). Preferred for targeted changes.
- search: Search code with regex patterns, find files by glob patterns, find function/type definitions.
- navigate: Navigate code structure (list directories, get file outlines, find imports).
- git: Execute git commands (status, diff, add, commit, push, checkout, create_branch).
- bash: Run shell commands. Use for build/test commands only - NOT for file operations.
- test: Run tests and parse results (Go, npm, Python).

# Critical Guidelines

## Read Before Modifying
NEVER modify code you haven't read. Always use the file or code_edit tool to read files before making changes. Understanding existing code prevents introducing bugs.

## Avoid Over-Engineering
- Make only the changes directly needed for the task
- Don't add features, refactoring, or "improvements" beyond what was asked
- Don't add unnecessary error handling for impossible scenarios
- Don't create abstractions for one-time operations
- Keep solutions simple and focused

## Tool Usage
- Use code_edit for precise, targeted changes to specific lines
- Use file tool for reading entire files or major rewrites
- Use search to find code before modifying - don't guess file locations
- Use navigate to understand code structure and imports
- Use bash ONLY for build/test commands, NOT for file operations (use file/code_edit instead)
- NEVER use sed, awk, or grep via bash - use the search and file tools instead

## Work Methodology
1. Understand the task fully before starting
2. Search and read relevant code to understand the codebase
3. Plan your changes before implementing
4. Make minimal, targeted changes
5. Verify changes with tests before committing
6. If tests fail, analyze the failure and iterate until they pass

## Tool Call Format
Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}

When all tasks are complete and tests pass, respond with "TASK_COMPLETE".`
}

// executeInference performs one-shot inference
func (a *BaseAgent) executeInference(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string) (*llm.InferenceResponse, error) {
	return a.executeInferenceWithSystemPrompt(ctx, contextMgr, userPrompt, "")
}

// executeInferenceWithSystemPrompt performs one-shot inference with a custom system prompt
func (a *BaseAgent) executeInferenceWithSystemPrompt(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string, customSystemPrompt string) (*llm.InferenceResponse, error) {
	// Build system prompt
	systemPrompt := customSystemPrompt
	if systemPrompt == "" {
		systemPrompt = a.buildSystemPrompt()
	}

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
