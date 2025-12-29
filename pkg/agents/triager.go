package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// TriagerAgent diagnoses issues without fixing them
type TriagerAgent struct {
	*CodingBaseAgent
}

// IssueSeverity represents the severity of an issue
type IssueSeverity string

const (
	IssueSeverityCritical IssueSeverity = "critical"
	IssueSeverityHigh     IssueSeverity = "high"
	IssueSeverityMedium   IssueSeverity = "medium"
	IssueSeverityLow      IssueSeverity = "low"
	IssueSeverityInfo     IssueSeverity = "info"
)

// IssueCategory represents the category of an issue
type IssueCategory string

const (
	IssueCategoryBug         IssueCategory = "bug"
	IssueCategoryPerformance IssueCategory = "performance"
	IssueCategorySecurity    IssueCategory = "security"
	IssueCategoryDependency  IssueCategory = "dependency"
	IssueCategoryInfra       IssueCategory = "infrastructure"
	IssueCategoryTest        IssueCategory = "test"
	IssueCategoryDoc         IssueCategory = "documentation"
)

// NewTriagerAgent creates a new triager agent
func NewTriagerAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *TriagerAgent {
	base := NewCodingBaseAgent(
		"triager",
		"Diagnose issues and provide triage reports (no fixes)",
		cfg,
		backend,
		jobMgr,
	)

	return &TriagerAgent{
		CodingBaseAgent: base,
	}
}

// Execute executes the triager agent asynchronously
func (t *TriagerAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get issue description
	description, ok := input["description"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'description' in input")
	}

	// Create job
	job, err := t.jobManager.Create("triage", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	t.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, t.config.Debug.Enabled)
		if err != nil {
			t.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Build triage prompt
		userPrompt := t.buildTriagePrompt(input)

		// Create inference executor with coding system prompt
		executor := NewInferenceExecutor(t.BaseAgent, contextMgr)
		executor.SetSystemPrompt(t.buildCodingSystemPrompt())

		// Execute the inference loop
		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			t.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"job_dir": contextMgr.GetJobDir(),
			"status":  "completed",
		}

		t.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)
	}()

	// Return immediately with the running job
	return job, nil
}

// buildTriagePrompt builds the triage prompt
func (t *TriagerAgent) buildTriagePrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	// Get the triager-specific prompt from the prompt manager
	basePrompt := t.promptMgr.GetPrompt("coding", "triager")

	prompt := basePrompt + fmt.Sprintf(`
## Current Task

Issue Description: %s

Your goal is to provide a comprehensive diagnostic report WITHOUT implementing any fixes.

### Triage Report Should Include

## 1. Issue Summary
- Brief description of the problem
- Impact and affected components

## 2. Severity Assessment
Choose one: critical, high, medium, low, info
- Explain your severity rating

## 3. Category
Choose primary category: bug, performance, security, dependency, infrastructure, test, documentation
- List any secondary categories

## 4. Root Cause Analysis
- Identify the likely root cause
- Explain the chain of events leading to the issue
- Reference specific files and line numbers if possible

## 5. Affected Components
- List all affected files, modules, or services
- Describe the scope of the issue

## 6. Diagnostic Evidence
- Error messages and stack traces
- Relevant log entries
- Test failures
- Performance metrics (if applicable)

## 7. Recommended Fix Approach
- Suggest 2-3 possible approaches to fix the issue
- Estimate complexity (simple, moderate, complex)
- List any risks or considerations

## 8. Related Issues
- Check if this is related to other known issues
- Identify any blockers or dependencies

### Important Instructions
- Use tools to investigate: search, file, git, test, bash
- Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}
- DO NOT implement any fixes - your job is only to diagnose and recommend
- When you have completed your triage report, respond with "TASK_COMPLETE"`, description)

	// Add optional context
	if errorLog, ok := input["error_log"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Error Log\n```\n%s\n```", errorLog)
	}

	if stackTrace, ok := input["stack_trace"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Stack Trace\n```\n%s\n```", stackTrace)
	}

	if logs, ok := input["logs"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Relevant Logs\n```\n%s\n```", logs)
	}

	if reproduction, ok := input["reproduction_steps"].(string); ok {
		prompt += fmt.Sprintf("\n\n### Reproduction Steps\n%s", reproduction)
	}

	return prompt
}
