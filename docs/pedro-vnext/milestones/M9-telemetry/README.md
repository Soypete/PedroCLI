# M9: Telemetry + Cost Tracking

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, token usage and latency are tracked partially (InferenceResponse.TokensUsed, PhaseResult.RoundsUsed) but there's no unified telemetry system that aggregates and reports on execution metrics.

## Solution

<!-- How we solved it -->

Implement a telemetry collector that tracks:

| Metric | Granularity | Storage |
|--------|-------------|---------|
| Tokens (prompt + completion) | Per inference round | Job file |
| Latency (LLM call) | Per inference round | Job file |
| Tool calls (count, success/fail) | Per round | Job file |
| Phase duration | Per phase | Job file |
| Total job cost estimate | Per job | Job file + summary |
| Retries and failures | Per tool call | Job file |

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/telemetry.go` - TelemetryCollector interface
- `pkg/orchestration/file_telemetry.go` - File-based implementation
- `pkg/orchestration/telemetry_types.go` - ExecutionMetrics, SessionMetricsSummary

### Modified Files
- `pkg/agents/executor.go` - Emit telemetry events
- `pkg/agents/phased_executor.go` - Emit phase telemetry
- `pkg/httpbridge/handlers.go` - Expose telemetry API

## Dependencies

- None (can start anytime after M1)

## Next Steps

- [M10: Kairos Memory](../M10-kairos-memory/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-9-telemetry--cost-tracking)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m9-telemetry)