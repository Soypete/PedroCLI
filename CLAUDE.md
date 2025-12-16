# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PedroCLI is an autonomous coding agent built as an MCP (Model Context Protocol) server. It uses llama.cpp or Ollama backends to power AI agents that can autonomously build features, debug code, review PRs, and triage issues.

**Key Architecture Principle**: PedroCLI IS the MCP server, not a client. It communicates via JSON-RPC over stdio, not HTTP.

## Building and Testing

```bash
# First-time setup: install development tools (golangci-lint, etc.)
make setup

# Build for current platform
make build

# Build MCP server
make build-server

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run single test
go test -v -run TestFunctionName ./pkg/package

# Format code
make fmt
```

## Configuration System

The project uses `.pedroceli.json` for configuration (see `.pedroceli.json.example`). Configuration is loaded via `pkg/config`:

- Looks in current directory first, then home directory
- Validates model type (llamacpp/ollama) and context sizes
- Sets sensible defaults for missing fields
- Key configs: model settings, git behavior, tool permissions, execution limits

## Architecture Overview

### Core Components

**Agents** (`pkg/agents/`):
- `BaseAgent`: Common functionality, handles inference execution, tool management
- `Builder`: Autonomously builds features from descriptions
- `Debugger`: Debugs and fixes issues
- `Reviewer`: Performs blind code reviews on branches
- `Triager`: Diagnoses issues without fixing them
- All agents use one-shot inference (no conversational state)

**Context Management** (`pkg/llmcontext/`):
- File-based storage in `/tmp/pedroceli-jobs/`
- No in-memory context maintained
- Files stored as: `001-prompt.txt`, `002-response.txt`, `003-tool-calls.json`, etc.
- `GetHistoryWithinBudget()`: Loads recent history that fits in context window
- Automatically summarizes old inference rounds when context is tight
- Cleanup behavior controlled by debug mode

**Job Management** (`pkg/jobs/`):
- Tracks agent execution state
- Persists to disk for recovery
- Each job gets unique ID and timestamped directory

**LLM Backends** (`pkg/llm/`):
- Interface: `Backend` with `Infer()` method
- `LlamaCppBackend`: Executes llama.cpp via subprocess
- Token estimation: ~4 chars per token for budget calculations
- `CalculateBudget()`: Reserves space for system prompt, user prompt, response, and history

**Tools System** (`pkg/tools/`):
- All tools implement `Tool` interface with `Execute(ctx, args)` method
- **file**: Read, write, append, delete entire files
- **code_edit**: Precise line-based editing (edit_lines, insert_at_line, delete_lines)
- **search**: Grep patterns, find files by name/pattern, find code definitions with regex
- **navigate**: Directory trees, file outlines (functions/types), list imports
- **git**: Branch, commit, diff, status, PR via `gh` CLI
- **bash**: Restricted to allowed commands only (no sed/grep/find - use dedicated tools)
- **test**: Run and parse test output for Go, npm, Python

### Key Design Patterns

**Context Window Management**:
- Total context = System prompt + History + User prompt + Response buffer (8192 tokens)
- Usable context typically 75% of model's context_size
- Recent history (last 3 inference rounds) kept verbatim
- Older history summarized as: tool calls made, files modified
- See `pkg/llmcontext/manager.go:GetHistoryWithinBudget()`

**Tool Execution Flow**:
1. Agent receives task input
2. Creates context manager for job
3. Builds system prompt with available tools
4. Executes inference with budget-constrained history
5. Parses tool calls from response
6. Executes tools sequentially
7. Saves tool results to context
8. Repeats until task complete (max 20 inference runs by default)

**Cross-Platform Support**:
- Platform detection in `pkg/platform/`
- Uses Go stdlib, NOT shell commands like sed/grep
- `code_edit` tool implements editing in pure Go
- `search` tool uses Go filepath/regex, not grep
- Bash tool validates allowed commands per platform

## Important Implementation Details

**Why file-based context?**
- Enables job recovery after crashes
- Makes debugging easy (inspect `/tmp/pedroceli-jobs/`)
- Allows external tools to monitor agent progress
- Simpler than managing in-memory state

**Why one-shot inference?**
- Each inference gets full context from files
- No need to manage conversation state
- Agent can be stateless between inference rounds
- Tool results feed back as history for next round

**Why restricted bash?**
- Use dedicated tools (search, code_edit, navigate) instead
- Prevents accidental destructive commands
- Cross-platform compatibility issues with sed/grep
- Forbidden: sed, grep, find, xargs, rm, mv, dd, sudo

**Git workflow**:
- Default branch prefix: `pedroceli/`
- Always creates draft PRs (configurable)
- Uses `gh` CLI for PR creation
- Tool validates git/gh available before use

## Testing Philosophy

- Test coverage currently at 45.3% overall
- Each tool has comprehensive unit tests
- Mock LLM backend for agent tests
- Test files match implementation: `foo.go` → `foo_test.go`
- Focus on testing tool behavior, context management, budget calculations

## Common Gotchas

1. **Don't use shell commands for file operations**: Use `file` or `code_edit` tools
2. **Context budget includes everything**: System prompt + history + user prompt + response buffer
3. **Temp files location**: Always `/tmp/pedroceli-jobs/<jobID>-<timestamp>/`
4. **MCP is stdio-based**: Not HTTP. Use JSON-RPC messages via stdin/stdout
5. **Config file required**: Must have `.pedroceli.json` in project root or home directory
6. **llama.cpp path matters**: Must be absolute path to `llama-cli` binary

## Code Locations for Common Tasks

- **Adding a new tool**: Implement `tools.Tool` interface in `pkg/tools/`
- **Adding a new agent**: Extend `BaseAgent` in `pkg/agents/`
- **Modifying inference logic**: `pkg/agents/base.go:executeInference()`
- **Changing context budget**: `pkg/llm/tokens.go:CalculateBudget()`
- **Adding config options**: `pkg/config/config.go` + `.pedroceli.json.example`
- **Platform-specific code**: `pkg/platform/platform.go`

## Development Workflow

**IMPORTANT: Always run `make lint` before pushing!** The CI will fail if linting doesn't pass.

1. Make changes
2. Run `make fmt` to format code
3. Run `make lint` to check for issues (**MUST pass before pushing**)
4. Run `make test` to verify tests pass
5. Add tests for new functionality
6. Update `.pedroceli.json.example` if adding config options
7. Run `make lint` one more time before committing

### Linting

```bash
# Run linting (required before push)
make lint

# Fix linting issues automatically where possible
golangci-lint run --fix

# If you don't have golangci-lint installed
make setup  # Installs golangci-lint and other dev tools
```

**CI uses Go 1.25** - ensure your local Go version matches to avoid toolchain issues.
