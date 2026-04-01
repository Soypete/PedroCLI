# ADR-012: Kairos-Inspired Memory Consolidation Design for cmd/pedrocode

**Status:** Proposed  
**Scope:** GUI-driven coding harness memory, session continuity, and background consolidation  
**Goal:** Give PedroCode the useful parts of "persistent mode" without turning it into an unsafe always-on autonomous agent  
**Date:** 2026-04-01

---

## TL;DR

We should not start with a fully autonomous daemon.

We should start with a **post-session maintenance loop** that:
- summarizes what happened,
- extracts structured memory,
- prunes noisy context,
- records unfinished work,
- and prepares a clean handoff for the next session.

This takes the most useful public ideas associated with KAIROS-like behavior — persistent state, idle-time consolidation, and memory hygiene — while avoiding the operational risk of a bot that keeps taking actions on its own. Public reverse-engineering writeups describe KAIROS as a persistent/background mode tied to periodic ticks and an autoDream-style memory consolidation flow, likely handled by a separate worker rather than the main active session.

---

## 1. Problem

Right now, a coding harness usually forgets too much or remembers too badly.

If Pedro only stores raw chat history, then every long session becomes expensive, noisy, and harder to trust. If it stores nothing, then every session starts cold and repeats discovery work. Public descriptions of KAIROS-like behavior suggest the valuable pattern is not raw persistence, but **structured persistence with maintenance**: when the user is idle, the system reorganizes what it learned and compresses it into something reusable.

For Pedro, the problem is not "how do we make the agent live forever?" The problem is:

> How do we make one coding session leave behind useful state for the next session?

---

## 2. Design Principles

### Memory is a hint, not source of truth

Any memory Pedro stores must be revalidated against the repo, files, git state, or test results before it is used for action. Public reporting on the leaked Claude Code internals describes memory as something that still needs verification against real artifacts, which is exactly the right posture.

### Consolidation should be isolated

The active coding loop should not also be the memory janitor. KAIROS-style writeups describe a separate autoDream or forked worker doing maintenance work, which is the right pattern for Pedro too.

### Persistence should be phased

Pedro should first support:
1. end-of-session consolidation,
2. resume-from-summary,
3. optional scheduled maintenance,
4. only later event-driven background processing.

### Structured memory beats transcript memory

Pedro should store typed artifacts, not giant blobs of prose.

---

## 3. Proposed Architecture

```
Active GUI Session
    ↓
Session Artifact Store
    ↓
Consolidation Trigger
    ↓
Memory Worker ("dreamer")
    ↓
--------------------------------
| Session Summary              |
| Structured Memory            |
| Open Tasks                   |
| Repo Facts Cache             |
| Recent Failures              |
--------------------------------
    ↓
Resume Loader for next session
```

The key addition is a **Memory Worker** that runs after active work, not during the main interaction loop unless explicitly requested.

---

## 4. What Pedro Should Store

Pedro should keep five classes of persisted state.

### A. Session Summary

A concise narrative of what the session did.

Example fields:
- user goal
- files inspected
- files changed
- tests run
- blockers
- result status

### B. Structured Memory

Typed facts that may help later.

Example buckets:
- `repo_facts`
- `architecture_notes`
- `tool_hints`
- `user_preferences`
- `coding_conventions`
- `known_risks`

### C. Open Tasks

Actionable unfinished work.

Examples:
- "Retry logic added but circuit breaker not implemented"
- "Unit tests missing for rate limit path"
- "Need to verify Windows path handling"

### D. Recent Failures

Useful negative knowledge.

Examples:
- bad commands tried
- failing tests
- rejected approaches
- tool errors

### E. Resume Context

A small package specifically optimized for reopening a session.

Example fields:
- current branch
- changed files
- current objective
- next recommended step
- validation status

---

## 5. Memory Object Model

### SessionRecord

```json
{
  "session_id": "sess_123",
  "repo_id": "pedrocode",
  "started_at": "2026-04-01T14:00:00Z",
  "ended_at": "2026-04-01T15:12:00Z",
  "mode": "build",
  "status": "partial_success",
  "summary_id": "sum_123",
  "artifacts": [
    "artifact_repo_map_1",
    "artifact_diff_2",
    "artifact_test_results_3"
  ]
}
```

### MemoryFact

```json
{
  "id": "mem_45",
  "type": "repo_fact",
  "scope": "repo",
  "subject": "cmd/pedrocode",
  "fact": "Query execution currently lives too close to GUI event handling.",
  "confidence": "medium",
  "evidence_artifacts": ["artifact_repo_map_1"],
  "last_validated_at": "2026-04-01T15:12:00Z"
}
```

### OpenTask

```json
{
  "id": "task_88",
  "title": "Add approval UI for write actions",
  "scope": "cmd/pedrocode",
  "status": "open",
  "priority": "medium",
  "depends_on": [],
  "evidence_artifacts": ["artifact_plan_4"]
}
```

### ResumePacket

```json
{
  "repo_id": "pedrocode",
  "branch": "feature/gui-harness",
  "goal": "Refactor harness into phased execution model",
  "next_step": "Implement QueryEngine wrapper around current GUI loop",
  "changed_files": [
    "cmd/pedrocode/main.go",
    "internal/agent/session.go"
  ],
  "warnings": [
    "Permissions not yet enforced",
    "Subagent results still return freeform text"
  ]
}
```

---

## 6. Consolidation Pipeline

### Trigger Conditions

Start with these triggers:
- user ends session,
- session idle for N minutes,
- token/context threshold exceeded,
- explicit "save and summarize" action.

Later, you can add scheduled maintenance or repo event triggers.

### Pipeline Stages

**Stage 1: Collect artifacts**

Gather: messages, tool calls, diffs, file reads, test outputs, git state, planner outputs.

**Stage 2: Classify**

Sort artifacts into: facts, decisions, failures, open work, noisy/throwaway content.

**Stage 3: Summarize**

Produce: concise session summary, open task list, recommended next step.

**Stage 4: Normalize**

Convert useful findings into typed memory objects.

**Stage 5: Validate**

Before storing a fact, link it to evidence: file path, diff, test result, command output, repo snapshot.

**Stage 6: Prune**

Discard: duplicate observations, obsolete temporary reasoning, repetitive failed tool traces.

**Stage 7: Save resume packet**

Create a tiny package optimized for the next GUI load.

---

## 7. The "Dreamer" Worker

This is the Kairos-inspired piece.

Pedro should have a dedicated maintenance component, not the main session agent. Public writeups about KAIROS describe idle-time memory consolidation and suggest a separate maintenance path rather than using the main interactive loop for this work.

### Responsibilities

The dreamer worker should:
- read completed session artifacts,
- extract durable facts,
- generate a resume packet,
- merge or update existing memory,
- mark stale facts for revalidation,
- **never modify code**,
- **never run unsafe tools**,
- **never take external actions**.

### Allowed Tools
- read artifacts
- read repo metadata
- read git status
- read prior memory store
- write memory store
- write summaries

### Forbidden Tools
- file edits
- shell execution beyond read-only metadata
- network actions
- git mutations
- autonomous code generation

This keeps the worker safe and cheap.

---

## 8. Resume Flow

When the user opens PedroCode again:

**Step 1:** Load the latest `ResumePacket`.

**Step 2:** Show the user a compact handoff:
- last goal,
- what changed,
- current blockers,
- recommended next step.

**Step 3:** Revalidate critical memory:
- branch still exists,
- files still present,
- changed files still differ,
- failing tests still fail.

**Step 4:** Use only validated facts to seed the next session.

This is important. **Memory should accelerate startup, not silently control behavior.**

---

## 9. Guardrails

### Guardrail 1: No autonomous edits
The memory worker never edits code.

### Guardrail 2: No memory-only execution
Pedro cannot take action solely because memory says so.

### Guardrail 3: Evidence required
Every durable fact must reference an artifact.

### Guardrail 4: Expiration and staleness

Facts need TTL or revalidation.

Suggested examples:
- repo facts: revalidate weekly
- branch facts: revalidate on session start
- failure facts: revalidate after related file changes
- user preferences: persist longer

### Guardrail 5: Distinguish fact from inference

Pedro should label:
- observed fact,
- inferred conclusion,
- suggested next step.

---

## 10. Integration with Phased Execution

This fits naturally with the planned phase engine (see ADR-005, vNext design doc).

### During Active Session

Phases remain:
- intake
- plan
- explore
- implement
- validate
- summarize

### After Summarize

Add a final maintenance phase: **consolidate**

So the full lifecycle becomes:

```
intake → plan → explore → implement → validate → summarize → consolidate
```

That gives Pedro a clean endcap.

---

## 11. Suggested Go Interfaces

### MemoryStore

```go
// MemoryStore handles persistence of session memory and facts.
// Located in: pkg/memory/store.go
type MemoryStore interface {
    SaveSession(ctx context.Context, session SessionRecord) error
    SaveFacts(ctx context.Context, facts []MemoryFact) error
    SaveOpenTasks(ctx context.Context, tasks []OpenTask) error
    SaveResumePacket(ctx context.Context, packet ResumePacket) error

    LoadLatestResumePacket(ctx context.Context, repoID string) (*ResumePacket, error)
    LoadRelevantFacts(ctx context.Context, repoID string, scope string) ([]MemoryFact, error)
    MarkStale(ctx context.Context, factIDs []string) error
}
```

### Consolidator

```go
// Consolidator processes raw session artifacts into structured memory.
// Located in: pkg/memory/consolidator.go
type Consolidator interface {
    Consolidate(ctx context.Context, input ConsolidationInput) (*ConsolidationResult, error)
}
```

### DreamerWorker

```go
// DreamerWorker runs isolated post-session memory maintenance.
// Located in: pkg/memory/dreamer.go
type DreamerWorker interface {
    Run(ctx context.Context, sessionID string) error
}
```

### ConsolidationInput

```go
type ConsolidationInput struct {
    Session    SessionRecord
    Artifacts  []Artifact
    GitState   GitSnapshot
    PriorFacts []MemoryFact
}
```

### ConsolidationResult

```go
type ConsolidationResult struct {
    Summary      SessionSummary
    Facts        []MemoryFact
    OpenTasks    []OpenTask
    ResumePacket ResumePacket
}
```

---

## 12. Storage Approach

### MVP: Local Structured Files

```
.pedro/
  sessions/
    sess_123.json
  memory/
    facts.jsonl
    open_tasks.jsonl
  resume/
    pedrocode.latest.json
```

### Later: SQLite

Move to sqlite if needed for:
- better querying,
- deduplication,
- expiration,
- multi-session analytics.

---

## 13. Rollout Plan

### Phase 1: Session summary only
At session end: generate summary, generate next-step recommendation, save latest resume packet.

### Phase 2: Typed memory
Add: repo facts, open tasks, recent failures.

### Phase 3: Revalidation on resume
Before using saved memory: verify against repo state.

### Phase 4: Dreamer worker
Run isolated post-session consolidation automatically.

### Phase 5: Scheduled maintenance
Optional idle/scheduled memory cleanup.

### Phase 6: Event-triggered persistence
Later, maybe support repo events or ticket hooks.

---

## 14. Pros and Cons

### Pros
- better session continuity
- less repeated exploration
- smaller active context
- more structured state
- easier debugging
- cleaner GUI handoffs

### Cons
- more moving parts
- memory drift risk
- new storage layer
- requires fact validation discipline

---

## 15. Final Recommendation

For PedroCode, the right Kairos-inspired design is **not** "always-on autonomous coding."

It is:

> **Persistent session intelligence with isolated memory consolidation.**

That means:
- preserve useful state,
- clean it after the session,
- resume from a compact handoff,
- never let memory become authority,
- and keep consolidation outside the main coding loop.

That gets you the part that actually matters.

---

## Related Documents
- ADR-005: Agent Workflow Refactoring
- ADR-008: Phased Compaction Middleware
- ADR-009: Evaluation System Architecture
- PedroCode vNext Engineering Design Document (proposed)
