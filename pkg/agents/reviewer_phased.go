package agents

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

//go:embed prompts/reviewer_phased_gather.md
var reviewerGatherPrompt string

//go:embed prompts/reviewer_phased_security.md
var reviewerSecurityPrompt string

//go:embed prompts/reviewer_phased_quality.md
var reviewerQualityPrompt string

//go:embed prompts/reviewer_phased_compile.md
var reviewerCompilePrompt string

//go:embed prompts/reviewer_phased_publish.md
var reviewerPublishPrompt string

// ReviewerPhasedAgent performs code review using a 5-phase workflow
type ReviewerPhasedAgent struct {
	*CodingBaseAgent
	contextTool *tools.ContextTool
}

// NewReviewerPhasedAgent creates a new phased reviewer agent
func NewReviewerPhasedAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *ReviewerPhasedAgent {
	base := NewCodingBaseAgent(
		"reviewer_phased",
		"Review code using a structured 5-phase workflow: Gather, Security, Quality, Compile, Publish",
		cfg,
		backend,
		jobMgr,
	)

	contextTool := tools.NewContextTool()
	base.RegisterTool(contextTool)

	return &ReviewerPhasedAgent{
		CodingBaseAgent: base,
		contextTool:     contextTool,
	}
}

// GetPhases returns the workflow phases for the reviewer agent
func (r *ReviewerPhasedAgent) GetPhases() []Phase {
	return []Phase{
		{
			Name:         "gather",
			Description:  "Fetch PR details, checkout branch, get diff and LSP diagnostics",
			SystemPrompt: reviewerGatherPrompt,
			Tools:        []string{"github", "git", "lsp", "search", "navigate", "file"},
			MaxRounds:    10,
			ExpectsJSON:  false,
			Validator: func(result *PhaseResult) error {
				if result.Output == "" {
					return fmt.Errorf("gather phase produced no output")
				}
				return nil
			},
		},
		{
			Name:         "security",
			Description:  "Analyze for security vulnerabilities and issues",
			SystemPrompt: reviewerSecurityPrompt,
			Tools:        []string{"search", "file", "lsp", "context"},
			MaxRounds:    8,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "quality",
			Description:  "Review code quality, performance, and maintainability",
			SystemPrompt: reviewerQualityPrompt,
			Tools:        []string{"search", "file", "lsp", "navigate", "context"},
			MaxRounds:    10,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "compile",
			Description:  "Compile all findings into structured review",
			SystemPrompt: reviewerCompilePrompt,
			Tools:        []string{"context"},
			MaxRounds:    5,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				if result.Output == "" {
					return fmt.Errorf("compile phase produced no review")
				}
				return nil
			},
		},
		{
			Name:         "publish",
			Description:  "Post review to GitHub (optional)",
			SystemPrompt: reviewerPublishPrompt,
			Tools:        []string{"github"},
			MaxRounds:    3,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
	}
}

// Execute executes the phased reviewer agent
func (r *ReviewerPhasedAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get PR or branch reference
	var description string
	if prNum, ok := input["pr_number"].(float64); ok {
		description = fmt.Sprintf("Review PR #%d", int(prNum))
	} else if branch, ok := input["branch"].(string); ok {
		description = fmt.Sprintf("Review branch: %s", branch)
	} else {
		return nil, fmt.Errorf("missing 'pr_number' or 'branch' in input")
	}

	// Register GitHub tool
	githubTool := tools.NewGitHubTool("")
	r.RegisterTool(githubTool)

	// Create job
	job, err := r.jobManager.Create(ctx, "reviewer_phased", description, input)
	if err != nil {
		return nil, err
	}

	r.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the phased workflow in background
	go func() {
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, r.config.Debug.Enabled, r.config.Model.ContextSize)
		if err != nil {
			r.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		if err := r.jobManager.SetContextDir(bgCtx, job.ID, contextMgr.GetJobDir()); err != nil {
			r.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set context_dir: %w", err))
			return
		}

		workDir, err := r.setupWorkDirectory(bgCtx, job.ID, input)
		if err != nil {
			r.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		if err := r.jobManager.SetWorkDir(bgCtx, job.ID, workDir); err != nil {
			r.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set work_dir: %w", err))
			return
		}

		// Update GitHub tool with correct workDir
		r.RegisterTool(tools.NewGitHubTool(workDir))

		// Build initial prompt
		initialPrompt := r.buildInitialPrompt(input)

		// Create phased executor
		executor := NewPhasedExecutor(r.BaseAgent, contextMgr, r.GetPhases())

		// Execute all phases
		err = executor.Execute(bgCtx, initialPrompt)
		if err != nil {
			r.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Collect results
		output := map[string]interface{}{
			"status":        "completed",
			"workflow_type": "reviewer_phased",
			"job_dir":       contextMgr.GetJobDir(),
			"work_dir":      workDir,
			"phases":        executor.GetAllResults(),
		}

		// Extract recommendation from compile phase
		if compileResult, ok := executor.GetPhaseResult("compile"); ok && compileResult.Data != nil {
			if recommendation, ok := compileResult.Data["recommendation"]; ok {
				output["recommendation"] = recommendation
			}
		}

		r.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// buildInitialPrompt builds the initial prompt for the gather phase
func (r *ReviewerPhasedAgent) buildInitialPrompt(input map[string]interface{}) string {
	var prompt string

	if prNum, ok := input["pr_number"].(float64); ok {
		prompt = fmt.Sprintf(`# Code Review Request

Review PR #%d

## Instructions

You are starting a 5-phase code review workflow:
1. **Gather** - Fetch PR details, checkout branch, get diff and diagnostics
2. **Security** - Analyze for security vulnerabilities
3. **Quality** - Review code quality, performance, maintainability
4. **Compile** - Compile findings into structured review
5. **Publish** - Post review to GitHub (optional)

Start with the **Gather** phase:
1. Use the github tool to fetch PR details: {"tool": "github", "args": {"action": "pr_fetch", "pr_number": %d, "include_diff": true}}
2. Checkout the PR branch: {"tool": "github", "args": {"action": "pr_checkout", "pr_number": %d}}
3. Run LSP diagnostics on changed files
4. Read key changed files to understand the changes

When you've gathered all necessary information, summarize what you found and say PHASE_COMPLETE.`, int(prNum), int(prNum), int(prNum))
	} else if branch, ok := input["branch"].(string); ok {
		prompt = fmt.Sprintf(`# Code Review Request

Review branch: %s

## Instructions

You are starting a 5-phase code review workflow:
1. **Gather** - Get diff, checkout branch, analyze changes
2. **Security** - Analyze for security vulnerabilities
3. **Quality** - Review code quality, performance, maintainability
4. **Compile** - Compile findings into structured review
5. **Publish** - Post review to GitHub (optional)

Start with the **Gather** phase:
1. Use git to get the diff: {"tool": "git", "args": {"action": "diff", "base": "main", "branch": "%s"}}
2. Checkout the branch: {"tool": "git", "args": {"action": "checkout", "branch": "%s"}}
3. Run LSP diagnostics on changed files
4. Read key changed files to understand the changes

When you've gathered all necessary information, summarize what you found and say PHASE_COMPLETE.`, branch, branch, branch)
	}

	// Add focus areas if specified
	if focus, ok := input["focus"].([]interface{}); ok && len(focus) > 0 {
		prompt += "\n\n## Focus Areas"
		for _, f := range focus {
			if area, ok := f.(string); ok {
				prompt += fmt.Sprintf("\n- %s", area)
			}
		}
	}

	return prompt
}
