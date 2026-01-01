package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// DebuggerAgent debugs and fixes issues autonomously
type DebuggerAgent struct {
	*CodingBaseAgent
}

// NewDebuggerAgent creates a new debugger agent
func NewDebuggerAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *DebuggerAgent {
	base := NewCodingBaseAgent(
		"debugger",
		"Debug and fix issues autonomously",
		cfg,
		backend,
		jobMgr,
	)

	return &DebuggerAgent{
		CodingBaseAgent: base,
	}
}

// Execute executes the debugger agent asynchronously
func (d *DebuggerAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get issue description
	description, ok := input["description"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'description' in input")
	}

	// Create job
	job, err := d.jobManager.Create(ctx, "debug", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	d.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, d.config.Debug.Enabled)
		if err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Build debugging prompt
		userPrompt := d.buildDebugPrompt(input)

		// Create inference executor with coding system prompt
		executor := NewInferenceExecutor(d.BaseAgent, contextMgr)
		executor.SetSystemPrompt(d.buildCodingSystemPrompt())

		// Execute the inference loop
		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}

		d.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	// Return immediately with the running job
	return job, nil
}

// buildDebugPrompt builds the debugging prompt
func (d *DebuggerAgent) buildDebugPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	// Get the debugger-specific prompt from the prompt manager
	basePrompt := d.promptMgr.GetPrompt("coding", "debugger")

	prompt := basePrompt + fmt.Sprintf(`
## Current Task

## Issue Description
%s

## Debugging Process

### 1. Reproduce the Issue
- Run the failing test or trigger the error condition
- Confirm you can consistently reproduce the problem
- Note the exact error messages and behavior

### 2. Gather Evidence
- Read error messages and stack traces carefully - they often point to the exact location
- Use search tool to find related code
- Use git tool to check recent changes that might have introduced the bug
- NEVER modify code without reading it first

### 3. Narrow Down the Root Cause
- Use binary search approach: isolate which component is failing
- Read the relevant code to understand the logic flow
- Identify the exact line(s) causing the issue
- Distinguish between symptoms and root cause

### 4. Develop a Fix
- Create a minimal, targeted fix - only change what's necessary
- Don't fix multiple unrelated issues at once
- Don't refactor surrounding code
- Follow existing code patterns and style

### 5. Verify the Fix
- Run tests to confirm the fix works
- Check that no new failures were introduced
- If tests still fail, analyze why and iterate
- Keep trying until all tests pass

### 6. Commit the Fix
- Write a clear commit message explaining what was fixed and why
- Reference any issue numbers if applicable

## Debugging Principles
- **Read Error Messages Carefully**: They usually contain the exact location and cause
- **Check Recent Changes**: Bugs often come from recent modifications (use git diff/log)
- **Fix One Thing at a Time**: Don't attempt multiple fixes simultaneously
- **Test Before and After**: Always verify the fix with tests

## Tool Usage
Use JSON format: {"tool": "tool_name", "args": {"key": "value"}}

## Completion
When the bug is fixed and all tests pass, respond with "TASK_COMPLETE".
Only indicate completion when you're confident the fix works correctly.

Begin by reproducing the issue and gathering evidence about the root cause.`, description)

	// Add optional context
	if errorLog, ok := input["error_log"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Error Log\n```\n%s\n```", errorLog)
	}

	if stackTrace, ok := input["stack_trace"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Stack Trace\n```\n%s\n```", stackTrace)
	}

	if failingTest, ok := input["failing_test"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Failing Test\n%s", failingTest)
	}

	if reproduction, ok := input["reproduction_steps"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Reproduction Steps\n%s", reproduction)
	}

	return prompt
}
