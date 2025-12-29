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
func NewDebuggerAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *DebuggerAgent {
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
	job, err := d.jobManager.Create("debug", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	d.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, d.config.Debug.Enabled)
		if err != nil {
			d.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
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
			d.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}

		d.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)
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

Issue Description: %s

### Your Goals
1. **Analyze Symptoms**: Understand what's wrong by examining error messages, logs, and failing tests
2. **Identify Root Cause**: Trace the issue to its source in the codebase
3. **Develop a Fix**: Create a minimal, targeted fix for the issue
4. **Verify the Fix**: Run tests to ensure the fix works and doesn't break anything else
5. **Document the Solution**: Add comments or documentation if needed

### Debugging Steps
1. Search for relevant files using the search tool
2. Read error messages and stack traces
3. Examine relevant code files
4. Run failing tests to reproduce the issue using the test tool
5. Identify the root cause
6. Implement a fix using code_edit tool
7. Run tests to verify - if they still fail, analyze and fix again
8. Keep iterating until all tests pass
9. Commit the fix with a clear message using git tool

### Important Instructions
- Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}
- If tests fail, don't give up - analyze the failure and try a different approach
- Keep trying until you get it right!
- When the bug is fixed and all tests pass, respond with "TASK_COMPLETE"
- Only indicate completion when you're confident the fix works

Be systematic and thorough. Always verify your fix with tests before committing.`, description)

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
