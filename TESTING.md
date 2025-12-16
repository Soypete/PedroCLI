# Testing Guide for PedroCLI

This document describes all tests performed for the Phase 2 CLI PR and how to run manual tests.

---

## Automated Tests

### Unit Tests

All unit tests can be run with:

```bash
make test
```

**Test Coverage:**
- `pkg/config`: Configuration loading and validation
- `pkg/jobs`: Job creation, listing, status management
- `pkg/llm`: LLM backend (Ollama, llama.cpp) initialization and prompt building
- `pkg/llmcontext`: Context management and history tracking
- `pkg/platform`: Platform detection
- `pkg/tools`: All 7 tools (File, CodeEdit, Search, Navigate, Git, Bash, Test)

**Test Results (as of this PR):**
```
ok  	github.com/soypete/pedrocli/pkg/config
ok  	github.com/soypete/pedrocli/pkg/jobs
ok  	github.com/soypete/pedrocli/pkg/llm
ok  	github.com/soypete/pedrocli/pkg/llmcontext
ok  	github.com/soypete/pedrocli/pkg/platform
ok  	github.com/soypete/pedrocli/pkg/tools
```

### Build Tests

Test that all binaries build successfully:

```bash
# Clean build
make clean
make build

# Cross-platform builds
make build-mac
make build-linux
make build-linux-arm64
```

**Expected Output:**
- `pedrocli` - CLI binary (~3.7 MB on macOS arm64)
- `pedrocli-server` - MCP server binary (~3.6 MB on macOS arm64)

---

## Manual Tests

### Test 1: Help and Version (No Config Required)

These commands should work without a config file:

```bash
# Version
./pedrocli --version
# Expected: pedrocli version 0.2.0-dev

# Help
./pedrocli --help
# Expected: Shows usage, commands, flags, examples

# Command-specific help
./pedrocli build --help
# Expected: Shows build command flags (-description, -issue, -direct, -verbose)
```

**Why This Matters:** Users should be able to explore the CLI before setting up config.

---

### Test 2: Job Management Commands

Test job listing, status, and cancellation:

```bash
# List all jobs
./pedrocli list
# Expected: Shows list of jobs from /tmp/pedrocli-jobs/
# If no jobs: "No jobs found"
# If jobs exist: Table with ID, type, status, description

# Get job status (use a real job ID from list)
./pedrocli status job-1765770969
# Expected: Detailed job info (ID, type, status, created, started, completed times)

# Cancel a running job (if one exists)
./pedrocli cancel job-1765770323
# Expected: "Job job-1765770323 cancelled"
```

**Why This Matters:** Job management should work independently of agents/LLMs.

---

### Test 3: Dependency Checker

Test that missing dependencies are detected:

```bash
# Run with dependency checks (default)
./pedrocli build -description "Test task"
# Expected: If ollama or git missing, shows clear error with installation hints

# Skip dependency checks
./pedrocli build -description "Test task" --skip-checks
# Expected: Skips validation, proceeds to execution
```

**Why This Matters:** Users need clear guidance on what tools are required.

---

### Test 4: MCP Mode (Default)

Test that MCP client can spawn MCP server and call agents:

**Prerequisites:**
1. Create `.pedrocli.json` with valid config
2. Have Ollama or llama.cpp configured

```bash
# Test MCP mode with builder agent
./pedrocli build -description "Add a hello world function to /tmp/test_main.go"

# Expected behavior:
# 1. Loads config
# 2. Validates dependencies
# 3. Finds pedrocli-server binary
# 4. Spawns MCP server subprocess
# 5. Sends JSON-RPC request to builder agent
# 6. MCP server creates job and returns result
# 7. Displays job ID
```

**How to verify MCP is working:**
- Check that `pedrocli-server` process starts (use `ps aux | grep pedrocli-server`)
- Check `/tmp/pedrocli-jobs/` for new job directory
- Check job files for prompts and responses

**Common Issues:**
- `Error: MCP server binary not found` → Run `make build-server`
- `Error: timeout` → Check if LLM backend is accessible
- `Error: failed to start server` → Check logs in stderr

---

### Test 5: Direct Mode (--direct flag)

Test direct execution bypassing MCP:

```bash
# Test direct mode with verbose output
./pedrocli build -description "Add a hello world function to /tmp/test_main.go" --direct --verbose

# Expected behavior:
# 1. Loads config
# 2. Creates DirectExecutor
# 3. Creates agent in-process (no subprocess)
# 4. Shows real-time progress updates every 500ms
# 5. Displays job status changes (running → completed)
# 6. Shows final output
```

**Benefits of --direct mode:**
- 30-50% faster startup (no subprocess spawn)
- Real-time progress updates
- Verbose mode shows detailed logs

**How to verify:**
- Should NOT see `pedrocli-server` process in `ps aux`
- Should see progress updates in real-time
- Should complete faster than MCP mode

---

### Test 6: Ollama Backend (One-Shot)

Test Ollama backend with subprocess execution:

**Prerequisites:**
1. Install Ollama: `brew install ollama` or `curl -sS https://webi.sh/ollama | sh`
2. Pull a model: `ollama pull qwen3-coder:30b` (or any model)
3. Config:
```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen3-coder:30b",
    "context_size": 32768,
    "temperature": 0.2
  }
}
```

```bash
# Test with direct mode (faster feedback)
./pedrocli build -description "Add tests for a simple function" --direct --verbose

# Expected:
# 1. Spawns `ollama run qwen3-coder:30b "...prompt..."`
# 2. ANSI escape codes stripped from output
# 3. Clean LLM response returned
# 4. Agent processes response
```

**How to verify Ollama is working:**
- Run `ollama list` to see installed models
- Test manually: `ollama run qwen3-coder:30b "Hello, write a function that adds two numbers"`
- Check job directory for clean response (no ANSI codes like `[?25h`)

---

### Test 7: llama.cpp Backend (One-Shot)

Test llama.cpp backend with subprocess execution:

**Prerequisites:**
1. Build llama.cpp:
```bash
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make LLAMA_CUDA=1  # For NVIDIA GPUs, or just `make` for CPU
sudo cp llama-cli /usr/local/bin/
```

2. Download a model (GGUF format)
```bash
wget https://huggingface.co/Qwen/Qwen2.5-Coder-32B-Instruct-GGUF/resolve/main/qwen2.5-coder-32b-instruct-q4_k_m.gguf
```

3. Config:
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/path/to/qwen2.5-coder-32b-instruct-q4_k_m.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 32768,
    "n_gpu_layers": -1,
    "temperature": 0.2,
    "threads": 8
  }
}
```

```bash
# Test with direct mode
./pedrocli build -description "Add a simple test" --direct --verbose

# Expected:
# 1. Spawns `llama-cli -m /path/to/model.gguf -p "...prompt..." -c 32768`
# 2. Returns LLM response
# 3. Agent processes response
```

**How to verify llama.cpp is working:**
- Test manually: `llama-cli -m /path/to/model.gguf -p "Hello" -n 50`
- Check that GPU layers are used (if configured): Look for "using GPU" in output

---

### Test 8: Error Handling

Test that errors are clear and actionable:

```bash
# Missing config file
mv .pedrocli.json .pedrocli.json.bak
./pedrocli build -description "Test"
# Expected: Clear error about missing config, shows path checked

# Missing description
./pedrocli build
# Expected: "Error: -description is required"

# Invalid job ID
./pedrocli status invalid-job-id
# Expected: "Error: job not found"

# Model not found (Ollama)
# Edit config to use non-existent model "fake-model:7b"
./pedrocli build -description "Test" --skip-checks
# Expected: "ollama execution failed: model 'fake-model:7b' not found"
```

---

### Test 9: Configuration Variations

Test different configuration scenarios:

**Minimal config:**
```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen3-coder:30b"
  }
}
```
Expected: Uses defaults for context_size, temperature, etc.

**Development config:**
```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen3-coder:30b",
    "context_size": 32768,
    "temperature": 0.2
  },
  "debug": {
    "enabled": true,
    "keep_temp_files": true
  }
}
```
Expected: Keeps job files in `/tmp/pedrocli-jobs/` after completion

**Production config:**
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "llamacpp_path": "/usr/local/bin/llama-cli",
    "context_size": 131072,
    "n_gpu_layers": -1
  },
  "limits": {
    "max_task_duration_minutes": 60,
    "max_inference_runs": 50
  },
  "debug": {
    "enabled": false,
    "keep_temp_files": false
  }
}
```
Expected: Uses large context, GPU acceleration, strict limits, no debug files

---

## Integration Tests (Manual)

### End-to-End Test: Build Feature

**Goal:** Verify agent can complete a simple coding task

**Setup:**
1. Create test directory: `mkdir -p /tmp/test-pedrocli && cd /tmp/test-pedrocli`
2. Create test file:
```go
// main.go
package main

func main() {
    // TODO: Add hello world
}
```
3. Create config pointing to this directory

**Test:**
```bash
./pedrocli build -description "Replace the TODO comment with fmt.Println(\"Hello, World!\")" --direct --verbose
```

**Expected:**
1. Agent reads main.go
2. Agent edits file to add hello world
3. Job completes successfully
4. main.go now contains `fmt.Println("Hello, World!")`

**Note:** Full tool calling is not implemented yet (see TOOL_CALLING_PLAN.md), so this test will currently only show LLM response text, not actual file modifications.

---

## Performance Tests

### MCP vs Direct Mode Startup Time

Test the performance difference:

```bash
# MCP mode
time ./pedrocli build -description "Test task"

# Direct mode
time ./pedrocli build -description "Test task" --direct
```

**Expected Results:**
- MCP mode: ~1.5-2s startup (subprocess spawn overhead)
- Direct mode: ~0.2-0.5s startup (in-process execution)
- Direct mode should be 50-70% faster

### Ollama vs llama.cpp Inference Time

Compare inference performance:

```bash
# Ollama (measure time to first token)
time ./pedrocli build -description "Simple task" --direct --verbose

# llama.cpp (measure time to first token)
# Switch config to llamacpp
time ./pedrocli build -description "Simple task" --direct --verbose
```

**Expected:**
- Ollama: Varies by model, typically 2-5s for first token
- llama.cpp: Varies by hardware, can be faster with GPU

---

## Test Results Summary (This PR)

### Automated Tests
- ✅ All unit tests pass
- ✅ Clean build succeeds
- ✅ Cross-platform builds work (macOS, Linux)

### Manual Tests
- ✅ Help/version work without config
- ✅ Job management commands work (list, status, cancel)
- ✅ Dependency checker detects missing tools
- ✅ MCP server binary auto-discovery works
- ✅ Direct mode works with --direct flag
- ✅ Ollama one-shot subprocess execution works
- ✅ ANSI escape codes stripped correctly

### Known Issues
- ⚠️ Tool calling not implemented yet (agents don't execute tools)
- ⚠️ Agents do single inference then stop (no iterative loop)
- ⚠️ No integration tests for full task completion

These issues are documented in `TOOL_CALLING_PLAN.md` and will be addressed in a future PR.

---

## CI/CD Recommendations

For automated testing in CI:

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Build
        run: make build
      - name: Test
        run: make test
      - name: Test help (no config)
        run: ./pedrocli --help
      - name: Test list (no jobs)
        run: ./pedrocli list
```

---

## Troubleshooting Test Failures

### "MCP server binary not found"
**Solution:** Run `make build` to build both binaries

### "ollama: command not found"
**Solution:** Install Ollama or use `--skip-checks` flag

### "model 'qwen3-coder:30b' not found"
**Solution:** `ollama pull qwen3-coder:30b` or use a different model

### "timeout waiting for job creation"
**Solution:** Check config points to valid LLM backend, increase timeout in config

### Tests pass but CLI doesn't work
**Solution:** Check you're running the binary from the correct directory, verify config file exists

---

## Contributing Tests

When adding new features, please add:
1. Unit tests in `pkg/*/` directories
2. Manual test instructions in this document
3. Expected output examples

**Test Coverage Goals:**
- Unit tests: >70% coverage
- Integration tests: All major workflows
- Manual tests: All CLI commands
