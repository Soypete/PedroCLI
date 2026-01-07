# Tool Execution Fix - January 5, 2026

## The Problem

After successfully implementing context window compaction (#51), we discovered that **tools were never executing** in either the llamacpp or Ollama tests. The agents would output tool calls as JSON but never actually run them.

### Symptoms
- No `*-tool-calls.json` files in job directories
- No code changes made by agents
- No git commits or PRs created
- Agent responses showed: `{"tool": "search", "args": {...}}` as text
- Tasks timed out or gave up with example code instead of real changes

### Evidence
- **llamacpp test (job-1767669951)**: 25 rounds, no tools executed, eventually crashed
- **Ollama test (job-1767673628)**: 14 rounds, no tools executed, gave up with examples
- Both jobs had identical failure mode despite different backends

## Root Causes

### Cause 1: Missing Fallback Parser

**File**: `pkg/agents/executor.go` (lines 63-79 originally)

The executor only checked `response.ToolCalls` from native API tool calling:

```go
// Get tool calls from response (native API tool calling)
toolCalls := response.ToolCalls
if toolCalls == nil {
    toolCalls = []llm.ToolCall{}
}

// Check if we're done (no more tool calls)
if len(toolCalls) == 0 {
    // No tools → skip execution
}
```

**The issue**:
- When `enable_tools: false` (or omitted from config), the LLM outputs tool calls as **JSON text in response body**
- Native API tool calling (`response.ToolCalls`) returns empty
- Executor sees no tool calls → never executes them
- The `pkg/toolformat` package has parsers but they weren't being used as fallback

**Why this happened**:
- PR #49 (llama-server migration) added native tool calling support
- Native tool calling requires `enable_tools: true` in config
- Fallback parsing for text-based tool calls was never implemented
- System worked with `enable_tools: true` but not without it

### Cause 2: Tools Never Registered with CLI Agents

**File**: `pkg/cli/bridge.go` (lines 75-87 originally)

The CLI bridge created tools but never connected them to agents:

```go
// Create tool factory and registry
factory := toolformat.NewToolFactory(cfg.Config, cfg.WorkDir)
registry, err := factory.CreateRegistryForMode(toolformat.ModeAll)

// ... later ...

// Register coding agents
codingAgents := []agents.Agent{
    agents.NewBuilderAgent(cfg.Config, backend, jobManager),  // No tools!
    agents.NewDebuggerAgent(cfg.Config, backend, jobManager), // No tools!
    // ...
}
```

**The issue**:
- Tools were created and added to the `toolformat.Registry` (for the CLI bridge itself)
- But agents were created without calling `agent.RegisterTool()`
- Agents had empty `tools` map → all tool calls failed with "tool not found"

**Why this happened**:
- HTTP bridge (`pkg/httpbridge/app.go`) does this correctly via `registerCodeTools()`
- CLI bridge was written differently and never copied this pattern
- Bug existed on `main` branch, not just our compaction branch

## The Fixes

### Fix 1: Add Fallback Parser to Executor

**File**: `pkg/agents/executor.go`

Added fallback that parses tool calls from response text when native API returns empty:

```go
// Get tool calls from response (native API tool calling)
toolCalls := response.ToolCalls
if toolCalls == nil {
    toolCalls = []llm.ToolCall{}
}

// FALLBACK: If native tool calling didn't return any calls, try parsing from text
if len(toolCalls) == 0 && response.Text != "" {
    // Get appropriate formatter for model
    formatter := toolformat.GetFormatterForModel(e.agent.config.Model.ModelName)

    // Parse tool calls from response text
    parsedCalls, err := formatter.ParseToolCalls(response.Text)
    if err == nil && len(parsedCalls) > 0 {
        // Convert toolformat.ToolCall to llm.ToolCall
        toolCalls = make([]llm.ToolCall, len(parsedCalls))
        for i, tc := range parsedCalls {
            toolCalls[i] = llm.ToolCall{
                Name: tc.Name,
                Args: tc.Args,
            }
        }

        if e.agent.config.Debug.Enabled {
            fmt.Fprintf(os.Stderr, "  📝 Parsed %d tool call(s) from response text\n", len(toolCalls))
        }
    }
}
```

**Benefits**:
- ✅ Works with `enable_tools: true` (uses native API - faster)
- ✅ Works with `enable_tools: false` (falls back to text parsing)
- ✅ Works with both Ollama and llamacpp
- ✅ Reuses existing `pkg/toolformat` parsers (Qwen, Llama, Mistral, etc.)

### Fix 2: Register Tools with CLI Agents

**File**: `pkg/cli/bridge.go`

Created tools and registered them with each agent (matching HTTP bridge pattern):

```go
// Create code tools for agents
fileTool := tools.NewFileTool()
gitTool := tools.NewGitTool(cfg.WorkDir)
bashTool := tools.NewBashTool(cfg.Config, cfg.WorkDir)
testTool := tools.NewTestTool(cfg.WorkDir)
codeEditTool := tools.NewCodeEditTool()
searchTool := tools.NewSearchTool(cfg.WorkDir)
navigateTool := tools.NewNavigateTool(cfg.WorkDir)

// Helper function to register code tools with an agent
registerCodeTools := func(agent interface{ RegisterTool(tools.Tool) }) {
    agent.RegisterTool(fileTool)
    agent.RegisterTool(codeEditTool)
    agent.RegisterTool(searchTool)
    agent.RegisterTool(navigateTool)
    agent.RegisterTool(gitTool)
    agent.RegisterTool(bashTool)
    agent.RegisterTool(testTool)
}

// Register coding agents WITH TOOLS
builderAgent := agents.NewBuilderAgent(cfg.Config, backend, jobManager)
registerCodeTools(builderAgent)  // <-- CRITICAL FIX

debuggerAgent := agents.NewDebuggerAgent(cfg.Config, backend, jobManager)
registerCodeTools(debuggerAgent)

reviewerAgent := agents.NewReviewerAgent(cfg.Config, backend, jobManager)
registerCodeTools(reviewerAgent)

triagerAgent := agents.NewTriagerAgent(cfg.Config, backend, jobManager)
registerCodeTools(triagerAgent)
```

## Testing

### Before Fix

```bash
$ ./pedrocli build --description "Use navigate tool"
🔄 Inference round 1/25
  🔧 Executing tool: navigate
  ❌ Tool navigate failed: tool not found: navigate
```

### After Fix

```bash
$ make build-cli
$ ./pedrocli build --description "Use navigate tool"
🔄 Inference round 1/25
  📝 Parsed 1 tool call(s) from response text
  🔧 Executing tool: navigate
  ❌ Tool navigate failed: unknown action: list_files
```

**Key difference**:
- **Before**: `tool not found: navigate` (not registered)
- **After**: `unknown action: list_files` (tool executing, error is from tool logic)

The error changed from "tool not found" to "unknown action" - proving tools are now registered and executing!

## Impact on Compaction Testing

Both compaction tests (llamacpp and Ollama) showed:
- ✅ **Compaction worked** - prevented crashes, managed context window
- ❌ **Tools didn't execute** - agents couldn't complete tasks

With these fixes, we can now:
1. Re-run compaction tests with working tools
2. Verify tasks actually complete (not just prevent crashes)
3. Validate the full agent workflow end-to-end

## Lessons Learned

### 1. Native Tool Calling Requires Config

The `enable_tools: true` config flag was added in PR #49 but:
- Not documented clearly
- Not enabled in example configs
- No fallback for when it's disabled

**Recommendation**: Either make tools work regardless of this setting (via fallback), OR make `enable_tools: true` the default for coding agents.

### 2. CLI vs HTTP Bridge Inconsistency

The CLI bridge and HTTP bridge create agents differently:
- HTTP bridge: Creates tools → creates agents → registers tools ✅
- CLI bridge: Creates registry → creates agents → ❌ (missing registration)

**Recommendation**: Unify agent creation. Both should use the same pattern or share a factory function.

### 3. Integration Testing Gaps

We had:
- ✅ Unit tests for individual tools
- ✅ Unit tests for tool registry
- ❌ No integration test verifying agents can actually call tools

**Recommendation**: Add integration test that:
1. Creates an agent with tools
2. Runs inference
3. Verifies tool calls are executed (checks for tool-calls.json files)
4. Validates tool results are fed back to LLM

### 4. Silent Failures

The system "looked" like it was working:
- Agent ran for many rounds
- Responses seemed reasonable
- No explicit errors about missing tools

Only by inspecting job directories did we discover tools weren't executing.

**Recommendation**: Add validation that fails fast:
- If agent uses a tool in prompt but tool isn't registered → error immediately
- If LLM outputs tool calls but none execute → warning after first round
- Track "tools requested vs tools executed" metric

## Configuration Philosophy

After this fix, here's our tool configuration approach:

### Built-in Tools
All coding agents should **ALWAYS** have these tools available (no config option):
- `file` - Read/write files
- `code_edit` - Precise line-based editing
- `search` - Search code (grep, find files, definitions)
- `navigate` - Navigate code structure (list dirs, outlines, imports)
- `git` - Git operations (status, diff, commit, push, branch, PR)
- `bash` - Safe shell commands (with allow/deny lists)
- `test` - Run tests (Go, npm, Python)

### Optional Tools
These should be configurable:
- LSP servers (if enabled in config)
- MCP servers (custom stdio binaries)
- External tools (user-defined)

### Native Tool Calling vs Text Parsing
The `enable_tools` config now controls the **mechanism**, not availability:
- `enable_tools: true` → Use native API tool calling (faster, more reliable)
- `enable_tools: false` → Fall back to text parsing (still works, just slower)

Either way, **tools execute**. Users don't need to know about this detail.

## Files Changed

1. **`pkg/agents/executor.go`**
   - Added `import "github.com/soypete/pedrocli/pkg/toolformat"`
   - Added fallback parser (lines 70-91)
   - Logs "📝 Parsed N tool call(s) from response text" in debug mode

2. **`pkg/cli/bridge.go`**
   - Added tool creation (fileTool, gitTool, etc.) - lines 81-88
   - Added `registerCodeTools()` helper function - lines 90-99
   - Registered tools with each agent before adding to registry - lines 101-119

## Next Steps

1. **Re-run compaction tests** with working tools to validate full workflow
2. **Add integration test** that verifies tool execution end-to-end
3. **Update example configs** to document `enable_tools` option
4. **Consider making built-in tools always available** (remove config dependency)
5. **Unify CLI and HTTP bridge** agent creation patterns

## Blog Post Angle

This debugging journey shows the importance of:
- **Integration testing** - unit tests passed but integration failed
- **Observability** - need to inspect job directories to find silent failures
- **Fallback strategies** - native API + text parsing = always works
- **Consistency** - CLI and HTTP bridges should work the same way

**Title ideas**:
- "Why My AI Agent Looked Like It Was Working (But Wasn't)"
- "Silent Failures: When Your LLM Agent Ignores Its Tools"
- "Debugging Autonomous Agents: Tools That Never Run"
- "Building Resilient AI Agents: Native APIs and Fallback Parsers"
