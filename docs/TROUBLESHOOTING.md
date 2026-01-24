# PedroCLI Troubleshooting Guide

## Lessons from Interactive REPL Development

This document captures key learnings and debugging patterns discovered while building pedrocode.

---

## Issue: Agent Starts But Doesn't Execute

### Symptoms
- Job shows status "running" but never completes
- Context directory (`/tmp/pedroceli-jobs/job-XXX/`) is empty
- No LLM requests in logs
- No tool calls in logs

### Root Cause
Agent is missing `workspace_dir` in the input arguments.

### How to Debug
```bash
# 1. Check job status
cat /tmp/pedrocli-jobs/job-XXX.json | jq .

# 2. Check if context directory exists and has files
ls -lh /tmp/pedroceli-jobs/job-XXX-TIMESTAMP/

# 3. If empty, agent stalled before making any LLM calls
```

### Solution
Ensure `workspace_dir` is passed to agent execution:

```go
// WRONG - Missing workspace_dir
args := map[string]interface{}{
    "description": description,
}

// RIGHT - Includes workspace_dir
args := map[string]interface{}{
    "description":   description,
    "workspace_dir": config.Project.Workdir,
}
```

**Location:** `pkg/cli/bridge.go` line ~254

**Why this matters:** Agents use workspace_dir to:
- Set up working directory for tool execution
- Record on the job for isolation
- Validate they're working in the right place

### Verification
After fix, you should see in stderr (debug mode):
```
Setting workspace_dir for job job-XXX: /path/to/project
Successfully set workspace_dir for job job-XXX
```

---

## Issue: Stderr Log Spam in Normal Mode

### Symptoms
- Running pedrocode without `--debug` shows log messages like:
  ```
  2026/01/22 20:18:37 No workspace_dir in input for job job-XXX
  2026/01/22 20:18:37 Setting workspace_dir for job job-XXX
  ```
- These clutter the clean REPL interface

### Root Cause
Go's `log.Printf()` always writes to stderr, regardless of debug mode.

### How to Debug
```bash
# Run without --debug and watch for stderr output
./pedrocode

# Should be clean - no log.Printf messages
```

### Solution
Suppress stderr in non-debug mode:

```go
// In pkg/repl/stderr.go
func ConditionalStderr(debugMode bool) func() {
    if debugMode {
        return func() {} // Keep stderr
    }
    return SuppressStderr() // Redirect to /dev/null
}

// In pkg/repl/repl.go
func (r *REPL) Run() error {
    cleanup := ConditionalStderr(r.session.DebugMode)
    defer cleanup()
    // ... rest of REPL loop
}
```

**Why this matters:**
- **Normal mode:** Users want clean, UI-focused output
- **Debug mode:** Developers need all logs for troubleshooting

### Verification
**Normal mode:**
```bash
./pedrocode
pedro:build> add a print statement
# Should see NO "2026/01/22..." log lines
```

**Debug mode:**
```bash
./pedrocode --debug
pedro:build> add a print statement
# Should see "Setting workspace_dir..." logs
```

---

## Issue: Agents Not Executing (Called as Tools)

### Symptoms
- Error like: `unknown tool: build`
- Agent names passed to `CallTool()` instead of executed directly

### Root Cause
Confusion between **tools** and **agents**:
- **Tools** = Individual operations (read file, edit code, git commit)
- **Agents** = Orchestrators that USE tools to complete tasks

### How to Debug
```bash
# Check what's registered in the tool registry
# In code, look for:
registry.GetToolNames()

# Agents won't be in this list!
```

### Solution
Don't call agents through the tool bridge - execute them directly:

```go
// WRONG - Treating agent as a tool
result, err := bridge.CallTool(ctx, "build", map[string]interface{}{
    "description": prompt,
})

// RIGHT - Execute agent directly
result, err := bridge.ExecuteAgent(ctx, "build", prompt)
```

**Implementation:**
```go
// pkg/cli/bridge.go
func (b *CLIBridge) ExecuteAgent(ctx context.Context, agentName string, description string) (*toolformat.ToolResult, error) {
    // Create agent based on name
    var agent interface {
        Execute(ctx context.Context, args map[string]interface{}) (*jobs.Job, error)
    }

    switch agentName {
    case "build":
        agent = agents.NewBuilderPhasedAgent(b.config, b.backend, b.jobManager)
        // Register tools with the agent
        agent.RegisterTool(tools.NewFileTool())
        // ... more tools
    }

    // Execute with proper args
    job, err := agent.Execute(ctx, args)
}
```

### Verification
Agent should create a job and start executing:
```
✅ Job job-XXX started successfully
```

---

## Issue: Can't Find Logs After REPL Session

### Symptoms
- Agent ran but can't find what it did
- Don't know where logs are stored
- Session exited and logs were cleaned up

### Root Cause
Different log locations for different scenarios:
- REPL sessions: `/tmp/pedrocode-sessions/<session-id>/`
- Job context: `/tmp/pedroceli-jobs/<job-id>/`

### How to Debug
```bash
# 1. During session - use /logs command
pedro:build> /logs

# 2. After session - find recent sessions
ls -lt /tmp/pedrocode-sessions/ | head

# 3. Check if debug mode was enabled
# (Normal mode cleans up logs on exit)
```

### Solution
**For debugging - always use `--debug`:**

```bash
./pedrocode --debug
```

This:
- Keeps logs after exit
- Shows log directory at startup
- Creates all 4 log files (session, agent, tool, llm)

**Log file purposes:**
```
session.log       - Full REPL transcript (input/output)
agent-calls.log   - Agent execution timeline
tool-calls.log    - Every tool call and result
llm-requests.log  - LLM API requests/responses (debug only)
```

### Verification
```bash
# Start with debug
./pedrocode --debug

# Note the log directory shown:
# 📁 Logs: /tmp/pedrocode-sessions/code-XXX/

# After session, logs still exist:
ls /tmp/pedrocode-sessions/code-XXX/
cat /tmp/pedrocode-sessions/code-XXX/session.log
```

---

## Issue: Empty llm-requests.log

### Symptoms
- `llm-requests.log` is 0 bytes
- Agent seems to run but no LLM calls recorded
- Other logs have content

### Root Cause
Two possible causes:
1. LLM backend not responding (agent stalled before making requests)
2. LLM logging only enabled in debug mode

### How to Debug
```bash
# 1. Check if llama-server is running
curl http://localhost:8082/health

# 2. Check if debug mode is enabled
# (llm-requests.log only written in debug mode)

# 3. Check if job context directory has files
ls /tmp/pedroceli-jobs/job-XXX-TIMESTAMP/
```

### Solution
**If llama-server not running:**
```bash
make llama-server
# Or check logs if it crashed
```

**If debug mode not enabled:**
```bash
./pedrocode --debug  # LLM requests will now be logged
```

**If job stalled (empty context):**
- See "Agent Starts But Doesn't Execute" section above
- Check workspace_dir was provided

### Verification
After running a task in debug mode:
```bash
ls -lh /tmp/pedrocode-sessions/code-XXX/llm-requests.log
# Should be > 0 bytes

cat /tmp/pedrocode-sessions/code-XXX/llm-requests.log
# Should show LLM API request/response JSON
```

---

## Issue: Interactive Mode Doesn't Ask for Approval

### Symptoms
- Agent runs immediately without asking
- No "Start this task? [y/n]:" prompt

### Root Cause
Session is in background mode instead of interactive mode.

### How to Debug
```bash
# Check if you previously ran:
pedro:build> /background

# This disables approval prompts
```

### Solution
Switch back to interactive mode:
```bash
pedro:build> /interactive
✅ Interactive mode enabled (default)
```

Or check session state:
```bash
pedro:build> /context
# Look for InteractiveMode field (future enhancement)
```

### Verification
After enabling interactive mode:
```bash
pedro:build> add a comment
🔍 Analyzing your request (interactive mode)...
Start this task? [y/n]:  # <-- Should appear
```

---

## Issue: Approval Prompt Disappears / Task Cancelled

### Symptoms
- Typed something at approval prompt
- Task was cancelled unexpectedly
- Message: "❌ Task cancelled"

### Root Cause
Approval prompt only accepts `y` or `yes` to proceed. Any other input cancels.

### Examples
```bash
# These APPROVE the task:
Start this task? [y/n]: y
Start this task? [y/n]: yes

# These CANCEL the task:
Start this task? [y/n]: n
Start this task? [y/n]: no
Start this task? [y/n]: /interactive  # Treated as "no"
Start this task? [y/n]: [Enter]       # Empty = "no"
```

### Solution
At the approval prompt, only type:
- `y` or `yes` to run the task
- `n` or `no` to cancel

Don't type REPL commands like `/interactive` at the approval prompt.

### Verification
```bash
pedro:build> add a comment
Start this task? [y/n]: y
✅ Task proceeds

pedro:build> add another comment
Start this task? [y/n]: n
❌ Task cancelled
```

---

## Debugging Workflow

### 1. Start with Debug Mode
```bash
./pedrocode --debug
```

This gives you:
- Full visibility into what's happening
- Logs kept after exit
- LLM request/response logging
- stderr messages (workspace_dir, etc.)

### 2. Use /logs Command
```bash
pedro:build> /logs
```

Shows:
- Log directory path
- File sizes (helps spot empty files)
- Commands to view logs

### 3. Tail Logs in Real-Time
```bash
# In another terminal
tail -f /tmp/pedrocode-sessions/code-XXX/*.log
```

Watch logs update as agent executes.

### 4. Check Job Status
```bash
# If agent seems stuck, check the job
cat /tmp/pedrocli-jobs/job-XXX.json | jq .

# Check context directory
ls /tmp/pedroceli-jobs/job-XXX-TIMESTAMP/
```

### 5. Verify LLM Backend
```bash
# Is llama-server running?
curl http://localhost:8082/health

# Recent activity?
make llama-health
```

---

## Common Gotchas

### 1. Two Different Job Directories
- **REPL logs:** `/tmp/pedrocode-sessions/<session-id>/`
- **Job context:** `/tmp/pedroceli-jobs/<job-id>/`

Don't confuse them! REPL logs = your session. Job context = agent's work.

### 2. Debug Mode vs Normal Mode
- **Debug:** Keeps logs, shows stderr, verbose
- **Normal:** Cleans logs, hides stderr, clean UI

Use debug when troubleshooting, normal for daily use.

### 3. Interactive vs Background Mode
- **Interactive:** Asks for approval (default)
- **Background:** Runs immediately (opt-in with `/background`)

If no approval prompt, you're in background mode.

### 4. Agents vs Tools
- **Agents:** Build, debug, review, triage (orchestrators)
- **Tools:** file_read, code_edit, git_commit (operations)

Agents USE tools. Don't call agents as tools.

### 5. Approval Prompt Only Accepts y/n
At the `[y/n]:` prompt:
- Type `y` or `yes` to proceed
- Type `n` or `no` to cancel
- Anything else = cancelled

---

## Quick Reference

### Log Locations
```bash
# REPL session logs
/tmp/pedrocode-sessions/<session-id>/
  ├── session.log       # Full transcript
  ├── agent-calls.log   # Agent timeline
  ├── tool-calls.log    # Tool execution
  └── llm-requests.log  # LLM API (debug only)

# Job context (agent's working files)
/tmp/pedroceli-jobs/<job-id>-TIMESTAMP/
```

### Essential Commands
```bash
# REPL commands
/logs                 # Show log directory
/debug                # Show debug info
/context              # Show session state
/interactive          # Enable approval mode
/background           # Disable approval mode

# Debug workflow
./pedrocode --debug                    # Start with logging
pedro:build> /logs                     # Get log directory
tail -f /tmp/pedrocode-sessions/.../   # Watch in real-time
```

### Health Checks
```bash
# Is llama-server running?
curl http://localhost:8082/health

# Are logs being written?
ls -lh /tmp/pedrocode-sessions/code-XXX/

# Is job executing?
cat /tmp/pedrocli-jobs/job-XXX.json | jq .status
```

---

## When to File a Bug

If you've tried the above and still have issues, file a bug with:

1. **Steps to reproduce**
2. **Session logs** (`/tmp/pedrocode-sessions/<session-id>/`)
3. **Job status** (`/tmp/pedrocli-jobs/job-XXX.json`)
4. **llama-server health** (`curl http://localhost:8082/health`)
5. **pedrocode version** (`./pedrocode --version`)

Include the log directory path so we can investigate!

---

## See Also

- [pedrocode REPL Guide](./pedrocode-repl.md)
- [Debugging Guide](./pedrocode-debugging.md)
- [ADR-008: Interactive REPL Mode](./adr/ADR-008-interactive-repl-mode.md)
- [CLAUDE.md](../CLAUDE.md)
