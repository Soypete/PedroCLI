package agents

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

//go:embed prompts/blog_editor_system.md
var blogEditorSystemPrompt string

// EditorAgent reviews and refines blog posts
type EditorAgent struct {
	*BaseAgent
	autoRevise bool // If true, auto-revise; if false, just review
}

// NewEditorAgent creates a new editor agent
func NewEditorAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager, autoRevise bool) *EditorAgent {
	base := NewBaseAgent(
		"editor",
		"Review and refine blog posts for quality and coherence",
		cfg,
		backend,
		jobMgr,
	)

	return &EditorAgent{
		BaseAgent:  base,
		autoRevise: autoRevise,
	}
}

// Execute executes the editor agent asynchronously
func (e *EditorAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get draft content
	draft, ok := input["draft"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'draft' in input")
	}

	// Get optional original transcription for context
	transcription, _ := input["original_transcription"].(string)

	// Get post title for job description
	postTitle, _ := input["title"].(string)
	if postTitle == "" {
		postTitle = "Untitled Post"
	}

	// Create job
	jobType := "blog_editor_review"
	if e.autoRevise {
		jobType = "blog_editor_revise"
	}

	job, err := e.jobManager.Create(ctx, jobType, "Edit blog post: "+postTitle, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	e.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the editing process in background
	go func() {
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, e.config.Debug.Enabled, e.config.Model.ContextSize)
		if err != nil {
			e.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Configure context manager with model and stats tracking
		contextMgr.SetModelName(e.config.Model.ModelName)
		if e.compactionStatsStore != nil {
			contextMgr.SetStatsStore(e.compactionStatsStore)
		}

		// Build prompt
		userPrompt := e.buildEditingPrompt(draft, transcription, input)

		// Write to context
		if err := contextMgr.SavePrompt(userPrompt); err != nil {
			e.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Execute inference
		result, err := e.executeEditing(bgCtx, contextMgr, userPrompt)
		if err != nil {
			e.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Write response
		if err := contextMgr.SaveResponse(result.Text); err != nil {
			e.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":      "completed",
			"content":     result.Text,
			"auto_revise": e.autoRevise,
			"job_dir":     contextMgr.GetJobDir(),
		}

		e.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// executeEditing performs the editing task
func (e *EditorAgent) executeEditing(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string) (*llm.InferenceResponse, error) {
	// Build system prompt from embedded file
	systemPrompt := blogEditorSystemPrompt

	// Add mode instruction
	if e.autoRevise {
		systemPrompt += "\n\n**MODE: Auto-Revise** - Make automatic improvements and return the fully revised post."
	} else {
		systemPrompt += "\n\n**MODE: Review** - Provide structured feedback without making automatic changes."
	}

	// Calculate context budget for max tokens
	budget := llm.CalculateBudget(e.config, systemPrompt, userPrompt, "")

	// Generate with LLM using Infer
	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.7, // Higher temperature for creative writing
		MaxTokens:    budget.Available,
	}

	result, err := e.llm.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to edit blog post: %w", err)
	}

	return result, nil
}

// buildEditingPrompt builds the prompt for the editor
func (e *EditorAgent) buildEditingPrompt(draft, transcription string, input map[string]interface{}) string {
	mode := "review"
	if e.autoRevise {
		mode = "auto-revise"
	}

	prompt := fmt.Sprintf(`Review the following blog post draft and provide feedback or revisions as specified in your system prompt.

**Review Mode**: %s

# Draft to Review

%s`, mode, draft)

	// Add original transcription if available
	if transcription != "" {
		prompt += fmt.Sprintf(`

# Original Dictation (for context)

%s`, transcription)
	}

	// Add specific review focus if provided
	if focus, ok := input["review_focus"].(string); ok && focus != "" {
		prompt += fmt.Sprintf(`

# Specific Focus for This Review

%s`, focus)
	}

	// Add instructions based on mode
	if e.autoRevise {
		prompt += `

# Instructions

Provide the fully revised and polished version of this blog post, along with notes on what was changed and why.`
	} else {
		prompt += `

# Instructions

Provide a detailed review of this blog post covering:
1. Thesis clarity and support
2. Narrative flow and structure
3. Content quality
4. Writing mechanics
5. Calls to action
6. Headline and metadata

Be specific about what works well and what needs improvement.`
	}

	return prompt
}
