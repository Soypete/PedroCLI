package webscrape

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SearchEngine defines the interface for web search engines
type SearchEngine interface {
	Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error)
}

// DuckDuckGoSearch implements search using DuckDuckGo
type DuckDuckGoSearch struct {
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

// NewDuckDuckGoSearch creates a new DuckDuckGo search engine
func NewDuckDuckGoSearch() *DuckDuckGoSearch {
	return &DuckDuckGoSearch{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(),
	}
}

// Search performs a search using DuckDuckGo's HTML interface
// Note: DuckDuckGo doesn't have a free public API, so we use their HTML interface
func (d *DuckDuckGoSearch) Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	// Build search query
	q := query
	if opts.Site != "" {
		q = fmt.Sprintf("site:%s %s", opts.Site, query)
	}
	if opts.FileType != "" {
		q = fmt.Sprintf("filetype:%s %s", opts.FileType, q)
	}

	// Use DuckDuckGo HTML version (no API key needed)
	// We'll use the lite version for easier parsing
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(q))

	if err := d.rateLimiter.Wait(ctx, "duckduckgo.com"); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, &ScrapeError{
			Type:    ErrNetwork,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	req.Header.Set("User-Agent", "PedroCLI/1.0 (Web Scraping Tool)")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, &ScrapeError{
			Type:      ErrNetwork,
			Message:   fmt.Sprintf("search request failed: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ScrapeError{
			Type:       ErrNetwork,
			Message:    fmt.Sprintf("search returned status %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
			Retryable:  resp.StatusCode >= 500,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ScrapeError{
			Type:    ErrNetwork,
			Message: fmt.Sprintf("failed to read response: %v", err),
		}
	}

	return d.parseResults(string(body), opts.MaxResults)
}

// parseResults parses search results from DuckDuckGo HTML
func (d *DuckDuckGoSearch) parseResults(html string, maxResults int) ([]SearchResult, error) {
	var results []SearchResult

	// Simple parsing of DuckDuckGo HTML results
	// Look for result links with class "result__a"
	// Format: <a rel="nofollow" class="result__a" href="...">Title</a>

	// Split by result divs
	parts := strings.Split(html, `class="result__body"`)

	for _, part := range parts[1:] { // Skip first part (before results)
		if len(results) >= maxResults {
			break
		}

		// Find title and URL
		titleStart := strings.Index(part, `class="result__a"`)
		if titleStart == -1 {
			continue
		}

		// Find href
		hrefStart := strings.Index(part[titleStart:], `href="`)
		if hrefStart == -1 {
			continue
		}
		hrefStart += titleStart + 6 // Move past 'href="'

		hrefEnd := strings.Index(part[hrefStart:], `"`)
		if hrefEnd == -1 {
			continue
		}

		rawURL := part[hrefStart : hrefStart+hrefEnd]

		// DuckDuckGo uses redirect URLs, extract the actual URL
		actualURL := extractDDGURL(rawURL)
		if actualURL == "" {
			actualURL = rawURL
		}

		// Find title text
		titleTextStart := strings.Index(part[hrefStart:], ">")
		if titleTextStart == -1 {
			continue
		}
		titleTextStart += hrefStart + 1

		titleTextEnd := strings.Index(part[titleTextStart:], "</a>")
		if titleTextEnd == -1 {
			continue
		}

		title := stripTags(part[titleTextStart : titleTextStart+titleTextEnd])

		// Find snippet
		snippetStart := strings.Index(part, `class="result__snippet"`)
		snippet := ""
		if snippetStart != -1 {
			snippetTextStart := strings.Index(part[snippetStart:], ">")
			if snippetTextStart != -1 {
				snippetTextStart += snippetStart + 1
				snippetTextEnd := strings.Index(part[snippetTextStart:], "</a>")
				if snippetTextEnd == -1 {
					snippetTextEnd = strings.Index(part[snippetTextStart:], "</span>")
				}
				if snippetTextEnd != -1 {
					snippet = stripTags(part[snippetTextStart : snippetTextStart+snippetTextEnd])
				}
			}
		}

		if title != "" && actualURL != "" {
			results = append(results, SearchResult{
				Title:   strings.TrimSpace(title),
				URL:     actualURL,
				Snippet: strings.TrimSpace(snippet),
				Source:  "duckduckgo",
			})
		}
	}

	return results, nil
}

// extractDDGURL extracts the actual URL from DuckDuckGo redirect URL
func extractDDGURL(ddgURL string) string {
	// DuckDuckGo uses URLs like:
	// //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2F&rut=...
	if strings.Contains(ddgURL, "uddg=") {
		parsed, err := url.Parse(ddgURL)
		if err != nil {
			return ""
		}
		uddg := parsed.Query().Get("uddg")
		if uddg != "" {
			decoded, err := url.QueryUnescape(uddg)
			if err == nil {
				return decoded
			}
		}
	}
	return ddgURL
}

// stripTags removes HTML tags from a string
func stripTags(s string) string {
	inTag := false
	var result strings.Builder
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
	text := result.String()
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	return text
}

// SearXNGSearch implements search using a SearXNG instance
type SearXNGSearch struct {
	baseURL     string
	httpClient  *http.Client
	rateLimiter *RateLimiter
}

// NewSearXNGSearch creates a new SearXNG search engine
func NewSearXNGSearch(baseURL string) *SearXNGSearch {
	return &SearXNGSearch{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: NewRateLimiter(),
	}
}

// Search performs a search using a SearXNG instance
func (s *SearXNGSearch) Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	// Build search query
	q := query
	if opts.Site != "" {
		q = fmt.Sprintf("site:%s %s", opts.Site, query)
	}

	// SearXNG JSON API
	params := url.Values{}
	params.Set("q", q)
	params.Set("format", "json")
	params.Set("categories", "general")

	if opts.SafeSearch {
		params.Set("safesearch", "1")
	}

	searchURL := fmt.Sprintf("%s/search?%s", s.baseURL, params.Encode())

	domain := ExtractDomain(s.baseURL)
	if err := s.rateLimiter.Wait(ctx, domain); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, &ScrapeError{
			Type:    ErrNetwork,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "PedroCLI/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, &ScrapeError{
			Type:      ErrNetwork,
			Message:   fmt.Sprintf("search request failed: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ScrapeError{
			Type:       ErrNetwork,
			Message:    fmt.Sprintf("search returned status %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
		}
	}

	var response struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
			Engine  string `json:"engine"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, &ScrapeError{
			Type:    ErrParseFailure,
			Message: fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	var results []SearchResult
	for i, r := range response.Results {
		if i >= opts.MaxResults {
			break
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Source:  "searxng",
		})
	}

	return results, nil
}

// MetaSearch aggregates results from multiple search engines
type MetaSearch struct {
	engines []SearchEngine
}

// NewMetaSearch creates a new meta search engine
func NewMetaSearch(engines ...SearchEngine) *MetaSearch {
	return &MetaSearch{
		engines: engines,
	}
}

// Search performs a search across all configured engines
func (m *MetaSearch) Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	var allResults []SearchResult
	seen := make(map[string]bool)

	for _, engine := range m.engines {
		results, err := engine.Search(ctx, query, opts)
		if err != nil {
			// Continue with other engines
			continue
		}

		for _, r := range results {
			// Deduplicate by URL
			if !seen[r.URL] {
				seen[r.URL] = true
				allResults = append(allResults, r)
			}
		}

		// Stop if we have enough results
		if len(allResults) >= opts.MaxResults {
			break
		}
	}

	// Limit to requested number
	if len(allResults) > opts.MaxResults {
		allResults = allResults[:opts.MaxResults]
	}

	return allResults, nil
}
