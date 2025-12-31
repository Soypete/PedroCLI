package agents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/tools"
)

// TestRSSFeedTool_RealFeed tests fetching from actual Substack RSS feed
// This test requires network access and is skipped in short mode or CI
func TestRSSFeedTool_RealFeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("skipping integration test in CI environment")
	}

	cfg := &config.Config{
		Blog: config.BlogConfig{
			RSSFeedURL: "https://soypetetech.substack.com/feed",
			Research: config.BlogResearchConfig{
				Enabled:     true,
				RSSEnabled:  true,
				MaxRSSPosts: 3,
			},
		},
	}

	tool := tools.NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_configured",
		"limit":  3,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Should have feed in data
	feed, ok := result.Data["feed"].(*tools.RSSFeed)
	if !ok {
		t.Fatal("expected feed in data")
	}

	if len(feed.Items) == 0 {
		t.Fatal("expected at least one RSS item")
	}

	// Verify first item has required fields
	firstItem := feed.Items[0]
	if firstItem.Title == "" {
		t.Error("expected title on RSS item")
	}
	if firstItem.Link == "" {
		t.Error("expected link on RSS item")
	}

	t.Logf("Successfully fetched %d RSS items from Soypete Tech", len(feed.Items))
	for i, item := range feed.Items {
		t.Logf("  %d. %s (%s)", i+1, item.Title, item.PubDateStr)
	}
}

// TestStaticLinksTool_WithConfig tests static links tool with full config
func TestStaticLinksTool_WithConfig(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				Discord:            "https://discord.gg/soypete",
				LinkTree:           "https://linktr.ee/soypete_tech",
				YouTube:            "https://youtube.com/@soypete",
				Twitter:            "https://twitter.com/soypete",
				Bluesky:            "https://bsky.app/soypete",
				LinkedIn:           "https://linkedin.com/in/soypete",
				Newsletter:         "https://soypetetech.substack.com",
				YouTubePlaceholder: "Latest Video: [ADD LINK BEFORE SUBSTACK PUBLISH]",
				CustomLinks: []config.CustomLink{
					{Name: "GitHub", URL: "https://github.com/soypete", Icon: ""},
				},
			},
		},
	}

	tool := tools.NewStaticLinksTool(cfg)

	// Test get_all
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_all",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	totalCount, ok := result.Data["total_count"].(int)
	if !ok {
		t.Fatal("expected total_count in data")
	}

	// 7 social links + 1 custom = 8
	if totalCount != 8 {
		t.Errorf("expected 8 total links, got %d", totalCount)
	}

	// Test markdown formatting
	md := tool.FormatAsMarkdown()

	if !strings.Contains(md, "[Join our Discord]") {
		t.Error("markdown should contain Discord link")
	}

	if !strings.Contains(md, "linktr.ee/soypete_tech") {
		t.Error("markdown should contain LinkTree link")
	}

	t.Logf("Static links markdown:\n%s", md)
}

// TestRSSFeedTool_MockServer tests RSS tool with a mock HTTP server
func TestRSSFeedTool_MockServer(t *testing.T) {
	// Create mock RSS feed
	mockFeed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
	<channel>
		<title>Test Blog</title>
		<link>https://test.example.com</link>
		<description>Test RSS Feed</description>
		<item>
			<title>Test Post 1</title>
			<link>https://test.example.com/post-1</link>
			<description>First test post description</description>
			<pubDate>Mon, 30 Dec 2024 12:00:00 GMT</pubDate>
		</item>
		<item>
			<title>Test Post 2</title>
			<link>https://test.example.com/post-2</link>
			<description>Second test post description</description>
			<pubDate>Sun, 29 Dec 2024 12:00:00 GMT</pubDate>
		</item>
		<item>
			<title>Test Post 3</title>
			<link>https://test.example.com/post-3</link>
			<description>Third test post description</description>
			<pubDate>Sat, 28 Dec 2024 12:00:00 GMT</pubDate>
		</item>
	</channel>
</rss>`

	// Start mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(mockFeed))
	}))
	defer server.Close()

	cfg := &config.Config{
		Blog: config.BlogConfig{
			RSSFeedURL: server.URL,
			Research: config.BlogResearchConfig{
				Enabled:     true,
				RSSEnabled:  true,
				MaxRSSPosts: 5,
			},
		},
	}

	tool := tools.NewRSSFeedTool(cfg)

	// Test fetch action
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
		"url":    server.URL,
		"limit":  2.0, // JSON numbers come as float64
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	feed, ok := result.Data["feed"].(*tools.RSSFeed)
	if !ok {
		t.Fatal("expected feed in data")
	}

	if len(feed.Items) != 2 {
		t.Errorf("expected 2 items (limit=2), got %d", len(feed.Items))
	}

	if feed.Items[0].Title != "Test Post 1" {
		t.Errorf("expected first item title 'Test Post 1', got '%s'", feed.Items[0].Title)
	}

	if feed.Items[0].Link != "https://test.example.com/post-1" {
		t.Errorf("expected correct link, got '%s'", feed.Items[0].Link)
	}
}

// TestBlogOrchestrator_HelperFunctions tests the orchestrator helper functions
func TestBlogOrchestrator_HelperFunctions(t *testing.T) {
	// Test extractJSON with various inputs
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean JSON",
			input:    `{"main_topic": "2025 Review"}`,
			expected: `{"main_topic": "2025 Review"}`,
		},
		{
			name:     "JSON with surrounding text",
			input:    `Here is the analysis: {"main_topic": "Test"} That's the result.`,
			expected: `{"main_topic": "Test"}`,
		},
		{
			name:     "nested JSON",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJSON(tc.input)
			if result != tc.expected {
				t.Errorf("extractJSON(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestBlogOrchestrator_ContentSectionFormatting tests section formatting
func TestBlogOrchestrator_ContentSectionFormatting(t *testing.T) {
	sections := []ContentSection{
		{Title: "Introduction", Description: "Opening hook", Priority: 1},
		{Title: "Main Content", Description: "Core ideas", Priority: 1},
		{Title: "Conclusion", Description: "Wrap up", Priority: 2},
	}

	formatted := formatSections(sections)

	if !strings.Contains(formatted, "1. Introduction") {
		t.Error("should contain Introduction section")
	}
	if !strings.Contains(formatted, "2. Main Content") {
		t.Error("should contain Main Content section")
	}
	if !strings.Contains(formatted, "3. Conclusion") {
		t.Error("should contain Conclusion section")
	}
	if !strings.Contains(formatted, "Opening hook") {
		t.Error("should contain section description")
	}
}

// TestBlogOrchestrator_ResearchDataFormatting tests research data formatting
func TestBlogOrchestrator_ResearchDataFormatting(t *testing.T) {
	// Test empty data
	emptyResult := formatResearchData(map[string]interface{}{})
	if !strings.Contains(emptyResult, "No research data available") {
		t.Error("empty data should indicate no data available")
	}

	// Test with calendar data
	calendarData := map[string]interface{}{
		"calendar": "Upcoming event: Go Meetup on Jan 15",
	}
	calResult := formatResearchData(calendarData)
	if !strings.Contains(calResult, "calendar") {
		t.Error("should contain calendar key")
	}
	if !strings.Contains(calResult, "Go Meetup") {
		t.Error("should contain calendar event details")
	}
}

// TestBlogOrchestrator_OutlineSectionExtraction tests extracting sections from outline
func TestBlogOrchestrator_OutlineSectionExtraction(t *testing.T) {
	outline := `# 2025 Year in Review

## Introduction
Opening thoughts about the year

## Major Achievements
What we accomplished

## Content Created
Blog posts, videos, streams

## Lessons Learned
Key takeaways

## What's Next
Plans for 2026

## Conclusion
Final thoughts`

	sections := extractSectionsFromOutline(outline)

	expectedSections := []string{
		"Introduction",
		"Major Achievements",
		"Content Created",
		"Lessons Learned",
		"What's Next",
		"Conclusion",
	}

	if len(sections) != len(expectedSections) {
		t.Errorf("expected %d sections, got %d", len(expectedSections), len(sections))
	}

	for i, expected := range expectedSections {
		if i >= len(sections) {
			t.Errorf("missing section %d: %s", i, expected)
			continue
		}
		if sections[i] != expected {
			t.Errorf("section %d: expected %q, got %q", i, expected, sections[i])
		}
	}
}

// TestBlogOrchestrator_StructMarshaling tests that output structs work correctly
func TestBlogOrchestrator_StructMarshaling(t *testing.T) {
	output := BlogOrchestratorOutput{
		Analysis: &BlogPromptAnalysis{
			MainTopic: "2025 Year Review",
			ContentSections: []ContentSection{
				{Title: "Intro", Description: "Hook", Priority: 1},
				{Title: "Body", Description: "Content", Priority: 1},
			},
			ResearchTasks: []ResearchTask{
				{Type: "calendar", Params: map[string]interface{}{"action": "list_events"}},
				{Type: "rss_feed", Params: map[string]interface{}{"action": "get_configured"}},
			},
			IncludeNewsletter:  true,
			EstimatedWordCount: 1500,
		},
		ResearchData: map[string]interface{}{
			"calendar": "Events data here",
			"rss_feed": "Posts data here",
		},
		Outline:       "## Intro\nHook\n\n## Body\nContent",
		ExpandedDraft: "Full blog post content here...",
		Newsletter:    "Newsletter section here...",
		FullContent:   "Full blog post content here...\n\n---\n\nNewsletter section here...",
		SocialPosts: map[string]string{
			"twitter_post":  "Check out my 2025 review!",
			"linkedin_post": "Reflecting on an amazing year...",
			"bluesky_post":  "2025 was incredible!",
		},
		SuggestedTitle: "2025: A Year of Growth",
	}

	// Verify all fields are set correctly
	if output.Analysis.MainTopic != "2025 Year Review" {
		t.Error("MainTopic not set correctly")
	}

	if len(output.Analysis.ContentSections) != 2 {
		t.Error("ContentSections not set correctly")
	}

	if len(output.Analysis.ResearchTasks) != 2 {
		t.Error("ResearchTasks not set correctly")
	}

	if len(output.SocialPosts) != 3 {
		t.Error("SocialPosts should have 3 entries")
	}

	if output.SuggestedTitle != "2025: A Year of Growth" {
		t.Error("SuggestedTitle not set correctly")
	}
}
