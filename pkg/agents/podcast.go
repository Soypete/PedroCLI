package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/prompts"
)

// PodcastBaseAgent provides common functionality for podcast agents
type PodcastBaseAgent struct {
	*BaseAgent
	promptMgr *prompts.Manager
}

// NewPodcastBaseAgent creates a new podcast base agent
func NewPodcastBaseAgent(name, description string, cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *PodcastBaseAgent {
	base := NewBaseAgent(name, description, cfg, backend, jobMgr)
	return &PodcastBaseAgent{
		BaseAgent: base,
		promptMgr: prompts.NewManager(cfg),
	}
}

// buildPodcastSystemPrompt builds the system prompt for podcast agents
func (a *PodcastBaseAgent) buildPodcastSystemPrompt() string {
	return a.promptMgr.GetPodcastSystemPrompt() + `

Available tools:
- notion: Query and update Notion databases (scripts, articles, news, guests)
- calendar: Manage Google Calendar events for recording sessions
- file: Read and write local files for scripts and notes

Tool usage format:
{"tool": "tool_name", "args": {"action": "action_name", ...}}

When you have completed all tasks, respond with "TASK_COMPLETE".`
}

// ScriptCreatorAgent creates podcast scripts
type ScriptCreatorAgent struct {
	*PodcastBaseAgent
}

// NewScriptCreatorAgent creates a new script creator agent
func NewScriptCreatorAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *ScriptCreatorAgent {
	base := NewPodcastBaseAgent(
		"create_podcast_script",
		"Create podcast episode scripts from topics and notes",
		cfg,
		backend,
		jobMgr,
	)
	return &ScriptCreatorAgent{
		PodcastBaseAgent: base,
	}
}

// Execute executes the script creator agent
func (a *ScriptCreatorAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	topic, ok := input["topic"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'topic' in input")
	}

	description := fmt.Sprintf("Create script for: %s", topic)
	job, err := a.jobManager.Create(ctx, "create_podcast_script", description, input)
	if err != nil {
		return nil, err
	}

	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()
		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		userPrompt := a.buildPrompt(input)
		executor := NewInferenceExecutor(a.BaseAgent, contextMgr)
		executor.SetSystemPrompt(a.buildPodcastSystemPrompt())

		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}
		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

func (a *ScriptCreatorAgent) buildPrompt(input map[string]interface{}) string {
	topic := input["topic"].(string)
	prompt := a.promptMgr.GetPrompt("podcast", "create_podcast_script")
	prompt += fmt.Sprintf("\n\n## Current Task\nTopic: %s\n", topic)

	if notes, ok := input["notes"].(string); ok && notes != "" {
		prompt += fmt.Sprintf("\nNotes:\n%s\n", notes)
	}
	if guests, ok := input["guests"].([]interface{}); ok && len(guests) > 0 {
		prompt += "\nGuests:\n"
		for _, g := range guests {
			if guest, ok := g.(string); ok {
				prompt += fmt.Sprintf("- %s\n", guest)
			}
		}
	}
	if recordingDate, ok := input["recording_date"].(string); ok && recordingDate != "" {
		prompt += fmt.Sprintf("\nRecording Date: %s\n", recordingDate)
	}

	return prompt
}

// NewsReviewerAgent summarizes news for podcast prep
type NewsReviewerAgent struct {
	*PodcastBaseAgent
}

// NewNewsReviewerAgent creates a new news reviewer agent
func NewNewsReviewerAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *NewsReviewerAgent {
	base := NewPodcastBaseAgent(
		"review_news_summary",
		"Summarize news items for podcast episode preparation",
		cfg,
		backend,
		jobMgr,
	)
	return &NewsReviewerAgent{
		PodcastBaseAgent: base,
	}
}

// Execute executes the news reviewer agent
func (a *NewsReviewerAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	description := "Review and summarize news items for podcast"
	if topic, ok := input["focus_topic"].(string); ok {
		description = fmt.Sprintf("Review news about: %s", topic)
	}

	job, err := a.jobManager.Create(ctx, "review_news_summary", description, input)
	if err != nil {
		return nil, err
	}

	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()
		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		userPrompt := a.buildPrompt(input)
		executor := NewInferenceExecutor(a.BaseAgent, contextMgr)
		executor.SetSystemPrompt(a.buildPodcastSystemPrompt())

		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}
		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

func (a *NewsReviewerAgent) buildPrompt(input map[string]interface{}) string {
	prompt := a.promptMgr.GetPrompt("podcast", "review_news_summary")

	if focusTopic, ok := input["focus_topic"].(string); ok && focusTopic != "" {
		prompt += fmt.Sprintf("\n\n## Focus Area\nPrioritize news related to: %s\n", focusTopic)
	}
	if maxItems, ok := input["max_items"].(float64); ok {
		prompt += fmt.Sprintf("\nLimit to top %d items.\n", int(maxItems))
	}

	return prompt
}

// LinkAdderAgent adds links to Notion databases
type LinkAdderAgent struct {
	*PodcastBaseAgent
}

// NewLinkAdderAgent creates a new link adder agent
func NewLinkAdderAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *LinkAdderAgent {
	base := NewPodcastBaseAgent(
		"add_notion_link",
		"Add article or news links to Notion databases for review",
		cfg,
		backend,
		jobMgr,
	)
	return &LinkAdderAgent{
		PodcastBaseAgent: base,
	}
}

// Execute executes the link adder agent
func (a *LinkAdderAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	url, ok := input["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'url' in input")
	}

	description := fmt.Sprintf("Add link: %s", url)
	job, err := a.jobManager.Create(ctx, "add_notion_link", description, input)
	if err != nil {
		return nil, err
	}

	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()
		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		userPrompt := a.buildPrompt(input)
		executor := NewInferenceExecutor(a.BaseAgent, contextMgr)
		executor.SetSystemPrompt(a.buildPodcastSystemPrompt())

		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}
		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

func (a *LinkAdderAgent) buildPrompt(input map[string]interface{}) string {
	url := input["url"].(string)
	prompt := a.promptMgr.GetPrompt("podcast", "add_notion_link")
	prompt += fmt.Sprintf("\n\n## Link to Add\nURL: %s\n", url)

	if title, ok := input["title"].(string); ok && title != "" {
		prompt += fmt.Sprintf("Title: %s\n", title)
	}
	if notes, ok := input["notes"].(string); ok && notes != "" {
		prompt += fmt.Sprintf("Notes: %s\n", notes)
	}
	if database, ok := input["database"].(string); ok && database != "" {
		prompt += fmt.Sprintf("Target Database: %s\n", database)
	}

	return prompt
}

// GuestAdderAgent adds guests to the database
type GuestAdderAgent struct {
	*PodcastBaseAgent
}

// NewGuestAdderAgent creates a new guest adder agent
func NewGuestAdderAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *GuestAdderAgent {
	base := NewPodcastBaseAgent(
		"add_guest",
		"Add guest information to the podcast guests database",
		cfg,
		backend,
		jobMgr,
	)
	return &GuestAdderAgent{
		PodcastBaseAgent: base,
	}
}

// Execute executes the guest adder agent
func (a *GuestAdderAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	name, ok := input["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'name' in input")
	}

	description := fmt.Sprintf("Add guest: %s", name)
	job, err := a.jobManager.Create(ctx, "add_guest", description, input)
	if err != nil {
		return nil, err
	}

	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()
		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		userPrompt := a.buildPrompt(input)
		executor := NewInferenceExecutor(a.BaseAgent, contextMgr)
		executor.SetSystemPrompt(a.buildPodcastSystemPrompt())

		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}
		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

func (a *GuestAdderAgent) buildPrompt(input map[string]interface{}) string {
	name := input["name"].(string)
	prompt := a.promptMgr.GetPrompt("podcast", "add_guest")
	prompt += fmt.Sprintf("\n\n## Guest Information\nName: %s\n", name)

	if title, ok := input["title"].(string); ok && title != "" {
		prompt += fmt.Sprintf("Title/Role: %s\n", title)
	}
	if org, ok := input["organization"].(string); ok && org != "" {
		prompt += fmt.Sprintf("Organization: %s\n", org)
	}
	if bio, ok := input["bio"].(string); ok && bio != "" {
		prompt += fmt.Sprintf("Bio: %s\n", bio)
	}
	if email, ok := input["email"].(string); ok && email != "" {
		prompt += fmt.Sprintf("Email: %s\n", email)
	}
	if topics, ok := input["topics"].([]interface{}); ok && len(topics) > 0 {
		prompt += "Topics of Expertise:\n"
		for _, t := range topics {
			if topic, ok := t.(string); ok {
				prompt += fmt.Sprintf("- %s\n", topic)
			}
		}
	}

	return prompt
}

// EpisodeOutlinerAgent creates episode outlines
type EpisodeOutlinerAgent struct {
	*PodcastBaseAgent
}

// NewEpisodeOutlinerAgent creates a new episode outliner agent
func NewEpisodeOutlinerAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *EpisodeOutlinerAgent {
	base := NewPodcastBaseAgent(
		"create_episode_outline",
		"Create episode outlines and structure from topics",
		cfg,
		backend,
		jobMgr,
	)
	return &EpisodeOutlinerAgent{
		PodcastBaseAgent: base,
	}
}

// Execute executes the episode outliner agent
func (a *EpisodeOutlinerAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	topic, ok := input["topic"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'topic' in input")
	}

	description := fmt.Sprintf("Create outline for: %s", topic)
	job, err := a.jobManager.Create(ctx, "create_episode_outline", description, input)
	if err != nil {
		return nil, err
	}

	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()
		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		userPrompt := a.buildPrompt(input)
		executor := NewInferenceExecutor(a.BaseAgent, contextMgr)
		executor.SetSystemPrompt(a.buildPodcastSystemPrompt())

		err = executor.Execute(bgCtx, userPrompt)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		output := map[string]interface{}{
			"status":  "completed",
			"job_dir": contextMgr.GetJobDir(),
		}
		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

func (a *EpisodeOutlinerAgent) buildPrompt(input map[string]interface{}) string {
	topic := input["topic"].(string)
	prompt := a.promptMgr.GetPrompt("podcast", "create_episode_outline")
	prompt += fmt.Sprintf("\n\n## Episode Topic\n%s\n", topic)

	if angle, ok := input["angle"].(string); ok && angle != "" {
		prompt += fmt.Sprintf("\nAngle/Approach: %s\n", angle)
	}
	if audience, ok := input["target_audience"].(string); ok && audience != "" {
		prompt += fmt.Sprintf("Target Audience: %s\n", audience)
	}
	if duration, ok := input["target_duration"].(string); ok && duration != "" {
		prompt += fmt.Sprintf("Target Duration: %s\n", duration)
	}

	return prompt
}
