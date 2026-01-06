package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/logits"
)

// ContextTool provides context management operations for phased workflows
type ContextTool struct {
	// summaries stores compacted summaries by key
	summaries map[string]string
}

// NewContextTool creates a new context tool
func NewContextTool() *ContextTool {
	return &ContextTool{
		summaries: make(map[string]string),
	}
}

// Name returns the tool name
func (c *ContextTool) Name() string {
	return "context"
}

// Description returns the tool description
func (c *ContextTool) Description() string {
	return `Manage context during phased workflows.

Actions:
- compact: Summarize completed work to save context window space
  Args: key (string), content (string), summary (string)
  Stores a summary of work completed, replacing verbose details

- recall: Retrieve a previously stored summary
  Args: key (string)
  Returns the summary stored under that key

- checkpoint: Mark a checkpoint in the workflow
  Args: name (string), description (string)
  Creates a named checkpoint for potential resume

- list: List all stored summaries and checkpoints
  Returns all keys and brief previews

Use context compaction after completing significant work chunks to prevent
context window exhaustion during long workflows.

Examples:
{"tool": "context", "args": {"action": "compact", "key": "phase1_analysis", "content": "...", "summary": "Analyzed 15 files, found 3 issues..."}}
{"tool": "context", "args": {"action": "recall", "key": "phase1_analysis"}}
{"tool": "context", "args": {"action": "checkpoint", "name": "pre_implementation", "description": "Ready to start coding"}}`
}

// Execute executes the context tool
func (c *ContextTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'action' parameter"}, nil
	}

	switch action {
	case "compact":
		return c.compact(ctx, args)
	case "recall":
		return c.recall(ctx, args)
	case "checkpoint":
		return c.checkpoint(ctx, args)
	case "list":
		return c.list(ctx, args)
	default:
		return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
	}
}

// compact stores a summary of content
func (c *ContextTool) compact(ctx context.Context, args map[string]interface{}) (*Result, error) {
	key, ok := args["key"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'key' parameter"}, nil
	}

	summary, ok := args["summary"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'summary' parameter"}, nil
	}

	// Store the summary
	c.summaries[key] = summary

	// Calculate space saved if content was provided
	spaceSaved := 0
	if content, ok := args["content"].(string); ok {
		spaceSaved = len(content) - len(summary)
	}

	output := fmt.Sprintf("Compacted '%s': %s", key, summary)
	if spaceSaved > 0 {
		output += fmt.Sprintf("\nSpace saved: ~%d chars", spaceSaved)
	}

	return &Result{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"key":         key,
			"summary":     summary,
			"space_saved": spaceSaved,
		},
	}, nil
}

// recall retrieves a stored summary
func (c *ContextTool) recall(ctx context.Context, args map[string]interface{}) (*Result, error) {
	key, ok := args["key"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'key' parameter"}, nil
	}

	summary, exists := c.summaries[key]
	if !exists {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("no summary found for key '%s'", key),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  summary,
		Data: map[string]interface{}{
			"key":     key,
			"summary": summary,
		},
	}, nil
}

// checkpoint creates a named checkpoint
func (c *ContextTool) checkpoint(ctx context.Context, args map[string]interface{}) (*Result, error) {
	name, ok := args["name"].(string)
	if !ok {
		return &Result{Success: false, Error: "missing 'name' parameter"}, nil
	}

	description, ok := args["description"].(string)
	if !ok {
		description = ""
	}

	// Store checkpoint as a special summary
	checkpointKey := "checkpoint:" + name
	c.summaries[checkpointKey] = description

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Checkpoint '%s' created: %s", name, description),
		Data: map[string]interface{}{
			"checkpoint": name,
			"description": description,
		},
	}, nil
}

// list returns all stored summaries
func (c *ContextTool) list(ctx context.Context, args map[string]interface{}) (*Result, error) {
	if len(c.summaries) == 0 {
		return &Result{
			Success: true,
			Output:  "No summaries or checkpoints stored",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString("Stored context:\n")

	checkpoints := []string{}
	summaries := []string{}

	for key := range c.summaries {
		if strings.HasPrefix(key, "checkpoint:") {
			checkpoints = append(checkpoints, key)
		} else {
			summaries = append(summaries, key)
		}
	}

	if len(checkpoints) > 0 {
		sb.WriteString("\nCheckpoints:\n")
		for _, key := range checkpoints {
			name := strings.TrimPrefix(key, "checkpoint:")
			preview := truncateString(c.summaries[key], 50)
			sb.WriteString(fmt.Sprintf("  • %s: %s\n", name, preview))
		}
	}

	if len(summaries) > 0 {
		sb.WriteString("\nSummaries:\n")
		for _, key := range summaries {
			preview := truncateString(c.summaries[key], 50)
			sb.WriteString(fmt.Sprintf("  • %s: %s\n", key, preview))
		}
	}

	return &Result{
		Success: true,
		Output:  sb.String(),
		Data: map[string]interface{}{
			"count":       len(c.summaries),
			"checkpoints": len(checkpoints),
			"summaries":   len(summaries),
		},
	}, nil
}

// GetSummary returns a summary by key (for programmatic access)
func (c *ContextTool) GetSummary(key string) (string, bool) {
	summary, ok := c.summaries[key]
	return summary, ok
}

// SetSummary sets a summary programmatically
func (c *ContextTool) SetSummary(key, summary string) {
	c.summaries[key] = summary
}

// Metadata returns rich tool metadata
func (c *ContextTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type:        "string",
					Enum:        []interface{}{"compact", "recall", "checkpoint", "list"},
					Description: "The context operation to perform",
				},
				"key": {
					Type:        "string",
					Description: "Key for storing/retrieving summaries",
				},
				"content": {
					Type:        "string",
					Description: "Original verbose content to be compacted",
				},
				"summary": {
					Type:        "string",
					Description: "Concise summary of the content",
				},
				"name": {
					Type:        "string",
					Description: "Checkpoint name",
				},
				"description": {
					Type:        "string",
					Description: "Checkpoint description",
				},
			},
			Required: []string{"action"},
		},
		Category:    CategoryUtility,
		Optionality: ToolOptional,
		UsageHint:   "Use after completing significant work chunks to save context window space",
		Examples: []ToolExample{
			{
				Description: "Compact analysis results",
				Input: map[string]interface{}{
					"action":  "compact",
					"key":     "file_analysis",
					"summary": "Analyzed 10 files, found 2 bugs in auth.go and config.go",
				},
			},
			{
				Description: "Create checkpoint before implementation",
				Input: map[string]interface{}{
					"action":      "checkpoint",
					"name":        "pre_impl",
					"description": "Plan complete, ready for implementation",
				},
			},
		},
	}
}

// Helper function to truncate strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
