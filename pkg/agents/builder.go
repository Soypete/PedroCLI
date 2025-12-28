package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// BuilderAgent builds new features autonomously
type BuilderAgent struct {
	*CodingBaseAgent
}

// NewBuilderAgent creates a new builder agent
func NewBuilderAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *BuilderAgent {
	base := NewCodingBaseAgent(
		"builder",
		"Build new features autonomously and create draft PRs",
		cfg,
		backend,
		jobMgr,
	)

	return &BuilderAgent{
		CodingBaseAgent: base,
	}
}

// Execute executes the builder agent asynchronously
func (b *BuilderAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get description
	description, ok := input["description"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'description' in input")
	}

	// Create job
	job, err := b.jobManager.Create("build", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	b.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, b.config.Debug.Enabled)
		if err != nil {
			b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Build initial prompt
		userPrompt := b.buildInitialPrompt(input)

		// Create inference executor with coding system prompt
		executor := NewInferenceExecutor(b.BaseAgent, contextMgr)
		executor.SetSystemPrompt(b.buildCodingSystemPrompt())

		// Execute the inference loop
		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}

		b.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)
	}()

	// Return immediately with the running job
	return job, nil
}

// buildInitialPrompt builds the initial prompt for the builder
func (b *BuilderAgent) buildInitialPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	// Get the builder-specific prompt from the prompt manager
	basePrompt := b.promptMgr.GetPrompt("coding", "builder")

	prompt := basePrompt + fmt.Sprintf(`
## Current Task

## Description
%s

## Implementation Process

### 1. Understand Requirements
- Analyze the feature description thoroughly
- Identify key components and dependencies
- Consider edge cases and error handling needs

### 2. Explore the Codebase
- Use search tool to find related code patterns
- Use navigate tool to understand project structure
- Read existing implementations for similar features
- NEVER modify code without reading it first

### 3. Plan the Implementation
- Identify which files need to be created or modified
- Determine the minimal set of changes needed
- Follow existing code patterns and conventions

### 4. Implement the Feature
- Use code_edit for precise, targeted changes
- Use file tool only for new files or complete rewrites
- Keep changes focused - don't refactor unrelated code
- Match existing code style and conventions

### 5. Add Tests
- Write tests that cover the new functionality
- Include edge cases and error scenarios
- Ensure tests are comprehensive but not excessive

### 6. Verify and Iterate
- Run tests using the test tool
- If tests fail, analyze the failure carefully
- Fix issues and re-run tests until all pass
- Don't give up - keep iterating until success

### 7. Commit Changes
- Create a new branch using git tool
- Commit with a clear, descriptive message
- Create a draft pull request

## Tool Usage
Use JSON format: {"tool": "tool_name", "args": {"key": "value"}}

## Completion
When ALL steps are complete and tests pass, respond with "TASK_COMPLETE".
Only indicate completion when you're confident everything works correctly.

Begin by exploring the codebase to understand where and how to implement the feature.`, description)

	// Add optional context
	if issue, ok := input["issue"].(string); ok {
		prompt += fmt.Sprintf("\n\nRelated issue: %s", issue)
	}

	if criteria, ok := input["criteria"].([]interface{}); ok {
		prompt += "\n\nAcceptance criteria:"
		for i, c := range criteria {
			if criterion, ok := c.(string); ok {
				prompt += fmt.Sprintf("\n%d. %s", i+1, criterion)
			}
		}
	}

	return prompt
}
