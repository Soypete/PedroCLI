# Web Scraping & Code Fetching Tool

This document describes the web scraping and code fetching capabilities in PedroCLI, which enables agents to fetch current information from the web, pull code examples from public repositories, and search for solutions on Q&A sites.

## Overview

The web scraping tool provides agents with the ability to:

- Fetch and extract content from any URL
- Search the web using DuckDuckGo or SearXNG
- Fetch files and directories from GitHub repositories
- Fetch files from GitLab repositories (including self-hosted instances)
- Search and fetch Stack Overflow questions and answers
- Extract code blocks from web pages

This is particularly useful when:
- Self-hosted models have knowledge cutoffs and can't browse
- Agents need access to current documentation or API references
- Code examples from GitHub/GitLab/Stack Overflow are needed for coding tasks

## Architecture

### Package Structure

```
pkg/webscrape/
├── types.go          # Core type definitions
├── fetcher.go        # HTTP fetcher with caching and rate limiting
├── extractor.go      # HTML content extraction (text, code blocks)
├── cache.go          # Caching layer (SQLite or in-memory)
├── ratelimit.go      # Per-domain rate limiting
├── search.go         # Search engine integrations
└── handlers/
    ├── github.go     # GitHub API handler
    ├── gitlab.go     # GitLab API handler
    └── stackoverflow.go  # Stack Exchange API handler

pkg/tools/
└── webscrape.go      # MCP tool wrapper
```

### Component Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     WebScrapeTool (MCP)                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────────┐   │
│  │   Fetcher   │  │   Search    │  │  Site Handlers    │   │
│  │ (HTTP/HTTPS)│  │  Engines    │  │ GitHub/GitLab/SO  │   │
│  └──────┬──────┘  └──────┬──────┘  └─────────┬─────────┘   │
│         │                │                   │              │
│  ┌──────┴──────┐  ┌──────┴──────┐           │              │
│  │  Extractor  │  │ DuckDuckGo  │           │              │
│  │ (HTML→Text) │  │   SearXNG   │           │              │
│  └─────────────┘  └─────────────┘           │              │
│                                             │              │
│  ┌───────────────────────────────────────────┐             │
│  │         Rate Limiter (per-domain)         │             │
│  └───────────────────────────────────────────┘             │
│                                                             │
│  ┌───────────────────────────────────────────┐             │
│  │     Cache (SQLite or In-Memory)           │             │
│  └───────────────────────────────────────────┘             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## MCP Tool Actions

The `web_scrape` tool exposes the following actions:

### Generic Fetching

#### `fetch_url`
Fetch content from any URL, optionally extracting clean text and code blocks.

```json
{
  "action": "fetch_url",
  "url": "https://example.com/page",
  "extract_text": true,
  "extract_code": true
}
```

#### `extract_code_from_url`
Extract all code blocks from a webpage, optionally filtered by language.

```json
{
  "action": "extract_code_from_url",
  "url": "https://example.com/tutorial",
  "language": "python"
}
```

### Web Search

#### `search_web`
Search the web for information using DuckDuckGo or SearXNG.

```json
{
  "action": "search_web",
  "query": "golang http client example",
  "max_results": 10,
  "site": "stackoverflow.com"
}
```

### GitHub

#### `fetch_github_file`
Fetch a specific file from a GitHub repository.

```json
{
  "action": "fetch_github_file",
  "owner": "golang",
  "repo": "go",
  "path": "src/net/http/client.go",
  "ref": "master"
}
```

#### `fetch_github_readme`
Fetch the README from a GitHub repository.

```json
{
  "action": "fetch_github_readme",
  "owner": "golang",
  "repo": "go"
}
```

#### `fetch_github_directory`
List files in a GitHub repository directory.

```json
{
  "action": "fetch_github_directory",
  "owner": "golang",
  "repo": "go",
  "path": "src/net/http",
  "ref": "master"
}
```

#### `search_github_code`
Search for code on GitHub.

```json
{
  "action": "search_github_code",
  "query": "func ServeHTTP",
  "language": "go",
  "repo": "golang/go"
}
```

### GitLab

#### `fetch_gitlab_file`
Fetch a file from a GitLab repository.

```json
{
  "action": "fetch_gitlab_file",
  "project": "gitlab-org/gitlab",
  "path": "README.md",
  "ref": "main"
}
```

#### `fetch_gitlab_readme`
Fetch the README from a GitLab repository.

```json
{
  "action": "fetch_gitlab_readme",
  "project": "gitlab-org/gitlab"
}
```

### Stack Overflow

#### `fetch_stackoverflow_question`
Fetch a Stack Overflow question with its answers.

```json
{
  "action": "fetch_stackoverflow_question",
  "question_id": 12345678,
  "include_answers": true,
  "max_answers": 5
}
```

#### `search_stackoverflow`
Search Stack Overflow for questions.

```json
{
  "action": "search_stackoverflow",
  "query": "golang http client timeout",
  "tags": "go,http",
  "sort": "relevance",
  "max_results": 10
}
```

## Configuration

Add the following to your `.pedrocli.json`:

```json
{
  "web_scraping": {
    "enabled": true,
    "user_agent": "PedroCLI/1.0 (Web Scraping Tool)",
    "timeout_seconds": 30,
    "max_size_mb": 10,

    "cache_enabled": true,
    "cache_type": "memory",
    "cache_ttl_hours": 1,
    "cache_max_size_mb": 100,

    "search_engine": "duckduckgo",
    "searxng_url": "",

    "github_token": "",
    "gitlab_token": "",
    "gitlab_url": "https://gitlab.com",
    "stackoverflow_key": ""
  }
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `false` | Enable/disable web scraping |
| `user_agent` | `PedroCLI/1.0` | HTTP User-Agent header |
| `timeout_seconds` | `30` | HTTP request timeout |
| `max_size_mb` | `10` | Maximum response size in MB |
| `cache_enabled` | `true` | Enable response caching |
| `cache_type` | `memory` | Cache type: `memory` or `sqlite` |
| `cache_ttl_hours` | `1` | Default cache TTL |
| `cache_max_size_mb` | `100` | Maximum cache size |
| `search_engine` | `duckduckgo` | Search engine: `duckduckgo` or `searxng` |
| `searxng_url` | `` | SearXNG instance URL (if using) |
| `github_token` | `` | GitHub API token (optional) |
| `gitlab_token` | `` | GitLab API token (optional) |
| `gitlab_url` | `https://gitlab.com` | GitLab instance URL |
| `stackoverflow_key` | `` | Stack Exchange API key (optional) |

### Environment Variables

API tokens can also be set via environment variables:

- `GITHUB_TOKEN` - GitHub API token
- `GITLAB_TOKEN` - GitLab API token
- `SO_API_KEY` - Stack Exchange API key

## Rate Limiting

The tool implements per-domain rate limiting to be respectful to external services:

| Domain | Default Rate (req/s) |
|--------|---------------------|
| `github.com` | 10 |
| `api.github.com` | 30 (with token) |
| `gitlab.com` | 10 |
| `stackoverflow.com` | 30 |
| Default | 2 |

Rate limits can be customized via configuration:

```json
{
  "web_scraping": {
    "rate_limits": {
      "github.com": 15,
      "my-gitlab.example.com": 20,
      "default": 5
    }
  }
}
```

## Caching

The cache layer helps reduce API calls and improve response times:

### In-Memory Cache (Default)
- Fast access
- Lost on restart
- Configurable max size

### SQLite Cache
- Persistent across restarts
- Automatic cleanup of expired entries
- Good for larger caches

Configure SQLite cache:

```json
{
  "web_scraping": {
    "cache_type": "sqlite",
    "cache_path": "/var/pedro/cache/webscrape.db"
  }
}
```

## Content Extraction

### HTML to Clean Text

The extractor removes:
- `<script>` and `<style>` tags
- Navigation, header, footer elements
- HTML comments
- Converts block elements to newlines
- Decodes HTML entities

### Code Block Extraction

Detects and extracts code from:
- `<pre><code>` blocks with language classes
- Standalone `<pre>` blocks
- Fenced markdown code blocks (\`\`\`language)

Language detection is performed based on:
- HTML class attributes (e.g., `language-python`)
- Heuristic analysis of code content

## Error Handling

The tool returns structured errors with suggestions for agents:

| Error Type | Description | Suggestion |
|------------|-------------|------------|
| `rate_limited` | API rate limit exceeded | Wait before retrying |
| `not_found` | Resource not found | Verify URL/path |
| `access_denied` | Authentication required | Check token/permissions |
| `timeout` | Request timed out | Retry or increase timeout |
| `invalid_url` | Malformed URL | Check URL format |
| `parse_failure` | Content parsing failed | Page format may be unsupported |
| `network_error` | Network connectivity issue | Check connectivity |

## Response Format

All responses are formatted as JSON for easy agent consumption:

```json
{
  "success": true,
  "source": "https://example.com/page",
  "content_type": "mixed",
  "summary": "Page Title - Brief description",
  "content": "Clean text content...",
  "code_blocks": [
    {
      "language": "python",
      "code": "def hello(): ...",
      "description": "Example function"
    }
  ],
  "links": [
    {
      "text": "Documentation",
      "url": "https://docs.example.com",
      "type": "documentation"
    }
  ],
  "metadata": {
    "title": "Page Title",
    "fetched_at": "2024-01-15T10:30:00Z"
  }
}
```

## Security Considerations

1. **Token Security**: API tokens are stored in configuration, not exposed to LLMs
2. **Rate Limiting**: Prevents abuse of external APIs
3. **Size Limits**: Maximum response size prevents memory exhaustion
4. **URL Validation**: Only HTTP/HTTPS URLs are allowed
5. **Timeout Limits**: Prevents hanging on slow responses

## Best Practices

1. **Use Caching**: Enable caching to reduce API calls
2. **Set API Tokens**: Tokens increase rate limits significantly
3. **Prefer Site-Specific Actions**: Use `fetch_github_file` over `fetch_url` for GitHub
4. **Limit Results**: Set appropriate `max_results` to reduce response size
5. **Filter by Language**: When extracting code, specify the language filter

## Troubleshooting

### Rate Limit Errors
- Enable caching to reduce repeated requests
- Add API tokens for higher limits
- Increase wait time between requests

### Content Not Extracted
- Some pages may use JavaScript rendering (not supported)
- Check if the page is actually accessible
- Try the site-specific handler if available

### Cache Issues
- Clear cache by deleting the SQLite file or restarting (memory cache)
- Check cache TTL settings
- Verify cache directory permissions (SQLite)

## Future Enhancements

TODO items for future development:

- [ ] Google Custom Search integration (requires API key)
- [ ] JavaScript rendering support (headless browser)
- [ ] Additional site handlers (Bitbucket, SourceHut)
- [ ] Automatic language detection improvement
- [ ] PDF and documentation site parsing
- [ ] WebSocket support for real-time pages
