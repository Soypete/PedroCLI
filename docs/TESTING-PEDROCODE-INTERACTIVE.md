# Testing PedroCode Interactive Mode

## What Was Fixed

### Problem
The approval prompt was confusing because:
1. System printed "Start this task? [y/n]: "
2. But then showed `pedro:build>` prompt
3. Users thought they could type a new command
4. But system was actually waiting for y/n input

### Solution
- Changed the prompt to `[y/n]>` while waiting for approval
- Shows dancing Pedro ASCII art while task runs
- Added spinner animation for visual feedback
- Restores normal `pedro:build>` prompt after approval

## Testing Steps

### 1. Start PedroCode

```bash
./pedrocode --debug
```

**Expected:**
```
╔═══════════════════════════════════════════╗
║   pedrocode - Interactive Coding Agent   ║
╚═══════════════════════════════════════════╝

Mode: code
Agent: build

🐛 Debug mode enabled
📁 Logs: /tmp/pedrocode-sessions/code-<id>/

Type /help for available commands
Type /quit to exit

pedro:build>
```

### 2. Test Interactive Approval Flow

Type a simple task:
```
pedro:build> add a print statement to the README
```

**Expected Output:**
```
🔍 Analyzing your request (interactive mode)...
   Task: add a print statement to the README

[y/n]>
```

**Note the prompt!** It says `[y/n]>` NOT `pedro:build>`, making it clear you need to type y or n.

### 3. Approve the Task

Type `y` and press Enter.

**Expected:**
```
🤖 Processing with build agent...
   (Running in background - full interactive workflow coming soon!)

   🤖 Pedro is working...
   \|/
    |
   / \

⠋ 🤖 Running build agent
```

You should see:
- Pedro ASCII art
- Spinner animation (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ rotating)
- Message showing what's happening

### 4. Wait for Completion

The spinner will run until the agent finishes, then you'll see:

**Success:**
```
✅ Job job-1234567890-20260124-210000 started successfully

pedro:build>
```

**Failure:**
```
❌ Agent failed: <error message>

pedro:build>
```

### 5. Test Cancellation

Try another task but cancel it:
```
pedro:build> add a test function
```

When you see `[y/n]>`, type `n`:

**Expected:**
```
❌ Task cancelled

pedro:build>
```

### 6. Test Background Mode

Switch to background mode (no approval):
```
pedro:build> /background
```

**Expected:**
```
⚡ Background mode enabled
   Agent will run autonomously without approval
```

Now try a task:
```
pedro:build> add a comment to main.go
```

**Expected:**
- NO approval prompt
- Immediately shows Pedro + spinner
- Task runs right away

### 7. Switch Back to Interactive

```
pedro:build> /interactive
```

**Expected:**
```
✅ Interactive mode enabled (default)
   Agent will ask for approval before writing code
```

Try a task - should ask for approval again.

## Current Behavior vs Future Plans

### ✅ What Works Now (as of this fix)

- Clear approval prompt: `[y/n]>` instead of confusing `pedro:build>`
- Visual feedback: Pedro ASCII art + spinner
- Interactive mode (default): asks for approval
- Background mode: runs without asking
- Can switch modes with `/interactive` and `/background`
- Logs written to `/tmp/pedrocode-sessions/<session-id>/`

### 🚧 What's Still Synchronous (Blocks REPL)

**Current:** When you approve a task, the REPL **blocks** until completion. You cannot:
- Type new commands while agent is running
- Cancel a running task (Ctrl+C kills the whole REPL)
- See live progress (tool calls, LLM responses)

**Why:** Tasks run synchronously via `ExecuteAgent()` which blocks until done.

### 🎯 Future: True Background Execution

For truly async background jobs (like the HTTP server), we need:

1. **Goroutine-based execution**
   ```go
   // Start job in background
   jobID, _ := startJobAsync(agent, prompt)

   // Return to prompt immediately
   fmt.Printf("Job %s started in background\n", jobID)
   fmt.Printf("Use /jobs to see status\n")

   // Prompt returns immediately
   pedro:build>
   ```

2. **Job status indicator in prompt**
   ```
   pedro:build [1 job]> /jobs

   Active Jobs:
   ├─ job-123 (build) - Running (5/20 iterations)
   └─ job-456 (debug) - Complete ✅
   ```

3. **Live progress in background**
   ```
   pedro:build> your next command here
   [job-123] 🔧 Called search_code(pattern="handleRequest")
   pedro:build> another command
   [job-123] 💭 LLM response received
   pedro:build>
   [job-123] ✅ Complete! Modified 3 files
   ```

4. **Job management commands**
   ```
   /jobs        - List all jobs
   /jobs <id>   - Show job details
   /cancel <id> - Cancel a running job
   /logs <id>   - Tail job logs
   ```

## Architecture Notes

### Current Design

```
User Input → REPL → ExecuteAgent() [BLOCKS] → Wait for completion → Show result
                                    └─> Agent runs inference loop
                                    └─> Returns when done
```

### True Background Design

```
User Input → REPL → StartJobAsync() → Job ID returned immediately → Prompt ready
                         ↓
                    Background goroutine:
                    ├─> Create job in JobManager
                    ├─> Run agent inference loop
                    ├─> Stream progress events
                    └─> Update job status when complete
```

## Debugging Tips

### Check if task is truly running

Open a second terminal:
```bash
# Watch the job directory
watch -n 1 'ls -lt /tmp/pedrocli-jobs/ | head -10'

# Tail the session log
tail -f /tmp/pedrocode-sessions/code-*/session.log
```

### Common Issues

**Issue:** Prompt shows `pedro:build>` when expecting `[y/n]>`

**Cause:** Old version of pedrocode binary

**Fix:** Rebuild with `make build`

---

**Issue:** Task runs but no Pedro animation

**Cause:** Terminal doesn't support ANSI/emoji

**Fix:** Use a modern terminal (iTerm2, Terminal.app, etc.)

---

**Issue:** Spinner doesn't animate

**Cause:** Task completes too quickly OR llama-server is slow

**Fix:** Normal behavior - spinner only shows while waiting

---

**Issue:** REPL hangs, can't type anything

**Cause:** Task is still running (blocking)

**Fix:** Ctrl+C to cancel (WARNING: kills whole REPL session)

## Success Criteria

✅ Approval prompt shows `[y/n]>` clearly

✅ Can type 'y' to proceed or 'n' to cancel

✅ Pedro ASCII art shows while running

✅ Spinner animates while waiting

✅ Normal prompt returns after completion

✅ Can switch between interactive/background modes

✅ Debug logs saved to session directory

## Next Steps

To implement true background execution:

1. Modify `ExecuteAgent` to return a Job ID immediately
2. Run agent in goroutine
3. Stream progress events via channels
4. Update prompt to show running job count
5. Add `/jobs` command to list/manage background jobs
6. Allow multiple concurrent jobs

See `pkg/jobs/manager.go` for the job management infrastructure that's already in place.
