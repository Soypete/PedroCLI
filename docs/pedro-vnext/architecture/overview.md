# PedroCode vNext - Architecture Overview

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          GUI Layer                                      │
│  ┌─────────────────────┐              ┌─────────────────────────────┐  │
│  │  cmd/pedrocode (REPL) │            │  cmd/http-server (Web UI)  │  │
│  └──────────┬──────────┘              └──────────────┬──────────────┘  │
└─────────────┼─────────────────────────────────────────┼─────────────────┘
              │                                         │
              ▼                                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Session Controller                                   │
│                         pkg/repl/session.go                             │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       Query Engine (NEW)                                │
│                     pkg/orchestration/query_engine.go                  │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐  │
│  │   Intent     │  │    Mode      │  │   Phase      │  │  Subagent  │  │
│  │ Classification│  │   Engine     │  │   Engine     │  │  Manager   │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬─────┘  │
│         │                 │                 │                 │         │
│         ▼                 ▼                 ▼                 ▼         │
│  ┌──────────────────────────────────────────────────────────────┐      │
│  │                     Artifact Store                             │      │
│  │                   pkg/artifacts/store.go                       │      │
│  └──────────────────────────────────────────────────────────────┘      │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────┐      │
│  │                    Permission Engine                           │      │
│  │                  pkg/orchestration/permissions.go             │      │
│  └──────────────────────────────────────────────────────────────┘      │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────┐      │
│  │                     Prompt Builder                             │      │
│  │                pkg/orchestration/prompt_layers.go             │      │
│  └──────────────────────────────────────────────────────────────┘      │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Agent Layer                                     │
│  ┌──────────────────────────────────────────────────────────────┐      │
│  │                    BaseAgent + InferenceExecutor              │      │
│  │                     pkg/agents/executor.go                    │      │
│  └──────────────────────────────────────────────────────────────┘      │
│                                                                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐  │
│  │  Builder    │ │  Debugger   │ │  Reviewer   │ │   Triager       │  │
│  │  Phased     │ │  Phased     │ │  Phased     │ │   Phased        │  │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────────┘  │
└──────────────────────────────────┬──────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Tool Layer                                      │
│                   pkg/tools/registry.go + router.go                     │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────┐  │
│  │ search │ │ navigate│ │  file  │ │code_edit│ │  git   │ │   bash   │  │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └──────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      LLM Backend                                        │
│                    pkg/llm/ (Ollama, llama.cpp)                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### 1. User Input → Query Engine
```
User types query in REPL/Web UI
         ↓
QueryEngine.Classify() → Intent (chat/plan/build/debug/review)
         ↓
QueryEngine.Execute() → Selects mode, agent, phases
```

### 2. Query → Execution Plan
```
QueryEngine creates TaskEnvelope
         ↓
PhaseEngine.BuildPhases() → ExecutionPlan with phases
         ↓
(Optionally) SubagentManager.Spawn() → Parallel subagents
```

### 3. Execution → Artifacts
```
InferenceExecutor loops:
  - Send prompt to LLM
  - Parse tool calls
  - PermissionEngine.Check()
  - Execute tools
  - Store artifacts
         ↓
Phase completes → ArtifactStore.Put()
```

### 4. Session End → Memory
```
Session ends
         ↓
DreamerWorker.Run() → Consolidate artifacts into memory
         ↓
MemoryStore.SaveResumePacket() → .pedro/resume/
```

## Key Interfaces

### QueryEngine
- `Execute(ctx, query) → QueryResult`
- `Classify(ctx, input) → Intent`
- `ExecuteWithMode(ctx, input, mode) → QueryResult`

### PhaseEngine
- `Plan(ctx, query) → ExecutionPlan`
- `Execute(ctx, plan) → PlanResult`
- `RegisterPhaseTemplate(template)`

### SubagentManager
- `Spawn(ctx, task) → SubagentHandle`
- `Wait(ctx, handle) → TaskResult`
- `Cancel(handle)`

### ArtifactStore
- `Put(ctx, type, name, reader, creator) → Artifact`
- `Get(ctx, id) → Artifact`
- `List(ctx, filter) → []Artifact`

### PermissionEngine
- `Check(ctx, request) → PermissionDecision`
- `RequestApproval(ctx, request) → bool`

### MemoryStore
- `SaveSession(ctx, record)`
- `SaveFacts(ctx, facts)`
- `LoadLatestResumePacket(ctx, repoID) → ResumePacket`

## Where Each Component Applies

| Component | pedrocode (REPL) | HTTP Server | pedrocli (CLI) |
|-----------|:---------------:|:-----------:|:--------------:|
| Query Engine | Yes | Yes | N/A (direct commands) |
| Modes | Yes | Yes | N/A (implicit per command) |
| Phase Registry | Yes | Yes | Yes |
| Task Envelope | Yes | Yes | Yes |
| Subagents | Sequential | Parallel (goroutines) | Sequential |
| Artifacts | File + API | File + API | File |
| Permissions | Interactive (ask) | Interactive (SSE) | Config-only |
| Prompt Layers | Yes | Yes | Yes |
| Telemetry | File + display | File + API + UI | File |
| Kairos Memory | Yes | Yes | N/A |

## Configuration

New sections in `.pedrocli.json`:

```json
{
  "orchestration": {
    "enabled": true,
    "default_mode": "build",
    "max_subagents": 4
  },
  "modes": { ... },
  "permissions": { ... },
  "phases": { ... },
  "memory": { ... },
  "telemetry": { ... }
}
```

## Related Documents

- [Implementation Plan](../pedrocode-vnext-implementation-plan.md)
- [Interface Definitions](../pedrocode-vnext-interfaces.md)
- [ADR-012: Kairos Memory](../adr/ADR-012-kairos-memory-consolidation.md)
- [ADR-013: Orchestration Architecture](../adr/ADR-013-pedrocode-vnext-orchestration.md)