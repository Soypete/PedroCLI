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
	*BaseAgent
}

// NewBuilderAgent creates a new builder agent
func NewBuilderAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *BuilderAgent {
	base := NewBaseAgent(
		"builder",
		"Build new features autonomously and create draft PRs",
		cfg,
		backend,
		jobMgr,
	)

	return &BuilderAgent{
		BaseAgent: base,
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

		// Create inference executor
		executor := NewInferenceExecutor(b.BaseAgent, contextMgr)

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

	prompt := fmt.Sprintf(`Task: Build a new feature

Description: %s

Steps to complete:
1. Understand the requirements
2. Search for relevant files using the search tool
3. Read necessary files to understand the codebase
4. Write the implementation using code_edit or file tools
5. Add or update tests
6. Run tests using the test tool - if they fail, fix the issues and try again
7. Keep trying until all tests pass
8. Commit changes to a new branch using git tool
9. Create a draft pull request using git tool

IMPORTANT INSTRUCTIONS:
- Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}
- If a tool fails, analyze the error and try again with corrections
- Keep iterating until you get it right - don't give up!
- When ALL steps are complete and tests pass, respond with "TASK_COMPLETE"
- Only indicate completion when you're confident everything works correctly

Begin by analyzing what needs to be done and using the appropriate tools.`, description)

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
