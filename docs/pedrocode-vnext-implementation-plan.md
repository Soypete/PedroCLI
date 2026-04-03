# PedroCode vNext — Implementation Plan

**Project**: cmd/pedrocode + pkg/agents  
**Owner**: Miriah Peterson  
**Status**: Proposed  
**Date**: 2026-04-01  
**Related ADRs**: ADR-005 (Agent Workflow Refactoring), ADR-009 (Evaluation System), ADR-011 (Middleware Tool Filtering), ADR-012 (Kairos Memory Consolidation), ADR-013 (vNext Orchestration Architecture)

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

**Status**: ✅ COMPLETED (2026-04)

**What was built**:
- `pkg/artifacts/store.go` — `ArtifactStore` interface + `InMemoryArtifactStore` implementation
- `pkg/artifacts/types.go` — `Artifact`, `ArtifactType`, `ArtifactFilter` types
- `pkg/agents/phased_executor.go` — Added `SetArtifactStore()`, `PutArtifact()`, `GetArtifactByName()`, `storePhaseArtifact()` methods
- Wired artifact store into: `BuilderPhasedAgent`, `DebuggerPhasedAgent`, `ReviewerPhasedAgent`, REPL, HTTP bridge

**Key decisions**:
- Used existing `pkg/artifacts/` package (not `pkg/orchestration/artifacts`)
- In-memory store per job (not persisted to disk)
- Automatic phase artifact storage: each phase output is stored as artifact after completion
- Artifact types match phase names for easy lookup

**Files modified**:
- `pkg/agents/builder_phased.go` - Creates artifact store, calls SetArtifactStore
- `pkg/agents/debugger_phased.go` - Creates artifact store, calls SetArtifactStore  
- `pkg/agents/reviewer_phased.go` - Creates artifact store, calls SetArtifactStore
- `pkg/agents/phased_executor.go` - Added artifact store integration methods
- `pkg/repl/interactive_sync.go` - Creates artifact store for REPL execution
- `docs/pedro-vnext/README.md` - Updated M6 status to Completed
- `docs/pedro-vnext/milestones/M6-artifact-store/README.md` - Updated with integration details

**Original plan (not used)**:
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

### Milestone 10: Kairos Memory Consolidation (ADR-012)

**Goal**: Post-session memory consolidation and resume for session continuity.

**Files to create/modify**:
- `pkg/memory/store.go` — `MemoryStore` interface
- `pkg/memory/file_store.go` — file-based implementation (`.pedro/` directory)
- `pkg/memory/types.go` — `SessionRecord`, `MemoryFact`, `OpenTask`, `ResumePacket`
- `pkg/memory/consolidator.go` — LLM-based consolidation pipeline
- `pkg/memory/dreamer.go` — `DreamerWorker` post-session maintenance
- `pkg/memory/resume.go` — resume loader + validator
- `pkg/repl/session.go` — load resume on start, run dreamer on close

**Issues**:
1. Define memory types and `MemoryStore` interface
2. Implement `FileMemoryStore` with `.pedro/sessions/`, `.pedro/memory/`, `.pedro/resume/`
3. Implement `Consolidator` — LLM-based artifact → facts + summary extraction
4. Implement `DreamerWorker` — runs consolidator after session ends (read-only tools only)
5. Implement resume loader — loads `ResumePacket`, validates against repo state (branch exists, files present)
6. Wire into REPL session lifecycle — load on start, display handoff, consolidate on close
7. Add fact TTL and staleness revalidation

**Guardrails (from ADR-012)**:
- Dreamer worker **never** modifies code
- Memory is a hint, not source of truth — revalidate before use
- Every durable fact must reference an evidence artifact
- Facts have TTL (repo facts: weekly, branch facts: per-session)

See [ADR-012](adr/ADR-012-kairos-memory-consolidation.md) for full design.

---

## Go Interface Definitions

All new interfaces for the orchestration layer. These build on existing types from `pkg/agents/`, `pkg/tools/`, and `pkg/llm/`.

### QueryEngine (`pkg/orchestration/query.go`)

```go
package orchestration

import (
    "context"
    "time"
)

// QueryEngine is the top-level orchestrator for all user requests.
type QueryEngine interface {
    Execute(ctx context.Context, query *Query) (*QueryResult, error)
    SetMode(mode Mode)
    GetMode() Mode
}

type Query struct {
    ID          string              `json:"id"`
    Raw         string              `json:"raw"`
    Intent      QueryIntent         `json:"intent"`
    Mode        Mode                `json:"mode"`
    Context     *QueryContext       `json:"context"`
    Constraints *ExecutionConstraints `json:"constraints"`
}

type QueryIntent string

const (
    IntentChat    QueryIntent = "chat"
    IntentExplore QueryIntent = "explore"
    IntentPlan    QueryIntent = "plan"
    IntentBuild   QueryIntent = "build"
    IntentDebug   QueryIntent = "debug"
    IntentReview  QueryIntent = "review"
    IntentTest    QueryIntent = "test"
)

type QueryContext struct {
    SessionID    string            `json:"session_id"`
    WorkDir      string            `json:"work_dir"`
    Branch       string            `json:"branch"`
    ResumePacket *ResumePacket     `json:"resume_packet,omitempty"` // From Kairos memory
    PriorResults []string          `json:"prior_results,omitempty"`
}

type ExecutionConstraints struct {
    MaxDuration    time.Duration `json:"max_duration"`
    MaxInference   int           `json:"max_inference"`
    MaxSubagents   int           `json:"max_subagents"`
    RequireApproval bool         `json:"require_approval"`
}

type QueryResult struct {
    ID        string              `json:"id"`
    Status    ResultStatus        `json:"status"`
    Summary   string              `json:"summary"`
    Artifacts []Artifact          `json:"artifacts"`
    Tasks     []TaskRecord        `json:"tasks,omitempty"`
    Metrics   *ExecutionMetrics   `json:"metrics,omitempty"`
}

type ResultStatus string

const (
    StatusSuccess  ResultStatus = "success"
    StatusPartial  ResultStatus = "partial"
    StatusFailed   ResultStatus = "failed"
    StatusCancelled ResultStatus = "cancelled"
)
```

### Mode (`pkg/orchestration/modes.go`)

```go
type Mode string

const (
    ModeChat   Mode = "chat"
    ModePlan   Mode = "plan"
    ModeBuild  Mode = "build"
    ModeReview Mode = "review"
)

type ModeConfig struct {
    Mode             Mode     `json:"mode"`
    AllowedPhases    []string `json:"allowed_phases"`
    AllowedTools     []string `json:"allowed_tools"`
    ForbiddenTools   []string `json:"forbidden_tools"`
    RequiresApproval bool     `json:"requires_approval"`
    MaxInferenceRuns int      `json:"max_inference_runs"`
}
```

### PhaseEngine (`pkg/orchestration/phases.go`)

```go
// PhaseEngine manages phase lifecycle, transitions, and enforcement.
// Wraps the existing PhasedExecutor with planning and conditional transitions.
type PhaseEngine interface {
    Plan(ctx context.Context, query *Query) (*ExecutionPlan, error)
    Execute(ctx context.Context, plan *ExecutionPlan) (*PlanResult, error)
    RegisterPhaseTemplate(template PhaseTemplate)
}

type ExecutionPlan struct {
    ID      string         `json:"id"`
    QueryID string         `json:"query_id"`
    Phases  []PlannedPhase `json:"phases"`
    Mode    Mode           `json:"mode"`
}

// PlannedPhase extends existing agents.Phase with orchestration metadata.
type PlannedPhase struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    SystemPrompt string   `json:"system_prompt"`
    Tools        []string `json:"tools"`
    MaxRounds    int      `json:"max_rounds"`
    AgentType    string   `json:"agent_type"`  // Which subagent runs this
    DependsOn    []string `json:"depends_on"`  // Phase dependencies
    OnSuccess    string   `json:"on_success"`  // Next phase on success
    OnFailure    string   `json:"on_failure"`  // Next phase on failure (or "abort")
    Artifacts    []string `json:"artifacts"`   // Expected output artifact types
}

type PhaseTemplate struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    DefaultTools []string `json:"default_tools"`
    MaxRounds    int      `json:"max_rounds"`
    Category     string   `json:"category"` // "analysis", "execution", "validation"
}

type PlanResult struct {
    PlanID      string                    `json:"plan_id"`
    Success     bool                      `json:"success"`
    PhaseResults map[string]*PhaseResult  `json:"phase_results"`
    Artifacts   []Artifact                `json:"artifacts"`
}
```

### SubagentManager (`pkg/orchestration/subagents.go`)

```go
// SubagentManager spawns bounded workers with isolated contexts.
type SubagentManager interface {
    Spawn(ctx context.Context, task *TaskEnvelope) (string, error)
    Wait(ctx context.Context, agentID string) (*SubagentResult, error)
    SpawnAndWait(ctx context.Context, task *TaskEnvelope) (*SubagentResult, error)
    List(ctx context.Context) ([]*SubagentStatus, error)
    Cancel(ctx context.Context, agentID string) error
}

type TaskEnvelope struct {
    ID           string                 `json:"id"`
    AgentType    string                 `json:"agent_type"`
    Goal         string                 `json:"goal"`
    Context      map[string]interface{} `json:"context"`
    ToolsAllowed []string               `json:"tools_allowed"`
    MaxSteps     int                    `json:"max_steps"`
    ReturnSchema map[string]string      `json:"return_schema"`
    ParentID     string                 `json:"parent_id"`
}

type SubagentResult struct {
    AgentID    string                 `json:"agent_id"`
    TaskID     string                 `json:"task_id"`
    Status     ResultStatus           `json:"status"`
    Output     map[string]interface{} `json:"output"`
    Artifacts  []Artifact             `json:"artifacts"`
    TokensUsed int                    `json:"tokens_used"`
    RoundsUsed int                    `json:"rounds_used"`
    Duration   time.Duration          `json:"duration"`
}

type SubagentStatus struct {
    AgentID     string       `json:"agent_id"`
    AgentType   string       `json:"agent_type"`
    Status      string       `json:"status"` // "running", "completed", "failed"
    CurrentStep int          `json:"current_step"`
    MaxSteps    int          `json:"max_steps"`
}
```

### ArtifactStore (`pkg/artifacts/store.go`)

```go
package artifacts

// ArtifactStore manages structured artifacts produced by agents.
type ArtifactStore interface {
    Save(ctx context.Context, artifact *Artifact) error
    Get(ctx context.Context, id string) (*Artifact, error)
    List(ctx context.Context, filter ArtifactFilter) ([]*Artifact, error)
    GetByType(ctx context.Context, queryID string, artifactType ArtifactType) ([]*Artifact, error)
}

type Artifact struct {
    ID        string                 `json:"id"`
    QueryID   string                 `json:"query_id"`
    Type      ArtifactType           `json:"type"`
    Name      string                 `json:"name"`
    Data      map[string]interface{} `json:"data"`
    CreatedBy string                 `json:"created_by"`
    CreatedAt time.Time              `json:"created_at"`
    Version   int                    `json:"version"`
}

type ArtifactType string

const (
    ArtifactRepoMap     ArtifactType = "repo_map"
    ArtifactPlan        ArtifactType = "plan"
    ArtifactDiff        ArtifactType = "diff"
    ArtifactTestResults ArtifactType = "test_results"
    ArtifactReview      ArtifactType = "review"
    ArtifactTaskList    ArtifactType = "task_list"
    ArtifactSummary     ArtifactType = "summary"
)

type ArtifactFilter struct {
    QueryID string       `json:"query_id,omitempty"`
    Type    ArtifactType `json:"type,omitempty"`
    AgentID string       `json:"agent_id,omitempty"`
}
```

### PermissionEngine (`pkg/permissions/engine.go`)

```go
package permissions

// PermissionEngine enforces tool and action permissions.
// Wraps existing middleware.PolicyEvaluator with config-driven policies.
type PermissionEngine interface {
    Check(ctx context.Context, req *PermissionRequest) (PermissionDecision, error)
    LoadPolicy(policy *PermissionPolicy) error
}

type PermissionPolicy struct {
    Scope    string                      `json:"scope"`    // "agent", "phase", "global"
    ScopeID  string                      `json:"scope_id"`
    Rules    map[string]PermissionAction `json:"rules"`
    Paths    map[string]PermissionAction `json:"paths"`
    Commands map[string]PermissionAction `json:"commands"`
}

type PermissionAction string

const (
    PermAllow PermissionAction = "allow"
    PermDeny  PermissionAction = "deny"
    PermAsk   PermissionAction = "ask"
)

type PermissionRequest struct {
    Agent  string `json:"agent"`
    Phase  string `json:"phase"`
    Tool   string `json:"tool"`
    Action string `json:"action"` // "read", "write", "execute", "network"
    Target string `json:"target"` // File path, command, URL
}

type PermissionDecision struct {
    Allowed       bool   `json:"allowed"`
    Reason        string `json:"reason"`
    NeedsApproval bool   `json:"needs_approval"`
}
```

### ToolRouter (`pkg/tools/router.go`)

```go
// ToolRouter wraps ToolRegistry with permission-aware dispatch.
// Extends existing ToolRegistry.Get() + Tool.Execute() with permission checks.
type ToolRouter interface {
    Dispatch(ctx context.Context, call *llm.ToolCall, scope PermissionScope) (*Result, error)
    AvailableTools(scope PermissionScope) []Tool
}

type PermissionScope struct {
    AgentType string
    Phase     string
    Mode      string
}
```

### MetricsCollector (`pkg/telemetry/tracker.go`)

```go
package telemetry

// MetricsCollector aggregates execution metrics.
// Unifies existing InferenceResponse.TokensUsed, PhaseResult.RoundsUsed,
// and storage.CompactionStatsStore into a single tracking system.
type MetricsCollector interface {
    Record(ctx context.Context, metrics *ExecutionMetrics) error
    GetSession(ctx context.Context, sessionID string) ([]*ExecutionMetrics, error)
    GetSummary(ctx context.Context, sessionID string) (*SessionMetricsSummary, error)
}

type ExecutionMetrics struct {
    QueryID       string        `json:"query_id"`
    TotalTokens   int           `json:"total_tokens"`
    PromptTokens  int           `json:"prompt_tokens"`
    OutputTokens  int           `json:"output_tokens"`
    TotalLatency  time.Duration `json:"total_latency"`
    ToolCalls     int           `json:"tool_calls"`
    ToolErrors    int           `json:"tool_errors"`
    Retries       int           `json:"retries"`
    PhasesRun     int           `json:"phases_run"`
    SubagentsUsed int           `json:"subagents_used"`
    InferenceRuns int           `json:"inference_runs"`
}

type SessionMetricsSummary struct {
    SessionID     string        `json:"session_id"`
    TotalQueries  int           `json:"total_queries"`
    TotalTokens   int           `json:"total_tokens"`
    TotalLatency  time.Duration `json:"total_latency"`
    TotalToolCalls int          `json:"total_tool_calls"`
}
```

### MemoryStore (`pkg/memory/store.go`)

```go
package memory

// MemoryStore handles persistence of session memory and facts.
// See ADR-012 for full design.
type MemoryStore interface {
    SaveSession(ctx context.Context, session SessionRecord) error
    SaveFacts(ctx context.Context, facts []MemoryFact) error
    SaveOpenTasks(ctx context.Context, tasks []OpenTask) error
    SaveResumePacket(ctx context.Context, packet ResumePacket) error

    LoadLatestResumePacket(ctx context.Context, repoID string) (*ResumePacket, error)
    LoadRelevantFacts(ctx context.Context, repoID string, scope string) ([]MemoryFact, error)
    MarkStale(ctx context.Context, factIDs []string) error
}

// Consolidator processes raw session artifacts into structured memory.
type Consolidator interface {
    Consolidate(ctx context.Context, input ConsolidationInput) (*ConsolidationResult, error)
}

// DreamerWorker runs isolated post-session memory maintenance.
type DreamerWorker interface {
    Run(ctx context.Context, sessionID string) error
}
```

---

## New Package Layout

```
pkg/
├── agents/              # EXISTING — gains SubagentManager field in M5
├── artifacts/           # NEW (M6)
│   ├── store.go         # ArtifactStore interface
│   ├── file_store.go    # File-based implementation
│   ├── types.go         # Artifact, ArtifactType, ArtifactFilter
│   └── blackboard.go    # Convenience read/write by query + type
├── memory/              # NEW (M10 — ADR-012)
│   ├── store.go         # MemoryStore interface
│   ├── file_store.go    # .pedro/ directory storage
│   ├── types.go         # SessionRecord, MemoryFact, OpenTask, ResumePacket
│   ├── consolidator.go  # LLM-based consolidation pipeline
│   ├── dreamer.go       # Post-session maintenance worker
│   └── resume.go        # Resume loader + validator
├── orchestration/       # NEW (M1-M5)
│   ├── query.go         # QueryEngine interface + DefaultQueryEngine
│   ├── intent.go        # Intent classification (rule-based + LLM)
│   ├── modes.go         # Mode, ModeConfig
│   ├── phases.go        # PhaseEngine, ExecutionPlan, PlannedPhase
│   ├── phase_registry.go # Reusable PhaseTemplate catalog
│   ├── subagents.go     # SubagentManager interface + implementation
│   ├── subagent_explorer.go     # Explorer subagent type
│   ├── subagent_implementer.go  # Implementer subagent type
│   ├── subagent_tester.go       # Tester subagent type
│   ├── subagent_reviewer.go     # Reviewer subagent type
│   ├── task.go          # TaskEnvelope, TaskResult
│   └── prompt_layers.go # Layered prompt composition
├── permissions/         # NEW (M7)
│   ├── engine.go        # PermissionEngine interface
│   ├── policy.go        # PermissionPolicy types
│   ├── evaluator.go     # Default evaluator (wraps middleware)
│   └── config.go        # Load from .pedrocli.json
├── telemetry/           # NEW (M9)
│   ├── tracker.go       # MetricsCollector interface
│   ├── memory_store.go  # In-memory collector
│   └── types.go         # ExecutionMetrics, SessionMetricsSummary
└── tools/               # EXISTING
    └── router.go        # NEW — ToolRouter (permission-aware dispatch)
```

---

## Configuration Schema

New sections added to `.pedrocli.json`:

```json
{
  "orchestration": {
    "enabled": true,
    "default_mode": "build",
    "intent_classification": "rule_based",
    "max_subagents": 4,
    "max_plan_retries": 2
  },
  "permissions": {
    "default": {
      "read": "allow",
      "write": "ask",
      "bash": "ask",
      "network": "deny"
    },
    "agents": {
      "explorer": { "read": "allow", "write": "deny", "bash": "deny" },
      "implementer": { "read": "allow", "write": "allow", "bash": "ask" },
      "tester": { "read": "allow", "write": "deny", "bash": "allow" },
      "reviewer": { "read": "allow", "write": "deny", "bash": "deny" }
    },
    "paths": {
      "deny": [".env", "*.key", "*.pem", "credentials.*"],
      "ask": ["go.mod", "go.sum", "Makefile"]
    }
  },
  "memory": {
    "enabled": false,
    "storage_dir": ".pedro",
    "consolidate_on_exit": true,
    "fact_ttl_days": 7,
    "max_facts": 100
  },
  "telemetry": {
    "enabled": true,
    "store": "memory"
  },
  "phases": {
    "templates": {
      "explore":    { "tools": ["search", "navigate", "file", "git"],                       "max_rounds": 10 },
      "plan":       { "tools": ["search", "navigate", "file", "context"],                   "max_rounds": 5  },
      "implement":  { "tools": ["file", "code_edit", "search", "navigate", "bash", "git"],  "max_rounds": 30 },
      "validate":   { "tools": ["test", "bash", "file", "search"],                          "max_rounds": 15 },
      "summarize":  { "tools": [],                                                          "max_rounds": 3  }
    }
  }
}
```

---

## Existing Code Reuse Map

| New Component | Builds On | How |
|---------------|-----------|-----|
| `QueryEngine` | `cli.CLIBridge` (`pkg/cli/bridge.go`) | Wraps bridge as pass-through initially |
| `PhaseEngine` | `PhasedExecutor` (`pkg/agents/phased_executor.go`) | Translates `ExecutionPlan` into `[]Phase` |
| `SubagentManager` | `BaseAgent` + `InferenceExecutor` + `llmcontext.Manager` | Creates isolated agent instances per task |
| `ArtifactStore` | `llmcontext.Manager` file patterns (`pkg/llmcontext/`) | Same `/tmp/pedrocli-jobs/` directory structure |
| `PermissionEngine` | `middleware.PolicyEvaluator` + `repl.ApprovalPrompt` | Wraps existing middleware, adds config-driven policies |
| `ToolRouter` | `tools.ToolRegistry` (`pkg/tools/registry.go`) | Adds permission check before `registry.Get()` + `tool.Execute()` |
| `MetricsCollector` | `InferenceResponse.TokensUsed` + `ProgressEvent` | Aggregates existing per-round data |
| `MemoryStore` | `llmcontext.Manager` file patterns | New `.pedro/` directory, same JSON file approach |
| `ModeConfig` | `repl.Session.Mode` (`pkg/repl/session.go`) | Extends existing mode string with behavioral constraints |
| `IntentClassifier` | `BlogOrchestratorAgent.analyzePrompt()` | Same LLM-classification pattern, generalized |

---

## Milestone Dependency Graph

```
M1: Query Engine ─────────────────────────────┐
 ├── M2: Modes ───────────────────────────────┤
 │                                             │
M3: Phase Registry ──┐                         │
 └── M4: Task Envelope ──┐                     │
      └── M5: Subagents ─┤                     │
           └── M6: Artifacts                   │
                          └── M8: Prompt Layers │
                                                │
M7: Permissions ←── can start after M1 ────────┘
M9: Telemetry ←── can start anytime
M10: Kairos Memory ←── can start after M1
```

**Parallelizable work:**
- M2 and M3 can run in parallel after M1
- M7, M9, and M10 can each start independently after M1
- M6 and M8 can run in parallel after M5

**Recommended execution order**:
1. **M3** (Phase Registry) — low risk, high reuse, no dependencies
2. **M1** (Query Engine) — enables everything else
3. **M4** (Task Envelope) — structured I/O for all agents
4. **M2** (Modes) — constrains execution
5. **M7** (Permissions) — safety layer
6. **M5** (Subagents) — biggest feature
7. **M6** (Artifacts) — enables subagent coordination
8. **M8** (Prompt Layers) — polish
9. **M9** (Telemetry) — observability
10. **M10** (Kairos Memory) — session continuity

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
| Kairos Memory | N/A | Yes (resume + consolidation) | Yes (resume + consolidation) |

---

## Risk Register

| Risk | Mitigation |
|------|------------|
| Query engine adds latency | Pass-through mode for simple queries; intent classification is cheap |
| Subagent context isolation increases memory | Each subagent uses small context; cleanup after completion |
| Permission system blocks legitimate tool use | Default policies are permissive; `ask` mode for uncertain cases |
| LLM-based intent classification unreliable | Rule-based fallback; user can override with `/mode` |
| Memory consolidation produces bad facts | Evidence-required guardrail; TTL expiration; revalidation on resume |
| Too many new packages | Each milestone is independently useful; can stop at any milestone |
| Orchestration disabled breaks existing behavior | `orchestration.enabled: false` falls through to existing code path |

---

## Success Criteria

After all milestones:

1. User types query → system auto-classifies intent → correct phases execute → structured result
2. Explorer subagent maps repo without write access
3. Implementer subagent writes code in isolation, returns diff
4. Tester subagent validates changes, returns structured results
5. Reviewer subagent checks quality, can send work back to implementer
6. All tool access governed by permissions with GUI approval for `ask`
7. Token usage and latency tracked per query, visible via `/metrics`
8. Session produces resume packet; next session shows handoff summary
9. All existing CLI/HTTP/REPL behavior preserved when `orchestration.enabled: false`

---

## References

- [ccunpacked.dev](https://ccunpacked.dev) — Agent loop and phased execution patterns
- [OpenCode](https://github.com/opencode-ai/opencode) — UX patterns for terminal-based AI coding
- [ADR-005](adr/ADR-005-agent-workflow-refactoring.md) — Agent Workflow Refactoring
- [ADR-009](adr/ADR-009-evaluation-system-architecture.md) — Evaluation System Architecture
- [ADR-011](adr/ADR-011-middleware-tool-filtering-adoption.md) — Middleware Tool Filtering
- [ADR-012](adr/ADR-012-kairos-memory-consolidation.md) — Kairos Memory Consolidation
- [ADR-013](adr/ADR-013-pedrocode-vnext-orchestration.md) — vNext Orchestration Architecture
- `pkg/agents/phased_executor.go` — existing phase execution engine
- `pkg/agents/executor.go` — existing inference loop
- `pkg/tools/registry.go` — existing tool registry with filtering
- `pkg/repl/session.go` — existing session lifecycle
- `pkg/repl/approval.go` — existing GUI approval flow
