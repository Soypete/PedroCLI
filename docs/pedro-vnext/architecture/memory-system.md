# Memory System Architecture

This document describes the Kairos-inspired memory consolidation system for PedroCode vNext.

## Overview

The memory system provides session continuity through:
1. **End-of-session consolidation** - Extract structured memory from artifacts
2. **Resume packets** - Compact handoff for next session
3. **Dreamer worker** - Isolated post-session maintenance

## Design Principles

1. **Memory is a hint, not source of truth** - Must be revalidated before use
2. **Consolidation should be isolated** - Separate worker, not main loop
3. **Structured memory beats transcript memory** - Typed artifacts, not prose

## Storage Structure

```
.pedro/                              # Project root or ~/.pedro/
├── sessions/
│   └── {repo_id}/
│       └── {session_id}.json        # Session records
├── memory/
│   ├── facts.jsonl                  # Structured memory facts
│   ├── open_tasks.jsonl             # Unfinished work
│   └── failures.jsonl               # Recent failures
└── resume/
    └── {repo_id}.latest.json        # Resume packet for next session
```

## Key Components

### MemoryStore

```go
type MemoryStore interface {
    SaveSession(ctx, session SessionRecord) error
    SaveFacts(ctx, facts []MemoryFact) error
    SaveOpenTasks(ctx, tasks []OpenTask) error
    SaveResumePacket(ctx, packet ResumePacket) error

    LoadLatestResumePacket(ctx, repoID) (*ResumePacket, error)
    LoadRelevantFacts(ctx, repoID, scope) ([]MemoryFact, error)
    MarkStale(ctx, factIDs []string) error
}
```

### Consolidator

```go
type Consolidator interface {
    Consolidate(ctx, input ConsolidationInput) (*ConsolidationResult, error)
}
```

### DreamerWorker

```go
type DreamerWorker interface {
    Run(ctx, sessionID string) error
}
```

## Data Types

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
  "artifacts": ["artifact_repo_map_1", "artifact_diff_2"]
}
```

### MemoryFact

```json
{
  "id": "mem_45",
  "type": "repo_fact",
  "scope": "repo",
  "subject": "cmd/pedrocode",
  "fact": "Query execution lives in orchestration package",
  "confidence": "medium",
  "evidence_artifacts": ["artifact_repo_map_1"],
  "last_validated_at": "2026-04-01T15:12:00Z"
}
```

### ResumePacket

```json
{
  "repo_id": "pedrocode",
  "branch": "feature/orchestration",
  "goal": "Implement QueryEngine",
  "next_step": "Wire into REPL session",
  "changed_files": ["pkg/orchestration/query_engine.go"],
  "warnings": ["Permission engine not yet integrated"]
}
```

## Consolidation Pipeline

### Trigger Conditions

- User ends session
- Session idle for N minutes
- Token/context threshold exceeded
- Explicit "save and summarize" action

### Pipeline Stages

1. **Collect** - Gather artifacts, messages, tool calls, diffs
2. **Classify** - Sort into facts, decisions, failures, open work
3. **Summarize** - Produce session summary, task list, next step
4. **Normalize** - Convert to typed memory objects
5. **Validate** - Link facts to evidence artifacts
6. **Prune** - Discard duplicates, obsolete reasoning
7. **Save** - Create resume packet

## Dreamer Worker

### Responsibilities

- Read completed session artifacts
- Extract durable facts
- Generate resume packet
- Merge/update existing memory
- Mark stale facts for revalidation

### What It MUST NOT Do

- Never modify code
- Never run unsafe tools
- Never take external actions

### Allowed Tools

- Read artifacts
- Read repo metadata
- Read git status
- Read prior memory
- Write memory store
- Write summaries

### Forbidden Tools

- File edits
- Shell execution beyond read-only
- Network actions
- Git mutations
- Autonomous code generation

## Resume Flow

When user opens PedroCode again:

1. **Load ResumePacket** - `MemoryStore.LoadLatestResumePacket()`
2. **Show Handoff** - Display goal, changes, blockers, next step
3. **Revalidate** - Check branch exists, files present, tests still fail
4. **Seed Session** - Use validated facts to accelerate startup

## Guardrails

| Guardrail | Description |
|-----------|-------------|
| No autonomous edits | Dreamer never edits code |
| No memory-only execution | Memory is hint, not authority |
| Evidence required | Every fact must reference artifact |
| TTL + staleness | Facts expire or get revalidated |
| Fact vs inference | Distinguish observed vs inferred |

### TTL Recommendations

- **Repo facts**: Revalidate weekly
- **Branch facts**: Revalidate per session
- **Failure facts**: Revalidate after related file changes
- **User preferences**: Persist longer

## Integration with Phased Execution

Full lifecycle:
```
intake → plan → explore → implement → validate → summarize → consolidate
```

The **consolidate** phase runs via DreamerWorker after active session ends.

## Integration Points

### REPL Session

```go
type Session struct {
    // ... existing fields
    MemoryStore *FileMemoryStore
    Dreamer     *DreamerWorker
}

func (s *Session) Start() error {
    // Load resume packet on start
    packet, err := s.MemoryStore.LoadLatestResumePacket(s.repoID)
    if err == nil && packet != nil {
        s.DisplayHandoff(packet)
    }
    // ...
}

func (s *Session) Close() error {
    // Run dreamer on close
    go s.Dreamer.Run(context.Background(), s.sessionID)
    // ...
}
```

### Query Context

ResumePacket feeds into QueryContext:

```go
type QueryContext struct {
    // ... existing fields
    ResumePacket *ResumePacket `json:"resume_packet,omitempty"`
}
```

## Related Documents

- [ADR-012: Kairos Memory Consolidation](../adr/ADR-012-kairos-memory-consolidation.md)
- [Implementation Plan](../pedrocode-vnext-implementation-plan.md#milestone-10-kairos-memory-consolidation-adr-012)
- [M10: Kairos Memory Milestone](../milestones/M10-kairos-memory/)