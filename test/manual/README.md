# Manual Testing for Research Tools

This directory contains manual tests for the web search and web scraper tools used in the blog workflow.

## Prerequisites

- Internet connection (for web search and URL scraping)
- From project root directory: `/Users/miriahpeterson/Code/go-projects/pedrocli/`

## Available Tests

### 1. Web Search Test

Tests DuckDuckGo search functionality:

```bash
go run test/manual/research_tools.go search
```

**What it does:**
- Searches for "golang error handling best practices" (3 results)
- Searches for "site:github.com kubernetes operators" with filter (3 results)
- Displays titles, URLs, and snippets

**Expected output:**
- List of search results with titles and URLs
- Results should be relevant to the query
- GitHub results should contain "github" in URL

### 2. URL Scraping Test

Tests scraping content from a web page:

```bash
go run test/manual/research_tools.go scrape-url
```

**What it does:**
- Scrapes https://example.com
- Strips HTML tags
- Truncates to 500 characters

**Expected output:**
- Clean text content from example.com
- Should contain "Example Domain"
- No HTML tags in output

### 3. GitHub Scraping Test

Tests scraping files from GitHub repositories:

```bash
go run test/manual/research_tools.go scrape-github
```

**What it does:**
- Scrapes README from torvalds/linux repository
- Uses master branch
- Truncates to 1000 characters

**Expected output:**
- Content of Linux kernel README
- Should mention "Linux kernel"
- Plain text format

### 4. Local File Scraping Test

Tests reading local files with security:

```bash
go run test/manual/research_tools.go scrape-local
```

**What it does:**
- Reads test/manual/research_tools.go
- Truncates to 500 characters
- Tests security (prevents directory traversal)

**Expected output:**
- First 500 characters of the test file
- Should show Go source code
- Includes truncation message

### 5. Full Workflow Test

Tests the complete search → scrape workflow:

```bash
go run test/manual/research_tools.go workflow
```

**What it does:**
1. Searches for "golang official documentation"
2. Shows search results
3. Scrapes https://go.dev/doc/
4. Shows scraped content (800 chars)

**Expected output:**
- Search results with Go documentation links
- Scraped content from go.dev
- Clean text without HTML tags

## Quick Test All

Test all functionality in sequence:

```bash
# From project root
go run test/manual/research_tools.go search
echo "\n---\n"
go run test/manual/research_tools.go scrape-url
echo "\n---\n"
go run test/manual/research_tools.go scrape-github
echo "\n---\n"
go run test/manual/research_tools.go scrape-local
echo "\n---\n"
go run test/manual/research_tools.go workflow
```

## Troubleshooting

### "Security error" when scraping local files

This is expected behavior! The tool prevents reading files outside the working directory.

To test security:
```bash
cd /tmp
go run /path/to/pedrocli/test/manual/research_tools.go scrape-local
# Should fail with "access denied: path outside working directory"
```

### "Request failed" errors

- Check internet connection
- DuckDuckGo or GitHub might be rate-limiting
- Try again in a few seconds

### "No results found"

- DuckDuckGo's HTML parsing might have changed
- Search query might be too specific
- Try a different query

## Integration with Blog Workflow

These tools are designed to work together in the blog research phase:

1. **Search** for relevant content (web_search)
2. **Scrape** the most relevant URLs (web_scraper)
3. **Gather** code examples from GitHub (web_scraper)
4. **Read** local files for context (web_scraper)

Example agent workflow:
```
Research Phase:
├─ web_search: "kubernetes custom resources tutorial"
├─ web_scraper: scrape_url → top result
├─ web_scraper: scrape_github → k8s/sample-controller
└─ web_scraper: scrape_local → local example code
```

## Next Steps

After verifying these tools work:
- Phase 4: Integrate into BlogContentAgent
- Phase 5: Add CLI commands for blog workflow
- Phase 6: Add web UI for blog review
