# Integration Points

This document describes how the orchestration layer integrates with existing PedroCLI components.

## REPL Integration (`cmd/pedrocode`)

### Entry Point
The REPL is in `pkg/repl/session.go`. Currently:
```go
type Session struct {
    Bridge *cli.CLIBridge
    Mode   string
    // ...
}
```

### QueryEngine Integration

Replace direct agent dispatch with QueryEngine:

```go
// Before: switch on slash command, create agent, execute
// After:
result, err := session.queryEngine.Execute(ctx, userInput)
```

### Mode Persistence

```go
// Add to Session struct
type Session struct {
    QueryEngine *orchestration.DefaultQueryEngine
    ModeEngine  *orchestration.ModeEngine
    // ...
}
```

### Slash Commands

Add new commands:
- `/mode chat|plan|build|review` - Switch execution mode
- `/subagents` - List active subagents
- `/resume` - Load and display resume packet

## HTTP Server Integration (`cmd/http-server`)

### AppContext
The HTTP server uses `pkg/httpbridge/app.go` for shared dependencies:

```go
type AppContext struct {
    QueryEngine *orchestration.DefaultQueryEngine
    // ...
}
```

### Handler Integration

In `pkg/httpbridge/handlers.go`, replace job creation switch:

```go
// Before:
switch req.Type {
case "builder":
    agent = NewBuilderPhasedAgent(...)
case "debugger":
    agent = NewDebuggerPhasedAgent(...)
// ...
}

// After:
result, err := app.queryEngine.ExecuteWithMode(ctx, req.Description, mode)
```

### SSE for Subagents

When subagents run in parallel, emit progress via SSE:
```go
// SubagentManager emits events
sse.WriteEvent("subagent-progress", SubagentStatus{...})
```

### Permission Approval

"ask" permissions in HTTP mode should prompt via SSE:
```go
// In PermissionEngine
decision, err := permissions.Check(ctx, request)
if decision.NeedsApproval {
    // Send approval request via SSE
    sse.WriteEvent("permission-request", request)
    // Wait for approval response
}
```

## CLI Integration (`cmd/pedrocli`)

The CLI uses direct commands (build, debug, review, triage) without orchestration:
- QueryEngine is NOT required for CLI
- But orchestration config should be loaded
- Pass through to existing agents

### Setup

In `cmd/pedrocli/setup.go`:

```go
// Load orchestration config (optional for CLI)
var qe *orchestration.DefaultQueryEngine
if cfg.Orchestration.Enabled {
    qe = orchestration.NewDefaultQueryEngine(cfg, backend, jobManager)
}
```

## Blog Agents (No Integration)

Blog agents (BlogOrchestratorAgent, BlogContentAgent) work differently:
- Already have multi-phase orchestration
- Should use ArtifactStore for research results
- Don't need QueryEngine (fixed workflow)
- Could use SubagentManager for parallel research

## Podcast Agents (No Integration)

Podcast agents have fixed workflows:
- No integration required
- Could benefit from ArtifactStore

## Job Manager Integration

The Job Manager (`pkg/jobs/manager.go`) tracks job lifecycle:

### Subagent Tracking

```go
// Subagents create child jobs
type Job struct {
    ParentID string  // Parent job ID
    AgentType string // explorer, implementer, tester, reviewer
    // ...
}
```

### Artifact Association

```go
// Artifacts link to jobs
type Artifact struct {
    JobID string
    // ...
}
```

## LLM Backend Integration

The orchestration layer uses the existing LLM backend interface:

```go
type Backend interface {
    Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)
    GetContextWindow() int
    GetUsableContext() int
}
```

QueryEngine and SubagentManager don't need to know the backend type.

## Config Integration

New config sections in `.pedrocli.json`:

```json
{
  "orchestration": {
    "enabled": true,
    "default_mode": "build"
  },
  "modes": { ... },
  "permissions": { ... },
  "phases": { ... },
  "memory": { ... },
  "telemetry": { ... }
}
```

Loaded in `pkg/config/config.go`:
```go
type Config struct {
    // ... existing fields
    Orchestration *OrchestrationConfig `json:"orchestration,omitempty"`
    Modes         map[string]ModeConfig `json:"modes,omitempty"`
    Permissions   *PermissionConfig     `json:"permissions,omitempty"`
    Phases        PhaseTemplates        `json:"phases,omitempty"`
    Memory        *MemoryConfig         `json:"memory,omitempty"`
    Telemetry     *TelemetryConfig      `json:"telemetry,omitempty"`
}
```

## Telemetry API

Add new endpoints:

| Endpoint | Description |
|----------|-------------|
| `GET /api/telemetry/:job_id` | Get job telemetry |
| `GET /api/telemetry/:job_id/events` | Get raw events |
| `GET /api/sessions/:session_id/metrics` | Session summary |

## Backward Compatibility

Ensure `orchestration.enabled: false` falls through to existing behavior:

```go
func (s *Session) Execute(ctx context.Context, input string) (*Result, error) {
    if s.queryEngine != nil {
        return s.queryEngine.Execute(ctx, input)
    }
    // Fall through to existing agent dispatch
    return s.legacyExecute(ctx, input)
}
```

## Related Documents

- [Implementation Plan](../pedrocode-vnext-implementation-plan.md#where-each-feature-applies)
- [Interface Definitions](../pedrocode-vnext-interfaces.md#integration-points-with-existing-code)
- [ADR-013: Where This Applies](../adr/ADR-013-pedrocode-vnext-orchestration.md#where-this-applies)