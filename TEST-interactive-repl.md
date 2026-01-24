# Interactive REPL Test Plan

## Setup

```bash
# Build pedrocode
make build-pedrocode

# Start llama-server (if not already running)
make llama-server

# In another terminal, check health
make llama-health
```

## Test 1: Interactive Mode (Default)

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
📁 Logs: /tmp/pedrocode-sessions/code-...
```

**Actions:**
1. Type: `add a comment to main.go`
2. Should see: `Start this task? [y/n]:`
3. Type: `y`
4. Should see: Agent starts working

**Verify:**
- Approval prompt appears ✓
- Only runs after saying "yes" ✓
- Logs saved in debug mode ✓

## Test 2: Background Mode

**In same REPL session:**

```bash
pedro:build> /background
```

**Expected:**
```
⚡ Background mode enabled
   Agent will run autonomously without approval
```

**Actions:**
1. Type: `add another comment`
2. Should see: NO approval prompt
3. Agent runs immediately

**Verify:**
- No "Start this task?" prompt ✓
- Runs immediately ✓

## Test 3: Switch Back to Interactive

```bash
pedro:build> /interactive
```

**Expected:**
```
✅ Interactive mode enabled (default)
   Agent will ask for approval before writing code
```

**Actions:**
1. Type: `add a third comment`
2. Should see: `Start this task? [y/n]:`
3. Type: `n`
4. Should see: `❌ Task cancelled`

**Verify:**
- Approval prompt returns ✓
- Can cancel with "n" ✓
- Agent doesn't run if cancelled ✓

## Test 4: Help Command

```bash
pedro:build> /help
```

**Expected:**
Should show commands including:
- `/interactive` - Enable interactive mode
- `/background, /auto` - Enable background mode

**Verify:**
- New commands documented ✓

## Test 5: Context Command

```bash
pedro:build> /context
```

**Expected:**
```
Session Context:
  Session ID: code-...
  Mode: code
  Current Agent: build
  Duration: ...
  Commands: ...
```

**Note:** Doesn't show InteractiveMode status yet (could add this!)

## Test 6: Logs Command

```bash
pedro:build> /logs
```

**Expected:**
```
📁 Log Directory: /tmp/pedrocode-sessions/...

  session.log          : Full transcript (X KB)
  agent-calls.log      : Agent execution (X KB)
  tool-calls.log       : Tool calls (X KB)
  llm-requests.log     : LLM API calls (debug only) (X KB)

To view logs:
  cat /tmp/pedrocode-sessions/.../session.log
  tail -f /tmp/pedrocode-sessions/.../*.log
```

**Verify:**
- Shows log directory ✓
- Shows file sizes ✓
- Commands to view logs ✓

## Test 7: Normal Mode (No Debug)

Exit and restart without `--debug`:

```bash
./pedrocode
```

**Expected:**
- No debug info shown
- No stderr log spam
- Still in interactive mode by default

**Actions:**
1. Type: `add a comment`
2. Should see: `Start this task? [y/n]:`
3. Type: `y`
4. NO stderr logs like "Setting workspace_dir..."

**Verify:**
- Interactive mode works ✓
- No stderr spam ✓
- Cleaner output ✓

## Test 8: Mode Switching Persistence

**In one session:**

```bash
pedro:build> /background
pedro:build> add comment 1
[runs immediately]

pedro:build> /interactive
pedro:build> add comment 2
Start this task? [y/n]: y
[runs after approval]

pedro:build> add comment 3
Start this task? [y/n]: n
❌ Task cancelled
```

**Verify:**
- Mode persists across commands ✓
- Switching works mid-session ✓

## Test 9: Agent Switching

```bash
pedro:build> /mode debug
✅ Switched from build to debug

pedro:debug> investigate the auth bug
Start this task? [y/n]: y
[debug agent runs]
```

**Verify:**
- Can switch agents ✓
- Interactive mode persists after agent switch ✓

## Test 10: Check Logs After Run

After running a task in interactive mode:

```bash
# In another terminal
LOG_DIR="/tmp/pedrocode-sessions/code-..." # Get from /logs command
cat $LOG_DIR/session.log
cat $LOG_DIR/agent-calls.log
```

**Expected in session.log:**
```
>>> add a comment to main.go
<<< Job job-123 started successfully
```

**Verify:**
- Input logged ✓
- Output logged ✓
- Agent calls logged ✓

## Expected Issues (Known Limitations)

1. ⚠️ **Still runs in background after approval**
   - After saying "yes", agent runs autonomously
   - Message says: "(Running in background - full interactive workflow coming soon!)"
   - This is expected - full proposal→diff→approve workflow not implemented yet

2. ⚠️ **No diff shown**
   - Doesn't show code diffs before writing
   - Just asks "Start this task?"
   - Phase 2 will add proper diff display

3. ⚠️ **Can't edit proposals**
   - No `[y/n/e/q]` options yet
   - Only `[y/n]` supported
   - Phase 2 will add edit/quit/view options

## Success Criteria

✅ Interactive mode is default
✅ Approval prompt appears before execution
✅ Can reject tasks with "n"
✅ Can toggle to background mode
✅ Can toggle back to interactive mode
✅ Mode persists across commands
✅ No stderr spam in normal mode
✅ Debug mode shows logs
✅ `/logs` command shows log files
✅ All REPL commands work

## Quick One-Liner Test

```bash
make build-pedrocode && ./pedrocode --debug
# Type: add a print statement
# Type: y
# Type: /background
# Type: add another print
# Type: /quit
```

Should complete without errors and show interactive/background mode switching.

## Next Steps After Testing

If tests pass:
1. Create PR with this feature
2. Update main README with interactive mode docs
3. Start on Phase 2 (proposal→diff→approve workflow)
4. Integrate with PR #60 slash commands

If tests fail:
1. Check logs in `/tmp/pedrocode-sessions/`
2. Run with `--debug` to see stderr
3. Report issues
