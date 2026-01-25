# ADR 010: PedroCode Async Background Jobs with Persistence

**Status:** Accepted
**Date:** 2026-01-24
**Deciders:** Team
**Tags:** pedrocode, jobs, async, persistence, ux

## Context

PedroCode initially ran agents synchronously, blocking the REPL until completion. This created several problems:

1. **Confusing UX**: Prompt appeared ready for input while task was running
2. **No progress visibility**: Users couldn't see what the agent was doing
3. **No cancellation**: Ctrl+C killed the entire REPL session
4. **Lost work on crashes**: If pedrocode crashed, all progress was lost
5. **Single task limitation**: Could only run one task at a time

The existing pedrocli CLI has mature job management in `/tmp/pedroceli-jobs/` with persistence and cleanup. We should adopt the same pattern.

## Decision

Implement true async background jobs in PedroCode with the following features:

### 1. Async Job Execution

- Jobs run in goroutines, prompt returns immediately
- User can start multiple jobs concurrently (max 3 concurrent)
- Jobs tracked in `JobManager` with status updates
- Live progress notifications printed to terminal
- If 3 jobs already running, new jobs queue until a slot opens

### 2. Job Persistence (matching pedrocli pattern)

**Directory Structure:**
```
/tmp/pedrocode-jobs/
├── job-1737766800/
│   ├── job.json          # Job metadata and status
│   ├── output.log        # Job output (optional)
│   └── error.log         # Errors (if any)
└── job-1737766900/
    └── job.json
```

**Job State File (`job.json`):**
```json
{
  "id": "job-1737766800",
  "agent": "build",
  "description": "Add authentication to API",
  "status": "running|complete|failed|cancelled",
  "start_time": "2026-01-24T21:00:00Z",
  "end_time": "2026-01-24T21:05:30Z",
  "last_event": "Called code_edit(file='api/auth.go')",
  "tool_calls": 12,
  "llm_calls": 3,
  "current_round": 3,
  "session_id": "code-abc123-20260124-210000",
  "success": true,
  "output": "Successfully added authentication",
  "error": ""
}
```

### 3. Cleanup Strategy (storage-based, not time-based)

**On Job Completion:**
- Completed/successful jobs: Mark for immediate deletion (unless debug mode)
- Failed/cancelled jobs: Keep for review (up to limit)
- If `config.debug.keep_temp_files == true`: Keep all jobs for debugging

**Storage Limits:**
- Keep last **10 unfinished jobs** (running/pending/failed/cancelled)
- Completed jobs deleted immediately (unless debug mode)
- When limit exceeded, delete oldest unfinished jobs first

**On Shutdown:**
- Mark all running/pending jobs as "failed" with error: "Interrupted: pedrocode exited"
- Set `end_time` to shutdown time
- Persist updated state to disk

**On Startup:**
- Scan `/tmp/pedrocode-jobs/` for incomplete jobs
- If found, show summary and prompt:
  ```
  Found 2 incomplete job(s) from previous session:
  1. job-123 (build) - Add authentication
     Started: 2026-01-24 20:00:00
     Last: Called code_edit(file='auth.go')

  2. job-456 (debug) - Fix login bug
     Started: 2026-01-24 20:30:00
     Last: Round 2, 5 tool calls

  Options:
  [v] View full details  [d] Delete all  [k] Keep  [Enter to continue]
  ```
  - `v` = Show full job details (all fields)
  - `d` = Delete all incomplete jobs
  - `k` or `Enter` = Keep for later review

### 4. Job Management Commands

**`/jobs`** - List all jobs
```
Jobs (3 total, 1 active):

🔄 job-123 (build) - running - Round 3 (12 tools, 3 LLM) [2m30s]
   └─ Called code_edit(file='api/auth.go')

✅ job-456 (debug) - complete [45s]

❌ job-789 (review) - failed [1m15s]
```

**`/jobs <id>`** - Show job details
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

**`/cancel <id>`** - Cancel a running job
```
🚫 Cancelled job-123 (build)
```

**`/logs <id>`** - Show job logs
```
tail -f /tmp/pedrocode-jobs/job-123/output.log
```

### 5. Interactive Mode (Phase-by-Phase Approval)

**Default Behavior:**
- Interactive mode is DEFAULT
- Shows Pedro ASCII art at start
- Agent pauses after EACH phase for user review
- User can: **Continue**, **Retry** (TODO), or **Cancel** at each phase

**Workflow Example:**
```
pedro:build> add authentication

🔍 Starting interactive execution
   You'll review and approve each phase

🤖 Running build agent with phase-by-phase approval...

📋 Phase 1/5: analyze
   🔄 Round 1/10
   ✅ Phase analyze completed in 1 rounds

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Phase: analyze
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ Phase completed in 1 rounds

📝 Output:
   Analyzed codebase, found auth/ directory...

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

**Background Mode:**
- Switch with `/background` or `/auto`
- No approval prompts
- Jobs start immediately and run async
- All 5 phases execute automatically

**Prompt Updates:**
```
# No active jobs
pedro:build>

# 1 active job
pedro:build [1 job]>

# Multiple active jobs
pedro:build [3 jobs]>
```

**Live Progress Notifications:**
```
pedro:build> your next command here
[job-123] 🔧 search_code(pattern="handleAuth")
pedro:build> another command
[job-123] 💭 LLM response received
pedro:build>
[job-123] ✅ Complete! Modified 3 files
   🎉 Done!
    🤖
   \|/
    |
   / \
```

## Consequences

### Positive

- ✅ Users can work while jobs run
- ✅ Multiple concurrent jobs supported
- ✅ Clear job status and progress
- ✅ Jobs survive crashes (can be resumed)
- ✅ Matches pedrocli patterns (consistency)
- ✅ Better UX with clear prompts and feedback
- ✅ Easy to cancel jobs without killing REPL

### Negative

- ⚠️ More complex state management
- ⚠️ Terminal output can be noisy with multiple jobs
- ⚠️ Need to handle concurrent access to stdout
- ⚠️ Job resume logic TBD (follow-up work)

### Neutral

- Adds `/tmp/pedrocode-jobs/` directory (users need to be aware)
- Similar cleanup config as pedrocli (`keep_temp_files`)

## Implementation Notes

### Files Modified/Created

**New Files:**
- `pkg/repl/jobs.go` - Job manager and BackgroundJob struct
- `pkg/repl/jobstate.go` - Job persistence to /tmp
- `pkg/repl/spinner.go` - Pedro ASCII art and spinners
- `pkg/repl/stepwise.go` - Phase-by-phase interactive execution
- `pkg/repl/debuglog.go` - Debug logging to job-specific files
- `pkg/agents/context.go` - Context helpers for phase callbacks

**Modified Files:**
- `pkg/repl/session.go` - Add JobManager field
- `pkg/repl/repl.go` - Add /jobs, /cancel commands, phase-by-phase interactive mode
- `pkg/repl/input.go` - Add SetPrompt() for [c/r/x]> prompt
- `pkg/repl/commands.go` - Update help text
- `pkg/agents/phased_executor.go` - Add PhaseCallback mechanism

### Configuration Changes

Uses existing `config.debug.keep_temp_files` setting:
- `true` = Keep ALL job directories for debugging
- `false` = Delete completed jobs immediately, keep last 10 unfinished jobs

**New Constants:**
- `MaxConcurrentJobs = 3` - Maximum jobs running at once
- `MaxUnfinishedJobs = 10` - Maximum incomplete jobs to keep

### Migration Path

**Backward Compatibility:**
- Existing sessions work unchanged
- No config file changes required
- Opt-in with `/background` command

**Future Enhancements:**
- [ ] `/resume <job-id>` command (resume interrupted jobs) - **GitHub Issue**
- [ ] Job priority in queue
- [ ] Job templates/saved commands
- [ ] Export job results to files
- [ ] Configurable concurrent job limit (hardcoded to 3 for now)

## References

- Existing pedrocli job management: `pkg/llmcontext/manager.go`
- Job state pattern: `/tmp/pedroceli-jobs/` directories
- HTTP server async jobs: `pkg/httpbridge/app.go`

## Alternatives Considered

### 1. Keep Synchronous, Add Progress Bar

**Pros:** Simpler implementation
**Cons:** Still blocks REPL, can't run multiple tasks

**Decision:** Rejected - doesn't solve core UX issues

### 2. Use Channels for Job Communication

**Pros:** More "Go idiomatic"
**Cons:** Complex state management, harder to persist

**Decision:** Rejected - file-based persistence is simpler and matches pedrocli

### 3. Store Jobs in SQLite

**Pros:** Better querying, structured data
**Cons:** Overkill for REPL, adds dependency

**Decision:** Rejected - JSON files in /tmp are sufficient

## Follow-Up Work

1. **GitHub Issue #TBD**: Add `/resume <job-id>` command to resume interrupted jobs
   - Parse job state from /tmp
   - Reconstruct context and continue from last round
   - Show diff of what was done vs. what remains
2. **Documentation**: Update user guide with job management commands
3. **Testing**: Add integration tests for async job execution
4. **UX Polish**: Improve terminal output formatting for multiple jobs
5. **Configurable limits**: Make max concurrent/unfinished jobs configurable

## Status Updates

- **2026-01-24**: Initial implementation (async jobs + persistence)
- **2026-01-24**: Updated interactive mode to phase-by-phase approval (replaced single approval)
  - Added `PhaseCallback` mechanism to `pkg/agents/phased_executor.go`
  - Interactive mode now pauses after each of 5 phases
  - User can continue/retry/cancel at each step
  - Background mode for fully autonomous execution
- **TBD**: Add job resume feature
- **TBD**: Add job export/archiving
- **TBD**: Add retry support for individual phases
