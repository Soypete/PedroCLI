# PedroCode vNext

Implementation documentation for the PedroCode orchestration system. This folder contains detailed technical documentation for each milestone, written during implementation to preserve context for future reference.

## Overview

PedroCode vNext evolves from a GUI chat interface with tools into a **phased, multi-agent orchestration system** with structured execution, permissions, and artifact-driven workflows.

## Milestones

| Milestone | Status | Description |
|-----------|--------|-------------|
| [M1: Query Engine](./milestones/M1-query-engine/) | Completed | Intent routing and dispatch |
| [M2: Execution Modes](./milestones/M2-execution-modes/) | Completed | chat/plan/build/review modes |
| [M3: Phase Registry](./milestones/M3-phase-registry/) | Completed | Reusable phase catalog |
| [M4: Task Envelope](./milestones/M4-task-envelope/) | Completed | Structured I/O between agents |
| [M5: Subagent Manager](./milestones/M5-subagent-manager/) | Completed | Spawn child agents with bounded execution |
| [M6: Artifact Store](./milestones/M6-artifact-store/) | Completed | Structured shared workspace |
| [M7: Permission Engine](./milestones/M7-permission-engine/) | Completed | Per-agent, per-tool, per-path permissions |
| [M8: Prompt Architecture](./milestones/M8-prompt-architecture/) | Completed | Layered prompt composition |
| [M9: Telemetry](./milestones/M9-telemetry/) | Planned | Token/cost tracking |
| [M10: Kairos Memory](./milestones/M10-kairos-memory/) | Planned | Session continuity and consolidation |

## Architecture

- [Architecture Overview](./architecture/overview.md)
- [Query Engine](./architecture/query-engine.md)
- [Memory System](./architecture/memory-system.md)
- [Integration Points](./architecture/integration.md)

## Related Documents

- [Implementation Plan](../pedrocode-vnext-implementation-plan.md) - Full milestone plan
- [Interface Definitions](../pedrocode-vnext-interfaces.md) - Go interface specs
- [ADR-012: Kairos Memory](../adr/ADR-012-kairos-memory-consolidation.md) - Memory design
- [ADR-013: Orchestration Architecture](../adr/ADR-013-pedrocode-vnext-orchestration.md) - Architecture

## Quick Links

- **Code**: `pkg/orchestration/`
- **REPL Integration**: `pkg/repl/session.go`
- **HTTP Integration**: `pkg/httpbridge/handlers.go`
- **CLI Integration**: `cmd/pedrocli/setup.go`

## Goals

Primary: Make Pedro **reliable, debuggable, and controllable** — not just "smart"

Non-goals:
- Not cloning Claude Code
- Not replacing GUI with CLI
- Not building a research agent framework