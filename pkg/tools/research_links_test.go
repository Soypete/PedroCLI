package tools

import (
	"context"
	"testing"
)

func TestResearchLinksTool_Name(t *testing.T) {
	tool := NewResearchLinksTool(nil)
	if tool.Name() != "research_links" {
		t.Errorf("expected name 'research_links', got '%s'", tool.Name())
	}
}

func TestResearchLinksTool_List_Empty(t *testing.T) {
	tool := NewResearchLinksTool(nil)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["count"].(int) != 0 {
		t.Errorf("expected 0 links, got %d", result.Data["count"].(int))
	}
}

func TestResearchLinksTool_List_WithLinks(t *testing.T) {
	links := []ResearchLink{
		{
			URL:      "https://example.com/doc1",
			Title:    "Documentation 1",
			Category: "reference",
			Notes:    "Main reference doc",
		},
		{
			URL:      "https://example.com/doc2",
			Title:    "Citation Source",
			Category: "citation",
		},
		{
			URL:      "https://example.com/code",
			Title:    "Code Example",
			Category: "example",
			Labels:   []string{"go", "tutorial"},
		},
	}

	tool := NewResearchLinksToolFromLinks(links, "General notes here")

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["count"].(int) != 3 {
		t.Errorf("expected 3 links, got %d", result.Data["count"].(int))
	}
	if result.Data["plain_notes"].(string) != "General notes here" {
		t.Errorf("expected plain_notes, got '%s'", result.Data["plain_notes"].(string))
	}
}

func TestResearchLinksTool_List_FilterByCategory(t *testing.T) {
	links := []ResearchLink{
		{URL: "https://example.com/ref1", Category: "reference"},
		{URL: "https://example.com/ref2", Category: "reference"},
		{URL: "https://example.com/cite1", Category: "citation"},
	}

	tool := NewResearchLinksToolFromLinks(links, "")

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":          "list",
		"filter_category": "reference",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["count"].(int) != 2 {
		t.Errorf("expected 2 reference links, got %d", result.Data["count"].(int))
	}
}

func TestResearchLinksTool_DefaultAction(t *testing.T) {
	tool := NewResearchLinksTool(nil)

	// Empty action should default to list
	result, err := tool.Execute(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
}

func TestResearchLinksTool_UnknownAction(t *testing.T) {
	tool := NewResearchLinksTool(nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "invalid_action",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for unknown action")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestResearchLinksTool_Fetch_MissingURL(t *testing.T) {
	tool := NewResearchLinksTool(nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for missing URL")
	}
	if result.Error != "missing 'url' parameter" {
		t.Errorf("expected 'missing url' error, got: %s", result.Error)
	}
}

func TestResearchLinksTool_FetchAll_Empty(t *testing.T) {
	tool := NewResearchLinksTool(nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fetch_all",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data["count"].(int) != 0 {
		t.Errorf("expected 0 fetched, got %d", result.Data["count"].(int))
	}
}

func TestResearchLinksTool_Metadata(t *testing.T) {
	tool := NewResearchLinksTool(nil)
	metadata := tool.Metadata()

	if metadata == nil {
		t.Fatal("expected metadata, got nil")
	}
	if metadata.Category != CategoryResearch {
		t.Errorf("expected category 'research', got '%s'", metadata.Category)
	}
	if metadata.Optionality != ToolOptional {
		t.Errorf("expected optionality 'optional', got '%s'", metadata.Optionality)
	}
	if len(metadata.Examples) == 0 {
		t.Error("expected examples in metadata")
	}
	if metadata.Schema == nil {
		t.Error("expected schema in metadata")
	}
	if len(metadata.Schema.Properties) == 0 {
		t.Error("expected properties in schema")
	}
}

func TestResearchLinksTool_HasLinks(t *testing.T) {
	toolEmpty := NewResearchLinksTool(nil)
	if toolEmpty.HasLinks() {
		t.Error("expected HasLinks() false for nil context")
	}

	toolWithLinks := NewResearchLinksToolFromLinks([]ResearchLink{
		{URL: "https://example.com"},
	}, "")
	if !toolWithLinks.HasLinks() {
		t.Error("expected HasLinks() true when links present")
	}
}

func TestResearchLinksTool_GetLinks(t *testing.T) {
	links := []ResearchLink{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	}
	tool := NewResearchLinksToolFromLinks(links, "")

	retrieved := tool.GetLinks()
	if len(retrieved) != 2 {
		t.Errorf("expected 2 links, got %d", len(retrieved))
	}
}

func TestResearchLinksTool_GetPlainNotes(t *testing.T) {
	tool := NewResearchLinksToolFromLinks(nil, "Test notes")
	if tool.GetPlainNotes() != "Test notes" {
		t.Errorf("expected 'Test notes', got '%s'", tool.GetPlainNotes())
	}
}

func TestResearchLinksTool_FormatAsMarkdown(t *testing.T) {
	links := []ResearchLink{
		{URL: "https://example.com/cite", Title: "Citation Source", Category: "citation"},
		{URL: "https://example.com/ref", Title: "Reference Doc", Category: "reference", Notes: "Important"},
		{URL: "https://example.com/other"},
	}
	tool := NewResearchLinksToolFromLinks(links, "General notes")

	md := tool.FormatAsMarkdown()

	if md == "" {
		t.Error("expected non-empty markdown")
	}
	if !contains(md, "### Research Links") {
		t.Error("expected '### Research Links' header")
	}
	if !contains(md, "[Citation Source]") {
		t.Error("expected citation link")
	}
	if !contains(md, "[Reference Doc]") {
		t.Error("expected reference link")
	}
	if !contains(md, "Important") {
		t.Error("expected notes in markdown")
	}
	if !contains(md, "**Notes:**") {
		t.Error("expected plain notes section")
	}
}

func TestResearchLinksTool_FormatAsMarkdown_Empty(t *testing.T) {
	tool := NewResearchLinksTool(nil)
	md := tool.FormatAsMarkdown()

	if md != "" {
		t.Errorf("expected empty markdown for nil context, got '%s'", md)
	}
}

func TestResearchLink_Struct(t *testing.T) {
	link := ResearchLink{
		URL:      "https://example.com",
		Title:    "Test",
		Notes:    "Notes here",
		Category: "reference",
		Labels:   []string{"go", "testing"},
	}

	if link.URL != "https://example.com" {
		t.Errorf("expected URL, got '%s'", link.URL)
	}
	if link.Title != "Test" {
		t.Errorf("expected Title, got '%s'", link.Title)
	}
	if link.Category != "reference" {
		t.Errorf("expected Category, got '%s'", link.Category)
	}
	if len(link.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(link.Labels))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
