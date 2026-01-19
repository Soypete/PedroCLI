# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PedroCLI is a self-hosted autonomous coding agent system - an open-source alternative to Cursor's background jobs using your own LLMs. It runs autonomous agents powered by self-hosted models (via llama.cpp or Ollama) that can build features, debug issues, review code, and triage problems.

**Key Concept**: PedroCLI uses **direct agent execution** - agents run embedded in the CLI and HTTP server binaries with direct LLM backend integration. No subprocess spawning required.

## Build & Test Commands

### Build
```bash
make build              # Build CLI and HTTP server (current platform)
make build-cli          # Build CLI only
make build-http         # Build HTTP server only
make build-mac          # Build for macOS (arm64 + amd64)
make build-linux        # Build for Linux (amd64)
```

The build produces binaries:
- `pedrocli` - CLI client (cmd/pedrocli/main.go)
- `pedrocli-http-server` - HTTP server with web UI (cmd/http-server/main.go)

### Test
```bash
make test               # Run all tests
make test-coverage      # Run with coverage report
go test ./pkg/tools/... # Test specific package
go test -run TestName   # Run specific test
```

### Format & Lint
```bash
make fmt                # Format all code
make lint               # Run golangci-lint
make tidy               # Tidy dependencies
```

### Running the Web UI
```bash
# Start the HTTP server
./pedrocli-http-server

# Access at http://localhost:8080
```

### Running llama-server (LLM Backend)

**IMPORTANT: GPU Layer Configuration**
- Always use `-1` for `--n-gpu-layers` to offload ALL layers to GPU
- Context size affects VRAM usage significantly
- For 32B models on M1 Max (32GB RAM): use 16K context max
- For 32K context: may need smaller models (7B/14B) or fewer GPU layers

```bash
# Start llama-server (uses make llama-server from project root)
make llama-server

# Or manually:
llama-server \
  --model ~/.cache/huggingface/.../model.gguf \
  --port 8082 \
  --ctx-size 16384 \
  --n-gpu-layers -1 \  # Auto-detect (offloads all layers)
  --jinja \
  --no-webui \
  --metrics
```

### Running whisper.cpp (Voice Dictation)

The web UI supports voice-to-text using whisper.cpp. To enable:

1. **Start whisper.cpp server**:
```bash
# From whisper.cpp directory (e.g., ~/Code/ml/whisper.cpp)
./build/bin/whisper-server -m models/ggml-base.en.bin --port 9090 --host 0.0.0.0
```

2. **Enable in config** (`.pedrocli.json`):
```json
{
  "voice": {
    "enabled": true,
    "whisper_url": "http://localhost:9090",
    "language": "en"
  }
}
```

3. **Restart HTTP server** - voice buttons will now work in the web UI.

## Core Architecture

### Direct Agent Execution

Both CLI and HTTP server embed agents directly - no subprocess communication needed.

```
cmd/pedrocli/main.go          → User-facing commands (build, debug, review, triage)
                              → Loads .pedrocli.json config
                              → Creates LLM backend and agents directly
                              → Executes autonomous inference loops

cmd/http-server/main.go       → Web UI and API endpoints
                              → Creates AppContext with shared dependencies
                              → Agents execute jobs in background goroutines
                              → Job manager tracks status and results
```

### Package Structure

```
pkg/
├── agents/          # Autonomous agents (Code + Blog)
│   ├── base.go      # BaseAgent - common agent functionality
│   ├── executor.go  # InferenceExecutor - THE INFERENCE LOOP (critical!)
│   ├── builder.go   # Build new features
│   ├── debugger.go  # Debug and fix issues
│   ├── reviewer.go  # Code review
│   ├── triager.go   # Diagnose issues without fixing
│   ├── blog_orchestrator.go  # Multi-phase rigid blog generation
│   └── blog_dynamic.go       # Dynamic LLM-driven blog creation (ADR-003)
│
├── tools/           # Tools that agents use
│   ├── file.go      # Read/write entire files
│   ├── codeedit.go  # Precise line-based editing (edit/insert/delete)
│   ├── search.go    # Search code (grep, find files, find definitions)
│   ├── navigate.go  # Navigate code structure (list dirs, outlines, imports)
│   ├── git.go       # Git operations
│   ├── bash.go      # Safe shell commands (with allow/deny lists)
│   ├── test.go      # Run tests (Go, npm, Python)
│   ├── rss.go       # RSS/Atom feed parsing for blog research
│   └── static_links.go  # Static links from config for newsletters
│
├── toolformat/      # Model-specific tool call formatting (ADR-007)
│   ├── formatter.go # ToolFormatter interface
│   ├── generic.go   # Generic JSON format (default)
│   ├── qwen.go      # Qwen 2.5 <tool_call> format
│   ├── llama.go     # Llama 3.x <|python_tag|> format
│   ├── mistral.go   # Mistral [TOOL_CALLS] format
│   └── selector.go  # GetFormatter(modelName) selection
│
├── prompts/         # Dynamic prompt generation (ADR-002)
│   └── tool_generator.go  # Generate tool sections from registry
│
├── llm/             # LLM backend abstraction (native tool calling)
│   ├── interface.go # Backend interface with ToolDefinition support
│   ├── server.go    # Generic HTTP client (OpenAI-compatible APIs)
│   ├── llamacpp.go  # llama-server HTTP API wrapper
│   ├── ollama.go    # Ollama HTTP API wrapper
│   ├── tokens.go    # Token estimation and context window detection
│   └── factory.go   # Backend factory
│
├── llmcontext/      # File-based context management
│   └── manager.go   # Manages /tmp/pedrocli-jobs/<job-id>/ files
│
├── httpbridge/      # HTTP server and API
│   ├── app.go       # AppContext with agent factories
│   ├── server.go    # HTTP server setup
│   ├── handlers.go  # API endpoint handlers
│   └── sse.go       # Server-sent events for job updates
│
├── config/          # Configuration management
│   └── config.go    # Load/validate .pedrocli.json
│
├── jobs/            # Job management
│   └── manager.go   # Job lifecycle, status tracking
│
├── init/            # Dependency checking
│   └── checker.go   # Verify git, Ollama, llama.cpp, etc.
│
└── platform/        # Platform-specific utilities
    └── shell.go     # Shell command execution
```

## Critical Architecture Details

### 1. The Inference Loop (pkg/agents/executor.go)

**This is the heart of autonomous operation.** The `InferenceExecutor` runs an iterative loop:

```
1. Send prompt to LLM
2. Parse tool calls from response (JSON format)
3. Execute tools (search, read, edit, test, git, bash)
4. Feed results back to LLM
5. Repeat until task complete or max iterations (default: 20)
```

**Key files**:
- `pkg/agents/executor.go` - `InferenceExecutor.Execute()` method
- `pkg/agents/base.go` - `BaseAgent.executeInference()` for single round
- Tool calls are parsed from LLM JSON output: `{"tool": "tool_name", "args": {...}}`

### 2. File-Based Context (pkg/llmcontext/manager.go)

Unlike in-memory systems, PedroCLI writes all context to `/tmp/pedrocli-jobs/<job-id>/`:

```
/tmp/pedrocli-jobs/job-1234567890-20231215-143022/
├── 001-prompt.txt         # Initial prompt
├── 002-response.txt       # LLM response
├── 003-tool-calls.json    # Parsed tool calls
├── 004-tool-results.json  # Tool execution results
├── 005-prompt.txt         # Next prompt with feedback
└── ...
```

**Benefits**:
- Survives process crashes
- Easy to debug (inspect files)
- Natural context window management
- Clear audit trail

### 3. Context Window Management

**Critical**: Different models have different context limits. The system auto-detects for Ollama models and respects user-configured limits for llama.cpp.

- **Ollama**: Auto-detected in `pkg/llm/tokens.go` (Qwen 32B = 32k, Qwen 72B = 128k, etc.)
- **llama.cpp**: User specifies in `.pedrocli.json` (`context_size`, `usable_context`)
- **Rule**: Use 75% of stated context (leave room for response)
- See `docs/pedrocli-context-guide.md` for full details

Token estimation (rough): `tokens ≈ text_length / 4`

### 4. Tool Architecture

All tools implement `pkg/tools/interface.go`:

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args map[string]interface{}) (*Result, error)
}
```

**Tool execution flow**:
1. Agent calls tool via JSON: `{"tool": "code_edit", "args": {"file": "main.go", ...}}`
2. Executor parses and dispatches to appropriate tool
3. Tool executes and returns `Result{Success, Output, Error, ModifiedFiles}`
4. Result fed back to LLM in next prompt

### 5. Agent Types

Each agent has specialized prompts but shares the same BaseAgent + InferenceExecutor:

**Code Agents** (use 7 code tools):
- **Builder** (`pkg/agents/builder.go`) - Build new features from descriptions
- **Debugger** (`pkg/agents/debugger.go`) - Debug and fix issues (accepts symptoms, logs)
- **Reviewer** (`pkg/agents/reviewer.go`) - Code review on branches/PRs
- **Triager** (`pkg/agents/triager.go`) - Diagnose without fixing

**Blog Agents** (use research tools):
- **BlogContentAgent** (`pkg/agents/blog_content.go`) - 7-phase autonomous blog workflow (RECOMMENDED)
- **Writer** (`pkg/agents/writer.go`) - Expand dictation into blog posts
- **Editor** (`pkg/agents/editor.go`) - Review and refine blog content
- **BlogOrchestrator** (`pkg/agents/blog_orchestrator.go`) - Legacy multi-phase blog generation

The **BlogContentAgent** is the recommended blog creation system with a 7-phase autonomous workflow:
1. **Transcribe** - Load voice dictation or text input
2. **Research** - Web search, RSS feeds, GitHub scraping, calendar integration
3. **Outline** - Generate structured section outline (4-8 sections)
4. **Generate Sections** - Expand each section + create TLDR with logit bias
5. **Assemble** - Combine sections + generate social media posts (Twitter/Bluesky/LinkedIn)
6. **Editor Review** - Grammar, coherence, technical accuracy review
7. **Publish** - Save to PostgreSQL + version snapshots

**Key Features:**
- Progress tracking with tree view (like Claude Code's agent progress)
- Version snapshots at each phase (stored in `blog_post_versions` table)
- TLDR generation with logit bias (3-5 bullets, ~200 words)
- Platform-specific social posts with length enforcement
- O'Reilly course link prominently featured
- File-based context management for long-running tasks
- Research integration (web scraping, RSS, calendar, static links)

**CLI Usage:**
```bash
# From transcription file
./pedrocli blog -file transcript.txt

# From prompt
./pedrocli blog -prompt "Write about Go contexts"

# From content
./pedrocli blog -content "..." -title "My Post"
```

See `docs/blog-workflow.md` for full documentation.

### 6. Configuration (.pedrocli.json)

Config structure in `pkg/config/config.go`:

```json
{
  "model": {
    "type": "ollama",              // or "llamacpp"
    "model_name": "qwen2.5-coder:32b",
    "temperature": 0.2
  },
  "project": {
    "name": "ProjectName",
    "workdir": "/path/to/project"
  },
  "limits": {
    "max_task_duration_minutes": 30,
    "max_inference_runs": 20       // Max iterations in inference loop
  },
  "tools": {
    "allowed_bash_commands": ["go", "git", "ls", ...],
    "forbidden_commands": ["rm", "sudo", ...]
  },
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
      "newsletter": "https://soypetetech.substack.com",
      "youtube_placeholder": "Latest Video: [ADD LINK]"
    }
  }
}
```

Config files can be in:
1. `./.pedrocli.json` (current directory)
2. `~/.pedrocli.json` (home directory)

## Development Environment Setup

### Required Services

To run PedroCLI with all features, start these services:

```bash
# 1. Build all binaries
make build

# 2. Start Ollama (LLM backend)
ollama serve                    # In separate terminal, or runs as service

# 3. Start HTTP server with secrets
op run --env-file=.env -- ./pedrocli-http-server
```

### Environment Variables (.env)

```bash
# .env file - uses 1Password references
NOTION_TOKEN="op://pedro/notion_api_key/credential"
# Add other secrets as needed
```

### Quick Start (All Services)

```bash
# Terminal 1: Ollama (if not running as service)
ollama serve

# Terminal 2: Whisper server (for voice transcription)
~/Code/ml/whisper.cpp/build/bin/whisper-server \
  --model ~/Code/ml/whisper.cpp/models/ggml-base.en.bin \
  --port 8081 \
  --convert  # Uses ffmpeg to convert browser audio (WebM) to WAV

# Terminal 3: HTTP server with all features
cd /path/to/pedrocli
op run --env-file=.env -- ./pedrocli-http-server
```

Then open http://localhost:8080

### Blog Tools Setup

For blog writing features:

1. **Create a Notion Integration**:
   - Go to https://www.notion.so/my-integrations
   - Create a new integration (e.g., "PedroCLI Podcast Tools")
   - Copy the "Internal Integration Secret"

2. **Share your Notion database with the integration**:
   - Open your Tasks database in Notion
   - Click Share → Invite → Search for your integration name
   - The integration needs direct access to the database (not just parent page)

3. **Store the token in 1Password**:
   ```bash
   # .env file
   NOTION_TOKEN="op://pedro/notion_api_key/credential"
   ```

4. **Find your database ID**:
   ```bash
   # Test your token and list accessible databases
   op run --env-file=.env -- sh -c 'curl -s \
     -H "Authorization: Bearer $NOTION_TOKEN" \
     -H "Notion-Version: 2022-06-28" \
     "https://api.notion.com/v1/search" -X POST \
     -H "Content-Type: application/json" \
     -d "{\"filter\":{\"property\":\"object\",\"value\":\"database\"}}"' | jq '.results[] | {id: .id, title: .title[0].plain_text}'
   ```

5. **Configure in `.pedrocli.json`**:
   ```json
   {
     "blog": {
       "enabled": true,
       "notion_drafts_db": "YOUR-DATABASE-UUID-HERE",
       "notion_ideas_db": "YOUR-PROJECT-PAGE-UUID",
       "whisper_url": "http://localhost:8081",
       "whisper_model": "base.en"
     }
   }
   ```

   Example with real IDs:
   ```json
   {
     "blog": {
       "enabled": true,
       "notion_drafts_db": "18aa4c9f-9845-81d5-aad1-e53b75ab3a2b",
       "notion_ideas_db": "191a4c9f-9845-803b-b008-d16d6a025ba2",
       "whisper_url": "http://localhost:8081",
       "whisper_model": "base.en"
     }
   }
   ```

6. **Test the integration**:
   ```bash
   # Via CLI
   ./pedrocli blog -title "Test Post" -content "Hello world"

   # Via API
   curl -X POST http://localhost:8080/api/blog \
     -H "Content-Type: application/json" \
     -d '{"title":"Test Post","content":"Hello world"}'
   ```

### Voice Transcription (Whisper)

For voice dictation in the blog tools, whisper.cpp is installed at `~/Code/ml/whisper.cpp/`:

```bash
# Start whisper server (port 8081 to avoid conflict with HTTP server)
~/Code/ml/whisper.cpp/build/bin/whisper-server \
  --model ~/Code/ml/whisper.cpp/models/ggml-base.en.bin \
  --port 8081 \
  --convert  # Uses ffmpeg to convert browser audio (WebM) to WAV

# Check health
curl http://localhost:8081/health
```

Configure in `.pedrocli.json`:
```json
{
  "blog": {
    "whisper_url": "http://localhost:8081",
    "whisper_model": "base.en"
  }
}
```

The web UI also supports browser-based speech recognition (Chrome/Edge) as a fallback.

### Podcast Tools Setup

For podcast episode preparation features (outline, script, news, scheduling):

The podcast tools use the same Notion integration as blog tools (see "Blog Tools Setup" above).

**Running Podcast Commands with Notion Integration**:

```bash
# Use op run to inject NOTION_TOKEN from .env
op run --env-file=.env -- ./pedrocli podcast outline \
  -episode "S01E03" \
  -title "How to Choose a Local LLM" \
  -duration 25 \
  -output outline.md

# Generate script from outline
op run --env-file=.env -- ./pedrocli podcast script \
  -outline outline.md \
  -episode "S01E03"

# Create Cal.com booking link (requires CAL_API_KEY in .env)
op run --env-file=.env -- ./pedrocli podcast schedule \
  -episode "S01E03" \
  -title "How to Choose a Model" \
  -duration 60
```

**Environment Variables Required** (in `.env`):
```bash
# Notion integration
NOTION_TOKEN="op://pedro/notion_api_key/credential"

# Cal.com integration (for scheduling)
CAL_API_KEY="op://pedro/calcom_api_key/credential"
```

**Notion Database Configuration** (in `.pedrocli.json`):
```json
{
  "podcast": {
    "enabled": true,
    "notion": {
      "enabled": true,
      "databases": {
        "scripts": "YOUR-SCRIPTS-DATABASE-UUID",
        "guests": "YOUR-GUESTS-DATABASE-UUID",
        "topics": "YOUR-TOPICS-DATABASE-UUID",
        "news": "YOUR-NEWS-DATABASE-UUID"
      }
    }
  }
}
```

**Available Podcast Commands**:
- `podcast outline` - Generate structured episode outline from topic/summary
- `podcast script` - Generate episode script from outline
- `podcast news` - Review and summarize AI/tech news for episode prep
- `podcast schedule` - Create Cal.com booking link with Riverside.fm integration
- `podcast prep` - Full workflow (script + news + schedule)

**Without Notion**: If you don't need Notion integration, run without `op run`:
```bash
./pedrocli podcast outline -episode "S01E03" -title "My Topic" -output outline.md
```

The outline will be saved to the output file but not uploaded to Notion.

### Service Ports (Complete)

| Service | Port | Description |
|---------|------|-------------|
| HTTP Server | 8080 | Web UI and API |
| Whisper | 8081 | Voice transcription |
| Ollama | 11434 | LLM inference API |
| PostgreSQL | 5432 | Blog storage (if enabled) |

### Cal.com Scheduling Integration

PedroCLI integrates with Cal.com for scheduling and booking management. This enables agents to create booking links, manage calendars, and handle podcast episode scheduling.

#### Setup

1. **Get your Cal.com API key**:
   - Log into Cal.com (https://app.cal.com or your self-hosted instance)
   - Go to **Settings** → **Developer** → **API Keys**
   - Click **"+ Add"** button
   - Name the key (e.g., "Pedro CLI")
   - Click **Save** and **immediately copy the key** (format: `cal_live_xxx...`)

2. **Store in 1Password** (recommended):
   ```bash
   # .env file
   CAL_API_KEY="op://pedro/calcom_api_key/credential"
   ```

3. **Configure in `.pedrocli.json`**:
   ```json
   {
     "calcom": {
       "enabled": true,
       "api_key": "",  // Leave empty to use env var CAL_API_KEY
       "base_url": "https://api.cal.com/v1"  // Optional: for self-hosted Cal.com
     }
   }
   ```

4. **Run with API key** (if using 1Password):
   ```bash
   op run --env-file=.env -- ./pedrocli-http-server
   ```

#### Available Actions

The Cal.com tool supports the following actions:

**Bookings**:
- `get_bookings` - List all bookings (filterable by status)
- `get_booking` - Get specific booking details
- `create_booking` - Create a new booking
- `reschedule_booking` - Reschedule an existing booking
- `cancel_booking` - Cancel a booking

**Event Types** (booking pages):
- `get_event_types` - List all event types
- `get_event_type` - Get event type details
- `create_event_type` - Create a new booking page
- `update_event_type` - Update event type settings
- `delete_event_type` - Delete an event type

**Availability**:
- `get_schedules` - List availability schedules
- `get_availability` - Get available time slots for an event type
- `get_busy_times` - Get busy/unavailable periods

**User**:
- `get_me` - Get authenticated user profile

#### Example Usage

```json
{
  "tool": "cal_com",
  "args": {
    "action": "get_event_types"
  }
}
```

```json
{
  "tool": "cal_com",
  "args": {
    "action": "create_event_type",
    "title": "Podcast Interview",
    "slug": "podcast-60min",
    "length": 60,
    "description": "60-minute podcast interview for SoypeteTech"
  }
}
```

#### Podcast Workflow

For podcast scheduling, agents can:
1. Fetch episode template from Notion
2. Create Cal.com event type with template as description
3. Return shareable booking link for guests

#### Riverside.fm Integration for Podcasts

Cal.com has native integration with Riverside.fm for high-quality podcast recording. You can set up Riverside in two ways:

**Option 1: Native Cal.com + Riverside Integration** (Recommended)
1. Connect Riverside to your Cal.com account:
   - Log into Cal.com → Settings → Apps
   - Search for "Riverside" and install the app
   - Connect your Riverside account
2. Create event types with Riverside location type:
   ```json
   {
     "tool": "cal_com",
     "args": {
       "action": "create_event_type",
       "title": "SoypeteTech Podcast Interview",
       "slug": "podcast-interview-60min",
       "length": 60,
       "description": "60-minute interview for SoypeteTech podcast. We'll discuss your expertise in [TOPIC]. Recording happens on Riverside.fm for professional audio/video quality.",
       "locations": [
         {
           "type": "integration",
           "integration": "riverside"
         }
       ]
     }
   }
   ```

**Option 2: Direct Riverside Studio Link**
If you have a dedicated Riverside studio, you can provide the direct link:
```json
{
  "tool": "cal_com",
  "args": {
    "action": "create_event_type",
    "title": "SoypeteTech Podcast Interview",
    "slug": "podcast-interview-60min",
    "length": 60,
    "description": "60-minute interview for SoypeteTech podcast. We'll discuss your expertise in [TOPIC]. Recording happens on Riverside.fm for professional audio/video quality.",
    "locations": [
      {
        "type": "link",
        "link": "https://riverside.fm/studio/soypete-tech-podcast",
        "displayLocationLabel": "Riverside Studio"
      }
    ]
  }
}
```

**Complete Podcast Setup Example**:
```json
{
  "tool": "cal_com",
  "args": {
    "action": "create_event_type",
    "title": "SoypeteTech Deep Dive - 90min",
    "slug": "soypete-deep-dive-90",
    "length": 90,
    "description": "Extended 90-minute technical deep dive for SoypeteTech podcast. Topics: Cloud-native development, Go programming, AI/ML engineering, developer tools.\n\nFormat:\n- 5 min: Intro & guest background\n- 75 min: Technical discussion\n- 10 min: Rapid-fire Q&A\n\nRecording on Riverside.fm for studio-quality audio/video.",
    "locations": [
      {
        "type": "integration",
        "integration": "riverside"
      }
    ],
    "minimumBookingNotice": 1440,
    "beforeEventBuffer": 15,
    "afterEventBuffer": 15,
    "requiresConfirmation": true
  }
}
```

This creates a podcast booking page with:
- 90-minute duration
- Riverside.fm integration for recording
- 24-hour minimum notice (1440 minutes)
- 15-minute buffer before/after (prep time)
- Manual confirmation required (screen guests)

The booking link will be returned in the response as `bookingURL` (e.g., `https://cal.com/yourname/soypete-deep-dive-90`).

## Development Workflow

### Adding a New Tool

1. Create `pkg/tools/newtool.go` implementing `tools.Tool` interface
2. Add tests in `pkg/tools/newtool_test.go`
3. Register with agents in `pkg/httpbridge/app.go` or `cmd/pedrocli/setup.go`
4. Tool descriptions are auto-generated from `ExtendedTool` metadata (ADR-002)

### Modifying Agent Behavior

1. **Prompt changes**: Edit system prompts in `pkg/agents/prompts/*.md`
2. **Inference loop changes**: Modify `pkg/agents/executor.go` (careful - this is critical)
3. **Tool selection**: Register tools with agents in factory methods (`NewBuilderAgent()`, etc.)

### Testing Changes

```bash
# Test specific package
go test ./pkg/tools/...
go test ./pkg/agents/...

# Test with coverage
make test-coverage

# Manual integration test
make build
./pedrocli build -description "Add a test function"
```

Check job output in `/tmp/pedrocli-jobs/` to debug issues.

## Key Design Principles

### 1. File-Based Everything
All context, prompts, responses, tool calls/results are written to files. This enables crash recovery and easy debugging.

### 2. Tool Safety
- Bash commands are restricted via allow/deny lists in config
- Tools validate inputs and return structured errors
- No tool should perform destructive operations without explicit confirmation

### 3. Iterative Refinement
Agents don't need to succeed on first try. The inference loop allows them to:
- Try, fail, learn from error
- Run tests, see failures, fix them
- Keep iterating until success (up to `max_inference_runs`)

### 4. Model Agnostic
The system works with any model that can:
- Understand tool-use instructions
- Output JSON in the correct format
- Fit the code context in its context window

Better models (Qwen 2.5 Coder 32B+) = better results.

## Common Gotchas

### When Tests Fail
- Check `/tmp/pedrocli-jobs/<job-id>/` for full execution history
- Set `debug.keep_temp_files: true` in config to preserve job directories
- Look at `*-tool-results.json` files to see what actually happened

### Context Window Issues
- If agent seems confused or forgets context, check token usage
- Larger repos may need larger models (32B+ recommended)
- See `docs/pedrocli-context-guide.md` for strategies

### Tool Call Parsing Failures
- LLM must output exact JSON format: `{"tool": "name", "args": {...}}`
- Some models do this better than others
- Check `*-response.txt` to see what LLM actually generated

### Inference Loop Not Stopping
- Agent should output "TASK_COMPLETE" when done
- Check `max_inference_runs` in config
- Look at final response to see if completion signal was detected

## Important Files to Understand

1. **pkg/agents/executor.go** - The inference loop (most critical)
2. **pkg/agents/base.go** - Agent foundation and system prompts
3. **pkg/llmcontext/manager.go** - File-based context management
4. **pkg/tools/*.go** - Tool implementations
5. **pkg/toolformat/*.go** - Model-specific tool call parsing (Qwen, Llama, Mistral)
6. **pkg/httpbridge/app.go** - Shared dependencies and agent factories
7. **cmd/pedrocli/main.go** - CLI entry point

## Release & Distribution

- GoReleaser config: `.goreleaser.yml`
- Install script: `install.sh` (one-line install)
- Docker: `Dockerfile` + `docker-compose.yml`
- GitHub Actions: `.github/workflows/release.yml` (not visible in this snapshot but exists)
- Homebrew tap: Built into GoReleaser config

To cut a release: Tag a commit (`v1.2.3`) and push - GoReleaser handles the rest.
