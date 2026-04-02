# M5: Subagent Manager

> Status: Implemented | Started: 2026-04-02 | Completed: 2026-04-02

## Problem

The current PhasedExecutor runs all phases sequentially in one context. There's no way to spawn child agents with isolated contexts for parallel execution or to compose specialized workers (explorer, implementer, tester, reviewer).

## Solution

Implemented a SubagentManager that spawns bounded workers with isolated contexts:

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

1. **Same LLM Backend**: Subagents use the same LLM backend as the parent agent by default. Future enhancement could allow per-subagent backend configuration.

2. **Sequential by Default**: SpawnAll executes sequentially (CLI pattern) by default. Parallel execution is available via the `parallel` flag for HTTP use cases.

3. **Context Inheritance via Files**: Parent passes specific files to child via TaskEnvelope.ContextFiles, not full history. Child creates its own isolated context directory.

4. **Config Path**: Used `m.config.Model.ContextSize` (not `m.config.Limits.ContextSize`) to get context limit.

5. **Error Handling**: TaskResult.Error is a string, so checks use `result.Error != ""` instead of `result.Error != nil`.

## Files Changed

### New Files
- `pkg/orchestration/subagent.go` - SubagentManager interface and types
- `pkg/orchestration/default_subagent_manager.go` - Implementation
- `pkg/orchestration/subagent_test.go` - Unit tests

### Modified Files
- `pkg/orchestration/default_subagent_manager.go:295` - Fixed errcheck lint error

## Known Issues / Technical Debt

1. **Cancel doesn't stop execution**: `Cancel()` only marks the handle as cancelled but doesn't cancel the context or signal the executing subagent to stop. The subagent continues running.

2. **Hardcoded timeout**: The `Wait()` method has a 5-minute timeout hardcoded. Should be configurable via TaskEnvelope or config.

3. **Limited test coverage**: Unit tests only cover types and helpers, not `DefaultSubagentManager` behavior (Spawn/Wait lifecycle, parallel execution, cancellation).

## Dependencies

- M1: Query Engine
- M4: Task Envelope

## Next Steps

- [M6: Artifact Store](../M6-artifact-store/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-5-subagent-manager)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m5-subagent-manager)