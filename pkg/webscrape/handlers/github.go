// Package handlers provides site-specific web scraping handlers
package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/webscrape"
)

// GitHubHandler handles GitHub-specific fetching operations
type GitHubHandler struct {
	apiToken    string
	httpClient  *http.Client
	rateLimiter *webscrape.RateLimiter
}

// GitHubConfig configures the GitHub handler
type GitHubConfig struct {
	APIToken string // Optional, for higher rate limits
	Timeout  time.Duration
}

// NewGitHubHandler creates a new GitHub handler
func NewGitHubHandler(cfg *GitHubConfig) *GitHubHandler {
	if cfg == nil {
		cfg = &GitHubConfig{}
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	h := &GitHubHandler{
		apiToken: cfg.APIToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		rateLimiter: webscrape.NewRateLimiter(),
	}

	// Set appropriate rate limits based on authentication
	if cfg.APIToken != "" {
		h.rateLimiter.SetLimit("api.github.com", 30)
	} else {
		h.rateLimiter.SetLimit("api.github.com", 10)
	}

	return h
}

// FetchFile fetches a file from a GitHub repository
func (g *GitHubHandler) FetchFile(ctx context.Context, owner, repo, filePath, ref string) (*webscrape.GitHubFile, error) {
	if ref == "" {
		ref = "main"
	}

	// Use Contents API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(owner),
		url.PathEscape(repo),
		filePath,
		url.QueryEscape(ref))

	var content struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		SHA         string `json:"sha"`
		Size        int    `json:"size"`
		URL         string `json:"url"`
		HTMLURL     string `json:"html_url"`
		DownloadURL string `json:"download_url"`
		Type        string `json:"type"`
		Content     string `json:"content"`
		Encoding    string `json:"encoding"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &content); err != nil {
		return nil, err
	}

	if content.Type != "file" {
		return nil, &webscrape.ScrapeError{
			Type:       webscrape.ErrNotFound,
			Message:    fmt.Sprintf("%s is not a file (type: %s)", filePath, content.Type),
			Suggestion: "Use FetchDirectory for directories",
		}
	}

	// Decode content
	var fileContent string
	if content.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
		if err != nil {
			return nil, &webscrape.ScrapeError{
				Type:    webscrape.ErrParseFailure,
				Message: fmt.Sprintf("failed to decode file content: %v", err),
			}
		}
		fileContent = string(decoded)
	} else if content.DownloadURL != "" {
		// Fetch raw content
		raw, err := g.fetchRaw(ctx, content.DownloadURL)
		if err != nil {
			return nil, err
		}
		fileContent = string(raw)
	}

	// Detect language from extension
	language := detectLanguageFromPath(content.Path)

	return &webscrape.GitHubFile{
		Path:     content.Path,
		Content:  fileContent,
		Language: language,
		Size:     content.Size,
		SHA:      content.SHA,
		URL:      content.HTMLURL,
	}, nil
}

// FetchREADME fetches the README from a GitHub repository
func (g *GitHubHandler) FetchREADME(ctx context.Context, owner, repo string) (*webscrape.GitHubFile, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/readme",
		url.PathEscape(owner),
		url.PathEscape(repo))

	var content struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		SHA         string `json:"sha"`
		Size        int    `json:"size"`
		URL         string `json:"url"`
		HTMLURL     string `json:"html_url"`
		DownloadURL string `json:"download_url"`
		Content     string `json:"content"`
		Encoding    string `json:"encoding"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &content); err != nil {
		return nil, err
	}

	// Decode content
	var fileContent string
	if content.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content.Content, "\n", ""))
		if err != nil {
			return nil, &webscrape.ScrapeError{
				Type:    webscrape.ErrParseFailure,
				Message: fmt.Sprintf("failed to decode README content: %v", err),
			}
		}
		fileContent = string(decoded)
	} else if content.DownloadURL != "" {
		raw, err := g.fetchRaw(ctx, content.DownloadURL)
		if err != nil {
			return nil, err
		}
		fileContent = string(raw)
	}

	return &webscrape.GitHubFile{
		Path:     content.Path,
		Content:  fileContent,
		Language: "markdown",
		Size:     content.Size,
		SHA:      content.SHA,
		URL:      content.HTMLURL,
	}, nil
}

// FetchDirectory lists files in a GitHub repository directory
func (g *GitHubHandler) FetchDirectory(ctx context.Context, owner, repo, dirPath, ref string) ([]webscrape.GitHubEntry, error) {
	if ref == "" {
		ref = "main"
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		url.PathEscape(owner),
		url.PathEscape(repo),
		dirPath,
		url.QueryEscape(ref))

	var contents []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		SHA         string `json:"sha"`
		Size        int    `json:"size"`
		URL         string `json:"url"`
		HTMLURL     string `json:"html_url"`
		DownloadURL string `json:"download_url"`
		Type        string `json:"type"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &contents); err != nil {
		return nil, err
	}

	var entries []webscrape.GitHubEntry
	for _, c := range contents {
		entries = append(entries, webscrape.GitHubEntry{
			Name: c.Name,
			Path: c.Path,
			Type: c.Type,
			Size: c.Size,
			SHA:  c.SHA,
			URL:  c.HTMLURL,
		})
	}

	return entries, nil
}

// SearchCode searches for code on GitHub
func (g *GitHubHandler) SearchCode(ctx context.Context, query string, opts *webscrape.GitHubSearchOpts) ([]webscrape.GitHubSearchResult, error) {
	if opts == nil {
		opts = &webscrape.GitHubSearchOpts{}
	}

	// Build search query
	q := query
	if opts.Language != "" {
		q += " language:" + opts.Language
	}
	if opts.Repo != "" {
		q += " repo:" + opts.Repo
	}
	if opts.Path != "" {
		q += " path:" + opts.Path
	}

	perPage := opts.PerPage
	if perPage == 0 {
		perPage = 30
	}

	apiURL := fmt.Sprintf("https://api.github.com/search/code?q=%s&per_page=%d",
		url.QueryEscape(q),
		perPage)

	var response struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Name       string `json:"name"`
			Path       string `json:"path"`
			SHA        string `json:"sha"`
			URL        string `json:"url"`
			HTMLURL    string `json:"html_url"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			TextMatches []struct {
				Fragment string `json:"fragment"`
			} `json:"text_matches"`
		} `json:"items"`
	}

	// Need to request text matches
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	g.setHeaders(req)
	req.Header.Set("Accept", "application/vnd.github.text-match+json")

	if err := g.rateLimiter.Wait(ctx, "api.github.com"); err != nil {
		return nil, err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, &webscrape.ScrapeError{
			Type:      webscrape.ErrNetwork,
			Message:   fmt.Sprintf("search request failed: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, g.handleHTTPError(resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, &webscrape.ScrapeError{
			Type:    webscrape.ErrParseFailure,
			Message: fmt.Sprintf("failed to parse search response: %v", err),
		}
	}

	var results []webscrape.GitHubSearchResult
	for _, item := range response.Items {
		fragment := ""
		if len(item.TextMatches) > 0 {
			fragment = item.TextMatches[0].Fragment
		}

		results = append(results, webscrape.GitHubSearchResult{
			Repository: item.Repository.FullName,
			Path:       item.Path,
			SHA:        item.SHA,
			URL:        item.HTMLURL,
			Fragment:   fragment,
		})
	}

	return results, nil
}

// FetchGist fetches a GitHub gist
func (g *GitHubHandler) FetchGist(ctx context.Context, gistID string) (*webscrape.Gist, error) {
	apiURL := fmt.Sprintf("https://api.github.com/gists/%s", url.PathEscape(gistID))

	var response struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Public      bool   `json:"public"`
		HTMLURL     string `json:"html_url"`
		CreatedAt   string `json:"created_at"`
		Files       map[string]struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
			Language string `json:"language"`
			Size     int    `json:"size"`
		} `json:"files"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &response); err != nil {
		return nil, err
	}

	files := make(map[string]string)
	for filename, file := range response.Files {
		files[filename] = file.Content
	}

	createdAt, _ := time.Parse(time.RFC3339, response.CreatedAt)

	return &webscrape.Gist{
		ID:          response.ID,
		Description: response.Description,
		Public:      response.Public,
		Files:       files,
		URL:         response.HTMLURL,
		CreatedAt:   createdAt,
	}, nil
}

// FetchRawURL fetches content from a raw.githubusercontent.com URL
func (g *GitHubHandler) FetchRawURL(ctx context.Context, rawURL string) (string, error) {
	if !strings.Contains(rawURL, "raw.githubusercontent.com") {
		return "", &webscrape.ScrapeError{
			Type:       webscrape.ErrInvalidURL,
			Message:    "not a raw.githubusercontent.com URL",
			Suggestion: "Use FetchFile for regular GitHub URLs",
		}
	}

	content, err := g.fetchRaw(ctx, rawURL)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// doAPIRequest performs an API request and decodes the JSON response
func (g *GitHubHandler) doAPIRequest(ctx context.Context, apiURL string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrNetwork,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	g.setHeaders(req)

	if err := g.rateLimiter.Wait(ctx, "api.github.com"); err != nil {
		return err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:      webscrape.ErrNetwork,
			Message:   fmt.Sprintf("request failed: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return g.handleHTTPError(resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrParseFailure,
			Message: fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	return nil
}

// fetchRaw fetches raw content from a URL
func (g *GitHubHandler) fetchRaw(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	g.setHeaders(req)

	domain := webscrape.ExtractDomain(rawURL)
	if err := g.rateLimiter.Wait(ctx, domain); err != nil {
		return nil, err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, &webscrape.ScrapeError{
			Type:      webscrape.ErrNetwork,
			Message:   fmt.Sprintf("failed to fetch raw content: %v", err),
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, g.handleHTTPError(resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// setHeaders sets common headers for GitHub API requests
func (g *GitHubHandler) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PedroCLI/1.0")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	if g.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiToken)
	}
}

// handleHTTPError converts HTTP status codes to ScrapeError
func (g *GitHubHandler) handleHTTPError(statusCode int) *webscrape.ScrapeError {
	switch statusCode {
	case 401:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrAccessDenied,
			Message:    "GitHub API authentication failed",
			StatusCode: statusCode,
			Suggestion: "Check your GitHub API token",
		}
	case 403:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrRateLimited,
			Message:    "GitHub API rate limit exceeded",
			StatusCode: statusCode,
			Retryable:  true,
			Suggestion: "Wait before making more requests or use an API token",
		}
	case 404:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNotFound,
			Message:    "resource not found on GitHub",
			StatusCode: statusCode,
			Suggestion: "Verify the repository, file path, or reference exists",
		}
	case 422:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrInvalidURL,
			Message:    "GitHub API validation failed",
			StatusCode: statusCode,
			Suggestion: "Check your query parameters",
		}
	default:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNetwork,
			Message:    fmt.Sprintf("GitHub API error: %d", statusCode),
			StatusCode: statusCode,
			Retryable:  statusCode >= 500,
		}
	}
}

// detectLanguageFromPath determines language from file extension
func detectLanguageFromPath(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))

	languages := map[string]string{
		".go":     "go",
		".py":     "python",
		".js":     "javascript",
		".ts":     "typescript",
		".tsx":    "typescript",
		".jsx":    "javascript",
		".java":   "java",
		".c":      "c",
		".cpp":    "cpp",
		".cc":     "cpp",
		".h":      "c",
		".hpp":    "cpp",
		".rs":     "rust",
		".rb":     "ruby",
		".php":    "php",
		".cs":     "csharp",
		".swift":  "swift",
		".kt":     "kotlin",
		".scala":  "scala",
		".sh":     "bash",
		".bash":   "bash",
		".zsh":    "zsh",
		".md":     "markdown",
		".json":   "json",
		".yaml":   "yaml",
		".yml":    "yaml",
		".xml":    "xml",
		".html":   "html",
		".css":    "css",
		".scss":   "scss",
		".sql":    "sql",
		".r":      "r",
		".R":      "r",
		".lua":    "lua",
		".perl":   "perl",
		".pl":     "perl",
		".ex":     "elixir",
		".exs":    "elixir",
		".erl":    "erlang",
		".clj":    "clojure",
		".hs":     "haskell",
		".ml":     "ocaml",
		".fs":     "fsharp",
		".dart":   "dart",
		".vue":    "vue",
		".svelte": "svelte",
	}

	if lang, ok := languages[ext]; ok {
		return lang
	}

	return ""
}

// ParseGitHubURL parses a GitHub URL into owner, repo, path, and ref
func ParseGitHubURL(rawURL string) (owner, repo, filePath, ref string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", "", err
	}

	if !strings.Contains(parsed.Host, "github.com") {
		return "", "", "", "", fmt.Errorf("not a GitHub URL")
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", "", "", fmt.Errorf("invalid GitHub URL format")
	}

	owner = parts[0]
	repo = parts[1]

	if len(parts) > 2 {
		switch parts[2] {
		case "blob", "tree":
			if len(parts) > 3 {
				ref = parts[3]
				if len(parts) > 4 {
					filePath = strings.Join(parts[4:], "/")
				}
			}
		case "raw":
			if len(parts) > 3 {
				ref = parts[3]
				if len(parts) > 4 {
					filePath = strings.Join(parts[4:], "/")
				}
			}
		}
	}

	return owner, repo, filePath, ref, nil
}
