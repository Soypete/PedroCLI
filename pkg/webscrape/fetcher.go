package webscrape

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher is the main interface for fetching web content
type Fetcher interface {
	Fetch(ctx context.Context, url string, opts *FetchOptions) (*FetchResult, error)
}

// HTTPFetcher implements Fetcher using HTTP
type HTTPFetcher struct {
	client      *http.Client
	rateLimiter *RateLimiter
	cache       *Cache
	extractor   *ReadabilityExtractor
}

// FetcherConfig configures the HTTP fetcher
type FetcherConfig struct {
	UserAgent    string
	Timeout      time.Duration
	MaxRedirects int
	Cache        *CacheConfig
	RateLimits   map[string]float64
}

// DefaultFetcherConfig returns default fetcher configuration
func DefaultFetcherConfig() *FetcherConfig {
	return &FetcherConfig{
		UserAgent:    "PedroCLI/1.0 (Web Scraping Tool)",
		Timeout:      30 * time.Second,
		MaxRedirects: 10,
		Cache:        DefaultCacheConfig(),
		RateLimits:   DefaultRateLimits,
	}
}

// NewHTTPFetcher creates a new HTTP fetcher
func NewHTTPFetcher(cfg *FetcherConfig) (*HTTPFetcher, error) {
	if cfg == nil {
		cfg = DefaultFetcherConfig()
	}

	// Create HTTP client with custom transport
	transport := &http.Transport{
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
		DisableKeepAlives:  false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("too many redirects (max %d)", cfg.MaxRedirects)
			}
			return nil
		},
	}

	// Create rate limiter
	rateLimiter := NewRateLimiter()
	for domain, rate := range cfg.RateLimits {
		if domain != "default" {
			rateLimiter.SetLimit(domain, rate)
		}
	}

	// Create cache
	var cache *Cache
	if cfg.Cache != nil && cfg.Cache.Enabled {
		var err error
		cache, err = NewCache(cfg.Cache)
		if err != nil {
			// Log but don't fail - caching is optional
			cache = nil
		}
	}

	return &HTTPFetcher{
		client:      client,
		rateLimiter: rateLimiter,
		cache:       cache,
		extractor:   NewReadabilityExtractor(),
	}, nil
}

// Fetch fetches content from a URL
func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string, opts *FetchOptions) (*FetchResult, error) {
	if opts == nil {
		opts = DefaultFetchOptions()
	}

	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, &ScrapeError{
			Type:       ErrInvalidURL,
			Message:    fmt.Sprintf("invalid URL: %s", rawURL),
			Suggestion: "Please provide a valid URL with scheme (http:// or https://)",
		}
	}

	// Normalize URL scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, &ScrapeError{
			Type:       ErrInvalidURL,
			Message:    fmt.Sprintf("unsupported URL scheme: %s", parsedURL.Scheme),
			Suggestion: "Use http:// or https:// URLs",
		}
	}

	// Check cache first
	if f.cache != nil {
		if cached, found := f.cache.Get(rawURL); found {
			return cached, nil
		}
	}

	// Wait for rate limit
	if err := f.rateLimiter.WaitForURL(ctx, rawURL); err != nil {
		return nil, &ScrapeError{
			Type:       ErrTimeout,
			Message:    "rate limit wait cancelled",
			Retryable:  true,
			Suggestion: "Try again in a few seconds",
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, &ScrapeError{
			Type:      ErrNetwork,
			Message:   fmt.Sprintf("failed to create request: %v", err),
			Retryable: true,
		}
	}

	// Set headers
	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ScrapeError{
				Type:       ErrTimeout,
				Message:    "request cancelled or timed out",
				Retryable:  true,
				Suggestion: "Try again with a longer timeout",
			}
		}
		return nil, &ScrapeError{
			Type:       ErrNetwork,
			Message:    fmt.Sprintf("network error: %v", err),
			Retryable:  true,
			Suggestion: "Check network connectivity and try again",
		}
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode >= 400 {
		return nil, f.handleHTTPError(resp.StatusCode, rawURL)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, opts.MaxSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, &ScrapeError{
			Type:      ErrNetwork,
			Message:   fmt.Sprintf("failed to read response: %v", err),
			Retryable: true,
		}
	}

	// Build result
	result := &FetchResult{
		URL:         resp.Request.URL.String(), // Final URL after redirects
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		RawHTML:     string(body),
		FetchedAt:   time.Now(),
	}

	// Extract content if requested
	if opts.ExtractContent || opts.ExtractCode {
		extracted, err := f.extractor.Extract(result.RawHTML, result.URL)
		if err == nil {
			result.Title = extracted.Title
			result.CleanText = extracted.MainText
			if opts.ExtractCode {
				result.CodeBlocks = extracted.CodeBlocks
			}
			if desc, ok := extracted.Metadata["description"]; ok {
				result.Description = desc
			}
		}
	}

	// Extract links
	result.Links = ExtractLinks(result.RawHTML)

	// Cache the result
	if f.cache != nil {
		_ = f.cache.Set(rawURL, result)
	}

	return result, nil
}

// handleHTTPError converts HTTP status codes to ScrapeError
func (f *HTTPFetcher) handleHTTPError(statusCode int, url string) *ScrapeError {
	switch {
	case statusCode == 404:
		return &ScrapeError{
			Type:       ErrNotFound,
			Message:    fmt.Sprintf("page not found: %s", url),
			StatusCode: statusCode,
			Retryable:  false,
			Suggestion: "Verify the URL is correct",
		}
	case statusCode == 401 || statusCode == 403:
		return &ScrapeError{
			Type:       ErrAccessDenied,
			Message:    fmt.Sprintf("access denied: %s", url),
			StatusCode: statusCode,
			Retryable:  false,
			Suggestion: "This resource may require authentication or be private",
		}
	case statusCode == 429:
		return &ScrapeError{
			Type:       ErrRateLimited,
			Message:    fmt.Sprintf("rate limited: %s", url),
			StatusCode: statusCode,
			Retryable:  true,
			Suggestion: "Wait a few seconds and try again",
		}
	case statusCode >= 500:
		return &ScrapeError{
			Type:       ErrNetwork,
			Message:    fmt.Sprintf("server error (%d): %s", statusCode, url),
			StatusCode: statusCode,
			Retryable:  true,
			Suggestion: "The server may be temporarily unavailable. Try again later.",
		}
	default:
		return &ScrapeError{
			Type:       ErrNetwork,
			Message:    fmt.Sprintf("HTTP error %d: %s", statusCode, url),
			StatusCode: statusCode,
			Retryable:  statusCode < 400,
		}
	}
}

// FetchRaw fetches raw content without any processing
func (f *HTTPFetcher) FetchRaw(ctx context.Context, rawURL string, opts *FetchOptions) ([]byte, error) {
	if opts == nil {
		opts = DefaultFetchOptions()
	}

	// Validate URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, &ScrapeError{
			Type:    ErrInvalidURL,
			Message: fmt.Sprintf("invalid URL: %s", rawURL),
		}
	}

	// Wait for rate limit
	if err := f.rateLimiter.WaitForURL(ctx, rawURL); err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", opts.UserAgent)
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, f.handleHTTPError(resp.StatusCode, rawURL)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, opts.MaxSize)
	return io.ReadAll(limitedReader)
}

// Close cleans up resources
func (f *HTTPFetcher) Close() error {
	if f.cache != nil {
		return f.cache.Close()
	}
	return nil
}

// FormatForAgent converts a FetchResult to an AgentResponse
func FormatForAgent(result *FetchResult) *AgentResponse {
	resp := &AgentResponse{
		Success:     true,
		Source:      result.URL,
		ContentType: detectContentType(result),
		Summary:     generateSummary(result),
		Content:     result.CleanText,
		Metadata:    make(map[string]interface{}),
	}

	// Format code blocks
	for _, block := range result.CodeBlocks {
		resp.CodeBlocks = append(resp.CodeBlocks, AgentCodeBlock{
			Language:    block.Language,
			Code:        block.Code,
			Description: block.Context,
		})
	}

	// Format links
	for _, link := range result.Links {
		resp.Links = append(resp.Links, AgentLink(link))
	}

	resp.Metadata["title"] = result.Title
	resp.Metadata["fetched_at"] = result.FetchedAt

	return resp
}

func detectContentType(result *FetchResult) string {
	if len(result.CodeBlocks) > 0 {
		if len(result.CleanText) > len(result.CodeBlocks[0].Code)*2 {
			return "mixed"
		}
		return "code"
	}
	return "text"
}

func generateSummary(result *FetchResult) string {
	var parts []string

	if result.Title != "" {
		parts = append(parts, result.Title)
	}

	if result.Description != "" {
		parts = append(parts, result.Description)
	}

	if len(result.CodeBlocks) > 0 {
		parts = append(parts, fmt.Sprintf("Contains %d code block(s)", len(result.CodeBlocks)))
	}

	if len(parts) == 0 {
		// Generate from clean text
		text := result.CleanText
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		return strings.TrimSpace(text)
	}

	return strings.Join(parts, " - ")
}
