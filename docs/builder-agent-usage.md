# Builder Agent Usage Guide

## Overview

The builder agent autonomously implements features by searching the codebase, reading relevant files, writing code, creating tests, and iterating until everything works.

## Basic Command Pattern

```bash
pedrocli build -description "<feature-description>" [-issue "<issue-number>"]
```

## Parameters

### `-description` (required)
A clear description of what to build. The more specific, the better results.

**Examples:**
```bash
# Basic feature
pedrocli build -description "Add a health check endpoint at /api/health"

# More detailed
pedrocli build -description "Add rate limiting middleware to protect against DoS attacks. Use 100 requests per minute per IP."

# Complex implementation
pedrocli build -description "Implement Prometheus metrics with HTTP request counts, latencies, and job success rates. Add /metrics endpoint."
```

### `-issue` (optional)
Reference a GitHub issue number. The agent will include this in commit messages and PR titles.

**Examples:**
```bash
# GitHub issue format
pedrocli build -description "Add user authentication" -issue "GH-123"

# Simple number format (recommended)
pedrocli build -description "Add Prometheus observability" -issue "32"
```

## Real-World Example: Implementing Prometheus Observability

This example shows the exact command used to test the builder agent on a complex, multi-file feature (GitHub issue #32).

### Setup

1. **Create configuration** (if using non-default config):
```bash
# Check what config will be used
ls -la .pedrocli.json      # Current directory (highest priority)
ls -la ~/.pedrocli.json    # Home directory (fallback)
```

2. **Create feature branch**:
```bash
git checkout -b feat/prometheus-observability-issue-32
```

3. **Build the CLI** (if working on pedrocli itself):
```bash
make build-cli
```

### The Command

```bash
./pedrocli build -issue "32" -description "Implement Prometheus observability metrics for issue #32. Create pkg/metrics package with HTTP, job, LLM, and tool metrics. Instrument server.go, handlers.go, db_manager.go, executor.go, ollama.go. Add /metrics and /api/ready endpoints. Write tests. Create PR when done."
```

### Command Breakdown

**Issue Reference:**
- `-issue "32"` - Links to GitHub issue #32, will be included in commits and PR title

**Description Structure:**
1. **Goal**: "Implement Prometheus observability metrics for issue #32"
2. **What to create**: "Create pkg/metrics package with HTTP, job, LLM, and tool metrics"
3. **What to modify**: "Instrument server.go, handlers.go, db_manager.go, executor.go, ollama.go"
4. **Specific requirements**: "Add /metrics and /api/ready endpoints"
5. **Testing**: "Write tests"
6. **Completion**: "Create PR when done"

### Expected Behavior

The agent will autonomously:

1. **Exploration (Rounds 1-4)**
   - Navigate codebase structure
   - Search for relevant patterns
   - Understand existing architecture

2. **Reading (Rounds 5-8)**
   - Read key files mentioned in description
   - Understand integration points
   - Review similar implementations

3. **Implementation (Rounds 9-22)**
   - Create new `pkg/metrics` package
   - Implement metric collectors
   - Modify integration points
   - Add configuration support

4. **Testing (Rounds 23-25)**
   - Write comprehensive tests
   - Run test suite
   - Fix any failures

5. **Build & Verification (Rounds 26-27)**
   - Run `make build` or `go build`
   - Ensure compilation succeeds
   - Fix any errors

6. **PR Creation (Rounds 28-29)**
   - Stage and commit changes
   - Push to remote
   - Create PR via `gh` CLI

7. **Completion (Round 30)**
   - Verify success
   - Output summary

### Monitoring Progress

**Real-time output:**
```bash
# The CLI will show:
⏳ Job <id> is running...
Checking status every 5 seconds. Press Ctrl+C to stop watching (job will continue in background).
🔄 Inference round 1/30
  🔍 Parsing tool calls from 12259 bytes of text using generic formatter
  📋 Parsed 0 tool call(s)
🔄 Inference round 2/30
  🔧 Executing tool: search
  ✅ Tool search succeeded
...
```

**Job files (for debugging):**
```bash
# Find the latest job
JOB_DIR=$(ls -td /tmp/pedrocli-jobs/*/ | head -n 1)

# View execution history
ls -la $JOB_DIR

# Check prompts and responses
cat $JOB_DIR/001-prompt.txt
cat $JOB_DIR/002-response.txt

# Check tool calls
cat $JOB_DIR/003-tool-calls.json | jq

# Check tool results
cat $JOB_DIR/004-tool-results.json | jq
```

## Writing Effective Descriptions

### ✅ Good Descriptions

**Specific and actionable:**
```bash
pedrocli build -description "Add /api/health endpoint that returns JSON with status and timestamp. Return 200 if healthy, 503 if database unreachable."
```

**Includes context:**
```bash
pedrocli build -description "Refactor user authentication to use JWT tokens instead of sessions. Update login handler, add token validation middleware, migrate existing session storage."
```

**Mentions constraints:**
```bash
pedrocli build -description "Add Redis caching layer for API responses. Use 5-minute TTL, cache GET requests only, follow existing database patterns in pkg/storage."
```

### ❌ Poor Descriptions

**Too vague:**
```bash
pedrocli build -description "Make it faster"
```

**Too broad:**
```bash
pedrocli build -description "Add authentication"
```

**Missing context:**
```bash
pedrocli build -description "Fix the bug"
```

## Configuration for Builder Tasks

### Recommended Settings

For complex implementation tasks, adjust these config values:

```json
{
  "limits": {
    "max_task_duration_minutes": 60,
    "max_inference_runs": 30
  },
  "tools": {
    "allowed_bash_commands": [
      "go", "git", "make", "gh", "npm", "curl", "grep",
      "cat", "ls", "head", "tail", "wc", "sort", "uniq"
    ]
  },
  "debug": {
    "enabled": true,
    "keep_temp_files": true,
    "log_level": "info"
  }
}
```

**Key settings:**
- `max_inference_runs: 30` - Complex tasks may need more iterations
- `max_task_duration_minutes: 60` - Allow sufficient time for large changes
- `allowed_bash_commands` - Include `gh` for PR creation, `make` for builds
- `keep_temp_files: true` - Preserve job files for debugging

### Model Selection

**For complex implementations:**
- **Recommended**: Qwen2.5-Coder-32B or larger
- **Minimum**: Qwen2.5-Coder-7B with GBNF grammar enabled

**GBNF Grammar** (for llama.cpp):
Automatically enabled when using tool registry. Constrains model output to valid tool calls, dramatically improving accuracy for smaller models.

## Troubleshooting

### Agent Doesn't Create PR

**Symptom:** Implementation complete but no PR created

**Check:**
1. Is `gh` in `allowed_bash_commands`?
2. Is GitHub CLI authenticated? (`gh auth status`)
3. Check tool results for auth errors

**Fix:**
```bash
# Authenticate GitHub CLI
gh auth login

# Or create PR manually
git add .
git commit -m "feat: <description> (#<issue>)"
git push origin <branch>
gh pr create --title "feat: <description> (#<issue>)"
```

### Tests Fail and Agent Doesn't Fix

**Symptom:** Tests written but failing, agent gives up

**Check:**
1. Review test failures in job files
2. Check if agent had enough inference rounds
3. Look for tool execution errors

**Fix:**
- Increase `max_inference_runs`
- Add explicit instruction: "Iterate until all tests pass"
- Break task into smaller pieces

### Agent Loops Without Progress

**Symptom:** Same tool calls repeated across rounds

**Check:**
1. Tool results for persistent errors
2. Context window exhaustion
3. Model confusion

**Fix:**
- Simplify the description
- Use a larger model
- Check for file permission issues

## Advanced Usage

### Multi-Step Implementation

For very large features, break into phases:

```bash
# Phase 1: Core infrastructure
pedrocli build -issue "32" -description "Create pkg/metrics package with registry and basic HTTP metrics"

# Phase 2: Job metrics
pedrocli build -issue "32" -description "Add job lifecycle metrics to pkg/metrics, instrument db_manager.go"

# Phase 3: LLM metrics
pedrocli build -issue "32" -description "Add LLM timing metrics, instrument ollama.go to record latencies and token counts"
```

### Combining with Other Agents

```bash
# 1. Build the feature
pedrocli build -description "Add user registration endpoint"

# 2. Review your own implementation
pedrocli review -branch feat/user-registration

# 3. Debug if tests fail
pedrocli debug -symptoms "Registration tests failing" -logs test.log
```

## Success Metrics

Track these to evaluate autonomous performance:

1. **Completion Rate**: Did agent finish without human intervention?
2. **Accuracy**: What % of requirements were implemented correctly?
3. **Efficiency**: How many inference rounds needed? (lower is better)
4. **Code Quality**: Does code follow project patterns?
5. **Test Coverage**: Did agent write comprehensive tests?
6. **PR Quality**: Is PR well-formatted and ready for review?

## See Also

- [Agent Architecture](../CLAUDE.md#core-architecture) - How agents work internally
- [Tool Documentation](../CLAUDE.md#package-structure) - Available tools and capabilities
- [Configuration Guide](../docs/pedrocli-context-guide.md) - Context window management
