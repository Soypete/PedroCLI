# M5: Subagent Manager

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

The current PhasedExecutor runs all phases sequentially in one context. There's no way to spawn child agents with isolated contexts for parallel execution or to compose specialized workers (explorer, implementer, tester, reviewer).

## Solution

<!-- How we solved it -->

Implement a SubagentManager that spawns bounded workers with isolated contexts:

| Subagent | Role | Tools | Max Rounds |
|----------|------|-------|------------|
| `explorer` | Search and map codebase | search, navigate, file, lsp | 10 |
| `implementer` | Write code changes | file, code_edit, bash | 20 |
| `tester` | Run and fix tests | test, bash, file, code_edit | 15 |
| `reviewer` | Validate changes | search, file, git, test | 10 |
| `doc-writer` | Generate documentation | file, search, navigate | 10 |

Key design:
- Subagents get isolated context directories: `/tmp/pedrocli-jobs/<parent-id>/subagents/<child-id>/`
- Subagents inherit a subset of parent context (specified files, not full history)
- Subagents return TaskResult, not chat

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/subagent.go` - SubagentManager interface and types
- `pkg/orchestration/default_subagent_manager.go` - Implementation

### Modified Files
- `pkg/agents/base.go` - Add subagent spawning capability
- `pkg/llmcontext/manager.go` - Support child context directories

## Dependencies

- M1: Query Engine
- M4: Task Envelope

## Next Steps

- [M6: Artifact Store](../M6-artifact-store/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-5-subagent-manager)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m5-subagent-manager)