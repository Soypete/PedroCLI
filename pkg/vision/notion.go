package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NotionClient provides integration with the Notion API.
type NotionClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NotionConfig configures the Notion client.
type NotionConfig struct {
	APIKey string `json:"api_key"`
	// TODO: Add Notion database IDs for different content types
	BlogDatabaseID string `json:"blog_database_id"`
}

// NewNotionClient creates a new Notion client.
func NewNotionClient(cfg *NotionConfig) *NotionClient {
	return &NotionClient{
		apiKey:  cfg.APIKey,
		baseURL: "https://api.notion.com/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// BlogPost represents a blog post from Notion.
type BlogPost struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	CoverImage  string    `json:"cover_image,omitempty"`
	URL         string    `json:"url,omitempty"`
}

// NotionImage represents an image in Notion.
type NotionImage struct {
	ID          string            `json:"id"`
	URL         string            `json:"url"`
	Caption     string            `json:"caption"`
	AltText     string            `json:"alt_text"`
	Type        string            `json:"type"` // "external" or "file"
	Metadata    map[string]string `json:"metadata"`
	IsGenerated bool              `json:"is_generated"`
}

// GetBlogPost retrieves a blog post from Notion by page ID.
func (c *NotionClient) GetBlogPost(ctx context.Context, pageID string) (*BlogPost, error) {
	// Get page metadata
	pageURL := fmt.Sprintf("%s/pages/%s", c.baseURL, pageID)
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("notion API error: %s - %s", resp.Status, string(body))
	}

	var pageResp notionPageResponse
	if err := json.NewDecoder(resp.Body).Decode(&pageResp); err != nil {
		return nil, err
	}

	// Get page content (blocks)
	content, err := c.getPageContent(ctx, pageID)
	if err != nil {
		return nil, err
	}

	return &BlogPost{
		ID:      pageID,
		Title:   c.extractTitle(pageResp),
		Content: content,
		URL:     pageResp.URL,
	}, nil
}

// getPageContent retrieves the content of a Notion page.
func (c *NotionClient) getPageContent(ctx context.Context, pageID string) (string, error) {
	url := fmt.Sprintf("%s/blocks/%s/children", c.baseURL, pageID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get blocks: %s", resp.Status)
	}

	var blocksResp notionBlocksResponse
	if err := json.NewDecoder(resp.Body).Decode(&blocksResp); err != nil {
		return "", err
	}

	// Extract text content from blocks
	var content strings.Builder
	for _, block := range blocksResp.Results {
		text := c.extractBlockText(block)
		if text != "" {
			content.WriteString(text)
			content.WriteString("\n\n")
		}
	}

	return content.String(), nil
}

// ListBlogPosts lists blog posts from a database.
func (c *NotionClient) ListBlogPosts(ctx context.Context, databaseID string) ([]*BlogPost, error) {
	url := fmt.Sprintf("%s/databases/%s/query", c.baseURL, databaseID)

	body := map[string]interface{}{
		"page_size": 100,
		"sorts": []map[string]string{
			{"property": "Created", "direction": "descending"},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("notion API error: %s - %s", resp.Status, string(respBody))
	}

	var queryResp notionQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, err
	}

	var posts []*BlogPost
	for _, result := range queryResp.Results {
		posts = append(posts, &BlogPost{
			ID:    result.ID,
			Title: c.extractTitleFromProperties(result.Properties),
			URL:   result.URL,
		})
	}

	return posts, nil
}

// UploadImage uploads an image to Notion as an external link.
// Notion doesn't support direct file uploads via API, so we use external URLs.
func (c *NotionClient) UploadImage(ctx context.Context, pageID string, imageURL string, caption string, altText string) error {
	url := fmt.Sprintf("%s/blocks/%s/children", c.baseURL, pageID)

	// Create image block
	block := map[string]interface{}{
		"object": "block",
		"type":   "image",
		"image": map[string]interface{}{
			"type": "external",
			"external": map[string]string{
				"url": imageURL,
			},
			"caption": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]string{
						"content": caption,
					},
					"annotations": map[string]bool{
						"bold":          false,
						"italic":        false,
						"strikethrough": false,
						"underline":     false,
						"code":          false,
					},
					"plain_text": caption,
				},
			},
		},
	}

	body := map[string]interface{}{
		"children": []interface{}{block},
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add image block: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// UpdateCoverImage updates the cover image of a page.
func (c *NotionClient) UpdateCoverImage(ctx context.Context, pageID string, imageURL string) error {
	url := fmt.Sprintf("%s/pages/%s", c.baseURL, pageID)

	body := map[string]interface{}{
		"cover": map[string]interface{}{
			"type": "external",
			"external": map[string]string{
				"url": imageURL,
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update cover: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// GetPageImages retrieves all images from a page.
func (c *NotionClient) GetPageImages(ctx context.Context, pageID string) ([]*NotionImage, error) {
	url := fmt.Sprintf("%s/blocks/%s/children?page_size=100", c.baseURL, pageID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var blocksResp notionBlocksResponse
	if err := json.NewDecoder(resp.Body).Decode(&blocksResp); err != nil {
		return nil, err
	}

	var images []*NotionImage
	for _, block := range blocksResp.Results {
		if block.Type == "image" {
			img := c.extractImage(block)
			if img != nil {
				images = append(images, img)
			}
		}
	}

	return images, nil
}

// Helper methods

func (c *NotionClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Notion-Version", "2022-06-28")
}

func (c *NotionClient) extractTitle(page notionPageResponse) string {
	if page.Properties == nil {
		return ""
	}

	// Try common title property names
	for _, name := range []string{"title", "Title", "Name", "name"} {
		if prop, ok := page.Properties[name]; ok {
			if titleArr, ok := prop.([]interface{}); ok && len(titleArr) > 0 {
				if titleObj, ok := titleArr[0].(map[string]interface{}); ok {
					if plainText, ok := titleObj["plain_text"].(string); ok {
						return plainText
					}
				}
			}
		}
	}

	return ""
}

func (c *NotionClient) extractTitleFromProperties(props map[string]interface{}) string {
	for _, name := range []string{"title", "Title", "Name", "name"} {
		if prop, ok := props[name].(map[string]interface{}); ok {
			if titleArr, ok := prop["title"].([]interface{}); ok && len(titleArr) > 0 {
				if titleObj, ok := titleArr[0].(map[string]interface{}); ok {
					if plainText, ok := titleObj["plain_text"].(string); ok {
						return plainText
					}
				}
			}
		}
	}
	return ""
}

func (c *NotionClient) extractBlockText(block notionBlock) string {
	var richText []interface{}

	switch block.Type {
	case "paragraph":
		if p, ok := block.Paragraph["rich_text"].([]interface{}); ok {
			richText = p
		}
	case "heading_1", "heading_2", "heading_3":
		if h, ok := block.Heading["rich_text"].([]interface{}); ok {
			richText = h
		}
	case "bulleted_list_item", "numbered_list_item":
		if l, ok := block.ListItem["rich_text"].([]interface{}); ok {
			richText = l
		}
	case "quote":
		if q, ok := block.Quote["rich_text"].([]interface{}); ok {
			richText = q
		}
	case "code":
		if c, ok := block.Code["rich_text"].([]interface{}); ok {
			richText = c
		}
	}

	var text strings.Builder
	for _, item := range richText {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if plainText, ok := itemMap["plain_text"].(string); ok {
				text.WriteString(plainText)
			}
		}
	}

	return text.String()
}

func (c *NotionClient) extractImage(block notionBlock) *NotionImage {
	if block.Image == nil {
		return nil
	}

	img := &NotionImage{
		ID:   block.ID,
		Type: block.Image["type"].(string),
	}

	// Get URL based on type
	if img.Type == "external" {
		if ext, ok := block.Image["external"].(map[string]interface{}); ok {
			img.URL, _ = ext["url"].(string)
		}
	} else if img.Type == "file" {
		if file, ok := block.Image["file"].(map[string]interface{}); ok {
			img.URL, _ = file["url"].(string)
		}
	}

	// Get caption
	if captionArr, ok := block.Image["caption"].([]interface{}); ok && len(captionArr) > 0 {
		if captionObj, ok := captionArr[0].(map[string]interface{}); ok {
			img.Caption, _ = captionObj["plain_text"].(string)
		}
	}

	return img
}

// Notion API response types
type notionPageResponse struct {
	ID         string                 `json:"id"`
	URL        string                 `json:"url"`
	Properties map[string]interface{} `json:"properties"`
	Cover      map[string]interface{} `json:"cover"`
}

type notionBlocksResponse struct {
	Results []notionBlock `json:"results"`
	HasMore bool          `json:"has_more"`
}

type notionBlock struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Paragraph map[string]interface{} `json:"paragraph,omitempty"`
	Heading   map[string]interface{} `json:"heading_1,omitempty"`
	ListItem  map[string]interface{} `json:"bulleted_list_item,omitempty"`
	Quote     map[string]interface{} `json:"quote,omitempty"`
	Code      map[string]interface{} `json:"code,omitempty"`
	Image     map[string]interface{} `json:"image,omitempty"`
}

type notionQueryResponse struct {
	Results []struct {
		ID         string                 `json:"id"`
		URL        string                 `json:"url"`
		Properties map[string]interface{} `json:"properties"`
	} `json:"results"`
	HasMore bool `json:"has_more"`
}
