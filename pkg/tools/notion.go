package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
)

const notionAPIBase = "https://api.notion.com/v1"
const notionAPIVersion = "2022-06-28"

// TokenManager defines the interface for retrieving tokens
// IMPORTANT: Tokens retrieved from this manager are NEVER exposed to the LLM
// They are only used internally by tools for API authentication
type TokenManager interface {
	GetToken(ctx context.Context, provider, service string) (accessToken string, err error)
}

// NotionTool provides access to Notion via REST API
type NotionTool struct {
	config       *config.Config
	tokenManager TokenManager
	httpClient   *http.Client
}

// NewNotionTool creates a new Notion tool
func NewNotionTool(cfg *config.Config, tokenMgr TokenManager) *NotionTool {
	return &NotionTool{
		config:       cfg,
		tokenManager: tokenMgr,
		httpClient:   &http.Client{},
	}
}

// Name returns the tool name
func (t *NotionTool) Name() string {
	return "notion"
}

// Description returns the tool description
func (t *NotionTool) Description() string {
	return `Notion database and page management via REST API.

Actions:
- query_database: Query a Notion database
  Args: database_id (string), filter (optional object), sorts (optional array)

- create_page: Create a new page in a database
  Args: database_id (string), properties (object), content (optional string)

- update_page: Update an existing page
  Args: page_id (string), properties (object)

- get_page: Get a page by ID
  Args: page_id (string)

- append_blocks: Append content blocks to a page
  Args: page_id (string), content (string)

- search: Search across all accessible pages
  Args: query (string)

Example:
{"tool": "notion", "args": {"action": "query_database", "database_id": "abc123", "filter": {"property": "Status", "status": {"equals": "To Review"}}}}`
}

// Execute executes a Notion action
func (t *NotionTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Check if Notion is enabled
	if !t.config.Podcast.Notion.Enabled {
		return &Result{
			Success: false,
			Error:   "Notion integration is not enabled. Set podcast.notion.enabled=true in config.",
		}, nil
	}

	action, ok := args["action"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "action is required",
		}, nil
	}

	switch action {
	case "query_database":
		return t.queryDatabase(ctx, args)
	case "create_page":
		return t.createPage(ctx, args)
	case "update_page":
		return t.updatePage(ctx, args)
	case "get_page":
		return t.getPage(ctx, args)
	case "append_blocks":
		return t.appendBlocks(ctx, args)
	case "search":
		return t.search(ctx, args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// getAPIKey retrieves the Notion API key
func (t *NotionTool) getAPIKey(ctx context.Context) (string, error) {
	// Try TokenManager first
	if t.tokenManager != nil {
		apiKey, err := t.tokenManager.GetToken(ctx, "notion", "database")
		if err == nil && apiKey != "" {
			return apiKey, nil
		}
	}

	// Fall back to config
	if t.config.Podcast.Notion.APIKey != "" {
		return t.config.Podcast.Notion.APIKey, nil
	}

	return "", fmt.Errorf("Notion API key not configured")
}

// makeRequest makes an HTTP request to the Notion API
func (t *NotionTool) makeRequest(ctx context.Context, method, path string, body interface{}) (map[string]interface{}, error) {
	apiKey, err := t.getAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	url := notionAPIBase + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Notion-Version", notionAPIVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Notion API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// queryDatabase queries a Notion database
func (t *NotionTool) queryDatabase(ctx context.Context, args map[string]interface{}) (*Result, error) {
	databaseID, ok := args["database_id"].(string)
	if !ok {
		// Try to get from config based on database name
		dbName, _ := args["database_name"].(string)
		databaseID = t.getDatabaseID(dbName)
		if databaseID == "" {
			return &Result{
				Success: false,
				Error:   "database_id or database_name is required",
			}, nil
		}
	}

	// Remove hyphens from database ID if present
	databaseID = strings.ReplaceAll(databaseID, "-", "")

	reqBody := make(map[string]interface{})
	if filter, ok := args["filter"]; ok {
		reqBody["filter"] = filter
	}
	if sorts, ok := args["sorts"]; ok {
		reqBody["sorts"] = sorts
	}

	result, err := t.makeRequest(ctx, "POST", "/databases/"+databaseID+"/query", reqBody)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
}

// createPage creates a new page in a database
func (t *NotionTool) createPage(ctx context.Context, args map[string]interface{}) (*Result, error) {
	databaseID, ok := args["database_id"].(string)
	if !ok {
		dbName, _ := args["database_name"].(string)
		databaseID = t.getDatabaseID(dbName)
		if databaseID == "" {
			return &Result{
				Success: false,
				Error:   "database_id or database_name is required",
			}, nil
		}
	}

	properties, ok := args["properties"].(map[string]interface{})
	if !ok {
		return &Result{
			Success: false,
			Error:   "properties is required",
		}, nil
	}

	// Remove hyphens from database ID
	databaseID = strings.ReplaceAll(databaseID, "-", "")

	// Convert properties to Notion format
	notionProps := make(map[string]interface{})
	for key, value := range properties {
		notionProps[key] = t.convertToNotionProperty(key, value)
	}

	reqBody := map[string]interface{}{
		"parent": map[string]interface{}{
			"database_id": databaseID,
		},
		"properties": notionProps,
	}

	// Add content/children if provided
	if content, ok := args["content"].(string); ok && content != "" {
		reqBody["children"] = []interface{}{
			map[string]interface{}{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": map[string]interface{}{
								"content": content,
							},
						},
					},
				},
			},
		}
	}

	result, err := t.makeRequest(ctx, "POST", "/pages", reqBody)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Extract page URL for easy access
	pageURL, _ := result["url"].(string)
	pageID, _ := result["id"].(string)

	// Create a summary message with the URL prominently displayed
	outputMsg := fmt.Sprintf("✓ Page created successfully!\n\nPage URL: %s\nPage ID: %s\n\n", pageURL, pageID)
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	outputMsg += "Full response:\n" + string(resultJSON)

	return &Result{
		Success: true,
		Output:  outputMsg,
	}, nil
}

// convertToNotionProperty converts a simple value to Notion property format
// propertyName is used to determine if this is a title property (only one per database)
func (t *NotionTool) convertToNotionProperty(propertyName string, value interface{}) map[string]interface{} {
	switch v := value.(type) {
	case string:
		// Check if this is likely a title property
		// Title properties typically have names like "Name", "Title", "Episode #", etc.
		if strings.Contains(strings.ToLower(propertyName), "#") ||
			strings.ToLower(propertyName) == "name" ||
			strings.ToLower(propertyName) == "title" {
			return map[string]interface{}{
				"title": []interface{}{
					map[string]interface{}{
						"text": map[string]interface{}{
							"content": v,
						},
					},
				},
			}
		}
		// Otherwise, use rich_text for string properties
		return map[string]interface{}{
			"rich_text": []interface{}{
				map[string]interface{}{
					"text": map[string]interface{}{
						"content": v,
					},
				},
			},
		}
	case float64, int:
		return map[string]interface{}{
			"number": v,
		}
	case bool:
		return map[string]interface{}{
			"checkbox": v,
		}
	default:
		// Return as-is if already in Notion format
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
		return map[string]interface{}{}
	}
}

// updatePage updates an existing page
func (t *NotionTool) updatePage(ctx context.Context, args map[string]interface{}) (*Result, error) {
	pageID, ok := args["page_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "page_id is required",
		}, nil
	}

	properties, ok := args["properties"].(map[string]interface{})
	if !ok {
		return &Result{
			Success: false,
			Error:   "properties is required",
		}, nil
	}

	// Remove hyphens
	pageID = strings.ReplaceAll(pageID, "-", "")

	// Convert properties
	notionProps := make(map[string]interface{})
	for key, value := range properties {
		notionProps[key] = t.convertToNotionProperty(key, value)
	}

	reqBody := map[string]interface{}{
		"properties": notionProps,
	}

	result, err := t.makeRequest(ctx, "PATCH", "/pages/"+pageID, reqBody)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
}

// getPage gets a page by ID
func (t *NotionTool) getPage(ctx context.Context, args map[string]interface{}) (*Result, error) {
	pageID, ok := args["page_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "page_id is required",
		}, nil
	}

	// Remove hyphens
	pageID = strings.ReplaceAll(pageID, "-", "")

	result, err := t.makeRequest(ctx, "GET", "/pages/"+pageID, nil)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
}

// appendBlocks appends content blocks to a page
func (t *NotionTool) appendBlocks(ctx context.Context, args map[string]interface{}) (*Result, error) {
	pageID, ok := args["page_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "page_id is required",
		}, nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "content is required",
		}, nil
	}

	// Remove hyphens
	pageID = strings.ReplaceAll(pageID, "-", "")

	reqBody := map[string]interface{}{
		"children": []interface{}{
			map[string]interface{}{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": map[string]interface{}{
								"content": content,
							},
						},
					},
				},
			},
		},
	}

	result, err := t.makeRequest(ctx, "PATCH", "/blocks/"+pageID+"/children", reqBody)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
}

// search searches across all accessible pages
func (t *NotionTool) search(ctx context.Context, args map[string]interface{}) (*Result, error) {
	query, ok := args["query"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "query is required",
		}, nil
	}

	reqBody := map[string]interface{}{
		"query": query,
	}

	result, err := t.makeRequest(ctx, "POST", "/search", reqBody)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
}

// getDatabaseID returns the database ID for a given name from config
func (t *NotionTool) getDatabaseID(name string) string {
	dbs := t.config.Podcast.Notion.Databases
	switch strings.ToLower(name) {
	case "scripts":
		return dbs.Scripts
	case "articles", "articles_review":
		return dbs.ArticlesReview
	case "potential_articles":
		return dbs.PotentialArticle
	case "news", "news_review":
		return dbs.NewsReview
	case "guests":
		return dbs.Guests
	default:
		return ""
	}
}
