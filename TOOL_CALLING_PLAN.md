# Tool Calling Architecture Plan

## Executive Summary

**Goal:** Enable agents to autonomously call tools (File, CodeEdit, Search, Navigate, Git, Bash, Test) during one-shot LLM inference to complete coding tasks.

**Current State:** Infrastructure exists but tool calling loop is not implemented (marked as TODO).

**Approach:** Keep one-shot subprocess execution for LLMs, implement tool call parsing and iterative execution in agents.

---

## Current Architecture (What We Have)

### 1. CLI → Agent Flow ✅ WORKING
```
pedrocli build --direct → DirectExecutor → AgentFactory.CreateAgent("builder") → BuilderAgent.Execute()
```

### 2. Tool Registration ✅ WORKING
```go
// AgentFactory creates all 7 tools
toolsMap := {
    "file":      FileTool,
    "code_edit": CodeEditTool,
    "search":    SearchTool,
    "navigate":  NavigateTool,
    "git":       GitTool,
    "bash":      BashTool,
    "test":      TestTool,
}

// Registers them with each agent
for _, tool := range f.tools {
    agent.RegisterTool(tool)
}
```

### 3. System Prompt ✅ WORKING
```
You are an autonomous coding agent. You can execute tools to interact with code, run tests, and make changes.

Available tools:
- file: Read, write, and modify entire files
- code_edit: Precise line-based editing (edit/insert/delete specific lines)
- search: Search code (grep patterns, find files, find definitions)
- navigate: Navigate code structure (list directories, get file outlines, find imports)
- git: Execute git commands (status, diff, commit, push, etc.)
- bash: Run safe shell commands (limited to allowed commands)
- test: Run tests and parse results (Go, npm, Python)
```

### 4. LLM One-Shot Execution ✅ WORKING
```
ollama run qwen3-coder:30b "System: ...\n\nUser: Add tests for GenerateInstances\n\nAssistant: "
```

---

## What's Missing (The Gaps)

### 1. Tool Call Parsing ❌ NOT IMPLEMENTED
**Location:** `pkg/llm/ollama.go:61` and `pkg/llm/llamacpp.go:63`

Current code:
```go
response := &InferenceResponse{
    Text:       strings.TrimSpace(cleanOutput),
    ToolCalls:  []ToolCall{}, // TODO: Parse tool calls from response
    NextAction: "COMPLETE",
    TokensUsed: EstimateTokens(cleanOutput),
}
```

**Problem:** LLM response contains text that MAY include tool calls, but we don't parse them.

**Example LLM Response:**
```
I'll help you build a comprehensive test suite for the GenerateInstances function.

## Step 1: Understanding Requirements
...

```bash
file pkg/recurrence/generator.go
```
```

We need to extract the tool call: `file pkg/recurrence/generator.go`

### 2. Iterative Execution Loop ❌ NOT IMPLEMENTED
**Location:** `pkg/agents/builder.go:61-73`

Current code:
```go
// Execute inference loop (simplified - full implementation would be iterative)
response, err := b.executeInference(ctx, contextMgr, userPrompt)
if err != nil {
    b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
    return job, err
}

// Parse and execute tool calls (simplified)
// In full implementation, this would:
// 1. Parse tool calls from response
// 2. Execute each tool
// 3. Feed results back for next inference
// 4. Repeat until task is complete
```

**Problem:** Does one inference then stops. Doesn't actually execute tools or loop.

---

## Solution: Implement Tool Calling Loop

### Phase 1: Define Tool Call Format

The LLM needs to output tool calls in a parseable format. Options:

**Option A: Markdown Code Blocks (Current Implicit Format)**
```markdown
Let me search for the function:

```bash
search -t "GenerateInstances" pkg/recurrence/
```

Now let me read the file:

```bash
file pkg/recurrence/generator.go
```
```

**Option B: JSON Format**
```json
{
  "thoughts": "I need to understand the current implementation first",
  "tool_calls": [
    {"tool": "search", "args": {"-t": "GenerateInstances", "path": "pkg/recurrence/"}},
    {"tool": "file", "args": {"path": "pkg/recurrence/generator.go"}}
  ]
}
```

**Option C: XML Format (Claude-style)**
```xml
<thinking>I need to understand the current implementation first</thinking>
<tool_use>
  <tool>search</tool>
  <args>
    <pattern>GenerateInstances</pattern>
    <path>pkg/recurrence/</path>
  </args>
</tool_use>
```

**Recommendation:** **Option A (Markdown)** - It's what the LLM is already outputting, simplest to parse.

### Phase 2: Implement Tool Call Parser

**File:** `pkg/llm/parser.go` (new)

```go
package llm

import (
    "regexp"
    "strings"
)

// ToolCall represents a single tool invocation
type ToolCall struct {
    Tool string                 `json:"tool"`
    Args map[string]interface{} `json:"args"`
}

// ParseToolCalls extracts tool calls from markdown code blocks
func ParseToolCalls(response string) ([]ToolCall, error) {
    var calls []ToolCall

    // Match: ```bash\ntool_name arg1 arg2\n```
    re := regexp.MustCompile("```bash\\s*\\n([^`]+)\\n```")
    matches := re.FindAllStringSubmatch(response, -1)

    for _, match := range matches {
        if len(match) < 2 {
            continue
        }

        cmdLine := strings.TrimSpace(match[1])
        call, err := parseCommandLine(cmdLine)
        if err != nil {
            continue // Skip unparseable commands
        }

        calls = append(calls, call)
    }

    return calls, nil
}

// parseCommandLine converts "tool arg1 arg2" to ToolCall
func parseCommandLine(cmdLine string) (ToolCall, error) {
    // Split by whitespace, handle quoted args
    parts := splitCommand(cmdLine)
    if len(parts) == 0 {
        return ToolCall{}, fmt.Errorf("empty command")
    }

    toolName := parts[0]
    args := make(map[string]interface{})

    // Parse args based on tool
    // Each tool has its own argument format
    switch toolName {
    case "file":
        // file <path>
        if len(parts) > 1 {
            args["path"] = parts[1]
        }
    case "search":
        // search -t "pattern" path
        args = parseSearchArgs(parts[1:])
    case "code_edit":
        // code_edit -f file -l start:end -r "replacement"
        args = parseCodeEditArgs(parts[1:])
    // ... etc for each tool
    }

    return ToolCall{Tool: toolName, Args: args}, nil
}
```

### Phase 3: Implement Execution Loop

**File:** `pkg/agents/base.go` (modify `executeInference` → `executeLoop`)

```go
// executeLoop performs iterative inference with tool execution
func (a *BaseAgent) executeLoop(ctx context.Context, contextMgr *llmcontext.Manager, initialPrompt string, maxRounds int) error {
    userPrompt := initialPrompt

    for round := 0; round < maxRounds; round++ {
        // Perform inference
        response, err := a.executeInference(ctx, contextMgr, userPrompt)
        if err != nil {
            return fmt.Errorf("inference round %d failed: %w", round, err)
        }

        // Parse tool calls from response
        toolCalls, err := llm.ParseToolCalls(response.Text)
        if err != nil {
            return fmt.Errorf("failed to parse tool calls: %w", err)
        }

        // If no tool calls, task is complete
        if len(toolCalls) == 0 {
            // Check if LLM explicitly said it's done
            if strings.Contains(strings.ToLower(response.Text), "task complete") {
                return nil
            }
            // Otherwise, continue with empty tool results
        }

        // Execute each tool call
        var toolResults []string
        for i, call := range toolCalls {
            result, err := a.executeTool(ctx, call.Tool, call.Args)
            if err != nil {
                toolResults = append(toolResults, fmt.Sprintf("Tool %d (%s) failed: %v", i+1, call.Tool, err))
            } else if !result.Success {
                toolResults = append(toolResults, fmt.Sprintf("Tool %d (%s) error: %s", i+1, call.Tool, result.Error))
            } else {
                toolResults = append(toolResults, fmt.Sprintf("Tool %d (%s) output:\n%s", i+1, call.Tool, result.Output))
            }
        }

        // Save tool calls and results to context
        if err := contextMgr.SaveToolCalls(toolCalls, toolResults); err != nil {
            return fmt.Errorf("failed to save tool results: %w", err)
        }

        // Build next prompt with tool results
        userPrompt = "Tool results:\n\n" + strings.Join(toolResults, "\n\n") + "\n\nContinue with next steps."
    }

    return fmt.Errorf("max rounds (%d) exceeded", maxRounds)
}
```

### Phase 4: Update Builder Agent

**File:** `pkg/agents/builder.go` (modify Execute)

```go
func (b *BuilderAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
    // Get description
    description, ok := input["description"].(string)
    if !ok {
        return nil, fmt.Errorf("missing 'description' in input")
    }

    // Create job
    job, err := b.jobManager.Create("build", description, input)
    if err != nil {
        return nil, err
    }

    // Update status to running
    b.jobManager.Update(job.ID, jobs.StatusRunning, nil, nil)

    // Create context manager
    contextMgr, err := llmcontext.NewManager(job.ID, b.config.Debug.Enabled)
    if err != nil {
        b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
        return job, err
    }
    defer contextMgr.Cleanup()

    // Build initial prompt
    userPrompt := b.buildInitialPrompt(input)

    // Execute iterative loop (NEW!)
    maxRounds := b.config.Limits.MaxInferenceRuns
    if maxRounds == 0 {
        maxRounds = 20 // Default
    }

    err = b.executeLoop(ctx, contextMgr, userPrompt, maxRounds)
    if err != nil {
        b.jobManager.Update(job.ID, jobs.StatusFailed, nil, err)
        return job, err
    }

    // Update job with results
    output := map[string]interface{}{
        "status": "completed",
    }

    b.jobManager.Update(job.ID, jobs.StatusCompleted, output, nil)

    return job, nil
}
```

---

## MCP's Role (Optional Transport Layer)

**MCP is NOT needed for tool calling.** Tool calling happens internally within the agent.

MCP's purpose:
- Exposes agents as "tools" that external clients can call via JSON-RPC
- Useful for: IDEs, web UIs, other processes that want to invoke agents remotely
- NOT needed for: CLI → Agent → Tools flow

### Current MCP Usage:
```
pedrocli (MCP client) → JSON-RPC → pedrocli-server (MCP server) → Agent → Tools
```

### Direct Mode (Recommended):
```
pedrocli (direct executor) → Agent → Tools
```

**Recommendation:** Remove MCP from default flow, keep code for future web UI integration.

---

## Implementation Steps

### Step 1: Implement Tool Call Parser (2-3 hours)
- [ ] Create `pkg/llm/parser.go`
- [ ] Implement `ParseToolCalls()` for markdown code blocks
- [ ] Implement `parseCommandLine()` for each tool
- [ ] Add unit tests

### Step 2: Implement Execution Loop (2-3 hours)
- [ ] Add `executeLoop()` to `BaseAgent`
- [ ] Update context manager to save tool calls/results
- [ ] Add round limit enforcement
- [ ] Add completion detection

### Step 3: Update All Agents (1 hour)
- [ ] Update `BuilderAgent.Execute()`
- [ ] Update `DebuggerAgent.Execute()`
- [ ] Update `ReviewerAgent.Execute()`
- [ ] Update `TriagerAgent.Execute()`

### Step 4: Integration Testing (2-3 hours)
- [ ] Test with real Ollama model
- [ ] Verify tool calls are parsed correctly
- [ ] Verify tools execute and return results
- [ ] Verify agent completes tasks end-to-end
- [ ] Test max rounds limit

### Step 5: Documentation (1 hour)
- [ ] Update CLAUDE.md with tool calling flow
- [ ] Update README with examples
- [ ] Add troubleshooting guide

**Total Estimated Time:** 8-12 hours

---

## Success Criteria

1. ✅ Agent receives task description
2. ✅ Agent performs inference (LLM call)
3. ✅ Response is parsed for tool calls (markdown code blocks)
4. ✅ Each tool is executed with parsed arguments
5. ✅ Tool results are fed back to LLM for next round
6. ✅ Loop continues until task complete or max rounds
7. ✅ Agent creates PR / commits changes
8. ✅ Job marked as completed

---

## Example End-to-End Flow

```
User: pedrocli build -description "Add tests for GenerateInstances" --direct

DirectExecutor: Creates BuilderAgent
BuilderAgent: Creates job, starts execution loop

[Round 1]
  LLM Input: "Task: Build new feature\nDescription: Add tests for GenerateInstances..."
  LLM Output: "I'll start by reading the file:\n```bash\nfile pkg/recurrence/generator.go\n```"
  Parsed: [ToolCall{Tool: "file", Args: {"path": "pkg/recurrence/generator.go"}}]
  Executed: file.Execute() returns file contents
  Context: Saves tool call + result

[Round 2]
  LLM Input: "Tool results:\nTool 1 (file) output:\n<file contents>\n\nContinue..."
  LLM Output: "Now I'll create the test file:\n```bash\ncode_edit -f pkg/recurrence/generator_test.go -a ...\n```"
  Parsed: [ToolCall{Tool: "code_edit", Args: {...}}]
  Executed: code_edit.Execute() creates test file
  Context: Saves tool call + result

[Round 3-N]
  ... continues until tests pass and PR created ...

[Final Round]
  LLM Output: "Task complete. Created PR #123 with comprehensive tests."
  Parsed: [] (no tool calls)
  Detected: "task complete" → exit loop

BuilderAgent: Marks job as completed
DirectExecutor: Displays success message
```

---

## Questions to Resolve

1. **Tool call format:** Stick with markdown code blocks? (Recommended: Yes, it's working)
2. **Completion detection:** How does LLM signal it's done? (Check for "task complete" or no tool calls for 2 rounds)
3. **Error handling:** What if tool fails? (Feed error back to LLM, let it retry)
4. **Max rounds:** Default to 20? Configurable? (Yes, use config.Limits.MaxInferenceRuns)
5. **MCP future:** Keep it for web UI? (Yes, but make direct mode default)

---

## Next Steps

**Immediate:**
1. Discuss this plan with user - get approval on approach
2. Clarify: Keep MCP code but remove from default flow?
3. Start implementation with parser.go

**This Week:**
1. Implement tool call parser
2. Implement execution loop
3. Test end-to-end with real tasks

**This Month:**
1. Refine based on real-world usage
2. Add more sophisticated parsing (handle errors, edge cases)
3. Optimize context management for long tasks
