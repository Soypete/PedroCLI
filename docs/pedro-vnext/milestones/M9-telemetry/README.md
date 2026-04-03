# M9: Telemetry + Cost Tracking

> Status: Completed | Completed: 2025-04-02

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, token usage and latency are tracked partially (InferenceResponse.TokensUsed, PhaseResult.RoundsUsed) but there's no unified telemetry system that aggregates and reports on execution metrics.

## Solution

<!-- How we solved it -->

Implement a telemetry collector that tracks:

| Metric | Granularity | Storage |
|--------|-------------|---------|
| Tokens (prompt + completion) | Per inference round | Job file (telemetry.jsonl) |
| Latency (LLM call) | Per inference round | Job file |
| Tool calls (count, success/fail) | Per round | Job file |
| Phase duration | Per phase | Job file |
| Total job cost estimate | Per job | Job file + API response |
| Retries and failures | Per tool call | Job file |

## Implementation

### Core Components

**Telemetry Types** (`pkg/telemetry/types.go`):
- `TelemetryEvent` - Single event with JobID, AgentID, Phase, Round, EventType, Data
- `TelemetrySummary` - Aggregated metrics for a job
- `EventType` - Enum: inference, tool_call, phase_complete, job_complete, error
- `EstimateCost()` - Token pricing calculator for common models

**Collectors** (`pkg/collector.go`):
- `TelemetryCollector` interface - Record(), Summary(), Events(), Close()
- `InMemoryCollector` - In-memory storage (for testing)
- `FileTelemetryCollector` - JSONL file storage in job directory

### Wiring

**InferenceExecutor** (`pkg/agents/executor.go`):
- Added `telemetryCollector` and `jobID` fields
- Added `SetTelemetryCollector()` method
- Records inference events after each LLM call (tokens, latency, success)

**PhasedExecutor** (`pkg/agents/phased_executor.go`):
- Added `telemetryCollector` field
- Added `SetTelemetryCollector()` method
- Records phase completion events (duration, rounds, tokens, success)
- Records job completion events (total stats, estimated cost)

**HTTP API** (`pkg/httpbridge/handlers.go`):
- Added `/api/jobs/:id/telemetry` endpoint
- Returns events array and calculated summary
- Includes: total_tokens, rounds, phases, tool_calls, estimated_cost

## Usage

### HTTP API
```bash
curl http://localhost:8080/api/jobs/job-123/telemetry
```

Response:
```json
{
  "job_id": "job-123",
  "events": [...],
  "summary": {
    "total_tokens": 5000,
    "prompt_tokens": 2500,
    "completion_tokens": 2500,
    "total_rounds": 5,
    "total_phases": 3,
    "tool_calls": 12,
    "tool_failures": 1,
    "estimated_cost": 0.005,
    "llm_latency_seconds": 45.2,
    "phases": ["analyze", "plan", "implement"]
  }
}
```

### CLI (Future Enhancement)
After job completion, display summary:
```
Job completed in 5 rounds, 3 phases
Tokens: 5,000 (prompt: 2,500, completion: 2,500)
Tool calls: 12 (1 failure)
LLM latency: 45.2s
Estimated cost: $0.005
```

### REPL Side Panel (Future Enhancement)
- Live telemetry updates during execution
- Round-by-round token count
- Current phase progress

## Key Decisions

1. **Decision**: Per-job telemetry storage in `telemetry.jsonl`
   - **Reason**: Simple, inspectable, survives restarts
2. **Decision**: Estimated cost calculation uses hardcoded model pricing
   - **Reason**: Quick estimate; real cost requires backend API
3. **Decision**: API calculates summary on-demand from raw events
   - **Reason**: Keep storage simple; compute is cheap

## Files Changed

### New Files
- `pkg/telemetry/types.go` - TelemetryEvent, TelemetrySummary, EventType, cost estimation
- `pkg/telemetry/collector.go` - TelemetryCollector interface, InMemoryCollector, FileTelemetryCollector

### Modified Files
- `pkg/agents/executor.go` - Added SetTelemetryCollector, recordInferenceTelemetry
- `pkg/agents/phased_executor.go` - Added SetTelemetryCollector, phase/job telemetry recording
- `pkg/httpbridge/handlers.go` - Added handleJobTelemetry endpoint, calculateTelemetrySummary
- `pkg/httpbridge/server.go` - Registered /api/jobs/*/telemetry route

## Dependencies

- None (can start anytime after M1)

## Next Steps

- [M10: Kairos Memory](../M10-kairos-memory/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-9-telemetry--cost-tracking)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m9-telemetry)