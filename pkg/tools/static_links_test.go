package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestStaticLinksTool_GetAll(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				Discord:            "https://discord.gg/test",
				LinkTree:           "https://linktr.ee/test",
				YouTube:            "https://youtube.com/@test",
				Twitter:            "https://twitter.com/test",
				Bluesky:            "https://bsky.app/test",
				LinkedIn:           "https://linkedin.com/in/test",
				Newsletter:         "https://test.substack.com",
				YouTubePlaceholder: "Latest Video: [ADD LINK]",
				CustomLinks: []config.CustomLink{
					{Name: "GitHub", URL: "https://github.com/test", Icon: ""},
				},
			},
		},
	}

	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_all",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	links, ok := result.Data["links"].(StaticLinksOutput)
	if !ok {
		t.Fatal("expected links in data")
	}

	if links.Discord != "https://discord.gg/test" {
		t.Errorf("expected Discord link, got '%s'", links.Discord)
	}

	if links.YouTube != "https://youtube.com/@test" {
		t.Errorf("expected YouTube link, got '%s'", links.YouTube)
	}

	totalCount, ok := result.Data["total_count"].(int)
	if !ok {
		t.Fatal("expected total_count in data")
	}

	// 7 social links + 1 custom = 8
	if totalCount != 8 {
		t.Errorf("expected 8 total links, got %d", totalCount)
	}
}

func TestStaticLinksTool_GetSocial(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				Discord:  "https://discord.gg/test",
				Twitter:  "https://twitter.com/test",
				Bluesky:  "https://bsky.app/test",
				LinkedIn: "https://linkedin.com/in/test",
				YouTube:  "https://youtube.com/@test",
				// Non-social links (should not be included)
				Newsletter: "https://test.substack.com",
				LinkTree:   "https://linktr.ee/test",
			},
		},
	}

	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_social",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	social, ok := result.Data["social"].(map[string]string)
	if !ok {
		t.Fatal("expected social map in data")
	}

	if len(social) != 5 {
		t.Errorf("expected 5 social links, got %d", len(social))
	}

	if social["Discord"] != "https://discord.gg/test" {
		t.Errorf("expected Discord link")
	}

	if social["Twitter"] != "https://twitter.com/test" {
		t.Errorf("expected Twitter link")
	}
}

func TestStaticLinksTool_GetCustom(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				Discord: "https://discord.gg/test",
				CustomLinks: []config.CustomLink{
					{Name: "GitHub", URL: "https://github.com/test", Icon: ""},
					{Name: "Website", URL: "https://example.com", Icon: ""},
				},
			},
		},
	}

	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_custom",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	count, ok := result.Data["count"].(int)
	if !ok {
		t.Fatal("expected count in data")
	}

	if count != 2 {
		t.Errorf("expected 2 custom links, got %d", count)
	}
}

func TestStaticLinksTool_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{},
		},
	}

	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_all",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	totalCount, ok := result.Data["total_count"].(int)
	if !ok {
		t.Fatal("expected total_count in data")
	}

	if totalCount != 0 {
		t.Errorf("expected 0 links for empty config, got %d", totalCount)
	}
}

func TestStaticLinksTool_GetYouTubePlaceholder(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				YouTubePlaceholder: "Latest Video: [ADD YOUR LINK HERE]",
			},
		},
	}

	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_youtube_placeholder",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	if result.Output != "Latest Video: [ADD YOUR LINK HERE]" {
		t.Errorf("expected placeholder text, got '%s'", result.Output)
	}
}

func TestStaticLinksTool_GetYouTubePlaceholderDefault(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{}, // No placeholder set
		},
	}

	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "get_youtube_placeholder",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Should return default placeholder
	if result.Output != "Latest Video: [ADD LINK]" {
		t.Errorf("expected default placeholder, got '%s'", result.Output)
	}
}

func TestStaticLinksTool_DefaultAction(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				Discord: "https://discord.gg/test",
			},
		},
	}

	tool := NewStaticLinksTool(cfg)

	// No action specified - should default to get_all
	result, err := tool.Execute(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	// Should contain all links data
	if _, ok := result.Data["links"]; !ok {
		t.Error("expected links in data for default action")
	}
}

func TestStaticLinksTool_UnknownAction(t *testing.T) {
	cfg := &config.Config{}
	tool := NewStaticLinksTool(cfg)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "unknown_action",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Success {
		t.Fatal("expected failure for unknown action")
	}

	if !strings.Contains(result.Error, "unknown action") {
		t.Errorf("expected 'unknown action' error, got '%s'", result.Error)
	}
}

func TestStaticLinksTool_FormatAsMarkdown(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				Discord:    "https://discord.gg/test",
				Twitter:    "https://twitter.com/test",
				YouTube:    "https://youtube.com/@test",
				Newsletter: "https://test.substack.com",
				CustomLinks: []config.CustomLink{
					{Name: "GitHub", URL: "https://github.com/test", Icon: ""},
				},
			},
		},
	}

	tool := NewStaticLinksTool(cfg)
	md := tool.FormatAsMarkdown()

	if !strings.Contains(md, "### Stay Connected") {
		t.Error("expected header in markdown output")
	}

	if !strings.Contains(md, "[Join our Discord]") {
		t.Error("expected Discord link in markdown")
	}

	if !strings.Contains(md, "[Follow on Twitter/X]") {
		t.Error("expected Twitter link in markdown")
	}

	if !strings.Contains(md, "[GitHub]") {
		t.Error("expected custom link in markdown")
	}
}

func TestStaticLinksTool_FormatAsMarkdownWithIcons(t *testing.T) {
	cfg := &config.Config{
		Blog: config.BlogConfig{
			StaticLinks: config.BlogStaticLinks{
				CustomLinks: []config.CustomLink{
					{Name: "GitHub", URL: "https://github.com/test", Icon: ""},
					{Name: "Website", URL: "https://example.com", Icon: ""},
				},
			},
		},
	}

	tool := NewStaticLinksTool(cfg)
	md := tool.FormatAsMarkdown()

	if !strings.Contains(md, " [GitHub]") {
		t.Error("expected GitHub icon in markdown")
	}

	if !strings.Contains(md, " [Website]") {
		t.Error("expected Website icon in markdown")
	}
}

func TestStaticLinksTool_Name(t *testing.T) {
	tool := NewStaticLinksTool(&config.Config{})

	if tool.Name() != "static_links" {
		t.Errorf("expected name 'static_links', got '%s'", tool.Name())
	}
}

func TestStaticLinksTool_Description(t *testing.T) {
	tool := NewStaticLinksTool(&config.Config{})

	desc := tool.Description()
	if !strings.Contains(desc, "static links") {
		t.Error("description should mention static links")
	}

	if !strings.Contains(desc, "get_all") {
		t.Error("description should mention get_all action")
	}
}
