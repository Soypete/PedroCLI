package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
)

// BlogNotionTool publishes blog posts to Notion with AI-expanded content
type BlogNotionTool struct {
	apiKey     string
	databaseID string
	projectID  string
	httpClient *http.Client
}

// NewBlogNotionTool creates a new blog notion tool
func NewBlogNotionTool(cfg *config.Config) *BlogNotionTool {
	return &BlogNotionTool{
		apiKey:     os.Getenv("NOTION_TOKEN"),
		databaseID: cfg.Blog.NotionDraftsDB,
		projectID:  cfg.Blog.NotionIdeasDB,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *BlogNotionTool) Name() string {
	return "blog_publish"
}

func (t *BlogNotionTool) Description() string {
	return "Publish an AI-expanded blog draft to Notion with title suggestions, tags, and social media posts"
}

func (t *BlogNotionTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Required: title and expanded_draft
	title, ok := args["title"].(string)
	if !ok || title == "" {
		return &Result{Success: false, Error: "title is required"}, nil
	}

	expandedDraft, ok := args["expanded_draft"].(string)
	if !ok || expandedDraft == "" {
		return &Result{Success: false, Error: "expanded_draft is required"}, nil
	}

	if t.apiKey == "" {
		return &Result{Success: false, Error: "NOTION_TOKEN environment variable not set"}, nil
	}

	if t.databaseID == "" {
		return &Result{Success: false, Error: "blog.notion_drafts_db not configured in .pedrocli.json"}, nil
	}

	// Optional fields
	originalDictation, _ := args["original_dictation"].(string)
	suggestedTitles := toStringSlice(args["suggested_titles"])
	substackTags := toStringSlice(args["substack_tags"])
	twitterPost, _ := args["twitter_post"].(string)
	linkedinPost, _ := args["linkedin_post"].(string)
	blueskyPost, _ := args["bluesky_post"].(string)
	keyTakeaways := toStringSlice(args["key_takeaways"])
	targetAudience, _ := args["target_audience"].(string)
	readTime, _ := args["read_time"].(string)

	// Create the page in Notion with all sections
	pageID, err := t.createNotionPage(ctx, title, expandedDraft, originalDictation,
		suggestedTitles, substackTags, twitterPost, linkedinPost, blueskyPost,
		keyTakeaways, targetAudience, readTime)
	if err != nil {
		return &Result{Success: false, Error: fmt.Sprintf("failed to create Notion page: %v", err)}, nil
	}

	notionURL := fmt.Sprintf("https://www.notion.so/%s", strings.ReplaceAll(pageID, "-", ""))

	return &Result{
		Success: true,
		Output: fmt.Sprintf("Blog draft created successfully!\n\nTitle: %s\nNotion Page: %s\nStatus: Not Started\n\nSuggested Titles: %v\nTags: %v\nRead Time: %s\n\nThe post has been added to your blog drafts with AI suggestions.",
			title, notionURL, suggestedTitles, substackTags, readTime),
		Data: map[string]interface{}{
			"page_id":    pageID,
			"notion_url": notionURL,
		},
	}, nil
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func (t *BlogNotionTool) createNotionPage(ctx context.Context, title, expandedDraft, originalDictation string,
	suggestedTitles, substackTags []string, twitterPost, linkedinPost, blueskyPost string,
	keyTakeaways []string, targetAudience, readTime string) (string, error) {

	// Build properties - using first suggested title if provided
	displayTitle := title
	if len(suggestedTitles) > 0 && suggestedTitles[0] != "" {
		displayTitle = suggestedTitles[0]
	}

	properties := map[string]interface{}{
		"Task name": map[string]interface{}{
			"title": []map[string]interface{}{
				{
					"text": map[string]interface{}{
						"content": displayTitle,
					},
				},
			},
		},
		"Status": map[string]interface{}{
			"status": map[string]interface{}{
				"name": "Not Started",
			},
		},
		"Priority": map[string]interface{}{
			"select": map[string]interface{}{
				"name": "Medium",
			},
		},
	}

	// Add project relation if configured
	if t.projectID != "" {
		properties["Project"] = map[string]interface{}{
			"relation": []map[string]interface{}{
				{
					"id": t.projectID,
				},
			},
		}
	}

	// Build the content blocks
	blocks := t.buildContentBlocks(expandedDraft, originalDictation, suggestedTitles,
		substackTags, twitterPost, linkedinPost, blueskyPost, keyTakeaways, targetAudience, readTime)

	// Build the request payload
	payload := map[string]interface{}{
		"parent": map[string]interface{}{
			"type":        "database_id",
			"database_id": t.databaseID,
		},
		"properties": properties,
		"children":   blocks,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.notion.com/v1/pages", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("notion API error (%d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	pageID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("no page ID in response")
	}

	return pageID, nil
}

// buildContentBlocks creates Notion blocks for all content sections
func (t *BlogNotionTool) buildContentBlocks(expandedDraft, originalDictation string,
	suggestedTitles, substackTags []string, twitterPost, linkedinPost, blueskyPost string,
	keyTakeaways []string, targetAudience, readTime string) []map[string]interface{} {

	var blocks []map[string]interface{}

	// Read time callout
	if readTime != "" {
		blocks = append(blocks, t.createCallout(fmt.Sprintf("📖 %s", readTime), "gray_background"))
	}

	// Key Takeaways section
	if len(keyTakeaways) > 0 {
		blocks = append(blocks, t.createHeading2("🎯 Key Takeaways"))
		for _, takeaway := range keyTakeaways {
			blocks = append(blocks, t.createBulletedListItem(takeaway))
		}
		blocks = append(blocks, t.createDivider())
	}

	// Main Draft Content
	blocks = append(blocks, t.createHeading2("📝 Draft"))
	blocks = append(blocks, t.markdownToBlocks(expandedDraft)...)
	blocks = append(blocks, t.createDivider())

	// Title Suggestions
	if len(suggestedTitles) > 0 {
		blocks = append(blocks, t.createHeading2("💡 Title Suggestions"))
		for i, title := range suggestedTitles {
			blocks = append(blocks, t.createNumberedListItem(title, i+1))
		}
		blocks = append(blocks, t.createDivider())
	}

	// Substack Tags
	if len(substackTags) > 0 {
		blocks = append(blocks, t.createHeading2("🏷️ Substack Tags"))
		blocks = append(blocks, t.createParagraph(strings.Join(substackTags, ", ")))
		blocks = append(blocks, t.createDivider())
	}

	// Social Media Posts
	hasSocial := twitterPost != "" || linkedinPost != "" || blueskyPost != ""
	if hasSocial {
		blocks = append(blocks, t.createHeading2("📱 Social Media Posts"))

		if twitterPost != "" {
			blocks = append(blocks, t.createHeading3("Twitter/X"))
			blocks = append(blocks, t.createQuote(twitterPost))
		}

		if blueskyPost != "" {
			blocks = append(blocks, t.createHeading3("Bluesky"))
			blocks = append(blocks, t.createQuote(blueskyPost))
		}

		if linkedinPost != "" {
			blocks = append(blocks, t.createHeading3("LinkedIn"))
			blocks = append(blocks, t.createQuote(linkedinPost))
		}

		blocks = append(blocks, t.createDivider())
	}

	// Target Audience
	if targetAudience != "" {
		blocks = append(blocks, t.createHeading2("👥 Target Audience"))
		blocks = append(blocks, t.createParagraph(targetAudience))
		blocks = append(blocks, t.createDivider())
	}

	// Original Dictation (collapsed toggle for reference)
	if originalDictation != "" {
		blocks = append(blocks, t.createToggle("🎤 Original Dictation", originalDictation))
	}

	return blocks
}

// Helper functions to create Notion blocks

func (t *BlogNotionTool) createHeading2(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "heading_2",
		"heading_2": map[string]interface{}{
			"rich_text": t.createRichText(text),
		},
	}
}

func (t *BlogNotionTool) createHeading3(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "heading_3",
		"heading_3": map[string]interface{}{
			"rich_text": t.createRichText(text),
		},
	}
}

func (t *BlogNotionTool) createParagraph(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]interface{}{
			"rich_text": t.createRichText(text),
		},
	}
}

func (t *BlogNotionTool) createBulletedListItem(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "bulleted_list_item",
		"bulleted_list_item": map[string]interface{}{
			"rich_text": t.createRichText(text),
		},
	}
}

func (t *BlogNotionTool) createNumberedListItem(text string, num int) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "numbered_list_item",
		"numbered_list_item": map[string]interface{}{
			"rich_text": t.createRichText(text),
		},
	}
}

func (t *BlogNotionTool) createQuote(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "quote",
		"quote": map[string]interface{}{
			"rich_text": t.createRichText(text),
		},
	}
}

func (t *BlogNotionTool) createCallout(text, color string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "callout",
		"callout": map[string]interface{}{
			"rich_text": t.createRichText(text),
			"color":     color,
		},
	}
}

func (t *BlogNotionTool) createDivider() map[string]interface{} {
	return map[string]interface{}{
		"object":  "block",
		"type":    "divider",
		"divider": map[string]interface{}{},
	}
}

func (t *BlogNotionTool) createToggle(title, content string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "toggle",
		"toggle": map[string]interface{}{
			"rich_text": t.createRichText(title),
			"children": []map[string]interface{}{
				t.createParagraph(content),
			},
		},
	}
}

func (t *BlogNotionTool) createRichText(text string) []map[string]interface{} {
	// Notion has a 2000 character limit per rich text segment
	const maxLen = 2000
	var segments []map[string]interface{}

	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			chunk = text[:maxLen]
		}
		segments = append(segments, map[string]interface{}{
			"type": "text",
			"text": map[string]interface{}{
				"content": chunk,
			},
		})
		text = text[len(chunk):]
	}

	if len(segments) == 0 {
		return []map[string]interface{}{
			{
				"type": "text",
				"text": map[string]interface{}{
					"content": "",
				},
			},
		}
	}

	return segments
}

// markdownToBlocks converts markdown to Notion blocks
// This is a simple implementation - could be expanded for more markdown features
func (t *BlogNotionTool) markdownToBlocks(markdown string) []map[string]interface{} {
	var blocks []map[string]interface{}
	lines := strings.Split(markdown, "\n")

	var currentParagraph strings.Builder

	flushParagraph := func() {
		text := strings.TrimSpace(currentParagraph.String())
		if text != "" {
			blocks = append(blocks, t.createParagraph(text))
		}
		currentParagraph.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Empty line - flush current paragraph
		if trimmed == "" {
			flushParagraph()
			continue
		}

		// Heading 2
		if strings.HasPrefix(trimmed, "## ") {
			flushParagraph()
			blocks = append(blocks, t.createHeading2(strings.TrimPrefix(trimmed, "## ")))
			continue
		}

		// Heading 3
		if strings.HasPrefix(trimmed, "### ") {
			flushParagraph()
			blocks = append(blocks, t.createHeading3(strings.TrimPrefix(trimmed, "### ")))
			continue
		}

		// Bulleted list
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushParagraph()
			blocks = append(blocks, t.createBulletedListItem(trimmed[2:]))
			continue
		}

		// Numbered list (simple pattern: "1. ", "2. ", etc.)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' && trimmed[2] == ' ' {
			flushParagraph()
			blocks = append(blocks, t.createNumberedListItem(trimmed[3:], int(trimmed[0]-'0')))
			continue
		}

		// Code block (simplified - just treat as paragraph for now)
		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			// TODO: Handle code blocks properly
			continue
		}

		// Regular text - accumulate into paragraph
		if currentParagraph.Len() > 0 {
			currentParagraph.WriteString(" ")
		}
		currentParagraph.WriteString(trimmed)
	}

	// Flush any remaining content
	flushParagraph()

	return blocks
}
