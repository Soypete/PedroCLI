package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/prompts"
)

// Outline Generation Phases
// These implement the 5-phase workflow for generating podcast episode outlines

// phaseGatherOutlineContext collects episode metadata and context
func (a *UnifiedPodcastAgent) phaseGatherOutlineContext(ctx context.Context) error {
	fmt.Println("   📋 Gathering episode context...")

	// Store input parameters in content data
	a.currentContent.Data["episode"] = a.episode
	a.currentContent.Data["title"] = a.title
	a.currentContent.Data["guests"] = a.guests
	a.currentContent.Data["duration"] = a.duration

	// If topic summary provided in outline field, use it
	if a.outline != "" {
		a.currentContent.Data["topic_summary"] = a.outline
		fmt.Printf("   📝 Topic summary: %s\n", truncate(a.outline, 100))
	}

	// Check for pre-provided news items
	if len(a.newsItems) > 0 {
		a.currentContent.Data["news_provided"] = true
		fmt.Printf("   📰 %d news items provided\n", len(a.newsItems))
	}

	fmt.Println("   ✅ Context gathered")
	return nil
}

// phaseResearchNews searches for recent news relevant to the episode topic
func (a *UnifiedPodcastAgent) phaseResearchNews(ctx context.Context) error {
	fmt.Println("   🔍 Researching relevant news...")

	// Skip if news items already provided
	if len(a.newsItems) > 0 {
		fmt.Println("   ⏭️  News items already provided, skipping research")
		return nil
	}

	// Use web search and RSS tools to find relevant news
	searchTool, hasSearch := a.tools["web_search"]
	rssTool, hasRSS := a.tools["rss_feed"]

	var foundNews []NewsItem

	// Search for news related to episode topic
	if hasSearch && a.title != "" {
		searchQuery := fmt.Sprintf("AI %s news recent", a.title)
		result, err := searchTool.Execute(ctx, map[string]interface{}{
			"query":       searchQuery,
			"max_results": 5,
		})
		if err != nil {
			fmt.Printf("   ⚠️  Web search failed: %v\n", err)
		} else if result.Success {
			// Parse search results into news items
			// The search tool returns formatted results in Output
			fmt.Printf("   🔎 Found web search results\n")
		}
	}

	// Fetch from RSS feeds
	if hasRSS {
		result, err := rssTool.Execute(ctx, map[string]interface{}{
			"action":    "fetch",
			"max_items": 5,
		})
		if err != nil {
			fmt.Printf("   ⚠️  RSS fetch failed: %v\n", err)
		} else if result.Success {
			fmt.Printf("   📡 Fetched RSS feed items\n")
		}
	}

	// Limit to maxNews items
	maxNews := a.maxNews
	if maxNews == 0 {
		maxNews = 3 // Default to 3 news items
	}
	if len(foundNews) > maxNews {
		foundNews = foundNews[:maxNews]
	}

	a.newsItems = foundNews
	a.currentContent.Data["news_items"] = a.newsItems

	fmt.Printf("   📰 Collected %d news items\n", len(a.newsItems))
	return nil
}

// phaseGenerateOutline generates the structured episode outline using LLM
func (a *UnifiedPodcastAgent) phaseGenerateOutline(ctx context.Context) error {
	fmt.Println("   ✍️  Generating episode outline...")

	// Get the outline prompt template
	promptManager := prompts.NewManager(a.config)
	outlinePrompt := promptManager.GetPrompt("podcast", "generate_episode_outline")

	if outlinePrompt == "" {
		return fmt.Errorf("failed to load outline prompt template")
	}

	// Build the user prompt with episode details
	var userPrompt strings.Builder
	userPrompt.WriteString("Generate an episode outline with these details:\n\n")
	userPrompt.WriteString(fmt.Sprintf("**Episode Number**: %s\n", a.episode))
	userPrompt.WriteString(fmt.Sprintf("**Episode Title**: %s\n", a.title))
	userPrompt.WriteString(fmt.Sprintf("**Recording Date**: %s\n", time.Now().AddDate(0, 0, 7).Format("January 2, 2006")))
	userPrompt.WriteString(fmt.Sprintf("**Publish Date**: %s\n", time.Now().AddDate(0, 0, 14).Format("January 2, 2006")))
	userPrompt.WriteString("**Status**: Idea\n")

	if a.guests != "" {
		userPrompt.WriteString(fmt.Sprintf("**Guests**: %s\n", a.guests))
	} else {
		userPrompt.WriteString("**Guests**: None\n")
	}

	// Topic summary
	topicSummary, _ := a.currentContent.Data["topic_summary"].(string)
	if topicSummary != "" {
		userPrompt.WriteString(fmt.Sprintf("\n**Topic Summary**:\n%s\n", topicSummary))
	}

	// News items
	if len(a.newsItems) > 0 {
		userPrompt.WriteString("\n**News Items**:\n")
		for i, item := range a.newsItems {
			userPrompt.WriteString(fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n",
				i+1, item.Title, item.URL, item.Description))
		}
	} else {
		userPrompt.WriteString("\n**News Items**: [Research and add 3 relevant AI/tech news items]\n")
	}

	// Call LLM to generate outline
	request := &llm.InferenceRequest{
		SystemPrompt: outlinePrompt,
		UserPrompt:   userPrompt.String(),
		Temperature:  0.7, // Slightly higher for creative content
		MaxTokens:    4000,
	}

	response, err := a.backend.Infer(ctx, request)
	if err != nil {
		return fmt.Errorf("LLM inference failed: %w", err)
	}

	// Store the generated outline
	a.outline = response.Text
	a.currentContent.Data["generated_outline"] = a.outline

	fmt.Printf("   📝 Generated outline (%d characters)\n", len(a.outline))
	return nil
}

// phaseReviewOutline reviews the outline for completeness
func (a *UnifiedPodcastAgent) phaseReviewOutline(ctx context.Context) error {
	fmt.Println("   🔍 Reviewing outline...")

	if a.outline == "" {
		return fmt.Errorf("no outline to review")
	}

	// Check for required sections
	requiredSections := []string{
		"Episode Details",
		"Host Bios",
		"Segment Outline",
		"Intro",
		"News Segment",
		"Main Conversation",
		"Outro",
		"Show Notes",
	}

	var missingSections []string
	outlineLower := strings.ToLower(a.outline)
	for _, section := range requiredSections {
		if !strings.Contains(outlineLower, strings.ToLower(section)) {
			missingSections = append(missingSections, section)
		}
	}

	if len(missingSections) > 0 {
		fmt.Printf("   ⚠️  Missing sections: %s\n", strings.Join(missingSections, ", "))
		a.currentContent.Data["review_warnings"] = missingSections
	} else {
		fmt.Println("   ✅ All required sections present")
	}

	// Check approximate timing (for 25-minute episode)
	// Main conversation should be ~12 minutes
	if strings.Contains(a.outline, "10:00 – 22:00") || strings.Contains(a.outline, "10:00 - 22:00") {
		fmt.Println("   ⏱️  Timing looks correct for 25-minute episode")
	}

	a.currentContent.Data["reviewed"] = true
	a.currentContent.Data["review_timestamp"] = time.Now().UTC().Format(time.RFC3339)

	return nil
}

// phaseSaveOutline saves the outline to Notion
func (a *UnifiedPodcastAgent) phaseSaveOutline(ctx context.Context) error {
	fmt.Println("   📤 Saving outline...")

	if a.outline == "" {
		return fmt.Errorf("no outline to save")
	}

	// Save to Notion Episode Planner database if enabled
	if a.config.Podcast.Notion.Enabled {
		notionTool, ok := a.tools["notion"]
		if !ok {
			fmt.Println("   ⚠️  Notion tool not available, saving to storage only")
		} else {
			fmt.Println("   📝 Creating Notion page in Episode Planner...")

			// Prepare properties for Notion page
			properties := map[string]interface{}{
				"Episode #":             fmt.Sprintf("%s - %s", a.episode, a.title),
				"Title / Working Topic": a.title,
				"Status 🎛":              "Outline",
				"Notes":                 fmt.Sprintf("Duration: %d minutes\nGuests: %s\n\nAuto-generated outline", a.duration, a.guests),
			}

			// Create page in Episode Planner database
			result, err := notionTool.Execute(ctx, map[string]interface{}{
				"action":        "create_page",
				"database_name": "scripts", // Uses scripts database for outlines too
				"properties":    properties,
				"content":       a.outline,
			})

			if err != nil {
				fmt.Printf("   ⚠️  Failed to save to Notion: %v\n", err)
			} else if !result.Success {
				fmt.Printf("   ⚠️  Notion save error: %s\n", result.Error)
			} else {
				fmt.Println("   ✅ Outline saved to Notion")
				a.currentContent.Data["notion_page_created"] = true
				a.currentContent.Data["notion_output"] = result.Output
			}
		}
	} else {
		fmt.Println("   ℹ️  Notion integration disabled")
	}

	// Mark as saved in content store
	a.currentContent.Data["saved"] = true
	a.currentContent.Data["save_timestamp"] = time.Now().UTC().Format(time.RFC3339)

	fmt.Println("   ✅ Outline saved to storage")
	return nil
}

// Helper function to truncate strings for display
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
