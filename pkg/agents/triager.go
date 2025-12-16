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
	*BaseAgent
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
	base := NewBaseAgent(
		"triager",
		"Diagnose issues and provide triage reports (no fixes)",
		cfg,
		backend,
		jobMgr,
	)

	return &TriagerAgent{
		BaseAgent: base,
	}
}

// Execute executes the triager agent
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
	if err := t.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil); err != nil {
		return job, err
	}

	// Create context manager
	contextMgr, err := llmcontext.NewManager(job.ID, t.config.Debug.Enabled)
	if err != nil {
		_ = t.jobManager.Update(job.ID, jobs.StatusFailed, nil, err) // Ignore error during error handling
		return job, err
	}
	defer func() {
		_ = contextMgr.Cleanup()
	}()

	// Build triage prompt
	userPrompt := t.buildTriagePrompt(input)

	// Execute inference
	response, err := t.executeInference(ctx, contextMgr, userPrompt)
	if err != nil {
		_ = t.jobManager.Update(job.ID, jobs.StatusFailed, nil, err) // Ignore error during error handling
		return job, err
	}

	// Parse triage report from response
	// In a full implementation, this would parse structured data
	triageReport := response.Text

	// Update job with results
	output := map[string]interface{}{
		"triage_report": triageReport,
		"status":        "completed",
	}

	if err := t.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil); err != nil {
		return job, err
	}

	return job, nil
}

// buildTriagePrompt builds the triage prompt
func (t *TriagerAgent) buildTriagePrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	prompt := fmt.Sprintf(`Task: Triage and diagnose an issue (DO NOT FIX)

Issue Description: %s

Your goal is to provide a comprehensive diagnostic report WITHOUT implementing any fixes.

Triage Report Should Include:

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

DO NOT implement any fixes. Your job is only to diagnose and recommend.`, description)

	// Add optional context
	if errorLog, ok := input["error_log"].(string); ok {
		prompt += fmt.Sprintf("\n\nError Log:\n```\n%s\n```", errorLog)
	}

	if stackTrace, ok := input["stack_trace"].(string); ok {
		prompt += fmt.Sprintf("\n\nStack Trace:\n```\n%s\n```", stackTrace)
	}

	if logs, ok := input["logs"].(string); ok {
		prompt += fmt.Sprintf("\n\nRelevant Logs:\n```\n%s\n```", logs)
	}

	if reproduction, ok := input["reproduction_steps"].(string); ok {
		prompt += fmt.Sprintf("\n\nReproduction Steps:\n%s", reproduction)
	}

	return prompt
}

// buildSystemPrompt overrides the base system prompt for triage
func (t *TriagerAgent) buildSystemPrompt() string {
	return `You are an expert issue triager and diagnostician.

Your role is to:
- Analyze issues thoroughly and systematically
- Provide detailed diagnostic reports
- Recommend fix approaches without implementing them
- Assess severity and impact accurately
- Identify root causes, not just symptoms

Triage Principles:
1. **Gather Evidence**: Collect all relevant error messages, logs, and stack traces
2. **Reproduce**: Verify the issue can be reproduced consistently
3. **Isolate**: Narrow down to the exact component and code location
4. **Categorize**: Determine severity, category, and scope
5. **Recommend**: Suggest fix approaches with pros/cons
6. **Document**: Provide a clear, actionable report

Severity Guidelines:
- Critical: System down, data loss, security breach
- High: Major functionality broken, widespread impact
- Medium: Significant issue affecting some users
- Low: Minor issue with workarounds available
- Info: Enhancement, refactoring, or documentation

Available tools:
- file: Read files and search code
- git: Check history, blame, and recent changes
- bash: Run diagnostic commands, check logs
- test: Run tests to understand failures

DO NOT make any changes or fixes. Your job is diagnosis only.`
}
