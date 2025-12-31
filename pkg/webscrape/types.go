// Package webscrape provides web scraping capabilities for fetching code and documentation
// from external sources like GitHub, GitLab, Stack Overflow, and general web pages.
package webscrape

import (
	"time"
)

// FetchOptions configures how a URL should be fetched
type FetchOptions struct {
	UserAgent       string            // HTTP User-Agent header
	Timeout         time.Duration     // Request timeout
	FollowRedirects bool              // Whether to follow HTTP redirects
	MaxSize         int64             // Maximum response size in bytes
	Headers         map[string]string // Additional HTTP headers
	ExtractContent  bool              // Strip HTML, return clean text
	ExtractCode     bool              // Extract code blocks specifically
}

// DefaultFetchOptions returns sensible default fetch options
func DefaultFetchOptions() *FetchOptions {
	return &FetchOptions{
		UserAgent:       "PedroCLI/1.0 (Web Scraping Tool)",
		Timeout:         30 * time.Second,
		FollowRedirects: true,
		MaxSize:         10 * 1024 * 1024, // 10MB
		ExtractContent:  true,
		ExtractCode:     true,
	}
}

// FetchResult contains the result of fetching a URL
type FetchResult struct {
	URL         string      `json:"url"`
	StatusCode  int         `json:"status_code"`
	ContentType string      `json:"content_type"`
	RawHTML     string      `json:"raw_html,omitempty"`
	CleanText   string      `json:"clean_text,omitempty"`
	CodeBlocks  []CodeBlock `json:"code_blocks,omitempty"`
	Links       []Link      `json:"links,omitempty"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	FetchedAt   time.Time   `json:"fetched_at"`
}

// CodeBlock represents an extracted code block from a page
type CodeBlock struct {
	Language string `json:"language,omitempty"`
	Code     string `json:"code"`
	Context  string `json:"context,omitempty"` // Surrounding text/description
	LineNum  int    `json:"line_num,omitempty"`
}

// Link represents an extracted hyperlink
type Link struct {
	Text string `json:"text"`
	URL  string `json:"url"`
	Type string `json:"type,omitempty"` // documentation, source, reference
}

// SearchOptions configures web search behavior
type SearchOptions struct {
	MaxResults int    // Maximum number of results to return
	SafeSearch bool   // Enable safe search filtering
	TimeRange  string // day, week, month, year
	Site       string // Limit to specific site (e.g., "github.com")
	FileType   string // Limit to file type
}

// DefaultSearchOptions returns sensible default search options
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		MaxResults: 10,
		SafeSearch: true,
	}
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"` // google, duckduckgo, etc.
}

// ExtractedContent represents content extracted from an HTML page
type ExtractedContent struct {
	Title      string            `json:"title"`
	MainText   string            `json:"main_text"`
	CodeBlocks []CodeBlock       `json:"code_blocks,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// GitHubFile represents a file from a GitHub repository
type GitHubFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
	Size     int    `json:"size"`
	SHA      string `json:"sha"`
	URL      string `json:"url"`
}

// GitHubEntry represents a file or directory in a GitHub repository
type GitHubEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
	Size int    `json:"size,omitempty"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitHubSearchResult represents a code search result from GitHub
type GitHubSearchResult struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	SHA        string `json:"sha"`
	URL        string `json:"url"`
	Fragment   string `json:"fragment,omitempty"`
}

// GitHubSearchOpts contains options for GitHub code search
type GitHubSearchOpts struct {
	Language string `json:"language,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Path     string `json:"path,omitempty"`
	PerPage  int    `json:"per_page,omitempty"`
}

// Gist represents a GitHub gist
type Gist struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Public      bool              `json:"public"`
	Files       map[string]string `json:"files"` // filename -> content
	URL         string            `json:"url"`
	CreatedAt   time.Time         `json:"created_at"`
}

// GitLabFile represents a file from a GitLab repository
type GitLabFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Language string `json:"language,omitempty"`
	Size     int    `json:"size"`
	BlobID   string `json:"blob_id"`
	URL      string `json:"url"`
}

// GitLabSearchOpts contains options for GitLab code search
type GitLabSearchOpts struct {
	Scope   string `json:"scope,omitempty"` // blobs, commits, etc.
	PerPage int    `json:"per_page,omitempty"`
}

// GitLabSearchResult represents a code search result from GitLab
type GitLabSearchResult struct {
	Project  string `json:"project"`
	Path     string `json:"path"`
	Ref      string `json:"ref"`
	URL      string `json:"url"`
	Fragment string `json:"fragment,omitempty"`
}

// SOQuestion represents a Stack Overflow question
type SOQuestion struct {
	ID           int        `json:"id"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	BodyMarkdown string     `json:"body_markdown,omitempty"`
	Tags         []string   `json:"tags"`
	Score        int        `json:"score"`
	Answers      []SOAnswer `json:"answers,omitempty"`
	AcceptedID   *int       `json:"accepted_id,omitempty"`
	URL          string     `json:"url"`
	CreatedAt    time.Time  `json:"created_at"`
}

// SOAnswer represents a Stack Overflow answer
type SOAnswer struct {
	ID           int         `json:"id"`
	Body         string      `json:"body"`
	BodyMarkdown string      `json:"body_markdown,omitempty"`
	CodeBlocks   []CodeBlock `json:"code_blocks,omitempty"`
	Score        int         `json:"score"`
	IsAccepted   bool        `json:"is_accepted"`
	URL          string      `json:"url"`
}

// SOSearchResult represents a Stack Overflow search result
type SOSearchResult struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Score     int      `json:"score"`
	Answers   int      `json:"answers"`
	URL       string   `json:"url"`
	IsAnswerd bool     `json:"is_answered"`
}

// DocsPage represents a documentation page
type DocsPage struct {
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	CodeBlocks  []CodeBlock `json:"code_blocks,omitempty"`
	Sections    []string    `json:"sections,omitempty"`
	Breadcrumbs []Link      `json:"breadcrumbs,omitempty"`
	URL         string      `json:"url"`
}

// AgentResponse formats responses for easy agent consumption
type AgentResponse struct {
	Success     bool                   `json:"success"`
	Source      string                 `json:"source"`       // URL or search engine
	ContentType string                 `json:"content_type"` // text, code, mixed
	Summary     string                 `json:"summary"`      // Brief summary for context
	Content     string                 `json:"content"`      // Main content
	CodeBlocks  []AgentCodeBlock       `json:"code_blocks,omitempty"`
	Links       []AgentLink            `json:"links,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CachedAt    *time.Time             `json:"cached_at,omitempty"`
}

// AgentCodeBlock is a simplified code block for agent responses
type AgentCodeBlock struct {
	Language    string `json:"language"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"` // File path or URL
}

// AgentLink is a simplified link for agent responses
type AgentLink struct {
	Text string `json:"text"`
	URL  string `json:"url"`
	Type string `json:"type,omitempty"` // documentation, source, reference
}

// ErrorType represents the type of scraping error
type ErrorType string

const (
	ErrRateLimited  ErrorType = "rate_limited"
	ErrNotFound     ErrorType = "not_found"
	ErrAccessDenied ErrorType = "access_denied"
	ErrTimeout      ErrorType = "timeout"
	ErrInvalidURL   ErrorType = "invalid_url"
	ErrParseFailure ErrorType = "parse_failure"
	ErrNetwork      ErrorType = "network_error"
)

// ScrapeError represents a scraping error with context for agents
type ScrapeError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	StatusCode int       `json:"status_code,omitempty"`
	Retryable  bool      `json:"retryable"`
	Suggestion string    `json:"suggestion"` // What agent should do
}

func (e *ScrapeError) Error() string {
	return e.Message
}

// ForAgent returns a user-friendly error message for the agent
func (e *ScrapeError) ForAgent() string {
	switch e.Type {
	case ErrRateLimited:
		return "Rate limited. Try again in a few seconds or use cached version."
	case ErrNotFound:
		return "Page not found. Verify the URL is correct."
	case ErrAccessDenied:
		return "Access denied. This may be a private resource."
	case ErrTimeout:
		return "Request timed out. The server may be slow or unresponsive."
	case ErrInvalidURL:
		return "Invalid URL format. Please check the URL."
	case ErrParseFailure:
		return "Failed to parse page content. The page format may be unsupported."
	case ErrNetwork:
		return "Network error. Check connectivity and try again."
	default:
		return e.Message
	}
}

// CacheEntry represents a cached fetch result
type CacheEntry struct {
	ID          string    `json:"id"`
	URLHash     string    `json:"url_hash"`
	URL         string    `json:"url"`
	ContentType string    `json:"content_type"`
	RawContent  string    `json:"raw_content"`
	CleanText   string    `json:"clean_text"`
	CodeBlocks  []byte    `json:"code_blocks"` // JSON-encoded
	Metadata    []byte    `json:"metadata"`    // JSON-encoded
	FetchedAt   time.Time `json:"fetched_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// RateLimitConfig defines rate limit settings per domain
type RateLimitConfig struct {
	Domain         string        `json:"domain"`
	RequestsPerSec float64       `json:"requests_per_sec"`
	BurstSize      int           `json:"burst_size"`
	RetryAfter     time.Duration `json:"retry_after,omitempty"`
}

// DefaultRateLimits provides default rate limits for common domains
var DefaultRateLimits = map[string]float64{
	"github.com":            10, // 10 req/s without auth
	"api.github.com":        30, // With auth
	"gitlab.com":            10,
	"api.gitlab.com":        10,
	"stackoverflow.com":     30, // SE API is generous
	"api.stackexchange.com": 30,
	"default":               2, // Conservative default
}
