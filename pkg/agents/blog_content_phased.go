package agents

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/storage/blog"
	"github.com/soypete/pedrocli/pkg/tools"
)

// BlogContentAgentPhased orchestrates the 7-phase blog post creation workflow using PhasedExecutor
// This replaces the original BlogContentAgent with a unified checkpoint/resume architecture
type BlogContentAgentPhased struct {
	backend       llm.Backend
	storage       blog.BlogStorage
	progress      *ProgressTracker
	currentPost   *blog.BlogPost
	config        *config.Config
	styleAnalyzer *BlogStyleAnalyzerAgent
	useStyleGuide bool
	baseAgent     *BaseAgent

	// Phase data stores structured outputs from each phase
	phaseData map[string]interface{}
}

// NewBlogContentAgentPhased creates a new phased blog content agent
func NewBlogContentAgentPhased(cfg BlogContentAgentConfig) *BlogContentAgentPhased {
	// Create progress tracker and add phases
	progress := NewProgressTracker()
	phases := []string{
		"Transcribe",
		"Analyze Style",
		"Research",
		"Outline",
		"Generate Sections", // Will expand into N section phases dynamically
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

	// Code introspection tools
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
		calendarTool = tools.NewCalendarTool(cfg.Config, nil)
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

	// Initialize style analyzer if RSS feed is configured
	var styleAnalyzer *BlogStyleAnalyzerAgent
	useStyleGuide := false
	if cfg.Config != nil && cfg.Config.Blog.RSSFeedURL != "" {
		styleAnalyzer = NewBlogStyleAnalyzerAgent(cfg.Backend, cfg.Config)
		useStyleGuide = true
	}

	// Create base agent for tool execution
	// Use NewBaseAgent to ensure tokenIDProvider is initialized for logit bias
	baseAgent := NewBaseAgent(
		"blog_content_phased",
		"Phased blog content generation with checkpoint/resume",
		cfg.Config,
		cfg.Backend,
		nil, // No job manager for CLI execution
	)
	// Set the registered tools
	baseAgent.tools = registeredTools

	agent := &BlogContentAgentPhased{
		backend:       cfg.Backend,
		storage:       cfg.Storage,
		progress:      progress,
		config:        cfg.Config,
		styleAnalyzer: styleAnalyzer,
		useStyleGuide: useStyleGuide,
		baseAgent:     baseAgent,
		phaseData:     make(map[string]interface{}),
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

// Execute runs the complete blog creation workflow using PhasedExecutor
func (a *BlogContentAgentPhased) Execute(ctx context.Context) error {
	fmt.Println("\n=== BlogContentAgentPhased: 7-Phase Workflow with Checkpoints ===")
	a.progress.PrintTree()

	// Create context manager for entire workflow
	contextMgr, err := llmcontext.NewManager(
		fmt.Sprintf("blog-%s", a.currentPost.ID.String()),
		a.config.Debug.Enabled,
		a.config.Model.ContextSize,
	)
	if err != nil {
		return fmt.Errorf("failed to create context manager: %w", err)
	}
	defer contextMgr.Cleanup()

	// Build phase definitions
	phases := a.buildPhases()

	// Create phased executor
	executor := NewPhasedExecutor(a.baseAgent, contextMgr, phases)

	// Set progress callback to update progress tracker
	executor.SetPhaseCallback(func(phase Phase, result *PhaseResult) (bool, error) {
		status := PhaseStatusDone
		if !result.Success {
			status = PhaseStatusFailed
		}

		a.progress.UpdatePhase(phase.Name, status, "")
		a.progress.AddTokens(phase.Name, result.RoundsUsed*100) // Rough estimate
		a.progress.PrintTree()

		return true, nil
	})

	// Build initial prompt
	initialPrompt := fmt.Sprintf(`Begin blog post creation workflow.

TRANSCRIPTION:
%s

TITLE: %s

Process this through all phases to create a complete blog post.`,
		a.currentPost.RawTranscription,
		a.currentPost.Title)

	// Execute all phases
	if err := executor.Execute(ctx, initialPrompt); err != nil {
		return fmt.Errorf("phased execution failed: %w", err)
	}

	fmt.Println("\n✅ All phases complete!")
	a.progress.PrintTree()

	return nil
}

// buildPhases defines all workflow phases with their configurations
func (a *BlogContentAgentPhased) buildPhases() []Phase {
	return []Phase{
		// Phase 1: Transcribe
		{
			Name:        "transcribe",
			Description: "Validate transcription input",
			SystemPrompt: `You are validating blog post transcription input. The transcription is provided below.

Your task: Confirm the transcription is present and signal completion with PHASE_COMPLETE.

Do NOT use any tools. Do NOT try to read or write files. Simply acknowledge the transcription and signal PHASE_COMPLETE.`,
			Tools:     []string{}, // No tools needed
			MaxRounds: 1,
			Validator: a.validateTranscribe,
		},

		// Phase 1.5: Analyze Style (optional, only if RSS feed configured)
		{
			Name:         "analyze_style",
			Description:  "Analyze writing style from RSS feed",
			SystemPrompt: a.buildStyleAnalysisPrompt(),
			Tools:        []string{"rss_feed"},
			MaxRounds:    10,
			Validator:    a.validateStyleAnalysis,
		},

		// Phase 2: Research
		{
			Name:         "research",
			Description:  "Gather research from web, RSS, calendar, code",
			SystemPrompt: a.buildResearchPrompt(),
			Tools:        []string{"web_search", "web_scraper", "search", "navigate", "file", "rss_feed", "calendar", "static_links"},
			MaxRounds:    20,
			Validator:    a.validateResearch,
		},

		// Phase 3: Outline
		{
			Name:           "outline",
			Description:    "Generate blog post outline",
			SystemPrompt:   a.buildOutlinePrompt(),
			Tools:          []string{}, // No tools for outline generation
			MaxRounds:      3,
			Validator:      a.validateOutline,
			PhaseGenerator: a.generateSectionPhases, // DYNAMIC PHASE INSERTION
		},

		// Phases 4.x: Sections (generated dynamically by outline phase)
		// Phase 5: TLDR (inserted after last section by generateSectionPhases)

		// Phase 6: Assemble
		{
			Name:         "assemble",
			Description:  "Combine sections, generate title and social posts",
			SystemPrompt: a.buildAssemblePrompt(),
			Tools:        []string{},
			MaxRounds:    5,
			Validator:    a.validateAssemble,
		},

		// Phase 7: Editor Review
		{
			Name:         "editor_review",
			Description:  "Review grammar, clarity, technical accuracy",
			SystemPrompt: a.buildEditorPrompt(),
			Tools:        []string{},
			MaxRounds:    3,
			Validator:    a.validateEditorReview,
		},

		// Phase 8: Publish
		{
			Name:        "publish",
			Description: "Save to storage and optionally publish to Notion",
			SystemPrompt: `You are completing the blog post publication workflow.

Your task: Signal completion with TASK_COMPLETE.

The blog post has been saved to storage. All work is complete. Do NOT use any tools.`,
			Tools:     []string{},
			MaxRounds: 1,
			Validator: a.validatePublish,
		},
	}
}

// Prompt builders for each phase

func (a *BlogContentAgentPhased) buildStyleAnalysisPrompt() string {
	if !a.useStyleGuide || a.styleAnalyzer == nil {
		return "" // Skip if not enabled
	}

	return `You are analyzing the author's writing style from their published blog posts.

WORKFLOW:
1. First, call rss_feed tool ONCE to get recent posts: {"tool": "rss_feed", "args": {"action": "get_configured"}}
2. After receiving RSS results, analyze them and output style guide
3. Signal completion with PHASE_COMPLETE

DO NOT call rss_feed multiple times. Fetch once, then analyze.

Identify from the posts:
1. Tone and voice characteristics
2. Common sentence structures
3. Vocabulary preferences
4. Technical depth level
5. Use of humor, analogies, or examples

COMPLETION FORMAT:
PHASE_COMPLETE

Style Guide:
[Your concise analysis here - 2-3 paragraphs max]`
}

func (a *BlogContentAgentPhased) buildResearchPrompt() string {
	return `You are a research assistant for technical blog writing.

CRITICAL: When writing about code, ALWAYS search the local codebase first using code introspection tools.

AVAILABLE TOOLS:
- web_search: Search the web for relevant articles and documentation
- web_scraper: Scrape content from URLs, GitHub repos, or local files
- search: Search for code patterns (grep, find files, find definitions)
- navigate: Navigate code structure (list dirs, get file outlines, analyze imports)
- file: Read/write files from the local codebase
- rss_feed: Get recent blog posts from RSS feed
- calendar: Get recent events and activities
- static_links: Get configured social media and newsletter links

WORKFLOW:
1. Analyze transcription to identify topics
2. IF writing about code/implementation:
   - Use search tool to find relevant functions/types
   - Use file to read actual source files
   - Use navigate to understand code structure
3. Use web_search for external articles/docs
4. Use rss_feed for recent related posts
5. When done, respond with "PHASE_COMPLETE" + summary

COMPLETION FORMAT:
PHASE_COMPLETE

Summary:
- [Finding 1 with file paths if code-related]
- [Finding 2]
- [Finding 3]`
}

func (a *BlogContentAgentPhased) buildOutlinePrompt() string {
	return `You are a technical blog post outliner.

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
Wrap up and call to action

When complete, output "PHASE_COMPLETE" followed by the outline.`
}

func (a *BlogContentAgentPhased) buildSectionPrompt(title string, index int) string {
	basePrompt := fmt.Sprintf(`You are writing a section for a technical blog post.

SECTION: %s (Section %d)

CRITICAL CODE EXAMPLES RULE:
- When writing about code, ALWAYS use search + file tools
- Find real code from the repository being documented
- Include file paths and line numbers (e.g., pkg/agents/executor.go:120-147)
- NEVER hallucinate code examples

AVAILABLE TOOLS:
- search: Find functions, types, patterns in codebase
- navigate: Explore code structure
- file: Read actual source files
- web_search: Find external references

WORKFLOW:
1. Determine if section needs code examples
2. IF code examples needed:
   - Use search tool to find relevant code
   - Use file to read complete implementations
   - Extract actual code snippets with file references
3. Write 2-4 paragraphs of clear content
4. Use markdown formatting
5. When done, respond with "PHASE_COMPLETE"

Do NOT include the section heading - just the content.

COMPLETION FORMAT:
PHASE_COMPLETE

[Section content with real code examples and file paths]`, title, index+1)

	return a.enhancePromptWithStyle(basePrompt)
}

func (a *BlogContentAgentPhased) buildTLDRPrompt() string {
	return `Generate a TL;DR summary for the blog post.

Requirements:
- 3-5 bullet points
- Each bullet 1-2 sentences max
- Focus on key takeaways
- Total ~200 words

Output only the bulleted list, no heading.

COMPLETION FORMAT:
PHASE_COMPLETE

- First key takeaway
- Second key takeaway
- Third key takeaway`
}

func (a *BlogContentAgentPhased) buildAssemblePrompt() string {
	return `Assemble the final blog post from all sections.

Tasks:
1. If no title exists, generate an SEO-friendly title (40-70 chars)
2. Combine all sections in order
3. Generate social media posts for Twitter, Bluesky, LinkedIn

Each social post should:
- Twitter: Max 280 chars, include hashtags
- Bluesky: Max 300 chars, conversational
- LinkedIn: Max 700 chars, professional tone

Output format:
TITLE: [generated title if needed]

TWITTER:
[tweet]

BLUESKY:
[post]

LINKEDIN:
[post]

PHASE_COMPLETE`
}

func (a *BlogContentAgentPhased) buildEditorPrompt() string {
	basePrompt := `You are a technical blog editor focusing on clarity for generalist software engineers.

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

Keep feedback concise and actionable.

COMPLETION FORMAT:
PHASE_COMPLETE

[Editorial feedback]`

	// Enhance with style guide if available
	if a.styleAnalyzer != nil && a.styleAnalyzer.GetStyleGuide() != "" {
		return a.styleAnalyzer.GetEditorPromptWithStyle(basePrompt)
	}

	return basePrompt
}

// Validators for each phase

func (a *BlogContentAgentPhased) validateTranscribe(result *PhaseResult) error {
	if a.currentPost.RawTranscription == "" {
		return fmt.Errorf("no transcription provided")
	}

	// Save initial post to storage
	ctx := context.Background()
	if err := a.storage.CreatePost(ctx, a.currentPost); err != nil {
		return fmt.Errorf("failed to save initial post: %w", err)
	}

	// Create initial version snapshot
	if err := a.saveVersion("Transcribe", blog.VersionTypeAutoSnapshot); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	return nil
}

func (a *BlogContentAgentPhased) validateStyleAnalysis(result *PhaseResult) error {
	if !a.useStyleGuide || a.styleAnalyzer == nil {
		return nil // Skip if not enabled
	}

	// Store style guide in phase data
	a.phaseData["style_guide"] = result.Output
	return nil
}

func (a *BlogContentAgentPhased) validateResearch(result *PhaseResult) error {
	// Extract research summary and store in phase data
	research := a.extractResearchSummary(result.Output)
	a.phaseData["research"] = research

	// Update post with research data
	ctx := context.Background()
	a.currentPost.WriterOutput = fmt.Sprintf("RESEARCH:\n%s", research)
	if err := a.storage.UpdatePost(ctx, a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version snapshot
	if err := a.saveVersion("Research", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	return nil
}

func (a *BlogContentAgentPhased) validateOutline(result *PhaseResult) error {
	// Parse sections from outline
	sections := a.parseSectionsFromOutline(result.Output)
	if len(sections) == 0 {
		return fmt.Errorf("no sections found in outline")
	}

	// Store outline and sections in phase data
	a.phaseData["outline"] = result.Output
	a.phaseData["outline_sections"] = sections

	// Update post
	ctx := context.Background()
	research := ""
	if r, ok := a.phaseData["research"].(string); ok {
		research = r
	}
	a.currentPost.WriterOutput = fmt.Sprintf("OUTLINE:\n%s\n\nRESEARCH:\n%s", result.Output, research)
	if err := a.storage.UpdatePost(ctx, a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version
	if err := a.saveVersion("Outline", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	return nil
}

func (a *BlogContentAgentPhased) validateSection(sectionIndex int) func(result *PhaseResult) error {
	return func(result *PhaseResult) error {
		// Extract section content
		content := a.extractSectionContent(result.Output)

		// Store in phase data
		sectionsKey := "sections"
		if _, ok := a.phaseData[sectionsKey]; !ok {
			a.phaseData[sectionsKey] = make([]SectionContent, 0)
		}

		sections := a.phaseData[sectionsKey].([]SectionContent)

		// Get section title from outline sections
		outlineSections := a.phaseData["outline_sections"].([]string)
		if sectionIndex >= len(outlineSections) {
			return fmt.Errorf("section index %d out of range", sectionIndex)
		}

		sections = append(sections, SectionContent{
			Title:   outlineSections[sectionIndex],
			Content: content,
			Order:   sectionIndex,
		})

		a.phaseData[sectionsKey] = sections

		return nil
	}
}

func (a *BlogContentAgentPhased) validateTLDR(result *PhaseResult) error {
	// Store TLDR in phase data
	a.phaseData["tldr"] = result.Output
	return nil
}

func (a *BlogContentAgentPhased) validateAssemble(result *PhaseResult) error {
	// Parse assembled content to extract title and social posts
	// Expected format:
	// TITLE: [title]
	// TWITTER: [post]
	// BLUESKY: [post]
	// LINKEDIN: [post]

	lines := strings.Split(result.Output, "\n")
	socialPosts := make(map[string]string)
	var currentPlatform string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "TITLE:") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "TITLE:"))
			if title != "" {
				a.currentPost.Title = title
			}
		} else if strings.HasPrefix(trimmed, "TWITTER:") {
			currentPlatform = "twitter"
			socialPosts[currentPlatform] = ""
		} else if strings.HasPrefix(trimmed, "BLUESKY:") {
			currentPlatform = "bluesky"
			socialPosts[currentPlatform] = ""
		} else if strings.HasPrefix(trimmed, "LINKEDIN:") {
			currentPlatform = "linkedin"
			socialPosts[currentPlatform] = ""
		} else if currentPlatform != "" && trimmed != "" && !strings.Contains(trimmed, "PHASE_COMPLETE") {
			if socialPosts[currentPlatform] != "" {
				socialPosts[currentPlatform] += "\n"
			}
			socialPosts[currentPlatform] += trimmed
		}
	}

	// Store social posts
	a.phaseData["social_posts"] = socialPosts

	// Build final content from sections and TLDR
	var finalContent strings.Builder

	if a.currentPost.Title != "" {
		finalContent.WriteString(fmt.Sprintf("# %s\n\n", a.currentPost.Title))
	}

	// TLDR
	if tldr, ok := a.phaseData["tldr"].(string); ok {
		finalContent.WriteString("## TL;DR\n\n")
		finalContent.WriteString(tldr)
		finalContent.WriteString("\n\n---\n\n")
	}

	// Sections
	if sections, ok := a.phaseData["sections"].([]SectionContent); ok {
		for _, section := range sections {
			finalContent.WriteString(fmt.Sprintf("## %s\n\n", section.Title))
			finalContent.WriteString(section.Content)
			finalContent.WriteString("\n\n")
		}
	}

	// Add Stay Connected section
	finalContent.WriteString(a.buildStayConnectedSection(socialPosts))

	// Update post
	ctx := context.Background()
	a.currentPost.FinalContent = finalContent.String()
	a.currentPost.Status = blog.StatusDrafted
	a.currentPost.SocialPosts = socialPosts
	if err := a.storage.UpdatePost(ctx, a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version
	if err := a.saveVersion("Assemble", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	return nil
}

func (a *BlogContentAgentPhased) validateEditorReview(result *PhaseResult) error {
	ctx := context.Background()
	a.currentPost.EditorOutput = result.Output
	a.currentPost.Status = blog.StatusEdited

	if err := a.storage.UpdatePost(ctx, a.currentPost); err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Save version
	if err := a.saveVersion("Editor Review", blog.VersionTypePhaseResult); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	return nil
}

func (a *BlogContentAgentPhased) validatePublish(result *PhaseResult) error {
	ctx := context.Background()
	a.currentPost.Status = blog.StatusPublished
	a.currentPost.UpdatedAt = time.Now()

	if err := a.storage.UpdatePost(ctx, a.currentPost); err != nil {
		return fmt.Errorf("failed to publish post: %w", err)
	}

	// Save final version
	if err := a.saveVersion("Publish", blog.VersionTypeManualSave); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	// Publish to Notion if configured
	if a.config != nil && a.config.Blog.NotionPublishedDB != "" {
		if err := a.publishToNotion(ctx); err != nil {
			// Log error but don't fail
			fmt.Printf("⚠️  Warning: Failed to publish to Notion: %v\n", err)
		}
	}

	return nil
}

// generateSectionPhases dynamically creates phases for each section from the outline
func (a *BlogContentAgentPhased) generateSectionPhases(result *PhaseResult) ([]Phase, error) {
	// Get sections from phase data (populated by validateOutline)
	sectionsInterface, ok := a.phaseData["outline_sections"]
	if !ok {
		return nil, fmt.Errorf("outline_sections not found in phase data")
	}

	sections, ok := sectionsInterface.([]string)
	if !ok {
		return nil, fmt.Errorf("outline_sections has wrong type")
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections to generate")
	}

	// Create one phase per section
	sectionPhases := make([]Phase, 0, len(sections)+1)
	for i, title := range sections {
		sectionPhases = append(sectionPhases, Phase{
			Name:         fmt.Sprintf("section_%d", i),
			Description:  fmt.Sprintf("Generate section: %s", title),
			SystemPrompt: a.buildSectionPrompt(title, i),
			Tools:        []string{"search", "navigate", "file", "web_search"},
			MaxRounds:    15,
			Validator:    a.validateSection(i),
		})
	}

	// Add TLDR phase at the end
	sectionPhases = append(sectionPhases, Phase{
		Name:         "tldr",
		Description:  "Generate TLDR summary",
		SystemPrompt: a.buildTLDRPrompt(),
		Tools:        []string{},
		MaxRounds:    3,
		Validator:    a.validateTLDR,
	})

	fmt.Fprintf(os.Stderr, "\n📋 Generated %d section phases + TLDR from outline\n", len(sections))

	return sectionPhases, nil
}

// Helper methods (reused from original implementation)

func (a *BlogContentAgentPhased) enhancePromptWithStyle(basePrompt string) string {
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

func (a *BlogContentAgentPhased) buildStayConnectedSection(socialPosts map[string]string) string {
	var section strings.Builder

	section.WriteString("---\n\n")
	section.WriteString("## Stay Connected\n\n")

	// O'Reilly link with prominence
	section.WriteString("**📚 Learn More:**\n")
	section.WriteString("- [My Go Programming Course on O'Reilly](https://learning.oreilly.com/) - Comprehensive Go training\n\n")

	// Social media posts
	section.WriteString("**Share this post:**\n\n")

	if twitter, ok := socialPosts["twitter"]; ok && twitter != "" {
		section.WriteString(fmt.Sprintf("**Twitter:** %s\n\n", twitter))
	}

	if bluesky, ok := socialPosts["bluesky"]; ok && bluesky != "" {
		section.WriteString(fmt.Sprintf("**Bluesky:** %s\n\n", bluesky))
	}

	if linkedin, ok := socialPosts["linkedin"]; ok && linkedin != "" {
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

func (a *BlogContentAgentPhased) parseSectionsFromOutline(outline string) []string {
	var sections []string
	lines := strings.Split(outline, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Try multiple formats
		if strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimPrefix(trimmed, "## ")
			sections = append(sections, title)
		} else if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimPrefix(trimmed, "# ")
			sections = append(sections, title)
		} else if len(trimmed) > 0 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed, ".") {
			parts := strings.SplitN(trimmed, ".", 2)
			if len(parts) == 2 {
				title := strings.TrimSpace(parts[1])
				if len(title) > 0 {
					sections = append(sections, title)
				}
			}
		}
	}

	return sections
}

func (a *BlogContentAgentPhased) saveVersion(phase string, versionType blog.VersionType) error {
	ctx := context.Background()

	nextVersion, err := a.storage.GetNextVersionNumber(ctx, a.currentPost.ID)
	if err != nil {
		return err
	}

	// Build sections JSON
	var sectionsData []blog.Section
	if sections, ok := a.phaseData["sections"].([]SectionContent); ok {
		sectionsData = make([]blog.Section, len(sections))
		for i, sec := range sections {
			sectionsData[i] = blog.Section{
				Title:   sec.Title,
				Content: sec.Content,
				Order:   sec.Order,
			}
		}
	}

	outline := ""
	if o, ok := a.phaseData["outline"].(string); ok {
		outline = o
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
		Outline:          outline,
		Sections:         sectionsData,
		FullContent:      a.currentPost.FinalContent,
		CreatedBy:        "system",
		CreatedAt:        time.Now(),
	}

	return a.storage.CreateVersion(ctx, version)
}

func (a *BlogContentAgentPhased) publishToNotion(ctx context.Context) error {
	fmt.Println("\n📝 Publishing to Notion...")
	fmt.Printf("   Database ID: %s\n", a.config.Blog.NotionPublishedDB)
	fmt.Printf("   Post Title: %s\n", a.currentPost.Title)
	fmt.Println("   ✅ Notion publishing placeholder - integration pending")
	return nil
}

func (a *BlogContentAgentPhased) extractResearchSummary(history string) string {
	if idx := strings.Index(history, "PHASE_COMPLETE"); idx != -1 {
		remaining := history[idx+len("PHASE_COMPLETE"):]
		if userIdx := strings.Index(remaining, "\n\n---\nUser:"); userIdx != -1 {
			remaining = remaining[:userIdx]
		}
		summary := strings.TrimSpace(remaining)
		if len(summary) > 0 {
			return summary
		}
	}

	// Fallback: Extract last assistant response
	return strings.TrimSpace(history)
}

func (a *BlogContentAgentPhased) extractSectionContent(history string) string {
	if idx := strings.Index(history, "SECTION_COMPLETE"); idx != -1 {
		remaining := history[idx+len("SECTION_COMPLETE"):]
		if userIdx := strings.Index(remaining, "\n\n---\nUser:"); userIdx != -1 {
			remaining = remaining[:userIdx]
		}
		content := strings.TrimSpace(remaining)
		if len(content) > 0 {
			return content
		}
	}

	// Fallback
	return strings.TrimSpace(history)
}

// Public API methods

func (a *BlogContentAgentPhased) GetCurrentPost() *blog.BlogPost {
	return a.currentPost
}

func (a *BlogContentAgentPhased) GetSocialPosts() map[string]string {
	if posts, ok := a.phaseData["social_posts"].(map[string]string); ok {
		return posts
	}
	return make(map[string]string)
}

func (a *BlogContentAgentPhased) GetProgress() *ProgressTracker {
	return a.progress
}
