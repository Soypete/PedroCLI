package tools

import (
	"context"
	"testing"
)

func TestWebSearchTool_Name(t *testing.T) {
	tool := NewWebSearchTool()
	if tool.Name() != "web_search" {
		t.Errorf("Expected name 'web_search', got '%s'", tool.Name())
	}
}

func TestWebSearchTool_Description(t *testing.T) {
	tool := NewWebSearchTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestWebSearchTool_Search(t *testing.T) {
	t.Skip("Skipping external API test - web search requires DuckDuckGo API")

	tool := NewWebSearchTool()

	args := map[string]interface{}{
		"query":       "golang best practices",
		"max_results": 3,
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// External API test - may fail due to rate limits or API changes
	// Just verify the tool executes without panic
	if !result.Success {
		t.Logf("Search failed (expected with external API): %s", result.Error)
	} else {
		t.Logf("Search succeeded: %d bytes returned", len(result.Output))
		// If successful, output should not be empty
		if len(result.Output) == 0 {
			t.Error("Expected non-empty output on success")
		}
	}
}

func TestWebSearchTool_SearchWithFilter(t *testing.T) {
	t.Skip("Skipping external API test - web search requires DuckDuckGo API")

	tool := NewWebSearchTool()

	args := map[string]interface{}{
		"query":       "site:github.com kubernetes",
		"max_results": 5,
		"filter":      "github",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// External API test - may fail due to rate limits or API changes
	// Just verify the tool executes without panic
	if !result.Success {
		t.Logf("Filtered search failed (expected with external API): %s", result.Error)
	} else {
		t.Logf("Filtered search succeeded: %d bytes returned", len(result.Output))
	}
}

func TestWebSearchTool_MissingQuery(t *testing.T) {
	tool := NewWebSearchTool()

	args := map[string]interface{}{}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for missing query")
	}

	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestWebSearchTool_MaxResultsLimit(t *testing.T) {
	tool := NewWebSearchTool()

	// Request more than max allowed (10)
	args := map[string]interface{}{
		"query":       "test",
		"max_results": 100,
	}

	// Should not fail, but should limit to 10
	result, _ := tool.Execute(context.Background(), args)

	// This is a light test - we can't verify the actual limit without making a real request
	// but we verify the args are accepted
	if result == nil {
		t.Error("Expected result, got nil")
	}
}

func TestWebSearchTool_CleanURL(t *testing.T) {
	tool := NewWebSearchTool()

	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com",
			expected: "https://example.com",
		},
		{
			input:    "//example.com/page",
			expected: "https://example.com/page",
		},
		{
			input:    "https://example.com",
			expected: "https://example.com",
		},
	}

	for _, tc := range testCases {
		result := tool.cleanURL(tc.input)
		if result != tc.expected {
			t.Errorf("cleanURL(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestWebSearchTool_FilterResults(t *testing.T) {
	tool := NewWebSearchTool()

	results := []SearchResult{
		{Title: "Go Best Practices", URL: "https://go.dev/doc", Snippet: "Official Go docs"},
		{Title: "Kubernetes Guide", URL: "https://kubernetes.io", Snippet: "K8s documentation"},
		{Title: "GitHub Actions", URL: "https://github.com/features/actions", Snippet: "CI/CD with GitHub"},
	}

	// Filter by "github"
	filtered := tool.filterResults(results, "github")

	if len(filtered) != 1 {
		t.Errorf("Expected 1 result, got %d", len(filtered))
	}

	if len(filtered) > 0 && !contains(filtered[0].URL, "github") {
		t.Error("Expected filtered result to contain 'github'")
	}

	// Filter by "go"
	filtered = tool.filterResults(results, "go")

	if len(filtered) < 1 {
		t.Error("Expected at least 1 result for 'go'")
	}
}

func TestWebSearchTool_FormatResults(t *testing.T) {
	tool := NewWebSearchTool()

	results := []SearchResult{
		{Title: "Go Documentation", URL: "https://go.dev", Snippet: "Learn Go"},
		{Title: "Effective Go", URL: "https://go.dev/doc/effective_go", Snippet: "Writing good Go code"},
	}

	output := tool.formatResults(results)

	if !contains(output, "Found 2 result(s)") {
		t.Error("Expected output to show result count")
	}

	if !contains(output, "Go Documentation") {
		t.Error("Expected output to contain first title")
	}

	if !contains(output, "https://go.dev") {
		t.Error("Expected output to contain URL")
	}

	// Test empty results
	emptyOutput := tool.formatResults([]SearchResult{})
	if !contains(emptyOutput, "No results found") {
		t.Error("Expected 'No results found' for empty results")
	}
}

func TestWebSearchTool_Metadata(t *testing.T) {
	tool := NewWebSearchTool()
	metadata := tool.Metadata()

	if metadata.Category != CategoryResearch {
		t.Errorf("Expected category 'research', got '%s'", metadata.Category)
	}

	if metadata.Optionality != ToolOptional {
		t.Errorf("Expected optionality 'optional', got '%s'", metadata.Optionality)
	}

	if metadata.Schema == nil {
		t.Error("Expected schema to be defined")
	}

	if len(metadata.Schema.Required) == 0 {
		t.Error("Expected required fields in schema")
	}

	if len(metadata.Examples) == 0 {
		t.Error("Expected examples to be provided")
	}
}

// Note: contains() helper is defined in research_links_test.go
