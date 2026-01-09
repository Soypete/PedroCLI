package agents

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage/blog"
	"github.com/soypete/pedrocli/pkg/tools"
)

// BlogContentAgent orchestrates the 7-phase blog post creation workflow
type BlogContentAgent struct {
	backend       llm.Backend
	db            *sql.DB
	postStore     *blog.PostStore
	versionStore  *blog.VersionStore
	progress      *ProgressTracker
	currentPost   *blog.BlogPost
	researchData  string
	outline       string
	sections      []SectionContent
	tldr          string
	socialPosts   map[string]string          // platform -> post
	toolsList     []tools.Tool               // Tools for InferenceExecutor
	config        *config.Config             // Configuration for Notion publishing
	styleAnalyzer *BlogStyleAnalyzerAgent    // Optional style analyzer
	useStyleGuide bool                       // Whether to use style guide in editor
}

// SectionContent represents a generated blog section
type SectionContent struct {
	Title   string
	Content string
	Order   int
}

// BlogContentAgentConfig configures the blog content agent
type BlogContentAgentConfig struct {
	Backend       llm.Backend
	DB            *sql.DB
	WorkingDir    string
	MaxIterations int
	Transcription string         // Initial voice transcription
	Title         string         // Optional initial title
	Config        *config.Config // For tool initialization
}

// NewBlogContentAgent creates a new blog content agent
func NewBlogContentAgent(cfg BlogContentAgentConfig) *BlogContentAgent {
	// Create progress tracker and add phases
	progress := NewProgressTracker()
	phases := []string{
		"Transcribe",
		"Research",
		"Outline",
		"Generate Sections",
		"Assemble",
		"Editor Review",
		"Publish",
	}
	for _, phase := range phases {
		progress.AddPhase(phase)
	}

	// Register all research tools
	searchTool := tools.NewWebSearchTool()
	scraperTool := tools.NewWebScraperTool()

	// Code introspection tools (for local codebase analysis)
	// Get working directory for code tools
	workDir := "."
	if cfg.Config != nil && cfg.Config.Project.Workdir != "" {
		workDir = cfg.Config.Project.Workdir
	}
	codeSearchTool := tools.NewSearchTool(workDir)
	navigateTool := tools.NewNavigateTool(workDir)
	fileTool := tools.NewFileTool()

	// Tools that need config
	var rssTool tools.Tool
	var calendarTool tools.Tool
	var staticLinksTool tools.Tool

	if cfg.Config != nil {
		rssTool = tools.NewRSSFeedTool(cfg.Config)
		calendarTool = tools.NewCalendarTool(cfg.Config, nil) // nil TokenManager for now
		staticLinksTool = tools.NewStaticLinksTool(cfg.Config)
	}

	registeredTools := make(map[string]tools.Tool)
	registeredTools[searchTool.Name()] = searchTool
	registeredTools[scraperTool.Name()] = scraperTool
	registeredTools[codeSearchTool.Name()] = codeSearchTool
	registeredTools[navigateTool.Name()] = navigateTool
	registeredTools[fileTool.Name()] = fileTool

	if rssTool != nil {
		registeredTools[rssTool.Name()] = rssTool
	}
	if calendarTool != nil {
		registeredTools[calendarTool.Name()] = calendarTool
	}
	if staticLinksTool != nil {
		registeredTools[staticLinksTool.Name()] = staticLinksTool
	}

	// Use helper tools list for executor
	toolsList := []tools.Tool{searchTool, scraperTool, codeSearchTool, navigateTool, fileTool}
	if rssTool != nil {
		toolsList = append(toolsList, rssTool)
	}
	if calendarTool != nil {
		toolsList = append(toolsList, calendarTool)
	}
	if staticLinksTool != nil {
		toolsList = append(toolsList, staticLinksTool)
	}

	// Initialize style analyzer if RSS feed is configured
	var styleAnalyzer *BlogStyleAnalyzerAgent
	useStyleGuide := false
	if cfg.Config != nil && cfg.Config.Blog.RSSFeedURL != "" {
		styleAnalyzer = NewBlogStyleAnalyzerAgent(cfg.Backend, cfg.Config)
		useStyleGuide = true
		fmt.Println("✓ Style analyzer enabled - will enhance editor with writing style guide")
	}

	agent := &BlogContentAgent{
		backend:       cfg.Backend,
		db:            cfg.DB,
		postStore:     blog.NewPostStore(cfg.DB),
		versionStore:  blog.NewVersionStore(cfg.DB),
		progress:      progress,
		socialPosts:   make(map[string]string),
		toolsList:     toolsList,
		config:        cfg.Config,
		styleAnalyzer: styleAnalyzer,
		useStyleGuide: useStyleGuide,
	}

	// Create initial blog post record
	agent.currentPost = &blog.BlogPost{
		ID:               uuid.New(),
		Title:            cfg.Title,
		Status:           blog.StatusDictated,
		RawTranscription: cfg.Transcription,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	return agent
}

// Execute runs the complete 7-phase blog creation workflow
func (a *BlogContentAgent) Execute(ctx context.Context) error {
	fmt.Println("\n=== BlogContentAgent: 7-Phase Workflow ===")
	a.progress.PrintTree()

	// Phase 1: Transcribe (already done - we received transcription)
	if err := a.phaseTranscribe(ctx); err != nil {
		return fmt.Errorf("phase 1 failed: %w", err)
	}

	// Phase 1.5: Analyze Writing Style (if enabled)
	if err := a.phaseAnalyzeStyle(ctx); err != nil {
		// Non-fatal: continue with standard prompts if style analysis fails
		fmt.Printf("⚠️  Warning: Style analysis failed: %v\n", err)
		fmt.Println("   Continuing without personalized style guide...")
	}

	// Phase 2: Research
	if err := a.phaseResearch(ctx); err != nil {
		return fmt.Errorf("phase 2 failed: %w", err)
	}

	// Phase 3: Outline
	if err := a.phaseOutline(ctx); err != nil {
		return fmt.Errorf("phase 3 failed: %w", err)
	}

	// Phase 4: Generate Sections
	if err := a.phaseGenerateSections(ctx); err != nil {
		return fmt.Errorf("phase 4 failed: %w", err)
	}

	// Phase 5: Assemble
	if err := a.phaseAssemble(ctx); err != nil {
		return fmt.Errorf("phase 5 failed: %w", err)
	}

	// Phase 6: Editor Review
	if err := a.phaseEditorReview(ctx); err != nil {
		return fmt.Errorf("phase 6 failed: %w", err)
	}

	// Phase 7: Publish
	if err := a.phasePublish(ctx); err != nil {
		return fmt.Errorf("phase 7 failed: %w", err)
	}

	fmt.Println("\n✅ All phases complete!")
	a.progress.PrintTree()

	return nil
}

// phaseTranscribe - Phase 1: Process transcription
func (a *BlogContentAgent) phaseTranscribe(ctx context.Context) error {
	a.progress.UpdatePhase("Transcribe", PhaseStatusInProgress, "")
	a.progress.PrintTree()

	// Transcription already received in config
	if a.currentPost.RawTranscription == "" {
		a.progress.UpdatePhase("Transcribe", PhaseStatusFailed, "No transcription provided")
		a.progress.PrintTree()
		return fmt.Errorf("no transcription provided")
	}

	// Save initial post to database
	if err := a.postStore.Create(a.currentPost); err != nil {
		a.progress.UpdatePhase("Transcribe", PhaseStatusFailed, err.Error())
		a.progress.PrintTree()
		return fmt.Errorf("failed to save initial post: %w", err)
	}

	// Create initial version snapshot
	if err := a.saveVersion("Transcribe", blog.VersionTypeAutoSnapshot); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	a.progress.UpdatePhase("Transcribe", PhaseStatusDone, "")
	a.progress.AddTokens("Transcribe", 0) // No LLM tokens used
	a.progress.PrintTree()

	return nil
}

// phaseAnalyzeStyle - Phase 1.5: Analyze writing style from Substack (optional)
func (a *BlogContentAgent) phaseAnalyzeStyle(ctx context.Context) error {
	if !a.useStyleGuide || a.styleAnalyzer == nil {
		return nil // Skip if not enabled
	}

	fmt.Println("\n📚 Analyzing your writing style from Substack RSS feed...")
	fmt.Println("   This will help generate content in your authentic voice...\n")

	styleGuide, err := a.styleAnalyzer.AnalyzeStyle(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("\n✓ Writing style analysis complete!\n")
	fmt.Printf("  Generated style guide: %d characters\n", len(styleGuide))
	fmt.Println("  All content phases will now use your personal writing style\n")

	return nil
}

// enhancePromptWithStyle adds writing style guide to system prompt if available
func (a *BlogContentAgent) enhancePromptWithStyle(basePrompt string) string {
	if a.styleAnalyzer == nil || a.styleAnalyzer.GetStyleGuide() == "" {
		return basePrompt
	}

	styleGuide := a.styleAnalyzer.GetStyleGuide()
	return fmt.Sprintf(`%s

---
WRITING STYLE GUIDE:
The author has a specific voice and style. Match these characteristics in ALL output:

%s

IMPORTANT: Apply this writing style to maintain the author's authentic voice throughout.
---`, basePrompt, styleGuide)
}

// phaseResearch - Phase 2: Gather research data
func (a *BlogContentAgent) phaseResearch(ctx context.Context) error {
	a.progress.UpdatePhase("Research", PhaseStatusInProgress, "")
	a.progress.PrintTree()

	systemPrompt := `You are a research assistant for technical blog writing.

Your task is to use the available research tools to gather relevant information for a blog post.

AVAILABLE TOOLS:
- web_search: Search the web for relevant articles and documentation
- web_scraper: Scrape content from URLs, GitHub repos, or local files (supports GitHub links!)
- search_code: Search for code patterns, grep files, find files by name
- navigate_code: List directories, get file outlines, analyze imports
- file: Read/write files from the local codebase
- rss_feed: Get recent blog posts from RSS feed
- calendar: Get recent events and activities
- static_links: Get configured social media and newsletter links

CODE EXAMPLES:
For blog posts about code, use these tools to find real examples:
- Use search_code to find functions, patterns, or specific implementations
- Use web_scraper with action=scrape_local to read local Go files
- Use web_scraper with action=scrape_github to fetch code from GitHub repos
- Use navigate_code to understand code structure and imports

INSTRUCTIONS:
1. Analyze the transcription to identify key topics
2. If writing about code, use code introspection tools to find real examples
3. Search for 2-3 relevant articles or documentation pages
4. Scrape the most relevant content (including GitHub code if relevant)
5. Check RSS feed for recent related posts
6. When done, respond with "RESEARCH_COMPLETE" followed by a summary

Output format:
RESEARCH_COMPLETE

Summary of findings:
- [Key finding 1]
- [Key finding 2]
- [Key finding 3]`

	userPrompt := fmt.Sprintf(`Gather research for this blog post:

TRANSCRIPTION:
%s

Analyze the transcription and identify 2-3 key topics to research.
Summarize what research would be helpful.`, a.currentPost.RawTranscription)

	// For now, generate a simple research summary using LLM
	// Full tool-based research will be implemented in Phase 5
	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.4,
		MaxTokens:    500,
	}

	resp, err := a.backend.Infer(ctx, req)
	if err != nil {
		a.progress.UpdatePhase("Research", PhaseStatusFailed, err.Error())
		a.progress.PrintTree()
		return fmt.Errorf("research failed: %w", err)
	}

	a.researchData = resp.Text
	a.progress.AddTokens("Research", resp.TokensUsed)

	// Update post with research data (store in writer_output temporarily)
	a.currentPost.WriterOutput = fmt.Sprintf("RESEARCH:\n%s", a.researchData)
	if err := a.postStore.Update(a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version snapshot
	if err := a.saveVersion("Research", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	a.progress.UpdatePhase("Research", PhaseStatusDone, "")
	a.progress.PrintTree()

	return nil
}

// phaseOutline - Phase 3: Generate blog post outline
func (a *BlogContentAgent) phaseOutline(ctx context.Context) error {
	a.progress.UpdatePhase("Outline", PhaseStatusInProgress, "")
	a.progress.PrintTree()

	systemPrompt := `You are a technical blog post outliner.

Create a structured outline for a technical blog post based on the transcription and research.

REQUIREMENTS:
- Use markdown headings (##) for main sections
- Include 4-6 main sections
- First section should be introduction
- Last section should be conclusion
- Each section should have a brief description

Format:
## Introduction
Brief intro to hook the reader

## Section 1 Title
What this section covers

## Section 2 Title
What this section covers

... (continue for all sections)

## Conclusion
Wrap up and call to action`

	userPrompt := fmt.Sprintf(`Create an outline for this blog post:

TRANSCRIPTION:
%s

RESEARCH FINDINGS:
%s

Generate a clear, logical outline.`, a.currentPost.RawTranscription, a.researchData)

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3, // Lower for more structured output
		MaxTokens:    1000,
	}

	resp, err := a.backend.Infer(ctx, req)
	if err != nil {
		a.progress.UpdatePhase("Outline", PhaseStatusFailed, err.Error())
		a.progress.PrintTree()
		return fmt.Errorf("outline generation failed: %w", err)
	}

	a.outline = resp.Text
	a.progress.AddTokens("Outline", resp.TokensUsed)

	// Update post
	a.currentPost.WriterOutput = fmt.Sprintf("OUTLINE:\n%s\n\nRESEARCH:\n%s", a.outline, a.researchData)
	if err := a.postStore.Update(a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version
	if err := a.saveVersion("Outline", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	a.progress.UpdatePhase("Outline", PhaseStatusDone, "")
	a.progress.PrintTree()

	return nil
}

// phaseGenerateSections - Phase 4: Generate each section independently
func (a *BlogContentAgent) phaseGenerateSections(ctx context.Context) error {
	a.progress.UpdatePhase("Generate Sections", PhaseStatusInProgress, "parsing outline")
	a.progress.PrintTree()

	// Parse sections from outline
	sections := a.parseSectionsFromOutline(a.outline)
	if len(sections) == 0 {
		a.progress.UpdatePhase("Generate Sections", PhaseStatusFailed, "no sections found in outline")
		a.progress.PrintTree()
		return fmt.Errorf("failed to parse sections from outline")
	}

	a.sections = make([]SectionContent, 0, len(sections))

	// Generate each section
	for i, sectionTitle := range sections {
		a.progress.UpdatePhase("Generate Sections", PhaseStatusInProgress, fmt.Sprintf("section %d/%d", i+1, len(sections)))
		a.progress.PrintTree()

		content, tokens, err := a.generateSection(ctx, sectionTitle, i)
		if err != nil {
			a.progress.UpdatePhase("Generate Sections", PhaseStatusFailed, err.Error())
			a.progress.PrintTree()
			return fmt.Errorf("failed to generate section %d: %w", i, err)
		}

		a.sections = append(a.sections, SectionContent{
			Title:   sectionTitle,
			Content: content,
			Order:   i,
		})

		a.progress.AddTokens("Generate Sections", tokens)
	}

	// Generate TLDR with logit bias
	a.progress.UpdatePhase("Generate Sections", PhaseStatusInProgress, "generating TLDR")
	a.progress.PrintTree()

	tldr, err := GenerateTLDR(ctx, a.backend, GenerateTLDROptions{
		Outline:     a.outline,
		Research:    a.researchData,
		MaxBullets:  5,
		MaxTokens:   200,
		Temperature: 0.3,
		UseGrammar:  false, // Grammar not supported by all servers
	})
	if err != nil {
		return fmt.Errorf("failed to generate TLDR: %w", err)
	}

	a.tldr = tldr
	a.progress.AddTokens("Generate Sections", 200) // Estimated TLDR tokens

	// Save version with sections
	if err := a.saveVersion("Generate Sections", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	a.progress.UpdatePhase("Generate Sections", PhaseStatusDone, fmt.Sprintf("%d sections + TLDR", len(sections)))
	a.progress.PrintTree()

	return nil
}

// phaseAssemble - Phase 5: Assemble final post with TLDR, sections, and social posts
func (a *BlogContentAgent) phaseAssemble(ctx context.Context) error {
	a.progress.UpdatePhase("Assemble", PhaseStatusInProgress, "combining sections")
	a.progress.PrintTree()

	// Build final content
	var finalContent strings.Builder

	// Title
	if a.currentPost.Title != "" {
		finalContent.WriteString(fmt.Sprintf("# %s\n\n", a.currentPost.Title))
	}

	// TLDR
	finalContent.WriteString("## TL;DR\n\n")
	finalContent.WriteString(a.tldr)
	finalContent.WriteString("\n\n---\n\n")

	// Sections
	for _, section := range a.sections {
		finalContent.WriteString(fmt.Sprintf("## %s\n\n", section.Title))
		finalContent.WriteString(section.Content)
		finalContent.WriteString("\n\n")
	}

	// Generate title if not already set
	if a.currentPost.Title == "" || len(a.currentPost.Title) < 10 {
		a.progress.UpdatePhase("Assemble", PhaseStatusInProgress, "generating title")
		a.progress.PrintTree()

		title, tokens, err := a.generateTitle(ctx, finalContent.String())
		if err != nil {
			return fmt.Errorf("failed to generate title: %w", err)
		}
		a.currentPost.Title = title
		a.progress.AddTokens("Assemble", tokens)

		// Rebuild content with new title
		finalContent.Reset()
		finalContent.WriteString(fmt.Sprintf("# %s\n\n", a.currentPost.Title))
		finalContent.WriteString("## TL;DR\n\n")
		finalContent.WriteString(a.tldr)
		finalContent.WriteString("\n\n---\n\n")
		for _, section := range a.sections {
			finalContent.WriteString(fmt.Sprintf("## %s\n\n", section.Title))
			finalContent.WriteString(section.Content)
			finalContent.WriteString("\n\n")
		}
	}

	// Generate social media posts
	a.progress.UpdatePhase("Assemble", PhaseStatusInProgress, "generating social posts")
	a.progress.PrintTree()

	contentSummary := a.tldr + "\n\n" + a.outline

	platforms := []SocialMediaPlatform{PlatformTwitter, PlatformBluesky, PlatformLinkedIn}
	for _, platform := range platforms {
		post, err := GenerateSocialMediaPost(ctx, a.backend, SocialMediaPostOptions{
			Platform:    platform,
			Content:     contentSummary,
			Link:        "https://soypetetech.substack.com/p/SLUG", // Placeholder
			Temperature: 0.4,
			UseGrammar:  false, // Grammar not supported by all servers
		})
		if err != nil {
			return fmt.Errorf("failed to generate %s post: %w", platform, err)
		}

		a.socialPosts[string(platform)] = post
		a.progress.AddTokens("Assemble", 75) // Estimated tokens per social post
	}

	// Add "Stay Connected" section with O'Reilly link prominence
	finalContent.WriteString(a.buildStayConnectedSection())

	// Update post with final content
	a.currentPost.FinalContent = finalContent.String()
	a.currentPost.Status = blog.StatusDrafted
	if err := a.postStore.Update(a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version
	if err := a.saveVersion("Assemble", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	a.progress.UpdatePhase("Assemble", PhaseStatusDone, "final draft ready")
	a.progress.PrintTree()

	return nil
}

// phaseEditorReview - Phase 6: AI editor review for grammar and coherence
func (a *BlogContentAgent) phaseEditorReview(ctx context.Context) error {
	a.progress.UpdatePhase("Editor Review", PhaseStatusInProgress, "reviewing content")
	a.progress.PrintTree()

	// Optionally analyze writing style from RSS feed
	var styleGuide string
	if a.useStyleGuide && a.styleAnalyzer != nil {
		fmt.Println("\n📚 Analyzing writing style from Substack RSS feed...")
		var err error
		styleGuide, err = a.styleAnalyzer.AnalyzeStyle(ctx)
		if err != nil {
			fmt.Printf("⚠️  Warning: Style analysis failed: %v\n", err)
			fmt.Println("   Continuing with standard editor review...")
		} else {
			fmt.Println("✓ Style guide generated - will apply to editor review\n")
		}
	}

	baseSystemPrompt := `You are a technical blog editor focusing on clarity for generalist software engineers.

Review the blog post for:
1. Grammar and spelling errors
2. Clarity and readability
3. Technical accuracy
4. Logical flow between sections
5. Accessibility for generalist engineers (not just specialists)

Provide a brief review with:
- What works well
- Suggestions for improvement
- Any critical issues

Keep feedback concise and actionable.`

	// Enhance system prompt with style guide if available
	systemPrompt := baseSystemPrompt
	if styleGuide != "" {
		systemPrompt = a.styleAnalyzer.GetEditorPromptWithStyle(baseSystemPrompt)
	}

	userPrompt := fmt.Sprintf(`Review this blog post:

%s

Provide editorial feedback.`, a.currentPost.FinalContent)

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
		MaxTokens:    1000,
	}

	resp, err := a.backend.Infer(ctx, req)
	if err != nil {
		a.progress.UpdatePhase("Editor Review", PhaseStatusFailed, err.Error())
		a.progress.PrintTree()
		return fmt.Errorf("editor review failed: %w", err)
	}

	a.currentPost.EditorOutput = resp.Text
	a.currentPost.Status = blog.StatusEdited
	a.progress.AddTokens("Editor Review", resp.TokensUsed)

	if err := a.postStore.Update(a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version
	if err := a.saveVersion("Editor Review", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	a.progress.UpdatePhase("Editor Review", PhaseStatusDone, "review complete")
	a.progress.PrintTree()

	return nil
}

// phasePublish - Phase 7: Save to database and optionally publish to Notion
func (a *BlogContentAgent) phasePublish(ctx context.Context) error {
	a.progress.UpdatePhase("Publish", PhaseStatusInProgress, "saving to database")
	a.progress.PrintTree()

	// Final save with published status
	a.currentPost.Status = blog.StatusPublished
	a.currentPost.UpdatedAt = time.Now()
	a.currentPost.SocialPosts = a.socialPosts

	if err := a.postStore.Update(a.currentPost); err != nil {
		a.progress.UpdatePhase("Publish", PhaseStatusFailed, err.Error())
		a.progress.PrintTree()
		return fmt.Errorf("failed to publish post: %w", err)
	}

	// Save final version
	if err := a.saveVersion("Publish", blog.VersionTypeManualSave); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	// Publish to Notion if configured
	if a.config != nil && a.config.Blog.NotionPublishedDB != "" {
		if err := a.publishToNotion(ctx); err != nil {
			// Log error but don't fail the whole publish phase
			fmt.Printf("⚠️  Warning: Failed to publish to Notion: %v\n", err)
		}
	}

	a.progress.UpdatePhase("Publish", PhaseStatusDone, fmt.Sprintf("post ID: %s", a.currentPost.ID))
	a.progress.PrintTree()

	return nil
}

// publishToNotion creates a page in the Notion published database
func (a *BlogContentAgent) publishToNotion(ctx context.Context) error {
	// Note: This is a placeholder for Notion MCP integration
	// The actual MCP tool calls would happen here
	// For now, we'll just log that Notion publishing is configured

	fmt.Println("\n📝 Publishing to Notion...")
	fmt.Printf("   Database ID: %s\n", a.config.Blog.NotionPublishedDB)
	fmt.Printf("   Post Title: %s\n", a.currentPost.Title)

	// TODO: Use Notion MCP tools to create page
	// The MCP tools are available in the system, but we need to:
	// 1. Get NOTION_TOKEN from environment
	// 2. Call mcp__notion__notion-create-pages with proper formatting
	// 3. Store the Notion page URL in the blog post record

	// For now, just return success
	fmt.Println("   ✅ Notion publishing placeholder - integration pending")

	return nil
}

// Helper methods

func (a *BlogContentAgent) parseSectionsFromOutline(outline string) []string {
	var sections []string
	lines := strings.Split(outline, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for markdown h2 headings (##)
		if strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimPrefix(trimmed, "## ")
			sections = append(sections, title)
		}
	}

	return sections
}

func (a *BlogContentAgent) generateSection(ctx context.Context, title string, index int) (string, int, error) {
	baseSystemPrompt := `You are writing a section for a technical blog post.

Write clear, engaging content for generalist software engineers.
Use code examples where appropriate.
Keep paragraphs concise (2-4 sentences).
Use markdown formatting.

Do NOT include the section heading - just the content.`

	systemPrompt := a.enhancePromptWithStyle(baseSystemPrompt)

	userPrompt := fmt.Sprintf(`Write the "%s" section for this blog post:

OUTLINE:
%s

RESEARCH:
%s

TRANSCRIPTION:
%s

Write 2-4 paragraphs of clear, technical content.`, title, a.outline, a.researchData, a.currentPost.RawTranscription)

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.4,
		MaxTokens:    800,
	}

	resp, err := a.backend.Infer(ctx, req)
	if err != nil {
		return "", 0, err
	}

	return resp.Text, resp.TokensUsed, nil
}

// generateTitle creates an engaging blog post title based on the content
func (a *BlogContentAgent) generateTitle(ctx context.Context, content string) (string, int, error) {
	systemPrompt := `You are a blog post title generator for technical content.

Create an engaging, SEO-friendly title that:
1. Captures the main topic clearly
2. Is between 40-70 characters
3. Uses active voice
4. Appeals to generalist software engineers
5. Avoids clickbait or excessive hype

Output ONLY the title, nothing else. No quotes, no explanations.`

	// Truncate content for title generation (use TLDR + first section)
	contentPreview := content
	if len(content) > 2000 {
		contentPreview = content[:2000] + "..."
	}

	userPrompt := fmt.Sprintf(`Generate a compelling title for this blog post:

CONTENT PREVIEW:
%s

Output the title only.`, contentPreview)

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.5, // Balance creativity and consistency
		MaxTokens:    30,  // Titles should be short
	}

	resp, err := a.backend.Infer(ctx, req)
	if err != nil {
		return "", 0, fmt.Errorf("title generation failed: %w", err)
	}

	// Clean up the title (remove quotes if present)
	title := strings.TrimSpace(resp.Text)
	title = strings.Trim(title, "\"'")

	return title, resp.TokensUsed, nil
}

func (a *BlogContentAgent) buildStayConnectedSection() string {
	var section strings.Builder

	section.WriteString("---\n\n")
	section.WriteString("## Stay Connected\n\n")

	// O'Reilly link with prominence (first!)
	section.WriteString("**📚 Learn More:**\n")
	section.WriteString("- [My Go Programming Course on O'Reilly](https://learning.oreilly.com/) - Comprehensive Go training\n\n")

	// Social media posts
	section.WriteString("**Share this post:**\n\n")

	if twitter, ok := a.socialPosts["twitter"]; ok {
		section.WriteString(fmt.Sprintf("**Twitter:** %s\n\n", twitter))
	}

	if bluesky, ok := a.socialPosts["bluesky"]; ok {
		section.WriteString(fmt.Sprintf("**Bluesky:** %s\n\n", bluesky))
	}

	if linkedin, ok := a.socialPosts["linkedin"]; ok {
		section.WriteString(fmt.Sprintf("**LinkedIn:** %s\n\n", linkedin))
	}

	// Other links
	section.WriteString("**Connect:**\n")
	section.WriteString("- [Discord Community](https://discord.gg/soypete)\n")
	section.WriteString("- [YouTube](https://youtube.com/@soypete)\n")
	section.WriteString("- [Newsletter](https://soypetetech.substack.com)\n")
	section.WriteString("- [Twitter/X](https://twitter.com/soypete)\n\n")

	return section.String()
}

func (a *BlogContentAgent) saveVersion(phase string, versionType blog.VersionType) error {
	// Get next version number
	nextVersion, err := a.versionStore.GetNextVersionNumber(context.Background(), a.currentPost.ID)
	if err != nil {
		return err
	}

	// Build sections JSON
	sectionsData := make([]blog.Section, len(a.sections))
	for i, sec := range a.sections {
		sectionsData[i] = blog.Section{
			Title:   sec.Title,
			Content: sec.Content,
			Order:   sec.Order,
		}
	}

	version := &blog.PostVersion{
		ID:               uuid.New(),
		PostID:           a.currentPost.ID,
		VersionNumber:    nextVersion,
		VersionType:      versionType,
		Status:           a.currentPost.Status,
		Phase:            phase,
		PostTitle:        a.currentPost.Title,
		RawTranscription: a.currentPost.RawTranscription,
		Outline:          a.outline,
		Sections:         sectionsData,
		FullContent:      a.currentPost.FinalContent,
		CreatedBy:        "system",
		CreatedAt:        time.Now(),
	}

	return a.versionStore.CreateVersion(context.Background(), version)
}

// GetCurrentPost returns the current blog post
func (a *BlogContentAgent) GetCurrentPost() *blog.BlogPost {
	return a.currentPost
}

// GetSocialPosts returns generated social media posts
func (a *BlogContentAgent) GetSocialPosts() map[string]string {
	return a.socialPosts
}

// GetProgress returns the progress tracker
func (a *BlogContentAgent) GetProgress() *ProgressTracker {
	return a.progress
}
