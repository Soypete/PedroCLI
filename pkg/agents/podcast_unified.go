package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage/content"
	"github.com/soypete/pedrocli/pkg/tools"
)

// PodcastWorkflowType defines the type of podcast workflow to execute
type PodcastWorkflowType string

const (
	WorkflowScript   PodcastWorkflowType = "script"   // Generate episode script from outline
	WorkflowNews     PodcastWorkflowType = "news"     // Review and summarize news items
	WorkflowSchedule PodcastWorkflowType = "schedule" // Create Cal.com booking link
	WorkflowFullPrep PodcastWorkflowType = "prep"     // Full workflow (all three)
)

// UnifiedPodcastAgent orchestrates podcast episode preparation workflows
// It implements AgentOrchestrator for consistency with blog and coding agents
type UnifiedPodcastAgent struct {
	// Core dependencies
	backend      llm.Backend
	contentStore content.ContentStore
	versionStore content.VersionStore
	config       *config.Config
	jobManager   jobs.JobManager

	// Workflow state
	workflowType    PodcastWorkflowType
	currentContent  *content.Content
	phases          []Phase
	currentPhase    string
	progress        *ProgressTracker
	mode            ExecutionMode

	// Tools
	tools map[string]tools.Tool

	// Podcast-specific state
	outline     string
	script      string
	newsItems   []NewsItem
	bookingURL  string
	episode     string
	title       string
	guests      string
	duration    int
	focus       string // News focus topic
	maxNews     int
	riverside   bool
}

// NewsItem represents a news item for podcast prep
type NewsItem struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Description string    `json:"description"`
	Relevance   float64   `json:"relevance"` // 0-1 relevance score
	PubDate     time.Time `json:"pub_date"`
}

// UnifiedPodcastAgentConfig configures the podcast agent
type UnifiedPodcastAgentConfig struct {
	Backend      llm.Backend
	ContentStore content.ContentStore
	VersionStore content.VersionStore
	Config       *config.Config
	JobManager   jobs.JobManager
	Mode         ExecutionMode

	// Workflow selection
	WorkflowType PodcastWorkflowType

	// Input data
	Outline   string // Episode outline (markdown)
	Episode   string // Episode number (e.g., "S01E03")
	Title     string // Episode title
	Guests    string // Guest names (comma-separated)
	Duration  int    // Target duration in minutes
	Focus     string // News focus topic
	MaxNews   int    // Maximum news items
	Riverside bool   // Include Riverside.fm integration
}

// NewUnifiedPodcastAgent creates a new podcast agent
func NewUnifiedPodcastAgent(cfg UnifiedPodcastAgentConfig) *UnifiedPodcastAgent {
	agent := &UnifiedPodcastAgent{
		backend:      cfg.Backend,
		contentStore: cfg.ContentStore,
		versionStore: cfg.VersionStore,
		config:       cfg.Config,
		jobManager:   cfg.JobManager,
		mode:         cfg.Mode,
		workflowType: cfg.WorkflowType,
		tools:        make(map[string]tools.Tool),

		// Input data
		outline:   cfg.Outline,
		episode:   cfg.Episode,
		title:     cfg.Title,
		guests:    cfg.Guests,
		duration:  cfg.Duration,
		focus:     cfg.Focus,
		maxNews:   cfg.MaxNews,
		riverside: cfg.Riverside,

		// Default values
		newsItems: make([]NewsItem, 0),
	}

	// Initialize content record
	agent.currentContent = &content.Content{
		ID:     uuid.New(),
		Type:   content.ContentTypePodcast,
		Status: content.StatusDraft,
		Title:  fmt.Sprintf("Episode %s: %s", cfg.Episode, cfg.Title),
		Data:   make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store input data in content
	agent.currentContent.Data["episode"] = cfg.Episode
	agent.currentContent.Data["title"] = cfg.Title
	agent.currentContent.Data["outline"] = cfg.Outline
	agent.currentContent.Data["guests"] = cfg.Guests
	agent.currentContent.Data["duration"] = cfg.Duration

	// Initialize tools
	agent.initializeTools()

	// Build phases based on workflow type
	agent.phases = agent.buildPhases()

	// Initialize progress tracker
	agent.progress = NewProgressTracker()
	for _, phase := range agent.phases {
		agent.progress.AddPhase(phase.Name)
	}

	return agent
}

// RegisterTool registers a tool with the podcast agent
func (a *UnifiedPodcastAgent) RegisterTool(tool tools.Tool) {
	a.tools[tool.Name()] = tool
}

// initializeTools registers all tools needed for podcast workflows
func (a *UnifiedPodcastAgent) initializeTools() {
	// Research tools
	searchTool := tools.NewWebSearchTool()
	scraperTool := tools.NewWebScraperTool()
	rssTool := tools.NewRSSFeedTool(a.config)

	// Cal.com integration
	calcomTool := tools.NewCalComTool(a.config, nil) // nil TokenManager for now

	// Notion integration (for publishing scripts and news summaries)
	if a.config.Podcast.Notion.Enabled {
		notionTool := tools.NewNotionTool(a.config, nil) // nil TokenManager for now
		a.tools[notionTool.Name()] = notionTool
	}

	a.tools[searchTool.Name()] = searchTool
	a.tools[scraperTool.Name()] = scraperTool
	a.tools[rssTool.Name()] = rssTool
	a.tools[calcomTool.Name()] = calcomTool
}

// buildPhases constructs the workflow phases based on workflow type
func (a *UnifiedPodcastAgent) buildPhases() []Phase {
	switch a.workflowType {
	case WorkflowScript:
		return a.buildScriptPhases()
	case WorkflowNews:
		return a.buildNewsPhases()
	case WorkflowSchedule:
		return a.buildSchedulePhases()
	case WorkflowFullPrep:
		return a.buildFullPrepPhases()
	default:
		return a.buildScriptPhases()
	}
}

// buildScriptPhases builds the 6-phase script generation workflow
func (a *UnifiedPodcastAgent) buildScriptPhases() []Phase {
	return []Phase{
		{
			Name:        "Parse Outline",
			Description: "Extract episode structure, segments, guests, and duration from outline",
			Execute:     a.phaseParseOutline,
			Required:    true,
		},
		{
			Name:        "Research Topics",
			Description: "Search web and RSS feeds for relevant background information",
			Execute:     a.phaseResearchTopics,
			Required:    false,
		},
		{
			Name:        "Generate Segments",
			Description: "Create intro, main discussion segments, Q&A, and outro",
			Execute:     a.phaseGenerateSegments,
			Required:    true,
		},
		{
			Name:        "Assemble Script",
			Description: "Combine segments into cohesive script with timing and formatting",
			Execute:     a.phaseAssembleScript,
			Required:    true,
		},
		{
			Name:        "Review & Edit",
			Description: "Check grammar, coherence, timing, and flow",
			Execute:     a.phaseReviewScript,
			Required:    true,
		},
		{
			Name:        "Publish",
			Description: "Save script to Notion Scripts database",
			Execute:     a.phasePublishScript,
			Required:    true,
		},
	}
}

// buildNewsPhases builds the 5-phase news review workflow
func (a *UnifiedPodcastAgent) buildNewsPhases() []Phase {
	return []Phase{
		{
			Name:        "Fetch Sources",
			Description: "Fetch news from RSS feeds and web search",
			Execute:     a.phaseFetchSources,
			Required:    true,
		},
		{
			Name:        "Filter by Topic",
			Description: "Filter news items by focus topic using keyword matching",
			Execute:     a.phaseFilterByTopic,
			Required:    true,
		},
		{
			Name:        "Summarize Items",
			Description: "Extract key points from each news item",
			Execute:     a.phaseSummarizeItems,
			Required:    true,
		},
		{
			Name:        "Rank by Relevance",
			Description: "Rank news items by relevance score and select top items",
			Execute:     a.phaseRankByRelevance,
			Required:    true,
		},
		{
			Name:        "Generate Summary",
			Description: "Create formatted news summary for episode prep",
			Execute:     a.phaseGenerateNewsSummary,
			Required:    true,
		},
	}
}

// buildSchedulePhases builds the 5-phase Cal.com scheduling workflow
func (a *UnifiedPodcastAgent) buildSchedulePhases() []Phase {
	return []Phase{
		{
			Name:        "Parse Template",
			Description: "Extract episode details from template/outline",
			Execute:     a.phaseParseTemplate,
			Required:    true,
		},
		{
			Name:        "Create Event Type",
			Description: "Create or update Cal.com event type for episode",
			Execute:     a.phaseCreateEventType,
			Required:    true,
		},
		{
			Name:        "Configure Riverside",
			Description: "Set up Riverside.fm integration for recording",
			Execute:     a.phaseConfigureRiverside,
			Required:    false, // Optional based on riverside flag
		},
		{
			Name:        "Generate Booking Link",
			Description: "Get shareable booking URL from Cal.com",
			Execute:     a.phaseGenerateBookingLink,
			Required:    true,
		},
		{
			Name:        "Save to Notion",
			Description: "Store booking link and episode info in Notion",
			Execute:     a.phaseSaveBookingToNotion,
			Required:    false, // Optional if Notion enabled
		},
	}
}

// buildFullPrepPhases builds the complete prep workflow (all three workflows)
func (a *UnifiedPodcastAgent) buildFullPrepPhases() []Phase {
	// Combine all three workflows in sequence
	scriptPhases := a.buildScriptPhases()
	newsPhases := a.buildNewsPhases()
	schedulePhases := a.buildSchedulePhases()

	allPhases := make([]Phase, 0)
	allPhases = append(allPhases, scriptPhases...)
	allPhases = append(allPhases, newsPhases...)
	allPhases = append(allPhases, schedulePhases...)

	return allPhases
}

// Execute executes the podcast agent asynchronously and returns a job
func (a *UnifiedPodcastAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Extract workflow type from input (or use agent's default)
	workflowTypeStr, _ := input["workflow_type"].(string)
	if workflowTypeStr == "" {
		workflowTypeStr = string(a.workflowType)
	}

	// Build description for job
	description := fmt.Sprintf("Podcast %s workflow: %s", workflowTypeStr, a.title)

	// Create job
	job, err := a.jobManager.Create(ctx, "podcast_"+workflowTypeStr, description, input)
	if err != nil {
		return nil, err
	}

	// Update status to running
	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the workflow in background
	go func() {
		bgCtx := context.Background()

		// Execute the workflow
		if err := a.ExecuteWorkflow(bgCtx); err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Success - update job with output
		output := a.GetOutput()
		outputMap, ok := output.(map[string]interface{})
		if !ok {
			outputMap = map[string]interface{}{"result": output}
		}
		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, outputMap, nil)
	}()

	return job, nil
}

// ExecuteWorkflow executes the workflow synchronously without job management (for CLI)
func (a *UnifiedPodcastAgent) ExecuteWorkflow(ctx context.Context) error {
	// Create initial content record
	if err := a.contentStore.Create(ctx, a.currentContent); err != nil {
		return fmt.Errorf("failed to create content record: %w", err)
	}

	// Execute each phase
	for i, phase := range a.phases {
		a.currentPhase = phase.Name
		a.progress.UpdatePhase(phase.Name, PhaseStatusInProgress, "")

		fmt.Printf("\n📋 Phase %d/%d: %s\n", i+1, len(a.phases), phase.Name)
		fmt.Printf("   %s\n", phase.Description)

		// Skip optional phases if needed
		if !phase.Required && a.shouldSkipPhase(phase) {
			a.progress.UpdatePhase(phase.Name, PhaseStatusSkipped, "Phase skipped")
			fmt.Printf("   ⏭️  Skipped (optional)\n")
			continue
		}

		// Execute phase
		if err := phase.Execute(ctx); err != nil {
			a.progress.UpdatePhase(phase.Name, PhaseStatusFailed, err.Error())
			return fmt.Errorf("phase %s failed: %w", phase.Name, err)
		}

		a.progress.UpdatePhase(phase.Name, PhaseStatusDone, "")
		fmt.Printf("   ✅ Complete\n")

		// Save version snapshot after each phase
		if err := a.saveVersion(ctx, phase.Name, i+1); err != nil {
			return fmt.Errorf("failed to save version: %w", err)
		}

		// Update content store
		a.currentContent.UpdatedAt = time.Now()
		if err := a.contentStore.Update(ctx, a.currentContent); err != nil {
			return fmt.Errorf("failed to update content: %w", err)
		}
	}

	// Mark as complete
	a.currentContent.Status = content.StatusPublished
	a.currentContent.UpdatedAt = time.Now()
	if err := a.contentStore.Update(ctx, a.currentContent); err != nil {
		return fmt.Errorf("failed to mark content as published: %w", err)
	}

	fmt.Println("\n✅ Podcast workflow complete!")
	return nil
}

// shouldSkipPhase determines if an optional phase should be skipped
func (a *UnifiedPodcastAgent) shouldSkipPhase(phase Phase) bool {
	switch phase.Name {
	case "Configure Riverside":
		return !a.riverside
	case "Save to Notion":
		return !a.config.Podcast.Notion.Enabled
	case "Research Topics":
		return a.outline == "" // Skip research if no outline
	default:
		return false
	}
}

// saveVersion saves a version snapshot after a phase completes
func (a *UnifiedPodcastAgent) saveVersion(ctx context.Context, phaseName string, versionNum int) error {
	version := &content.Version{
		ID:         uuid.New(),
		ContentID:  a.currentContent.ID,
		Phase:      phaseName,
		VersionNum: versionNum,
		Snapshot:   make(map[string]interface{}),
		CreatedAt:  time.Now(),
	}

	// Snapshot current state
	version.Snapshot["episode"] = a.episode
	version.Snapshot["title"] = a.title
	version.Snapshot["outline"] = a.outline
	version.Snapshot["script"] = a.script
	version.Snapshot["news_items"] = a.newsItems
	version.Snapshot["booking_url"] = a.bookingURL

	return a.versionStore.SaveVersion(ctx, version)
}

// GetPhases implements AgentOrchestrator.GetPhases
func (a *UnifiedPodcastAgent) GetPhases() []Phase {
	return a.phases
}

// GetCurrentPhase implements AgentOrchestrator.GetCurrentPhase
func (a *UnifiedPodcastAgent) GetCurrentPhase() string {
	return a.currentPhase
}

// GetProgress implements AgentOrchestrator.GetProgress
func (a *UnifiedPodcastAgent) GetProgress() *ProgressTracker {
	return a.progress
}

// GetOutput implements AgentOrchestrator.GetOutput
func (a *UnifiedPodcastAgent) GetOutput() interface{} {
	return map[string]interface{}{
		"content_id":  a.currentContent.ID,
		"episode":     a.episode,
		"title":       a.title,
		"script":      a.script,
		"news_items":  a.newsItems,
		"booking_url": a.bookingURL,
		"workflow":    string(a.workflowType),
	}
}

// Phase implementations will go in separate files:
// - podcast_script_phases.go (script generation phases)
// - podcast_news_phases.go (news review phases)
// - podcast_schedule_phases.go (Cal.com scheduling phases)
