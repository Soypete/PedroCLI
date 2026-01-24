# ADR-008: Interactive REPL Mode with Approval Workflow

## Status

**Accepted** - Implemented 2026-01-22

## Context

PedroCLI had three distinct interfaces but they all shared the same autonomous execution model:
- **pedrocli** - Background CLI (async job creation, polling)
- **pedroweb** - HTTP server with web UI (background jobs via SSE)
- **pedrocode** - Interactive REPL (new binary)

However, **pedrocode was still running agents autonomously** - the agent would analyze, plan, write code, and commit without any user interaction. This is fine for background automation but **not ideal for an interactive development experience**.

### Problems with Autonomous Execution in REPL

1. **No visibility** - Code was written without showing the user what would change
2. **No control** - User couldn't approve/reject proposed changes
3. **Risky** - Agent could make unwanted changes to the codebase
4. **Not collaborative** - Human completely out of the loop
5. **Different UX than Claude Code** - Claude Code shows diffs and waits for approval

### Desired Experience

We want pedrocode to work like **Claude Code's "Open Code"** experience:

```
pedro> fix the authentication bug

🔍 Analyzing...
   Found bug in pkg/auth/handler.go:42

📝 Proposed Fix:
   [Shows unified diff]

Apply this change? [y/n]: y
✅ Applied
🧪 Tests passing
```

**Key characteristics:**
- **Human-in-the-loop** - User approves every major action
- **Transparency** - Show what will change before changing it
- **Control** - Easy to reject/modify proposals
- **Safety** - No surprises, no unwanted changes

## Decision

Implement a **two-mode execution system** for pedrocode:

### 1. Interactive Mode (Default)

**Behavior:**
- Asks for approval before starting tasks
- Shows what the agent plans to do
- Waits for user confirmation: `[y/n]`
- Only proceeds if approved

**Use case:** Normal REPL usage, exploratory development, learning

**Implementation:**
- `Session.InteractiveMode = true` (default)
- Wraps agent execution with approval prompts
- Uses `handleInteractive()` method in REPL

### 2. Background Mode (Opt-in)

**Behavior:**
- Runs agents autonomously (like pedrocli)
- No approval prompts
- Executes immediately

**Use case:** When you trust the agent, batch operations, scripts

**Implementation:**
- Toggle with `/background` or `/auto` commands
- Uses `handleBackground()` method in REPL
- `Session.InteractiveMode = false`

### Commands

```
/interactive         Enable interactive mode (default)
/background, /auto   Enable background mode (no approval)
```

### Session State

```go
type Session struct {
    // ... existing fields
    InteractiveMode bool  // Interactive mode - ask for approval before writing code
}
```

### Execution Flow

**Interactive Mode:**
```
User prompt
  ↓
Show task summary
  ↓
Ask: "Start this task? [y/n]"
  ↓
If yes → Execute agent
  ↓
Show result
```

**Background Mode:**
```
User prompt
  ↓
Execute agent immediately
  ↓
Show result
```

## Architecture

### Package Structure

```
pkg/repl/
├── repl.go           # Main REPL loop with mode handling
├── session.go        # Session state with InteractiveMode field
├── approval.go       # Approval prompt utilities (NEW)
├── interactive.go    # Interactive execution helpers (NEW)
├── commands.go       # Command parser (updated with new commands)
└── output.go         # Progress output
```

### Key Components

**1. Session Mode Tracking**
```go
type Session struct {
    InteractiveMode bool  // NEW: Default true
}

func (s *Session) SetInteractiveMode(enabled bool)
func (s *Session) IsInteractive() bool
```

**2. REPL Mode Switching**
```go
func (r *REPL) handleNaturalLanguage(cmd *Command) error {
    if r.session.IsInteractive() {
        return r.handleInteractive(agent, cmd.Text)
    }
    return r.handleBackground(agent, cmd.Text)
}
```

**3. Approval Prompts**
```go
// pkg/repl/approval.go
type ApprovalPrompt struct {
    Title    string
    Details  string
    Options  []string
}

func (a *ApprovalPrompt) Ask() (*ApprovalResponse, error)
```

## Consequences

### Positive

1. **Safer REPL experience** - Users see what will happen before it happens
2. **More control** - Easy to stop unwanted changes
3. **Better for learning** - See the agent's reasoning and proposals
4. **Aligns with Claude Code** - Familiar UX for Claude users
5. **Flexible** - Can toggle between interactive and autonomous
6. **Foundation for slash commands** - Ready for `/fix`, `/refactor`, etc.

### Negative

1. **More user interaction required** - Extra confirmation steps
2. **Slower workflow** - Background mode is faster for trusted operations
3. **Incomplete implementation** - Full proposal→diff→approve workflow not done yet

### Neutral

1. **Breaking change** - Behavior changed from autonomous to interactive by default
   - **Mitigation:** Can use `/background` to restore old behavior
2. **More code complexity** - Two execution paths instead of one
   - **Mitigation:** Clear separation in `handleInteractive()` vs `handleBackground()`

## Implementation Details

### Phase 1 (Current - MVP)

✅ **Basic interactive mode**
- Session tracks `InteractiveMode` flag
- REPL checks mode before executing
- Simple confirmation prompt: "Start this task? [y/n]"
- Commands to toggle: `/interactive`, `/background`

### Phase 2 (Future - Full Workflow)

⏳ **Proposal-based workflow**
- Agent generates proposal (code diffs, plan)
- Show formatted diffs to user
- Rich approval prompts: `[y/n/e/q]` (yes/no/edit/quit)
- Apply changes only if approved
- Undo/redo support

### Phase 3 (Future - Slash Commands)

⏳ **Slash command integration (PR #60)**
- `/fix` - Fix with diff approval
- `/refactor` - Show before/after, approve
- `/test` - Run tests, ask to fix failures
- `/commit` - Show diff, approve message
- `/undo` - Revert last change

## Examples

### Interactive Mode (Default)

```bash
$ ./pedrocode

pedro:build> add a print statement to main

🔍 Analyzing your request (interactive mode)...
   Task: add a print statement to main

Start this task? [y/n]: y

🤖 Processing with build agent...
   (Running in background - full interactive workflow coming soon!)

✅ Job job-123 started successfully
```

### Background Mode

```bash
pedro:build> /background
⚡ Background mode enabled
   Agent will run autonomously without approval

pedro:build> add a print statement to main

🤖 Processing with build agent...

✅ Job job-123 started successfully
```

### Switching Back

```bash
pedro:build> /interactive
✅ Interactive mode enabled (default)
   Agent will ask for approval before writing code
```

## Alternatives Considered

### 1. Always Interactive (No Background Mode)

**Rejected:** Some users want autonomous execution for trusted operations

### 2. Flag-based (--interactive vs --background)

**Rejected:** Want runtime toggleable, not startup-time only

### 3. Per-command approval (no global mode)

**Rejected:** Too granular, hard to remember which commands need approval

### 4. Separate binaries (pedrocode-interactive vs pedrocode-auto)

**Rejected:** Unnecessarily complex, mode switching is simpler

## Future Enhancements

1. **Proposal Display**
   - Show actual code diffs before applying
   - Unified diff format with syntax highlighting
   - Summary of changes (files, lines added/removed)

2. **Advanced Approval Options**
   - `[y/n/e/q/v]` - yes/no/edit/quit/view
   - Inline editing of proposals
   - Multi-step approval (analyze→plan→implement)

3. **History & Undo**
   - Track all applied proposals
   - `/undo` to revert last change
   - `/redo` to reapply
   - Session replay

4. **Dry-run Mode for Tools**
   - Tools return diffs instead of applying changes
   - Accumulate proposals across multiple tool calls
   - Batch approval of all changes

5. **Slash Commands Integration**
   - Each slash command has its own approval workflow
   - Context-aware prompts (e.g., commit message approval)
   - Smart defaults based on command type

## Related ADRs

- ADR-002: Dynamic Tool Prompt Generation
- ADR-003: Dynamic Blog Content Generation
- ADR-007: Model-Specific Tool Call Formatting
- Future: ADR-009: Slash Commands Registry (PR #60)

## References

- [pedrocode REPL Guide](../pedrocode-repl.md)
- [Interactive Workflow Design](../interactive-workflow.md)
- [Debugging Guide](../pedrocode-debugging.md)
- Claude Code "Open Code" UX pattern

## Notes

This ADR represents the **foundation** for a fully interactive REPL experience. The current implementation (Phase 1) provides basic approval prompts, but the vision is a full proposal→diff→approve→apply workflow similar to Claude Code.

The two-mode design (interactive vs background) provides flexibility while defaulting to the safer, more collaborative interactive mode.

---

**Author:** Claude Sonnet 4.5
**Date:** 2026-01-22
**Status:** Accepted & Implemented (Phase 1)
