# Learning: Why Jobs Mark Complete Without Writing Code

**Date**: 2026-01-05
**Job ID**: f13ec17a-8833-4648-9253-a3f7ef64a0d0
**Task**: Migrate from `fmt` to `slog` logging

## Problem

Job marked as TASK_COMPLETE but didn't actually write any code or create a PR. Only read files and searched.

## Root Cause Analysis

### Issue #1: Tool Call Format Mismatch

The agent's LLM is outputting tool calls in a format that doesn't match the actual tool implementation:

**What the agent outputs**:
```json
{"tool": "navigate", "args": {"action": "list_files", "path": "."}}
```

**What happens**:
```json
{
  "tool": null,
  "success": false,
  "error": "unknown action: list_files"
}
```

The `navigate` tool doesn't have a `list_files` action. The agent is using an outdated or incorrect tool schema.

### Issue #2: Code Edit Tool Not Being Parsed

The agent tried to use `code_edit`:
- **File**: 002-response.txt:63
- **Tool call**: `{"tool": "code_edit", "args": {"action": "replace", ...}}`

But this call was **NOT** parsed into 003-tool-calls.json. Only 30 calls were parsed (navigate, search, file), but the `code_edit` call was silently dropped.

### Issue #3: No Error Feedback to LLM

When tools fail with "unknown action" errors, these errors **may not be** fed back to the LLM properly, so the agent doesn't know it's failing and keeps trying invalid actions.

### Issue #4: Missing Tool Call Validation

The tool call parser doesn't validate:
1. Tool names exist
2. Required arguments are present
3. Action names are valid

So invalid tool calls are either:
- Dropped silently (code_edit)
- Executed and fail (navigate with unknown action)

## Evidence

### Tool Usage Summary
```
Round 3 (003-tool-calls.json):
- 1 navigate (failed: unknown action)
- 14 search
- 15 file

Round 9 (009-tool-calls.json):
- 1 navigate
- 1 repo
- 13 search
- 299 file (!!!)

Total: 0 code_edit calls executed
```

### What the Agent Intended

From 002-response.txt, the agent planned to:
1. Search for all `fmt.Print` calls ✓ (succeeded)
2. Create `pkg/logger.go` with slog setup ✗ (code_edit not parsed)
3. Replace all fmt calls with slog calls ✗ (never attempted)
4. Create a PR ✗ (no git tool)

## Solutions Needed

### 1. Fix Tool Schema Alignment

**Problem**: Agent's system prompt has outdated tool schemas

**Solution**:
- Review tool definitions in system prompts (pkg/prompts/)
- Ensure `navigate` tool schema matches implementation
- Add validation to reject unknown actions early

**Files to check**:
- pkg/tools/navigate.go - actual implementation
- pkg/prompts/tool_generator.go - where tool docs are generated
- pkg/agents/base.go - where tools are registered

### 2. Fix Tool Call Parsing

**Problem**: `code_edit` calls are being dropped silently

**Solution**:
- Debug tool call parser in pkg/agents/executor.go
- Add logging when tool calls are dropped
- Ensure ALL valid JSON tool calls are parsed

**File**: pkg/agents/executor.go - parseToolCalls() method

### 3. Add Error Feedback Loop

**Problem**: Tool failures don't inform the LLM

**Solution**:
- Ensure tool results (including errors) are in next prompt
- Add explicit "Tool failed" messages to prompt
- Consider adding tool call validation before execution

**File**: pkg/agents/executor.go - buildFeedbackPrompt() method

### 4. Add Git/PR Tool

**Problem**: Agent has no way to create PRs

**Solution**: Add a `git` tool that can:
- Create branches
- Commit changes
- Create PRs (via gh CLI)

**New file**: pkg/tools/git.go

### 5. Add Tool Call Validation

**Problem**: Invalid tool calls are accepted

**Solution**: Add validation before execution:
```go
func validateToolCall(call ToolCall) error {
    tool := registry.Get(call.Name)
    if tool == nil {
        return fmt.Errorf("unknown tool: %s", call.Name)
    }

    if action, ok := call.Args["action"]; ok {
        if !tool.SupportsAction(action) {
            return fmt.Errorf("tool %s does not support action: %s",
                call.Name, action)
        }
    }

    // Validate required args...
    return nil
}
```

## Testing Strategy

1. **Unit test**: Tool call parser with code_edit examples
2. **Unit test**: Navigate tool with various actions
3. **Integration test**: Run builder agent with fmt→slog task
4. **Verify**: code_edit calls are parsed and executed
5. **Verify**: Tool errors appear in next prompt

## Impact

This affects ALL builder/debugger jobs:
- Tools fail silently
- Agent can't write code (code_edit dropped)
- Agent exhausts iterations reading files
- Marks complete without doing actual work

**Severity**: Critical - blocks primary use case (autonomous coding)

## Next Steps

1. ✅ Document this learning
2. ⬜ Fix tool schema alignment (navigate tool)
3. ⬜ Fix tool call parser (ensure code_edit is parsed)
4. ⬜ Add tool validation
5. ⬜ Add git tool for PR creation
6. ⬜ Add integration test for builder agent
7. ⬜ Test with same fmt→slog task

## Related Files

- `/tmp/pedroceli-jobs/f13ec17a-8833-4648-9253-a3f7ef64a0d0-20260105-222433/`
  - 002-response.txt - shows code_edit attempt
  - 003-tool-calls.json - shows code_edit was NOT parsed
  - 004-tool-results.json - shows "unknown action" errors
  - 012-response.txt - shows TASK_COMPLETE (false positive)

## References

- Issue #39: Migrate from fmt to slog (the task that revealed this)
- pkg/agents/executor.go - inference loop and tool call parsing
- pkg/tools/navigate.go - navigate tool implementation
- pkg/prompts/tool_generator.go - tool schema generation
