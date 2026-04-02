# M4: Task Envelope + Structured I/O

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, phases and agents pass unstructured strings between each other. This makes it hard to:
- Parse agent outputs reliably
- Chain agents together
- Validate what an agent produced
- Debug execution flow

## Solution

<!-- How we solved it -->

Introduce typed task envelopes that flow through the system:

```go
type TaskEnvelope struct {
    ID           string
    Agent        string
    Goal         string
    Mode         string
    Context      TaskContext
    ToolsAllowed []string
    MaxSteps     int
    ReturnSchema map[string]string
}
```

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/task.go` - TaskEnvelope, TaskContext, TaskResult

### Modified Files
- `pkg/agents/phased_executor.go` - Accept/return task envelopes
- `pkg/agents/executor.go` - Accept/return task envelopes

## Dependencies

- M1: Query Engine
- M3: Phase Registry

## Next Steps

- [M5: Subagent Manager](../M5-subagent-manager/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-4-task-envelope--structured-io)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m4-task-envelope)