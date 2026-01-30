# Context Manager Logging Fix - Verification Guide

## What Was Fixed

Interactive mode (pedrocode REPL) now logs tool calls and results to context manager files in `/tmp/pedroceli-jobs/`, enabling debugging of phased executor workflows.

### Changes Made

1. **pkg/agents/phased_executor.go** (lines 371-430):
   - Added `contextMgr.SaveToolCalls()` BEFORE tool execution
   - Added `contextMgr.SaveToolResults()` AFTER tool execution
   - Mirrors the pattern in standard executor (executor.go:204-234)

2. **pkg/agents/phased_executor_context_test.go** (NEW FILE):
   - Unit tests verifying context manager logging
   - Tests for graceful nil handling
   - Tests for file sequencing

## How to Verify

### Step 1: Build the New Binary

```bash
make build-pedrocode
```

### Step 2: Start Interactive Mode with Debug

```bash
./pedrocode --debug
```

### Step 3: Run a Simple Task

In the REPL, run a simple interactive build task:

```
pedro:build> add a print statement to main.go that says "hello interactive mode"
```

### Step 4: Check for Context Files

After the task completes (or fails), check for context files:

```bash
# Find the latest interactive job directory
ls -lt /tmp/pedroceli-jobs/ | head -5

# Look inside the directory
ls -la /tmp/pedroceli-jobs/interactive-<timestamp>/

# You should see files like:
# 001-prompt.txt
# 002-response.txt
# 003-tool-calls.json
# 004-tool-results.json
# 005-prompt.txt
# etc.
```

### Step 5: Inspect Tool Call Files

```bash
# View tool calls
cat /tmp/pedroceli-jobs/interactive-<timestamp>/003-tool-calls.json

# Expected format:
# [
#   {
#     "name": "search",
#     "args": {
#       "pattern": "main.go",
#       "path": "."
#     }
#   }
# ]

# View tool results
cat /tmp/pedroceli-jobs/interactive-<timestamp>/004-tool-results.json

# Expected format:
# [
#   {
#     "name": "search",
#     "success": true,
#     "output": "Found: ./main.go",
#     "modified_files": []
#   }
# ]
```

### Step 6: Verify Validate Phase (If It Reaches That Phase)

If the task progresses through analyze → plan → implement → validate:

```bash
# Find validate phase rounds
cd /tmp/pedroceli-jobs/interactive-<timestamp>/
grep -l "validate" *.txt

# Check what tools were called during validation
grep -A 10 "validate" <response-file-numbers>.txt

# Look for patterns:
# - Are tests passing? (check tool-results.json for test tool calls)
# - Is LLM calling the same tools repeatedly?
# - Did LLM output PHASE_COMPLETE when tests passed?
```

## Expected Outcomes

### ✅ Success Indicators

1. **Context files exist** in `/tmp/pedroceli-jobs/interactive-<timestamp>/`
2. **Tool calls are saved** to `*-tool-calls.json` files
3. **Tool results are saved** to `*-tool-results.json` files
4. **Files are numbered sequentially** (001, 002, 003, ...)
5. **Both interactive and async modes work** (async mode already had this working)

### ❌ Failure Indicators

1. **No context files created** - Check if `--debug` flag is set
2. **Empty tool call/result files** - Check error messages in terminal
3. **Missing files** - Check for file write errors
4. **Nil pointer errors** - Check test results

## Next Steps: Debugging Validate Phase Issue

Once context manager logging is verified, use the logs to debug the validate phase issue:

1. **Identify the pattern**: What tools are being called repeatedly during validation?
2. **Check completion detection**: Is the LLM outputting `PHASE_COMPLETE` or `TASK_COMPLETE`?
3. **Review test results**: Are tests passing but LLM not recognizing it?
4. **Analyze prompt clarity**: Does the validate phase system prompt clearly explain when to complete?

### Potential Fixes for Validate Phase (Future Work)

- **Strengthen completion detection** in `pkg/agents/phased_executor.go:452-469`
- **Improve validate phase prompt** in `pkg/agents/prompts/builder_phased_validate.md`
- **Add escape hatch** after N successful test runs without errors
- **Refine test tool output** to be clearer about success/failure

## Unit Tests

Run tests to verify the fix:

```bash
# Run context manager tests
go test ./pkg/agents -run TestContextManager -v

# Run all agent tests
go test ./pkg/agents -v

# Expected: All tests pass
```

## Rollback Instructions

If issues arise, revert the changes:

```bash
git checkout HEAD -- pkg/agents/phased_executor.go
git rm pkg/agents/phased_executor_context_test.go
make build-pedrocode
```

## Related Files

- `pkg/agents/phased_executor.go` - Main fix location
- `pkg/agents/executor.go` - Reference implementation
- `pkg/llmcontext/manager.go` - Context manager interface
- `pkg/agents/prompts/builder_phased_validate.md` - Validate phase prompt (for future debugging)

## GitHub Issue for Background Tasks

As discussed, create a GitHub issue for enabling goroutine background tasks in interactive mode. This would allow LLMs to call long-running tools (like builds or tests) asynchronously during tool execution.

**Issue Title**: "Add goroutine background task support for interactive mode tool execution"

**Description**: Enable async tool execution in interactive mode (pedrocode REPL) so LLMs can initiate long-running operations (tests, builds, downloads) without blocking the inference loop.
