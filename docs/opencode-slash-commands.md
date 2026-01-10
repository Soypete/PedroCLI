# OpenCode-Inspired Slash Commands

This document captures the implementation of slash commands in PedroCLI, inspired by OpenCode's command system.

## Overview

We added a slash command system that allows users to define reusable prompts/templates that can be invoked via `/command-name` syntax. Commands can optionally be routed to specific agents.

## Why We Made These Changes

1. **Reusability**: Users often have recurring tasks (e.g., "generate a blog outline", "run tests", "lint code"). Slash commands let them define these once and reuse.

2. **Template Expansion**: Commands support variable substitution (`$ARGUMENTS`, `$1`, `@filepath`, `` `!cmd` ``), making them flexible for different inputs.

3. **Agent Routing**: Commands can specify which agent should handle them, enabling workflows like `/blog-outline "topic"` automatically routing to the blog agent.

4. **Parity with OpenCode**: OpenCode has a similar command system that users find intuitive.

## Implementation Details

### New Files Created

| File | Purpose |
|------|---------|
| `pkg/commands/registry.go` | Command registry with template expansion |
| `pkg/cli/commands.go` | CLI command runner with `ParseSlashCommand` |
| `.pedro/command/*.md` | Example command definitions |

### CLI Support

```bash
# List available commands
pedrocli commands

# Run a command
pedrocli run /blog-outline "My topic"
pedrocli run /test
```

**Location**: `cmd/pedrocli/main.go`
- Added `run` subcommand → `runSlashCommand()`
- Added `commands` subcommand → `listSlashCommands()`

### Web API Support

```
GET  /api/commands      - List available slash commands
POST /api/commands/run  - Execute a slash command
```

**Location**: `pkg/httpbridge/handlers.go`
- Added `handleCommands()` for listing
- Added `handleCommandRun()` for execution

**Request format**:
```json
{
  "command": "/blog-outline",
  "arguments": "My topic here"
}
```

**Response (with agent)**:
```json
{
  "success": true,
  "job_id": "job-123456",
  "agent": "blog"
}
```

**Response (without agent)**:
```json
{
  "success": true,
  "expanded": "The expanded prompt text..."
}
```

### Command Definition Format

Commands are defined in `.pedro/command/<name>.md`:

```markdown
---
description: Generate a blog post outline from a topic
agent: blog
---
Create a detailed blog post outline for the following topic:

$ARGUMENTS

Include sections for introduction, main points, and conclusion.
```

### Template Variables

| Variable | Description |
|----------|-------------|
| `$ARGUMENTS` | All arguments joined with spaces |
| `$1`, `$2`, etc. | Positional arguments |
| `@filepath` | Contents of the specified file |
| `` `!command` `` | Output of shell command |

### Agent Routing

When a command has `agent: <name>` in its frontmatter, executing it will:
1. Expand the template with arguments
2. Route the expanded prompt to the specified agent
3. Return a job ID for tracking

Supported agents: `blog`, `build`, `debug`, `review`, `triage`

## TODO: Testing Required

The following tests need to be performed to verify the implementation:

### CLI Tests
- [ ] `pedrocli commands` lists all available commands
- [ ] `pedrocli run /blog-outline "test topic"` executes with blog agent
- [ ] `pedrocli run /test` expands and displays template (no agent)
- [ ] Commands without `/` prefix work: `pedrocli run blog-outline "topic"`
- [ ] Error handling for unknown commands

### Web API Tests
- [ ] `GET /api/commands` returns command list with correct format
- [ ] `POST /api/commands/run` with agent routes to correct agent
- [ ] `POST /api/commands/run` without agent returns expanded prompt
- [ ] Error responses for invalid commands
- [ ] Error responses for missing arguments

### Template Expansion Tests
- [ ] `$ARGUMENTS` substitution works
- [ ] `$1`, `$2` positional args work
- [ ] `@filepath` file inclusion works
- [ ] `` `!command` `` shell execution works
- [ ] Mixed template variables work together

### Integration Tests
- [ ] End-to-end: CLI → Command → Agent → Job created
- [ ] End-to-end: Web API → Command → Agent → Job created
- [ ] Command discovery from `.pedro/command/` directory
- [ ] Built-in commands (help, clear, etc.) work

## Related ADRs

- ADR-002: Dynamic prompt generation from tool registry
- ADR-003: Dynamic LLM-driven workflows
- ADR-007: Model-specific tool call formatting

## Commits

- `ab9f2e5`: feat: Add CLI slash command support with run and commands subcommands
- `763502e`: feat: Add web API support for slash commands
