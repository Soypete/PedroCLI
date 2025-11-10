package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/context"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
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

// Execute executes the builder agent
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

	// Create context manager
	contextMgr, err := context.NewManager(job.ID, b.config.Debug.Enabled)
	if err != nil {
		b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
		return job, err
	}
	defer contextMgr.Cleanup()

	// Build initial prompt
	userPrompt := b.buildInitialPrompt(input)

	// Execute inference loop (simplified - full implementation would be iterative)
	response, err := b.executeInference(ctx, contextMgr, userPrompt)
	if err != nil {
		b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
		return job, err
	}

	// Parse and execute tool calls (simplified)
	// In full implementation, this would:
	// 1. Parse tool calls from response
	// 2. Execute each tool
	// 3. Feed results back for next inference
	// 4. Repeat until task is complete

	// Update job with results
	output := map[string]interface{}{
		"response": response.Text,
		"status":   "completed",
	}

	b.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)

	return job, nil
}

// buildInitialPrompt builds the initial prompt for the builder
func (b *BuilderAgent) buildInitialPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	prompt := fmt.Sprintf(`Task: Build a new feature

Description: %s

Steps:
1. Understand the requirements
2. Identify files that need to be created or modified
3. Write the implementation
4. Add or update tests
5. Run tests to verify functionality
6. Commit changes to a new branch
7. Create a draft pull request

Begin by analyzing what needs to be done and creating a plan.`, description)

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
