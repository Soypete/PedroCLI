package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/logits"
)

// WebScraperTool scrapes code and content from local files, GitHub, and web URLs
type WebScraperTool struct {
	httpClient *http.Client
}

// NewWebScraperTool creates a new web scraper tool
func NewWebScraperTool() *WebScraperTool {
	return &WebScraperTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the tool name
func (t *WebScraperTool) Name() string {
	return "web_scraper"
}

// Description returns the tool description
func (t *WebScraperTool) Description() string {
	return `Scrape code and content from local files, GitHub repositories, or web URLs.

ACTIONS:
- scrape_local: Read a local code file
  Args: path (string, required), extract_code (bool, optional)

- scrape_github: Fetch a file from a public GitHub repository
  Args: repo (string, required, e.g., "kubernetes/kubernetes"),
        path (string, required, e.g., "pkg/api/api.go"),
        branch (string, optional, default: "main")

- scrape_url: Fetch and extract content from a web URL
  Args: url (string, required), extract_code (bool, optional)

All actions support:
- summarize (bool, optional): Summarize the content (requires LLM context)
- max_length (int, optional): Truncate output to max characters

Returns the scraped content as text.

EXAMPLES:
{"tool": "web_scraper", "args": {"action": "scrape_local", "path": "pkg/agents/base.go"}}
{"tool": "web_scraper", "args": {"action": "scrape_github", "repo": "torvalds/linux", "path": "kernel/sched/core.c"}}
{"tool": "web_scraper", "args": {"action": "scrape_url", "url": "https://go.dev/doc/", "extract_code": true}}`
}

// Execute executes the web scraper tool
func (t *WebScraperTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Parse action
	action, ok := args["action"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "action is required (scrape_local, scrape_github, scrape_url)",
		}, nil
	}

	// Parse common options
	extractCode := getBoolArg(args, "extract_code", false)
	summarize := getBoolArg(args, "summarize", false)
	maxLength := getIntArg(args, "max_length", 0)

	var content string
	var err error

	// Execute action
	switch action {
	case "scrape_local":
		path, ok := args["path"].(string)
		if !ok {
			return &Result{Success: false, Error: "path is required"}, nil
		}
		content, err = t.scrapeLocalFile(path)

	case "scrape_github":
		repo, ok := args["repo"].(string)
		if !ok {
			return &Result{Success: false, Error: "repo is required (e.g., 'kubernetes/kubernetes')"}, nil
		}
		path, ok := args["path"].(string)
		if !ok {
			return &Result{Success: false, Error: "path is required (e.g., 'pkg/api/api.go')"}, nil
		}
		branch := getStringArg(args, "branch", "main")
		content, err = t.scrapeGithub(ctx, repo, path, branch)

	case "scrape_url":
		url, ok := args["url"].(string)
		if !ok {
			return &Result{Success: false, Error: "url is required"}, nil
		}
		content, err = t.scrapeURL(ctx, url)

	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s (use scrape_local, scrape_github, scrape_url)", action),
		}, nil
	}

	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("scraping failed: %v", err),
		}, nil
	}

	// Extract code blocks if requested
	if extractCode {
		content = t.extractCodeBlocks(content)
	}

	// Truncate if max_length specified
	if maxLength > 0 && len(content) > maxLength {
		content = content[:maxLength] + "\n\n[truncated...]"
	}

	// Note: Summarization would require LLM access
	// For now, we just return a note if summarize is requested
	if summarize {
		content = "[Summarization requested but not yet implemented]\n\n" + content
	}

	return &Result{
		Success: true,
		Output:  content,
	}, nil
}

// scrapeLocalFile reads a local file
func (t *WebScraperTool) scrapeLocalFile(path string) (string, error) {
	// Clean path and check it exists
	cleanPath := filepath.Clean(path)

	// Security: prevent directory traversal outside current directory
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Ensure path is within current working directory
	if !strings.HasPrefix(absPath, cwd) {
		return "", fmt.Errorf("access denied: path outside working directory")
	}

	// Read file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}

// scrapeGithub fetches a file from GitHub using the raw content API
func (t *WebScraperTool) scrapeGithub(ctx context.Context, repo, path, branch string) (string, error) {
	// Build raw GitHub URL
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, branch, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned status %d (check repo/path/branch)", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(data), nil
}

// scrapeURL fetches content from a web URL
func (t *WebScraperTool) scrapeURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid blocks
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PedroCLI/1.0)")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	content := string(data)

	// Simple HTML cleanup if content type is HTML
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		content = t.stripHTMLTags(content)
	}

	return content, nil
}

// extractCodeBlocks extracts code blocks from markdown or HTML content
func (t *WebScraperTool) extractCodeBlocks(content string) string {
	var codeBlocks []string

	// Extract markdown code blocks (```language ... ```)
	markdownCodeRegex := regexp.MustCompile("(?s)```[a-z]*\n(.*?)```")
	matches := markdownCodeRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			codeBlocks = append(codeBlocks, match[1])
		}
	}

	// Extract HTML <code> and <pre> blocks
	htmlCodeRegex := regexp.MustCompile("(?s)<(?:code|pre)>(.*?)</(?:code|pre)>")
	matches = htmlCodeRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			cleaned := t.stripHTMLTags(match[1])
			codeBlocks = append(codeBlocks, cleaned)
		}
	}

	if len(codeBlocks) == 0 {
		return content // No code blocks found, return original
	}

	return strings.Join(codeBlocks, "\n\n---\n\n")
}

// stripHTMLTags removes HTML tags from content (basic implementation)
func (t *WebScraperTool) stripHTMLTags(html string) string {
	// Remove script tags with their content
	scriptRegex := regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	html = scriptRegex.ReplaceAllString(html, "")

	// Remove style tags with their content
	styleRegex := regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	html = styleRegex.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRegex := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRegex.ReplaceAllString(html, "")

	// Remove all HTML tags
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, " ")

	// Clean up whitespace
	text = strings.Join(strings.Fields(text), " ")

	return strings.TrimSpace(text)
}

// Helper functions for parsing args
func getBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	if val, ok := args[key].(int); ok {
		return val
	}
	return defaultVal
}

func getStringArg(args map[string]interface{}, key string, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
}

// Metadata returns tool metadata for registration
func (t *WebScraperTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Category:    CategoryResearch,
		Optionality: ToolOptional,
		UsageHint:   "Use to gather code examples, documentation, or technical content from local files, GitHub repositories, or web URLs before writing blog content",
		Examples: []ToolExample{
			{
				Description: "Scrape a local Go file",
				Input: map[string]interface{}{
					"action": "scrape_local",
					"path":   "pkg/agents/base.go",
				},
			},
			{
				Description: "Scrape a file from GitHub",
				Input: map[string]interface{}{
					"action": "scrape_github",
					"repo":   "kubernetes/kubernetes",
					"path":   "pkg/api/api.go",
					"branch": "main",
				},
			},
			{
				Description: "Scrape web documentation",
				Input: map[string]interface{}{
					"action":       "scrape_url",
					"url":          "https://go.dev/doc/",
					"extract_code": true,
				},
			},
		},
		RequiresCapabilities: []string{"network"},
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type:        "string",
					Description: "Action to perform: scrape_local, scrape_github, scrape_url",
					Enum:        []interface{}{"scrape_local", "scrape_github", "scrape_url"},
				},
				"path": {
					Type:        "string",
					Description: "File path (for scrape_local and scrape_github)",
				},
				"repo": {
					Type:        "string",
					Description: "GitHub repository in 'owner/repo' format (for scrape_github)",
				},
				"branch": {
					Type:        "string",
					Description: "Git branch name (for scrape_github, default: main)",
				},
				"url": {
					Type:        "string",
					Description: "Web URL to scrape (for scrape_url)",
				},
				"extract_code": {
					Type:        "boolean",
					Description: "Extract only code blocks from content",
				},
				"max_length": {
					Type:        "integer",
					Description: "Maximum length of returned content",
				},
			},
			Required: []string{"action"},
		},
	}
}

// MarshalJSON implements json.Marshaler for tool call formatting
func (t *WebScraperTool) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":        t.Name(),
		"description": t.Description(),
	})
}
