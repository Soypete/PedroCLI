# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PedroCLI is a self-hosted autonomous coding agent - an open-source alternative to Cursor's background jobs using your own LLMs. It runs coding agents via llama.cpp or Ollama to build features, debug issues, review code, and triage problems completely autonomously.

**Tech Stack**: Go 1.24.7, JSON-RPC 2.0, Model Context Protocol (MCP)

## Architecture

### Core Components

1. **MCP Server** (`pkg/mcp/server.go`): JSON-RPC 2.0 server that exposes tools via stdio transport
2. **Agents** (`pkg/agents/`): Autonomous agents that use tools to complete tasks
   - Builder: Builds new features from descriptions
   - Debugger: Fixes bugs and issues
   - Reviewer: Performs code reviews on PRs
   - Triager: Diagnoses problems without fixing
3. **Inference Loop** (`pkg/agents/executor.go`): The core autonomous loop that:
   - Sends prompts to LLM
   - Parses JSON tool calls from responses
   - Executes tools
   - Feeds results back to LLM
   - Repeats until task completion or max iterations
4. **Tools** (`pkg/tools/`): Seven specialized tools for code interaction
5. **Context Manager** (`pkg/llmcontext/manager.go`): File-based context in `/tmp/pedroceli-jobs/`
6. **LLM Backends** (`pkg/llm/`): Support for llama.cpp and Ollama

### Tool System

Seven tools available to agents (all implement `tools.Tool` interface):

- **file**: Read, write, and modify entire files
- **code_edit**: Precise line-based editing (edit/insert/delete specific lines)
- **search**: Search code (grep patterns, find files, find definitions)
- **navigate**: Navigate code structure (list directories, get file outlines, find imports)
- **git**: Execute git commands (status, diff, commit, push, etc.)
- **bash**: Run safe shell commands (limited to allowed commands in config)
- **test**: Run tests and parse results (Go, npm, Python)

### Key Design Decisions

1. **File-Based Context**: All context is written to `/tmp/pedroceli-jobs/<job-id>/` for crash resilience and debugging
2. **One-Shot Inference Loop**: Each inference includes full context (not conversational)
3. **JSON Tool Calls**: LLM outputs JSON objects with `{"tool": "name", "args": {...}}` format
4. **Context Budget Management**: Keeps recent history, summarizes old history to fit context window
5. **Cross-Platform**: Uses Go stdlib instead of shell commands (sed/grep/find) for portability

## Common Development Commands

### Build
```bash
make build              # Build both CLI and server for current platform
make build-cli          # Build CLI only
make build-server       # Build MCP server only
make build-mac          # Build for macOS (arm64 + amd64)
make build-linux        # Build for Linux (amd64)
```

### Testing
```bash
make test               # Run all tests
make test-coverage      # Run tests with coverage report
go test -v ./pkg/tools  # Run tests for specific package
```

### Code Quality
```bash
make fmt                # Format code
make lint               # Run golangci-lint
make tidy               # Tidy dependencies
```

### Running
```bash
make run-server         # Run MCP server (stdio mode)
make run-cli            # Run CLI directly
./pedrocli build -description "Add feature X"    # Execute a build agent
./pedrocli debug -symptoms "Bug description"     # Execute a debugger agent
./pedrocli review -branch feature/xyz            # Execute a code review
```

## Configuration

Config file: `.pedrocli.json` (in project root or home directory)

Key settings:
- `model.type`: "llamacpp" or "ollama"
- `model.context_size`: Total context window (auto-detected for Ollama models)
- `project.workdir`: Path to target project for agents to work on
- `limits.max_inference_runs`: Max iterations before timeout (default: 20)
- `tools.allowed_bash_commands`: Whitelist of safe bash commands
- `tools.forbidden_commands`: Blacklist (overrides whitelist)

See `.pedroceli.example.ollama.json` or `.pedroceli.example.llamacpp.json` for templates.

## Inference Loop Architecture

The autonomous operation happens in `pkg/agents/executor.go`:

1. **Agent receives task** → creates job and context manager
2. **Execute inference** → LLM analyzes task with system prompt + user prompt + history
3. **Parse tool calls** → Regex extracts JSON from response (code blocks or inline)
4. **Execute tools** → Each tool runs and returns success/failure + output
5. **Save results** → Context manager writes to files in job directory
6. **Build feedback** → Results formatted into next prompt
7. **Check completion** → Look for "TASK_COMPLETE" signal or PR creation
8. **Repeat** → Loop until done or max iterations reached

Context files in `/tmp/pedroceli-jobs/<job-id>/`:
- `001-prompt.txt`, `002-response.txt` (alternating)
- `003-tool-calls.json`, `004-tool-results.json`
- Numbered sequentially for full audit trail

## Code Organization

```
cmd/
├── pedrocli/       # Main CLI entrypoint
├── mcp-server/     # MCP server entrypoint (not typically run directly)
└── cli/            # Alternative CLI (legacy)

pkg/
├── agents/         # Agent implementations and inference executor
├── config/         # Configuration parsing and validation
├── jobs/           # Job state management
├── llm/            # LLM backends (llamacpp, ollama, factory)
├── llmcontext/     # File-based context manager with compaction
├── mcp/            # MCP protocol (JSON-RPC 2.0 server/client)
├── platform/       # OS detection for cross-platform support
├── tools/          # Seven tool implementations
└── init/           # Dependency checking

docs/               # Detailed phase-by-phase specifications
```

## Testing Patterns

1. **Tool Tests**: Mock context, test Execute() with various args
2. **Agent Tests**: Mock LLM backend, verify tool calls
3. **Context Tests**: File operations, token budget calculations
4. **Integration Tests**: End-to-end with real LLM (expensive, manual)

Test files follow `*_test.go` convention and are located alongside source files.

## Important Implementation Notes

### Tool Call Parsing
- LLM must output JSON in specific format: `{"tool": "name", "args": {...}}`
- Parser looks for JSON code blocks first: ` ```json\n{...}\n``` `
- Falls back to inline JSON if no code blocks found
- Some models do this better than others (Qwen 2.5 Coder recommended)

### Context Management
- Context manager creates numbered files for full history
- `GetHistoryWithinBudget()` keeps recent 3 inference rounds
- Older rounds are summarized (list of tool calls + modified files)
- Token estimation uses 1 token ≈ 4 characters heuristic

### Tool Execution
- All tools run in target project's `workdir` (from config)
- Bash tool enforces whitelist/blacklist for safety
- Git tool automatically creates branches with `pedroceli/` prefix
- Test tool detects framework and parses output for pass/fail

### Agent Prompts
- System prompt defines available tools and best practices (in `base.go`)
- User prompt is task-specific (build description, debug symptoms, etc.)
- Each agent type can override system prompt for specialized behavior
- Prompts are designed for tool use, not conversation

## LLM Backend Details

### llama.cpp Backend
- Executes `llama-cli` binary via subprocess
- Streams output, no chat API needed
- Full control over context size, GPU layers, threads
- Best for: Maximum performance, GPU utilization

### Ollama Backend
- HTTP API client to local Ollama server (default: `http://localhost:11434`)
- Auto-detects context windows for 20+ models
- Model names like `qwen2.5-coder:32b`, `deepseek-coder:33b`
- Best for: Convenience, easy model switching

Both backends implement `llm.Backend` interface with `Infer()` method.

## Common Gotchas

1. **Tool calls must be JSON**: If LLM outputs natural language instead of JSON, it fails silently
2. **Context overflow**: If context exceeds window, inference fails - context manager should prevent this
3. **Forbidden bash commands**: Using `sed`, `grep`, `find` in bash tool will fail - use dedicated tools instead
4. **Branch conflicts**: Agents create branches with timestamp suffix to avoid collisions
5. **File paths**: All file operations use absolute paths from `project.workdir`

## Debugging Tips

- Set `debug.enabled: true` and `debug.keep_temp_files: true` in config
- Check `/tmp/pedroceli-jobs/` for full inference history
- Look at numbered files to see exact LLM inputs/outputs
- Use `debug.log_level: "debug"` for verbose logging
- Test individual tools via MCP server's JSON-RPC interface

## Model Recommendations

**Best results**: Qwen 2.5 Coder 32B or 72B
**Good balance**: Qwen 2.5 Coder 14B
**Lightweight**: Qwen 2.5 Coder 7B (less reliable)

Temperature 0.2 works well for deterministic output. Lower (0.0-0.1) for more consistent JSON formatting.

## Related Documentation

- `README.md`: User-facing documentation and quick start
- `docs/`: Detailed phase-by-phase implementation specs
- `.pedroceli.example.*.json`: Configuration templates
- `pkg/*/`: Inline godoc comments for API details
