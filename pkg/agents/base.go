package agents

import (
	"context"
	"fmt"
	"os"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/prompts"
	"github.com/soypete/pedrocli/pkg/storage"
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
	name                 string
	description          string
	config               *config.Config
	llm                  llm.Backend
	tools                map[string]tools.Tool
	jobManager           jobs.JobManager
	registry             *tools.ToolRegistry
	toolPromptGen        *prompts.ToolPromptGenerator
	compactionStatsStore storage.CompactionStatsStore // Optional stats tracking

	// Logit bias for controlling token probabilities (set by executor)
	logitBias map[int]float32
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(name, description string, cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *BaseAgent {
	return &BaseAgent{
		name:          name,
		description:   description,
		config:        cfg,
		llm:           backend,
		tools:         make(map[string]tools.Tool),
		jobManager:    jobMgr,
		registry:      nil,
		toolPromptGen: nil,
	}
}

// NewBaseAgentWithRegistry creates a new base agent with a tool registry for dynamic prompts
func NewBaseAgentWithRegistry(name, description string, cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager, registry *tools.ToolRegistry) *BaseAgent {
	agent := NewBaseAgent(name, description, cfg, backend, jobMgr)
	agent.SetRegistry(registry)
	return agent
}

// SetRegistry sets the tool registry and initializes the prompt generator
func (a *BaseAgent) SetRegistry(registry *tools.ToolRegistry) {
	a.registry = registry
	if registry != nil {
		a.toolPromptGen = prompts.NewToolPromptGenerator(registry)
	}
}

// GetRegistry returns the tool registry
func (a *BaseAgent) GetRegistry() *tools.ToolRegistry {
	return a.registry
}

// SetCompactionStatsStore sets the compaction statistics store
func (a *BaseAgent) SetCompactionStatsStore(store storage.CompactionStatsStore) {
	a.compactionStatsStore = store
}

// SetLogitBias sets the logit bias for controlling token probabilities
func (a *BaseAgent) SetLogitBias(bias map[int]float32) {
	a.logitBias = bias
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
	// Use dynamic tool prompt if registry is available
	if a.toolPromptGen != nil {
		return a.buildDynamicSystemPrompt()
	}

	// Fall back to static prompt for backward compatibility
	return a.buildStaticSystemPrompt()
}

// buildDynamicSystemPrompt builds a system prompt with dynamically generated tool descriptions
func (a *BaseAgent) buildDynamicSystemPrompt() string {
	toolSection := a.toolPromptGen.GenerateToolSection()

	return `You are an autonomous coding agent that helps with software engineering tasks. You can execute tools to interact with code, run tests, and make changes.

# Available Tools

` + toolSection + `

# Critical Guidelines

## Read Before Modifying
NEVER modify code you haven't read. Always use the file or code_edit tool to read files before making changes. Understanding existing code prevents introducing bugs.

## Work Incrementally
**IMPORTANT**: Work on ONE file or ONE small change at a time. Do NOT try to implement everything at once.
- Break large tasks into small, incremental steps
- Complete each step fully before moving to the next
- After each change, verify it works before continuing
- If you see "Previous Work Summary" sections, those are older rounds that were compacted - refer to your Progress Checklist below to see what's already been done

## Progress Tracking
**Maintain a Progress Checklist** to track what's been completed across all rounds (including compacted history):

Example Progress Checklist:
## Progress Checklist
- [x] Created pkg/foo/bar.go with Foo struct
- [x] Added tests in pkg/foo/bar_test.go
- [ ] Update main.go to use new Foo struct
- [ ] Run all tests
- [ ] Create PR

**When you see compacted history ("Previous Work Summary" sections):**
1. Check your Progress Checklist to see what's already done
2. Continue from where you left off
3. Update the checklist as you complete each item
4. Keep the checklist visible in every response

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
1. Check your Progress Checklist to see what's already been done
2. Understand the current step fully before starting
3. Search and read relevant code to understand the codebase
4. Make ONE small, targeted change
5. Update your Progress Checklist
6. Verify changes work before moving to next item
7. If tests fail, analyze the failure and iterate until they pass

## Tool Call Format
Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}

When all tasks are complete and tests pass, respond with "TASK_COMPLETE".`
}

// buildStaticSystemPrompt returns the legacy static system prompt (for backward compatibility)
func (a *BaseAgent) buildStaticSystemPrompt() string {
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

	// Calculate total prompt size and record it
	totalPromptTokens := llm.EstimateTokens(systemPrompt) + llm.EstimateTokens(fullPrompt)
	contextMgr.RecordPromptTokens(totalPromptTokens)

	// Log if approaching threshold (for debugging)
	if a.config.Debug.Enabled {
		threshold := int(float64(a.config.Model.ContextSize) * 0.75)
		if totalPromptTokens >= threshold {
			fmt.Fprintf(os.Stderr, "⚠️  Context usage: %d/%d tokens (%.1f%% - threshold exceeded)\n",
				totalPromptTokens, a.config.Model.ContextSize,
				float64(totalPromptTokens)/float64(a.config.Model.ContextSize)*100)
		}
	}

	// Save prompt
	if err := contextMgr.SavePrompt(fullPrompt); err != nil {
		return nil, err
	}

	// Build inference request
	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   fullPrompt,
		Temperature:  a.config.Model.Temperature,
		MaxTokens:    8192,        // Reserve for response
		LogitBias:    a.logitBias, // Apply logit bias for token control
	}

	// Add tools if native tool calling is enabled
	if a.config.Model.EnableTools && a.registry != nil {
		req.Tools = a.convertToolsToDefinitions()
	}

	// Perform inference
	response, err := a.llm.Infer(ctx, req)

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

// convertToolsToDefinitions converts registered tools to LLM tool definitions for native API calling
func (a *BaseAgent) convertToolsToDefinitions() []llm.ToolDefinition {
	if a.registry == nil {
		return nil
	}

	var toolDefs []llm.ToolDefinition
	for _, extTool := range a.registry.List() {
		metadata := extTool.Metadata()

		// Convert JSONSchema to map for API
		schema := make(map[string]interface{})
		if metadata.Schema != nil {
			schema["type"] = "object"
			schema["properties"] = metadata.Schema.Properties
			if len(metadata.Schema.Required) > 0 {
				schema["required"] = metadata.Schema.Required
			}
		}

		toolDefs = append(toolDefs, llm.ToolDefinition{
			Name:        extTool.Name(),
			Description: extTool.Description(),
			Parameters:  schema,
		})
	}

	return toolDefs
}
