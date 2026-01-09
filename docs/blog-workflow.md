# Blog Content Workflow

PedroCLI includes a sophisticated 7-phase blog content generation workflow powered by the **BlogContentAgent**. This workflow transforms voice dictation or text prompts into publication-ready blog posts with automatic research, TLDR generation, social media posts, and editorial review.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [The 7-Phase Workflow](#the-7-phase-workflow)
- [CLI Usage](#cli-usage)
- [Configuration](#configuration)
- [Database Schema](#database-schema)
- [Version Management](#version-management)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Overview

The BlogContentAgent orchestrates a complete blog creation pipeline:

```
Voice Dictation → Research → Outline → Generate → Assemble → Review → Publish
```

**Key Features:**
- 🎤 Voice-to-text transcription support (via Whisper)
- 🔍 Automated research (web search, RSS feeds, GitHub scraping)
- 📝 Multi-section content generation with TLDR
- 📱 Social media post generation (Twitter, Bluesky, LinkedIn)
- ✏️ Grammar and coherence review
- 💾 Version snapshots at each phase
- 📊 Real-time progress tracking
- 🗄️ PostgreSQL storage with full version history

## Quick Start

### Prerequisites

1. **Start required services:**
```bash
# Start PostgreSQL
make postgres-up

# Start llama.cpp server (in separate terminal)
make llama-server

# Optional: Start Whisper for voice transcription
~/Code/ml/whisper.cpp/build/bin/whisper-server \
  --model ~/Code/ml/whisper.cpp/models/ggml-base.en.bin \
  --port 8081 \
  --convert
```

2. **Configure PedroCLI:**

Ensure your `.pedrocli.json` has blog settings:
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
      "oreilly": "https://tr.ee/lbgNjvyc6f",
      "discord": "https://discord.gg/soypete",
      "linktree": "https://linktr.ee/soypete_tech",
      "youtube": "https://youtube.com/@soypete",
      "twitter": "https://twitter.com/soypete",
      "newsletter": "https://soypetetech.substack.com"
    }
  }
}
```

### Create Your First Blog Post

**From a transcription file:**
```bash
./pedrocli blog -file test/fixtures/quick_tip_context_management.txt
```

**From a prompt:**
```bash
./pedrocli blog -prompt "Write about file-based context management in LLMs"
```

**From direct content:**
```bash
./pedrocli blog -content "Context window limits are challenging..." -title "LLM Context Tips"
```

## The 7-Phase Workflow

### Phase 1: Transcribe
**Purpose:** Load and prepare input content

**Input Options:**
- Voice transcription file (WAV/MP3 via Whisper)
- Text file
- Direct text input
- CLI prompt

**Output:** Raw transcription text

### Phase 2: Research
**Purpose:** Gather context and supporting material

**Tools Used:**
- `web_search` - DuckDuckGo web search
- `web_scraper` - Scrape articles, GitHub repos, local files
- `rss_feed` - Parse RSS/Atom feeds
- `calendar_tool` - Fetch calendar events (if configured)
- `static_links` - Load configured static links

**Output:** Research data JSON with sources and content

### Phase 3: Outline
**Purpose:** Generate structured outline

**Process:**
- Analyzes transcription and research
- Creates 4-8 section structure
- Determines flow and key points
- No tool calls (pure LLM generation)

**Output:** Markdown outline with section titles and summaries

**Example:**
```markdown
1. Introduction
   - Hook: Context window frustration
   - Problem statement

2. Understanding Context Window Limits
   - Token limits explained
   - Impact on coding tasks

3. File-Based Solution
   - Architecture overview
   - Benefits

...
```

### Phase 4: Generate Sections
**Purpose:** Expand each section with detailed content

**Features:**
- Generates each section independently
- Creates TLDR with logit bias (3-5 bullet points, ~200 words max)
- Progress tracking: "section 3/6"
- Parallel generation possible (future enhancement)

**Logit Bias Settings:**
- Encourages: bullet points (•, -), newlines, concise language
- Discourages: verbose transitions (however, moreover, furthermore)
- Max tokens: 200 for TLDR, 800-1200 per section

**Output:** Array of section objects + TLDR

### Phase 5: Assemble
**Purpose:** Combine sections and generate social content

**Process:**
1. Combine title + TLDR + sections + conclusion
2. Add "Stay Connected" section with O'Reilly link (prominent)
3. Generate platform-specific social posts
4. Inject static links from config

**Social Media Generation:**
- **Twitter**: Max 280 chars, hashtag included
- **Bluesky**: Max 300 chars, link + hashtag
- **LinkedIn**: Max 3000 chars, professional tone

**Output:** Complete blog post markdown + social posts map

### Phase 6: Editor Review
**Purpose:** Grammar, coherence, and technical accuracy

**Review Focus:**
- Grammar and spelling
- Coherence and flow
- Technical accuracy
- Generalist engineer audience (avoid jargon overload)
- Code example quality

**Output:** Revised content + editor notes

### Phase 7: Publish
**Purpose:** Save to database and publish

**Storage:**
- PostgreSQL `blog_posts` table
- Version snapshot with `phase_result` type
- Status: `published`

**Future Enhancements:**
- Notion publishing (currently not wired)
- Substack API integration
- Scheduled publishing

**Output:** Post ID and publish confirmation

## CLI Usage

### Command Syntax

```bash
pedrocli blog [flags]
```

### Flags

| Flag | Type | Description | Example |
|------|------|-------------|---------|
| `-file` | string | Path to transcription file | `-file transcript.txt` |
| `-prompt` | string | Direct prompt for agent | `-prompt "Write about Go contexts"` |
| `-content` | string | Direct content (requires `-title`) | `-content "..." -title "My Post"` |
| `-title` | string | Post title (used with `-content`) | `-title "Context Tips"` |
| `-publish` | bool | Publish to Notion (not yet implemented) | `-publish` |

### Input Methods

**Method 1: File-based (recommended for voice dictation)**
```bash
./pedrocli blog -file my_recording.txt
```

**Method 2: Prompt-based (quick posts)**
```bash
./pedrocli blog -prompt "Explain Go's context package with examples"
```

**Method 3: Content-based (pre-written content)**
```bash
./pedrocli blog \
  -title "Understanding Go Contexts" \
  -content "The context package provides..."
```

### Progress Output

The CLI displays a tree view of progress:

```
BlogContentAgent: Creating blog post
├─ ✓ Phase 1: Transcribe
│  └─ Done
├─ ✓ Phase 2: Research . 1.1k tokens . 4 tool uses
│  └─ Done
├─ ✓ Phase 3: Outline . 1.3k tokens
│  └─ Done (6 sections)
├─ ▶ Phase 4: Generate Sections . 5.2k tokens
│  └─ In Progress (section 3/6)
├─ ⏳ Phase 5: Assemble
│  └─ Pending
├─ ⏳ Phase 6: Editor Review
│  └─ Pending
└─ ⏳ Phase 7: Publish
   └─ Pending
```

**Status Icons:**
- ⏳ Pending
- ▶ In Progress
- ✓ Done
- ✗ Failed

## Configuration

### Blog Configuration (.pedrocli.json)

```json
{
  "blog": {
    "enabled": true,
    "rss_feed_url": "https://your-blog.substack.com/feed",
    "research": {
      "enabled": true,
      "calendar_enabled": true,
      "rss_enabled": true,
      "max_rss_posts": 5,
      "max_calendar_days": 30
    },
    "static_links": {
      "oreilly": "https://tr.ee/your-course",
      "discord": "https://discord.gg/your-server",
      "linktree": "https://linktr.ee/your-profile",
      "youtube": "https://youtube.com/@your-channel",
      "twitter": "https://twitter.com/your-handle",
      "newsletter": "https://your-newsletter.substack.com"
    }
  }
}
```

### Database Configuration

PostgreSQL connection settings:

```json
{
  "database": {
    "host": "localhost",
    "port": 5432,
    "user": "pedrocli",
    "password": "pedrocli",
    "database": "pedrocli",
    "sslmode": "disable"
  }
}
```

### LLM Backend Configuration

```json
{
  "model": {
    "type": "llamacpp",
    "server_url": "http://localhost:8082/v1",
    "model_name": "qwen2.5-coder-32b-instruct",
    "temperature": 0.3,
    "context_size": 32768
  }
}
```

**Recommended Models:**
- **Qwen 2.5 Coder 32B** (best quality, requires 24GB+ VRAM)
- **Qwen 2.5 Coder 14B** (good quality, 12GB VRAM)
- **Llama 3.1 8B** (fast, 8GB VRAM)

## Database Schema

### blog_posts Table

```sql
CREATE TABLE blog_posts (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    post_title TEXT,
    raw_transcription TEXT,
    outline TEXT,
    sections JSONB,
    final_content TEXT,
    social_posts JSONB,
    editor_output TEXT,
    current_version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### blog_post_versions Table

```sql
CREATE TABLE blog_post_versions (
    id TEXT PRIMARY KEY,
    post_id TEXT NOT NULL REFERENCES blog_posts(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    version_type TEXT NOT NULL,  -- 'auto_snapshot', 'manual_save', 'phase_result'
    status TEXT NOT NULL,
    phase TEXT,
    post_title TEXT,
    title TEXT,
    raw_transcription TEXT,
    outline TEXT,
    sections TEXT,              -- JSON array
    full_content TEXT,
    created_by TEXT DEFAULT 'system',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    change_notes TEXT,
    UNIQUE(post_id, version_number)
);
```

### Querying Posts

**Get latest posts:**
```sql
SELECT id, post_title, status, created_at
FROM blog_posts
ORDER BY created_at DESC
LIMIT 10;
```

**Get post with version history:**
```sql
SELECT p.post_title, p.status, v.version_number, v.phase, v.version_type
FROM blog_posts p
JOIN blog_post_versions v ON p.id = v.post_id
WHERE p.id = 'YOUR-POST-ID'
ORDER BY v.version_number;
```

**Get specific version:**
```sql
SELECT full_content
FROM blog_post_versions
WHERE post_id = 'YOUR-POST-ID'
  AND version_number = 3;
```

## Version Management

### Automatic Version Snapshots

The system creates version snapshots automatically:

1. **Phase Transitions**: After each phase completes (7 snapshots per post)
2. **Status Changes**: When status changes (dictated → outlined → drafted → reviewed → published)

**Version Types:**
- `phase_result` - Saved after each workflow phase
- `auto_snapshot` - Saved on status change
- `manual_save` - User-initiated save (future: web UI)

### Accessing Versions

**Via CLI (future enhancement):**
```bash
# List versions
./pedrocli blog versions -id abc123

# View specific version
./pedrocli blog version -id abc123 -version 3

# Diff two versions
./pedrocli blog diff -id abc123 -v1 1 -v2 5
```

**Via SQL:**
```sql
-- List all versions for a post
SELECT version_number, phase, status, created_at
FROM blog_post_versions
WHERE post_id = 'YOUR-POST-ID'
ORDER BY version_number;

-- View Phase 4 output (Generate Sections)
SELECT full_content
FROM blog_post_versions
WHERE post_id = 'YOUR-POST-ID'
  AND phase = 'Generate Sections';
```

### Version Storage Details

Each version stores:
- Complete post content at that phase
- Sections array (if applicable)
- Outline (if generated)
- Raw transcription
- Phase name and status
- Token counts and metadata

**Storage Efficiency:**
- Text columns (not BLOB)
- JSONB for structured data (sections, social_posts)
- Indexes on post_id, version_number, created_at

## Examples

### Example 1: Quick Technical Tip

```bash
./pedrocli blog -prompt "Explain how to use Go's sync.Pool for memory optimization"
```

**Expected Output:**
- ~800 word post
- 4-5 sections
- TLDR with 3-4 bullets
- Code examples
- Social media posts
- ~2-3 minutes generation time

### Example 2: Blog Series from Voice Dictation

```bash
# Record audio with Whisper
# (produces transcript.txt)

./pedrocli blog -file transcript.txt
```

**For longer dictations (>2000 words):**
- Generates 6-8 sections
- Includes research from RSS and web
- TLDR prominently at top
- ~5-7 minutes generation time

### Example 3: Technical Deep Dive

```bash
./pedrocli blog -file deep_dive_transcript.txt
```

**Content from `deep_dive_transcript.txt`:**
```
I want to write about building autonomous coding agents with local LLMs.
Cover the architecture, tool calling, context management, and real examples
from PedroCLI. Include code snippets and explain the inference loop.
```

**Expected Sections:**
1. Introduction - Why local LLMs for coding
2. Architecture Overview - Components and flow
3. Tool Calling System - How agents use tools
4. Context Management - File-based approach
5. The Inference Loop - Core execution logic
6. Real-World Examples - PedroCLI case studies
7. Conclusion - Lessons learned

### Example 4: Checking Results

**Terminal output:**
```
✅ Phase 7 complete!

📝 Post ID: be47235a-d7cb-432b-b7d4-118f4d501f04
📊 Version: 6
📅 Created: 2024-01-08 14:23:45
```

**View in database:**
```bash
psql postgresql://pedrocli:pedrocli@localhost:5432/pedrocli

SELECT final_content FROM blog_posts
WHERE id = 'be47235a-d7cb-432b-b7d4-118f4d501f04';
```

**Check version history:**
```sql
SELECT version_number, phase, status
FROM blog_post_versions
WHERE post_id = 'be47235a-d7cb-432b-b7d4-118f4d501f04'
ORDER BY version_number;
```

## Troubleshooting

### Issue: "Database connection failed"

**Solution:**
```bash
# Check if PostgreSQL is running
make postgres-up

# Verify connection
psql postgresql://pedrocli:pedrocli@localhost:5432/pedrocli -c "SELECT 1;"

# Check config
cat .pedrocli.json | jq '.database'
```

### Issue: "Context window exceeded"

**Symptoms:** Model outputs become confused or truncated

**Solutions:**
1. Use smaller transcription (<2000 words recommended)
2. Increase context size in llama-server:
   ```bash
   llama-server --ctx-size 32768 ...
   ```
3. Use larger model (32B+ recommended for long posts)
4. Disable research tools to reduce context:
   ```json
   {"blog": {"research": {"enabled": false}}}
   ```

### Issue: "Grammar constraints failed"

**Error:** `Failed to parse grammar`

**Cause:** GBNF grammar not supported by server

**Solution:** Already handled - grammar is disabled by default. Relies on prompt engineering and max_tokens.

### Issue: Research phase takes too long

**Solutions:**
1. Reduce RSS posts:
   ```json
   {"blog": {"research": {"max_rss_posts": 2}}}
   ```
2. Disable calendar:
   ```json
   {"blog": {"research": {"calendar_enabled": false}}}
   ```
3. Skip research entirely for quick posts:
   ```bash
   # Edit BlogContentAgent to skip Phase 2
   ```

### Issue: Social media posts too long

**Check:**
```sql
SELECT social_posts FROM blog_posts
WHERE id = 'YOUR-POST-ID';
```

**Verify lengths:**
- Twitter: ≤280 chars
- Bluesky: ≤300 chars
- LinkedIn: ≤3000 chars

**If too long:** Prompt engineering issue - check `pkg/agents/blog_helpers.go:GenerateSocialMediaPost()`

### Issue: TLDR not concise enough

**Expected:** 3-5 bullets, ~150-200 words total

**Check output:**
```sql
SELECT sections::jsonb -> 0 ->> 'content' AS tldr
FROM blog_posts
WHERE id = 'YOUR-POST-ID';
```

**If verbose:** Logit bias may need tuning in `blog_helpers.go:GenerateTLDR()`

### Issue: O'Reilly link not prominent

**Check "Stay Connected" section:**
```sql
SELECT final_content FROM blog_posts
WHERE id = 'YOUR-POST-ID';
```

**Expected format:**
```markdown
## Stay Connected

**📚 Learn More:**
- [My Go Programming Course on O'Reilly](https://tr.ee/...) - Comprehensive Go training

**Connect:**
- [Discord]...
```

**Fix:** Check `pkg/agents/blog_content.go:buildStayConnectedSection()`

## Performance Metrics

**Typical Benchmarks** (Qwen 2.5 Coder 32B, 4-bit quant, M1 Max 32GB):

| Post Length | Sections | Total Tokens | Generation Time |
|-------------|----------|--------------|-----------------|
| Quick Tip (400 words) | 4 | ~8k | 2-3 min |
| Standard Post (1200 words) | 6 | ~15k | 4-5 min |
| Deep Dive (2500 words) | 8 | ~28k | 8-10 min |

**Token Breakdown:**
- Research: 1-2k tokens
- Outline: 1-1.5k tokens
- Sections: 1-2k per section
- Assemble: 200-500 tokens
- Editor Review: 2-3k tokens

**Optimization Tips:**
- Use temperature 0.2-0.3 for technical content
- Enable GPU layers: `-ngl -1` in llama-server
- Keep context under 75% of model limit
- Generate sections in parallel (future enhancement)

## Next Steps

1. **Try the workflow:** Create your first blog post
2. **Customize prompts:** Edit system prompts in `pkg/agents/prompts/`
3. **Add Notion publishing:** Wire up Phase 7 with Notion API
4. **Build web UI:** Add review interface with diff view
5. **Fine-tune for your voice:** LoRA training on existing posts

## Resources

- [PedroCLI GitHub](https://github.com/soypete/pedrocli)
- [CLAUDE.md](../CLAUDE.md) - Development guide
- [Blog Orchestrator Docs](./blog-orchestrator.md) - Legacy multi-phase system
- [ADR-003: Dynamic Blog Workflow](../decisions/003-dynamic-blog-workflow.md)
- [O'Reilly Go Course](https://tr.ee/lbgNjvyc6f)

---

**Last Updated:** 2024-01-08
**Version:** 1.0.0
**Author:** PedroCLI Team
