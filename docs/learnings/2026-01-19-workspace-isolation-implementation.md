# Workspace Isolation Implementation - January 19, 2026

## Overview

Completed full implementation of workspace isolation for HTTP Bridge jobs (Issue #72, ADR-008). This enables concurrent job execution without file conflicts by giving each job its own isolated git clone.

## Problem Discovered

While testing Issue #32 (Prometheus metrics), discovered that HTTP Bridge was **not** using isolated workspaces despite WorkspaceManager existing and ADR-008 documentation claiming it was implemented. All jobs were editing the main repository directly, causing:
- Concurrent job conflicts
- Branch pollution
- No isolation between jobs
- Risk of data loss/corruption

## Root Cause

The architecture had two disconnected pieces:
1. **WorkspaceManager** existed but was never called
2. **Agents** had no way to know about or use isolated workspaces

The missing link: Agents spawn background goroutines immediately in Execute(), but workspace setup needs to happen **before** job creation and be **passed through** to the agent.

## Solution Architecture

### Input Map Pattern

Pass `workspace_dir` through input map to agents, which prioritize it in `setupWorkDirectory()`:

```go
// In handlers.go - BEFORE job creation
workspace, err := WorkspaceManager.SetupWorkspace(ctx, tempJobID, workDir)
input["workspace_dir"] = workspace

// In coding.go - Inside agent goroutine
func setupWorkDirectory(ctx, jobID, input) {
    // Priority: workspace_dir > repo info > cwd
    if workspaceDir, ok := input["workspace_dir"].(string); ok {
        return workspaceDir, nil
    }
    // ... existing logic
}
```

### Automatic Cleanup

Added cleanup hooks in all phased agents:

```go
// In base.go
func (a *BaseAgent) cleanupWorkspaceIfNeeded(ctx, jobID) {
    job, _ := a.jobManager.Get(ctx, jobID)
    if job.WorkspaceDir == "" {
        return // No workspace (CLI mode)
    }
    a.workspaceManager.CleanupWorkspace(ctx, jobID, config)
}

// In builder_phased.go, debugger_phased.go, reviewer_phased.go
err = executor.Execute(bgCtx, initialPrompt)
if err != nil {
    b.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
    b.cleanupWorkspaceIfNeeded(bgCtx, job.ID)  // Cleanup on failure
    return
}
b.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
b.cleanupWorkspaceIfNeeded(bgCtx, job.ID)  // Cleanup on success
```

### Database Schema

Created migration 017 to persist workspace directories:

```sql
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS work_dir TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS workspace_dir TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS context_dir TEXT;
```

## Critical Bug Fixed

**Bug**: `workspace_dir` field was not persisting to database even though `SetWorkspaceDir()` was called successfully.

**Root Cause**: `JobStore.Update()` SQL query was missing `workspace_dir` column.

**Fix**: Updated `pkg/storage/jobs.go`:
- Added `workspace_dir = $9` to UPDATE query
- Added `workspace_dir` to SELECT queries in Get() and List()
- Added `nullString(job.WorkspaceDir)` to ExecContext parameters

## Files Modified

Core implementation (10 files):
1. `pkg/database/migrations/017_add_workspace_fields.sql` - Database schema
2. `pkg/config/config.go` - Default workspace path (~/.pedrocli/worktrees)
3. `pkg/storage/jobs.go` - Database persistence (critical bug fix)
4. `pkg/jobs/manager.go` - File-based job manager
5. `pkg/jobs/interface.go` - SetWorkspaceDir interface
6. `pkg/jobs/db_manager.go` - Database-backed manager
7. `pkg/agents/base.go` - WorkspaceManager interface + cleanup
8. `pkg/agents/coding.go` - setupWorkDirectory prioritization
9. `pkg/httpbridge/handlers.go` - Workspace setup before job creation
10. `pkg/httpbridge/app.go` - SetWorkspaceManager on all agents

Cleanup integration (3 files):
11. `pkg/agents/builder_phased.go` - Record workspace_dir, cleanup hooks
12. `pkg/agents/debugger_phased.go` - Cleanup hooks
13. `pkg/agents/reviewer_phased.go` - Cleanup hooks

Documentation:
14. `docs/adr/ADR-008-dual-file-editing.md` - Implementation status update

## Testing & Validation

### Test 1: Issue #34 (Repository Link Input)
- Job: `ca1487e1-4ca9-4a6c-86ac-cd325ec13e4a`
- Result: ✅ Created isolated workspace at `~/.pedrocli/worktrees/c66e5e91.../workspace/`
- Verified workspace_dir stored correctly (before database fix)

### Test 2: Database Field Persistence
- Job: `c821fb7a-fe11-4855-b996-e470bb5f24bc`
- Result: ✅ workspace_dir field persisted correctly after SQL fix
- JSON response showed correct path

### Test 3: Cleanup Monitoring
- Script: `/tmp/monitor-workspace-cleanup.sh`
- Result: ✅ Cleanup respects `cleanup_on_complete` config (default: false)
- Workspace preserved for debugging as intended

## Key Learnings

### 1. Agent Goroutine Timing
Agents spawn goroutines **immediately** in Execute(). Any setup (workspace, tools) must happen:
- **Before** job creation (workspace setup)
- **Inside** goroutine (tool registration, work dir setup)

### 2. Input Map as Configuration Channel
The input map is the **only** way to pass runtime configuration to agents. Use it for:
- `workspace_dir` - Isolated workspace path
- `provider`, `owner`, `repo` - Repository info
- `description`, `issue`, `criteria` - Task details

### 3. Database Column Hygiene
**Always** include all columns in SQL queries:
- UPDATE: Set all fields that should persist
- SELECT: Include all fields in result scan
- Tests won't catch this - only runtime verification

### 4. Workspace Path Choice
Changed from `~/.cache/pedrocli/jobs/` to `~/.pedrocli/worktrees/`:
- More intuitive naming (worktrees vs jobs)
- XDG cache dir is for temporary data
- Worktrees are valuable debugging artifacts
- Consistent with git worktree concept

### 5. Cleanup Strategy
Default to **preserve** workspaces (cleanup_on_complete: false):
- Enables debugging after job completion
- Prevents accidental data loss
- User can manually clean with `rm -rf ~/.pedrocli/worktrees/*`
- Future: Add `pedrocli cache clean` command

## Configuration

In `.pedrocli.json`:

```json
{
  "http_bridge": {
    "workspace_path": "~/.pedrocli/worktrees",
    "cleanup_on_complete": false
  }
}
```

## Impact

### Positive
- ✅ **Concurrent jobs**: Multiple jobs can run without conflicts
- ✅ **Isolation**: Each job has clean workspace
- ✅ **Debugging**: Preserved workspaces enable post-mortem analysis
- ✅ **Safety**: No risk of corrupting main repository
- ✅ **Caching**: Workspaces can be reused (future enhancement)

### Negative
- ❌ **Disk usage**: Workspaces accumulate if cleanup disabled
- ❌ **Monitoring needed**: Need to track disk usage
- ⚠️ **Mitigation**: Configurable cleanup, manual cleanup commands

## Future Enhancements

### Short Term
1. Add workspace size limits (prevent disk exhaustion)
2. Implement `pedrocli cache clean` command
3. Add workspace age-based cleanup

### Long Term
1. Workspace reuse (git pull vs re-clone)
2. Workspace caching layer (share clones across jobs)
3. Remote workspace support (run jobs on different machines)
4. Workspace snapshots (save/restore state)

## Timeline

- **2026-01-09**: ADR-008 written, WorkspaceManager created (not integrated)
- **2026-01-19**: Bug discovered during Issue #32 testing
- **2026-01-19**: Full implementation completed (8 hours)
- **2026-01-19**: Database bug discovered and fixed
- **2026-01-19**: Testing validated, ADR-008 updated

## References

- Issue: https://github.com/Soypete/PedroCLI/issues/72
- ADR: `docs/adr/ADR-008-dual-file-editing.md`
- Migration: `pkg/database/migrations/017_add_workspace_fields.sql`
- Core Logic: `pkg/agents/coding.go:setupWorkDirectory()`
- Cleanup: `pkg/agents/base.go:cleanupWorkspaceIfNeeded()`
