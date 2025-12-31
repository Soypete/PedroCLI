package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/webscrape"
)

// GitLabHandler handles GitLab-specific fetching operations
type GitLabHandler struct {
	baseURL     string // Support self-hosted GitLab instances
	apiToken    string
	httpClient  *http.Client
	rateLimiter *webscrape.RateLimiter
}

// GitLabConfig configures the GitLab handler
type GitLabConfig struct {
	BaseURL  string // Default: "https://gitlab.com"
	APIToken string // Optional, for private repos and higher rate limits
	Timeout  time.Duration
}

// NewGitLabHandler creates a new GitLab handler
func NewGitLabHandler(cfg *GitLabConfig) *GitLabHandler {
	if cfg == nil {
		cfg = &GitLabConfig{}
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	h := &GitLabHandler{
		baseURL:  baseURL,
		apiToken: cfg.APIToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		rateLimiter: webscrape.NewRateLimiter(),
	}

	// Set rate limits for GitLab API
	domain := webscrape.ExtractDomain(baseURL)
	h.rateLimiter.SetLimit(domain, 10)

	return h
}

// FetchFile fetches a file from a GitLab repository
func (g *GitLabHandler) FetchFile(ctx context.Context, project, filePath, ref string) (*webscrape.GitLabFile, error) {
	if ref == "" {
		ref = "main"
	}

	// URL encode the project path and file path
	encodedProject := url.PathEscape(project)
	encodedPath := url.PathEscape(filePath)

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s?ref=%s",
		g.baseURL,
		encodedProject,
		encodedPath,
		url.QueryEscape(ref))

	var content struct {
		FileName      string `json:"file_name"`
		FilePath      string `json:"file_path"`
		Size          int    `json:"size"`
		Encoding      string `json:"encoding"`
		Content       string `json:"content"`
		ContentSHA256 string `json:"content_sha256"`
		Ref           string `json:"ref"`
		BlobID        string `json:"blob_id"`
		CommitID      string `json:"commit_id"`
		LastCommitID  string `json:"last_commit_id"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &content); err != nil {
		return nil, err
	}

	// Decode content
	var fileContent string
	if content.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(content.Content)
		if err != nil {
			return nil, &webscrape.ScrapeError{
				Type:    webscrape.ErrParseFailure,
				Message: fmt.Sprintf("failed to decode file content: %v", err),
			}
		}
		fileContent = string(decoded)
	} else {
		fileContent = content.Content
	}

	// Detect language from extension
	language := detectLanguageFromPath(content.FilePath)

	// Build URL to the file in GitLab UI
	fileURL := fmt.Sprintf("%s/%s/-/blob/%s/%s",
		g.baseURL,
		project,
		ref,
		filePath)

	return &webscrape.GitLabFile{
		Path:     content.FilePath,
		Content:  fileContent,
		Language: language,
		Size:     content.Size,
		BlobID:   content.BlobID,
		URL:      fileURL,
	}, nil
}

// FetchREADME fetches the README from a GitLab repository
func (g *GitLabHandler) FetchREADME(ctx context.Context, project string) (*webscrape.GitLabFile, error) {
	// Try common README filenames
	readmeNames := []string{"README.md", "README.rst", "README.txt", "README", "readme.md"}

	for _, name := range readmeNames {
		file, err := g.FetchFile(ctx, project, name, "")
		if err == nil {
			return file, nil
		}
		// Continue to next filename if not found
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok && scrapeErr.Type == webscrape.ErrNotFound {
			continue
		}
		// Return other errors
		return nil, err
	}

	return nil, &webscrape.ScrapeError{
		Type:       webscrape.ErrNotFound,
		Message:    "no README found in repository",
		Suggestion: "The repository may not have a README file",
	}
}

// FetchDirectory lists files in a GitLab repository directory
func (g *GitLabHandler) FetchDirectory(ctx context.Context, project, dirPath, ref string) ([]webscrape.GitHubEntry, error) {
	if ref == "" {
		ref = "main"
	}

	encodedProject := url.PathEscape(project)

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?path=%s&ref=%s",
		g.baseURL,
		encodedProject,
		url.QueryEscape(dirPath),
		url.QueryEscape(ref))

	var items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"` // "blob" or "tree"
		Path string `json:"path"`
		Mode string `json:"mode"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &items); err != nil {
		return nil, err
	}

	var entries []webscrape.GitHubEntry
	for _, item := range items {
		entryType := "file"
		if item.Type == "tree" {
			entryType = "dir"
		}

		fileURL := fmt.Sprintf("%s/%s/-/blob/%s/%s",
			g.baseURL,
			project,
			ref,
			item.Path)

		entries = append(entries, webscrape.GitHubEntry{
			Name: item.Name,
			Path: item.Path,
			Type: entryType,
			SHA:  item.ID,
			URL:  fileURL,
		})
	}

	return entries, nil
}

// SearchCode searches for code in a GitLab project or globally
func (g *GitLabHandler) SearchCode(ctx context.Context, query string, opts *webscrape.GitLabSearchOpts) ([]webscrape.GitLabSearchResult, error) {
	if opts == nil {
		opts = &webscrape.GitLabSearchOpts{}
	}

	scope := opts.Scope
	if scope == "" {
		scope = "blobs"
	}

	perPage := opts.PerPage
	if perPage == 0 {
		perPage = 20
	}

	apiURL := fmt.Sprintf("%s/api/v4/search?scope=%s&search=%s&per_page=%d",
		g.baseURL,
		scope,
		url.QueryEscape(query),
		perPage)

	var items []struct {
		Basename  string `json:"basename"`
		Data      string `json:"data"`
		Path      string `json:"path"`
		Filename  string `json:"filename"`
		ID        string `json:"id"`
		Ref       string `json:"ref"`
		Startline int    `json:"startline"`
		ProjectID int    `json:"project_id"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &items); err != nil {
		return nil, err
	}

	var results []webscrape.GitLabSearchResult
	for _, item := range items {
		results = append(results, webscrape.GitLabSearchResult{
			Project:  fmt.Sprintf("%d", item.ProjectID), // Would need another API call to get project path
			Path:     item.Path,
			Ref:      item.Ref,
			Fragment: item.Data,
		})
	}

	return results, nil
}

// SearchInProject searches for code within a specific project
func (g *GitLabHandler) SearchInProject(ctx context.Context, project, query string, opts *webscrape.GitLabSearchOpts) ([]webscrape.GitLabSearchResult, error) {
	if opts == nil {
		opts = &webscrape.GitLabSearchOpts{}
	}

	scope := opts.Scope
	if scope == "" {
		scope = "blobs"
	}

	perPage := opts.PerPage
	if perPage == 0 {
		perPage = 20
	}

	encodedProject := url.PathEscape(project)

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/search?scope=%s&search=%s&per_page=%d",
		g.baseURL,
		encodedProject,
		scope,
		url.QueryEscape(query),
		perPage)

	var items []struct {
		Basename  string `json:"basename"`
		Data      string `json:"data"`
		Path      string `json:"path"`
		Filename  string `json:"filename"`
		ID        string `json:"id"`
		Ref       string `json:"ref"`
		Startline int    `json:"startline"`
	}

	if err := g.doAPIRequest(ctx, apiURL, &items); err != nil {
		return nil, err
	}

	var results []webscrape.GitLabSearchResult
	for _, item := range items {
		fileURL := fmt.Sprintf("%s/%s/-/blob/%s/%s",
			g.baseURL,
			project,
			item.Ref,
			item.Path)

		results = append(results, webscrape.GitLabSearchResult{
			Project:  project,
			Path:     item.Path,
			Ref:      item.Ref,
			URL:      fileURL,
			Fragment: item.Data,
		})
	}

	return results, nil
}

// doAPIRequest performs an API request and decodes the JSON response
func (g *GitLabHandler) doAPIRequest(ctx context.Context, apiURL string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrNetwork,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	g.setHeaders(req)

	domain := webscrape.ExtractDomain(apiURL)
	if err := g.rateLimiter.Wait(ctx, domain); err != nil {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrNetwork,
			Message: fmt.Sprintf("failed to read response: %v", err),
		}
	}

	if err := json.Unmarshal(body, result); err != nil {
		return &webscrape.ScrapeError{
			Type:    webscrape.ErrParseFailure,
			Message: fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	return nil
}

// setHeaders sets common headers for GitLab API requests
func (g *GitLabHandler) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PedroCLI/1.0")

	if g.apiToken != "" {
		req.Header.Set("PRIVATE-TOKEN", g.apiToken)
	}
}

// handleHTTPError converts HTTP status codes to ScrapeError
func (g *GitLabHandler) handleHTTPError(statusCode int) *webscrape.ScrapeError {
	switch statusCode {
	case 401:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrAccessDenied,
			Message:    "GitLab API authentication failed",
			StatusCode: statusCode,
			Suggestion: "Check your GitLab API token",
		}
	case 403:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrAccessDenied,
			Message:    "access denied to GitLab resource",
			StatusCode: statusCode,
			Suggestion: "You may not have permission to access this resource",
		}
	case 404:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNotFound,
			Message:    "resource not found on GitLab",
			StatusCode: statusCode,
			Suggestion: "Verify the project path, file path, or reference exists",
		}
	case 429:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrRateLimited,
			Message:    "GitLab API rate limit exceeded",
			StatusCode: statusCode,
			Retryable:  true,
			Suggestion: "Wait before making more requests",
		}
	default:
		return &webscrape.ScrapeError{
			Type:       webscrape.ErrNetwork,
			Message:    fmt.Sprintf("GitLab API error: %d", statusCode),
			StatusCode: statusCode,
			Retryable:  statusCode >= 500,
		}
	}
}

// ParseGitLabURL parses a GitLab URL into project, path, and ref
func ParseGitLabURL(rawURL string) (project, filePath, ref string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", err
	}

	// Remove leading slash and split
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")

	// Find the separator (-/blob, -/tree, -/raw)
	separatorIdx := -1
	for i, part := range parts {
		if part == "-" && i+1 < len(parts) {
			separatorIdx = i
			break
		}
	}

	if separatorIdx == -1 {
		// No separator, just a project path
		project = strings.Join(parts, "/")
		return project, "", "", nil
	}

	project = strings.Join(parts[:separatorIdx], "/")

	// After separator: ["-", "blob"|"tree"|"raw", ref, path...]
	if separatorIdx+2 < len(parts) {
		ref = parts[separatorIdx+2]
	}

	if separatorIdx+3 < len(parts) {
		filePath = strings.Join(parts[separatorIdx+3:], "/")
	}

	return project, filePath, ref, nil
}
