package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Notion API client for blog workflow
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// Config holds Notion client configuration
type Config struct {
	APIKey string

	// Database IDs for different content types
	// TODO: User should provide these in .pedrocli.json
	BlogDraftsDB     string
	PublishedPostsDB string
	AssetDB          string
	IdeasDB          string
}

// NewClient creates a new Notion client
func NewClient(cfg *Config) *Client {
	return &Client{
		apiKey: cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.notion.com/v1",
	}
}

// Page represents a Notion page
type Page struct {
	ID          string                 `json:"id"`
	CreatedTime string                 `json:"created_time"`
	Properties  map[string]interface{} `json:"properties"`
	Children    []Block                `json:"children,omitempty"`
}

// Block represents a Notion block (content)
type Block struct {
	Type    string                 `json:"type"`
	Object  string                 `json:"object"`
	Content map[string]interface{} `json:"content"`
}

// CreateDraftPost creates a new draft post in Notion
func (c *Client) CreateDraftPost(title, content, status string) (string, error) {
	// TODO: Implement Notion API call to create page
	// This is a placeholder - implement when Notion database ID is available

	// Build page properties
	properties := map[string]interface{}{
		"Name": map[string]interface{}{
			"title": []map[string]interface{}{
				{
					"text": map[string]interface{}{
						"content": title,
					},
				},
			},
		},
		"Status": map[string]interface{}{
			"select": map[string]interface{}{
				"name": status,
			},
		},
	}

	// Build page content blocks from markdown
	blocks := c.markdownToBlocks(content)

	// Create page payload
	payload := map[string]interface{}{
		"parent": map[string]interface{}{
			"database_id": "TODO_BLOG_DRAFTS_DB_ID", // TODO: Get from config
		},
		"properties": properties,
		"children":   blocks,
	}

	pageID, err := c.createPage(payload)
	if err != nil {
		return "", fmt.Errorf("failed to create Notion page: %w", err)
	}

	return pageID, nil
}

// UpdatePost updates an existing Notion page
func (c *Client) UpdatePost(pageID, content string) error {
	// TODO: Implement Notion API call to update page content
	// This is a placeholder
	return fmt.Errorf("UpdatePost not yet implemented - TODO: Add Notion API integration")
}

// GetPost retrieves a post from Notion
func (c *Client) GetPost(pageID string) (*Page, error) {
	// TODO: Implement Notion API call to get page
	// This is a placeholder
	return nil, fmt.Errorf("GetPost not yet implemented - TODO: Add Notion API integration")
}

// UpdateStatus updates the status of a post
func (c *Client) UpdateStatus(pageID, status string) error {
	// TODO: Implement Notion API call to update status property
	// This is a placeholder
	return fmt.Errorf("UpdateStatus not yet implemented - TODO: Add Notion API integration")
}

// createPage creates a new page in Notion
func (c *Client) createPage(payload map[string]interface{}) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/pages", bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("notion API error: %s - %s", resp.Status, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	pageID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("no page ID in response")
	}

	return pageID, nil
}

// markdownToBlocks converts markdown content to Notion blocks
// This is a simplified version - TODO: Improve markdown parsing
func (c *Client) markdownToBlocks(markdown string) []map[string]interface{} {
	// Split markdown into paragraphs
	// This is a very basic implementation - a real one would parse headings,
	// lists, code blocks, etc.

	blocks := []map[string]interface{}{
		{
			"object": "block",
			"type":   "paragraph",
			"paragraph": map[string]interface{}{
				"rich_text": []map[string]interface{}{
					{
						"type": "text",
						"text": map[string]interface{}{
							"content": markdown,
						},
					},
				},
			},
		},
	}

	return blocks
}
