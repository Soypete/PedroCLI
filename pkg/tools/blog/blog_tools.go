package blog

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/agents"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/database"
	"github.com/soypete/pedrocli/pkg/integrations/notion"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	blogstorage "github.com/soypete/pedrocli/pkg/storage/blog"
	"github.com/soypete/pedrocli/pkg/tools"
)

// BlogToolsManager manages blog-specific tools and workflows
type BlogToolsManager struct {
	db            *database.DB
	postStore     *blogstorage.PostStore
	trainingStore *blogstorage.TrainingStore
	assetStore    *blogstorage.NewsletterStore
	notionClient  *notion.Client
	config        *config.Config
	jobManager    *jobs.Manager
	llmBackend    llm.Backend
}

// NewBlogToolsManager creates a new blog tools manager
func NewBlogToolsManager(cfg *config.Config, jobMgr *jobs.Manager, backend llm.Backend) (*BlogToolsManager, error) {
	// Initialize database if blog tools are enabled
	if !cfg.Blog.Enabled {
		return nil, fmt.Errorf("blog tools are not enabled in config")
	}

	dbConfig := &database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
		SSLMode:  cfg.Database.SSLMode,
	}

	db, err := database.New(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	if err := db.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize stores
	postStore := blogstorage.NewPostStore(db.DB)
	trainingStore := blogstorage.NewTrainingStore(db.DB)
	assetStore := blogstorage.NewNewsletterStore(db.DB)

	// Initialize Notion client if configured
	var notionClient *notion.Client
	if cfg.Blog.NotionAPIKey != "" {
		notionCfg := &notion.Config{
			APIKey:           cfg.Blog.NotionAPIKey,
			BlogDraftsDB:     cfg.Blog.NotionDraftsDB,
			PublishedPostsDB: cfg.Blog.NotionPublishedDB,
			AssetDB:          cfg.Blog.NotionAssetsDB,
			IdeasDB:          cfg.Blog.NotionIdeasDB,
		}
		notionClient = notion.NewClient(notionCfg)
	}

	return &BlogToolsManager{
		db:            db,
		postStore:     postStore,
		trainingStore: trainingStore,
		assetStore:    assetStore,
		notionClient:  notionClient,
		config:        cfg,
		jobManager:    jobMgr,
		llmBackend:    backend,
	}, nil
}

// CreateDraftFromDictation creates a new blog post from raw dictation
func (m *BlogToolsManager) CreateDraftFromDictation(ctx context.Context, transcription string, title string) (*blogstorage.BlogPost, error) {
	// Create database entry
	post := &blogstorage.BlogPost{
		Title:                     title,
		Status:                    blogstorage.StatusDictated,
		RawTranscription:          transcription,
		TranscriptionDurationSecs: 0, // TODO: Calculate from audio duration
	}

	if err := m.postStore.Create(post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// Save to training data
	trainingPair := &blogstorage.TrainingPair{
		SourceType: blogstorage.SourceDictation,
		InputText:  transcription,
		OutputText: "", // Will be filled when writer agent completes
		Metadata: map[string]interface{}{
			"post_id": post.ID.String(),
		},
	}
	if err := m.trainingStore.Create(trainingPair); err != nil {
		// Log error but don't fail - training data is nice-to-have
		fmt.Printf("Warning: failed to save training data: %v\n", err)
	}

	return post, nil
}

// ProcessWithWriter processes a dictated post with the writer agent
func (m *BlogToolsManager) ProcessWithWriter(ctx context.Context, postID uuid.UUID) (*jobs.Job, error) {
	// Get post
	post, err := m.postStore.Get(postID)
	if err != nil {
		return nil, err
	}

	// Create writer agent
	writer := agents.NewWriterAgent(m.config, m.llmBackend, m.jobManager)

	// Execute writer
	input := map[string]interface{}{
		"transcription": post.RawTranscription,
		"title":         post.Title,
	}

	job, err := writer.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	// Update post status
	if err := m.postStore.UpdateStatus(postID, blogstorage.StatusDrafted); err != nil {
		return nil, err
	}

	return job, nil
}

// ProcessWithEditor processes a drafted post with the editor agent
func (m *BlogToolsManager) ProcessWithEditor(ctx context.Context, postID uuid.UUID, autoRevise bool) (*jobs.Job, error) {
	// Get post
	post, err := m.postStore.Get(postID)
	if err != nil {
		return nil, err
	}

	if post.WriterOutput == "" {
		return nil, fmt.Errorf("post has no writer output to edit")
	}

	// Create editor agent
	editor := agents.NewEditorAgent(m.config, m.llmBackend, m.jobManager, autoRevise)

	// Execute editor
	input := map[string]interface{}{
		"draft":                  post.WriterOutput,
		"original_transcription": post.RawTranscription,
		"title":                  post.Title,
	}

	job, err := editor.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	// Update post status
	if err := m.postStore.UpdateStatus(postID, blogstorage.StatusEdited); err != nil {
		return nil, err
	}

	return job, nil
}

// AddNewsletterSection adds newsletter addendum to a post
func (m *BlogToolsManager) AddNewsletterSection(ctx context.Context, postID uuid.UUID) error {
	// Build newsletter content
	builder, err := blogstorage.NewNewsletterBuilder(m.assetStore)
	if err != nil {
		return err
	}

	newsletterContent, err := builder.BuildForPost(postID)
	if err != nil {
		return err
	}

	// Get post and append newsletter
	post, err := m.postStore.Get(postID)
	if err != nil {
		return err
	}

	// Append newsletter to final content
	if post.FinalContent == "" {
		post.FinalContent = post.EditorOutput
	}
	post.FinalContent += "\n\n" + newsletterContent

	// Save newsletter data to post
	post.NewsletterAddendum = map[string]interface{}{
		"generated_at": time.Now(),
		"content":      newsletterContent,
	}

	return m.postStore.Update(post)
}

// PublishToNotion publishes a post to Notion
func (m *BlogToolsManager) PublishToNotion(ctx context.Context, postID uuid.UUID) error {
	if m.notionClient == nil {
		return fmt.Errorf("notion integration not configured")
	}

	post, err := m.postStore.Get(postID)
	if err != nil {
		return err
	}

	// Create or update Notion page
	if post.NotionPageID == "" {
		// Create new page
		pageID, err := m.notionClient.CreateDraftPost(post.Title, post.FinalContent, string(post.Status))
		if err != nil {
			return err
		}
		post.NotionPageID = pageID
	} else {
		// Update existing page
		if err := m.notionClient.UpdatePost(post.NotionPageID, post.FinalContent); err != nil {
			return err
		}
	}

	return m.postStore.Update(post)
}

// MarkAsPublished marks a post as published with paywall
func (m *BlogToolsManager) MarkAsPublished(ctx context.Context, postID uuid.UUID, substackURL string) error {
	post, err := m.postStore.Get(postID)
	if err != nil {
		return err
	}

	// Set paywall until date
	paywallUntil := time.Now().AddDate(0, 0, m.config.Blog.PaywallDays)
	post.PaywallUntil = &paywallUntil
	post.SubstackURL = substackURL
	post.Status = blogstorage.StatusPublished

	return m.postStore.Update(post)
}

// GetExpiredPaywallPosts returns posts whose paywall has expired
func (m *BlogToolsManager) GetExpiredPaywallPosts(ctx context.Context) ([]*blogstorage.BlogPost, error) {
	return m.postStore.GetPostsWithExpiredPaywall()
}

// ListPosts lists blog posts with optional status filter
func (m *BlogToolsManager) ListPosts(ctx context.Context, status blogstorage.PostStatus) ([]*blogstorage.BlogPost, error) {
	return m.postStore.List(status)
}

// GetPost retrieves a specific blog post
func (m *BlogToolsManager) GetPost(ctx context.Context, postID uuid.UUID) (*blogstorage.BlogPost, error) {
	return m.postStore.Get(postID)
}

// Close closes database connections
func (m *BlogToolsManager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// Ensure BlogToolsManager implements the Tool interface if needed
var _ tools.Tool = (*DictationTool)(nil)
var _ tools.Tool = (*ProcessWriterTool)(nil)
var _ tools.Tool = (*ProcessEditorTool)(nil)
var _ tools.Tool = (*PublishTool)(nil)
