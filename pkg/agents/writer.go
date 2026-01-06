package agents

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
)

//go:embed prompts/blog_writer_system.md
var blogWriterSystemPrompt string

// BlogWriterOutput represents the structured output from the blog writer
type BlogWriterOutput struct {
	ExpandedDraft     string   `json:"expanded_draft"`
	SuggestedTitles   []string `json:"suggested_titles"`
	SubstackTags      []string `json:"substack_tags"`
	TwitterPost       string   `json:"twitter_post"`
	LinkedInPost      string   `json:"linkedin_post"`
	BlueskyPost       string   `json:"bluesky_post"`
	KeyTakeaways      []string `json:"key_takeaways"`
	TargetAudience    string   `json:"target_audience"`
	EstimatedReadTime string   `json:"estimated_read_time"`
	MetaDescription   string   `json:"meta_description"`
}

// WriterAgent transforms raw dictation into polished blog posts
type WriterAgent struct {
	*BaseAgent
}

// NewWriterAgent creates a new writer agent
func NewWriterAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *WriterAgent {
	base := NewBaseAgent(
		"writer",
		"Transform raw dictation into polished, narrative-driven blog posts with title suggestions, tags, and social media posts",
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
		// Also accept "dictation" as input key
		transcription, ok = input["dictation"].(string)
		if !ok {
			return nil, fmt.Errorf("missing 'transcription' or 'dictation' in input")
		}
	}

	// Get optional context
	postTitle, _ := input["title"].(string)
	if postTitle == "" {
		postTitle = "Untitled Post"
	}

	// Create job
	job, err := w.jobManager.Create(ctx, "blog_writer", "Write blog post: "+postTitle, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	w.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the writing process in background
	go func() {
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, w.config.Debug.Enabled, w.config.Model.ContextSize)
		if err != nil {
			w.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		// Configure context manager with model and stats tracking
		contextMgr.SetModelName(w.config.Model.ModelName)
		if w.compactionStatsStore != nil {
			contextMgr.SetStatsStore(w.compactionStatsStore)
		}

		// Build prompt
		userPrompt := w.buildWritingPrompt(transcription, input)

		// Write to context
		if err := contextMgr.SavePrompt(userPrompt); err != nil {
			w.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Execute inference
		result, err := w.executeWriting(bgCtx, contextMgr, userPrompt)
		if err != nil {
			w.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Write response
		if err := contextMgr.SaveResponse(result.Text); err != nil {
			w.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Parse the structured JSON output
		parsedOutput, parseErr := parseWriterOutput(result.Text)

		// Update job with results
		output := map[string]interface{}{
			"status":                 "completed",
			"raw_response":           result.Text,
			"original_transcription": transcription,
			"job_dir":                contextMgr.GetJobDir(),
		}

		if parseErr != nil {
			// Parsing failed, but we still have the raw content
			output["parse_error"] = parseErr.Error()
			output["expanded_draft"] = result.Text
			output["suggested_titles"] = []string{postTitle}
		} else {
			// Successfully parsed - add all structured fields
			output["expanded_draft"] = parsedOutput.ExpandedDraft
			output["suggested_titles"] = parsedOutput.SuggestedTitles
			output["substack_tags"] = parsedOutput.SubstackTags
			output["twitter_post"] = parsedOutput.TwitterPost
			output["linkedin_post"] = parsedOutput.LinkedInPost
			output["bluesky_post"] = parsedOutput.BlueskyPost
			output["key_takeaways"] = parsedOutput.KeyTakeaways
			output["target_audience"] = parsedOutput.TargetAudience
			output["read_time"] = parsedOutput.EstimatedReadTime
			output["meta_description"] = parsedOutput.MetaDescription
		}

		w.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// executeWriting performs the writing task
func (w *WriterAgent) executeWriting(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string) (*llm.InferenceResponse, error) {
	// Build system prompt from embedded file
	systemPrompt := blogWriterSystemPrompt

	// Calculate context budget
	budget := llm.CalculateBudget(w.config, systemPrompt, userPrompt, "")

	// Generate with LLM - use higher temperature for creative writing
	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.7,
		MaxTokens:    budget.Available,
	}

	result, err := w.llm.Infer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate blog post: %w", err)
	}

	return result, nil
}

// buildWritingPrompt builds the prompt for the writer
func (w *WriterAgent) buildWritingPrompt(transcription string, input map[string]interface{}) string {
	prompt := fmt.Sprintf(`Transform the following raw dictation into a polished blog post.

# Raw Dictation

%s

# Additional Context`, transcription)

	// Add optional context
	if title, ok := input["title"].(string); ok && title != "" {
		prompt += fmt.Sprintf("\n- Working Title: %s", title)
	}

	if theme, ok := input["theme"].(string); ok && theme != "" {
		prompt += fmt.Sprintf("\n- Main Theme: %s", theme)
	}

	if audience, ok := input["audience"].(string); ok && audience != "" {
		prompt += fmt.Sprintf("\n- Target Audience: %s", audience)
	}

	prompt += `

# Instructions

1. Read the entire dictation carefully
2. Identify the central thesis and key insights
3. Organize into a compelling narrative structure
4. Write the full blog post with proper flow and transitions
5. Generate all required fields (titles, tags, social posts, etc.)
6. Output as valid JSON following the format in your system prompt

Begin your transformation now. Output ONLY the JSON object.`

	return prompt
}

// parseWriterOutput parses the LLM response into structured output
func parseWriterOutput(text string) (*BlogWriterOutput, error) {
	// Try to extract JSON from the response
	// The LLM might wrap it in markdown code blocks
	jsonStr := text

	// Try to find JSON in code blocks first
	jsonBlockRegex := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	matches := jsonBlockRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		jsonStr = matches[1]
	}

	// Clean up the string
	jsonStr = strings.TrimSpace(jsonStr)

	// Try to parse
	var output BlogWriterOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		// Try to find JSON object in the text
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}")
		if start != -1 && end > start {
			jsonStr = text[start : end+1]
			if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
				return nil, fmt.Errorf("failed to parse JSON output: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no JSON object found in response")
		}
	}

	return &output, nil
}
