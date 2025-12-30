# Podcast CLI Tools - How-To Guide

This guide shows how to use the PedroCLI podcast tools to manage podcast production workflows with Notion integration.

## Prerequisites

### 1. Notion Setup

You'll need:
- A Notion workspace
- An internal integration token (starts with `ntn_`)
- Database IDs for your podcast databases

#### Getting Your Notion Token

1. Go to https://www.notion.so/my-integrations
2. Create a new internal integration
3. Copy the "Internal Integration Token" (starts with `ntn_`)
4. Share your databases with the integration:
   - Open each database in Notion
   - Click "..." → "Connections" → Add your integration

#### Getting Database IDs

Database IDs are in the URL when viewing a database in Notion:
```
https://www.notion.so/workspace/<DATABASE_ID>?v=...
```

The DATABASE_ID is a 32-character hex string (may contain hyphens).

### 2. Configuration

Add your Notion configuration to `.pedrocli.json`:

```json
{
  "podcast": {
    "enabled": true,
    "model_profile": "content",
    "notion": {
      "enabled": true,
      "databases": {
        "scripts": "YOUR_EPISODE_PLANNER_DB_ID",
        "articles_review": "YOUR_ARTICLES_DB_ID",
        "news_review": "YOUR_NEWS_DB_ID",
        "guests": "YOUR_GUESTS_DB_ID"
      }
    }
  }
}
```

### 3. Store Notion Token

Store your Notion token securely using one of these methods:

```bash
# Option 1: Environment variable (supports 1Password CLI)
export NOTION_TOKEN="your_token_here"
# Or with op run:
op run --env-file=.env -- ./pedrocli-http-server

# Option 2: Config file
# Add to .pedrocli.json: "podcast.notion.api_key": "your_token_here"

# Option 3: Token storage system (for OAuth flows)
# pedrocli will prompt for tokens when needed
```

## Commands

### Create a Podcast Script & Outline

Create a structured episode script and outline for your podcast:

```bash
./pedrocli podcast create-script \
  -topic "Getting Started with Local LLMs" \
  -notes "Cover hardware requirements, choosing models, first steps with Ollama"
```

**What it does:**
- Generates a structured script using the episode template (see `templates/podcast_episode_template.md`)
- Creates both a local markdown file and a Notion database entry
- Template includes: Episode Details, Segment Outline (Intro, News, Sponsor, Main, Reflection, Outro)
- Creates a page in your Notion "Episode Planner" database
- Sets properties: Episode #, Title/Working Topic, Status 🎛 (Draft), Notes

**Expected Output:**
```
Creating podcast script for: Getting Started with Local LLMs

Starting create_podcast_script job...
Job job-XXXXXXXXXX started and running in background.

⏳ Job job-XXXXXXXXXX is running...
✓ Job completed successfully!

Page URL: https://notion.so/...
```

### Add a Link to Review Database

Add articles or news links to your Notion database for review:

```bash
./pedrocli podcast add-link \
  -url "https://llama-cpp-python.readthedocs.io/en/stable/changelog/" \
  -title "llama-cpp-python Changelog" \
  -notes "Latest updates - useful for self-hosted AI discussions"
```

**What it does:**
- Analyzes the URL to determine if it's news or an article
- Creates a page in the appropriate Notion database
- Sets Status 🎛 to "To Review"

### Add a Guest

Add guest information to your podcast guests database:

```bash
./pedrocli podcast add-guest \
  -name "James Mumford" \
  -bio "Tech expert specializing in cloud infrastructure" \
  -email "guest@example.com"
```

**What it does:**
- Creates a new page in the Guests database
- Fills in name, bio, email, and other provided fields
- Sets status to "Potential"

### Review News Items

Summarize recent news for episode prep:

```bash
./pedrocli podcast review-news \
  -focus "AI" \
  -max-items 5
```

## Database Schema

### Episode Planner Database

The "Episode Planner" (aka "scripts") database should have these properties:

| Property Name | Type | Description |
|--------------|------|-------------|
| Episode # | Title | Episode number/identifier (e.g., "S01E01") |
| Title / Working Topic | Rich Text | Episode topic or working title |
| Status 🎛 | Rich Text | Current status (Draft, Recording, Published, etc.) |
| Hosts | Rich Text | Host names |
| Guests | Relation | Links to Guest Directory |
| Recording Date | Rich Text | When episode will be recorded |
| Publish Date | Rich Text | When episode will be published |
| Notes | Rich Text | Additional context or full script |
| New For the week | Relation | Links to news/articles |

**Note**: The Status property includes an emoji (🎛). Make sure to copy it exactly when setting up your database.

## Troubleshooting

### Common Issues

**Problem**: "Notion API key not configured"
- **Solution**: Set NOTION_TOKEN env var, add podcast.notion.api_key to config, or use token storage

**Problem**: "Could not find property with name or id: Status"
- **Solution**: The property is called "Status 🎛" (with emoji). Ensure your database has this exact property name.

**Problem**: "validation_error" from Notion API
- **Solution**: Check that your database has all required properties with exact names (case-sensitive)

### Checking Job Output

All agent runs create job directories at `/tmp/pedroceli-jobs/job-XXXXX-TIMESTAMP/`:

```bash
# List recent jobs
ls -lt /tmp/pedroceli-jobs/ | head -5

# Check specific job output
cat /tmp/pedroceli-jobs/job-XXXXX-TIMESTAMP/*-tool-results.json
```

Look for:
- Tool calls in `*-tool-calls.json` files
- Results in `*-tool-results.json` files
- Agent responses in `*-response.txt` files

### Verifying Notion Integration

Test your Notion connection directly:

```bash
# Query a database
curl -X POST "https://api.notion.com/v1/databases/<DB_ID>/query" \
  -H "Authorization: Bearer $NOTION_TOKEN" \
  -H "Notion-Version: 2022-06-28" \
  -H "Content-Type: application/json"
```

## Tips

1. **Start Simple**: Test with one database at a time
2. **Check Page URLs**: After creating pages, the agent returns the Notion URL - click it to verify
3. **Iterate**: The agent will try multiple approaches if the first one fails
4. **Monitor Jobs**: Use the job directory to debug if something goes wrong

## Advanced: Custom Workflows

You can chain commands together:

```bash
# Create a script and add related links
./pedrocli podcast create-script -topic "Topic here" -notes "..."
./pedrocli podcast add-link -url "https://..." -notes "Related article"
./pedrocli podcast add-link -url "https://..." -notes "Another reference"
```

## Need Help?

- Check `/tmp/pedroceli-jobs/` for detailed execution logs
- Verify your Notion token is valid and databases are shared with the integration
- Ensure database property names match exactly (including emojis!)
