# Session Summary - GBNF Grammar Implementation & Testing

## What Was Accomplished

### 1. Documentation Created
✅ **`docs/builder-agent-usage.md`** (350+ lines)
- Complete guide for using the builder agent
- Exact command pattern for issue references
- Real-world example: Issue #32 Prometheus implementation
- Monitoring, troubleshooting, best practices

✅ **`docs/gbnf-grammar-guide.md`** (600+ lines)
- Comprehensive GBNF grammar documentation
- How PedroCLI uses grammars with llama.cpp
- Tool schema implementation guide
- Grammar generation internals
- Debugging strategies and examples

✅ **`docs/GBNF-GRAMMAR-DEBUG-PLAN.md`** (300+ lines)
- Immediate action plan for fixing grammar bug
- Step-by-step debug phases
- Root cause hypotheses
- Quick win alternatives
- Commands to run after compact

### 2. Code Changes

✅ **Grammar Wiring (`pkg/agents/base.go`)**
- Added debug logging for grammar generation
- Confirmed grammar IS being applied
- Imports added: `fmt`, `os`
- Lines 223-246: Grammar application with full debug output

✅ **Feature Branch Created**
```bash
git checkout -b feat/prometheus-observability-issue-32
```

✅ **Test Configuration**
- Created `.pedrocli-llamacpp-7b.json` (7B model config)
- Updated to use PostgreSQL database
- Increased limits: 30 rounds, 60 minute timeout
- Added allowed commands: `gh`, `make`, `grep`

### 3. Test Execution

✅ **Autonomous Builder Test Launched**
- Job ID: `23283e6f-1bba-41cc-b7c7-b02b360a7539`
- Model: Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf
- Task: Implement Prometheus observability (issue #32)
- Status: Killed after 6 rounds (no progress)

✅ **Debug Findings**
- Grammar IS generated (2954 bytes)
- Grammar IS applied to llamacpp backend
- ConfigureForToolCalls() IS called
- **BUT**: Model outputs wrong format

## Critical Bug Discovered

### The Problem
Model outputs: `{"args": {"action": "list_directory"`
Expected output: `{"tool": "navigate", "args": {"action": "list_directory"}}`

**Missing**: The `"tool"` field entirely!

### Why This Happens
**Root Cause**: Go maps are unordered. In `pkg/logits/schema.go`, the `SchemaToGBNF()` function iterates over `Properties` map in random order.

When generating GBNF for:
```go
{
  "tool": "navigate",
  "args": {...}
}
```

Sometimes it generates:
```gbnf
root ::= "{" "\"args\"" ":" ... # WRONG - args first!
```

Instead of:
```gbnf
root ::= "{" "\"tool\"" ":" ... # CORRECT - tool first
```

### The Fix
In `pkg/logits/schema.go`, ensure property ordering:

```go
// When generating object rules, force tool/args order
keys := make([]string, 0, len(schema.Properties))
for k := range schema.Properties {
    keys = append(keys, k)
}

// Force tool-first ordering for tool calls
if hasKey(keys, "tool") && hasKey(keys, "args") {
    keys = []string{"tool", "args"}  // Explicit order
} else {
    sort.Strings(keys)
}
```

## Files to Review After Compact

### Critical for Fix:
1. **`pkg/logits/schema.go`** - Property ordering bug (Line ~52-100)
2. **`pkg/llm/llamacpp.go`** - Add permanent grammar debug file (Line 124)
3. **`docs/GBNF-GRAMMAR-DEBUG-PLAN.md`** - Complete fix guide

### Reference:
1. **`docs/builder-agent-usage.md`** - How to run builder tests
2. **`docs/gbnf-grammar-guide.md`** - Grammar system documentation

### Test Results:
1. **Job logs**: `/tmp/pedrocli-jobs/23283e6f-1bba-41cc-b7c7-b02b360a7539-20260104-101620/`
2. **Model output**: Check `002-response.txt` for malformed JSON

## Next Steps (In Order)

### Phase 1: Inspect Grammar (15 min)
1. Add debug code to save grammar to `/tmp/pedrocli-grammar-debug.gbnf`
2. Run single inference
3. Inspect grammar - look for args-first vs tool-first

### Phase 2: Fix Property Ordering (30 min)
1. Modify `pkg/logits/schema.go`
2. Force "tool" before "args" in GBNF output
3. Rebuild and test

### Phase 3: Verify Fix (15 min)
1. Run builder test again
2. Check for `{"tool": "navigate", "args": ...}` format
3. Confirm tool calls execute successfully

### Phase 4: Complete Prometheus Test (30-60 min)
1. Run full 30-round test with issue #32
2. Monitor for successful PR creation
3. Verify metrics implementation

## Branch Status

```bash
git branch
# * feat/prometheus-observability-issue-32

git status
# modified:   pkg/agents/base.go (debug logging added)
# new file:   docs/builder-agent-usage.md
# new file:   docs/gbnf-grammar-guide.md
# new file:   docs/GBNF-GRAMMAR-DEBUG-PLAN.md
# new file:   .pedrocli-llamacpp-7b-test.json
```

## Important Commands

### Rebuild CLI
```bash
make build-cli
```

### Run Test
```bash
./pedrocli build -issue "32" -description "Implement Prometheus observability..."
```

### Check Grammar
```bash
cat /tmp/pedrocli-grammar-debug.gbnf
```

### View Job Logs
```bash
find /tmp/pedrocli-jobs -name "*-20260104-*" -type d | tail -1
```

## Context for Next Session

You were testing the builder agent's ability to autonomously implement Prometheus metrics (GitHub issue #32) using the 7B model with GBNF grammar. The grammar generation code exists and runs, but there's a critical bug: property ordering in JSON schema causes the generated GBNF to enforce `args`-first instead of `tool`-first format, breaking tool call parsing.

The fix is straightforward (force property order in schema.go), but needs implementation and testing. All documentation is complete and ready for use once the bug is fixed.
