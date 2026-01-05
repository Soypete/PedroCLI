package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
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
func NewTriagerAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *TriagerAgent {
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

	// Register research_links tool if provided
	if researchLinks, ok := input["research_links"].([]tools.ResearchLink); ok && len(researchLinks) > 0 {
		plainNotes, _ := input["plain_notes"].(string)
		researchLinksTool := tools.NewResearchLinksToolFromLinks(researchLinks, plainNotes)
		t.RegisterTool(researchLinksTool)
	}

	// Create job
	job, err := t.jobManager.Create(ctx, "triage", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	t.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, t.config.Debug.Enabled)
		if err != nil {
			t.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Set context_dir for the job (LLM conversation storage)
		if err := t.jobManager.SetContextDir(bgCtx, job.ID, contextMgr.GetJobDir()); err != nil {
			t.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set context_dir: %w", err))
			return
		}

		// Setup repository if repo info provided, otherwise use current directory
		workDir, err := t.setupWorkDirectory(bgCtx, job.ID, input)
		if err != nil {
			t.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Set work_dir for the job
		if err := t.jobManager.SetWorkDir(bgCtx, job.ID, workDir); err != nil {
			t.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set work_dir: %w", err))
			return
		}

		// Build triage prompt
		userPrompt := t.buildTriagePrompt(input)

		// Create inference executor with coding system prompt
		executor := NewInferenceExecutor(t.BaseAgent, contextMgr)
		executor.SetSystemPrompt(t.buildCodingSystemPrompt())

		// Execute the inference loop
		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			t.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":   "completed",
			"job_dir":  contextMgr.GetJobDir(),
			"work_dir": workDir,
		}

		t.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
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

**IMPORTANT: DO NOT implement any fixes. Your job is diagnosis only.**

## Issue Description
%s

## Investigation Process

### 1. Gather Evidence
- Search for related code using the search tool
- Read error messages and stack traces carefully
- Check git history for recent changes that might be related
- Run tests to see current state of failures

### 2. Reproduce the Issue
- Understand exactly what triggers the problem
- Note any specific conditions required to reproduce

### 3. Trace the Root Cause
- Follow the code path from trigger to failure
- Identify the exact component and lines causing issues
- Distinguish between symptoms and actual root cause

## Required Output Format

### 1. Issue Summary
Brief 1-2 sentence description of the problem and its user impact.

### 2. Severity Assessment
Choose ONE: **critical** | **high** | **medium** | **low** | **info**

Severity Guide:
- **Critical**: System down, data loss, security breach, production outage
- **High**: Major functionality broken, widespread user impact
- **Medium**: Significant issue affecting some users/features
- **Low**: Minor issue with available workarounds
- **Info**: Enhancement, refactoring, or documentation improvement

Explain your reasoning for the severity rating.

### 3. Category
Primary: **bug** | **performance** | **security** | **dependency** | **infrastructure** | **test** | **documentation**

List any secondary categories that apply.

### 4. Root Cause Analysis
- What is the actual root cause (not just symptoms)?
- What chain of events leads to the issue?
- Reference specific files and line numbers

### 5. Affected Components
- List all files, modules, and services affected
- Describe the scope (isolated vs widespread)

### 6. Evidence Collected
- Error messages and stack traces
- Relevant log entries
- Test failures and their output
- Performance metrics if applicable

### 7. Recommended Fix Approaches
Suggest 2-3 approaches with:
- Brief description of each approach
- Complexity estimate (simple/moderate/complex)
- Pros and cons
- Any risks or considerations

### 8. Related Issues & Dependencies
- Related known issues
- Blockers or dependencies for fixing
- Potential regression risks

## Important Instructions
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
