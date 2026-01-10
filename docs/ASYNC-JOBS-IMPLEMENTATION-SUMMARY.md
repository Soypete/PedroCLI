# Async Jobs Implementation Summary

**Date:** 2026-01-24
**Status:** ✅ Complete
**ADR:** [docs/adr/010-pedrocode-async-jobs.md](adr/010-pedrocode-async-jobs.md)

## What Was Implemented

### 1. True Async Background Jobs

Jobs now run in goroutines and return control to the REPL immediately:

```
pedro:build> add authentication to the API
   🤖 Pedro is working...
   \|/
    |
   / \
✅ Started background job: job-1737766800
   Agent: build
   Use /jobs to see progress

pedro:build>  ← Prompt returns immediately!
```

### 2. Job Persistence to /tmp

Jobs saved to `/tmp/pedrocode-jobs/<job-id>/job.json`:

```bash
/tmp/pedrocode-jobs/
├── job-1737766800/
│   └── job.json  # Full job state
└── job-1737766900/
    └── job.json
```

**Job state includes:**
- ID, agent, description, status
- Start/end times
- Progress (rounds, tool calls, LLM calls)
- Last event, results, errors

### 3. Storage-Based Cleanup

**Rules:**
- ✅ Keep last **10 unfinished jobs** (failed/cancelled/interrupted)
- ✅ **Delete completed jobs immediately** (unless debug mode)
- ✅ When limit exceeded, delete oldest unfinished jobs
- ✅ Debug mode (`keep_temp_files: true`): Keep ALL jobs

**No time-based cleanup** - based on storage limits instead.

### 4. Max Concurrent Jobs: 3

```
pedro:build> task 1
✅ Started job-001

pedro:build> task 2
✅ Started job-002

pedro:build> task 3
✅ Started job-003

pedro:build> task 4
⚠️  Already running 3 jobs (max: 3)
   Use /jobs to see active jobs
```

### 5. Startup Check for Incomplete Jobs

When pedrocode starts, checks for interrupted jobs:

```
⚠️  Found 2 incomplete job(s) from previous session:

1. job-123 (build) - failed
   Description: Add authentication to API
   Started: 2026-01-24 20:00:00
   Last: Called code_edit(file='auth.go')
   Progress: Round 3, 12 tool calls

2. job-456 (debug) - failed
   Description: Fix login bug
   Started: 2026-01-24 20:30:00
   Last: Round 2, 5 tool calls

These jobs were interrupted when pedrocode exited.
Job files are saved in: /tmp/pedrocode-jobs/

Options: [v] View details  [d] Delete all  [k] Keep  [Enter to continue]:
```

**Options:**
- `v` = View full details of all incomplete jobs
- `d` = Delete all incomplete jobs
- `k` or `Enter` = Keep for later review

### 6. Shutdown Cleanup

When pedrocode exits:
1. Marks all running/pending jobs as "failed"
2. Sets error: "Interrupted: pedrocode exited"
3. Sets `end_time` to shutdown time
4. Saves updated state to disk

### 7. New Commands

**`/jobs`** - List all jobs:
```
Jobs (3 total, 1 active):

🔄 job-123 (build) - running - Round 3 (12 tools, 3 LLM) [2m30s]
   └─ Called code_edit(file='api/auth.go')

✅ job-456 (debug) - complete [45s]

❌ job-789 (review) - failed [1m15s]
```

**`/jobs <id>`** - Show job details:
```
Job: job-123
Agent: build
Status: running
Description: Add authentication to API
Started: 2026-01-24 21:00:00
Elapsed: 2m30s

Progress:
  Round: 3
  Tool calls: 12
  LLM calls: 3
  Last event: Called code_edit(file='api/auth.go')
```

**`/cancel <id>`** - Cancel a running job:
```
🚫 Cancelled job-123 (build)
```

### 8. Interactive Mode (Phase-by-Phase Approval)

**Default: Interactive with Pedro** 🤖

Interactive mode now pauses after EACH phase for user review:

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
[c/r/x]> c

📋 Phase 2/5: plan
   ...
```

Type `c` to continue, `r` to retry (coming soon), or `x` to cancel.

**Pedro ASCII Art:**
```
   🤖 Pedro is working...
   \|/
    |
   / \
```

**On completion:**
```
   🎉 Done!
    🤖
   \|/
    |
   / \
```

### 9. Live Progress Notifications

While you work, jobs print progress:

```
pedro:build> your next command
[job-123] 🔧 search_code(pattern="handleAuth")
pedro:build> another command
[job-123] 💭 LLM response received
pedro:build>
[job-123] ✅ Complete! Modified 3 files
```

## Files Created/Modified

### New Files
- `pkg/repl/jobs.go` - Job manager
- `pkg/repl/jobstate.go` - Job persistence
- `pkg/repl/jobcommands.go` - /jobs, /cancel commands
- `pkg/repl/jobconfig.go` - Constants (max jobs)
- `pkg/repl/async.go` - Async job execution
- `pkg/repl/spinner.go` - Pedro ASCII art & spinners
- `pkg/repl/stepwise.go` - Phase-by-phase interactive mode
- `pkg/repl/debuglog.go` - Debug logging to job files
- `pkg/agents/context.go` - Context helpers for phase callbacks
- `docs/adr/010-pedrocode-async-jobs.md` - Architecture decision

### Modified Files
- `pkg/repl/session.go` - Added JobManager field
- `pkg/repl/repl.go` - Added job commands, phase-by-phase interactive mode
- `pkg/repl/input.go` - Added SetPrompt() for [c/r/x]> approval
- `pkg/agents/phased_executor.go` - Added PhaseCallback mechanism
- `cmd/pedrocode/code.go` - Added startup/shutdown hooks

## Testing

### Quick Smoke Test

```bash
# Start pedrocode
./pedrocode --debug

# Switch to background mode (no approval)
pedro:build> /background

# Start a job
pedro:build> add a comment to the README
✅ Started background job: job-<id>

# Check jobs
pedro:build> /jobs

# Start another job (test concurrent)
pedro:build> add another comment
✅ Started background job: job-<id>

# List jobs again
pedro:build> /jobs

# Cancel a job
pedro:build> /cancel job-<id>

# Exit
pedro:build> /quit
```

### Test Startup Check

```bash
# Start pedrocode, start a job, then Ctrl+C to kill it
./pedrocode
pedro:build> /background
pedro:build> add a big feature
^C

# Restart - should show incomplete jobs
./pedrocode
# Should see: "Found 1 incomplete job(s)..."
# Options: [v/d/k]
```

### Test Storage Limits

```bash
# Start 11 jobs that fail (exceeds max of 10)
# The oldest should be auto-deleted

# Check /tmp
ls -la /tmp/pedrocode-jobs/
# Should see max 10 job directories
```

### Test Debug Mode

```bash
# Edit .pedrocli.json
{
  "debug": {
    "keep_temp_files": true
  }
}

# Start jobs - they should NOT be deleted
./pedrocode
pedro:build> /background
pedro:build> simple task
pedro:build> /quit

# Check /tmp - job should still be there
ls /tmp/pedrocode-jobs/
```

## Configuration

Uses existing `config.debug.keep_temp_files`:

```json
{
  "debug": {
    "keep_temp_files": false  // Default: cleanup completed jobs
  }
}
```

**New constants in code:**
- `MaxConcurrentJobs = 3`
- `MaxUnfinishedJobs = 10`

## Known Limitations

1. **Job resume not implemented** - GitHub issue to be created
2. **No job queue** - If 3 jobs running, new jobs are rejected (not queued)
3. **Hardcoded limits** - Max concurrent (3) and max unfinished (10) not configurable yet
4. **No job templates** - Can't save common tasks
5. **Terminal noise** - Multiple jobs can make output cluttered

## Future Work (GitHub Issues)

1. **`/resume <job-id>`** command
   - Parse job state from /tmp
   - Reconstruct context
   - Continue from last round

2. **Job queueing**
   - Queue jobs when at max concurrent
   - Auto-start when slots open

3. **Configurable limits**
   - Make max_concurrent_jobs configurable
   - Make max_unfinished_jobs configurable

4. **Job templates**
   - Save common tasks
   - Run with `/run <template>`

5. **Better output management**
   - Separate terminal for job output
   - Buffer job notifications
   - Color-coded job messages

## Success Criteria

✅ Jobs run async (prompt returns immediately)
✅ Can start multiple jobs (up to 3)
✅ Jobs persist to /tmp
✅ Startup checks for incomplete jobs
✅ Shutdown marks running jobs as interrupted
✅ Storage-based cleanup (not time-based)
✅ /jobs command lists all jobs
✅ /cancel command cancels jobs
✅ Interactive mode shows Pedro
✅ Clear [y/n]> prompt for approval
✅ Matches pedrocli cleanup pattern

## Architecture Notes

### Job Lifecycle

```
User Input
    ↓
[Interactive?] → Yes → Show task → [y/n]> → Approve?
    ↓                                           ↓
   No                                          Yes
    ↓                                           ↓
Check concurrent limit (3)
    ↓
Create BackgroundJob
    ↓
Save to /tmp
    ↓
Launch goroutine ←────────────────────────────┘
    ↓
Execute agent (async)
    ├─ Update status: running
    ├─ Save to /tmp
    ├─ Execute inference loop
    ├─ Update progress
    ├─ Save to /tmp
    └─ Complete/Fail
         ↓
    Update final status
         ↓
    Save to /tmp
         ↓
    Cleanup (if completed + !debug mode)
```

### Cleanup Strategy

```
On Job Completion:
├─ Status = complete?
│   └─ Delete immediately (unless debug mode)
├─ Status = failed/cancelled?
│   └─ Keep (up to max 10)
└─ Check storage limits
    └─ Too many unfinished? Delete oldest

On Startup:
├─ List incomplete jobs from /tmp
├─ Show summary to user
└─ Options: view/delete/keep

On Shutdown:
├─ Find all active jobs (running/pending)
├─ Mark as "failed" with "Interrupted" error
└─ Save to /tmp
```

## References

- **ADR:** [010-pedrocode-async-jobs.md](adr/010-pedrocode-async-jobs.md)
- **pedrocli pattern:** `pkg/llmcontext/manager.go`
- **HTTP server async jobs:** `pkg/httpbridge/app.go`

## Rollout Plan

1. ✅ Merge to main (this PR)
2. Create GitHub issues for:
   - `/resume` command
   - Job queueing
   - Configurable limits
   - Job templates
3. Update user documentation
4. Add integration tests
5. Announce in release notes

---

**Implementation complete!** Ready for testing and feedback.
