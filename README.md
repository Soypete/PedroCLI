# PedroCLI

Autonomous coding agent with MCP (Model Context Protocol) architecture.

## Project Status

**Phase 1 (In Progress)**: Foundation & MCP Server Core

### Completed ✅

- [x] Go project structure and module setup
- [x] Configuration system with `.pedroceli.json`
- [x] Platform detection utilities (Mac/Linux/Windows)
- [x] File-based context manager (stores in `/tmp/pedroceli-jobs/`)
- [x] Token estimation and context budget calculator
- [x] Dependency checker with platform-specific validation
- [x] llama.cpp client with one-shot inference
- [x] Job management system with disk persistence
- [x] Cross-platform tool system:
  - File tool (pure Go, no `sed`)
  - Git tool (branch, commit, PR via `gh`)
  - Bash tool (restricted safe commands)
  - Test tool (Go/npm/Python)
- [x] MCP server protocol and stdio handler
- [x] Base agent architecture
- [x] Builder agent (initial implementation)
- [x] Makefile for cross-compilation

### In Progress 🚧

- [ ] Complete agent implementations:
  - [ ] Debugger agent
  - [ ] Reviewer agent
  - [ ] Triager agent
- [ ] Inference loop with tool execution
- [ ] End-to-end testing

### Upcoming

- **Phase 2** (Week 3): CLI Client with Cobra
- **Phase 3** (Week 4): Ollama Backend
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

Create `.pedroceli.json` (see `.pedroceli.json.example`):

```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "usable_context": 24576,
    "temperature": 0.2
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/project"
  }
}
```

## Building

```bash
# Build for current platform
make build

# Build for Mac
make build-mac

# Build for Linux
make build-linux

# Build MCP server
make build-server

# Run tests
make test
```

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
