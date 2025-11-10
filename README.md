# PedroCLI

Autonomous coding agent with MCP (Model Context Protocol) architecture.

## Project Status

### Completed ✅

**Phase 1**: Foundation & MCP Server Core
- [x] Go project structure and module setup
- [x] Configuration system with `.pedroceli.json`
- [x] Platform detection utilities (Mac/Linux/Windows)
- [x] File-based context manager (stores in `/tmp/pedroceli-jobs/`)
- [x] Token estimation and context budget calculator
- [x] Dependency checker with platform-specific validation
- [x] Job management system with disk persistence
- [x] Comprehensive tool system:
  - File tool (read, write, append, delete)
  - **CodeEdit tool** (precise line-based editing: edit_lines, insert_at_line, delete_lines)
  - **Search tool** (grep patterns, find files, find definitions with regex)
  - **Navigate tool** (code structure, file outlines, imports, directory trees)
  - Git tool (branch, commit, PR via `gh`)
  - Bash tool (restricted safe commands)
  - Test tool (Go/npm/Python)
- [x] MCP server protocol and stdio handler
- [x] Complete agent implementations:
  - [x] Builder agent (build features autonomously)
  - [x] Reviewer agent (blind code review)
  - [x] Debugger agent (debug and fix issues)
  - [x] Triager agent (diagnose without fixing)
- [x] Makefile for cross-compilation
- [x] Comprehensive test suite

**Phase 2**: CLI Client
- [x] CLI with stdlib flags (no external dependencies)
- [x] All 7 commands implemented (build, debug, review, triage, status, list, cancel)
- [x] Help and version commands
- [x] Config integration and dependency checking

**Phase 3**: MCP Client Integration & Ollama Backend
- [x] MCP client library (JSON-RPC 2.0 via stdio)
- [x] CLI-to-server communication working
- [x] llama.cpp backend implementation
- [x] **Ollama backend implementation** ✨
- [x] Backend factory for easy switching
- [x] Auto-detected context windows for 20+ Ollama models

### In Progress 🚧

- [ ] Inference loop with tool execution
- [ ] End-to-end testing with real agents

### Upcoming

- **Phase 4** (Weeks 5-6): Web UI with Voice Interface

## Architecture

```
┌─────────────────────────────────────────┐
│          MCP CLIENTS (Future)            │
│  ┌──────────────┐   ┌──────────────┐   │
│  │   CLI        │   │   Web UI     │   │
│  └──────────────┘   └──────────────┘   │
└─────────────────────────────────────────┘
                  │
          MCP Protocol (stdio)
                  │
                  ▼
┌─────────────────────────────────────────┐
│       PEDROCLI MCP SERVER                │
│                                          │
│  Agents:                                 │
│  ├─ Builder   (build features)          │
│  ├─ Debugger  (fix issues)              │
│  ├─ Reviewer  (code review)             │
│  └─ Triager   (diagnose)                │
│                                          │
│  Backend: llama.cpp / Ollama            │
└─────────────────────────────────────────┘
```

## Key Design Decisions

1. **MCP Architecture**: Pedroceli IS an MCP server (not wrapped)
2. **File-Based Context**: No in-memory context, writes to `/tmp/pedroceli-jobs/`
3. **One-Shot Inference**: Full context per inference, not conversational
4. **Cross-Platform**: Uses Go stdlib, not shell commands (`sed`/`grep`)
5. **Context-Aware**: Tracks tokens, loads strategically, compacts history
6. **Fail-Fast**: Checks all dependencies before starting work

## Configuration

Pedrocli supports two LLM backends: **llama.cpp** (for local inference) and **Ollama** (for easier setup).

### llama.cpp Backend

Create `.pedroceli.json` (see `.pedroceli.example.llamacpp.json`):

```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "usable_context": 24576,
    "n_gpu_layers": -1,
    "temperature": 0.2,
    "threads": 32
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/project"
  }
}
```

### Ollama Backend

For easier setup, use Ollama (see `.pedroceli.example.ollama.json`):

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "ollama_url": "http://localhost:11434",
    "temperature": 0.2
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/project"
  }
}
```

**Supported Ollama Models:**
- `qwen2.5-coder:7b/14b/32b` (32k context)
- `qwen2.5-coder:72b` (128k context)
- `deepseek-coder:33b` (16k context)
- `codellama:7b/13b/34b` (16k context)
- `llama3.1:8b/70b/405b` (128k context)
- And more...

Context windows are auto-detected for known models.

## Building

```bash
# Build for current platform
make build

# Build for Mac
make build-mac

# Build for Linux (DGX Spark / Ubuntu)
make build-linux

# Build MCP server
make build-server

# Run tests
make test

# Run linter
go run github.com/golangci/golangci-lint/cmd/golangci-lint run
```

## Usage

### Running on DGX Spark or Mini PC

The MCP server uses **stdio protocol** (JSON-RPC over stdin/stdout), not HTTP. This means you interact with it by sending JSON to stdin and receiving responses on stdout.

#### 1. Setup on DGX Spark / Ubuntu Server

```bash
# On your development machine (Mac)
make build-linux
scp pedroceli-linux-amd64 user@dgx-spark:/home/user/bin/pedroceli-server

# SSH into the server
ssh user@dgx-spark

# Create config file
cat > ~/.pedroceli.json <<'EOF'
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "usable_context": 24576,
    "n_gpu_layers": -1,
    "temperature": 0.2
  },
  "project": {
    "name": "My Project",
    "workdir": "/home/user/my-project"
  }
}
EOF

# Run the MCP server
cd /home/user/my-project
~/bin/pedroceli-server
```

#### 2. Invoking MCP Tools via JSON-RPC

The server accepts JSON-RPC 2.0 requests via stdin:

**List available tools:**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | pedroceli-server
```

**Call the reviewer agent to review code:**
```bash
echo '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "reviewer",
    "arguments": {
      "branch": "feature/my-feature"
    }
  }
}' | pedroceli-server
```

**Call the builder agent to build a feature:**
```bash
echo '{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "builder",
    "arguments": {
      "description": "Add rate limiting to the API",
      "issue": "GH-123"
    }
  }
}' | pedroceli-server
```

**Use basic file tool:**
```bash
echo '{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "file",
    "arguments": {
      "action": "read",
      "path": "main.go"
    }
  }
}' | pedroceli-server
```

#### 3. MCP Server + Client Architecture

**Note:** The MCP server uses stdio, not HTTP. It's designed to be called by MCP clients that manage the stdio communication. In Phase 2, we'll build a CLI client that makes this easier:

```bash
# Future Phase 2 CLI (not yet implemented)
pedroceli review --branch feature/my-feature
pedroceli build --description "Add rate limiting"
```

For now, you can:
1. **Use stdio directly** (as shown above with echo/pipes)
2. **Build a wrapper script** to simplify common operations
3. **Wait for Phase 2 CLI** (coming in Week 3)

#### 4. Example Wrapper Script

Create `~/bin/pedrocli` to make invocation easier:

```bash
#!/bin/bash
# Simple wrapper for PedroCLI MCP server

ACTION=$1
shift

case "$ACTION" in
  review)
    BRANCH=$1
    echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"reviewer\",\"arguments\":{\"branch\":\"$BRANCH\"}}}" | pedroceli-server
    ;;
  build)
    DESC=$1
    echo "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"builder\",\"arguments\":{\"description\":\"$DESC\"}}}" | pedroceli-server
    ;;
  list)
    echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | pedroceli-server
    ;;
  *)
    echo "Usage: $0 {review|build|list} [args]"
    exit 1
    ;;
esac
```

Then use it:
```bash
chmod +x ~/bin/pedrocli
pedrocli review feature/my-feature
pedrocli build "Add rate limiting"
```

### Why Not HTTP/curl?

MCP uses stdio for several reasons:
- **Process isolation**: Each request gets its own context
- **Simplicity**: No need for ports, auth, or HTTP overhead
- **Compatibility**: Works with MCP ecosystem (Claude Desktop, VS Code MCP, etc.)
- **Security**: No network exposure by default

If you need HTTP access, consider wrapping the MCP server in a simple HTTP server (Phase 4 will include a web UI that does this).

## Documentation

See `/docs` for complete implementation specifications:

- `START-HERE.md` - Overview and navigation
- `pedroceli-implementation-plan.md` - Complete 6-week plan (65KB)
- `pedroceli-context-guide.md` - Context window management
- `00-README.md` - Quick navigation hub

## Development Timeline

- **Weeks 1-2**: Phase 1 - MCP Server Core ← **Currently here**
- **Week 3**: Phase 2 - CLI Client
- **Week 4**: Phase 3 - Ollama Backend
- **Weeks 5-6**: Phase 4 - Web UI with Voice

---

Built with Go for autonomous, voice-driven coding
