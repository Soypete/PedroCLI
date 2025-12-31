package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
)

const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Blog</title>
    <link>https://example.com</link>
    <description>A test blog feed</description>
    <item>
      <title>First Post</title>
      <link>https://example.com/first-post</link>
      <description>This is the first post description.</description>
      <pubDate>Mon, 02 Jan 2024 15:04:05 +0000</pubDate>
    </item>
    <item>
      <title>Second Post</title>
      <link>https://example.com/second-post</link>
      <description>This is the second post description.</description>
      <pubDate>Tue, 03 Jan 2024 10:00:00 +0000</pubDate>
    </item>
    <item>
      <title>Third Post</title>
      <link>https://example.com/third-post</link>
      <description>This is the third post description.</description>
      <pubDate>Wed, 04 Jan 2024 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

const testAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Atom Blog</title>
  <link href="https://example.com" rel="alternate"/>
  <entry>
    <title>Atom Post One</title>
    <link href="https://example.com/atom-post-one" rel="alternate"/>
    <summary>First atom post summary.</summary>
    <updated>2024-01-05T10:00:00Z</updated>
    <id>https://example.com/atom-post-one</id>
  </entry>
  <entry>
    <title>Atom Post Two</title>
    <link href="https://example.com/atom-post-two" rel="alternate"/>
    <content>Second atom post content.</content>
    <updated>2024-01-06T11:00:00Z</updated>
    <id>https://example.com/atom-post-two</id>
  </entry>
</feed>`

func TestRSSFeedTool_Fetch(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	cfg := &config.Config{
		Blog: config.BlogConfig{
			Research: config.BlogResearchConfig{
				MaxRSSPosts: 5,
			},
		},
	}

	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
		"url":    server.URL,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	feed, ok := result.Data["feed"].(*RSSFeed)
	if !ok {
		t.Fatal("expected feed in data")
	}

	if feed.Title != "Test Blog" {
		t.Errorf("expected title 'Test Blog', got '%s'", feed.Title)
	}

	if len(feed.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(feed.Items))
	}

	if feed.Items[0].Title != "First Post" {
		t.Errorf("expected first item title 'First Post', got '%s'", feed.Items[0].Title)
	}
}

func TestRSSFeedTool_GetConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(testRSSFeed))
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

	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_configured",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	count, ok := result.Data["count"].(int)
	if !ok {
		t.Fatal("expected count in data")
	}

	if count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}
}

func TestRSSFeedTool_ParseAtom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(testAtomFeed))
	}))
	defer server.Close()

	cfg := &config.Config{
		Blog: config.BlogConfig{
			Research: config.BlogResearchConfig{
				MaxRSSPosts: 5,
			},
		},
	}

	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
		"url":    server.URL,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	feed, ok := result.Data["feed"].(*RSSFeed)
	if !ok {
		t.Fatal("expected feed in data")
	}

	if feed.Title != "Test Atom Blog" {
		t.Errorf("expected title 'Test Atom Blog', got '%s'", feed.Title)
	}

	if len(feed.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(feed.Items))
	}

	if feed.Items[0].Title != "Atom Post One" {
		t.Errorf("expected first item title 'Atom Post One', got '%s'", feed.Items[0].Title)
	}
}

func TestRSSFeedTool_Limit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	cfg := &config.Config{
		Blog: config.BlogConfig{
			Research: config.BlogResearchConfig{
				MaxRSSPosts: 5,
			},
		},
	}

	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
		"url":    server.URL,
		"limit":  float64(2), // JSON numbers are float64
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	feed, ok := result.Data["feed"].(*RSSFeed)
	if !ok {
		t.Fatal("expected feed in data")
	}

	if len(feed.Items) != 2 {
		t.Errorf("expected 2 items (limited), got %d", len(feed.Items))
	}
}

func TestRSSFeedTool_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			Research: config.BlogResearchConfig{
				MaxRSSPosts: 5,
			},
		},
	}

	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
		"url":    "http://invalid.invalid.invalid/feed.xml",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Fatal("expected failure for invalid URL")
	}

	if result.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestRSSFeedTool_MissingURL(t *testing.T) {
	cfg := &config.Config{}
	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Fatal("expected failure for missing URL")
	}

	if result.Error != "url is required for fetch action" {
		t.Errorf("unexpected error message: %s", result.Error)
	}
}

func TestRSSFeedTool_NoConfiguredFeed(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			Research: config.BlogResearchConfig{
				RSSEnabled: true,
			},
		},
	}
	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_configured",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Fatal("expected failure for missing configured feed")
	}

	if result.Error != "no RSS feed URL configured (set blog.rss_feed_url in config)" {
		t.Errorf("unexpected error message: %s", result.Error)
	}
}

func TestRSSFeedTool_RSSDisabled(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			RSSFeedURL: "https://example.com/feed",
			Research: config.BlogResearchConfig{
				RSSEnabled: false,
			},
		},
	}
	tool := NewRSSFeedTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_configured",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Fatal("expected failure when RSS is disabled")
	}

	if result.Error != "RSS research is disabled (set blog.research.rss_enabled=true in config)" {
		t.Errorf("unexpected error message: %s", result.Error)
	}
}

func TestParseRSSDate(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
		hasError bool
	}{
		{
			input:    "Mon, 02 Jan 2006 15:04:05 -0700",
			expected: time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*3600)),
			hasError: false,
		},
		{
			input:    "2024-01-15T10:30:00Z",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			hasError: false,
		},
		{
			input:    "invalid date",
			hasError: true,
		},
	}

	for _, tt := range tests {
		result, err := parseRSSDate(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for input '%s'", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for input '%s': %v", tt.input, err)
			}
			if !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v for input '%s'", tt.expected, result, tt.input)
			}
		}
	}
}

func TestTruncateDescription(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a longer description", 15, "this is a lo..."},
		{"<p>HTML content</p>", 20, "HTML content"},
		{"<strong>Bold</strong> and <em>italic</em>", 30, "Bold and italic"},
	}

	for _, tt := range tests {
		result := truncateDescription(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateDescription(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello</p>", "Hello"},
		{"<strong>Bold</strong> text", "Bold text"},
		{"No tags", "No tags"},
		{"<a href=\"url\">Link</a>", "Link"},
		{"<div><p>Nested</p></div>", "Nested"},
	}

	for _, tt := range tests {
		result := stripHTMLTags(tt.input)
		if result != tt.expected {
			t.Errorf("stripHTMLTags(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
