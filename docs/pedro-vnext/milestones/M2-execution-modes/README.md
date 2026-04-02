# M2: Execution Modes

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently there is no way to constrain what an agent can do. All queries execute with the same permissions and tool access, regardless of user intent.

## Solution

<!-- How we solved it -->

Introduce execution modes that define behavioral constraints:

| Mode | Allowed Tools | Writes | Agent Types |
|------|--------------|--------|-------------|
| `chat` | search, navigate, file (read), context | No | any (read-only) |
| `plan` | search, navigate, file (read), context, git (status) | No | triager, reviewer |
| `build` | all | Yes | builder, debugger |
| `review` | search, navigate, file (read), git, github, test | No | reviewer |

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/mode.go` - Mode and ModeConfig
- `pkg/orchestration/mode_config.go` - YAML/JSON mode configuration

### Modified Files
- `pkg/agents/executor.go` - Respect mode constraints in executeTool()
- `pkg/repl/session.go` - Add /mode slash command, persist mode

## Dependencies

- M1: Query Engine (required)

## Next Steps

- [M3: Phase Registry](../M3-phase-registry/)
- [M7: Permission Engine](../M7-permission-engine/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-2-execution-modes)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m2-execution-modes)