package main

import (
	"context"
	"fmt"
	"os"

	"github.com/soypete/pedrocli/pkg/tools"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "search":
		testWebSearch()
	case "scrape-url":
		testScrapeURL()
	case "scrape-github":
		testScrapeGithub()
	case "scrape-local":
		testScrapeLocal()
	case "workflow":
		testSearchAndScrape()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Manual Research Tools Test

Usage: go run test/manual/research_tools.go <command>

Commands:
  search          - Test web search (DuckDuckGo)
  scrape-url      - Test URL scraping (example.com)
  scrape-github   - Test GitHub scraping (torvalds/linux README)
  scrape-local    - Test local file scraping (this file)
  workflow        - Test full search → scrape workflow

Examples:
  go run test/manual/research_tools.go search
  go run test/manual/research_tools.go workflow
`)
}

func testWebSearch() {
	fmt.Println("=== Testing Web Search ===\n")

	searchTool := tools.NewWebSearchTool()

	queries := []map[string]interface{}{
		{
			"query":       "golang error handling best practices",
			"max_results": 3,
		},
		{
			"query":       "site:github.com kubernetes operators",
			"max_results": 3,
			"filter":      "github",
		},
	}

	for i, args := range queries {
		fmt.Printf("Query %d: %v\n\n", i+1, args["query"])

		result, err := searchTool.Execute(context.Background(), args)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		if !result.Success {
			fmt.Printf("Failed: %s\n\n", result.Error)
			continue
		}

		fmt.Println(result.Output)
		fmt.Println("---")
	}
}

func testScrapeURL() {
	fmt.Println("=== Testing URL Scraping ===\n")

	scraperTool := tools.NewWebScraperTool()

	args := map[string]interface{}{
		"action":     "scrape_url",
		"url":        "https://example.com",
		"max_length": 500,
	}

	fmt.Printf("Scraping: %s\n\n", args["url"])

	result, err := scraperTool.Execute(context.Background(), args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("Failed: %s\n", result.Error)
		return
	}

	fmt.Println("Result:")
	fmt.Println(result.Output)
}

func testScrapeGithub() {
	fmt.Println("=== Testing GitHub Scraping ===\n")

	scraperTool := tools.NewWebScraperTool()

	args := map[string]interface{}{
		"action":     "scrape_github",
		"repo":       "torvalds/linux",
		"path":       "README",
		"branch":     "master",
		"max_length": 1000,
	}

	fmt.Printf("Scraping: %s/%s (branch: %s)\n\n", args["repo"], args["path"], args["branch"])

	result, err := scraperTool.Execute(context.Background(), args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("Failed: %s\n", result.Error)
		return
	}

	fmt.Println("Result:")
	fmt.Println(result.Output)
}

func testScrapeLocal() {
	fmt.Println("=== Testing Local File Scraping ===\n")

	scraperTool := tools.NewWebScraperTool()

	// Try to read this test file
	args := map[string]interface{}{
		"action":     "scrape_local",
		"path":       "test/manual/research_tools.go",
		"max_length": 500,
	}

	fmt.Printf("Scraping local file: %s\n\n", args["path"])

	result, err := scraperTool.Execute(context.Background(), args)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !result.Success {
		fmt.Printf("Failed: %s\n", result.Error)
		return
	}

	fmt.Println("Result (first 500 chars):")
	fmt.Println(result.Output)
}

func testSearchAndScrape() {
	fmt.Println("=== Testing Search → Scrape Workflow ===\n")

	searchTool := tools.NewWebSearchTool()
	scraperTool := tools.NewWebScraperTool()

	// Step 1: Search for Go documentation
	fmt.Println("Step 1: Searching for Go documentation...")
	searchArgs := map[string]interface{}{
		"query":       "golang official documentation",
		"max_results": 3,
	}

	searchResult, err := searchTool.Execute(context.Background(), searchArgs)
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
		return
	}

	if !searchResult.Success {
		fmt.Printf("Search failed: %s\n", searchResult.Error)
		return
	}

	fmt.Println(searchResult.Output)
	fmt.Println("---\n")

	// Step 2: Scrape the Go official site
	fmt.Println("Step 2: Scraping https://go.dev/doc/...")
	scrapeArgs := map[string]interface{}{
		"action":       "scrape_url",
		"url":          "https://go.dev/doc/",
		"extract_code": false,
		"max_length":   800,
	}

	scrapeResult, err := scraperTool.Execute(context.Background(), scrapeArgs)
	if err != nil {
		fmt.Printf("Scrape error: %v\n", err)
		return
	}

	if !scrapeResult.Success {
		fmt.Printf("Scrape failed: %s\n", scrapeResult.Error)
		return
	}

	fmt.Println("Scraped content:")
	fmt.Println(scrapeResult.Output)
	fmt.Println("\n✅ Workflow complete: Search → Scrape")
}
