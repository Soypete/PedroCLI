package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/soypete/pedrocli/pkg/config"
)

// TokenManager defines the interface for retrieving tokens
// IMPORTANT: Tokens retrieved from this manager are NEVER exposed to the LLM
// They are only used internally by tools for API authentication
type TokenManager interface {
	GetToken(ctx context.Context, provider, service string) (accessToken string, err error)
}

// NotionTool provides access to Notion via MCP server
type NotionTool struct {
	config       *config.Config
	tokenManager TokenManager
	mu           sync.Mutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       *bufio.Reader
	msgID        int
	started      bool
}

// NewNotionTool creates a new Notion tool
func NewNotionTool(cfg *config.Config, tokenMgr TokenManager) *NotionTool {
	return &NotionTool{
		config:       cfg,
		tokenManager: tokenMgr,
		msgID:        0,
	}
}

// Name returns the tool name
func (t *NotionTool) Name() string {
	return "notion"
}

// Description returns the tool description
func (t *NotionTool) Description() string {
	return `Notion database and page management via MCP server.

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

	// Check for API key
	if t.config.Podcast.Notion.APIKey == "" {
		return &Result{
			Success: false,
			Error:   "Notion API key not configured. Set podcast.notion.api_key in config. TODO: Add your Notion API key.",
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

// ensureStarted starts the MCP server if not already running
func (t *NotionTool) ensureStarted(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return nil
	}

	// Get API key from TokenManager (NEVER exposed to LLM)
	var apiKey string
	var err error
	if t.tokenManager != nil {
		apiKey, err = t.tokenManager.GetToken(ctx, "notion", "database")
		if err != nil {
			return fmt.Errorf("failed to retrieve Notion API key: %w", err)
		}
	} else {
		// Fallback to config (for backwards compatibility)
		apiKey = t.config.Podcast.Notion.APIKey
		if apiKey == "" {
			return fmt.Errorf("Notion API key not configured")
		}
	}

	// Parse command and args
	cmdParts := strings.Fields(t.config.Podcast.Notion.Command)
	if len(cmdParts) == 0 {
		return fmt.Errorf("no Notion MCP command configured")
	}

	// Set up environment with API key (used only for subprocess authentication, never logged or exposed)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("NOTION_API_KEY=%s", apiKey))

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Notion MCP server: %w", err)
	}

	t.cmd = cmd
	t.stdin = stdin
	t.stdout = bufio.NewReader(stdout)
	t.started = true

	// Initialize the MCP server
	if err := t.initialize(ctx); err != nil {
		t.stop()
		return fmt.Errorf("failed to initialize Notion MCP server: %w", err)
	}

	return nil
}

// stop stops the MCP server
func (t *NotionTool) stop() {
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	t.started = false
}

// initialize sends the initialize request to the MCP server
func (t *NotionTool) initialize(ctx context.Context) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      t.nextID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "pedrocli",
				"version": "1.0.0",
			},
		},
	}

	_, err := t.sendRequest(ctx, req)
	return err
}

// sendRequest sends a JSON-RPC request and waits for response
func (t *NotionTool) sendRequest(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	// Marshal request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	line, err := t.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for error
	if errObj, ok := resp["error"]; ok {
		return nil, fmt.Errorf("MCP error: %v", errObj)
	}

	return resp, nil
}

// nextID returns the next message ID
func (t *NotionTool) nextID() int {
	t.msgID++
	return t.msgID
}

// callTool calls an MCP tool
func (t *NotionTool) callTool(ctx context.Context, toolName string, toolArgs map[string]interface{}) (*Result, error) {
	if err := t.ensureStarted(ctx); err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      t.nextID(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": toolArgs,
		},
	}

	resp, err := t.sendRequest(ctx, req)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Extract result
	result, ok := resp["result"]
	if !ok {
		return &Result{
			Success: false,
			Error:   "no result in response",
		}, nil
	}

	// Format result as JSON for output
	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
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

	toolArgs := map[string]interface{}{
		"database_id": databaseID,
	}

	if filter, ok := args["filter"]; ok {
		toolArgs["filter"] = filter
	}
	if sorts, ok := args["sorts"]; ok {
		toolArgs["sorts"] = sorts
	}

	return t.callTool(ctx, "notion_query_database", toolArgs)
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

	toolArgs := map[string]interface{}{
		"database_id": databaseID,
		"properties":  properties,
	}

	if content, ok := args["content"].(string); ok {
		toolArgs["content"] = content
	}

	return t.callTool(ctx, "notion_create_page", toolArgs)
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

	toolArgs := map[string]interface{}{
		"page_id":    pageID,
		"properties": properties,
	}

	return t.callTool(ctx, "notion_update_page", toolArgs)
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

	toolArgs := map[string]interface{}{
		"page_id": pageID,
	}

	return t.callTool(ctx, "notion_get_page", toolArgs)
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

	toolArgs := map[string]interface{}{
		"page_id": pageID,
		"content": content,
	}

	return t.callTool(ctx, "notion_append_blocks", toolArgs)
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

	toolArgs := map[string]interface{}{
		"query": query,
	}

	return t.callTool(ctx, "notion_search", toolArgs)
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
