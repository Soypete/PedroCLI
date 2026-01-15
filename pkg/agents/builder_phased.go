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

//go:embed prompts/builder_phased_analyze.md
var builderAnalyzePrompt string

//go:embed prompts/builder_phased_plan.md
var builderPlanPrompt string

//go:embed prompts/builder_phased_implement.md
var builderImplementPrompt string

//go:embed prompts/builder_phased_validate.md
var builderValidatePrompt string

//go:embed prompts/builder_phased_deliver.md
var builderDeliverPrompt string

// BuilderPhasedAgent builds new features using a 5-phase workflow
type BuilderPhasedAgent struct {
	*CodingBaseAgent
	contextTool *tools.ContextTool
}

// NewBuilderPhasedAgent creates a new phased builder agent
func NewBuilderPhasedAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *BuilderPhasedAgent {
	base := NewCodingBaseAgent(
		"builder_phased",
		"Build new features using a structured 5-phase workflow: Analyze, Plan, Implement, Validate, Deliver",
		cfg,
		backend,
		jobMgr,
	)

	contextTool := tools.NewContextTool()
	base.RegisterTool(contextTool)

	return &BuilderPhasedAgent{
		CodingBaseAgent: base,
		contextTool:     contextTool,
	}
}

// GetPhases returns the workflow phases for the builder agent
func (b *BuilderPhasedAgent) GetPhases() []Phase {
	return []Phase{
		{
			Name:         "analyze",
			Description:  "Analyze the request, evaluate repo state, gather requirements",
			SystemPrompt: builderAnalyzePrompt,
			Tools:        []string{"search", "navigate", "file", "git", "github", "lsp", "bash"},
			MaxRounds:    10,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				if result.Data == nil || result.Data["analysis"] == nil {
					// Allow text-based analysis too
					if result.Output == "" {
						return fmt.Errorf("analysis phase produced no output")
					}
				}
				return nil
			},
		},
		{
			Name:         "plan",
			Description:  "Create a detailed implementation plan with numbered steps",
			SystemPrompt: builderPlanPrompt,
			Tools:        []string{"search", "navigate", "file", "context", "bash"},
			MaxRounds:    5,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				if result.Output == "" {
					return fmt.Errorf("plan phase produced no output")
				}
				return nil
			},
		},
		{
			Name:         "implement",
			Description:  "Write code following the plan, chunk by chunk",
			SystemPrompt: builderImplementPrompt,
			Tools:        []string{"file", "code_edit", "search", "navigate", "git", "bash", "lsp", "context"},
			MaxRounds:    30, // More rounds for implementation
			Validator: func(result *PhaseResult) error {
				// Implementation should produce some file modifications
				return nil
			},
		},
		{
			Name:         "validate",
			Description:  "Run tests, linter, verify the implementation works",
			SystemPrompt: builderValidatePrompt,
			Tools:        []string{"test", "bash", "file", "code_edit", "lsp", "search", "navigate"},
			MaxRounds:    15, // Allow iterations to fix failing tests
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "deliver",
			Description:  "Commit changes and create draft PR",
			SystemPrompt: builderDeliverPrompt,
			Tools:        []string{"git", "github"},
			MaxRounds:    5,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
	}
}

// Execute executes the phased builder agent
func (b *BuilderPhasedAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get description
	description, ok := input["description"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'description' in input")
	}

	// Register research_links tool if provided
	if researchLinks, ok := input["research_links"].([]tools.ResearchLink); ok && len(researchLinks) > 0 {
		plainNotes, _ := input["plain_notes"].(string)
		researchLinksTool := tools.NewResearchLinksToolFromLinks(researchLinks, plainNotes)
		b.RegisterTool(researchLinksTool)
	}

	// Register GitHub tool
	githubTool := tools.NewGitHubTool("")
	b.RegisterTool(githubTool)

	// Create job with workflow_type
	job, err := b.jobManager.Create(ctx, "builder_phased", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	b.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the phased workflow in background
	go func() {
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, b.config.Debug.Enabled, b.config.Model.ContextSize)
		if err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Set context_dir
		if err := b.jobManager.SetContextDir(bgCtx, job.ID, contextMgr.GetJobDir()); err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set context_dir: %w", err))
			return
		}

		// Setup work directory
		workDir, err := b.setupWorkDirectory(bgCtx, job.ID, input)
		if err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		if err := b.jobManager.SetWorkDir(bgCtx, job.ID, workDir); err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set work_dir: %w", err))
			return
		}

		// Update tool work directories
		if githubTool, ok := b.tools["github"].(*tools.GitHubTool); ok {
			// Re-register with correct workDir
			b.RegisterTool(tools.NewGitHubTool(workDir))
			_ = githubTool // silence unused warning
		}

		// Build initial prompt
		initialPrompt := b.buildInitialPrompt(input)

		// Create phased executor
		executor := NewPhasedExecutor(b.BaseAgent, contextMgr, b.GetPhases())

		// Execute all phases
		err = executor.Execute(bgCtx, initialPrompt)
		if err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Collect results from all phases
		output := map[string]interface{}{
			"status":        "completed",
			"workflow_type": "builder_phased",
			"job_dir":       contextMgr.GetJobDir(),
			"work_dir":      workDir,
			"phases":        executor.GetAllResults(),
		}

		b.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// buildInitialPrompt builds the initial prompt for the analyze phase
func (b *BuilderPhasedAgent) buildInitialPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	prompt := fmt.Sprintf(`# Task Request

%s

## Instructions

You are starting a 5-phase workflow to implement this request:
1. **Analyze** - Understand requirements, explore codebase, identify affected components
2. **Plan** - Create detailed implementation plan with numbered steps
3. **Implement** - Write code following the plan
4. **Validate** - Run tests, linter, verify implementation
5. **Deliver** - Commit changes and create draft PR

Start with the **Analyze** phase. Explore the codebase to understand:
- What files/components are relevant
- What patterns exist in the codebase
- What dependencies are involved
- What tests need to be considered

Use the available tools to gather information. When you've completed your analysis,
output a summary in JSON format:

{
  "analysis": {
    "summary": "Brief description of what needs to be done",
    "affected_files": ["list", "of", "files"],
    "dependencies": ["any", "dependencies"],
    "risks": ["potential", "risks"],
    "approach": "High-level approach to implement"
  }
}

Then indicate completion with PHASE_COMPLETE.`, description)

	// Add optional context
	if issue, ok := input["issue"].(string); ok && issue != "" {
		prompt += fmt.Sprintf("\n\n## Related Issue\n%s", issue)
	}

	if issueNum, ok := input["issue_number"].(float64); ok {
		prompt += fmt.Sprintf("\n\n## GitHub Issue\nFetch issue #%d for full details using the github tool.", int(issueNum))
	}

	if criteria, ok := input["criteria"].([]interface{}); ok && len(criteria) > 0 {
		prompt += "\n\n## Acceptance Criteria"
		for i, c := range criteria {
			if criterion, ok := c.(string); ok {
				prompt += fmt.Sprintf("\n%d. %s", i+1, criterion)
			}
		}
	}

	return prompt
}
