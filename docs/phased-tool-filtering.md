# Phased Execution Tool Filtering

## Overview

This document explains how tool filtering works in PedroCLI's phased execution system and why it's critical for agent performance.

## The Problem

Prior to the fix, the phased execution system sent ALL tool definitions to the LLM, then filtered tool calls after the LLM responded. This caused several issues:

1. **LLM Confusion**: The LLM was told about tools it couldn't actually use
2. **Wasted Tokens**: Large tool definition payloads consumed context window space
3. **Poor Tool Selection**: The LLM would suggest disallowed tools, wasting inference rounds
4. **Post-hoc Filtering**: Rejecting tool calls after generation didn't help the LLM learn

## The Solution

**Filter tool definitions BEFORE sending to the LLM**, not after receiving its response.

This ensures:
- The LLM only sees tools it can actually use
- Token efficiency (smaller payloads = more code context)
- Natural constraint guiding better tool selection
- Clear debug output showing filtering at each phase

## Visual Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    TOOL SCOPING FIX - VISUAL DIAGRAM                    │
└─────────────────────────────────────────────────────────────────────────┘

BEFORE (Broken Flow):
═══════════════════════════════════════════════════════════════════════════

Phase Definition:
┌──────────────────────────┐
│ Phase: "deliver"         │
│ Tools: ["git", "github"] │  ← Only wants 2 tools
└──────────────────────────┘
          ↓
executeInference():
┌─────────────────────────────────────────────────────────┐
│ allTools = convertToolsToDefinitions()                  │
│ → Returns ALL 15 tools                                  │
│                                                          │
│ req.Tools = allTools                                    │
│ → Sends ALL 15 tools to LLM ❌                          │
└─────────────────────────────────────────────────────────┘
          ↓
LLM sees:
┌────────────────────────────────────────────────────────────┐
│ Available tools (15):                                      │
│ [search, navigate, file, edit, bash, git, github, ...]    │
│                                                             │
│ ⚠️  Confused! Phase says "deliver" but shows all tools    │
└────────────────────────────────────────────────────────────┘
          ↓
LLM Response:
┌────────────────────────────────────────────────────────────┐
│ tool_calls: [                                              │
│   {name: "bash", args: {...}},    ← Not allowed!          │
│   {name: "file", args: {...}},    ← Not allowed!          │
│   {name: "git", args: {...}}      ← Allowed ✓             │
│ ]                                                          │
└────────────────────────────────────────────────────────────┘
          ↓
filterToolCalls():
┌────────────────────────────────────────────────────────────┐
│ ⚠️  Tool bash not allowed in phase deliver, skipping      │
│ ⚠️  Tool file not allowed in phase deliver, skipping      │
│ ✅ Tool git allowed                                        │
│                                                             │
│ Result: Wasted LLM tokens + confused agent                │
└────────────────────────────────────────────────────────────┘


AFTER (Fixed Flow):
═══════════════════════════════════════════════════════════════════════════

Phase Definition:
┌──────────────────────────┐
│ Phase: "deliver"         │
│ Tools: ["git", "github"] │  ← Only wants 2 tools
└──────────────────────────┘
          ↓
executeInference():
┌─────────────────────────────────────────────────────────┐
│ allTools = convertToolsToDefinitions()                  │
│ → Returns ALL 15 tools                                  │
│                                                          │
│ ✨ NEW: toolDefs = filterToolDefinitions(allTools)      │
│ → Filters to ONLY ["git", "github"]                     │
│                                                          │
│ req.Tools = toolDefs                                    │
│ → Sends ONLY 2 tools to LLM ✅                          │
│                                                          │
│ Debug: "Phase deliver tools: 2/15 allowed"              │
└─────────────────────────────────────────────────────────┘
          ↓
LLM sees:
┌────────────────────────────────────────────────────────────┐
│ Available tools (2):                                       │
│ [git, github]                                              │
│                                                             │
│ ✅ Clear! Only sees tools it's allowed to use             │
└────────────────────────────────────────────────────────────┘
          ↓
LLM Response:
┌────────────────────────────────────────────────────────────┐
│ tool_calls: [                                              │
│   {name: "git", args: {...}}      ← Naturally constrained │
│   {name: "github", args: {...}}   ← to allowed tools      │
│ ]                                                          │
│                                                             │
│ ✅ Better token usage + focused tool selection            │
└────────────────────────────────────────────────────────────┘
          ↓
filterToolCalls() (safety net):
┌────────────────────────────────────────────────────────────┐
│ ✅ All tools allowed (nothing to filter)                  │
│                                                             │
│ Result: Efficient + correct behavior                      │
└────────────────────────────────────────────────────────────┘
```

## Implementation Details

### 1. filterToolDefinitions() Method

Located in `pkg/agents/phased_executor.go`:

```go
// filterToolDefinitions filters tool definitions to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolDefinitions(defs []llm.ToolDefinition) []llm.ToolDefinition {
    // No restrictions if Tools list is empty
    if len(pie.phase.Tools) == 0 {
        return defs
    }

    // Build allowed set for O(1) lookup
    allowedSet := make(map[string]bool)
    for _, toolName := range pie.phase.Tools {
        allowedSet[toolName] = true
    }

    // Filter definitions
    filtered := make([]llm.ToolDefinition, 0, len(pie.phase.Tools))
    foundTools := make(map[string]bool)

    for _, def := range defs {
        if allowedSet[def.Name] {
            filtered = append(filtered, def)
            foundTools[def.Name] = true
        }
    }

    // Debug logging
    if pie.agent.config.Debug.Enabled {
        fmt.Fprintf(os.Stderr, "   [DEBUG] Filtered tool definitions: %d → %d (phase: %s)\n",
            len(defs), len(filtered), pie.phase.Name)

        // Warn about tools in phase spec that don't exist
        for _, toolName := range pie.phase.Tools {
            if !foundTools[toolName] {
                fmt.Fprintf(os.Stderr, "   ⚠️  Tool %q specified in phase but not registered\n", toolName)
            }
        }
    }

    return filtered
}
```

**Key Features:**
- Returns all tools if `Phase.Tools` is empty (backward compatible)
- Uses map for O(1) lookup performance
- Warns if phase specifies non-existent tools
- Clear debug output showing filtering results

### 2. Integration in executeInference()

Before (broken):
```go
var toolDefs []llm.ToolDefinition
if pie.agent.config.Model.EnableTools {
    toolDefs = pie.agent.convertToolsToDefinitions()
    // Sends ALL tools to LLM ❌
}
```

After (fixed):
```go
var toolDefs []llm.ToolDefinition
if pie.agent.config.Model.EnableTools {
    // Get all tool definitions from registry/tools map
    allToolDefs := pie.agent.convertToolsToDefinitions()

    // Filter to phase-allowed tools BEFORE sending to LLM ✅
    toolDefs = pie.filterToolDefinitions(allToolDefs)

    // Debug: Show filtering results
    if pie.agent.config.Debug.Enabled {
        if len(pie.phase.Tools) > 0 {
            fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s tools: %d/%d allowed (%v)\n",
                pie.phase.Name, len(toolDefs), len(allToolDefs), pie.phase.Tools)
        } else {
            fmt.Fprintf(os.Stderr, "   [DEBUG] Phase %s: all %d tools available (unrestricted)\n",
                pie.phase.Name, len(toolDefs))
        }
    }
}
```

### 3. Safety Net (filterToolCalls)

The post-hoc `filterToolCalls()` method remains active as a defense-in-depth measure against:
- LLM hallucinating non-existent tools
- Edge cases where pre-filtering might fail

When it triggers, it now includes a debug warning:
```
⚠️ Tool bash not allowed in phase deliver, skipping
[DEBUG] This should not happen if tool definitions were filtered correctly
```

This helps identify issues with the pre-filtering logic.

## Example: Builder Agent Phases

The builder agent uses a 5-phase workflow with different tool restrictions:

### Phase 1: Analyze
**Tools:** `["search", "navigate", "file"]`
**Purpose:** Understand the codebase without making changes
**LLM sees:** 3 tools only (read-only operations)

```
[DEBUG] Phase analyze tools: 3/15 allowed ([search navigate file])
```

### Phase 2: Plan
**Tools:** `["search", "navigate", "file", "context"]`
**Purpose:** Create implementation plan
**LLM sees:** 4 tools (read + context storage)

```
[DEBUG] Phase plan tools: 4/15 allowed ([search navigate file context])
```

### Phase 3: Implement
**Tools:** `["file", "code_edit", "navigate", "search", "context"]`
**Purpose:** Write the actual code
**LLM sees:** 5 tools (read + write operations)

```
[DEBUG] Phase implement tools: 5/15 allowed ([file code_edit navigate search context])
```

### Phase 4: Validate
**Tools:** `["test", "bash", "file", "search"]`
**Purpose:** Run tests and verify changes
**LLM sees:** 4 tools (testing tools)

```
[DEBUG] Phase validate tools: 4/15 allowed ([test bash file search])
```

### Phase 5: Deliver
**Tools:** `["git", "github"]`
**Purpose:** Create commits and PRs
**LLM sees:** 2 tools only (version control)

```
[DEBUG] Phase deliver tools: 2/15 allowed ([git github])
```

## Benefits

### 1. Token Efficiency
**Before:** Sending 15 tool definitions @ ~200 tokens each = ~3,000 tokens wasted
**After:** Sending 2 tool definitions @ ~200 tokens each = ~400 tokens used
**Savings:** ~2,600 tokens per inference call = more room for code context

### 2. Better LLM Performance
- **Focused tool selection**: LLM doesn't consider irrelevant tools
- **Clearer intent**: Phase restrictions guide the LLM's approach
- **Fewer errors**: No wasted rounds trying disallowed tools

### 3. Debugging
Clear debug output at each phase:
```
📋 Phase 5/5: deliver
   Create PR with changes
   [DEBUG] Phase deliver tools: 2/15 allowed ([git github])
   🔄 Round 1/5
   🔧 git
   ✅ git
```

If you see this warning, something is wrong:
```
⚠️ Tool bash not allowed in phase deliver, skipping
[DEBUG] This should not happen if tool definitions were filtered correctly
```

### 4. Backward Compatibility
Phases with empty `Tools` lists remain unrestricted:
```go
Phase{
    Name: "custom_phase",
    Tools: []string{}, // Empty = all tools available
}
```

## Testing

Unit test coverage in `pkg/agents/phased_executor_test.go`:

```go
func TestFilterToolDefinitions(t *testing.T) {
    tests := []struct {
        name       string
        phaseTools []string
        allTools   []llm.ToolDefinition
        want       int
        wantNames  []string
    }{
        {
            name:       "empty phase tools returns all",
            phaseTools: []string{},
            allTools:   []llm.ToolDefinition{{Name: "file"}, {Name: "git"}},
            want:       2,
            wantNames:  []string{"file", "git"},
        },
        {
            name:       "filters to allowed subset",
            phaseTools: []string{"git", "github"},
            allTools:   []llm.ToolDefinition{{Name: "file"}, {Name: "git"}, {Name: "github"}},
            want:       2,
            wantNames:  []string{"git", "github"},
        },
        {
            name:       "handles missing tools gracefully",
            phaseTools: []string{"git", "nonexistent"},
            allTools:   []llm.ToolDefinition{{Name: "file"}, {Name: "git"}},
            want:       1,
            wantNames:  []string{"git"},
        },
    }
    // Test implementation...
}
```

Run tests:
```bash
go test ./pkg/agents/... -run TestFilterToolDefinitions -v
```

## Related Files

- **Implementation:** `pkg/agents/phased_executor.go`
- **Tests:** `pkg/agents/phased_executor_test.go`
- **Phase Definitions:**
  - `pkg/agents/builder.go` (BuilderAgent phases)
  - `pkg/agents/debugger.go` (DebuggerAgent phases)
  - `pkg/agents/reviewer.go` (ReviewerAgent phases)

## See Also

- [Phased Execution Guide](./phased-execution.md) - Overview of the phased system
- [Context Management](./pedrocli-context-guide.md) - Token budget and context window management
- [ADR-003: Dynamic Blog Agent](../architecture/adr-003-dynamic-blog-agent.md) - Another phased workflow example
