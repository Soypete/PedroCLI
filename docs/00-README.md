# Pedroceli Specifications

Complete implementation specifications for the Pedroceli autonomous coding agent.

## 📋 Quick Navigation

This directory contains implementation specifications and planning documents for the Pedroceli autonomous coding agent. These are internal planning documents for development reference.

## 🎯 Implementation Order

### Week 1: Foundation
1. Project setup (Go modules, structure)
2. Config parsing
3. Dependency checker
4. Context manager (file-based)
5. llama.cpp client
6. Basic tool system

### Week 2: Agents & MCP
1. MCP server protocol
2. Build agent
3. Debug agent
4. Review agent
5. Triage agent
6. Job management
7. Integration testing

### Week 3: CLI
1. Cobra CLI setup
2. MCP client library
3. All CLI commands
4. Status/monitoring
5. End-to-end testing

### Week 4: Ollama
1. Ollama client
2. Backend factory
3. Config switching
4. Testing both backends

### Week 5-6: Web UI
1. Web server setup
2. Voice recording UI
3. Whisper.cpp integration
4. MCP client (web)
5. Tailscale deployment
6. Mobile testing

## 📦 Project Structure

```
pedroceli/
├── cmd/
│   ├── mcp-server.go      # MCP server (Phase 1)
│   ├── cli.go             # CLI client (Phase 2)
│   └── web.go             # Web server (Phase 4)
├── pkg/
│   ├── mcp/               # MCP protocol
│   ├── agents/            # 4 agents
│   ├── llm/               # Backends (llama.cpp, Ollama)
│   ├── tools/             # File, bash, git, test tools
│   ├── context/           # Context management
│   ├── init/              # Dependency checking
│   ├── jobs/              # Job state
│   ├── platform/          # OS detection
│   ├── config/            # Config parsing
│   ├── metrics/           # Prometheus
│   └── stt/               # Whisper (Phase 4)
├── web/
│   ├── static/
│   └── api/
├── docs/                  # THIS FOLDER (all specs)
├── .pedroceli.json
├── Makefile
└── README.md
```

## 🚀 Getting Started

For Claude Code implementing this:

1. **Read Phase 1 Overview first** - Understand the foundation
2. **Week by week** - Follow the phase documents in order
3. **Component specs** - Reference as needed for details
4. **Test after each phase** - Don't move on until phase works

## 💡 Key Design Decisions

1. **MCP Architecture** - Pedroceli IS an MCP server (not wrapped by one)
2. **File-Based Context** - No in-memory context, write to /tmp
3. **One-Shot Inference** - Full context per inference, not conversational
4. **Cross-Platform** - Use Go stdlib, not shell commands (sed/grep)
5. **Context-Aware** - Track tokens, load strategically, compact history
6. **Fail Fast** - Check all dependencies before starting work

## 📝 Configuration Example

```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "usable_context": 24576
  },
  "execution": {
    "run_on_spark": true,
    "spark_ssh": "miriah@dgx-spark-01"
  },
  "git": {
    "always_draft_pr": true,
    "branch_prefix": "pedroceli/"
  },
  "debug": {
    "enabled": false,
    "keep_temp_files": false
  }
}
```

## 🎬 Usage Examples

### CLI
```bash
# Build feature
pedroceli build --description "Add rate limiting"

# Debug issue
pedroceli debug --symptoms "Bot crashes on startup"

# Review PR
pedroceli review --branch feature/rate-limiting

# Check status
pedroceli status
```

### Web UI
```
1. Open phone browser: https://spark.tailnet.ts.net:8080
2. Tap microphone
3. Speak: "Build a webhook validation feature"
4. Review transcription
5. Tap "Start Job"
6. Go to sleep
7. Wake up to draft PR
```

## 🧪 Testing Strategy

Each phase includes:
- Unit tests (Go test)
- Integration tests (end-to-end)
- Manual testing checklist

## 📊 Success Criteria

**Phase 1**: MCP server responds to stdio, all agents work
**Phase 2**: CLI commands work end-to-end
**Phase 3**: Can switch between llama.cpp and Ollama
**Phase 4**: Voice interface works on phone via Tailnet

## 🤝 Contributing

This is Miriah's personal project for the Pedro bot, but the specs are detailed enough for:
- Claude Code to implement
- Other developers to understand
- Future you to remember why decisions were made

## 📞 Support

Questions? Check the detailed specs in each phase folder.

---

Built with ❤️ for autonomous coding
