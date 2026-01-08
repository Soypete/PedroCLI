package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/logits"
)

// WebSearchTool searches the web using DuckDuckGo
type WebSearchTool struct {
	httpClient *http.Client
}

// NewWebSearchTool creates a new web search tool
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the tool name
func (t *WebSearchTool) Name() string {
	return "web_search"
}

// Description returns the tool description
func (t *WebSearchTool) Description() string {
	return `Search the web using DuckDuckGo to find relevant URLs before scraping.

ARGUMENTS:
- query (string, required): Search query
- max_results (int, optional): Maximum number of results to return (default: 5, max: 10)
- filter (string, optional): Filter results by domain or keyword

Returns a list of search results with titles and URLs.

EXAMPLES:
{"tool": "web_search", "args": {"query": "golang error handling best practices"}}
{"tool": "web_search", "args": {"query": "kubernetes API design", "max_results": 3}}
{"tool": "web_search", "args": {"query": "site:github.com kubernetes operators", "filter": "github"}}`
}

// Execute executes the web search tool
func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Parse query
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &Result{
			Success: false,
			Error:   "query is required",
		}, nil
	}

	// Parse options
	maxResults := getIntArg(args, "max_results", 5)
	if maxResults > 10 {
		maxResults = 10 // Limit to prevent excessive requests
	}

	filter := getStringArg(args, "filter", "")

	// Perform search
	results, err := t.searchDuckDuckGo(ctx, query, maxResults)
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("search failed: %v", err),
		}, nil
	}

	// Apply filter if specified
	if filter != "" {
		results = t.filterResults(results, filter)
	}

	// Format output
	output := t.formatResults(results)

	return &Result{
		Success: true,
		Output:  output,
	}, nil
}

// searchDuckDuckGo performs a web search using DuckDuckGo HTML
func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	// Build DuckDuckGo search URL
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse results from HTML
	results := t.parseSearchResults(string(data), maxResults)

	return results, nil
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// parseSearchResults extracts search results from DuckDuckGo HTML
func (t *WebSearchTool) parseSearchResults(html string, maxResults int) []SearchResult {
	var results []SearchResult

	// DuckDuckGo HTML format: <a class="result__a" href="URL">Title</a>
	// Snippet: <a class="result__snippet">Text</a>

	// Extract result blocks (simplified parsing)
	resultRegex := regexp.MustCompile(`(?s)<div class="result[^"]*">.*?</div>\s*</div>`)
	resultBlocks := resultRegex.FindAllString(html, -1)

	for i, block := range resultBlocks {
		if i >= maxResults {
			break
		}

		result := SearchResult{}

		// Extract URL
		urlRegex := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]+)"`)
		if matches := urlRegex.FindStringSubmatch(block); len(matches) > 1 {
			result.URL = t.cleanURL(matches[1])
		}

		// Extract title
		titleRegex := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*>([^<]+)</a>`)
		if matches := titleRegex.FindStringSubmatch(block); len(matches) > 1 {
			result.Title = strings.TrimSpace(matches[1])
		}

		// Extract snippet
		snippetRegex := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>([^<]+)</a>`)
		if matches := snippetRegex.FindStringSubmatch(block); len(matches) > 1 {
			result.Snippet = strings.TrimSpace(matches[1])
		}

		// Only add if we have at least a URL
		if result.URL != "" {
			results = append(results, result)
		}
	}

	return results
}

// cleanURL removes DuckDuckGo redirect wrapper
func (t *WebSearchTool) cleanURL(rawURL string) string {
	// DuckDuckGo wraps URLs like: //duckduckgo.com/l/?uddg=ACTUAL_URL
	if strings.Contains(rawURL, "uddg=") {
		parts := strings.Split(rawURL, "uddg=")
		if len(parts) > 1 {
			decoded, err := url.QueryUnescape(parts[1])
			if err == nil {
				return decoded
			}
		}
	}

	// Handle relative URLs
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}

	return rawURL
}

// filterResults filters search results by domain or keyword
func (t *WebSearchTool) filterResults(results []SearchResult, filter string) []SearchResult {
	var filtered []SearchResult

	for _, result := range results {
		// Check if URL contains filter
		if strings.Contains(strings.ToLower(result.URL), strings.ToLower(filter)) {
			filtered = append(filtered, result)
			continue
		}

		// Check if title or snippet contains filter
		if strings.Contains(strings.ToLower(result.Title), strings.ToLower(filter)) ||
			strings.Contains(strings.ToLower(result.Snippet), strings.ToLower(filter)) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// formatResults formats search results as readable text
func (t *WebSearchTool) formatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d result(s):\n\n", len(results)))

	for i, result := range results {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		if result.Snippet != "" {
			output.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
		}
		output.WriteString("\n")
	}

	return output.String()
}

// Metadata returns tool metadata for registration
func (t *WebSearchTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Category:    CategoryResearch,
		Optionality: ToolOptional,
		UsageHint:   "Use to find relevant URLs before scraping content. Helpful for discovering documentation, code examples, and technical articles.",
		Examples: []ToolExample{
			{
				Description: "Search for Go best practices",
				Input: map[string]interface{}{
					"query": "golang error handling best practices",
				},
			},
			{
				Description: "Search GitHub for examples",
				Input: map[string]interface{}{
					"query":       "site:github.com kubernetes operators",
					"max_results": 3,
				},
			},
			{
				Description: "Search with filter",
				Input: map[string]interface{}{
					"query":  "docker compose tutorial",
					"filter": "docker",
				},
			},
		},
		RequiresCapabilities: []string{"network"},
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"query": {
					Type:        "string",
					Description: "Search query (supports site:domain.com syntax)",
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of results (default: 5, max: 10)",
				},
				"filter": {
					Type:        "string",
					Description: "Filter results by domain or keyword",
				},
			},
			Required: []string{"query"},
		},
	}
}
