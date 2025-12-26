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

//go:embed prompts/blog_writer_system.md
var blogWriterSystemPrompt string

// WriterAgent transforms raw dictation into polished blog posts
type WriterAgent struct {
	*BaseAgent
}

// NewWriterAgent creates a new writer agent
func NewWriterAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *WriterAgent {
	base := NewBaseAgent(
		"writer",
		"Transform raw dictation into polished, narrative-driven blog posts",
		cfg,
		backend,
		jobMgr,
	)

	return &WriterAgent{
		BaseAgent: base,
	}
}

// Execute executes the writer agent asynchronously
func (w *WriterAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get raw transcription
	transcription, ok := input["transcription"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'transcription' in input")
	}

	// Get optional context
	postTitle, _ := input["title"].(string)
	if postTitle == "" {
		postTitle = "Untitled Post"
	}

	// Create job
	job, err := w.jobManager.Create("blog_writer", "Write blog post: "+postTitle, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	w.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil)

	// Run the writing process in background
	go func() {
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, w.config.Debug.Enabled)
		if err != nil {
			w.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Build prompt
		userPrompt := w.buildWritingPrompt(transcription, input)

		// Write to context
		if err := contextMgr.WritePrompt(userPrompt); err != nil {
			w.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Execute inference
		result, err := w.executeWriting(bgCtx, contextMgr, userPrompt)
		if err != nil {
			w.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Write response
		if err := contextMgr.WriteResponse(result.Text); err != nil {
			w.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Update job with results
		output := map[string]interface{}{
			"status":  "completed",
			"content": result.Text,
			"job_dir": contextMgr.GetJobDir(),
		}

		w.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// executeWriting performs the writing task
func (w *WriterAgent) executeWriting(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string) (*llm.InferenceResponse, error) {
	// Build system prompt from embedded file
	systemPrompt := blogWriterSystemPrompt

	// Calculate context budget
	budget := llm.CalculateBudget(w.config, systemPrompt, userPrompt, "")

	// Generate with LLM
	result, err := w.llm.Generate(ctx, systemPrompt, userPrompt, budget.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to generate blog post: %w", err)
	}

	return result, nil
}

// buildWritingPrompt builds the prompt for the writer
func (w *WriterAgent) buildWritingPrompt(transcription string, input map[string]interface{}) string {
	prompt := fmt.Sprintf(`Transform the following raw dictation into a polished blog post following the guidelines in your system prompt.

# Raw Dictation

%s

# Additional Context`, transcription)

	// Add optional context
	if title, ok := input["title"].(string); ok && title != "" {
		prompt += fmt.Sprintf("\n- Suggested Title: %s", title)
	}

	if theme, ok := input["theme"].(string); ok && theme != "" {
		prompt += fmt.Sprintf("\n- Main Theme: %s", theme)
	}

	if audience, ok := input["audience"].(string); ok && audience != "" {
		prompt += fmt.Sprintf("\n- Target Audience: %s", audience)
	}

	// Add writing style preferences from config if available
	// TODO: Load from user's style guide when available
	prompt += `

# Instructions

1. Read the entire dictation carefully
2. Identify the central thesis
3. Extract key points, examples, and anecdotes
4. Organize into a compelling narrative structure
5. Write the full blog post with:
   - Engaging opening hook
   - Clear thesis statement
   - Well-structured body sections
   - Strong conclusion
   - Clear call to action
6. Provide 3 title options
7. Suggest 3 pull quotes
8. Write a meta description

Begin your transformation now.`

	return prompt
}
