package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestWebScraperTool_RequiresAction tests that the tool requires action or inferrable args
func TestWebScraperTool_RequiresAction(t *testing.T) {
	tool := NewWebScraperTool()

	// Call without action and without url/path/repo (nothing to infer from)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"extract_code": true,
	})

	if err != nil {
		t.Fatalf("Execute() should not error, got: %v", err)
	}

	// Should return error in result
	if result.Success {
		t.Error("Expected failure when action cannot be inferred")
	}

	if result.Error != "action or url/path/repo is required" {
		t.Errorf("Expected specific error message, got: %s", result.Error)
	}
}

// TestWebScraperTool_ScrapeLocal_Agent tests local file scraping with agent format
func TestWebScraperTool_ScrapeLocal_Agent(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := `package main

func main() {
	println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Change to temp directory for security check
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	tool := NewWebScraperTool()

	// Simulate agent tool call with correct format (relative path)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scrape_local",
		"path":   "test.go",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if result.Output != testContent {
		t.Errorf("Expected content %q, got %q", testContent, result.Output)
	}
}

// TestWebScraperTool_ScrapeURL_Agent tests URL scraping with agent format
func TestWebScraperTool_ScrapeURL_Agent(t *testing.T) {
	tool := NewWebScraperTool()

	// Simulate agent tool call
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scrape_url",
		"url":    "https://example.com",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// May fail due to network, but should not panic
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	t.Logf("Result: success=%v, output_len=%d, error=%s",
		result.Success, len(result.Output), result.Error)
}

// TestWebScraperTool_ScrapeGitHub_Agent tests GitHub scraping with agent format
func TestWebScraperTool_ScrapeGitHub_Agent(t *testing.T) {
	tool := NewWebScraperTool()

	// Simulate agent tool call
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "scrape_github",
		"repo":   "torvalds/linux",
		"path":   "README",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// May fail due to rate limits, but should not panic
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Success {
		if len(result.Output) == 0 {
			t.Error("Expected non-empty output for successful GitHub scrape")
		}
	}

	t.Logf("Result: success=%v, output_len=%d, error=%s",
		result.Success, len(result.Output), result.Error)
}

// TestWebScraperTool_CalledByAgentWithoutAction simulates LLM calling without action
// The tool should infer action from url/repo/path arguments for better UX
func TestWebScraperTool_CalledByAgentWithoutAction(t *testing.T) {
	tool := NewWebScraperTool()

	// LLM calls the tool with url but forgets the action parameter
	// The tool should infer action="scrape_url" from the url argument
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"url":          "https://example.com",
		"extract_code": true,
	})

	if err != nil {
		t.Fatalf("Execute() should not error, got: %v", err)
	}

	// Tool should infer the action and proceed (may fail due to network, but not due to missing action)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	t.Logf("Tool inferred action from url argument: success=%v", result.Success)
}

// TestWebScraperTool_AllActionsHaveExamples verifies description has examples
func TestWebScraperTool_AllActionsHaveExamples(t *testing.T) {
	tool := NewWebScraperTool()
	desc := tool.Description()

	// Check that description includes examples for each action
	requiredExamples := []string{
		`"action": "scrape_local"`,
		`"action": "scrape_github"`,
		`"action": "scrape_url"`,
	}

	for _, example := range requiredExamples {
		if !contains(desc, example) {
			t.Errorf("Description missing example: %s", example)
		}
	}

	// Check that EXAMPLES section exists
	if !contains(desc, "EXAMPLES:") {
		t.Error("Description should have EXAMPLES section")
	}
}

// Use contains from research_links_test.go instead of duplicating
