package agents

import (
	"context"
	"fmt"
	"strings"
)

// News Review Phases
// These implement the 5-phase workflow for reviewing and summarizing news items

// phaseFetchSources fetches news from RSS feeds and web search
func (a *UnifiedPodcastAgent) phaseFetchSources(ctx context.Context) error {
	fmt.Println("   📰 Fetching news sources...")

	// TODO: Use web_search and rss_feed tools to fetch news
	// For now, create placeholder news items
	a.newsItems = []NewsItem{
		{
			Title:       "Example AI News Article 1",
			URL:         "https://example.com/ai-news-1",
			Description: "Recent developments in AI technology...",
			Relevance:   0.8,
		},
		{
			Title:       "Example AI News Article 2",
			URL:         "https://example.com/ai-news-2",
			Description: "New model releases and updates...",
			Relevance:   0.7,
		},
	}

	a.currentContent.Data["news_sources_fetched"] = len(a.newsItems)

	fmt.Printf("   📰 Fetched %d news sources\n", len(a.newsItems))
	return nil
}

// phaseFilterByTopic filters news items by focus topic
func (a *UnifiedPodcastAgent) phaseFilterByTopic(ctx context.Context) error {
	fmt.Println("   🔍 Filtering by topic...")

	if a.focus == "" {
		a.focus = "AI" // Default focus
	}

	// TODO: Use keyword matching or LLM to filter news by topic
	// For now, keep all news items
	filteredCount := len(a.newsItems)

	a.currentContent.Data["news_filtered_count"] = filteredCount
	a.currentContent.Data["focus_topic"] = a.focus

	fmt.Printf("   ✅ Filtered to %d relevant items (focus: %s)\n", filteredCount, a.focus)
	return nil
}

// phaseSummarizeItems extracts key points from each news item
func (a *UnifiedPodcastAgent) phaseSummarizeItems(ctx context.Context) error {
	fmt.Println("   📝 Summarizing news items...")

	// TODO: Use LLM to summarize each news item
	// For now, use the existing descriptions
	summaries := make([]string, len(a.newsItems))
	for i, item := range a.newsItems {
		summaries[i] = fmt.Sprintf("**%s**: %s", item.Title, item.Description)
	}

	a.currentContent.Data["news_summaries"] = summaries

	fmt.Printf("   ✅ Summarized %d news items\n", len(summaries))
	return nil
}

// phaseRankByRelevance ranks news items by relevance score
func (a *UnifiedPodcastAgent) phaseRankByRelevance(ctx context.Context) error {
	fmt.Println("   📊 Ranking by relevance...")

	// TODO: Use LLM to score relevance to episode topic
	// For now, use placeholder relevance scores already set

	// Sort by relevance (descending)
	// TODO: Implement proper sorting

	// Limit to maxNews items
	if a.maxNews > 0 && len(a.newsItems) > a.maxNews {
		a.newsItems = a.newsItems[:a.maxNews]
	}

	a.currentContent.Data["news_ranked_count"] = len(a.newsItems)

	fmt.Printf("   ✅ Ranked and selected top %d items\n", len(a.newsItems))
	return nil
}

// phaseGenerateNewsSummary creates formatted news summary
func (a *UnifiedPodcastAgent) phaseGenerateNewsSummary(ctx context.Context) error {
	fmt.Println("   📄 Generating news summary...")

	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("# News Summary - Episode %s\n\n", a.episode))
	summary.WriteString(fmt.Sprintf("**Focus**: %s\n", a.focus))
	summary.WriteString(fmt.Sprintf("**Items**: %d\n\n", len(a.newsItems)))
	summary.WriteString("---\n\n")

	for i, item := range a.newsItems {
		summary.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, item.Title))
		summary.WriteString(fmt.Sprintf("**URL**: %s\n\n", item.URL))
		summary.WriteString(fmt.Sprintf("%s\n\n", item.Description))
		summary.WriteString(fmt.Sprintf("**Relevance**: %.1f%%\n\n", item.Relevance*100))
		summary.WriteString("---\n\n")
	}

	newsSummary := summary.String()
	a.currentContent.Data["news_summary"] = newsSummary

	// TODO: Save to Notion NewsReview database

	fmt.Printf("   ✅ Generated news summary (%d characters)\n", len(newsSummary))
	return nil
}
