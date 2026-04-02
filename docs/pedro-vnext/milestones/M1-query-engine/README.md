# M1: Query Engine

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

The current system has no orchestration layer between the GUI and agents. The path is:
```
REPL → CLIBridge → Agent
```

This means:
- No intent classification — the model guesses what the user wants
- No mode switching — all queries go through the same execution path
- No structured dispatch — agents are selected via switch statements
- No unified entry point — REPL and HTTP have separate code paths

## Solution

<!-- How we solved it -->

Introduce a **Query Engine** that sits between user input and agent execution:

```
REPL/HTTP → QueryEngine → Agent → Result
```

The Query Engine:
1. Classifies user intent (chat, plan, build, debug, review, triage)
2. Selects the appropriate execution mode
3. Routes to the correct agent
4. Returns structured results

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/query_engine.go` - QueryEngine interface and types
- `pkg/orchestration/default_query_engine.go` - Default implementation
- `pkg/orchestration/mode.go` - Mode definitions and ModeEngine

### Modified Files
- `pkg/repl/session.go` - Wire QueryEngine into REPL
- `pkg/httpbridge/handlers.go` - Wire QueryEngine into HTTP
- `AGENTS.md` - Add design doc references

## Integration Points

### REPL Integration
```go
// In pkg/repl/session.go
result, err := session.queryEngine.Execute(ctx, userInput)
```

### HTTP Integration
```go
// In pkg/httpbridge/handlers.go
result, err := app.queryEngine.ExecuteWithMode(ctx, req.Description, mode)
```

### CLI Integration
```go
// In cmd/pedrocli/setup.go - orchestration config passed through
```

## Testing

<!-- How to test this milestone -->
- Unit tests for intent classification (rule-based + LLM)
- Integration tests for REPL dispatch
- Integration tests for HTTP dispatch

## Dependencies

- M2: Execution Modes (can run in parallel)
- M3: Phase Registry (can run in parallel)

## Next Steps

- [M2: Execution Modes](../M2-execution-modes/)
- [M3: Phase Registry](../M3-phase-registry/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-1-query-engine-session-controller)
- [ADR-013: Orchestration Architecture](../../adr/ADR-013-pedrocode-vnext-orchestration.md#1-query-engine-orchestrator)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m1-query-engine)