# Blog Orchestrator

The Blog Orchestrator is a multi-phase agent that handles complex, multi-step blog prompts. It transforms dictated prompts into polished, publishable blog posts through research, outlining, and content generation.

## Overview

The Blog Orchestrator is designed for complex blog prompts like:
- "Write a 2025 year-in-review post with links to my events, previous blog posts, and upcoming meetups"
- "Create a recap of my conference talks this year with my community links"
- "Write a retrospective with newsletter section and social media posts"

## Multi-Phase Pipeline

The orchestrator runs a 6-phase pipeline:

### Phase 1: Analyze Prompt
Parses the user's complex prompt to identify:
- Main topic and key themes
- Content sections needed
- Research tasks (calendar, RSS, static links)
- Whether newsletter should be included
- Estimated word count

### Phase 2: Research
Executes identified research tasks:
- **Calendar**: Fetches upcoming events from Google Calendar
- **RSS Feed**: Gets previous blog posts from Substack RSS feed
- **Static Links**: Retrieves configured social/community links

### Phase 3: Generate Outline
Creates a detailed outline incorporating research data:
- Section headers with key points
- Where to use research data
- Word count per section

### Phase 4: Expand Sections
Expands each section independently to handle large content:
- Writes full content per section
- Maintains narrative flow
- Incorporates research data
- Preserves author's voice

### Phase 5: Assemble Final Post
Combines all sections into a cohesive post:
- Strong opening hook
- Smooth transitions
- Newsletter section (if requested)
- Social media posts

### Phase 6: Publish (Optional)
Auto-publishes to Notion with all metadata.

## Configuration

Add to `.pedrocli.json`:

```json
{
  "blog": {
    "enabled": true,
    "rss_feed_url": "https://soypetetech.substack.com/feed",
    "research": {
      "enabled": true,
      "calendar_enabled": true,
      "rss_enabled": true,
      "max_rss_posts": 5,
      "max_calendar_days": 30
    },
    "static_links": {
      "discord": "https://discord.gg/soypete",
      "linktree": "https://linktr.ee/soypete_tech",
      "youtube": "https://youtube.com/@soypete",
      "twitter": "https://twitter.com/soypete",
      "bluesky": "https://bsky.app/soypete",
      "linkedin": "https://linkedin.com/in/soypete",
      "newsletter": "https://soypetetech.substack.com",
      "youtube_placeholder": "Latest Video: [ADD LINK BEFORE SUBSTACK PUBLISH]",
      "custom_links": [
        {"name": "GitHub", "url": "https://github.com/soypete"}
      ]
    }
  }
}
```

## API Usage

### HTTP Endpoint

```bash
POST /api/blog/orchestrate
Content-Type: application/json

{
  "title": "Optional initial title",
  "prompt": "Write a 2025 year-in-review blog post...",
  "publish": true
}
```

Response:
```json
{
  "success": true,
  "message": "Blog orchestration job started. Poll /api/jobs/{job_id} for status.",
  "job_id": "job-1234567890"
}
```

### MCP Tool Call

```json
{
  "tool": "blog_orchestrator",
  "args": {
    "prompt": "Write a 2025 year-in-review...",
    "title": "Optional title",
    "publish": true
  }
}
```

## Research Tools

### RSS Feed Tool

Fetches previous blog posts from RSS/Atom feeds.

Actions:
- `get_configured`: Fetch from configured feed URL
- `fetch`: Fetch from a specific URL

```json
{"tool": "rss_feed", "args": {"action": "get_configured", "limit": 5}}
{"tool": "rss_feed", "args": {"action": "fetch", "url": "https://example.com/feed.xml"}}
```

### Static Links Tool

Returns configured social/community links.

Actions:
- `get_all`: Return all configured links
- `get_social`: Return social media links only
- `get_custom`: Return custom links only
- `get_youtube_placeholder`: Return YouTube video placeholder text

```json
{"tool": "static_links", "args": {"action": "get_all"}}
{"tool": "static_links", "args": {"action": "get_youtube_placeholder"}}
```

## Newsletter Template

The newsletter template auto-fills with research data:

```markdown
## Newsletter Highlights

### Featured Video
Latest Video: [ADD LINK BEFORE SUBSTACK PUBLISH]

### Upcoming Events
- Event 1 from calendar
- Event 2 from calendar

### Recent Posts You Might Have Missed
- Post 1 from RSS feed
- Post 2 from RSS feed

### Stay Connected
- [Join our Discord](link)
- [Subscribe on YouTube](link)
- [Follow on Twitter/X](link)
- [All links](linktree)
```

## Brand Voice Guidelines

The orchestrator follows Soypete Tech brand guidelines:
- **No emojis** unless specifically instructed
- **Preserve impactful sentences** from dictation without editing
- **Educational focus** - help people learn and take action
- **Conversational tone** - teaching a friend, not lecturing students
- **Include code examples** where relevant

## Output Structure

The orchestrator returns:

```json
{
  "analysis": {
    "main_topic": "2025 Year Review",
    "content_sections": [...],
    "research_tasks": [...],
    "include_newsletter": true,
    "estimated_word_count": 1500
  },
  "research_data": {
    "calendar": [...],
    "rss_feed": [...],
    "static_links": {...}
  },
  "outline": "## Section 1\n...",
  "expanded_draft": "Full blog content...",
  "newsletter": "Newsletter section...",
  "full_content": "Complete post with newsletter...",
  "social_posts": {
    "twitter_post": "Under 280 chars...",
    "linkedin_post": "2-3 paragraphs...",
    "bluesky_post": "Under 300 chars..."
  },
  "suggested_title": "2025: A Year of Growth"
}
```

## Error Handling

If research fails:
- Notes what couldn't be fetched
- Provides placeholders for missing data
- Continues with available information
- Lets user know what needs manual addition

## Testing

Run the orchestrator tests:

```bash
# Unit tests
go test ./pkg/agents/ -run "TestBlogOrchestrator"
go test ./pkg/tools/ -run "TestRSSFeed|TestStaticLinks"

# Integration tests (requires network)
go test ./pkg/agents/ -run "TestRSSFeedTool_RealFeed"
```
