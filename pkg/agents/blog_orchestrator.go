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
	"github.com/soypete/pedrocli/pkg/tools"
)

//go:embed prompts/blog_orchestrator_system.md
var blogOrchestratorSystemPrompt string

// BlogPromptAnalysis represents the parsed structure of a complex blog prompt
type BlogPromptAnalysis struct {
	MainTopic          string           `json:"main_topic"`
	ContentSections    []ContentSection `json:"content_sections"`
	ResearchTasks      []ResearchTask   `json:"research_tasks"`
	IncludeNewsletter  bool             `json:"include_newsletter"`
	EstimatedWordCount int              `json:"estimated_word_count"`
}

// ContentSection represents a section to be written
type ContentSection struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
}

// ResearchTask represents a research task to execute
type ResearchTask struct {
	Type   string                 `json:"type"` // calendar, rss_feed, static_links
	Params map[string]interface{} `json:"params,omitempty"`
}

// BlogOrchestratorOutput represents the final output from orchestration
type BlogOrchestratorOutput struct {
	Analysis       *BlogPromptAnalysis    `json:"analysis"`
	ResearchData   map[string]interface{} `json:"research_data"`
	Outline        string                 `json:"outline"`
	ExpandedDraft  string                 `json:"expanded_draft"`
	Newsletter     string                 `json:"newsletter,omitempty"`
	FullContent    string                 `json:"full_content"`
	SocialPosts    map[string]string      `json:"social_posts,omitempty"`
	SuggestedTitle string                 `json:"suggested_title,omitempty"`
	NotionURL      string                 `json:"notion_url,omitempty"`
	NotionPageID   string                 `json:"notion_page_id,omitempty"`
	Published      bool                   `json:"published"`
}

// BlogOrchestratorAgent handles complex multi-step blog prompts
type BlogOrchestratorAgent struct {
	*BaseAgent
	researchTools map[string]tools.Tool
	notionTool    tools.Tool // For publishing to Notion
}

// NewBlogOrchestratorAgent creates a new blog orchestrator agent
func NewBlogOrchestratorAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *BlogOrchestratorAgent {
	base := NewBaseAgent(
		"blog_orchestrator",
		"Orchestrate complex multi-step blog creation with research and outline-first generation",
		cfg,
		backend,
		jobMgr,
	)

	return &BlogOrchestratorAgent{
		BaseAgent:     base,
		researchTools: make(map[string]tools.Tool),
	}
}

// RegisterResearchTool registers a research tool for the orchestrator
func (o *BlogOrchestratorAgent) RegisterResearchTool(tool tools.Tool) {
	o.researchTools[tool.Name()] = tool
}

// RegisterNotionTool registers the Notion publishing tool
func (o *BlogOrchestratorAgent) RegisterNotionTool(tool tools.Tool) {
	o.notionTool = tool
}

// Execute executes the blog orchestrator asynchronously
func (o *BlogOrchestratorAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	prompt, ok := input["prompt"].(string)
	if !ok {
		// Also accept "dictation" as input key
		prompt, ok = input["dictation"].(string)
		if !ok {
			return nil, fmt.Errorf("missing 'prompt' or 'dictation' in input")
		}
	}

	title, _ := input["title"].(string)
	if title == "" {
		title = "Orchestrated Blog Post"
	}

	// Register research_links tool if provided
	if researchLinks, ok := input["research_links"].([]tools.ResearchLink); ok && len(researchLinks) > 0 {
		plainNotes, _ := input["plain_notes"].(string)
		researchLinksTool := tools.NewResearchLinksToolFromLinks(researchLinks, plainNotes)
		o.RegisterResearchTool(researchLinksTool)
	}

	// Create job
	job, err := o.jobManager.Create(ctx, "blog_orchestrator", "Orchestrate: "+title, input)
	if err != nil {
		return nil, err
	}

	o.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run orchestration in background
	go func() {
		bgCtx := context.Background()

		contextMgr, err := llmcontext.NewManager(job.ID, o.config.Debug.Enabled)
		if err != nil {
			o.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		result, err := o.runOrchestration(bgCtx, contextMgr, prompt, input)
		if err != nil {
			o.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Convert result to output map
		output := map[string]interface{}{
			"status":          "completed",
			"analysis":        result.Analysis,
			"research_data":   result.ResearchData,
			"outline":         result.Outline,
			"expanded_draft":  result.ExpandedDraft,
			"newsletter":      result.Newsletter,
			"full_content":    result.FullContent,
			"social_posts":    result.SocialPosts,
			"suggested_title": result.SuggestedTitle,
			"job_dir":         contextMgr.GetJobDir(),
			"notion_url":      result.NotionURL,
			"notion_page_id":  result.NotionPageID,
			"published":       result.Published,
		}

		o.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// runOrchestration executes the multi-phase orchestration process
func (o *BlogOrchestratorAgent) runOrchestration(ctx context.Context, contextMgr *llmcontext.Manager, prompt string, input map[string]interface{}) (*BlogOrchestratorOutput, error) {
	result := &BlogOrchestratorOutput{
		ResearchData: make(map[string]interface{}),
		SocialPosts:  make(map[string]string),
	}

	// Phase 1: Analyze prompt
	analysis, err := o.analyzePrompt(ctx, contextMgr, prompt)
	if err != nil {
		return nil, fmt.Errorf("phase 1 (analyze prompt) failed: %w", err)
	}
	result.Analysis = analysis
	result.SuggestedTitle = analysis.MainTopic

	// Phase 2: Execute research tasks
	researchData, err := o.executeResearch(ctx, analysis.ResearchTasks)
	if err != nil {
		// Log error but continue - research is optional
		researchData = map[string]interface{}{"error": err.Error()}
	}
	result.ResearchData = researchData

	// Phase 3: Generate outline
	outline, err := o.generateOutline(ctx, contextMgr, prompt, analysis, researchData)
	if err != nil {
		return nil, fmt.Errorf("phase 3 (generate outline) failed: %w", err)
	}
	result.Outline = outline

	// Phase 4: Expand sections
	expandedContent, err := o.expandSections(ctx, contextMgr, outline, analysis, researchData)
	if err != nil {
		return nil, fmt.Errorf("phase 4 (expand sections) failed: %w", err)
	}
	result.ExpandedDraft = expandedContent

	// Phase 5: Assemble final post with newsletter
	fullContent := expandedContent
	if analysis.IncludeNewsletter {
		newsletter := o.buildNewsletter(researchData)
		result.Newsletter = newsletter
		fullContent = expandedContent + "\n\n---\n\n" + newsletter
	}
	result.FullContent = fullContent

	// Phase 6: Generate social posts
	socialPosts, err := o.generateSocialPosts(ctx, contextMgr, expandedContent)
	if err == nil {
		result.SocialPosts = socialPosts
	} else {
		// Log social post generation error in output for debugging
		result.SocialPosts = map[string]string{"error": err.Error()}
	}

	// Phase 7: Publish to Notion (if requested)
	shouldPublish, _ := input["publish"].(bool)
	if shouldPublish {
		if o.notionTool == nil {
			result.NotionURL = "publish failed: notion tool not registered"
		} else {
			notionURL, pageID, err := o.publishToNotion(ctx, result)
			if err != nil {
				// Log error but don't fail - publishing is optional
				result.NotionURL = fmt.Sprintf("publish failed: %v", err)
			} else {
				result.NotionURL = notionURL
				result.NotionPageID = pageID
				result.Published = true
			}
		}
	} else {
		result.NotionURL = "publish=false: skipped Notion publishing"
	}

	return result, nil
}

// analyzePrompt analyzes the complex prompt and identifies tasks
func (o *BlogOrchestratorAgent) analyzePrompt(ctx context.Context, contextMgr *llmcontext.Manager, prompt string) (*BlogPromptAnalysis, error) {
	analysisPrompt := fmt.Sprintf(`Analyze this blog prompt and identify what needs to be done.

# User's Blog Prompt
%s

# Instructions
Parse the prompt and identify:
1. The main topic/theme
2. Content sections that should be written
3. Research tasks needed (calendar events, RSS posts, static links)
4. Whether a newsletter section should be included
5. Estimated word count

Output ONLY valid JSON matching this structure:
{
  "main_topic": "string",
  "content_sections": [{"title": "string", "description": "string", "priority": 1}],
  "research_tasks": [{"type": "calendar|rss_feed|static_links", "params": {"action": "..."}}],
  "include_newsletter": true,
  "estimated_word_count": 1500
}`, prompt)

	// Save and execute
	if err := contextMgr.SavePrompt(analysisPrompt); err != nil {
		return nil, err
	}

	result, err := o.executeInference(ctx, analysisPrompt)
	if err != nil {
		return nil, err
	}

	if err := contextMgr.SaveResponse(result.Text); err != nil {
		return nil, err
	}

	// Parse JSON response
	var analysis BlogPromptAnalysis
	jsonStr := extractJSON(result.Text)
	if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
		// Try to extract from code blocks
		jsonStr = extractJSONFromCodeBlock(result.Text)
		if err := json.Unmarshal([]byte(jsonStr), &analysis); err != nil {
			return nil, fmt.Errorf("failed to parse analysis: %w", err)
		}
	}

	return &analysis, nil
}

// executeResearch executes research tasks using registered tools
func (o *BlogOrchestratorAgent) executeResearch(ctx context.Context, tasks []ResearchTask) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	for _, task := range tasks {
		tool, exists := o.researchTools[task.Type]
		if !exists {
			data[task.Type+"_error"] = fmt.Sprintf("tool not registered: %s", task.Type)
			continue
		}

		args := task.Params
		if args == nil {
			args = make(map[string]interface{})
		}

		result, err := tool.Execute(ctx, args)
		if err != nil {
			data[task.Type+"_error"] = err.Error()
			continue
		}

		if result.Success {
			data[task.Type] = result.Output
			if result.Data != nil {
				data[task.Type+"_data"] = result.Data
			}
		} else {
			data[task.Type+"_error"] = result.Error
		}
	}

	return data, nil
}

// generateOutline generates a detailed outline for the blog post
func (o *BlogOrchestratorAgent) generateOutline(ctx context.Context, contextMgr *llmcontext.Manager, prompt string, analysis *BlogPromptAnalysis, researchData map[string]interface{}) (string, error) {
	// Format research data for the prompt
	researchStr := formatResearchData(researchData)

	outlinePrompt := fmt.Sprintf(`Generate a detailed outline for this blog post.

# Topic
%s

# Sections to Include
%s

# Research Data Available
%s

# Original Prompt
%s

# Instructions
Create a markdown outline with:
- Main sections with ## headers
- Key points under each section (bullet points)
- Where to incorporate research data
- Approximate word count per section

Output the outline in markdown format.`,
		analysis.MainTopic,
		formatSections(analysis.ContentSections),
		researchStr,
		prompt)

	if err := contextMgr.SavePrompt(outlinePrompt); err != nil {
		return "", err
	}

	result, err := o.executeInference(ctx, outlinePrompt)
	if err != nil {
		return "", err
	}

	if err := contextMgr.SaveResponse(result.Text); err != nil {
		return "", err
	}

	return result.Text, nil
}

// expandSections expands each section of the outline
func (o *BlogOrchestratorAgent) expandSections(ctx context.Context, contextMgr *llmcontext.Manager, outline string, analysis *BlogPromptAnalysis, researchData map[string]interface{}) (string, error) {
	// For shorter posts, expand all at once
	if analysis.EstimatedWordCount <= 1500 {
		return o.expandAllAtOnce(ctx, contextMgr, outline, researchData)
	}

	// For longer posts, expand section by section
	sections := extractSectionsFromOutline(outline)
	if len(sections) == 0 {
		// Fallback to expanding all at once
		return o.expandAllAtOnce(ctx, contextMgr, outline, researchData)
	}

	var expandedContent strings.Builder
	researchStr := formatResearchData(researchData)

	for i, section := range sections {
		sectionPrompt := fmt.Sprintf(`Expand this section of the blog post.

# Section %d: %s

# Full Outline Context
%s

# Research Data to Incorporate
%s

# Instructions
Write the full content for this section:
- Maintain narrative flow
- Use conversational but authoritative tone
- Include relevant research data where appropriate
- Use markdown formatting

Output ONLY the section content in markdown (no extra commentary).`,
			i+1, section, outline, researchStr)

		if err := contextMgr.SavePrompt(sectionPrompt); err != nil {
			return "", err
		}

		result, err := o.executeInference(ctx, sectionPrompt)
		if err != nil {
			return "", fmt.Errorf("failed to expand section %s: %w", section, err)
		}

		if err := contextMgr.SaveResponse(result.Text); err != nil {
			return "", err
		}

		expandedContent.WriteString(result.Text)
		expandedContent.WriteString("\n\n")
	}

	return expandedContent.String(), nil
}

// expandAllAtOnce expands the entire outline at once
func (o *BlogOrchestratorAgent) expandAllAtOnce(ctx context.Context, contextMgr *llmcontext.Manager, outline string, researchData map[string]interface{}) (string, error) {
	researchStr := formatResearchData(researchData)

	expandPrompt := fmt.Sprintf(`Expand this outline into a full blog post.

# Outline
%s

# Research Data to Incorporate
%s

# Instructions
Write the full blog post:
- Follow the outline structure
- Use conversational but authoritative tone
- Include research data where relevant
- Add smooth transitions between sections
- Start with a compelling hook
- End with a clear call-to-action
- Use markdown formatting

Output ONLY the blog post content in markdown (no extra commentary).`,
		outline, researchStr)

	if err := contextMgr.SavePrompt(expandPrompt); err != nil {
		return "", err
	}

	result, err := o.executeInference(ctx, expandPrompt)
	if err != nil {
		return "", err
	}

	if err := contextMgr.SaveResponse(result.Text); err != nil {
		return "", err
	}

	return result.Text, nil
}

// buildNewsletter builds the newsletter section from research data
func (o *BlogOrchestratorAgent) buildNewsletter(researchData map[string]interface{}) string {
	var newsletter strings.Builder

	newsletter.WriteString("## Newsletter Highlights\n\n")

	// YouTube Placeholder
	if placeholder, ok := researchData["static_links_data"].(map[string]interface{}); ok {
		if links, ok := placeholder["links"].(map[string]interface{}); ok {
			if ytPlaceholder, ok := links["youtube_placeholder"].(string); ok && ytPlaceholder != "" {
				newsletter.WriteString("### Featured Video\n\n")
				newsletter.WriteString(ytPlaceholder)
				newsletter.WriteString("\n\n")
			}
		}
	}

	// Calendar events
	if events, ok := researchData["calendar"].(string); ok && events != "" {
		newsletter.WriteString("### Upcoming Events\n\n")
		newsletter.WriteString(events)
		newsletter.WriteString("\n\n")
	}

	// RSS posts
	if posts, ok := researchData["rss_feed"].(string); ok && posts != "" {
		newsletter.WriteString("### Recent Posts You Might Have Missed\n\n")
		// Try to parse and format nicely
		var feed struct {
			Items []struct {
				Title string `json:"title"`
				Link  string `json:"link"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(posts), &feed); err == nil && len(feed.Items) > 0 {
			for _, item := range feed.Items {
				newsletter.WriteString(fmt.Sprintf("- [%s](%s)\n", item.Title, item.Link))
			}
		} else {
			newsletter.WriteString(posts)
		}
		newsletter.WriteString("\n")
	}

	// Static links
	if links, ok := researchData["static_links"].(string); ok && links != "" {
		newsletter.WriteString("### Stay Connected\n\n")
		// Try to parse and format nicely
		var staticLinks struct {
			All map[string]string `json:"all"`
		}
		if err := json.Unmarshal([]byte(links), &staticLinks); err == nil && len(staticLinks.All) > 0 {
			for name, url := range staticLinks.All {
				newsletter.WriteString(fmt.Sprintf("- [%s](%s)\n", name, url))
			}
		} else {
			newsletter.WriteString(links)
		}
		newsletter.WriteString("\n")
	}

	return newsletter.String()
}

// generateSocialPosts generates social media posts for the content
func (o *BlogOrchestratorAgent) generateSocialPosts(ctx context.Context, contextMgr *llmcontext.Manager, content string) (map[string]string, error) {
	// Truncate content if too long
	contentPreview := content
	if len(contentPreview) > 2000 {
		contentPreview = contentPreview[:2000] + "..."
	}

	socialPrompt := fmt.Sprintf(`Generate social media posts for this blog content.

# Blog Content Preview
%s

# Instructions
Generate promotional posts for each platform:
- Twitter/X: Under 280 chars, engaging, with hashtags
- LinkedIn: 2-3 paragraphs, professional tone
- Bluesky: Under 300 chars, casual tone

Output as JSON:
{
  "twitter_post": "...",
  "linkedin_post": "...",
  "bluesky_post": "..."
}`, contentPreview)

	result, err := o.executeInference(ctx, socialPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var social map[string]string
	jsonStr := extractJSON(result.Text)
	if err := json.Unmarshal([]byte(jsonStr), &social); err != nil {
		jsonStr = extractJSONFromCodeBlock(result.Text)
		if err := json.Unmarshal([]byte(jsonStr), &social); err != nil {
			return nil, err
		}
	}

	return social, nil
}

// publishToNotion publishes the blog post to Notion using the registered tool
func (o *BlogOrchestratorAgent) publishToNotion(ctx context.Context, result *BlogOrchestratorOutput) (string, string, error) {
	if o.notionTool == nil {
		return "", "", fmt.Errorf("notion tool not registered")
	}

	// Build arguments for the blog_publish tool
	args := map[string]interface{}{
		"title":          result.SuggestedTitle,
		"expanded_draft": result.FullContent,
	}

	// Add social posts if available
	if result.SocialPosts != nil {
		if twitter, ok := result.SocialPosts["twitter_post"]; ok {
			args["twitter_post"] = twitter
		}
		if linkedin, ok := result.SocialPosts["linkedin_post"]; ok {
			args["linkedin_post"] = linkedin
		}
		if bluesky, ok := result.SocialPosts["bluesky_post"]; ok {
			args["bluesky_post"] = bluesky
		}
	}

	// Execute the tool
	toolResult, err := o.notionTool.Execute(ctx, args)
	if err != nil {
		return "", "", fmt.Errorf("notion tool execution failed: %w", err)
	}

	if !toolResult.Success {
		return "", "", fmt.Errorf("notion publish failed: %s", toolResult.Error)
	}

	// Extract page ID from output or data
	var pageID string
	if toolResult.Data != nil {
		if id, ok := toolResult.Data["page_id"].(string); ok {
			pageID = id
		}
	}

	// Construct URL from page ID
	notionURL := ""
	if pageID != "" {
		notionURL = fmt.Sprintf("https://www.notion.so/%s", strings.ReplaceAll(pageID, "-", ""))
	}

	return notionURL, pageID, nil
}

// executeInference performs a single inference call
func (o *BlogOrchestratorAgent) executeInference(ctx context.Context, userPrompt string) (*llm.InferenceResponse, error) {
	budget := llm.CalculateBudget(o.config, blogOrchestratorSystemPrompt, userPrompt, "")

	req := &llm.InferenceRequest{
		SystemPrompt: blogOrchestratorSystemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.7, // Higher for creative writing
		MaxTokens:    budget.Available,
	}

	return o.llm.Infer(ctx, req)
}

// Helper functions

func extractJSON(text string) string {
	// Find first { and last }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return text
	}
	return text[start : end+1]
}

func extractJSONFromCodeBlock(text string) string {
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return extractJSON(text)
}

func formatSections(sections []ContentSection) string {
	var sb strings.Builder
	for i, s := range sections {
		sb.WriteString(fmt.Sprintf("%d. %s: %s (priority: %d)\n", i+1, s.Title, s.Description, s.Priority))
	}
	return sb.String()
}

func formatResearchData(data map[string]interface{}) string {
	if len(data) == 0 {
		return "No research data available"
	}

	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(formatted)
}

func extractSectionsFromOutline(outline string) []string {
	var sections []string
	re := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	matches := re.FindAllStringSubmatch(outline, -1)
	for _, match := range matches {
		if len(match) > 1 {
			sections = append(sections, match[1])
		}
	}
	return sections
}
