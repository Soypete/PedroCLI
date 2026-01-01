# ADR-006: Optional Tool Catalog

## Status

Proposed

## Context

The dynamic workflow architecture (ADRs 001-005) enables LLMs to selectively invoke tools. This ADR defines the catalog of optional tools, their purposes, and guidelines for when agents should use them.

### Current Optional Tools

| Tool | Location | Purpose |
|------|----------|---------|
| rss_feed | `pkg/tools/rss.go` | Fetch posts from RSS/Atom feeds |
| calendar | `pkg/tools/calendar.go` | Get events from Google Calendar |
| static_links | `pkg/tools/static_links.go` | Get configured social/community links |
| web_search | (proposed) | Search the web for current information |
| github_api | (proposed) | Interact with GitHub repositories |

### Missing Capabilities

1. **Web Search**: No way to get current information beyond training data.
2. **GitHub Integration**: No way to fetch code from external repos.
3. **Link Verification**: No way to validate URLs are accessible.
4. **Citation Management**: No structured way to track references.

## Decision

### 1. Optional Tool Categories

Define categories for optional tools:

| Category | Purpose | Example Tools |
|----------|---------|---------------|
| `research` | Gather external information | web_search, rss_feed, calendar |
| `reference` | Add citations and links | static_links, github_api, link_validator |
| `enhance` | Improve content quality | grammar_check, readability_analyzer |
| `publish` | Distribute content | blog_notion, twitter_post, linkedin_post |
| `utility` | General-purpose helpers | url_shortener, image_optimizer |

### 2. Tool Catalog

#### 2.1 Web Search Tool (New)

```go
// pkg/tools/websearch.go

type WebSearchTool struct {
    client *search.Client  // DuckDuckGo, Google, etc.
    config *config.Config
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
    return "Search the web for current information, news, documentation, or facts"
}

func (t *WebSearchTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "research",
        Optionality: tools.ToolOptional,
        UsageHint: `Use when you need:
- Current information (dates, events, news)
- Technical documentation or tutorials
- Facts that may have changed since training
- Verification of claims or statements`,
        Produces: []string{"search_results", "citations"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "query": {
                    Type:        "string",
                    Description: "Search query",
                },
                "max_results": {
                    Type:        "integer",
                    Description: "Maximum results to return (default: 5)",
                },
                "recency": {
                    Type:        "string",
                    Enum:        []interface{}{"day", "week", "month", "year", "any"},
                    Description: "Filter by recency (default: any)",
                },
            },
            Required: []string{"query"},
        },
        Examples: []tools.ToolExample{
            {
                Description: "Search for recent Go 1.23 features",
                Input: map[string]interface{}{
                    "query":   "Go 1.23 new features changelog",
                    "recency": "month",
                },
            },
        },
    }
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
    query := args["query"].(string)
    maxResults := 5
    if mr, ok := args["max_results"].(float64); ok {
        maxResults = int(mr)
    }

    results, err := t.client.Search(ctx, query, maxResults)
    if err != nil {
        return tools.ErrorResult(fmt.Sprintf("search failed: %v", err)), nil
    }

    // Format results with citations
    var output strings.Builder
    citations := make([]map[string]string, 0)

    for i, result := range results {
        output.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, result.Title))
        output.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
        output.WriteString(fmt.Sprintf("   Source: %s\n\n", result.URL))

        citations = append(citations, map[string]string{
            "title": result.Title,
            "url":   result.URL,
        })
    }

    return &tools.Result{
        Success: true,
        Output:  output.String(),
        Data: map[string]interface{}{
            "results":   results,
            "citations": citations,
        },
    }, nil
}
```

#### 2.2 GitHub API Tool (New)

```go
// pkg/tools/github.go

type GitHubAPITool struct {
    client *github.Client
    config *config.Config
}

func (t *GitHubAPITool) Name() string { return "github_api" }

func (t *GitHubAPITool) Description() string {
    return "Interact with GitHub: fetch code, issues, pull requests, and repository information"
}

func (t *GitHubAPITool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "reference",
        Optionality: tools.ToolOptional,
        UsageHint: `Use when you need:
- Example code from public repositories
- Documentation or README content
- Issue or PR context for discussions
- Library version or release information`,
        RequiresCapabilities: []string{"github_token"},
        Produces:             []string{"github_content", "citations"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "action": {
                    Type: "string",
                    Enum: []interface{}{
                        "get_file",
                        "get_readme",
                        "list_releases",
                        "get_issue",
                        "search_code",
                    },
                    Description: "The GitHub operation to perform",
                },
                "owner": {
                    Type:        "string",
                    Description: "Repository owner (username or org)",
                },
                "repo": {
                    Type:        "string",
                    Description: "Repository name",
                },
                "path": {
                    Type:        "string",
                    Description: "File path (for get_file)",
                },
                "issue_number": {
                    Type:        "integer",
                    Description: "Issue or PR number (for get_issue)",
                },
                "query": {
                    Type:        "string",
                    Description: "Search query (for search_code)",
                },
            },
            Required: []string{"action"},
        },
        Examples: []tools.ToolExample{
            {
                Description: "Get README from a repository",
                Input: map[string]interface{}{
                    "action": "get_readme",
                    "owner":  "anthropics",
                    "repo":   "claude-code",
                },
            },
            {
                Description: "Search for example usage of a function",
                Input: map[string]interface{}{
                    "action": "search_code",
                    "query":  "NewToolRegistry language:go",
                },
            },
        },
    }
}
```

#### 2.3 Link Validator Tool (New)

```go
// pkg/tools/linkvalidator.go

type LinkValidatorTool struct {
    client *http.Client
}

func (t *LinkValidatorTool) Name() string { return "link_validator" }

func (t *LinkValidatorTool) Description() string {
    return "Validate that URLs are accessible and return expected content types"
}

func (t *LinkValidatorTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "reference",
        Optionality: tools.ToolOptional,
        UsageHint: `Use before including links in content to ensure:
- URLs are accessible (not 404)
- Content type matches expectations
- Redirects lead to intended destination`,
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "urls": {
                    Type: "array",
                    Items: &logits.JSONSchema{
                        Type: "string",
                    },
                    Description: "List of URLs to validate",
                },
            },
            Required: []string{"urls"},
        },
    }
}
```

#### 2.4 Enhanced RSS Feed Tool

Update existing tool with better metadata:

```go
// pkg/tools/rss.go (updated)

func (t *RSSFeedTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "research",
        Optionality: tools.ToolOptional,
        UsageHint: `Use when content benefits from:
- References to recent blog posts
- Newsletter section with recent content
- "You might have missed" sections
- Context about what author has covered`,
        Produces: []string{"feed_items", "citations"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "action": {
                    Type:        "string",
                    Enum:        []interface{}{"get_configured", "fetch"},
                    Description: "Use get_configured for author's feed, fetch for custom URL",
                },
                "url": {
                    Type:        "string",
                    Description: "RSS/Atom feed URL (for fetch action)",
                },
                "limit": {
                    Type:        "integer",
                    Description: "Maximum items to return (default: 5)",
                },
            },
            Required: []string{"action"},
        },
        Examples: []tools.ToolExample{
            {
                Description: "Get recent posts from configured blog",
                Input: map[string]interface{}{
                    "action": "get_configured",
                    "limit":  3,
                },
            },
        },
    }
}
```

#### 2.5 Enhanced Calendar Tool

```go
// pkg/tools/calendar.go (updated)

func (t *CalendarTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "research",
        Optionality: tools.ToolOptional,
        UsageHint: `Use when content should include:
- Upcoming events or deadlines
- Conference/meetup mentions
- "What's happening" sections
- Time-sensitive information`,
        RequiresCapabilities: []string{"google_calendar"},
        Produces:             []string{"events"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "action": {
                    Type:        "string",
                    Enum:        []interface{}{"list_events"},
                    Description: "Calendar operation",
                },
                "time_min": {
                    Type:        "string",
                    Description: "Start of time range (RFC3339, default: now)",
                },
                "time_max": {
                    Type:        "string",
                    Description: "End of time range (RFC3339, default: 30 days from now)",
                },
                "max_results": {
                    Type:        "integer",
                    Description: "Maximum events to return (default: 10)",
                },
            },
            Required: []string{"action"},
        },
    }
}
```

### 3. Usage Guidelines

Encode guidelines in tool metadata and prompts:

```go
// pkg/tools/guidelines.go

var OptionalToolGuidelines = map[string]string{
    "web_search": `
## When to Use web_search

DO use when:
- Content mentions current events or recent dates
- Technical facts that may have changed (version numbers, API changes)
- Verifying claims or statistics
- Finding official documentation

DO NOT use when:
- Information is in user's prompt
- Topic is well-established and unlikely to have changed
- You're confident in your knowledge
- Simple creative writing tasks
`,

    "github_api": `
## When to Use github_api

DO use when:
- User asks about specific repository or project
- You need example code from a known library
- Citing open source project documentation
- Discussing specific issues or PRs

DO NOT use when:
- Writing general code (use your knowledge)
- Repository is likely private
- You already have sufficient context
`,

    "rss_feed": `
## When to Use rss_feed

DO use when:
- Content would benefit from "recent posts" section
- Building newsletter-style content
- Showing continuity with previous content
- User mentions wanting to reference past work

DO NOT use when:
- Content is standalone without newsletter
- User explicitly wants fresh, unreferenced content
- The feed isn't relevant to the topic
`,

    "calendar": `
## When to Use calendar

DO use when:
- Content should include upcoming events
- Writing about schedules or deadlines
- Newsletter section about "what's coming"
- User mentions events or conferences

DO NOT use when:
- Content is timeless/evergreen
- Events aren't relevant to the topic
- User wants just the core content
`,
}
```

### 4. Capability Detection

Determine which optional tools are available:

```go
// pkg/tools/capabilities.go

type CapabilityChecker struct {
    config *config.Config
}

func (c *CapabilityChecker) CheckCapabilities() map[string]bool {
    caps := make(map[string]bool)

    // Check for API keys/tokens
    caps["github_token"] = os.Getenv("GITHUB_TOKEN") != ""
    caps["google_calendar"] = c.config.Calendar.Enabled && c.config.Calendar.CredentialsPath != ""
    caps["notion"] = os.Getenv("NOTION_TOKEN") != ""
    caps["web_search"] = true  // Uses DuckDuckGo, no API key needed

    return caps
}

func (c *CapabilityChecker) FilterAvailableTools(tools []tools.ExtendedTool) []tools.ExtendedTool {
    caps := c.CheckCapabilities()
    var available []tools.ExtendedTool

    for _, tool := range tools {
        meta := tool.Metadata()
        if meta == nil {
            available = append(available, tool)
            continue
        }

        // Check if all required capabilities are present
        allMet := true
        for _, req := range meta.RequiresCapabilities {
            if !caps[req] {
                allMet = false
                break
            }
        }

        if allMet {
            available = append(available, tool)
        }
    }

    return available
}
```

### 5. Tool Discovery Prompt

Generate guidance for available optional tools:

```go
// pkg/prompts/optional_tools.go

func GenerateOptionalToolsSection(tools []tools.ExtendedTool) string {
    // Group by category
    byCategory := make(map[string][]tools.ExtendedTool)
    for _, tool := range tools {
        meta := tool.Metadata()
        if meta != nil && meta.Optionality == tools.ToolOptional {
            cat := meta.Category
            byCategory[cat] = append(byCategory[cat], tool)
        }
    }

    var sb strings.Builder
    sb.WriteString("# Optional Tools\n\n")
    sb.WriteString("These tools can enhance your output. Use them when relevant.\n\n")

    for category, catTools := range byCategory {
        sb.WriteString(fmt.Sprintf("## %s Tools\n\n", strings.Title(category)))

        for _, tool := range catTools {
            meta := tool.Metadata()
            sb.WriteString(fmt.Sprintf("### %s\n", tool.Name()))
            sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description()))
            sb.WriteString(fmt.Sprintf("**When to use:** %s\n\n", meta.UsageHint))
        }
    }

    return sb.String()
}
```

### 6. Future Tool Extension Pattern

Define how to add new optional tools:

```go
// pkg/tools/extension.go

// ToolExtension represents a pluggable tool
type ToolExtension struct {
    Tool             ExtendedTool
    RequiredPackages []string
    SetupInstructions string
    ConfigFields     []ConfigField
}

type ConfigField struct {
    Name        string
    Type        string
    Description string
    EnvVar      string
    Required    bool
}

// Example: Twitter posting tool extension
var TwitterToolExtension = &ToolExtension{
    Tool: &TwitterPostTool{},
    RequiredPackages: []string{"github.com/dghubble/go-twitter"},
    SetupInstructions: `
1. Create a Twitter Developer account
2. Create an app and get API keys
3. Set TWITTER_API_KEY and TWITTER_API_SECRET
4. Enable in config: twitter.enabled = true
`,
    ConfigFields: []ConfigField{
        {Name: "enabled", Type: "bool", Description: "Enable Twitter posting"},
        {Name: "api_key", Type: "string", EnvVar: "TWITTER_API_KEY", Required: true},
        {Name: "api_secret", Type: "string", EnvVar: "TWITTER_API_SECRET", Required: true},
    },
}
```

## Consequences

### Positive

1. **Rich Capabilities**: Agents can enhance output with current information.

2. **Graceful Degradation**: Missing capabilities don't break core functionality.

3. **Clear Guidelines**: LLM understands when to use each tool.

4. **Extensibility**: New tools follow established pattern.

5. **User Control**: Users configure which tools are available.

### Negative

1. **External Dependencies**: Web search, GitHub require network access.

2. **API Costs**: Some services may have rate limits or costs.

3. **Credential Management**: Users must manage API keys.

4. **Latency**: External tool calls add execution time.

### Mitigation

1. **Caching**: Cache external results where appropriate.

2. **Fallbacks**: Tool failures don't fail the task.

3. **Rate Limiting**: Built-in rate limiting for external APIs.

4. **Clear Setup**: Tool extensions include setup instructions.

## Implementation

### Phase 1: Core Optional Tools

1. Implement web_search using DuckDuckGo (no API key)
2. Update rss_feed and calendar with enhanced metadata
3. Implement link_validator

### Phase 2: GitHub Integration

1. Implement github_api tool
2. Add capability detection for GitHub token
3. Document setup process

### Phase 3: Guidelines Integration

1. Add usage guidelines to prompts
2. Update tool awareness protocol (ADR-002)
3. Test with various request types

### Phase 4: Extension System

1. Define ToolExtension interface
2. Create example extensions (Twitter, LinkedIn)
3. Document extension development

## Related ADRs

- **ADR-001**: Dynamic Tool Registry (tool registration)
- **ADR-002**: LLM Tool Awareness Protocol (prompt generation)
- **ADR-003**: Dynamic Tool Invocation (execution)
- **ADR-005**: Agent Workflow Refactoring (agent integration)
