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

// BuilderAgent builds new features autonomously
type BuilderAgent struct {
	*CodingBaseAgent
}

// NewBuilderAgent creates a new builder agent
func NewBuilderAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *BuilderAgent {
	base := NewCodingBaseAgent(
		"builder",
		"Build new features autonomously and create draft PRs",
		cfg,
		backend,
		jobMgr,
	)

	return &BuilderAgent{
		CodingBaseAgent: base,
	}
}

// Execute executes the builder agent asynchronously
func (b *BuilderAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
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

	// Create job
	job, err := b.jobManager.Create(ctx, "build", description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	b.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the inference loop in background with its own context
	go func() {
		// Use background context so it doesn't get cancelled when Execute() returns
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, b.config.Debug.Enabled, b.config.Model.ContextSize)
		if err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Configure context manager with model and stats tracking
		contextMgr.SetModelName(b.config.Model.ModelName)
		if b.compactionStatsStore != nil {
			contextMgr.SetStatsStore(b.compactionStatsStore)
		}

		// Set context_dir for the job (LLM conversation storage)
		if err := b.jobManager.SetContextDir(bgCtx, job.ID, contextMgr.GetJobDir()); err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set context_dir: %w", err))
			return
		}

		// Setup repository if repo info provided, otherwise use current directory
		workDir, err := b.setupWorkDirectory(bgCtx, job.ID, input)
		if err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Set work_dir for the job
		if err := b.jobManager.SetWorkDir(bgCtx, job.ID, workDir); err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set work_dir: %w", err))
			return
		}

		// Build initial prompt
		userPrompt := b.buildInitialPrompt(input)

		// Create inference executor with coding system prompt
		executor := NewInferenceExecutor(b.BaseAgent, contextMgr)
		executor.SetSystemPrompt(b.buildCodingSystemPrompt())

		// Execute the inference loop
		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":   "completed",
			"job_dir":  contextMgr.GetJobDir(),
			"work_dir": workDir,
		}

		b.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	// Return immediately with the running job
	return job, nil
}

// buildInitialPrompt builds the initial prompt for the builder
func (b *BuilderAgent) buildInitialPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	// Get the builder-specific prompt from the prompt manager
	basePrompt := b.promptMgr.GetPrompt("coding", "builder")

	prompt := basePrompt + fmt.Sprintf(`

## Task
%s

Start by quickly finding the relevant files, then immediately begin writing code.`, description)

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
