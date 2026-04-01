# ADR-013: PedroCode vNext — Agent Orchestration Architecture

**Status:** Proposed  
**Owner:** Miriah Peterson  
**Date:** 2026-04-01

---

## TL;DR

PedroCode will evolve from a GUI chat interface with tools into a **phased, multi-agent orchestration system** with structured execution, permissions, and artifact-driven workflows.

We will:
- keep the existing GUI UX (cmd/pedrocode REPL + HTTP web UI)
- introduce a **Query Engine** + **Phase Engine**
- implement **subagent orchestration**
- move from chat → task + artifact execution
- add **modes**, **permissions**, and **telemetry**

---

## Goals

### Primary Goal

Make Pedro: **reliable, debuggable, and controllable — not just "smart"**

### Non-Goals
- Not cloning Claude Code
- Not replacing GUI with CLI
- Not building a research agent framework

---

## Current System Assessment

### What Works
- GUI-driven interaction via `pkg/repl/` REPL and `pkg/httpbridge/` web UI
- Tool system with 25+ tools, `ToolRegistry`, `ExtendedTool` metadata (`pkg/tools/`)
- `InferenceExecutor` single-pass loop (`pkg/agents/executor.go`)
- `PhasedExecutor` multi-phase workflows (`pkg/agents/phased_executor.go`)
- Phased agents: `BuilderPhasedAgent`, `DebuggerPhasedAgent`, `ReviewerPhasedAgent`
- `BlogOrchestratorAgent` multi-step orchestration
- File-based context management (`pkg/llmcontext/`)
- Job manager with persistence (`pkg/jobs/`)
- Middleware policy evaluation (`pedro-agentware/middleware`)
- Interactive approval flow (`pkg/repl/approval.go`)

### What Breaks
- No orchestration layer between GUI and agents
- Single-agent overload — one agent handles everything per session
- Context pollution — no structured handoff between phases
- No task/state model — agents don't produce structured artifacts
- No loop control — phases are linear, no conditional branching
- No subagent spawning — `PhasedExecutor` runs phases sequentially in one context

---

## Key Insight

> The model is not the system. The system is what makes the model usable.

---

## Target Architecture

```
GUI (cmd/pedrocode REPL + HTTP web UI)
        ↓
Session Controller (pkg/repl/session.go — enhanced)
        ↓
Query Engine (NEW — pkg/orchestration/query.go)
        ↓
Phase Engine (NEW — pkg/orchestration/phases.go)
        ↓
Subagent Manager (NEW — pkg/orchestration/subagents.go)
        ↓
--------------------------------------------------
| Agents (plan / build / explore / test / review) |
| Tool Router (pkg/tools/registry.go — enhanced)  |
| Permission Engine (NEW — pkg/permissions/)       |
--------------------------------------------------
        ↓
Artifacts + Task Store (NEW — pkg/artifacts/)
        ↓
Providers (pkg/llm/ — unchanged)
```

---

## Core Concepts

### 1. Query Engine (Orchestrator)

**File:** `pkg/orchestration/query.go`

The entry point for all requests. Replaces the current direct path of `REPL → CLIBridge → Agent`.

```go
// QueryEngine is the top-level orchestrator for all user requests.
// It decides execution strategy, selects mode + phases, and coordinates subagents.
type QueryEngine interface {
    // Execute processes a user query through the full orchestration pipeline.
    Execute(ctx context.Context, query *Query) (*QueryResult, error)

    // SetMode sets the current execution mode (chat, plan, build, review).
    SetMode(mode Mode)

    // GetMode returns the current execution mode.
    GetMode() Mode
}

// Query represents a parsed user request.
type Query struct {
    ID          string                 `json:"id"`
    Raw         string                 `json:"raw"`           // Original user input
    Intent      QueryIntent            `json:"intent"`        // Classified intent
    Mode        Mode                   `json:"mode"`          // Execution mode
    Context     *QueryContext          `json:"context"`       // Session + repo context
    Constraints *ExecutionConstraints  `json:"constraints"`   // Limits, permissions
}

// QueryIntent classifies what the user wants.
type QueryIntent string

const (
    IntentChat     QueryIntent = "chat"      // Conversational, no tools
    IntentExplore  QueryIntent = "explore"   // Read-only codebase exploration
    IntentPlan     QueryIntent = "plan"      // Create implementation plan
    IntentBuild    QueryIntent = "build"     // Write code
    IntentDebug    QueryIntent = "debug"     // Fix issues
    IntentReview   QueryIntent = "review"    // Code review
    IntentTest     QueryIntent = "test"      // Run and analyze tests
)

// QueryResult is the structured output of query execution.
type QueryResult struct {
    ID        string                 `json:"id"`
    Status    ResultStatus           `json:"status"`
    Summary   string                 `json:"summary"`
    Artifacts []Artifact             `json:"artifacts"`
    Tasks     []TaskRecord           `json:"tasks,omitempty"`
    Metrics   *ExecutionMetrics      `json:"metrics,omitempty"`
}
```

**Why:** Right now the path is `GUI → model → tools`. After: `GUI → system → agents → tools`. The system owns execution strategy, not the model.

**Integration point:** `pkg/repl/session.go` currently holds `Bridge *cli.CLIBridge`. The `QueryEngine` wraps the bridge and sits between the REPL and agent execution.

---

### 2. Phase Engine

**File:** `pkg/orchestration/phases.go`

Extends the existing `PhasedExecutor` with dynamic phase generation, conditional transitions, and reusable phase definitions.

```go
// PhaseEngine manages phase lifecycle, transitions, and enforcement.
type PhaseEngine interface {
    // Plan generates an execution plan (ordered phases) for a query.
    Plan(ctx context.Context, query *Query) (*ExecutionPlan, error)

    // Execute runs an execution plan through its phases.
    Execute(ctx context.Context, plan *ExecutionPlan) (*PlanResult, error)

    // RegisterPhaseTemplate registers a reusable phase template.
    RegisterPhaseTemplate(template PhaseTemplate)
}

// ExecutionPlan is an ordered set of phases with transition rules.
type ExecutionPlan struct {
    ID          string          `json:"id"`
    QueryID     string          `json:"query_id"`
    Phases      []PlannedPhase  `json:"phases"`
    Mode        Mode            `json:"mode"`
    CreatedAt   time.Time       `json:"created_at"`
}

// PlannedPhase extends the existing Phase struct with transition rules.
type PlannedPhase struct {
    agents.Phase                          // Embeds existing Phase struct
    AgentType    string                   `json:"agent_type"`     // Which subagent runs this
    DependsOn    []string                 `json:"depends_on"`     // Phase dependencies
    OnSuccess    string                   `json:"on_success"`     // Next phase on success
    OnFailure    string                   `json:"on_failure"`     // Next phase on failure (or "abort")
    Artifacts    []string                 `json:"artifacts"`      // Expected output artifacts
}

// PhaseTemplate is a reusable phase definition.
type PhaseTemplate struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    DefaultTools []string `json:"default_tools"`
    MaxRounds    int      `json:"max_rounds"`
    Category     string   `json:"category"` // "analysis", "execution", "validation"
}
```

**Existing foundation:** `pkg/agents/phased_executor.go` already has `Phase`, `PhaseResult`, `PhaseCallback`, and `PhasedExecutor`. The `PhaseEngine` wraps this with planning and conditional transitions.

**Phase rules:**
- `plan` → no writes allowed
- `explore` → read + search only
- `implement` → write allowed
- `validate` → test + review only

---

### 3. Modes

**File:** `pkg/orchestration/modes.go`

Modes define top-level behavior constraints. They prevent the model from guessing intent.

```go
// Mode defines what kind of work is allowed in a session.
type Mode string

const (
    ModeChat   Mode = "chat"    // Safe, read-only, conversational
    ModePlan   Mode = "plan"    // Analysis only, no writes
    ModeBuild  Mode = "build"   // Full execution allowed
    ModeReview Mode = "review"  // Validation focus, read + comment
)

// ModeConfig defines the constraints for a mode.
type ModeConfig struct {
    Mode            Mode     `json:"mode"`
    AllowedPhases   []string `json:"allowed_phases"`
    AllowedTools    []string `json:"allowed_tools"`
    ForbiddenTools  []string `json:"forbidden_tools"`
    RequiresApproval bool   `json:"requires_approval"` // For write operations
    MaxInferenceRuns int    `json:"max_inference_runs"`
}
```

**Integration point:** `pkg/repl/session.go` already has a `Mode string` field (currently "code"/"blog"/"podcast"). This extends it with behavioral constraints.

---

### 4. Subagent System

**File:** `pkg/orchestration/subagents.go`

Subagents are **bounded workers, not conversational clones**. Each gets a task envelope, a tool subset, and returns structured output.

```go
// SubagentManager spawns, tracks, and collects results from subagents.
type SubagentManager interface {
    // Spawn creates and starts a subagent with a task envelope.
    Spawn(ctx context.Context, task *TaskEnvelope) (string, error) // returns agent ID

    // Wait blocks until a subagent completes and returns its result.
    Wait(ctx context.Context, agentID string) (*SubagentResult, error)

    // SpawnAndWait is a convenience method for synchronous execution.
    SpawnAndWait(ctx context.Context, task *TaskEnvelope) (*SubagentResult, error)

    // List returns all active subagents.
    List(ctx context.Context) ([]*SubagentStatus, error)

    // Cancel cancels a running subagent.
    Cancel(ctx context.Context, agentID string) error
}

// TaskEnvelope is the core execution unit for subagents.
type TaskEnvelope struct {
    ID            string                 `json:"id"`
    AgentType     string                 `json:"agent_type"`     // "explorer", "implementer", "tester", "reviewer"
    Goal          string                 `json:"goal"`
    Context       map[string]interface{} `json:"context"`        // Files, scope, prior results
    ToolsAllowed  []string               `json:"tools_allowed"`
    MaxSteps      int                    `json:"max_steps"`
    ReturnSchema  map[string]string      `json:"return_schema"`  // Expected output fields
    ParentID      string                 `json:"parent_id"`      // Parent task/query ID
}

// SubagentResult is the structured output from a subagent.
type SubagentResult struct {
    AgentID     string                 `json:"agent_id"`
    TaskID      string                 `json:"task_id"`
    Status      ResultStatus           `json:"status"`
    Output      map[string]interface{} `json:"output"`     // Matches ReturnSchema
    Artifacts   []Artifact             `json:"artifacts"`
    TokensUsed  int                    `json:"tokens_used"`
    RoundsUsed  int                    `json:"rounds_used"`
    Duration    time.Duration          `json:"duration"`
}
```

**Initial subagent types:**

| Type | Purpose | Tools | Returns |
|------|---------|-------|---------|
| `explorer` | Codebase exploration | search, navigate, file (read), git log | repo_map, relevant_files |
| `implementer` | Write code | file, code_edit, search, navigate, bash | diff, summary |
| `tester` | Run and analyze tests | test, bash, file (read), search | test_results, failures |
| `reviewer` | Validate changes | file (read), search, navigate, git diff | review, issues |
| `doc-writer` | Generate documentation | file, search, navigate | doc_content |

**Key rule:** Subagents return structured outputs — not chat.

**Implementation:** Each subagent gets its own `llmcontext.Manager` (isolated context directory), its own `InferenceExecutor` with a filtered `ToolRegistry`, and a scoped system prompt. The `SubagentManager` uses the existing `jobs.JobManager` for lifecycle tracking.

---

### 5. Artifact / Blackboard System

**File:** `pkg/artifacts/store.go`

A shared structured workspace that replaces prompt-stuffing with typed data.

```go
// ArtifactStore manages structured artifacts produced by agents and subagents.
type ArtifactStore interface {
    // Save stores an artifact.
    Save(ctx context.Context, artifact *Artifact) error

    // Get retrieves an artifact by ID.
    Get(ctx context.Context, id string) (*Artifact, error)

    // List returns artifacts matching the filter.
    List(ctx context.Context, filter ArtifactFilter) ([]*Artifact, error)

    // GetByType returns all artifacts of a given type for a query.
    GetByType(ctx context.Context, queryID string, artifactType ArtifactType) ([]*Artifact, error)
}

// Artifact is a typed, versioned piece of structured data.
type Artifact struct {
    ID        string                 `json:"id"`
    QueryID   string                 `json:"query_id"`
    Type      ArtifactType           `json:"type"`
    Name      string                 `json:"name"`
    Data      map[string]interface{} `json:"data"`
    CreatedBy string                 `json:"created_by"`  // Agent/subagent ID
    CreatedAt time.Time              `json:"created_at"`
    Version   int                    `json:"version"`
}

// ArtifactType enumerates known artifact types.
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
```

**Why:**
- Reduces context size (agents read artifacts, not full transcripts)
- Improves determinism (structured data vs freeform prose)
- Enables evals (compare artifact outputs)
- Easier debugging (inspect artifacts directly)

**Existing foundation:** `pkg/llmcontext/manager.go` already writes structured files to `/tmp/pedrocli-jobs/<job-id>/`. The artifact store formalizes this pattern.

---

### 6. Planner → Executor → Reviewer Pattern

```
1. Planner decomposes task → produces plan artifact
2. Executors perform work  → produce diff/test artifacts
3. Reviewer validates      → produces review artifact
```

**Why:** Separates thinking, doing, and checking. The existing `BuilderPhasedAgent` already follows this pattern (analyze → plan → implement → validate → deliver). This formalizes it as a first-class orchestration pattern.

---

### 7. Permission Engine

**File:** `pkg/permissions/engine.go`

```go
// PermissionEngine enforces tool and action permissions.
type PermissionEngine interface {
    // Check returns whether an action is allowed, denied, or requires approval.
    Check(ctx context.Context, req *PermissionRequest) (PermissionDecision, error)

    // LoadPolicy loads a permission policy.
    LoadPolicy(policy *PermissionPolicy) error
}

// PermissionPolicy defines permissions for a scope.
type PermissionPolicy struct {
    Scope    string                       `json:"scope"`    // "agent", "phase", "global"
    ScopeID  string                       `json:"scope_id"` // Agent name, phase name, or "*"
    Rules    map[string]PermissionAction  `json:"rules"`    // tool_name → action
    Paths    map[string]PermissionAction  `json:"paths"`    // glob pattern → action
    Commands map[string]PermissionAction  `json:"commands"` // command pattern → action
}

// PermissionAction is what happens when a permission is checked.
type PermissionAction string

const (
    PermissionAllow PermissionAction = "allow"
    PermissionDeny  PermissionAction = "deny"
    PermissionAsk   PermissionAction = "ask"  // Prompt user via GUI
)

// PermissionRequest is a request to check a permission.
type PermissionRequest struct {
    Agent   string `json:"agent"`
    Phase   string `json:"phase"`
    Tool    string `json:"tool"`
    Action  string `json:"action"`  // "read", "write", "execute", "network"
    Target  string `json:"target"`  // File path, command, URL
}

type PermissionDecision struct {
    Allowed bool   `json:"allowed"`
    Reason  string `json:"reason"`
    NeedsApproval bool `json:"needs_approval"`
}
```

**Existing foundation:** `pkg/repl/approval.go` already has `ApprovalPrompt` and `ApprovalResponse`. `middleware.PolicyEvaluator` already validates tool calls in the `InferenceExecutor`. The permission engine unifies these into a single system.

**Example policy (JSON config):**
```json
{
  "scope": "agent",
  "scope_id": "explorer",
  "rules": {
    "file": "allow",
    "search": "allow",
    "navigate": "allow",
    "code_edit": "deny",
    "bash": "deny",
    "git": "deny"
  }
}
```

---

### 8. Tool Router

**File:** `pkg/tools/router.go` (extends existing `pkg/tools/registry.go`)

The system enforces what tools exist and what tools are allowed. The model chooses within constraints.

```go
// ToolRouter wraps ToolRegistry with permission-aware dispatch.
type ToolRouter interface {
    // Dispatch executes a tool call after permission checks.
    Dispatch(ctx context.Context, call *llm.ToolCall, scope PermissionScope) (*Result, error)

    // AvailableTools returns tools allowed in the current scope.
    AvailableTools(scope PermissionScope) []Tool
}

// PermissionScope identifies the current execution context for tool filtering.
type PermissionScope struct {
    AgentType string
    Phase     string
    Mode      string
}
```

**Existing foundation:** `ToolRegistry` already has `FilterByCategory()`, `FilterByOptionality()`, and `Clone()`. The `PhasedExecutor` already filters tools per phase via `Phase.Tools []string`. The router adds permission checks.

---

### 9. Telemetry + Cost Tracking

**File:** `pkg/telemetry/tracker.go`

```go
// ExecutionMetrics tracks resource usage for a query/task.
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

// MetricsCollector aggregates execution metrics.
type MetricsCollector interface {
    Record(ctx context.Context, metrics *ExecutionMetrics) error
    GetSession(ctx context.Context, sessionID string) ([]*ExecutionMetrics, error)
    GetSummary(ctx context.Context, sessionID string) (*SessionMetricsSummary, error)
}
```

**Existing foundation:** `InferenceResponse.TokensUsed`, `PhaseResult.RoundsUsed`, `storage.CompactionStatsStore` all track partial metrics. This unifies them.

---

### 10. Prompt Architecture

Replace single prompts with a layered system.

```
Layer 1 — System Identity    Pedro personality + global rules
Layer 2 — Mode Prompt        What is allowed (chat/plan/build/review)
Layer 3 — Phase Prompt        What phase is active (explore/implement/validate)
Layer 4 — Task Envelope       What to do (structured goal + context)
Layer 5 — Skills / Rules      Repo conventions, style guides
Layer 6 — Output Contract     Expected response schema
```

**Existing foundation:** `BaseAgent.buildSystemPrompt()` and `CodingBaseAgent` prompt generation already compose prompts from multiple sources. The `PhasedExecutor` already uses per-phase `SystemPrompt`. This formalizes the layering.

---

## Tradeoffs

| Pros | Cons |
|------|------|
| Deterministic execution | Increased complexity |
| Scalable system | Requires refactor |
| Modular architecture | Slower initial iteration |
| Safer tool usage | More configuration surface |
| Better debugging | Learning curve |

---

## Integration with Kairos Memory (ADR-012)

The phase engine's lifecycle naturally extends to include memory consolidation:

```
intake → plan → explore → implement → validate → summarize → consolidate
```

The consolidation phase runs via the Dreamer Worker (ADR-012) after the active session ends, producing `ResumePacket` artifacts that feed into the next session's `QueryContext`.

---

## Where This Applies

| Component | Query Engine | Phase Engine | Subagents | Artifacts | Permissions |
|-----------|:-----------:|:------------:|:---------:|:---------:|:-----------:|
| `cmd/pedrocode` (REPL) | Yes | Yes | Yes | Yes | Yes |
| `cmd/http-server` (Web UI) | Yes | Yes | Yes | Yes | Yes |
| `cmd/pedrocli` (CLI jobs) | Partial | Yes (already has phased agents) | Later | Yes | Partial |
| Blog agents | No | Already phased | No | Yes | No |
| Podcast agents | No | No | No | No | No |

---

## Related Documents

- [ADR-005: Agent Workflow Refactoring](ADR-005-agent-workflow-refactoring.md)
- [ADR-008: Phased Compaction Middleware](ADR-008-phased-compaction-middleware.md)
- [ADR-011: Middleware Tool Filtering](ADR-011-middleware-tool-filtering-adoption.md)
- [ADR-012: Kairos Memory Consolidation](ADR-012-kairos-memory-consolidation.md)
- [ccunpacked.dev agent loop](https://www.ccunpacked.dev) — Phase execution inspiration
- [OpenCode](https://github.com/opencode-ai/opencode) — UX patterns reference
