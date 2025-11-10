# PedroCLI

**Self-hosted autonomous coding agent - an open-source alternative to Cursor's background jobs using your own models**

PedroCLI is a command-line tool that runs autonomous coding agents using self-hosted LLMs (via llama.cpp or Ollama). Think of it as Cursor's background job system, but fully open-source and running on your own hardware with your own models.

## What is This?

This project provides **autonomous coding agents** that can:
- ✅ Build new features from descriptions
- ✅ Debug and fix issues automatically
- ✅ Perform code reviews on PRs
- ✅ Triage and diagnose problems

All powered by **your own self-hosted LLMs** - no API calls, no subscriptions, complete privacy.

## Quick Start

```bash
# 1. Install Ollama (easiest option)
curl -fsSL https://ollama.com/install.sh | sh
ollama pull qwen2.5-coder:32b

# 2. Build PedroCLI
make build

# 3. Create config file
cp .pedroceli.example.ollama.json .pedroceli.json
# Edit .pedroceli.json to set your project path

# 4. Run your first agent
./pedrocli build -description "Add a health check endpoint to the API"
```

The agent will autonomously:
1. Search your codebase
2. Read relevant files
3. Write the implementation
4. Create tests
5. Run the tests (and fix if they fail)
6. Keep iterating until everything works
7. Create a git branch and commit the changes

## Project Status - ✅ FULLY FUNCTIONAL

### ✅ Completed - Core System

**Phase 1**: Foundation & MCP Server
- [x] MCP server architecture with JSON-RPC 2.0
- [x] File-based context management
- [x] 7 comprehensive tools (file, code_edit, search, navigate, git, bash, test)
- [x] 4 agent types (Builder, Debugger, Reviewer, Triager)

**Phase 2**: CLI Client
- [x] Command-line interface with 7 commands
- [x] MCP client library
- [x] Full CLI-to-server communication

**Phase 3**: LLM Backends
- [x] llama.cpp backend (maximum performance)
- [x] Ollama backend (maximum convenience)
- [x] Auto-detected context windows for 20+ models

**Phase 4**: Inference Loop ✨ **NEW**
- [x] **Complete iterative inference loop**
- [x] **Tool call parsing and execution**
- [x] **Result feedback to LLM**
- [x] **Automatic retry on failures**
- [x] **Completion detection**

### What Works Right Now

The system is **fully functional** for autonomous coding tasks. Agents can:
- Search and navigate codebases
- Read and edit files with precision
- Run tests and analyze failures
- Fix issues and iterate until success
- Create git branches and commits
- Work completely autonomously from start to finish

### Known Limitations

⚠️ **Prompt Engineering Required**: The prompts work but will need refinement for optimal results with different models. Pull requests are **welcome**, but I reserve the right to reject changes that don't align with the project's goals.

⚠️ **Model Quality Matters**: Better models = better results. Recommended: Qwen 2.5 Coder 32B or larger.

⚠️ **Tool Call Format**: Agents must output JSON in the correct format. Some models do this better than others.

## Installation & Setup

### Option 1: Ollama (Recommended for Beginners)

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Pull a coding model
ollama pull qwen2.5-coder:32b  # or :14b, :7b for smaller hardware

# Build PedroCLI
git clone https://github.com/Soypete/PedroCLI
cd PedroCLI
make build

# Create config
cp .pedroceli.example.ollama.json .pedroceli.json
```

Edit `.pedroceli.json`:
```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "temperature": 0.2
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/your/project"
  }
}
```

### Option 2: llama.cpp (Maximum Performance)

For maximum performance on GPU:

```bash
# Install llama.cpp
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make LLAMA_CUDA=1  # or LLAMA_METAL=1 for Mac

# Download a GGUF model
# Example: Qwen 2.5 Coder 32B from HuggingFace

# Build PedroCLI
cd /path/to/PedroCLI
make build

# Create config
cp .pedroceli.example.llamacpp.json .pedroceli.json
```

Edit `.pedroceli.json`:
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/path/to/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/path/to/llama-cli",
    "context_size": 32768,
    "n_gpu_layers": -1,
    "temperature": 0.2,
    "threads": 32
  },
  "project": {
    "name": "My Project",
    "workdir": "/path/to/your/project"
  }
}
```

## Usage

PedroCLI provides 7 commands:

### 1. Build Features

```bash
# Build a new feature
./pedrocli build -description "Add rate limiting to the API"

# With issue tracking
./pedrocli build -description "Add user authentication" -issue "GH-123"
```

The agent will:
- Understand requirements
- Search for relevant files
- Implement the feature
- Add tests
- Run tests (and fix if they fail)
- Keep trying until tests pass
- Commit to a new branch

### 2. Debug Issues

```bash
# Debug a problem
./pedrocli debug -symptoms "API returns 500 on POST /users"

# With log files
./pedrocli debug -symptoms "Memory leak" -logs error.log
```

The agent will:
- Analyze symptoms
- Search for the root cause
- Reproduce the issue
- Implement a fix
- Run tests to verify
- Keep iterating until fixed
- Commit the fix

### 3. Review Code

```bash
# Review a PR
./pedrocli review -branch feature/new-api

# Review with PR number
./pedrocli review -branch feature/auth -pr 42
```

The agent will:
- Get the git diff
- Analyze changes
- Look for bugs, security issues, performance problems
- Provide detailed feedback

### 4. Triage Issues

```bash
# Diagnose without fixing
./pedrocli triage -description "Users reporting slow page loads"
```

### 5. Monitor Jobs

```bash
# Check job status
./pedrocli status job-1234567890

# List all jobs
./pedrocli list

# Cancel a running job
./pedrocli cancel job-1234567890
```

## How It Works

### Architecture

```
┌─────────────────────────────────────┐
│         pedrocli (CLI)               │
│   Commands: build, debug, review     │
└─────────────────────────────────────┘
              │
        JSON-RPC (stdio)
              ▼
┌─────────────────────────────────────┐
│      pedrocli-server (MCP)           │
├─────────────────────────────────────┤
│  Agents:                             │
│  ├─ Builder   (build features)       │
│  ├─ Debugger  (fix bugs)             │
│  ├─ Reviewer  (code review)          │
│  └─ Triager   (diagnose)             │
├─────────────────────────────────────┤
│  Tools: file, code_edit, search,     │
│         navigate, git, bash, test    │
├─────────────────────────────────────┤
│  Backend: llama.cpp OR Ollama        │
└─────────────────────────────────────┘
              │
        Your Models ▼
  qwen2.5-coder, deepseek-coder, etc.
```

### The Inference Loop

The key to autonomous operation is the **inference loop** in `/pkg/agents/executor.go`:

1. **Agent receives task** (e.g., "Build rate limiting")
2. **LLM analyzes and plans** what tools to use
3. **Parser extracts tool calls** from LLM response (JSON format)
4. **Executor runs tools** (search files, read code, edit files, run tests)
5. **Results feed back to LLM** with success/failure information
6. **LLM analyzes results** and decides next steps
7. **Repeat until task complete** or max iterations reached

The agent keeps trying until:
- All tests pass
- The task is fully complete
- It explicitly says "TASK_COMPLETE"
- Or max iterations reached (configurable)

## Supported Models

### Ollama Models (Recommended)

The system auto-detects context windows for:

- **Qwen 2.5 Coder** (recommended): `7b`, `14b`, `32b` (32k), `72b` (128k)
- **DeepSeek Coder**: `33b` (16k)
- **Code Llama**: `7b`, `13b`, `34b` (16k)
- **Llama 3.1**: `8b`, `70b`, `405b` (128k)
- **Mistral**: `7b` (32k)
- **Mixtral**: `8x7b` (32k), `8x22b` (64k)
- And more...

### Model Recommendations

- **Best Results**: Qwen 2.5 Coder 32B or 72B
- **Good Balance**: Qwen 2.5 Coder 14B
- **Lightweight**: Qwen 2.5 Coder 7B (less reliable)

## Configuration Reference

Full config example:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "ollama_url": "http://localhost:11434",
    "temperature": 0.2
  },
  "execution": {
    "run_on_spark": false,
    "spark_ssh": ""
  },
  "git": {
    "always_draft_pr": true,
    "branch_prefix": "pedrocli/",
    "remote": "origin"
  },
  "tools": {
    "allowed_bash_commands": [
      "go", "git", "cat", "ls", "head", "tail", "wc", "sort", "uniq"
    ],
    "forbidden_commands": [
      "sed", "grep", "find", "xargs", "rm", "mv", "dd", "sudo"
    ]
  },
  "project": {
    "name": "PedroCLI",
    "workdir": "/home/user/PedroCLI",
    "tech_stack": ["Go"]
  },
  "limits": {
    "max_task_duration_minutes": 30,
    "max_inference_runs": 20
  },
  "debug": {
    "enabled": false,
    "keep_temp_files": false,
    "log_level": "info"
  },
  "init": {
    "skip_checks": false,
    "verbose": false
  }
}
```

## Development

```bash
# Build
make build          # Current platform
make build-mac      # Mac (arm64 + amd64)
make build-linux    # Linux

# Test
make test           # Run all tests
make test-coverage  # With coverage

# Format & Lint
make fmt
make lint
```

## Contributing

**Pull requests are welcome!** However:

✅ **Welcomed PRs:**
- Prompt improvements (especially for specific models)
- Bug fixes
- Performance optimizations
- Documentation improvements
- New tool implementations
- Test coverage increases

❌ **Will be Rejected:**
- Changes that remove self-hosting capability
- Dependencies on external APIs
- Breaking changes to the MCP protocol
- Unnecessary complexity

I reserve the right to reject PRs that don't align with the project's goals of being a self-hosted, privacy-focused Cursor alternative.

## Troubleshooting

### "Tool call failed"
- Check that your model supports JSON output
- Try temperature=0.0 for more deterministic output
- Use a larger model (32B+ recommended)

### "Max iterations reached"
- Increase `max_inference_runs` in config
- The task might be too complex - break it down
- Try a more capable model

### "Tests keep failing"
- The agent will retry automatically
- Check the job directory in `/tmp/pedroceli-jobs/` for full history
- Set `debug.keep_temp_files: true` to inspect what happened

### "No response from model"
- Check that Ollama is running: `ollama list`
- For llama.cpp, verify the binary path is correct
- Check context size isn't too large for your GPU

## Philosophy & Design

### Why Self-Hosted?

1. **Privacy**: Your code never leaves your machine
2. **Cost**: No per-token charges
3. **Control**: Use any model you want
4. **Offline**: Works without internet
5. **Customization**: Full control over prompts and behavior

### Why MCP?

The Model Context Protocol (MCP) provides:
- Clean separation between client and server
- Tool-based architecture (like OpenAI function calling)
- Extensibility for future integrations
- Process isolation and security

### File-Based Context

Unlike in-memory systems, PedroCLI writes all context to `/tmp/pedroceli-jobs/`:
- Survives process crashes
- Easy to debug (just read the files)
- Natural context window management
- Clear history of what the agent did

## License

MIT - Use it however you want. Build cool things.

## Acknowledgments

Inspired by Cursor's AI features but focused on open-source self-hosting for maximum privacy and control.

---

**Built for developers who want autonomous coding assistants without giving up their code or paying per token.**
