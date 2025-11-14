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
	*BaseAgent
}

// NewDebuggerAgent creates a new debugger agent
func NewDebuggerAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *DebuggerAgent {
	base := NewBaseAgent(
		"debugger",
		"Debug and fix issues autonomously",
		cfg,
		backend,
		jobMgr,
	)

	return &DebuggerAgent{
		BaseAgent: base,
	}
}

// Execute executes the debugger agent
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

	// Create context manager
	contextMgr, err := llmcontext.NewManager(job.ID, d.config.Debug.Enabled)
	if err != nil {
		d.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
		return job, err
	}
	defer contextMgr.Cleanup()

	// Build debugging prompt
	userPrompt := d.buildDebugPrompt(input)

	// Execute inference loop (simplified - full implementation would be iterative)
	response, err := d.executeInference(ctx, contextMgr, userPrompt)
	if err != nil {
		d.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
		return job, err
	}

	// In a full implementation, this would:
	// 1. Analyze symptoms and identify root cause
	// 2. Parse tool calls from response
	// 3. Execute diagnostic tools (read logs, run failing tests, etc.)
	// 4. Apply the fix
	// 5. Run tests to verify the fix
	// 6. Commit changes if tests pass
	// 7. Repeat until issue is resolved or max iterations reached

	// Update job with results
	output := map[string]interface{}{
		"response": response.Text,
		"status":   "completed",
	}

	d.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)

	return job, nil
}

// buildDebugPrompt builds the debugging prompt
func (d *DebuggerAgent) buildDebugPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	prompt := fmt.Sprintf(`Task: Debug and fix an issue

Issue Description: %s

Your goal is to:
1. **Analyze Symptoms**: Understand what's wrong by examining error messages, logs, and failing tests
2. **Identify Root Cause**: Trace the issue to its source in the codebase
3. **Develop a Fix**: Create a minimal, targeted fix for the issue
4. **Verify the Fix**: Run tests to ensure the fix works and doesn't break anything else
5. **Document the Solution**: Add comments or documentation if needed

Debugging Steps:
1. Read error messages and stack traces
2. Examine relevant code files
3. Run failing tests to reproduce the issue
4. Identify the root cause
5. Implement a fix
6. Run tests to verify
7. Commit the fix with a clear message

Be systematic and thorough. Always verify your fix with tests before committing.`, description)

	// Add optional context
	if errorLog, ok := input["error_log"].(string); ok {
		prompt += fmt.Sprintf("\n\nError Log:\n```\n%s\n```", errorLog)
	}

	if stackTrace, ok := input["stack_trace"].(string); ok {
		prompt += fmt.Sprintf("\n\nStack Trace:\n```\n%s\n```", stackTrace)
	}

	if failingTest, ok := input["failing_test"].(string); ok {
		prompt += fmt.Sprintf("\n\nFailing Test: %s", failingTest)
	}

	if reproduction, ok := input["reproduction_steps"].(string); ok {
		prompt += fmt.Sprintf("\n\nReproduction Steps:\n%s", reproduction)
	}

	return prompt
}

// buildSystemPrompt overrides the base system prompt for debugging
func (d *DebuggerAgent) buildSystemPrompt() string {
	return `You are an expert debugging agent.

Your role is to:
- Systematically diagnose code issues
- Identify root causes, not just symptoms
- Apply minimal, targeted fixes
- Verify fixes with tests
- Avoid introducing new bugs

Debugging Principles:
1. **Reproduce First**: Always reproduce the issue before attempting a fix
2. **Narrow Down**: Use binary search and isolation to find the exact cause
3. **Read Error Messages Carefully**: They often contain the exact location and cause
4. **Check Recent Changes**: Bugs often come from recent modifications
5. **Test Thoroughly**: Verify the fix works and doesn't break anything else
6. **Fix One Thing**: Don't fix multiple unrelated issues in one change

Available tools:
- file: Read, write, and modify files
- git: Check git history, diff changes, create branches
- bash: Run commands, check logs
- test: Run tests and parse results

Always run tests before and after your fix to verify the solution.`
}
