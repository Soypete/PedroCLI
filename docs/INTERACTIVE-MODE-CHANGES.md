# Interactive Mode Changes: Stepwise → Interactive

**Date:** 2026-01-24
**Status:** ✅ Complete

## Summary

Replaced single-approval interactive mode with phase-by-phase interactive mode (formerly called "stepwise"). This is now the default behavior.

## What Changed

### Before (Old Interactive)
```
pedro:build> add authentication

🔍 Analyzing your request (interactive mode)...
   Task: add authentication

[y/n]> y                           ← ONLY approval point

✅ Started background job: job-123  ← All 5 phases run to completion
```

User approves once, agent completes entire workflow automatically.

### After (New Interactive - Phase-by-Phase)
```
pedro:build> add authentication

🔍 Starting interactive execution
   You'll review and approve each phase

📋 Phase 1/5: analyze
   ✅ Phase completed in 1 rounds

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Phase: analyze

✅ Phase completed in 1 rounds

📝 Output:
   Analyzed codebase, found auth files...

┌─────────────────────────────────────────┐
│ What would you like to do?              │
│  [c] Continue to next phase (default)   │
│  [r] Retry this phase (TODO)            │
│  [x] Cancel task                        │
└─────────────────────────────────────────┘
[c/r/x]> c                         ← Approval after EACH phase

📋 Phase 2/5: plan
   ...
```

User reviews and approves after each of 5 phases.

## Benefits

1. **Fine-grained control** - Review each phase before proceeding
2. **Can cancel mid-workflow** - Don't waste time if plan looks wrong
3. **Can retry phases** - (Coming soon) Redo a phase if it didn't work
4. **Better transparency** - See exactly what the agent did at each step
5. **More interactive** - True interactive collaboration, not just a single "go/no-go" decision

## Implementation Details

### Files Created
- `pkg/agents/context.go` - Context helpers for passing phase callbacks
- `pkg/repl/stepwise.go` - Phase-by-phase interactive UI
- `pkg/repl/debuglog.go` - Debug logging infrastructure

### Files Modified
- `pkg/agents/phased_executor.go` - Added `PhaseCallback` mechanism
- `pkg/repl/repl.go` - Replaced old `handleInteractive()` with phase-by-phase version
- `docs/adr/010-pedrocode-async-jobs.md` - Updated to document new interactive mode

### Key Code Changes

**Added PhaseCallback to PhasedExecutor:**
```go
// PhaseCallback is called after each phase completes
type PhaseCallback func(phase Phase, result *PhaseResult) (shouldContinue bool, err error)

type PhasedExecutor struct {
    // ... existing fields
    phaseCallback PhaseCallback
}
```

**Context-based callback passing:**
```go
// In agents/context.go
func WithPhaseCallback(ctx context.Context, callback PhaseCallback) context.Context
func GetPhaseCallback(ctx context.Context) (PhaseCallback, bool)

// In phased_executor.go
func (pe *PhasedExecutor) Execute(ctx context.Context, input string) error {
    if callback, ok := GetPhaseCallback(ctx); ok {
        pe.SetPhaseCallback(callback)
    }
    // ... execute phases with callback
}
```

**Interactive handler:**
```go
// In repl.go
func (r *REPL) handleInteractive(agent string, prompt string) error {
    return r.handleInteractivePhased(agent, prompt)
}

// In stepwise.go
func (r *REPL) handleInteractivePhased(agent string, prompt string) error {
    callback := func(phase agents.Phase, result *agents.PhaseResult) (bool, error) {
        r.showPhaseSummary(phase, result)
        return r.askPhaseAction()
    }

    ctx := agents.WithPhaseCallback(r.ctx, callback)
    return r.session.Bridge.ExecuteAgent(ctx, agent, prompt)
}
```

## Modes Available

### Interactive (Default)
- Phase-by-phase approval
- Pauses after each of 5 phases
- User can continue/retry/cancel

**Enable:** `/interactive` (default on startup)

### Background
- Fully autonomous execution
- No approval prompts
- All 5 phases run automatically

**Enable:** `/background` or `/auto`

## Testing

```bash
# Build
make build

# Start pedrocode (interactive mode is default)
./pedrocode

# Test interactive mode
pedro:build> add a print statement to main.go
# Should pause after each phase for approval

# Switch to background mode
pedro:build> /background

# Test background mode
pedro:build> add a comment to main.go
# Should run all phases without pausing

# Switch back to interactive
pedro:build> /interactive
```

## Removed

- ❌ Old single-approval interactive mode
- ❌ `/stepwise` command (renamed to `/interactive`)

## Future Work

- [ ] Add retry support for individual phases (currently shows "TODO")
- [ ] Allow editing phase parameters before retry
- [ ] Add phase-level undo/rollback
- [ ] Save common workflows as templates

## Related Issues

- #80 - LLM streaming
- #81 - Debug writer refactoring
- #82 - Stepwise integration (now complete)

---

**Migration:** No action required - existing configs and workflows continue to work. Interactive mode is still the default, just with better granularity.
