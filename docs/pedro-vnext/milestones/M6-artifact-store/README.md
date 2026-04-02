# M6: Artifact / Blackboard System

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, agent outputs are stored as unstructured text files. There's no structured way to:
- Share data between subagents
- Retrieve specific outputs (repo maps, diffs, test results)
- Version control artifacts
- Query artifacts by type

## Solution

<!-- How we solved it -->

Implement a structured artifact store with typed artifacts:

| Artifact | Format | Produced By | Consumed By |
|----------|--------|-------------|-------------|
| `repo_map.json` | JSON | explorer | planner, implementer |
| `task.json` | JSON | query engine | all agents |
| `plan.md` | Markdown | planner | implementer |
| `diff.patch` | Unified diff | implementer | reviewer, tester |
| `test_results.json` | JSON | tester | reviewer, implementer |
| `review.md` | Markdown | reviewer | implementer |

Storage: File-based under job directory
```
/tmp/pedrocli-jobs/<job-id>/
├── artifacts/
│   ├── repo_map.json
│   ├── task.json
│   └── test_results.json
```

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/artifacts/store.go` - ArtifactStore interface
- `pkg/artifacts/file_store.go` - File-based implementation
- `pkg/artifacts/types.go` - Artifact, ArtifactType, ArtifactFilter
- `pkg/artifacts/blackboard.go` - Convenience read/write

### Modified Files
- `pkg/llmcontext/manager.go` - Integrate artifact storage
- `pkg/agents/phased_executor.go` - Read/write artifacts between phases

## Dependencies

- M5: Subagent Manager

## Next Steps

- [M8: Prompt Architecture](../M8-prompt-architecture/)
- [M9: Telemetry](../M9-telemetry/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-6-artifact--blackboard-system)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m6-artifact-store)