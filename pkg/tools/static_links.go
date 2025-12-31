package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
)

// StaticLinksOutput represents the structured output for static links
type StaticLinksOutput struct {
	Discord            string              `json:"discord,omitempty"`
	LinkTree           string              `json:"linktree,omitempty"`
	YouTube            string              `json:"youtube,omitempty"`
	Twitter            string              `json:"twitter,omitempty"`
	Bluesky            string              `json:"bluesky,omitempty"`
	LinkedIn           string              `json:"linkedin,omitempty"`
	Newsletter         string              `json:"newsletter,omitempty"`
	YouTubePlaceholder string              `json:"youtube_placeholder,omitempty"`
	CustomLinks        []config.CustomLink `json:"custom_links,omitempty"`
	All                map[string]string   `json:"all,omitempty"`
}

// StaticLinksTool provides access to configured static links for newsletter sections
type StaticLinksTool struct {
	config *config.Config
}

// NewStaticLinksTool creates a new static links tool
func NewStaticLinksTool(cfg *config.Config) *StaticLinksTool {
	return &StaticLinksTool{
		config: cfg,
	}
}

// Name returns the tool name
func (t *StaticLinksTool) Name() string {
	return "static_links"
}

// Description returns the tool description
func (t *StaticLinksTool) Description() string {
	return `Get configured static links for newsletter sections.

Actions:
- get_all: Get all configured static links (social media and custom)
- get_social: Get only social media links (Discord, Twitter, YouTube, etc.)
- get_custom: Get only custom defined links
- get_youtube_placeholder: Get the YouTube video placeholder text

Returns structured JSON with link names and URLs.

Example:
{"tool": "static_links", "args": {"action": "get_all"}}
{"tool": "static_links", "args": {"action": "get_social"}}`
}

// Execute executes the static links tool
func (t *StaticLinksTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	action, ok := args["action"].(string)
	if !ok || action == "" {
		action = "get_all"
	}

	switch action {
	case "get_all":
		return t.getAll()
	case "get_social":
		return t.getSocial()
	case "get_custom":
		return t.getCustom()
	case "get_youtube_placeholder":
		return t.getYouTubePlaceholder()
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// getAll returns all configured static links
func (t *StaticLinksTool) getAll() (*Result, error) {
	links := t.config.Blog.StaticLinks

	// Build all links map
	allLinks := make(map[string]string)
	if links.Discord != "" {
		allLinks["Discord"] = links.Discord
	}
	if links.LinkTree != "" {
		allLinks["LinkTree"] = links.LinkTree
	}
	if links.YouTube != "" {
		allLinks["YouTube"] = links.YouTube
	}
	if links.Twitter != "" {
		allLinks["Twitter"] = links.Twitter
	}
	if links.Bluesky != "" {
		allLinks["Bluesky"] = links.Bluesky
	}
	if links.LinkedIn != "" {
		allLinks["LinkedIn"] = links.LinkedIn
	}
	if links.Newsletter != "" {
		allLinks["Newsletter"] = links.Newsletter
	}

	// Add custom links
	for _, custom := range links.CustomLinks {
		allLinks[custom.Name] = custom.URL
	}

	output := StaticLinksOutput{
		Discord:            links.Discord,
		LinkTree:           links.LinkTree,
		YouTube:            links.YouTube,
		Twitter:            links.Twitter,
		Bluesky:            links.Bluesky,
		LinkedIn:           links.LinkedIn,
		Newsletter:         links.Newsletter,
		YouTubePlaceholder: links.YouTubePlaceholder,
		CustomLinks:        links.CustomLinks,
		All:                allLinks,
	}

	return t.formatResult(output)
}

// getSocial returns only social media links
func (t *StaticLinksTool) getSocial() (*Result, error) {
	links := t.config.Blog.StaticLinks

	social := map[string]string{}
	if links.Discord != "" {
		social["Discord"] = links.Discord
	}
	if links.Twitter != "" {
		social["Twitter"] = links.Twitter
	}
	if links.Bluesky != "" {
		social["Bluesky"] = links.Bluesky
	}
	if links.LinkedIn != "" {
		social["LinkedIn"] = links.LinkedIn
	}
	if links.YouTube != "" {
		social["YouTube"] = links.YouTube
	}

	data, err := json.MarshalIndent(social, "", "  ")
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  string(data),
		Data: map[string]interface{}{
			"social": social,
			"count":  len(social),
		},
	}, nil
}

// getCustom returns only custom links
func (t *StaticLinksTool) getCustom() (*Result, error) {
	links := t.config.Blog.StaticLinks

	if len(links.CustomLinks) == 0 {
		return &Result{
			Success: true,
			Output:  "[]",
			Data: map[string]interface{}{
				"custom": []config.CustomLink{},
				"count":  0,
			},
		}, nil
	}

	data, err := json.MarshalIndent(links.CustomLinks, "", "  ")
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  string(data),
		Data: map[string]interface{}{
			"custom": links.CustomLinks,
			"count":  len(links.CustomLinks),
		},
	}, nil
}

// getYouTubePlaceholder returns the YouTube placeholder text
func (t *StaticLinksTool) getYouTubePlaceholder() (*Result, error) {
	placeholder := t.config.Blog.StaticLinks.YouTubePlaceholder

	if placeholder == "" {
		placeholder = "Latest Video: [ADD LINK]"
	}

	return &Result{
		Success: true,
		Output:  placeholder,
		Data: map[string]interface{}{
			"placeholder": placeholder,
		},
	}, nil
}

// formatResult formats the output as a Result
func (t *StaticLinksTool) formatResult(output StaticLinksOutput) (*Result, error) {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("failed to marshal result: %v", err),
		}, nil
	}

	return &Result{
		Success: true,
		Output:  string(data),
		Data: map[string]interface{}{
			"links":       output,
			"total_count": len(output.All),
		},
	}, nil
}

// FormatAsMarkdown returns the static links formatted as markdown for newsletter
func (t *StaticLinksTool) FormatAsMarkdown() string {
	links := t.config.Blog.StaticLinks
	var md string

	md += "### Stay Connected\n\n"

	if links.Discord != "" {
		md += fmt.Sprintf("- [Join our Discord](%s)\n", links.Discord)
	}
	if links.Twitter != "" {
		md += fmt.Sprintf("- [Follow on Twitter/X](%s)\n", links.Twitter)
	}
	if links.Bluesky != "" {
		md += fmt.Sprintf("- [Follow on Bluesky](%s)\n", links.Bluesky)
	}
	if links.LinkedIn != "" {
		md += fmt.Sprintf("- [Connect on LinkedIn](%s)\n", links.LinkedIn)
	}
	if links.YouTube != "" {
		md += fmt.Sprintf("- [Subscribe on YouTube](%s)\n", links.YouTube)
	}
	if links.LinkTree != "" {
		md += fmt.Sprintf("- [All Links](%s)\n", links.LinkTree)
	}
	if links.Newsletter != "" {
		md += fmt.Sprintf("- [Subscribe to Newsletter](%s)\n", links.Newsletter)
	}

	// Add custom links
	for _, custom := range links.CustomLinks {
		if custom.Icon != "" {
			md += fmt.Sprintf("- %s [%s](%s)\n", custom.Icon, custom.Name, custom.URL)
		} else {
			md += fmt.Sprintf("- [%s](%s)\n", custom.Name, custom.URL)
		}
	}

	return md
}
