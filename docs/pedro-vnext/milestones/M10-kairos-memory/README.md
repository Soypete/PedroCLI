# M10: Kairos Memory Consolidation

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, each session starts from scratch. There's no:
- Session summary for continuity
- Memory of what was tried and failed
- Resume packet for the next session
- Post-session consolidation

## Solution

<!-- How we solved it -->

Implement a post-session maintenance loop (inspired by KAIROS):

### Storage Structure
```
.pedro/
├── sessions/
│   └── sess_123.json
├── memory/
│   ├── facts.jsonl
│   └── open_tasks.jsonl
└── resume/
    └── pedrocode.latest.json
```

### What Gets Stored

1. **Session Summary** - What happened, files changed, blockers
2. **Structured Memory** - Typed facts (repo_facts, architecture_notes, tool_hints)
3. **Open Tasks** - Unfinished work
4. **Recent Failures** - What was tried and didn't work
5. **Resume Packet** - Branch, goal, next step, changed files

### The Dreamer Worker

A separate maintenance component that runs after the session ends:
- Reads artifacts
- Extracts durable facts
- Generates resume packet
- **Never modifies code**
- **Never runs unsafe tools**

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/memory/types.go` - SessionRecord, MemoryFact, OpenTask, ResumePacket
- `pkg/memory/store.go` - MemoryStore interface
- `pkg/memory/file_store.go` - File-based implementation
- `pkg/memory/consolidator.go` - LLM-based consolidation
- `pkg/memory/dreamer.go` - DreamerWorker
- `pkg/memory/resume.go` - Resume loader + validator

### Modified Files
- `pkg/repl/session.go` - Load resume on start, run dreamer on close

## Dependencies

- M1: Query Engine (can start after M1)

## Guardrails

1. **No autonomous edits** - Dreamer never edits code
2. **No memory-only execution** - Memory is a hint, not authority
3. **Evidence required** - Every fact must reference an artifact
4. **TTL + staleness** - Facts expire or get revalidated

## Reference

- [ADR-012: Kairos Memory Consolidation](../../adr/ADR-012-kairos-memory-consolidation.md)
- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-10-kairos-memory-consolidation-adr-012)