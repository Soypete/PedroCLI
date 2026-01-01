# PedroCLI Web UI Architecture

## Overview

The PedroCLI web UI is a browser-based interface that uses **direct agent execution**. The HTTP server embeds agents directly - no subprocess spawning required. Agents execute in background goroutines with the job manager tracking status and results.

## Architecture (v0.3.0+)

```
Browser (HTMX)
    ↓ HTTP requests
HTTP Server (net/http)
    ↓ creates agent via AppContext
Agent (embedded, in-process)
    ↓ uses tools directly
Tools (file, git, bash, etc.)
    ↓ modifies code in
Project Directory (workdir)
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| HTTP Server | `cmd/http-server/main.go` | Entry point, graceful shutdown |
| AppContext | `pkg/httpbridge/app.go` | Shared dependencies, agent factories |
| Handlers | `pkg/httpbridge/handlers.go` | API endpoint implementations |
| SSE Broadcaster | `pkg/httpbridge/sse.go` | Real-time job updates |
| Job Manager | `pkg/jobs/manager.go` | Job lifecycle, status tracking |

## What Was Built

### HTTP Server (`cmd/http-server/main.go`)
- Creates `AppContext` with LLM backend, job manager, and tools
- Uses standard library `net/http` (no external frameworks)
- Binds to `0.0.0.0:8080` for Tailscale/remote access
- Background SSE polling for real-time updates

### AppContext (`pkg/httpbridge/app.go`)
- Centralizes all shared dependencies
- Factory methods for each agent type:
  - `NewBuilderAgent()` - Build new features
  - `NewDebuggerAgent()` - Debug issues
  - `NewReviewerAgent()` - Code review
  - `NewTriagerAgent()` - Diagnose problems
  - `NewBlogOrchestratorAgent()` - Rigid blog generation
  - `NewDynamicBlogAgent()` - LLM-driven blog creation

### Web UI (HTMX + Tailwind CSS)
- **Main page** (`pkg/web/templates/index.html`)
  - Job creation form with dynamic fields
  - Real-time job list with SSE updates
  - Responsive design (mobile-first)

### API Endpoints (`pkg/httpbridge/handlers.go`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/jobs` | POST | Create job (builder/debugger/reviewer/triager) |
| `/api/jobs` | GET | List all jobs |
| `/api/jobs/:id` | GET | Get job status |
| `/api/jobs/:id` | DELETE | Cancel job |
| `/api/stream/jobs/:id` | GET | SSE stream for job updates |
| `/api/blog` | POST | Create blog post |
| `/api/blog/orchestrate` | POST | Orchestrated blog generation |
| `/api/voice/transcribe` | POST | Voice transcription |
| `/api/voice/status` | GET | Whisper.cpp health check |
| `/api/health` | GET | Server health check |

## Configuration

```json
{
  "web": {
    "enabled": true,
    "port": 8080,
    "host": "0.0.0.0"
  }
}
```

## Build & Run

```bash
# Build
make build

# Run (with secrets from 1Password)
op run --env-file=.env -- ./pedrocli-http-server

# Access
open http://localhost:8080
```

## Phase 1: Core Web UI

- Lines of Code: ~600
- Standard library HTTP server
- HTMX for interactivity
- Tailwind CSS for styling

## Phase 2: Real-Time Updates (SSE)

### SSE Broadcaster (`pkg/httpbridge/sse.go`)
- Manages concurrent SSE connections
- Background polling every 2 seconds
- Broadcasts job status changes to connected clients
- Uses `jobs.Manager` directly for status checks

### Browser Integration
- EventSource for SSE connections
- Automatic reconnection on errors
- localStorage caching (24-hour expiry)

## Phase 3: Voice Dictation

### Voice Package (`pkg/voice/`)
- HTTP client for whisper.cpp server
- Browser MediaRecorder integration
- Automatic status checking

### Requirements
- whisper.cpp server running separately
- Configure in `.pedrocli.json`:
  ```json
  {
    "voice": {
      "enabled": true,
      "whisper_url": "http://localhost:8081"
    }
  }
  ```

## Service Ports

| Service | Port | Description |
|---------|------|-------------|
| HTTP Server | 8080 | Web UI and API |
| Whisper | 8081 | Voice transcription |
| Ollama | 11434 | LLM inference |
| pprof | 6060 | Debug profiling |

## Performance

- Server starts in <2 seconds
- Page load: <100ms
- Job creation: ~300-500ms
- SSE polling: 2 second intervals
- Binary size: ~27MB

## Mobile Support

- Responsive grid layout
- Touch-friendly buttons
- Tailscale-ready (0.0.0.0 binding)

## Files

### Core Server
- `cmd/http-server/main.go` - Entry point
- `pkg/httpbridge/app.go` - AppContext and agent factories
- `pkg/httpbridge/server.go` - Server setup and routes
- `pkg/httpbridge/handlers.go` - API handlers
- `pkg/httpbridge/sse.go` - SSE broadcaster

### Web UI
- `pkg/web/templates/base.html` - Base layout
- `pkg/web/templates/index.html` - Main page
- `pkg/web/templates/components/job_card.html` - Job component
- `pkg/web/static/js/app.js` - Client-side JavaScript
- `pkg/web/static/js/voice.js` - Voice recording

## Testing

```bash
# Unit tests
go test ./pkg/httpbridge/... -v

# Manual test
./pedrocli-http-server
open http://localhost:8080
```
