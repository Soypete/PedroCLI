# PedroCode vNext — Implementation Plan

**Project**: cmd/pedrocode + pkg/agents  
**Owner**: Miriah Peterson  
**Status**: Proposed  
**Date**: 2026-04-01  
**Related ADRs**: ADR-005 (Agent Workflow Refactoring), ADR-009 (Evaluation System), ADR-011 (Middleware Tool Filtering)

---

## Executive Summary

This document maps the PedroCode vNext engineering design into concrete Go implementation milestones. Each milestone produces a shippable increment that builds on existing code without breaking current functionality.

**Current state**: PedroCLI already has strong foundations — `PhasedExecutor`, `ToolRegistry`, `InferenceExecutor`, middleware policy evaluation, and phased agents (builder, debugger, reviewer). The design doc's concepts map cleanly onto these primitives.

**Key insight**: ~60% of the proposed architecture already exists in some form. The work is primarily about **composing existing pieces into a coherent orchestration layer**, not building from scratch.

---

## Audit: What Exists vs What's Needed

### Already Built

| Concept | Design Doc Name | Existing Code | Status |
|---------|----------------|---------------|--------|
| Phase execution | Phase Engine | `pkg/agents/phased_executor.go` — `PhasedExecutor` | Exists, needs enhancement |
| Tool restrictions per phase | Tool Router | `Phase.Tools []string` field + registry filtering | Exists |
| Tool metadata & discovery | Tool Router | `ExtendedTool` + `ToolRegistry` + `ToolMetadata` | Exists |
| Middleware validation | Permission Engine | `middleware.PolicyEvaluator` in `InferenceExecutor` | Exists, needs expansion |
| Progress events | Telemetry | `ProgressCallback` + `ProgressEvent` types | Exists, needs metrics |
| File-based context | Artifact System | `pkg/llmcontext/Manager` — file-per-turn storage | Exists |
| Phase callbacks | Session Controller | `PhaseCallback` in `PhasedExecutor` | Exists |
| Phased agents | Subagents | `BuilderPhasedAgent`, `DebuggerPhasedAgent`, `ReviewerPhasedAgent` | Exist |
| Multi-step orchestration | Query Engine | `BlogOrchestratorAgent` — research → outline → expand → publish | Exists as pattern |
| Prompt layering | Prompt Architecture | Phase-specific `SystemPrompt` + base system prompt | Partial |

### Gaps to Fill

| Concept | Design Doc Name | What's Missing |
|---------|----------------|----------------|
| Intent routing | Query Engine | No dispatcher that selects mode/agent/phase from user input |
| Execution modes | Modes | No `chat`/`plan`/`build`/`review` mode system |
| Subagent spawning | Subagent Manager | No mechanism to spawn child agents with isolated context |
| Structured task envelope | Task Envelope | No typed task input/output contract between agents |
| Shared artifact store | Blackboard | Context files exist but no structured artifact registry |
| Planner→Executor→Reviewer | Orchestration | Pattern exists in phased agents but not as reusable orchestrator |
| Per-agent permissions | Permission Engine | Middleware exists but not configured per-agent or per-path |
| Token/cost telemetry | Telemetry | `TokensUsed` tracked but not aggregated or reported |
| Phase registry | Phase Engine | Phases hardcoded per agent, not reusable across agents |

---

## Milestone Plan

### Milestone 1: Query Engine (Session Controller)

**Goal**: Wrap existing execution with an intent router that selects the right agent and mode.

**Files to create/modify**:
- `pkg/orchestration/query_engine.go` — new package
- `pkg/orchestration/mode.go` — mode definitions
- `cmd/pedrocode/code.go` — wire query engine into REPL bridge

**What it does**:
- Receives user input from REPL/HTTP
- Classifies intent: chat, plan, build, review, debug, triage
- Selects appropriate agent + mode
- Passes to existing executor

**No UI changes required.** The REPL continues to work; the query engine sits between user input and agent dispatch.

**Issues**:
1. Define `QueryEngine` interface and `DefaultQueryEngine` struct
2. Implement intent classification (LLM-based or heuristic)
3. Wire into `CLIBridge` and `httpbridge.AppContext`
4. Add mode flag to `Session` struct

---

### Milestone 2: Execution Modes

**Goal**: Add `chat`, `plan`, `build`, `review` modes that constrain what agents and tools are available.

**Files to create/modify**:
- `pkg/orchestration/mode.go` — mode definitions + constraints
- `pkg/orchestration/mode_config.go` — YAML/JSON mode configuration
- `pkg/agents/executor.go` — respect mode constraints
- `pkg/agents/phased_executor.go` — respect mode constraints

**Mode definitions**:

| Mode | Allowed Tools | Writes? | Agent Types |
|------|--------------|---------|-------------|
| `chat` | search, navigate, file (read), context | No | any (read-only) |
| `plan` | search, navigate, file (read), context, git (status) | No | triager, reviewer |
| `build` | all | Yes | builder, debugger |
| `review` | search, navigate, file (read), git, github, test | No | reviewer |

**Issues**:
1. Define `Mode` type with tool allowlists
2. Add mode selection to query engine
3. Enforce mode constraints in `InferenceExecutor.executeTool()`
4. Add `/mode` slash command to REPL
5. Persist mode in session state

---

### Milestone 3: Phase Registry

**Goal**: Make phases reusable across agents instead of hardcoded per agent.

**Files to create/modify**:
- `pkg/orchestration/phase_registry.go` — phase catalog
- `pkg/agents/builder_phased.go` — refactor to use registry
- `pkg/agents/debugger_phased.go` — refactor to use registry
- `pkg/agents/reviewer_phased.go` — refactor to use registry

**Reusable phases**:

| Phase | Tools | Used By |
|-------|-------|---------|
| `analyze` | search, navigate, file, git, lsp | builder, debugger, reviewer, triager |
| `plan` | search, navigate, file, context | builder, debugger |
| `implement` | file, code_edit, search, git, bash, lsp | builder, debugger |
| `validate` | test, bash, file, code_edit, search | builder, debugger |
| `deliver` | git, github | builder |
| `review` | search, navigate, file, git, github | reviewer |

**Issues**:
1. Extract common phases into `PhaseRegistry`
2. Allow agents to compose phases from registry
3. Support phase overrides (custom prompt, custom validator)
4. Add phase dependency declarations (e.g., `implement` requires `plan`)

---

### Milestone 4: Task Envelope + Structured I/O

**Goal**: Replace unstructured string passing between phases/agents with typed task envelopes.

**Files to create/modify**:
- `pkg/orchestration/task.go` — task envelope types
- `pkg/agents/phased_executor.go` — accept/return task envelopes
- `pkg/agents/executor.go` — accept/return task envelopes

**Task envelope**:
```go
type TaskEnvelope struct {
    ID            string                 `json:"id"`
    Agent         string                 `json:"agent"`
    Goal          string                 `json:"goal"`
    Mode          string                 `json:"mode"`
    Context       TaskContext            `json:"context"`
    ToolsAllowed  []string               `json:"tools_allowed"`
    MaxSteps      int                    `json:"max_steps"`
    ReturnSchema  map[string]string      `json:"return_schema"`
    ParentTaskID  string                 `json:"parent_task_id,omitempty"`
}

type TaskContext struct {
    Files         []string               `json:"files,omitempty"`
    Symbols       []string               `json:"symbols,omitempty"`
    PriorResults  map[string]interface{} `json:"prior_results,omitempty"`
    Artifacts     []string               `json:"artifacts,omitempty"`
}

type TaskResult struct {
    TaskID        string                 `json:"task_id"`
    Agent         string                 `json:"agent"`
    Success       bool                   `json:"success"`
    Output        map[string]interface{} `json:"output"`
    Artifacts     []Artifact             `json:"artifacts,omitempty"`
    TokensUsed    int                    `json:"tokens_used"`
    Duration      time.Duration          `json:"duration"`
}
```

**Issues**:
1. Define `TaskEnvelope` and `TaskResult` types
2. Refactor `PhasedExecutor.Execute()` to accept `TaskEnvelope`
3. Refactor `PhaseResult` to produce `TaskResult`
4. Update phased agents to use task envelopes
5. Add task serialization for file-based context

---

### Milestone 5: Subagent Manager

**Goal**: Enable parent agents to spawn child agents with isolated context and bounded execution.

**Files to create/modify**:
- `pkg/orchestration/subagent.go` — subagent manager
- `pkg/orchestration/subagent_context.go` — context inheritance
- `pkg/agents/base.go` — add subagent spawning capability
- `pkg/llmcontext/manager.go` — support child context directories

**Key design decisions**:
- Subagents get their own context directory: `/tmp/pedrocli-jobs/<parent-id>/subagents/<child-id>/`
- Subagents inherit a **subset** of parent context (specified files, not full history)
- Subagents return `TaskResult`, not chat
- Parent agent aggregates child results

**Subagent types** (initial):

| Subagent | Role | Tools | Max Rounds |
|----------|------|-------|------------|
| `explorer` | Search and map codebase | search, navigate, file, lsp | 10 |
| `implementer` | Write code changes | file, code_edit, bash | 20 |
| `tester` | Run and fix tests | test, bash, file, code_edit | 15 |
| `reviewer` | Validate changes | search, file, git, test | 10 |
| `doc-writer` | Generate documentation | file, search, navigate | 10 |

**Where subagents make sense**:
- **pedrocode (CLI)**: Sequential subagent execution (explore → implement → test)
- **HTTP server (web UI)**: Parallel subagent execution via goroutines with SSE progress updates
- **Blog orchestrator**: Research subagent + writing subagent + editing subagent

**Issues**:
1. Define `SubagentManager` interface
2. Implement context inheritance (parent → child)
3. Implement result aggregation (child → parent)
4. Add subagent progress tracking to job manager
5. Wire into `BuilderPhasedAgent` as first consumer
6. Add subagent support to HTTP bridge for parallel execution

---

### Milestone 6: Artifact / Blackboard System

**Goal**: Structured shared workspace for agent artifacts, replacing ad-hoc string passing.

**Files to create/modify**:
- `pkg/orchestration/artifacts.go` — artifact store
- `pkg/llmcontext/manager.go` — integrate artifact storage
- `pkg/agents/phased_executor.go` — read/write artifacts between phases

**Artifact types**:

| Artifact | Format | Produced By | Consumed By |
|----------|--------|-------------|-------------|
| `repo_map.json` | JSON | explorer | planner, implementer |
| `task.json` | JSON | query engine | all agents |
| `plan.md` | Markdown | planner | implementer |
| `diff.patch` | Unified diff | implementer | reviewer, tester |
| `test_results.json` | JSON | tester | reviewer, implementer |
| `review.md` | Markdown | reviewer | implementer (for fixes) |

**Storage**: File-based under job directory (extends existing `llmcontext.Manager` pattern):
```
/tmp/pedrocli-jobs/<job-id>/
├── artifacts/
│   ├── repo_map.json
│   ├── task.json
│   ├── plan.md
│   └── test_results.json
├── 001-prompt.txt
├── 002-response.txt
└── ...
```

**Issues**:
1. Define `ArtifactStore` interface
2. Implement file-based artifact storage
3. Add artifact read/write to phase transitions
4. Create `repo_map` generation from explorer subagent
5. Integrate with existing `llmcontext.Manager`

---

### Milestone 7: Permission Engine

**Goal**: Granular per-agent, per-tool, per-path permission enforcement.

**Files to create/modify**:
- `pkg/orchestration/permissions.go` — permission engine
- `pkg/orchestration/permissions_config.go` — config loading
- `pkg/agents/executor.go` — check permissions before tool execution
- `.pedrocli.json` — permission configuration

**Permission model**:
```json
{
  "permissions": {
    "defaults": {
      "read": "allow",
      "write": "ask",
      "bash": "ask",
      "network": "deny",
      "git_push": "ask"
    },
    "agents": {
      "explorer": {
        "read": "allow",
        "write": "deny"
      },
      "implementer": {
        "read": "allow",
        "write": "allow",
        "bash": "allow"
      }
    },
    "paths": {
      "deny": [".env", "*.key", "*.pem", "credentials.*"],
      "ask": ["go.mod", "go.sum", "Makefile"]
    }
  }
}
```

**Integration**:
- Extends existing `middleware.PolicyEvaluator`
- GUI prompts for `"ask"` permissions (already supported in REPL)
- HTTP bridge sends permission requests via SSE

**Issues**:
1. Define `PermissionEngine` interface
2. Implement permission resolution (agent → tool → path)
3. Add permission config to `.pedrocli.json`
4. Wire into `InferenceExecutor.executeTool()`
5. Add GUI approval flow for `"ask"` permissions
6. Add permission audit logging

---

### Milestone 8: Prompt Architecture (Layered Prompts)

**Goal**: Replace monolithic system prompts with composable layers.

**Files to create/modify**:
- `pkg/orchestration/prompt_layers.go` — prompt composition
- `pkg/agents/prompts/` — restructure prompt files
- `pkg/agents/base.go` — use layered prompt builder

**Layers**:

| Layer | Source | Purpose |
|-------|--------|---------|
| 1. Identity | `prompts/identity.md` | Pedro personality, global rules |
| 2. Mode | `prompts/mode_{name}.md` | What is allowed in this mode |
| 3. Phase | `prompts/phase_{name}.md` | What phase is active, goals |
| 4. Task | Task envelope JSON | What to do specifically |
| 5. Skills | `.pedrocli.json` + repo context | Repo conventions, tech stack |
| 6. Output contract | Phase/task return schema | Expected output format |

**Issues**:
1. Create `PromptBuilder` with layer composition
2. Extract identity prompt from existing system prompts
3. Create mode-specific prompt templates
4. Wire `PromptBuilder` into `InferenceExecutor`
5. Ensure backward compatibility with existing `SystemPrompt` field

---

### Milestone 9: Telemetry + Cost Tracking

**Goal**: Per-interaction logging of tokens, latency, tool usage, and failures.

**Files to create/modify**:
- `pkg/orchestration/telemetry.go` — telemetry collector
- `pkg/agents/executor.go` — emit telemetry events
- `pkg/agents/phased_executor.go` — emit phase telemetry
- `pkg/httpbridge/handlers.go` — expose telemetry API

**Metrics collected**:

| Metric | Granularity | Storage |
|--------|-------------|---------|
| Tokens (prompt + completion) | Per inference round | Job file |
| Latency (LLM call) | Per inference round | Job file |
| Tool calls (count, success/fail) | Per round | Job file |
| Phase duration | Per phase | Job file |
| Total job cost estimate | Per job | Job file + summary |
| Retries and failures | Per tool call | Job file |

**Issues**:
1. Define `TelemetryCollector` interface
2. Implement file-based telemetry storage (extends job directory)
3. Add telemetry hooks to `InferenceExecutor`
4. Add telemetry summary to job completion
5. Add `/api/telemetry/:job_id` endpoint to HTTP server
6. Display telemetry in web UI job details

---

## Milestone Dependency Graph

```
M1: Query Engine
 └── M2: Modes (depends on M1 for intent routing)
      └── M8: Prompt Layers (depends on M2 for mode prompts)

M3: Phase Registry (independent)
 └── M4: Task Envelope (depends on M3 for phase I/O)
      └── M5: Subagents (depends on M4 for task contracts)
           └── M6: Artifacts (depends on M5 for subagent outputs)

M7: Permissions (independent, can start anytime after M1)

M9: Telemetry (independent, can start anytime)
```

**Recommended execution order**:
1. M3 (Phase Registry) — low risk, high reuse
2. M1 (Query Engine) — enables everything else
3. M4 (Task Envelope) — structured I/O for all agents
4. M2 (Modes) — constrains execution
5. M7 (Permissions) — safety layer
6. M5 (Subagents) — biggest feature
7. M6 (Artifacts) — enables subagent coordination
8. M8 (Prompt Layers) — polish
9. M9 (Telemetry) — observability

---

## Where Each Feature Applies

| Feature | pedrocli (CLI) | pedrocode (REPL) | HTTP Server (Web UI) |
|---------|---------------|-----------------|---------------------|
| Query Engine | N/A (direct commands) | Yes (intent routing) | Yes (API dispatch) |
| Modes | N/A (implicit per command) | Yes (switchable) | Yes (per-job) |
| Phase Registry | Yes (all agents) | Yes (all agents) | Yes (all agents) |
| Task Envelope | Yes | Yes | Yes |
| Subagents | Sequential only | Sequential | Parallel (goroutines + SSE) |
| Artifacts | File-based | File-based | File-based + API |
| Permissions | Config-only | Interactive (ask) | Interactive (SSE approval) |
| Prompt Layers | Yes | Yes | Yes |
| Telemetry | File-based | File-based + display | File-based + API + UI |

---

## References

- [ccunpacked.dev](https://ccunpacked.dev) — Agent loop and phased execution patterns
- [OpenCode](https://github.com/opencode-ai/opencode) — UX patterns for terminal-based AI coding
- ADR-005: Agent Workflow Refactoring — existing phased agent design
- ADR-009: Evaluation System Architecture — testing and validation patterns
- ADR-011: Middleware Tool Filtering — existing policy evaluation
- `pkg/agents/phased_executor.go` — existing phase execution engine
- `pkg/agents/executor.go` — existing inference loop
- `pkg/tools/registry.go` — existing tool registry with filtering
