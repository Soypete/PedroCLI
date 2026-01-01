package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/logits"
	"github.com/soypete/pedrocli/pkg/webscrape"
	"github.com/soypete/pedrocli/pkg/webscrape/handlers"
)

// WebScrapeTool provides web scraping capabilities via MCP
type WebScrapeTool struct {
	config           *config.Config
	fetcher          *webscrape.HTTPFetcher
	githubHandler    *handlers.GitHubHandler
	gitlabHandler    *handlers.GitLabHandler
	stackoverHandler *handlers.StackOverflowHandler
	searchEngine     webscrape.SearchEngine
}

// NewWebScrapeTool creates a new web scraping tool
func NewWebScrapeTool(cfg *config.Config, tokenManager TokenManager) *WebScrapeTool {
	// Get tokens if available
	var githubToken, gitlabToken, soAPIKey string
	if tokenManager != nil {
		ctx := context.Background()
		githubToken, _ = tokenManager.GetToken(ctx, "github", "api")
		gitlabToken, _ = tokenManager.GetToken(ctx, "gitlab", "api")
		soAPIKey, _ = tokenManager.GetToken(ctx, "stackoverflow", "api")
	}

	// Create HTTP fetcher
	fetcherCfg := webscrape.DefaultFetcherConfig()
	if cfg != nil && cfg.WebScraping.UserAgent != "" {
		fetcherCfg.UserAgent = cfg.WebScraping.UserAgent
	}
	fetcher, _ := webscrape.NewHTTPFetcher(fetcherCfg)

	// Create handlers
	githubHandler := handlers.NewGitHubHandler(&handlers.GitHubConfig{
		APIToken: githubToken,
	})

	gitlabHandler := handlers.NewGitLabHandler(&handlers.GitLabConfig{
		APIToken: gitlabToken,
	})

	stackoverHandler := handlers.NewStackOverflowHandler(&handlers.StackOverflowConfig{
		APIKey: soAPIKey,
	})

	// Create search engine
	var searchEngine webscrape.SearchEngine
	if cfg != nil && cfg.WebScraping.SearXNGURL != "" {
		searchEngine = webscrape.NewSearXNGSearch(cfg.WebScraping.SearXNGURL)
	} else {
		searchEngine = webscrape.NewDuckDuckGoSearch()
	}

	return &WebScrapeTool{
		config:           cfg,
		fetcher:          fetcher,
		githubHandler:    githubHandler,
		gitlabHandler:    gitlabHandler,
		stackoverHandler: stackoverHandler,
		searchEngine:     searchEngine,
	}
}

// Name returns the tool name
func (w *WebScrapeTool) Name() string {
	return "web_scrape"
}

// Description returns the tool description
func (w *WebScrapeTool) Description() string {
	return `Web scraping and code fetching tool. Actions:
- fetch_url: Fetch content from a URL, extract clean text and code blocks
- search_web: Search the web for information
- fetch_github_file: Fetch a file from a GitHub repository
- fetch_github_readme: Fetch README from a GitHub repository
- fetch_github_directory: List files in a GitHub repository directory
- search_github_code: Search for code on GitHub
- fetch_gitlab_file: Fetch a file from a GitLab repository
- fetch_gitlab_readme: Fetch README from a GitLab repository
- fetch_stackoverflow_question: Fetch a Stack Overflow question with answers
- search_stackoverflow: Search Stack Overflow for questions
- extract_code_from_url: Extract all code blocks from a webpage`
}

// Execute executes the web scraping tool
func (w *WebScrapeTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "fetch_url":
		return w.fetchURL(ctx, args)
	case "search_web":
		return w.searchWeb(ctx, args)
	case "fetch_github_file":
		return w.fetchGitHubFile(ctx, args)
	case "fetch_github_readme":
		return w.fetchGitHubREADME(ctx, args)
	case "fetch_github_directory":
		return w.fetchGitHubDirectory(ctx, args)
	case "search_github_code":
		return w.searchGitHubCode(ctx, args)
	case "fetch_gitlab_file":
		return w.fetchGitLabFile(ctx, args)
	case "fetch_gitlab_readme":
		return w.fetchGitLabREADME(ctx, args)
	case "fetch_stackoverflow_question":
		return w.fetchSOQuestion(ctx, args)
	case "search_stackoverflow":
		return w.searchStackOverflow(ctx, args)
	case "extract_code_from_url":
		return w.extractCodeFromURL(ctx, args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// fetchURL fetches content from a URL
func (w *WebScrapeTool) fetchURL(ctx context.Context, args map[string]interface{}) (*Result, error) {
	url, ok := args["url"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'url' parameter"}, nil
	}

	opts := webscrape.DefaultFetchOptions()

	if extractText, ok := args["extract_text"].(bool); ok {
		opts.ExtractContent = extractText
	}
	if extractCode, ok := args["extract_code"].(bool); ok {
		opts.ExtractCode = extractCode
	}

	result, err := w.fetcher.Fetch(ctx, url, opts)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Format response for agent
	resp := webscrape.FormatForAgent(result)
	output, _ := json.MarshalIndent(resp, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// searchWeb performs a web search
func (w *WebScrapeTool) searchWeb(ctx context.Context, args map[string]interface{}) (*Result, error) {
	query, ok := args["query"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'query' parameter"}, nil
	}

	opts := webscrape.DefaultSearchOptions()

	if maxResults, ok := args["max_results"].(float64); ok {
		opts.MaxResults = int(maxResults)
	}
	if site, ok := args["site"].(string); ok {
		opts.Site = site
	}

	results, err := w.searchEngine.Search(ctx, query, opts)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// fetchGitHubFile fetches a file from GitHub
func (w *WebScrapeTool) fetchGitHubFile(ctx context.Context, args map[string]interface{}) (*Result, error) {
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	path, _ := args["path"].(string)
	ref, _ := args["ref"].(string)

	if owner == "" || repo == "" || path == "" {
		return &Result{Success: false, Error: "missing required parameters: owner, repo, path"}, nil
	}

	if ref == "" {
		ref = "main"
	}

	file, err := w.githubHandler.FetchFile(ctx, owner, repo, path, ref)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(file, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// fetchGitHubREADME fetches a README from GitHub
func (w *WebScrapeTool) fetchGitHubREADME(ctx context.Context, args map[string]interface{}) (*Result, error) {
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)

	if owner == "" || repo == "" {
		return &Result{Success: false, Error: "missing required parameters: owner, repo"}, nil
	}

	file, err := w.githubHandler.FetchREADME(ctx, owner, repo)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(file, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// fetchGitHubDirectory lists files in a GitHub directory
func (w *WebScrapeTool) fetchGitHubDirectory(ctx context.Context, args map[string]interface{}) (*Result, error) {
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	path, _ := args["path"].(string)
	ref, _ := args["ref"].(string)

	if owner == "" || repo == "" {
		return &Result{Success: false, Error: "missing required parameters: owner, repo"}, nil
	}

	if ref == "" {
		ref = "main"
	}

	entries, err := w.githubHandler.FetchDirectory(ctx, owner, repo, path, ref)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(entries, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// searchGitHubCode searches for code on GitHub
func (w *WebScrapeTool) searchGitHubCode(ctx context.Context, args map[string]interface{}) (*Result, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return &Result{Success: false, Error: "missing 'query' parameter"}, nil
	}

	opts := &webscrape.GitHubSearchOpts{}
	if language, ok := args["language"].(string); ok {
		opts.Language = language
	}
	if repo, ok := args["repo"].(string); ok {
		opts.Repo = repo
	}

	results, err := w.githubHandler.SearchCode(ctx, query, opts)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// fetchGitLabFile fetches a file from GitLab
func (w *WebScrapeTool) fetchGitLabFile(ctx context.Context, args map[string]interface{}) (*Result, error) {
	project, _ := args["project"].(string)
	path, _ := args["path"].(string)
	ref, _ := args["ref"].(string)

	if project == "" || path == "" {
		return &Result{Success: false, Error: "missing required parameters: project, path"}, nil
	}

	if ref == "" {
		ref = "main"
	}

	file, err := w.gitlabHandler.FetchFile(ctx, project, path, ref)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(file, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// fetchGitLabREADME fetches a README from GitLab
func (w *WebScrapeTool) fetchGitLabREADME(ctx context.Context, args map[string]interface{}) (*Result, error) {
	project, _ := args["project"].(string)

	if project == "" {
		return &Result{Success: false, Error: "missing 'project' parameter"}, nil
	}

	file, err := w.gitlabHandler.FetchREADME(ctx, project)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(file, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// fetchSOQuestion fetches a Stack Overflow question
func (w *WebScrapeTool) fetchSOQuestion(ctx context.Context, args map[string]interface{}) (*Result, error) {
	var questionID int

	// Accept question_id as either string or number
	switch v := args["question_id"].(type) {
	case float64:
		questionID = int(v)
	case string:
		var err error
		questionID, err = strconv.Atoi(v)
		if err != nil {
			return &Result{Success: false, Error: "invalid question_id format"}, nil
		}
	default:
		return &Result{Success: false, Error: "missing 'question_id' parameter"}, nil
	}

	includeAnswers := true
	if ia, ok := args["include_answers"].(bool); ok {
		includeAnswers = ia
	}

	maxAnswers := 5
	if ma, ok := args["max_answers"].(float64); ok {
		maxAnswers = int(ma)
	}

	var question *webscrape.SOQuestion
	var err error

	if includeAnswers {
		question, err = w.stackoverHandler.FetchQuestionWithAnswers(ctx, questionID, maxAnswers)
	} else {
		question, err = w.stackoverHandler.FetchQuestion(ctx, questionID)
	}

	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(question, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// searchStackOverflow searches Stack Overflow
func (w *WebScrapeTool) searchStackOverflow(ctx context.Context, args map[string]interface{}) (*Result, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return &Result{Success: false, Error: "missing 'query' parameter"}, nil
	}

	var tags []string
	if tagsStr, ok := args["tags"].(string); ok && tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i, t := range tags {
			tags[i] = strings.TrimSpace(t)
		}
	}

	sort := "relevance"
	if s, ok := args["sort"].(string); ok {
		sort = s
	}

	maxResults := 10
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	results, err := w.stackoverHandler.SearchQuestions(ctx, query, tags, sort, maxResults)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// extractCodeFromURL extracts code blocks from a URL
func (w *WebScrapeTool) extractCodeFromURL(ctx context.Context, args map[string]interface{}) (*Result, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return &Result{Success: false, Error: "missing 'url' parameter"}, nil
	}

	language, _ := args["language"].(string)

	opts := webscrape.DefaultFetchOptions()
	opts.ExtractCode = true

	result, err := w.fetcher.Fetch(ctx, url, opts)
	if err != nil {
		if scrapeErr, ok := err.(*webscrape.ScrapeError); ok {
			return &Result{Success: false, Error: scrapeErr.ForAgent()}, nil
		}
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Filter by language if specified
	var codeBlocks []webscrape.CodeBlock
	if language != "" {
		for _, block := range result.CodeBlocks {
			if strings.EqualFold(block.Language, language) {
				codeBlocks = append(codeBlocks, block)
			}
		}
	} else {
		codeBlocks = result.CodeBlocks
	}

	if len(codeBlocks) == 0 {
		return &Result{
			Success: true,
			Output:  "No code blocks found",
		}, nil
	}

	output, _ := json.MarshalIndent(codeBlocks, "", "  ")

	return &Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// Close cleans up resources
func (w *WebScrapeTool) Close() error {
	if w.fetcher != nil {
		return w.fetcher.Close()
	}
	return nil
}

// Metadata returns rich tool metadata for discovery and LLM guidance
func (w *WebScrapeTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type: "string",
					Enum: []interface{}{
						"fetch_url", "search_web", "fetch_github_file", "fetch_github_readme",
						"fetch_github_directory", "search_github_code", "fetch_gitlab_file",
						"fetch_gitlab_readme", "fetch_stackoverflow_question", "search_stackoverflow",
						"extract_code_from_url",
					},
					Description: "The web scraping action to perform",
				},
				"url": {
					Type:        "string",
					Description: "URL to fetch (for fetch_url, extract_code_from_url)",
				},
				"query": {
					Type:        "string",
					Description: "Search query (for search_web, search_github_code, search_stackoverflow)",
				},
				"owner": {
					Type:        "string",
					Description: "GitHub repository owner",
				},
				"repo": {
					Type:        "string",
					Description: "GitHub repository name",
				},
				"path": {
					Type:        "string",
					Description: "File or directory path within repository",
				},
				"ref": {
					Type:        "string",
					Description: "Git ref (branch/tag/commit), defaults to main",
				},
				"project": {
					Type:        "string",
					Description: "GitLab project path (e.g., 'group/project')",
				},
				"question_id": {
					Type:        "string",
					Description: "Stack Overflow question ID",
				},
				"language": {
					Type:        "string",
					Description: "Programming language filter",
				},
				"max_results": {
					Type:        "number",
					Description: "Maximum number of results to return",
				},
			},
			Required: []string{"action"},
		},
		Category:             CategoryResearch,
		Optionality:          ToolOptional,
		UsageHint:            "Use to fetch code examples, documentation, or search for solutions from GitHub, GitLab, Stack Overflow, or general web.",
		RequiresCapabilities: []string{"network"},
		Examples: []ToolExample{
			{
				Description: "Fetch a file from GitHub",
				Input: map[string]interface{}{
					"action": "fetch_github_file",
					"owner":  "anthropics",
					"repo":   "claude-code",
					"path":   "README.md",
				},
			},
			{
				Description: "Search Stack Overflow",
				Input: map[string]interface{}{
					"action": "search_stackoverflow",
					"query":  "golang context timeout",
				},
			},
			{
				Description: "Search web for documentation",
				Input: map[string]interface{}{
					"action": "search_web",
					"query":  "Go http client best practices",
				},
			},
		},
		Produces: []string{"code_examples", "documentation", "web_content"},
	}
}
