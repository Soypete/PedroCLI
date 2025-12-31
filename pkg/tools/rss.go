package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
)

// RSSItem represents an item from an RSS feed
type RSSItem struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description,omitempty"`
	PubDate     time.Time `json:"pub_date,omitempty"`
	PubDateStr  string    `json:"pub_date_str,omitempty"`
}

// RSSFeed represents a parsed RSS feed
type RSSFeed struct {
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description,omitempty"`
	Items       []RSSItem `json:"items"`
}

// rssChannel represents the RSS 2.0 channel element
type rssChannel struct {
	XMLName     xml.Name  `xml:"rss"`
	Title       string    `xml:"channel>title"`
	Link        string    `xml:"channel>link"`
	Description string    `xml:"channel>description"`
	Items       []rssItem `xml:"channel>item"`
}

// rssItem represents an RSS 2.0 item element
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// atomFeed represents an Atom feed
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Title   string      `xml:"title"`
	Link    []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

// atomLink represents an Atom link element
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// atomEntry represents an Atom entry element
type atomEntry struct {
	Title   string     `xml:"title"`
	Link    []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
	ID      string     `xml:"id"`
}

// RSSFeedTool fetches and parses RSS/Atom feeds
type RSSFeedTool struct {
	config     *config.Config
	httpClient *http.Client
}

// NewRSSFeedTool creates a new RSS feed tool
func NewRSSFeedTool(cfg *config.Config) *RSSFeedTool {
	return &RSSFeedTool{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the tool name
func (t *RSSFeedTool) Name() string {
	return "rss_feed"
}

// Description returns the tool description
func (t *RSSFeedTool) Description() string {
	return `Fetch and parse RSS/Atom feeds to get previous blog posts.

Actions:
- fetch: Fetch items from a specific RSS feed URL
  Args: url (string, required), limit (optional int, default 5)

- get_configured: Fetch items from the configured blog RSS feed
  Args: limit (optional int, default uses config.blog.research.max_rss_posts)

Returns a list of posts with title, link, description, and publication date.

Example:
{"tool": "rss_feed", "args": {"action": "get_configured", "limit": 5}}
{"tool": "rss_feed", "args": {"action": "fetch", "url": "https://example.com/feed.xml"}}`
}

// Execute executes the RSS feed tool
func (t *RSSFeedTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		action = "get_configured"
	}

	switch action {
	case "fetch":
		return t.fetchFeed(ctx, args)
	case "get_configured":
		return t.getConfiguredFeed(ctx, args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// fetchFeed fetches an RSS feed from a specific URL
func (t *RSSFeedTool) fetchFeed(ctx context.Context, args map[string]interface{}) (*Result, error) {
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return &Result{
			Success: false,
			Error:   "url is required for fetch action",
		}, nil
	}

	limit := t.config.Blog.Research.MaxRSSPosts
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	feed, err := t.parseFeed(ctx, url, limit)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to fetch feed: %v", err),
		}, nil
	}

	return t.formatResult(feed)
}

// getConfiguredFeed fetches the RSS feed from config
func (t *RSSFeedTool) getConfiguredFeed(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.config.Blog.RSSFeedURL == "" {
		return &Result{
			Success: false,
			Error:   "no RSS feed URL configured (set blog.rss_feed_url in config)",
		}, nil
	}

	if !t.config.Blog.Research.RSSEnabled {
		return &Result{
			Success: false,
			Error:   "RSS research is disabled (set blog.research.rss_enabled=true in config)",
		}, nil
	}

	limit := t.config.Blog.Research.MaxRSSPosts
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	feed, err := t.parseFeed(ctx, t.config.Blog.RSSFeedURL, limit)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to fetch configured feed: %v", err),
		}, nil
	}

	return t.formatResult(feed)
}

// parseFeed fetches and parses an RSS or Atom feed
func (t *RSSFeedTool) parseFeed(ctx context.Context, url string, limit int) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "PedroCLI/1.0 RSS Reader")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Try RSS 2.0 first
	feed, err := t.parseRSS(body, limit)
	if err == nil && len(feed.Items) > 0 {
		return feed, nil
	}

	// Try Atom
	feed, err = t.parseAtom(body, limit)
	if err == nil && len(feed.Items) > 0 {
		return feed, nil
	}

	return nil, fmt.Errorf("failed to parse feed as RSS or Atom")
}

// parseRSS parses an RSS 2.0 feed
func (t *RSSFeedTool) parseRSS(data []byte, limit int) (*RSSFeed, error) {
	var rss rssChannel
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}

	feed := &RSSFeed{
		Title:       rss.Title,
		Link:        rss.Link,
		Description: rss.Description,
		Items:       make([]RSSItem, 0, min(len(rss.Items), limit)),
	}

	for i, item := range rss.Items {
		if i >= limit {
			break
		}

		rssItem := RSSItem{
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			Description: truncateDescription(item.Description, 200),
			PubDateStr:  item.PubDate,
		}

		// Try to parse the date
		if pubDate, err := parseRSSDate(item.PubDate); err == nil {
			rssItem.PubDate = pubDate
		}

		feed.Items = append(feed.Items, rssItem)
	}

	return feed, nil
}

// parseAtom parses an Atom feed
func (t *RSSFeedTool) parseAtom(data []byte, limit int) (*RSSFeed, error) {
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err != nil {
		return nil, err
	}

	// Find the main link
	var mainLink string
	for _, link := range atom.Link {
		if link.Rel == "" || link.Rel == "alternate" {
			mainLink = link.Href
			break
		}
	}

	feed := &RSSFeed{
		Title: atom.Title,
		Link:  mainLink,
		Items: make([]RSSItem, 0, min(len(atom.Entries), limit)),
	}

	for i, entry := range atom.Entries {
		if i >= limit {
			break
		}

		// Find the entry link
		var entryLink string
		for _, link := range entry.Link {
			if link.Rel == "" || link.Rel == "alternate" {
				entryLink = link.Href
				break
			}
		}

		// Use summary or content for description
		description := entry.Summary
		if description == "" {
			description = entry.Content
		}

		atomItem := RSSItem{
			Title:       strings.TrimSpace(entry.Title),
			Link:        entryLink,
			Description: truncateDescription(description, 200),
			PubDateStr:  entry.Updated,
		}

		// Try to parse the date
		if pubDate, err := time.Parse(time.RFC3339, entry.Updated); err == nil {
			atomItem.PubDate = pubDate
		}

		feed.Items = append(feed.Items, atomItem)
	}

	return feed, nil
}

// formatResult formats the feed as a Result
func (t *RSSFeedTool) formatResult(feed *RSSFeed) (*Result, error) {
	data, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  string(data),
		Data: map[string]interface{}{
			"feed":  feed,
			"count": len(feed.Items),
		},
	}, nil
}

// parseRSSDate attempts to parse various RSS date formats
func parseRSSDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}

	dateStr = strings.TrimSpace(dateStr)
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// truncateDescription truncates a description to maxLen characters
func truncateDescription(desc string, maxLen int) string {
	// Strip HTML tags (simple approach)
	desc = stripHTMLTags(desc)
	desc = strings.TrimSpace(desc)

	if len(desc) <= maxLen {
		return desc
	}

	return desc[:maxLen-3] + "..."
}

// stripHTMLTags removes HTML tags from a string (simple approach)
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false

	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	return result.String()
}
