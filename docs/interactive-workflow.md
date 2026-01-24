# Interactive Workflow for pedrocode

## Overview

pedrocode should work like **Claude Code's "Open Code"** experience - interactive, approval-based coding with human-in-the-loop at every step.

## Core Principles

1. **Show, Don't Tell** - Display diffs before writing code
2. **Human Approval** - Never write code without user confirmation
3. **Incremental Changes** - One logical change at a time
4. **Fast Feedback** - Stream progress, show results immediately
5. **Undo/Redo** - Easy to back out of changes

## Workflow Example

```
pedro> /fix the authentication bug

🔍 Analyzing the authentication issue...
   Found bug in pkg/auth/handler.go:42
   Issue: Not handling ErrExpired token error

📝 Proposed Fix:

   ╭─ pkg/auth/handler.go ─────────────────╮
   │  40 │     token, err := ParseToken(req) │
   │  41 │     if err != nil {                │
   │  42 │ -       return err                 │
   │  42 │ +       if err == ErrExpired {     │
   │  43 │ +           return RefreshToken()  │
   │  44 │ +       }                          │
   │  45 │ +       return err                 │
   │  46 │     }                              │
   ╰────────────────────────────────────────╯

   1 file changed, +3 -1 lines

Apply this change? [y/n/e/q]: y

✅ Applied changes to pkg/auth/handler.go

🧪 Running tests...
   ✓ pkg/auth/... (2.1s)
   ✓ All tests passing

Would you like to commit? [y/n]: y

📦 Creating commit...
   Commit message: "fix: handle expired token error in auth handler"

✅ Committed to branch: fix/auth-expired-token
```

## Workflow Phases

### Phase 1: Analyze (Background)
- Agent analyzes the request
- Identifies files to change
- No user interaction needed
- Streams progress messages

### Phase 2: Propose (Show & Wait)
- Agent generates code changes
- **Shows diffs** to the user
- **Waits for approval**
- Options: `[y/n/e/q]`
  - `y` = yes, apply changes
  - `n` = no, reject and re-plan
  - `e` = edit the proposed changes
  - `q` = quit, abandon task

### Phase 3: Apply (Conditional)
- Only runs if user approved
- Writes code changes
- Shows progress for each file
- Can be undone with `/undo`

### Phase 4: Validate (Automatic)
- Runs tests automatically
- Shows test results
- If tests fail, asks: "Fix issues? [y/n]"

### Phase 5: Commit (Optional)
- Asks: "Would you like to commit? [y/n]"
- Generates commit message
- Shows commit message for approval
- Creates commit only if approved

## Slash Commands Integration

Slash commands trigger specific workflows:

### `/fix <description>`
Workflow: Analyze → Propose → Apply → Validate → Commit

```
pedro> /fix the login timeout issue

🔍 Analyzing...
📝 Proposed fix... [shows diff]
Apply? [y/n]: y
✅ Applied
🧪 Tests passing
Commit? [y/n]: y
📦 Committed
```

### `/refactor <code>`
Workflow: Analyze → Propose (show before/after) → Apply → Validate

```
pedro> /refactor extract the validation logic

📝 Proposed refactoring:
   Moving validation to validators/user.go
   [shows diffs for 3 files]

Apply? [y/n]: y
✅ Applied 3 changes
🧪 Tests passing
```

### `/test`
Workflow: Run tests → Show results

```
pedro> /test

🧪 Running tests...
   ✓ pkg/auth/... (2.1s)
   ✗ pkg/users/... (FAIL)
      TestUserCreate failed: expected nil, got error

Fix failing tests? [y/n]: y
[Enters fix workflow]
```

### `/commit [message]`
Workflow: Stage changes → Generate message → Confirm → Commit

```
pedro> /commit

📦 Staged changes:
   M pkg/auth/handler.go
   M pkg/auth/handler_test.go

Generated message:
   "fix: handle expired token error in auth handler

   - Added check for ErrExpired
   - Implemented token refresh on expiry
   - Added test coverage for expired tokens"

Commit with this message? [y/n/e]: y
✅ Committed abc123d
```

### `/undo`
Workflow: Show last change → Confirm → Revert

```
pedro> /undo

⏪ Last change:
   Committed abc123d: "fix: handle expired token error"
   Files: pkg/auth/handler.go

Undo this commit? [y/n]: y
✅ Reverted to previous state
```

## Implementation Architecture

### 1. Agent Execution Modes

Agents should support two modes:

**Autonomous Mode** (current behavior):
- Used by `pedrocli` (background CLI)
- Runs all phases without stopping
- No user interaction

**Interactive Mode** (new for `pedrocode`):
- Used by `pedrocode` (REPL)
- Pauses for approval between phases
- Shows diffs and waits for input

### 2. Approval System

**Components:**
- `pkg/repl/approval.go` - Approval prompts (✅ created)
- `pkg/repl/diff.go` - Diff formatting
- `pkg/repl/proposal.go` - Proposal tracking

**Flow:**
```go
// In REPL
proposal := agent.GenerateProposal(ctx, prompt)

// Show diff
diff := FormatProposal(proposal)
fmt.Println(diff)

// Ask for approval
approved := AskApproval("[y/n/e/q]")

if approved {
    agent.ApplyProposal(ctx, proposal)
}
```

### 3. Agent Modification

Modify phased agents to support interactive mode:

```go
type PhasedExecutor struct {
    // ... existing fields

    // Interactive mode callbacks
    onProposal  func(proposal *Proposal) (bool, error)
    onComplete  func(result *Result) error
}

// In execute loop
if e.onProposal != nil {
    approved, err := e.onProposal(proposal)
    if !approved {
        return nil, fmt.Errorf("user rejected proposal")
    }
}
```

### 4. Tool Registry Enhancement

Add "dry-run" mode to tools:

```go
type CodeEditArgs struct {
    File      string
    OldString string
    NewString string
    DryRun    bool  // NEW: Don't actually write, just return diff
}

// code_edit tool
if args.DryRun {
    return &Result{
        Success: true,
        Output:  GenerateDiff(file, oldString, newString),
        Data: map[string]interface{}{
            "diff": diff,
            "file": file,
        },
    }
}
```

## Session State

Track proposals in session:

```go
type Session struct {
    // ... existing fields

    CurrentProposal *Proposal
    History         []*Proposal  // For undo
}

type Proposal struct {
    ID          string
    Description string
    Changes     []FileChange
    Status      string  // "pending", "approved", "rejected", "applied"
    CreatedAt   time.Time
}

type FileChange struct {
    Path      string
    Operation string  // "edit", "create", "delete"
    OldContent string
    NewContent string
    Diff       string
}
```

## UI/UX Design

### Progress Indicators

**Analyzing:**
```
🔍 Analyzing the authentication issue...
   ⏳ Reading auth handler...
   ⏳ Checking test coverage...
   ✓ Found issue in pkg/auth/handler.go:42
```

**Proposing:**
```
📝 Generating fix...
   ✓ Proposed changes to pkg/auth/handler.go
   ✓ Added test coverage
```

**Applying:**
```
✍️  Applying changes...
   ✓ pkg/auth/handler.go (3 lines added)
   ✓ pkg/auth/handler_test.go (12 lines added)
```

**Validating:**
```
🧪 Running tests...
   ⏳ pkg/auth/...
   ✓ pkg/auth/... (2.1s)
   ✓ All tests passing
```

### Diff Display

**Unified diff format:**
```
╭─ pkg/auth/handler.go ─────────────────╮
│  40 │     token, err := ParseToken(req) │
│  41 │     if err != nil {                │
│  42 │ -       return err                 │
│  42 │ +       if err == ErrExpired {     │
│  43 │ +           return RefreshToken()  │
│  44 │ +       }                          │
│  45 │ +       return err                 │
│  46 │     }                              │
╰────────────────────────────────────────╯
```

**Summary:**
```
📝 Proposed Changes:
   1 file changed, +3 -1 lines

   • pkg/auth/handler.go
```

### Error Handling

**Tests fail after applying:**
```
❌ Tests failed:
   pkg/auth/handler_test.go:42: TestAuthExpired failed

Would you like to:
  f - Fix the test failures
  r - Revert the changes
  i - Ignore and continue
  q - Quit

[f/r/i/q]: f

🔍 Analyzing test failure...
```

## Benefits

### For Users

1. **Confidence** - See exactly what will change before it happens
2. **Control** - Approve/reject every change
3. **Learning** - Understand the agent's reasoning
4. **Safety** - Easy to undo mistakes

### For Development Workflow

1. **Incremental** - Small, reviewable changes
2. **Testable** - Validate after each change
3. **Traceable** - Clear history of what changed when
4. **Collaborative** - Human + AI working together

## Migration Path

### Phase 1: Add Approval System (Current)
- ✅ Create approval prompt UI
- ⏳ Add diff formatting
- ⏳ Modify agents to support interactive mode

### Phase 2: Interactive Builder
- ⏳ Create InteractiveBuilderAgent
- ⏳ Add proposal/apply workflow
- ⏳ Integrate with REPL

### Phase 3: Slash Commands
- ⏳ Integrate with PR #60 slash commands
- ⏳ Add command-specific workflows
- ⏳ Add `/undo`, `/redo` support

### Phase 4: Advanced Features
- ⏳ Multi-file diffs
- ⏳ Inline editing (modify proposed changes)
- ⏳ Branch management
- ⏳ PR creation from REPL

## Next Steps

1. **Fix InteractiveBuilderAgent** - Complete the implementation
2. **Add dry-run mode to tools** - Especially `code_edit`
3. **Wire up approval flow in REPL** - Connect agent → approval → apply
4. **Test with real workflow** - Try `/fix` command end-to-end
5. **Iterate on UX** - Make diffs readable, prompts clear

## See Also

- [pedrocode REPL Guide](./pedrocode-repl.md)
- [Debugging Guide](./pedrocode-debugging.md)
- [CLAUDE.md](../CLAUDE.md)
