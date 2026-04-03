# M6: Artifact / Blackboard System

> Status: Completed | Started: 2026-02 | Completed: 2026-04

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

1. **In-memory store per job**: Each agent execution creates its own `InMemoryArtifactStore` scoped to that job's lifecycle. This keeps artifact storage simple and avoids persistence complexity.
2. **Automatic phase artifacts**: Phase outputs are automatically stored as artifacts after each phase completes, accessible to later phases via `GetArtifactByName`.
3. **Artifact types match phase names**: Each phase stores its output with `ArtifactType` equal to the phase name (e.g., "analyze", "plan", "implement").

## Files Changed

### Existing Files (Already existed, wired up in this phase)
- `pkg/artifacts/store.go` - ArtifactStore interface + InMemoryArtifactStore
- `pkg/artifacts/types.go` - Artifact, ArtifactType, ArtifactFilter
- `pkg/agents/phased_executor.go` - Added SetArtifactStore, storePhaseArtifact methods

### Modified Files
- `pkg/agents/builder_phased.go` - Creates artifact store and sets on executor
- `pkg/agents/debugger_phased.go` - Creates artifact store and sets on executor
- `pkg/agents/reviewer_phased.go` - Creates artifact store and sets on executor
- `pkg/repl/interactive_sync.go` - Creates artifact store for REPL execution
- `pkg/httpbridge/app.go` - Already had ArtifactStore in AppContext

## Integration Points

| Interface | How Artifact Store is Wired |
|-----------|----------------------------|
| CLI (`cmd/pedrocli`) | Agents create in-memory store per job in Execute() |
| HTTP Server (`cmd/http-server`) | Agents create in-memory store per job |
| REPL (`pkg/repl/`) | Created in `executePhasedAgentSync` |

## Dependencies

- M5: Subagent Manager

## Next Steps (Optional Enhancements)

- Persist artifacts to file/database (currently in-memory only)
- Add artifact versioning for audit trail
- [M7: Permission Engine](../M7-permission-engine/)
- [M8: Prompt Architecture](../M8-prompt-architecture/)
- [M9: Telemetry](../M9-telemetry/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-6-artifact--blackboard-system)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m6-artifact-store)