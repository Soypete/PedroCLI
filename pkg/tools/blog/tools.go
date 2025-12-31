package blog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	blogstorage "github.com/soypete/pedrocli/pkg/storage/blog"
	"github.com/soypete/pedrocli/pkg/tools"
)

// DictationTool handles creating blog posts from dictation
type DictationTool struct {
	manager *BlogToolsManager
}

// NewDictationTool creates a new dictation tool
func NewDictationTool(manager *BlogToolsManager) *DictationTool {
	return &DictationTool{manager: manager}
}

// Name returns the tool name
func (t *DictationTool) Name() string {
	return "blog_dictation"
}

// Description returns the tool description
func (t *DictationTool) Description() string {
	return "Create a new blog post draft from raw dictation transcription"
}

// Execute executes the dictation tool
func (t *DictationTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	transcription, ok := args["transcription"].(string)
	if !ok {
		return &tools.Result{Success: false, Error: "missing 'transcription' argument"}, nil
	}

	title, ok := args["title"].(string)
	if !ok || title == "" {
		title = "Untitled Post"
	}

	post, err := t.manager.CreateDraftFromDictation(ctx, transcription, title)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("failed to create draft: %v", err)}, nil
	}

	// Include structured data in output as JSON
	data := map[string]interface{}{
		"post_id": post.ID.String(),
		"title":   post.Title,
		"status":  string(post.Status),
	}
	dataJSON, _ := json.Marshal(data)

	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Created blog post draft: %s (ID: %s)\n%s", post.Title, post.ID, string(dataJSON)),
	}, nil
}

// ProcessWriterTool handles processing posts with the writer agent
type ProcessWriterTool struct {
	manager *BlogToolsManager
}

// NewProcessWriterTool creates a new process writer tool
func NewProcessWriterTool(manager *BlogToolsManager) *ProcessWriterTool {
	return &ProcessWriterTool{manager: manager}
}

// Name returns the tool name
func (t *ProcessWriterTool) Name() string {
	return "blog_process_writer"
}

// Description returns the tool description
func (t *ProcessWriterTool) Description() string {
	return "Process a dictated blog post with the writer agent to create a polished draft"
}

// Execute executes the process writer tool
func (t *ProcessWriterTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	postIDStr, ok := args["post_id"].(string)
	if !ok {
		return &tools.Result{Success: false, Error: "missing 'post_id' argument"}, nil
	}

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("invalid post_id: %v", err)}, nil
	}

	job, err := t.manager.ProcessWithWriter(ctx, postID)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("failed to process with writer: %v", err)}, nil
	}

	// Include structured data in output as JSON
	data := map[string]interface{}{
		"job_id":  job.ID,
		"post_id": postIDStr,
		"status":  string(job.Status),
	}
	dataJSON, _ := json.Marshal(data)

	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Writer agent job started: %s\n%s", job.ID, string(dataJSON)),
	}, nil
}

// ProcessEditorTool handles processing posts with the editor agent
type ProcessEditorTool struct {
	manager *BlogToolsManager
}

// NewProcessEditorTool creates a new process editor tool
func NewProcessEditorTool(manager *BlogToolsManager) *ProcessEditorTool {
	return &ProcessEditorTool{manager: manager}
}

// Name returns the tool name
func (t *ProcessEditorTool) Name() string {
	return "blog_process_editor"
}

// Description returns the tool description
func (t *ProcessEditorTool) Description() string {
	return "Process a drafted blog post with the editor agent for review or revision"
}

// Execute executes the process editor tool
func (t *ProcessEditorTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	postIDStr, ok := args["post_id"].(string)
	if !ok {
		return &tools.Result{Success: false, Error: "missing 'post_id' argument"}, nil
	}

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("invalid post_id: %v", err)}, nil
	}

	autoRevise := false
	if ar, ok := args["auto_revise"].(bool); ok {
		autoRevise = ar
	}

	job, err := t.manager.ProcessWithEditor(ctx, postID, autoRevise)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("failed to process with editor: %v", err)}, nil
	}

	// Include structured data in output as JSON
	data := map[string]interface{}{
		"job_id":      job.ID,
		"post_id":     postIDStr,
		"status":      string(job.Status),
		"auto_revise": autoRevise,
	}
	dataJSON, _ := json.Marshal(data)

	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Editor agent job started: %s\n%s", job.ID, string(dataJSON)),
	}, nil
}

// PublishTool handles publishing blog posts to Notion
type PublishTool struct {
	manager *BlogToolsManager
}

// NewPublishTool creates a new publish tool
func NewPublishTool(manager *BlogToolsManager) *PublishTool {
	return &PublishTool{manager: manager}
}

// Name returns the tool name
func (t *PublishTool) Name() string {
	return "blog_publish"
}

// Description returns the tool description
func (t *PublishTool) Description() string {
	return "Publish a blog post to Notion and optionally mark as published on Substack"
}

// Execute executes the publish tool
func (t *PublishTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	postIDStr, ok := args["post_id"].(string)
	if !ok {
		return &tools.Result{Success: false, Error: "missing 'post_id' argument"}, nil
	}

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("invalid post_id: %v", err)}, nil
	}

	// Add newsletter if requested
	if addNewsletter, ok := args["add_newsletter"].(bool); ok && addNewsletter {
		if err := t.manager.AddNewsletterSection(ctx, postID); err != nil {
			return &tools.Result{Success: false, Error: fmt.Sprintf("failed to add newsletter section: %v", err)}, nil
		}
	}

	// Publish to Notion
	if err := t.manager.PublishToNotion(ctx, postID); err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("failed to publish to Notion: %v", err)}, nil
	}

	// Mark as published if Substack URL provided
	if substackURL, ok := args["substack_url"].(string); ok && substackURL != "" {
		if err := t.manager.MarkAsPublished(ctx, postID, substackURL); err != nil {
			return &tools.Result{Success: false, Error: fmt.Sprintf("failed to mark as published: %v", err)}, nil
		}
	}

	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Blog post published successfully (ID: %s)", postIDStr),
	}, nil
}

// ListPostsTool lists blog posts
type ListPostsTool struct {
	manager *BlogToolsManager
}

// NewListPostsTool creates a new list posts tool
func NewListPostsTool(manager *BlogToolsManager) *ListPostsTool {
	return &ListPostsTool{manager: manager}
}

// Name returns the tool name
func (t *ListPostsTool) Name() string {
	return "blog_list_posts"
}

// Description returns the tool description
func (t *ListPostsTool) Description() string {
	return "List blog posts, optionally filtered by status"
}

// Execute executes the list posts tool
func (t *ListPostsTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	var status blogstorage.PostStatus
	if statusStr, ok := args["status"].(string); ok && statusStr != "" {
		status = blogstorage.PostStatus(statusStr)
	}

	posts, err := t.manager.ListPosts(ctx, status)
	if err != nil {
		return &tools.Result{Success: false, Error: fmt.Sprintf("failed to list posts: %v", err)}, nil
	}

	output := fmt.Sprintf("Found %d blog posts\n", len(posts))
	postsData := make([]map[string]interface{}, len(posts))

	for i, post := range posts {
		output += fmt.Sprintf("- %s (ID: %s, Status: %s)\n", post.Title, post.ID, post.Status)
		postsData[i] = map[string]interface{}{
			"id":     post.ID.String(),
			"title":  post.Title,
			"status": string(post.Status),
		}
	}

	// Include structured data as JSON
	data := map[string]interface{}{
		"posts": postsData,
		"count": len(posts),
	}
	dataJSON, _ := json.Marshal(data)
	output += string(dataJSON)

	return &tools.Result{
		Success: true,
		Output:  output,
	}, nil
}
