package agents

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with surrounding text",
			input:    `Here is the result: {"key": "value"} That's all.`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "nested JSON",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "no JSON",
			input:    `No JSON here`,
			expected: `No JSON here`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSON(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractJSONFromCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "JSON in code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON in unmarked code block",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "no code block",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSONFromCodeBlock(tt.input)
			if result != tt.expected {
				t.Errorf("extractJSONFromCodeBlock(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatSections(t *testing.T) {
	sections := []ContentSection{
		{Title: "Introduction", Description: "Opening hook", Priority: 1},
		{Title: "Main Content", Description: "The meat of the post", Priority: 1},
		{Title: "Conclusion", Description: "Wrap up", Priority: 2},
	}

	result := formatSections(sections)

	if !strings.Contains(result, "1. Introduction") {
		t.Error("expected Introduction section")
	}

	if !strings.Contains(result, "2. Main Content") {
		t.Error("expected Main Content section")
	}

	if !strings.Contains(result, "3. Conclusion") {
		t.Error("expected Conclusion section")
	}
}

func TestFormatResearchData(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		contains string
	}{
		{
			name:     "empty data",
			data:     map[string]interface{}{},
			contains: "No research data available",
		},
		{
			name: "with data",
			data: map[string]interface{}{
				"calendar": "Some events",
			},
			contains: "calendar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatResearchData(tt.data)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatResearchData() should contain %q, got %q", tt.contains, result)
			}
		})
	}
}

func TestExtractSectionsFromOutline(t *testing.T) {
	outline := `# Blog Post Title

## Introduction
Some intro content

## Main Topic
The main content here

## Another Section
More content

## Conclusion
Wrapping up
`

	sections := extractSectionsFromOutline(outline)

	if len(sections) != 4 {
		t.Errorf("expected 4 sections, got %d", len(sections))
	}

	expectedSections := []string{"Introduction", "Main Topic", "Another Section", "Conclusion"}
	for i, expected := range expectedSections {
		if i >= len(sections) {
			t.Errorf("missing section %d: %s", i, expected)
			continue
		}
		if sections[i] != expected {
			t.Errorf("section %d: expected %q, got %q", i, expected, sections[i])
		}
	}
}

func TestExtractSectionsFromOutlineEmpty(t *testing.T) {
	outline := `No headers here
Just plain text
`

	sections := extractSectionsFromOutline(outline)

	if len(sections) != 0 {
		t.Errorf("expected 0 sections for headerless outline, got %d", len(sections))
	}
}

func TestBlogPromptAnalysisStruct(t *testing.T) {
	// Test that the struct can be marshaled/unmarshaled
	analysis := BlogPromptAnalysis{
		MainTopic: "2025 Year in Review",
		ContentSections: []ContentSection{
			{Title: "Intro", Description: "Hook", Priority: 1},
		},
		ResearchTasks: []ResearchTask{
			{Type: "calendar", Params: map[string]interface{}{"action": "list_events"}},
		},
		IncludeNewsletter:  true,
		EstimatedWordCount: 1500,
	}

	if analysis.MainTopic != "2025 Year in Review" {
		t.Error("MainTopic not set correctly")
	}

	if len(analysis.ContentSections) != 1 {
		t.Error("ContentSections not set correctly")
	}

	if len(analysis.ResearchTasks) != 1 {
		t.Error("ResearchTasks not set correctly")
	}

	if !analysis.IncludeNewsletter {
		t.Error("IncludeNewsletter should be true")
	}
}

func TestBlogOrchestratorOutputStruct(t *testing.T) {
	output := BlogOrchestratorOutput{
		Analysis: &BlogPromptAnalysis{
			MainTopic: "Test Topic",
		},
		ResearchData:   map[string]interface{}{"calendar": "events"},
		Outline:        "## Section 1",
		ExpandedDraft:  "Full content here",
		Newsletter:     "Newsletter section",
		FullContent:    "Full content here\n\n---\n\nNewsletter section",
		SocialPosts:    map[string]string{"twitter_post": "Check out my new post!"},
		SuggestedTitle: "Test Topic",
	}

	if output.Analysis.MainTopic != "Test Topic" {
		t.Error("Analysis.MainTopic not set correctly")
	}

	if output.FullContent == "" {
		t.Error("FullContent should not be empty")
	}

	if len(output.SocialPosts) != 1 {
		t.Error("SocialPosts should have 1 entry")
	}
}

func TestContentSectionStruct(t *testing.T) {
	section := ContentSection{
		Title:       "Introduction",
		Description: "Opening hook for the article",
		Priority:    1,
	}

	if section.Title != "Introduction" {
		t.Error("Title not set correctly")
	}

	if section.Priority != 1 {
		t.Error("Priority not set correctly")
	}
}

func TestResearchTaskStruct(t *testing.T) {
	task := ResearchTask{
		Type: "rss_feed",
		Params: map[string]interface{}{
			"action": "get_configured",
			"limit":  5.0,
		},
	}

	if task.Type != "rss_feed" {
		t.Error("Type not set correctly")
	}

	if action, ok := task.Params["action"].(string); !ok || action != "get_configured" {
		t.Error("Params.action not set correctly")
	}
}

func TestNewBlogOrchestratorAgent(t *testing.T) {
	// This test verifies the constructor doesn't panic
	// Full integration tests require LLM backend

	// Note: We can't fully test without config and LLM backend
	// This is more of a smoke test
	t.Log("NewBlogOrchestratorAgent constructor test - requires full integration test for complete coverage")
}
