# Podcast Tools - Manual Testing Guide

This guide walks through testing the podcast tools integration with Notion and Calendar.

## Prerequisites

### 1. Notion Setup

**Get Your Notion API Key:**
1. Go to https://www.notion.so/my-integrations
2. Create a new integration (or use existing)
3. Copy the "Internal Integration Token" (starts with `secret_`)
4. Share your Notion databases with the integration

**Get Database IDs:**
1. Open your Notion database in the browser
2. Copy the database ID from the URL:
   ```
   https://www.notion.so/YOUR_WORKSPACE/DATABASE_ID?v=...
                                         ^^^^^^^^^^^^^^^^
   ```

### 2. Build PedroCLI

```bash
cd /path/to/pedrocli
make build
./pedrocli --version
```

### 3. Configuration File

Create `.pedrocli.json` in your project directory:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "temperature": 0.2
  },
  "repo_storage": {
    "database_path": "~/.pedrocli/pedrocli.db"
  },
  "podcast": {
    "enabled": true,
    "model_profile": "content",
    "notion": {
      "enabled": true,
      "command": "npx -y @modelcontextprotocol/server-notion",
      "databases": {
        "scripts": "YOUR_SCRIPTS_DATABASE_ID",
        "potential_article": "YOUR_ARTICLES_DATABASE_ID",
        "articles_review": "YOUR_REVIEW_DATABASE_ID",
        "news_review": "YOUR_NEWS_DATABASE_ID",
        "guests": "YOUR_GUESTS_DATABASE_ID"
      }
    },
    "calendar": {
      "enabled": false,
      "command": "npx -y @modelcontextprotocol/server-google-calendar",
      "credentials_path": ""
    },
    "metadata": {
      "name": "Your Podcast Name",
      "description": "Your podcast description",
      "cohosts": ["Host 1", "Host 2"],
      "recording_platform": "Riverside/Zoom/etc"
    }
  }
}
```

## Testing Workflow

### Step 1: Store Notion API Key

Use the setup script to store your Notion API key securely:

```bash
go run scripts/setup-podcast-tokens.go \
  -provider notion \
  -service database \
  -api-key "secret_YOUR_NOTION_API_KEY"
```

**Verify token stored:**
```bash
sqlite3 ~/.pedrocli/pedrocli.db "SELECT provider, service, expires_at FROM oauth_tokens;"
```

Expected output:
```
notion|database|
```

### Step 2: Test Notion Connection

Verify Notion MCP server works with your API key:

```bash
export NOTION_API_KEY="secret_YOUR_NOTION_API_KEY"
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}' | npx -y @modelcontextprotocol/server-notion
```

You should see a JSON response with server capabilities.

### Step 3: Create a Test Podcast Script

Run the script creator agent:

```bash
./pedrocli podcast create-script \
  -topic "Starting a Homelab" \
  -notes "Cover: hardware requirements, OS choices (Proxmox vs bare metal), first services to deploy (Pi-hole, Home Assistant), networking basics"
```

**Expected output:**
```
Job created: job-1234567890-20231215-143022
Running in background...
Job status: running

To check status: pedrocli status job-1234567890-20231215-143022
```

### Step 4: Monitor Job Progress

Check job status:

```bash
./pedrocli status job-1234567890-20231215-143022
```

**View detailed logs:**
```bash
ls /tmp/pedrocli-jobs/job-1234567890-20231215-143022/
```

You should see files like:
- `001-prompt.txt` - Initial prompt
- `002-response.txt` - LLM response
- `003-tool-calls.json` - Tool calls made
- `004-tool-results.json` - Tool results
- `final-output.txt` - Final script content

**View final output:**
```bash
cat /tmp/pedrocli-jobs/job-1234567890-20231215-143022/final-output.txt
```

### Step 5: Verify Notion Integration

1. Open your Notion "Scripts" database
2. Look for a new page with title matching your topic
3. Verify the script has proper structure:
   - Cold Open
   - Introduction
   - Main Content (multiple sections)
   - Wrap-up
4. Check timestamps and metadata

### Step 6: Test Other Commands (Optional)

**Add a link to articles database:**
```bash
./pedrocli podcast add-link \
  -url "https://www.reddit.com/r/homelab/" \
  -title "r/homelab Community" \
  -notes "Great resource for homelab enthusiasts, very active community"
```

**Add a guest:**
```bash
./pedrocli podcast add-guest \
  -name "John Doe" \
  -email "john@example.com" \
  -bio "Homelab enthusiast and Kubernetes expert with 10 years of experience" \
  -topics "networking,kubernetes,homelab"
```

**Create episode outline:**
```bash
./pedrocli podcast create-outline \
  -topic "Kubernetes for Beginners" \
  -duration "45min" \
  -format "interview"
```

**Review news:**
```bash
./pedrocli podcast review-news \
  -focus "AI" \
  -max-items 5
```

## Success Criteria

- ✅ Token stored successfully in database
- ✅ Notion MCP server connects and authenticates
- ✅ Job completes without errors
- ✅ Script appears in Notion database
- ✅ Script content follows podcast format
- ✅ Links, guests, outlines created in appropriate databases
- ✅ No token exposure in logs or output

## Troubleshooting

### Error: "failed to retrieve Notion API key"

**Cause**: Token not in database or database not accessible

**Fix**:
```bash
# Check if database exists
ls -la ~/.pedrocli/pedrocli.db

# Re-store token
go run scripts/setup-podcast-tokens.go -provider notion -service database -api-key "secret_..."
```

### Error: "Notion API error: unauthorized"

**Cause**: Invalid API key or integration not shared with database

**Fix**:
1. Verify API key is correct
2. In Notion, click "..." on database → "Add connections" → Select your integration

**Test API key manually:**
```bash
curl -H "Authorization: Bearer secret_YOUR_API_KEY" \
     -H "Notion-Version: 2022-06-28" \
     https://api.notion.com/v1/users/me
```

### Error: "no Notion MCP command configured"

**Cause**: Missing or invalid `podcast.notion.command` in config

**Fix**: Add to `.pedrocli.json`:
```json
{
  "podcast": {
    "notion": {
      "command": "npx -y @modelcontextprotocol/server-notion"
    }
  }
}
```

### Error: "database_id is required"

**Cause**: Database IDs not configured in `.pedrocli.json`

**Fix**: Add database IDs to config (see Configuration File section above)

### Job hangs or times out

**Cause**: Model taking too long or MCP server stalled

**Debug**:
```bash
# Check job files for last activity
ls -lt /tmp/pedrocli-jobs/<job-id>/ | head -10

# Check MCP subprocess output
grep -r "MCP" /tmp/pedrocli-jobs/<job-id>/
```

### Script quality is poor

**Cause**: Wrong model or model profile

**Fix**:
1. Use larger model (32B+ recommended for content)
2. Set `podcast.model_profile` to `"content"` in config
3. Adjust temperature (0.3-0.7 for creative content)

## Expected Output Examples

### Successful Script Creation

```bash
$ ./pedrocli podcast create-script -topic "Starting a Homelab"

Job created: job-1234567890-20231215-143022
Running in background...

Iteration 1/20:
  Tool: notion
  Action: create_page
  Database: scripts
  ✓ Page created: https://notion.so/xyz

Job completed successfully!
Duration: 2m 34s

Final output saved to: /tmp/pedrocli-jobs/job-1234567890-20231215-143022/final-output.txt
```

### Successful Link Addition

```bash
$ ./pedrocli podcast add-link -url "https://example.com/article" -title "Great Article"

Job created: job-1234567891-20231215-143100
Running in background...

Iteration 1/20:
  Tool: notion
  Action: create_page
  Database: potential_article
  ✓ Link added

Job completed successfully!
Duration: 8s
```

## Testing Checklist

Use this checklist to verify all functionality:

- [ ] Token storage works (setup-podcast-tokens.go)
- [ ] Token retrieval from database works
- [ ] Notion MCP server connects successfully
- [ ] `create-script` command runs and completes
- [ ] Script appears in Notion Scripts database
- [ ] Script has proper structure and content
- [ ] `add-link` command works
- [ ] `add-guest` command works
- [ ] `create-outline` command works
- [ ] `review-news` command works
- [ ] No tokens visible in job logs
- [ ] Error messages are clear and actionable
- [ ] Job status command shows progress

## Next Steps After Testing

If testing succeeds:
1. Document any issues found as GitHub issues
2. Test with different models and configurations
3. Test Google Calendar integration (requires OAuth setup)
4. Create example podcast episodes

If testing fails:
1. Check troubleshooting section above
2. Review job logs in `/tmp/pedrocli-jobs/`
3. Verify Notion API key and database IDs
4. Create GitHub issue with error details and logs
