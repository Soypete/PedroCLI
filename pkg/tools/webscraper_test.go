package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWebScraperTool_Name(t *testing.T) {
	tool := NewWebScraperTool()
	if tool.Name() != "web_scraper" {
		t.Errorf("Expected name 'web_scraper', got '%s'", tool.Name())
	}
}

func TestWebScraperTool_Description(t *testing.T) {
	tool := NewWebScraperTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestWebScraperTool_ScrapeLocal(t *testing.T) {
	tool := NewWebScraperTool()

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	testContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Change to temp directory for security check
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	args := map[string]interface{}{
		"action": "scrape_local",
		"path":   "test.go",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if result.Output != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, result.Output)
	}
}

func TestWebScraperTool_ScrapeLocal_Security(t *testing.T) {
	tool := NewWebScraperTool()

	// Try to access file outside working directory
	args := map[string]interface{}{
		"action": "scrape_local",
		"path":   "../../etc/passwd",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for path outside working directory")
	}

	if result.Error == "" {
		t.Error("Expected error message")
	}
}

func TestWebScraperTool_ScrapeGithub(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping GitHub scraping test in short mode")
	}

	tool := NewWebScraperTool()

	args := map[string]interface{}{
		"action": "scrape_github",
		"repo":   "torvalds/linux",
		"path":   "README",
		"branch": "master",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if len(result.Output) == 0 {
		t.Error("Expected non-empty output from GitHub")
	}

	// README should contain "Linux kernel"
	if result.Output != "" && len(result.Output) < 50 {
		t.Error("Expected substantial README content")
	}
}

func TestWebScraperTool_ScrapeURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping URL scraping test in short mode")
	}

	tool := NewWebScraperTool()

	args := map[string]interface{}{
		"action": "scrape_url",
		"url":    "https://example.com",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if len(result.Output) == 0 {
		t.Error("Expected non-empty output from URL")
	}
}

func TestWebScraperTool_ExtractCode(t *testing.T) {
	tool := NewWebScraperTool()

	content := `# Example

Here's some code:

` + "```go" + `
package main

func main() {}
` + "```" + `

And more text.`

	extracted := tool.extractCodeBlocks(content)

	if !contains(extracted, "package main") {
		t.Error("Expected extracted code to contain 'package main'")
	}

	if contains(extracted, "Here's some code") {
		t.Error("Expected extracted code to NOT contain surrounding text")
	}
}

func TestWebScraperTool_StripHTML(t *testing.T) {
	tool := NewWebScraperTool()

	html := `<html>
		<head><title>Test</title></head>
		<body>
			<h1>Hello</h1>
			<p>World</p>
			<script>alert('bad');</script>
		</body>
	</html>`

	stripped := tool.stripHTMLTags(html)

	if contains(stripped, "<html>") {
		t.Error("Expected HTML tags to be removed")
	}

	if contains(stripped, "alert") {
		t.Error("Expected script tags to be removed")
	}

	if !contains(stripped, "Hello") {
		t.Error("Expected text content to be preserved")
	}

	if !contains(stripped, "World") {
		t.Error("Expected text content to be preserved")
	}
}

func TestWebScraperTool_MaxLength(t *testing.T) {
	tool := NewWebScraperTool()

	// Create a temporary test file with long content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "long.txt")
	longContent := string(make([]byte, 1000)) // 1000 characters
	for i := range longContent {
		longContent = longContent[:i] + "a" + longContent[i+1:]
	}
	if err := os.WriteFile(testFile, []byte(longContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	args := map[string]interface{}{
		"action":     "scrape_local",
		"path":       "long.txt",
		"max_length": 100,
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}

	if len(result.Output) > 120 { // 100 + "[truncated...]"
		t.Errorf("Expected output truncated to ~100 chars, got %d", len(result.Output))
	}

	if !contains(result.Output, "truncated") {
		t.Error("Expected truncation message")
	}
}

func TestWebScraperTool_MissingAction(t *testing.T) {
	tool := NewWebScraperTool()

	args := map[string]interface{}{}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for missing action")
	}
}

func TestWebScraperTool_InvalidAction(t *testing.T) {
	tool := NewWebScraperTool()

	args := map[string]interface{}{
		"action": "invalid",
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for invalid action")
	}
}

func TestWebScraperTool_Metadata(t *testing.T) {
	tool := NewWebScraperTool()
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
