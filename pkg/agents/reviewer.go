package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

// ReviewerAgent performs code review on PRs
type ReviewerAgent struct {
	*CodingBaseAgent
}

// ReviewSeverity represents the severity of a review finding
type ReviewSeverity string

const (
	SeverityCritical   ReviewSeverity = "critical"
	SeverityWarning    ReviewSeverity = "warning"
	SeveritySuggestion ReviewSeverity = "suggestion"
	SeverityNit        ReviewSeverity = "nit"
)

// ReviewIssue represents a single review finding
type ReviewIssue struct {
	Severity    ReviewSeverity `json:"severity"`
	File        string         `json:"file"`
	Line        int            `json:"line,omitempty"`
	Category    string         `json:"category"` // "bug", "security", "performance", "style", "test"
	Description string         `json:"description"`
	Suggestion  string         `json:"suggestion,omitempty"`
}

// NewReviewerAgent creates a new reviewer agent
func NewReviewerAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *ReviewerAgent {
	base := NewCodingBaseAgent(
		"reviewer",
		"Perform code review on PRs (blind review - unaware of AI authorship)",
		cfg,
		backend,
		jobMgr,
	)

	return &ReviewerAgent{
		CodingBaseAgent: base,
	}
}

// Execute executes the reviewer agent asynchronously
func (r *ReviewerAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get branch name
	branch, ok := input["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'branch' in input")
	}

	// Create job
	description := fmt.Sprintf("Review code on branch: %s", branch)
	job, err := r.jobManager.Create("review", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	r.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, r.config.Debug.Enabled)
		if err != nil {
			r.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Get git diff for the branch
		diff, err := r.getGitDiff(bgCtx, branch)
		if err != nil {
			r.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Build review prompt
		userPrompt := r.buildReviewPrompt(branch, diff, input)

		// Create inference executor with coding system prompt
		executor := NewInferenceExecutor(r.BaseAgent, contextMgr)
		executor.SetSystemPrompt(r.buildCodingSystemPrompt())

		// Execute the inference loop
		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			r.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"job_dir": contextMgr.GetJobDir(),
			"branch":  branch,
			"status":  "completed",
		}

		r.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)
	}()

	// Return immediately with the running job
	return job, nil
}

// buildReviewPrompt builds the review prompt
func (r *ReviewerAgent) buildReviewPrompt(branch, diff string, input map[string]interface{}) string {
	// Get the reviewer-specific prompt from the prompt manager
	basePrompt := r.promptMgr.GetPrompt("coding", "reviewer")

	prompt := basePrompt + fmt.Sprintf(`
## Current Task

You are reviewing a pull request. Analyze the code changes carefully and provide constructive feedback.

Branch: %s

### Code Changes
%s

### Review Criteria
1. **Code Quality**: Is the code well-structured, readable, and maintainable?
2. **Bugs**: Are there any potential bugs or logical errors?
3. **Security**: Are there any security vulnerabilities (SQL injection, XSS, etc.)?
4. **Performance**: Are there any performance concerns or inefficiencies?
5. **Testing**: Are there adequate tests? Do they cover edge cases?
6. **Best Practices**: Does the code follow language/framework best practices?
7. **Documentation**: Is the code well-documented?

### Output Format

Provide your review in the following format:

## Summary
[Brief overview of the changes and overall assessment]

## Issues Found

### Critical Issues
[List critical issues that must be fixed before merging]

### Warnings
[List important issues that should be addressed]

### Suggestions
[List optional improvements and best practices]

## Positive Feedback
[Highlight what was done well]

## Recommendation
[APPROVE / REQUEST_CHANGES / COMMENT]

Be constructive, specific, and helpful. Reference file names and line numbers when possible.

When you have completed the review, respond with "TASK_COMPLETE".`, branch, diff)

	return prompt
}

// getGitDiff gets the git diff for a branch
func (r *ReviewerAgent) getGitDiff(ctx context.Context, branch string) (string, error) {
	// Use the git tool to get the diff
	gitTool, ok := r.tools["git"]
	if !ok {
		return "", fmt.Errorf("git tool not available")
	}

	// Get diff against main/master
	result, err := gitTool.Execute(ctx, map[string]interface{}{
		"action": "diff",
		"base":   "main", // Could be configurable
		"branch": branch,
	})

	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("git diff failed: %s", result.Error)
	}

	return result.Output, nil
}

// parseReview parses the review text and extracts structured information
func (r *ReviewerAgent) parseReview(reviewText string) string {
	// For now, return the raw review text
	// In a more sophisticated implementation, we could parse this into ReviewIssue structs
	return reviewText
}

// extractIssueCount extracts the number of issues from the review
func (r *ReviewerAgent) extractIssueCount(review string) int {
	// Simple heuristic: count sections
	count := 0
	if strings.Contains(review, "Critical Issues") {
		count += strings.Count(review, "\n- ") // Count bullet points
	}
	return count
}

// postReviewToGitHub posts the review to GitHub using gh CLI
func (r *ReviewerAgent) postReviewToGitHub(ctx context.Context, branch, review string) error {
	// Get PR number from branch
	prNumber, ok := r.getPRNumber(ctx, branch)
	if !ok {
		return fmt.Errorf("no PR found for branch %s", branch)
	}

	// Use gh CLI to post review
	bashTool, ok := r.tools["bash"]
	if !ok {
		return fmt.Errorf("bash tool not available")
	}

	// Post review comment
	command := fmt.Sprintf("gh pr review %s --comment --body '%s'", prNumber, review)
	result, err := bashTool.Execute(ctx, map[string]interface{}{
		"command": command,
	})

	if err != nil || !result.Success {
		return fmt.Errorf("failed to post review: %v", err)
	}

	return nil
}

// getPRNumber gets the PR number for a branch
func (r *ReviewerAgent) getPRNumber(ctx context.Context, branch string) (string, bool) {
	bashTool, ok := r.tools["bash"]
	if !ok {
		return "", false
	}

	// Query gh for PR number
	command := fmt.Sprintf("gh pr list --head %s --json number --jq '.[0].number'", branch)
	result, err := bashTool.Execute(ctx, map[string]interface{}{
		"command": command,
	})

	if err != nil || !result.Success || result.Output == "" {
		return "", false
	}

	return strings.TrimSpace(result.Output), true
}
