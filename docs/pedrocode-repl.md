# pedrocode - Interactive REPL for PedroCLI

## Overview

`pedrocode` is the interactive REPL (Read-Eval-Print Loop) interface for PedroCLI. It provides a chat-like experience for interacting with autonomous coding agents, with real-time streaming progress updates.

## Three-Binary Architecture

PedroCLI now has three distinct binaries for different use cases:

1. **`pedrocli`** - Background job execution (async, polling-based)
   - Use for: CI/CD, scripts, automation
   - Commands: `build`, `debug`, `review`, `triage`, `blog`, `podcast`

2. **`pedrocode`** - Interactive REPL (NEW, streaming)
   - Use for: Interactive development, exploration
   - Modes: `code`, `blog`, `podcast`
   - Real-time progress updates with tree view

3. **`pedroweb`** - Web UI (HTTP server)
   - Use for: Team collaboration, remote access
   - Browser-based interface with SSE streaming

## Installation

Build from source:

```bash
make build-pedrocode
```

Or build all binaries:

```bash
make build
```

## Usage

### Start REPL

```bash
# Default: code mode
pedrocode

# Explicit mode selection
pedrocode code      # Coding agents
pedrocode blog      # Blog writing
pedrocode podcast   # Podcast prep
```

### Code Mode

Provides access to coding agents:
- **build** - Build new features from descriptions
- **debug** - Debug and fix issues
- **review** - Code review on branches/PRs
- **triage** - Diagnose issues without fixing

```bash
pedrocode

pedro:build> add rate limiting to the API
🤖 Processing with build agent...

├─ ⏳ Analyze
│  └─ In Progress
├─ ⏳ Plan
│  └─ Pending
├─ ⏳ Implement
│  └─ Pending
└─ ⏳ Test
   └─ Pending

[Real-time progress updates...]
```

### Blog Mode

Interactive blog writing:

```bash
pedrocode blog

pedro:blog> write about Go contexts and best practices
[Agent generates blog post with research...]
```

### Podcast Mode

Podcast episode preparation:

```bash
pedrocode podcast

pedro:podcast> outline for episode about choosing an LLM
[Agent creates structured outline...]
```

## REPL Commands

### Navigation

- `/help`, `/h`, `/?` - Show help message
- `/quit`, `/exit`, `/q` - Exit REPL
- `/mode <agent>` - Switch agent within current mode
- `/context`, `/info` - Show session information

### Utilities

- `/history` - Show command history
- `/clear`, `/cls` - Clear screen

### Natural Language

Just type your request naturally - no slash needed!

```
pedro:build> add authentication to the user service
pedro:build> fix the bug in the payment handler
pedro:build> refactor the database layer
```

## Progress Tracking

pedrocode provides real-time progress updates using a tree view display:

```
├─ ✓ Analyze
│  └─ Done . 3 tool uses . 2.5k tokens
├─ ▶ Plan
│  └─ In Progress (phase 2/4)
├─ ⏳ Implement
│  └─ Pending
└─ ⏳ Test
   └─ Pending
```

**Status Icons:**
- ⏳ Pending
- ▶ In Progress
- ✓ Done
- ✗ Failed

## Architecture

### REPL Infrastructure (`pkg/repl/`)

- **`repl.go`** - Main REPL orchestrator
- **`session.go`** - Session state management
- **`input.go`** - Readline integration (history, multi-line)
- **`output.go`** - Progress streaming handler
- **`commands.go`** - Command parser

### Progress Callbacks

Agents emit progress events during execution:

- `round_start` - Inference round begins
- `round_end` - Inference round completes
- `tool_call` - Tool is being called
- `tool_result` - Tool execution result
- `llm_response` - LLM response received
- `error` - Error occurred
- `complete` - Task completed

These events are streamed to the terminal in real-time using the existing `ProgressTracker` from `pkg/agents/progress.go`.

## Session Management

Each REPL session has:
- **Session ID** - Unique identifier (e.g., `code-a1b2c3d4-20240115-143022`)
- **Mode** - Current mode (code/blog/podcast)
- **Agent** - Current agent (build/debug/review/etc.)
- **History** - Command history (saved to `~/.pedrocode_history`)
- **Active Job** - Currently running job ID

View session info with `/context`:

```
pedro:build> /context

Session Context:
  Session ID: code-a1b2c3d4-20240115-143022
  Mode: code
  Current Agent: build
  Duration: 5m 32s
  Commands: 12
```

## Switching Agents

You can switch between agents within the same mode:

```
pedro:build> /mode debug
✅ Switched from build to debug

pedro:debug> investigate the auth bug
[Debug agent starts...]
```

## Multi-line Input

For longer prompts, use Ctrl+D or empty line to submit:

```
pedro:build> add a new API endpoint for user management
...   with the following requirements:
...   - GET /api/users - list all users
...   - POST /api/users - create user
...   - Include pagination and filtering
[Ctrl+D to submit]
```

## Comparison: pedrocli vs pedrocode

| Feature | pedrocli | pedrocode |
|---------|----------|-----------|
| Execution | Background jobs | Interactive REPL |
| Progress | Polling-based | Real-time streaming |
| Use Case | CI/CD, automation | Development, exploration |
| Interface | Command line args | Chat-like prompts |
| Output | File-based logs | Terminal streaming |
| Multi-turn | Manual chaining | Continuous conversation |

## Configuration

Uses the same `.pedrocli.json` configuration as other binaries:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "temperature": 0.2
  },
  "limits": {
    "max_inference_runs": 20
  },
  "tools": {
    "allowed_bash_commands": ["go", "git", "ls", "cat"]
  }
}
```

## Future Enhancements (Post-MVP)

1. **PR #60 Integration** - Slash commands, agent registry, tab completion
2. **Bubbletea TUI** - Rich terminal UI with panels and navigation
3. **Session Persistence** - Resume previous sessions
4. **Collaborative Mode** - Share sessions across team
5. **Streaming LLM Responses** - Token-by-token streaming

## Troubleshooting

### REPL doesn't start

Check config file:
```bash
# Try default mode
pedrocode code

# Check config exists
ls -la .pedrocli.json
```

### Agent not found

Verify agent is registered for current mode:
```
pedro:code> /mode blog
❌ Invalid agent: blog
  Available agents:
  - build
  - debug
  - review
  - triage
```

Use `/mode` without args to see available agents.

### No progress updates

Progress callbacks require agents to emit events. Check:
1. Using phased agent implementations (Builder/Debugger/Reviewer)
2. LLM backend is responding
3. Tools are executing successfully

## Examples

### Quick Feature Addition

```bash
pedrocode

pedro:build> add input validation to the signup form
[Agent analyzes code, creates plan, implements, tests]
✅ Task completed!

pedro:build> /quit
Goodbye!
```

### Debug Session

```bash
pedrocode code

pedro:build> /mode debug

pedro:debug> the login endpoint returns 500 errors
[Agent investigates logs, traces code, identifies issue]

pedro:debug> fix the bug you found
[Agent implements fix and runs tests]
✅ Task completed!
```

### Blog Workflow

```bash
pedrocode blog

pedro:blog> write a post about Go generics with examples
[Agent researches, outlines, generates content]

pedro:blog> make it more beginner-friendly
[Agent revises content]

pedro:blog> /quit
```

## See Also

- [CLAUDE.md](../CLAUDE.md) - Main project documentation
- [pedrocli Commands](../README.md) - Background CLI usage
- [Web UI Guide](./http-server.md) - HTTP server setup
- [Architecture Overview](./architecture.md) - System design
