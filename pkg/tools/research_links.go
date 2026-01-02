package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/webscrape"
)

// ResearchLink represents a user-provided research URL with metadata
type ResearchLink struct {
	URL      string   `json:"url"`                // Required: The URL to fetch
	Title    string   `json:"title,omitempty"`    // Optional: Human-readable title
	Notes    string   `json:"notes,omitempty"`    // Optional: Plain text notes about the link
	Category string   `json:"category,omitempty"` // Optional: reference, citation, example, research, inspiration
	Labels   []string `json:"labels,omitempty"`   // Optional: User-defined tags
}

// ResearchLinksContext holds all research links for a job
type ResearchLinksContext struct {
	Links      []ResearchLink `json:"links"`
	PlainNotes string         `json:"plain_notes,omitempty"` // General notes without URLs
}

// FetchedContent represents content fetched from a research link
type FetchedContent struct {
	URL        string   `json:"url"`
	Title      string   `json:"title,omitempty"`
	Content    string   `json:"content,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	CodeBlocks []string `json:"code_blocks,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// ResearchLinksTool provides access to user-provided research links for a job
type ResearchLinksTool struct {
	context *ResearchLinksContext
	fetcher *webscrape.HTTPFetcher
	cache   map[string]*FetchedContent // In-memory cache for this job
	mu      sync.RWMutex
}

// NewResearchLinksTool creates a new research links tool with the given context
func NewResearchLinksTool(ctx *ResearchLinksContext) *ResearchLinksTool {
	// Create HTTP fetcher with default config
	fetcherCfg := webscrape.DefaultFetcherConfig()
	fetcher, _ := webscrape.NewHTTPFetcher(fetcherCfg)

	return &ResearchLinksTool{
		context: ctx,
		fetcher: fetcher,
		cache:   make(map[string]*FetchedContent),
	}
}

// NewResearchLinksToolFromLinks creates a tool from a slice of links
func NewResearchLinksToolFromLinks(links []ResearchLink, plainNotes string) *ResearchLinksTool {
	ctx := &ResearchLinksContext{
		Links:      links,
		PlainNotes: plainNotes,
	}
	return NewResearchLinksTool(ctx)
}

// Name returns the tool name
func (t *ResearchLinksTool) Name() string {
	return "research_links"
}

// Description returns the tool description
func (t *ResearchLinksTool) Description() string {
	return `Access user-provided research URLs for this job.

Actions:
- list: List all available research links with their metadata (title, notes, category, labels)
- fetch: Fetch content from a specific URL and optionally summarize it
- fetch_all: Fetch all research links (uses caching to avoid duplicates)

Use this tool when:
- The user mentions "links I provided", "these resources", or "references"
- You need to cite sources or include references in content
- You want to extract code examples from provided documentation

Categories guide how to use the content:
- citation: Quote or cite with attribution (use [source](url) format)
- reference: Background reading, summarize key concepts
- example: Extract code blocks and implementation patterns
- research: Synthesize into original content
- inspiration: Ideas and creative direction

Example:
{"tool": "research_links", "args": {"action": "list"}}
{"tool": "research_links", "args": {"action": "fetch", "url": "https://example.com/doc", "summarize": true}}`
}

// Execute executes the research links tool
func (t *ResearchLinksTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		action = "list"
	}

	switch action {
	case "list":
		return t.list(args)
	case "fetch":
		return t.fetch(ctx, args)
	case "fetch_all":
		return t.fetchAll(ctx, args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s. Valid actions: list, fetch, fetch_all", action),
		}, nil
	}
}

// list returns all research links with their metadata
func (t *ResearchLinksTool) list(args map[string]interface{}) (*Result, error) {
	if t.context == nil || len(t.context.Links) == 0 {
		return &Result{
			Success: true,
			Output:  "No research links provided for this job.",
			Data: map[string]interface{}{
				"links":       []ResearchLink{},
				"plain_notes": "",
				"count":       0,
			},
		}, nil
	}

	// Filter by category if provided
	filterCategory, _ := args["filter_category"].(string)

	links := t.context.Links
	if filterCategory != "" {
		filtered := make([]ResearchLink, 0)
		for _, link := range links {
			if strings.EqualFold(link.Category, filterCategory) {
				filtered = append(filtered, link)
			}
		}
		links = filtered
	}

	// Build output
	output := struct {
		Links      []ResearchLink `json:"links"`
		PlainNotes string         `json:"plain_notes,omitempty"`
		Count      int            `json:"count"`
	}{
		Links:      links,
		PlainNotes: t.context.PlainNotes,
		Count:      len(links),
	}

	data, err := json.MarshalIndent(output, "", "  ")
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
			"links":       links,
			"plain_notes": t.context.PlainNotes,
			"count":       len(links),
		},
	}, nil
}

// fetch fetches content from a specific URL
func (t *ResearchLinksTool) fetch(ctx context.Context, args map[string]interface{}) (*Result, error) {
	url, ok := args["url"].(string)
	if !ok || url == "" {
		return &Result{
			Success: false,
			Error:   "missing 'url' parameter",
		}, nil
	}

	summarize, _ := args["summarize"].(bool)

	// Check cache first
	t.mu.RLock()
	if cached, exists := t.cache[url]; exists {
		t.mu.RUnlock()
		return t.formatFetchedContent(cached, summarize)
	}
	t.mu.RUnlock()

	// Find the link metadata if it exists
	var linkMeta *ResearchLink
	if t.context != nil {
		for i := range t.context.Links {
			if t.context.Links[i].URL == url {
				linkMeta = &t.context.Links[i]
				break
			}
		}
	}

	// Fetch the content
	fetched := t.fetchURL(ctx, url, linkMeta)

	// Cache the result
	t.mu.Lock()
	t.cache[url] = fetched
	t.mu.Unlock()

	return t.formatFetchedContent(fetched, summarize)
}

// fetchAll fetches all research links
func (t *ResearchLinksTool) fetchAll(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if t.context == nil || len(t.context.Links) == 0 {
		return &Result{
			Success: true,
			Output:  "No research links to fetch.",
			Data: map[string]interface{}{
				"fetched": []FetchedContent{},
				"count":   0,
			},
		}, nil
	}

	summarize, _ := args["summarize"].(bool)

	// Fetch all links (with caching)
	results := make([]FetchedContent, 0, len(t.context.Links))
	for _, link := range t.context.Links {
		// Check cache
		t.mu.RLock()
		cached, exists := t.cache[link.URL]
		t.mu.RUnlock()

		var fetched *FetchedContent
		if exists {
			fetched = cached
		} else {
			fetched = t.fetchURL(ctx, link.URL, &link)
			t.mu.Lock()
			t.cache[link.URL] = fetched
			t.mu.Unlock()
		}

		results = append(results, *fetched)
	}

	// Build output
	output := struct {
		Fetched   []FetchedContent `json:"fetched"`
		Count     int              `json:"count"`
		Summarize bool             `json:"summarize"`
	}{
		Fetched:   results,
		Count:     len(results),
		Summarize: summarize,
	}

	data, err := json.MarshalIndent(output, "", "  ")
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
			"fetched": results,
			"count":   len(results),
		},
	}, nil
}

// fetchURL fetches content from a URL using the HTTP fetcher
func (t *ResearchLinksTool) fetchURL(ctx context.Context, url string, meta *ResearchLink) *FetchedContent {
	result := &FetchedContent{
		URL: url,
	}

	if meta != nil && meta.Title != "" {
		result.Title = meta.Title
	}

	opts := webscrape.DefaultFetchOptions()
	opts.ExtractContent = true
	opts.ExtractCode = true

	fetchResult, err := t.fetcher.Fetch(ctx, url, opts)
	if err != nil {
		result.Error = fmt.Sprintf("failed to fetch: %v", err)
		return result
	}

	result.Content = fetchResult.CleanText
	if result.Title == "" && fetchResult.Title != "" {
		result.Title = fetchResult.Title
	}

	// Extract code blocks if present
	if len(fetchResult.CodeBlocks) > 0 {
		result.CodeBlocks = make([]string, 0, len(fetchResult.CodeBlocks))
		for _, block := range fetchResult.CodeBlocks {
			result.CodeBlocks = append(result.CodeBlocks, block.Code)
		}
	}

	return result
}

// formatFetchedContent formats the fetched content as a Result
func (t *ResearchLinksTool) formatFetchedContent(content *FetchedContent, summarize bool) (*Result, error) {
	if content.Error != "" {
		return &Result{
			Success: false,
			Error:   content.Error,
			Data: map[string]interface{}{
				"url": content.URL,
			},
		}, nil
	}

	// Truncate content if very long (preserve first 10k chars)
	outputContent := content.Content
	if len(outputContent) > 10000 {
		outputContent = outputContent[:10000] + "\n\n[Content truncated...]"
	}

	output := struct {
		URL        string   `json:"url"`
		Title      string   `json:"title,omitempty"`
		Content    string   `json:"content"`
		CodeBlocks []string `json:"code_blocks,omitempty"`
	}{
		URL:        content.URL,
		Title:      content.Title,
		Content:    outputContent,
		CodeBlocks: content.CodeBlocks,
	}

	data, err := json.MarshalIndent(output, "", "  ")
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
			"url":         content.URL,
			"title":       content.Title,
			"content":     content.Content,
			"code_blocks": content.CodeBlocks,
		},
	}, nil
}

// Metadata returns rich tool metadata for discovery and LLM guidance
func (t *ResearchLinksTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []interface{}{"list", "fetch", "fetch_all"},
					Description: "The action to perform",
				},
				"url": {
					Type:        "string",
					Description: "URL to fetch (required for 'fetch' action)",
				},
				"summarize": {
					Type:        "boolean",
					Description: "Whether to summarize the fetched content (default: false)",
				},
				"filter_category": {
					Type:        "string",
					Enum:        []interface{}{"reference", "citation", "example", "research", "inspiration"},
					Description: "Filter links by category (for 'list' action)",
				},
			},
			Required: []string{"action"},
		},
		Category:    CategoryResearch,
		Optionality: ToolOptional,
		UsageHint: `Use when the user provides research URLs or mentions "links I provided", "these resources", or "references".
Categories guide usage:
- citation: Quote with [source](url) attribution
- reference: Background context
- example: Extract code patterns
- research: Synthesize into content`,
		Examples: []ToolExample{
			{
				Description: "List all available research links",
				Input:       map[string]interface{}{"action": "list"},
			},
			{
				Description: "Fetch content from a specific URL",
				Input:       map[string]interface{}{"action": "fetch", "url": "https://example.com/doc"},
			},
			{
				Description: "List only citation links",
				Input:       map[string]interface{}{"action": "list", "filter_category": "citation"},
			},
			{
				Description: "Fetch all research links",
				Input:       map[string]interface{}{"action": "fetch_all"},
			},
		},
		RequiresCapabilities: []string{"network"},
		Consumes:             []string{"research_links"},
		Produces:             []string{"fetched_content", "citations"},
	}
}

// GetLinks returns the links from the context
func (t *ResearchLinksTool) GetLinks() []ResearchLink {
	if t.context == nil {
		return nil
	}
	return t.context.Links
}

// GetPlainNotes returns the plain notes from the context
func (t *ResearchLinksTool) GetPlainNotes() string {
	if t.context == nil {
		return ""
	}
	return t.context.PlainNotes
}

// HasLinks returns true if there are any research links
func (t *ResearchLinksTool) HasLinks() bool {
	return t.context != nil && len(t.context.Links) > 0
}

// FormatAsMarkdown returns the research links formatted as markdown
func (t *ResearchLinksTool) FormatAsMarkdown() string {
	if t.context == nil || len(t.context.Links) == 0 {
		return ""
	}

	var md strings.Builder
	md.WriteString("### Research Links\n\n")

	// Group by category
	byCategory := make(map[string][]ResearchLink)
	uncategorized := make([]ResearchLink, 0)

	for _, link := range t.context.Links {
		if link.Category != "" {
			byCategory[link.Category] = append(byCategory[link.Category], link)
		} else {
			uncategorized = append(uncategorized, link)
		}
	}

	// Output by category
	categoryOrder := []string{"citation", "reference", "example", "research", "inspiration"}
	for _, cat := range categoryOrder {
		if links, exists := byCategory[cat]; exists {
			// Capitalize first letter manually to avoid deprecated strings.Title
			catTitle := strings.ToUpper(cat[:1]) + cat[1:]
			md.WriteString(fmt.Sprintf("**%s:**\n", catTitle))
			for _, link := range links {
				title := link.Title
				if title == "" {
					title = link.URL
				}
				md.WriteString(fmt.Sprintf("- [%s](%s)", title, link.URL))
				if link.Notes != "" {
					md.WriteString(fmt.Sprintf(" - %s", link.Notes))
				}
				md.WriteString("\n")
			}
			md.WriteString("\n")
		}
	}

	// Output uncategorized
	if len(uncategorized) > 0 {
		md.WriteString("**Other:**\n")
		for _, link := range uncategorized {
			title := link.Title
			if title == "" {
				title = link.URL
			}
			md.WriteString(fmt.Sprintf("- [%s](%s)", title, link.URL))
			if link.Notes != "" {
				md.WriteString(fmt.Sprintf(" - %s", link.Notes))
			}
			md.WriteString("\n")
		}
	}

	if t.context.PlainNotes != "" {
		md.WriteString("\n**Notes:**\n")
		md.WriteString(t.context.PlainNotes)
		md.WriteString("\n")
	}

	return md.String()
}
