# InferenceExecutor: The Autonomous Inference Loop

## Overview

The `InferenceExecutor` is the heart of PedroCLI's autonomous agents. It runs an iterative loop that allows agents to:
- Call tools to gather information
- Process results
- Make decisions
- Continue until task completion

This is what makes agents **autonomous** - they don't just respond once, they **iterate** until the task is complete.

## How It Works

### The Inference Loop

```
┌─────────────────────────────────────────────┐
│  Start: User provides initial prompt        │
└─────────────┬───────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────┐
│  Round N (max 25 by default)                │
│                                             │
│  1. Send prompt to LLM                      │
│  2. LLM responds (text + optional tools)    │
│  3. Parse tool calls from response          │
│  4. Execute tools (if any)                  │
│  5. Build feedback prompt with results      │
└─────────────┬───────────────────────────────┘
              │
              ▼
         ┌────────────┐
         │ Tool calls?│
         └────┬───────┘
              │
    ┌─────────┴─────────┐
    │                   │
   Yes                 No
    │                   │
    ▼                   ▼
Execute tools    Check completion
    │                   │
    │            ┌──────┴──────┐
    │            │             │
    │          Done        Not done
    │            │             │
    │            ▼             │
    │          EXIT            │
    │                          │
    └──────────┬───────────────┘
               │
               ▼
          Next round
```

### Key Components

**File:** `pkg/agents/executor.go`

```go
type InferenceExecutor struct {
    agent        *BaseAgent           // Agent with tools
    contextMgr   *llmcontext.Manager  // File-based context
    maxRounds    int                  // Max iterations (from config)
    currentRound int                  // Current iteration
    systemPrompt string               // Custom system prompt
}
```

## Completion Signals

### When Does a Round Complete?

A round completes when the LLM responds. The executor then decides:

1. **If tools were called**: Execute them and continue to next round
2. **If no tools called**: Check for completion signal
3. **If max rounds reached**: Exit with error

### Completion Detection

**File:** `pkg/agents/executor.go` - `isDone()` method

The executor checks for these signals in the LLM response:

```go
doneSignals := []string{
    "task_complete",
    "task complete",
    "work is complete",
    "i'm done",
    "all done",
    "finished",
}
```

**Case-insensitive** - any of these anywhere in the response triggers completion.

### Custom Completion Signals

Some agents use custom signals:

- **Code agents**: `"TASK_COMPLETE"` (in system prompt)
- **Blog Research**: `"RESEARCH_COMPLETE"` (in system prompt)
- **Blog Sections**: `"SECTION_COMPLETE"` (in system prompt)

These are checked by the same `isDone()` method.

## The Execution Flow

### Step-by-Step Example

```bash
User: "Add a login function to auth.go"

Round 1:
  → LLM: "Let me search for auth.go first"
  → Tool calls: [{"tool": "search_code", "args": {"pattern": "auth.go"}}]
  → Execute: search_code finds pkg/auth/auth.go
  → Feedback: "Found pkg/auth/auth.go. Here's the search result..."

Round 2:
  → LLM: "Let me read the file to see the current structure"
  → Tool calls: [{"tool": "file", "args": {"action": "read", "file": "pkg/auth/auth.go"}}]
  → Execute: file tool reads the content
  → Feedback: "Here's the content of pkg/auth/auth.go: ..."

Round 3:
  → LLM: "Now I'll add the login function"
  → Tool calls: [{"tool": "code_edit", "args": {"file": "pkg/auth/auth.go", ...}}]
  → Execute: code_edit adds the function
  → Feedback: "Successfully modified pkg/auth/auth.go"

Round 4:
  → LLM: "Task complete! I've added the login function."
  → Tool calls: [] (none)
  → Check completion: "task complete" found ✓
  → EXIT: Success
```

### What If No Completion Signal?

If the LLM doesn't call tools AND doesn't signal completion:

```go
// Round 5
if len(toolCalls) == 0 {
    if e.isDone(response.Text) {
        return nil  // Exit
    }

    // No tools, no completion - prompt to continue
    currentPrompt = "You haven't called any tools yet. Please use the available tools to complete the task."
}
```

## Max Iterations

### Configuration

**File:** `.pedrocli.json`

```json
{
  "limits": {
    "max_inference_runs": 25
  }
}
```

Default: **25 rounds** (20 in some configs)

### What Happens at Max?

```go
if e.currentRound >= e.maxRounds {
    return fmt.Errorf("max inference rounds (%d) reached without completion", e.maxRounds)
}
```

**This is an error condition** - it means the agent didn't complete the task in time.

### Why Have a Limit?

Prevents infinite loops if:
- LLM keeps calling tools indefinitely
- LLM never signals completion
- Tool calls keep failing and retrying

## Tool Call Parsing

### Native Tool Calling (Preferred)

Modern LLM APIs return tool calls as structured data:

```go
response := &llm.InferenceResponse{
    Text: "Let me search for that file",
    ToolCalls: []llm.ToolCall{
        {
            Name: "search_code",
            Args: map[string]interface{}{
                "pattern": "auth.go",
            },
        },
    },
}
```

### Fallback: Text Parsing

If the API doesn't support native tool calling, the executor parses JSON from the response text:

```go
// LLM response text:
"Let me search for the file:
{"tool": "search_code", "args": {"pattern": "auth.go"}}"

// Parsed by toolformat.GetFormatterForModel()
```

**Model-specific formats** (see `pkg/toolformat/`):
- **Generic**: JSON objects
- **Qwen**: `<tool_call>` tags
- **Llama**: `<|python_tag|>` format
- **Mistral**: `[TOOL_CALLS]` format

## Tool Execution

### Sequential Execution

Tools are executed **one at a time** in the order called:

```go
for i, call := range toolCalls {
    fmt.Fprintf(os.Stderr, "🔧 Executing tool: %s\n", call.Name)

    result, err := e.agent.executeTool(ctx, call.Name, call.Args)

    if result.Success {
        fmt.Fprintf(os.Stderr, "✅ Tool %s succeeded\n", call.Name)
    } else {
        fmt.Fprintf(os.Stderr, "❌ Tool %s failed: %s\n", call.Name, result.Error)
    }

    results[i] = result
}
```

### Tool Results Feedback

After executing all tools, results are fed back to the LLM:

```go
feedbackPrompt := "Tool execution results:\n\n"

for i, call := range toolCalls {
    result := results[i]

    feedbackPrompt += fmt.Sprintf("Tool: %s\n", call.Name)

    if result.Success {
        feedbackPrompt += fmt.Sprintf("Status: Success\n")
        feedbackPrompt += fmt.Sprintf("Output: %s\n\n", result.Output)
    } else {
        feedbackPrompt += fmt.Sprintf("Status: Failed\n")
        feedbackPrompt += fmt.Sprintf("Error: %s\n\n", result.Error)
    }
}

// This becomes the prompt for the next round
```

## Context Management

### File-Based Context

Every inference round is saved to disk:

```
/tmp/pedrocli-jobs/job-{id}/
├── 001-prompt.txt          # Round 1 user prompt
├── 002-response.txt        # Round 1 LLM response
├── 003-tool-calls.json     # Round 1 parsed tools
├── 004-tool-results.json   # Round 1 tool results
├── 005-prompt.txt          # Round 2 prompt (with feedback)
├── 006-response.txt        # Round 2 LLM response
└── ...
```

**Benefits:**
- Survives crashes (can resume)
- Easy to debug (inspect files)
- Natural context window management
- Clear audit trail

### Context Window Limits

**File:** `pkg/llmcontext/manager.go`

The context manager ensures prompts fit in the model's context window:

```go
// Get conversation history that fits in budget
history, err := contextMgr.GetHistoryWithinBudget(usableContext)
```

**Token estimation**: `tokens ≈ text_length / 4`

If history exceeds the budget, older rounds are **compacted** (summarized).

## Progress Tracking

### Progress Callbacks

Agents can track progress during execution:

```go
executor := NewInferenceExecutor(agent, contextMgr)

executor.SetProgressCallback(func(event ProgressEvent) {
    switch event.Type {
    case ProgressEventRoundStart:
        fmt.Printf("🔄 Round %d started\n", event.Data["round"])

    case ProgressEventToolCall:
        fmt.Printf("🔧 Calling tool: %s\n", event.Data["tool"])

    case ProgressEventToolResult:
        if event.Data["success"].(bool) {
            fmt.Printf("✅ Tool succeeded\n")
        } else {
            fmt.Printf("❌ Tool failed\n")
        }

    case ProgressEventComplete:
        fmt.Printf("✅ Task completed!\n")
    }
})
```

### Event Types

**File:** `pkg/agents/executor.go`

```go
const (
    ProgressEventRoundStart  = "round_start"   // New round begins
    ProgressEventRoundEnd    = "round_end"     // Round completes
    ProgressEventToolCall    = "tool_call"     // Tool is about to execute
    ProgressEventToolResult  = "tool_result"   // Tool execution complete
    ProgressEventLLMResponse = "llm_response"  // LLM responded
    ProgressEventMessage     = "message"       // General message
    ProgressEventError       = "error"         // Error occurred
    ProgressEventComplete    = "complete"      // Task finished
)
```

## Examples

### Example 1: Code Agent (Builder)

```go
// Create executor
contextMgr, _ := llmcontext.NewManager("job-123", false, 32000)
executor := NewInferenceExecutor(builderAgent, contextMgr)

// Set custom system prompt
executor.SetSystemPrompt(buildCodingSystemPrompt())

// Execute with initial task
err := executor.Execute(ctx, "Add a login function to auth.go")

// Executor will iterate:
// Round 1: Search for auth.go
// Round 2: Read auth.go
// Round 3: Edit auth.go to add function
// Round 4: Respond "TASK_COMPLETE"
// Exit
```

### Example 2: Blog Research Phase

```go
// Create executor for research
contextMgr, _ := llmcontext.NewManager(
    fmt.Sprintf("blog-research-%s", postID),
    false,
    32000,
)

executor := NewInferenceExecutor(blogAgent.baseAgent, contextMgr)
executor.SetSystemPrompt(researchSystemPrompt)

// Track tool usage
executor.SetProgressCallback(func(event ProgressEvent) {
    if event.Type == ProgressEventToolCall {
        progress.IncrementToolUse("Research")
    }
})

// Execute research
err := executor.Execute(ctx, "Gather research for this blog post...")

// Executor will iterate:
// Round 1-N: Call web_search, web_scraper, rss_feed, etc.
// Final round: Respond "RESEARCH_COMPLETE"
// Exit
```

## Debugging

### Check Job Files

When debugging a stuck or failed job:

```bash
# Find recent jobs
ls -ltr /tmp/pedrocli-jobs/

# Inspect a specific job
cd /tmp/pedrocli-jobs/job-{id}/

# See what the LLM is responding
cat 002-response.txt
cat 006-response.txt
cat 010-response.txt

# See tool calls
cat 003-tool-calls.json
cat 007-tool-calls.json

# See tool results
cat 004-tool-results.json
cat 008-tool-results.json
```

### Common Issues

#### 1. Infinite Loop (Max Rounds Reached)

**Symptom:**
```
Error: max inference rounds (25) reached without completion
```

**Causes:**
- LLM keeps calling tools without finishing
- LLM never outputs completion signal
- Tool keeps failing, LLM keeps retrying

**Debug:**
```bash
# Check the last few rounds
tail -20 /tmp/pedrocli-jobs/job-{id}/*-response.txt

# Are tools being called repeatedly?
grep -r "tool.*search" /tmp/pedrocli-jobs/job-{id}/

# Is the LLM stuck in a pattern?
```

**Fix:**
- Improve system prompt to be more specific about completion
- Reduce `max_inference_runs` to fail faster during testing
- Add explicit completion criteria in prompt

#### 2. Exits Without Tool Calls

**Symptom:**
```
├─ ✓ Research . 0 tool uses . 2.1k tokens
```

**Causes:**
- System prompt doesn't emphasize tool usage
- LLM thinks it can answer without tools
- Task doesn't require tools

**Debug:**
```bash
# Check first response
cat /tmp/pedrocli-jobs/job-{id}/002-response.txt

# Does it contain a completion signal?
grep -i "complete\|done\|finished" /tmp/pedrocli-jobs/job-{id}/002-response.txt
```

**Fix:**
- Update system prompt to be more explicit: "You MUST use tools"
- Add examples of tool usage
- Make the task more specific to require tool interaction

#### 3. Tool Calls Parsed Incorrectly

**Symptom:**
```
⚠️  Failed to parse tool calls from response
```

**Causes:**
- LLM using wrong JSON format
- Model-specific format not recognized
- Malformed JSON in response

**Debug:**
```bash
# Check the raw response
cat /tmp/pedrocli-jobs/job-{id}/002-response.txt

# Is there JSON?
grep -o '{.*}' /tmp/pedrocli-jobs/job-{id}/002-response.txt
```

**Fix:**
- Verify correct `toolformat.GetFormatterForModel()` is used
- Add examples of correct tool call format to system prompt
- Try native tool calling if LLM API supports it

## Best Practices

### 1. Clear Completion Criteria

✅ **Good:**
```
When you have successfully added the login function and verified it compiles,
respond with "TASK_COMPLETE" to finish.
```

❌ **Bad:**
```
Add a login function.
```

### 2. Explicit Tool Usage

✅ **Good:**
```
CRITICAL: You MUST use the search_code tool to find the file before editing.
Never edit a file you haven't read.
```

❌ **Bad:**
```
You can use tools if needed.
```

### 3. Bounded Iterations

✅ **Good:**
```json
{
  "limits": {
    "max_inference_runs": 10  // For simple tasks
  }
}
```

❌ **Bad:**
```json
{
  "limits": {
    "max_inference_runs": 100  // Unbounded, will waste time/tokens
  }
}
```

### 4. Progress Callbacks

✅ **Good:**
```go
executor.SetProgressCallback(func(event ProgressEvent) {
    if event.Type == ProgressEventToolCall {
        tracker.IncrementToolUse(phaseName)
        tracker.PrintTree()  // Show live progress
    }
})
```

❌ **Bad:**
```go
// No progress tracking - user has no idea what's happening
executor.Execute(ctx, prompt)
```

### 5. Context Budget

✅ **Good:**
```go
// Leave room for response
usableContext := contextWindow * 0.75
contextMgr, _ := llmcontext.NewManager(jobID, false, usableContext)
```

❌ **Bad:**
```go
// Use full context window - no room for response
contextMgr, _ := llmcontext.NewManager(jobID, false, 32000)
```

## Configuration Reference

### Config File

**File:** `.pedrocli.json`

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "context_size": 32000
  },
  "limits": {
    "max_inference_runs": 25,
    "max_task_duration_minutes": 30
  },
  "debug": {
    "enabled": false,
    "keep_temp_files": true
  }
}
```

### Environment Variables

```bash
# Keep job files for debugging
PEDRO_DEBUG=true

# Custom job directory
PEDRO_JOB_DIR=/custom/path/jobs
```

## Related Documentation

- [ADR-003: Dynamic Tool Invocation](adr/ADR-003-dynamic-tool-invocation.md) - Design decision
- [ADR-002: LLM Tool Awareness](adr/ADR-002-llm-tool-awareness-protocol.md) - Tool calling protocol
- [Tool Format Guide](../pkg/toolformat/README.md) - Model-specific formats
- [Context Management](pedrocli-context-guide.md) - Context window handling
- [CLAUDE.md](../CLAUDE.md) - Architecture overview

## FAQ

### Q: How many rounds is normal?

**A:** Depends on task complexity:
- **Simple tasks**: 2-5 rounds (search, read, edit, complete)
- **Medium tasks**: 5-10 rounds (multiple files, testing, fixing)
- **Complex tasks**: 10-20 rounds (research, multiple edits, debugging)

If consistently hitting max rounds (25), the task is too complex or poorly specified.

### Q: Can I change max rounds per agent?

**A:** Currently, it's global in config. You can set it when creating the executor:

```go
executor := NewInferenceExecutor(agent, contextMgr)
executor.maxRounds = 10  // Override for this specific execution
```

### Q: What if I want the agent to never stop?

**A:** Don't do this. Always have a limit. But you can set it high:

```json
{
  "limits": {
    "max_inference_runs": 100
  }
}
```

Be aware: this can waste tokens and time if the agent gets stuck.

### Q: Can tools be called in parallel?

**A:** Not currently. Tools execute sequentially in the order the LLM specifies.

### Q: What happens if a tool fails?

**A:** The failure is fed back to the LLM:
```
Tool: search_code
Status: Failed
Error: pattern not found in any files
```

The LLM can then decide to:
- Try a different pattern
- Try a different tool
- Give up and complete with partial results

### Q: How do I make an agent more autonomous?

**A:**
1. Give it more tools
2. Make system prompt less prescriptive
3. Increase max rounds
4. Add examples of successful tool usage patterns
5. Use progress callbacks to monitor behavior

### Q: Can I resume a failed job?

**A:** Not automatically, but the context files remain in `/tmp/pedrocli-jobs/`. You could:
1. Read the job files
2. Determine what failed
3. Create a new job that continues from that point

This is a potential future feature.

## Summary

The InferenceExecutor is PedroCLI's autonomous agent engine:

- **Iterative**: Runs multiple rounds until task complete
- **Tool-driven**: Agents call tools to gather information and take action
- **Self-correcting**: Failed tools are reported, agent can retry or adapt
- **Bounded**: Max rounds prevent infinite loops
- **Transparent**: All context saved to files for debugging
- **Flexible**: Works with any LLM that can output JSON or use native tool calling

Understanding this loop is key to building effective autonomous agents with PedroCLI.
