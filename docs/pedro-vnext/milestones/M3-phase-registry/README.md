# M3: Phase Registry

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Phases (analyze, plan, implement, validate, deliver) are currently hardcoded per agent. The BuilderPhasedAgent, DebuggerPhasedAgent, and ReviewerPhasedAgent each define their own phase lists with duplicated definitions.

## Solution

<!-- How we solved it -->

Create a reusable phase registry where phases are defined once and can be composed by any agent:

| Phase | Tools | Used By |
|-------|-------|---------|
| `analyze` | search, navigate, file, git, lsp | builder, debugger, reviewer, triager |
| `plan` | search, navigate, file, context | builder, debugger |
| `implement` | file, code_edit, search, git, bash, lsp | builder, debugger |
| `validate` | test, bash, file, code_edit, search | builder, debugger |
| `deliver` | git, github | builder |
| `review` | search, navigate, file, git, github | reviewer |

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/phase_registry.go` - PhaseRegistry interface and implementation

### Modified Files
- `pkg/agents/builder_phased.go` - Refactor to use registry
- `pkg/agents/debugger_phased.go` - Refactor to use registry
- `pkg/agents/reviewer_phased.go` - Refactor to use registry

## Dependencies

- None (can start independently)

## Next Steps

- [M4: Task Envelope](../M4-task-envelope/)
- [M1: Query Engine](../M1-query-engine/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-3-phase-registry)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m3-phase-registry)