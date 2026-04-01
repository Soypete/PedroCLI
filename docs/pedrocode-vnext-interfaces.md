# PedroCode vNext — Interface Definitions

**Companion to**: `docs/pedrocode-vnext-implementation-plan.md`  
**Package**: `pkg/orchestration` (new)  
**Date**: 2026-04-01

This document defines the Go interfaces, structs, and configuration schemas for each milestone.

---

## Package Layout

```
pkg/orchestration/
├── query_engine.go        # M1: Intent routing and dispatch
├── mode.go                # M2: Execution modes
├── phase_registry.go      # M3: Reusable phase catalog
├── task.go                # M4: Task envelope types
├── subagent.go            # M5: Subagent manager
├── artifacts.go           # M6: Artifact/blackboard store
├── permissions.go         # M7: Permission engine
├── prompt_layers.go       # M8: Layered prompt builder
├── telemetry.go           # M9: Telemetry collector
└── orchestration_test.go  # Tests
```

---

## M1: Query Engine

```go
package orchestration

import (
    "context"

    "github.com/soypete/pedrocli/pkg/agents"
    "github.com/soypete/pedrocli/pkg/config"
    "github.com/soypete/pedrocli/pkg/jobs"
    "github.com/soypete/pedrocli/pkg/llm"
)

// Intent represents the classified intent of a user query.
type Intent string

const (
    IntentChat    Intent = "chat"
    IntentPlan    Intent = "plan"
    IntentBuild   Intent = "build"
    IntentDebug   Intent = "debug"
    IntentReview  Intent = "review"
    IntentTriage  Intent = "triage"
)

// QueryResult contains the result of processing a user query.
type QueryResult struct {
    Intent      Intent
    Agent       string            // selected agent name
    Mode        Mode              // resolved execution mode
    Task        *TaskEnvelope     // structured task for execution
    Job         *jobs.Job         // resulting job (after execution)
    Error       error
}

// QueryEngine routes user input to the appropriate agent and mode.
type QueryEngine interface {
    // Classify determines the intent of a user query without executing it.
    Classify(ctx context.Context, input string) (Intent, error)

    // Execute classifies the input and dispatches to the appropriate agent.
    Execute(ctx context.Context, input string) (*QueryResult, error)

    // ExecuteWithMode forces a specific mode regardless of classification.
    ExecuteWithMode(ctx context.Context, input string, mode Mode) (*QueryResult, error)
}

// DefaultQueryEngine implements QueryEngine using LLM-based classification
// with heuristic fallbacks.
type DefaultQueryEngine struct {
    config       *config.Config
    backend      llm.Backend
    jobManager   jobs.JobManager
    agentFactory AgentFactory
    modeEngine   *ModeEngine
    permissions  PermissionEngine
    telemetry    TelemetryCollector
}

// AgentFactory creates agents by name. This replaces the switch statements
// currently in httpbridge/handlers.go and cmd/pedrocode REPL.
type AgentFactory interface {
    // Create returns an agent by name, configured for the given mode.
    Create(name string, mode Mode) (agents.Agent, error)

    // Available returns agent names available in the given mode.
    Available(mode Mode) []string
}
```

---

## M2: Execution Modes

```go
package orchestration

// Mode defines an execution mode that constrains tools and behavior.
type Mode struct {
    Name          string   `json:"name" yaml:"name"`
    Description   string   `json:"description" yaml:"description"`
    AllowedTools  []string `json:"allowed_tools" yaml:"allowed_tools"`
    AllowWrite    bool     `json:"allow_write" yaml:"allow_write"`
    AllowBash     bool     `json:"allow_bash" yaml:"allow_bash"`
    AllowNetwork  bool     `json:"allow_network" yaml:"allow_network"`
    AllowGitPush  bool     `json:"allow_git_push" yaml:"allow_git_push"`
    AgentTypes    []string `json:"agent_types" yaml:"agent_types"`
    MaxRounds     int      `json:"max_rounds" yaml:"max_rounds"`
}

// ModeEngine manages mode selection and enforcement.
type ModeEngine struct {
    modes   map[string]Mode
    current string
}

// NewModeEngine creates a ModeEngine with default modes.
func NewModeEngine() *ModeEngine

// SetMode switches the active mode.
func (m *ModeEngine) SetMode(name string) error

// Current returns the active mode.
func (m *ModeEngine) Current() Mode

// IsToolAllowed checks if a tool is permitted in the current mode.
func (m *ModeEngine) IsToolAllowed(toolName string) bool

// DefaultModes returns the built-in mode definitions.
func DefaultModes() map[string]Mode
// Returns: chat, plan, build, review
```

---

## M3: Phase Registry

```go
package orchestration

import "github.com/soypete/pedrocli/pkg/agents"

// PhaseTemplate is a reusable phase definition that can be customized per agent.
type PhaseTemplate struct {
    Name          string   `json:"name" yaml:"name"`
    Description   string   `json:"description" yaml:"description"`
    DefaultPrompt string   `json:"default_prompt" yaml:"default_prompt"`  // path to prompt file
    Tools         []string `json:"tools" yaml:"tools"`
    MaxRounds     int      `json:"max_rounds" yaml:"max_rounds"`
    ExpectsJSON   bool     `json:"expects_json" yaml:"expects_json"`
    DependsOn     []string `json:"depends_on" yaml:"depends_on"`  // phase names that must complete first
}

// PhaseOverride allows an agent to customize a template.
type PhaseOverride struct {
    PromptOverride    string   // custom system prompt (replaces default)
    ToolsAppend       []string // additional tools beyond template
    ToolsRemove       []string // tools to exclude from template
    MaxRoundsOverride int      // custom max rounds (0 = use template default)
    Validator         func(result *agents.PhaseResult) error
}

// PhaseRegistry stores and retrieves reusable phase templates.
type PhaseRegistry interface {
    // Register adds a phase template.
    Register(template PhaseTemplate) error

    // Get retrieves a phase template by name.
    Get(name string) (*PhaseTemplate, bool)

    // List returns all registered phase templates.
    List() []PhaseTemplate

    // BuildPhases creates a []agents.Phase from template names with optional overrides.
    // This is the primary API: agents call this to get their phase list.
    BuildPhases(names []string, overrides map[string]PhaseOverride) ([]agents.Phase, error)
}
```

---

## M4: Task Envelope

```go
package orchestration

import "time"

// TaskEnvelope is the structured input for any agent or subagent execution.
type TaskEnvelope struct {
    ID           string            `json:"id"`
    ParentID     string            `json:"parent_id,omitempty"` // set for subagent tasks
    Agent        string            `json:"agent"`
    Goal         string            `json:"goal"`
    Mode         string            `json:"mode"`
    Context      TaskContext       `json:"context"`
    ToolsAllowed []string          `json:"tools_allowed,omitempty"` // empty = mode default
    MaxSteps     int               `json:"max_steps"`
    ReturnSchema map[string]string `json:"return_schema,omitempty"`
    CreatedAt    time.Time         `json:"created_at"`
}

// TaskContext provides scoped context for a task.
type TaskContext struct {
    Files        []string               `json:"files,omitempty"`     // relevant file paths
    Symbols      []string               `json:"symbols,omitempty"`   // function/type names
    PriorResults map[string]interface{} `json:"prior_results,omitempty"`
    Artifacts    []string               `json:"artifacts,omitempty"` // artifact IDs to load
    RepoMap      string                 `json:"repo_map,omitempty"`  // pre-computed repo map
}

// TaskResult is the structured output from agent or subagent execution.
type TaskResult struct {
    TaskID     string                 `json:"task_id"`
    Agent      string                 `json:"agent"`
    Success    bool                   `json:"success"`
    Output     map[string]interface{} `json:"output"`
    Summary    string                 `json:"summary"`
    Artifacts  []ArtifactRef          `json:"artifacts,omitempty"`
    TokensUsed int                    `json:"tokens_used"`
    Duration   time.Duration          `json:"duration"`
    Error      string                 `json:"error,omitempty"`
}

// ArtifactRef is a reference to a produced artifact.
type ArtifactRef struct {
    ID   string `json:"id"`
    Type string `json:"type"` // "repo_map", "plan", "diff", "test_results", "review"
    Path string `json:"path"` // file path in artifact store
}
```

---

## M5: Subagent Manager

```go
package orchestration

import "context"

// SubagentManager spawns and manages child agent executions.
type SubagentManager interface {
    // Spawn creates and starts a subagent from a task envelope.
    // The subagent runs with isolated context under the parent job directory.
    Spawn(ctx context.Context, task TaskEnvelope) (SubagentHandle, error)

    // SpawnAll starts multiple subagents. If parallel=true, they run concurrently.
    SpawnAll(ctx context.Context, tasks []TaskEnvelope, parallel bool) ([]SubagentHandle, error)

    // Wait blocks until a subagent completes and returns its result.
    Wait(ctx context.Context, handle SubagentHandle) (*TaskResult, error)

    // WaitAll blocks until all handles complete.
    WaitAll(ctx context.Context, handles []SubagentHandle) ([]*TaskResult, error)

    // Cancel stops a running subagent.
    Cancel(handle SubagentHandle) error
}

// SubagentHandle is an opaque reference to a running subagent.
type SubagentHandle struct {
    ID       string
    TaskID   string
    Agent    string
    ParentID string
    Status   SubagentStatus
}

// SubagentStatus tracks subagent lifecycle.
type SubagentStatus string

const (
    SubagentRunning   SubagentStatus = "running"
    SubagentCompleted SubagentStatus = "completed"
    SubagentFailed    SubagentStatus = "failed"
    SubagentCancelled SubagentStatus = "cancelled"
)

// DefaultSubagentManager implements SubagentManager using goroutines
// and the existing agent/executor infrastructure.
type DefaultSubagentManager struct {
    config       *config.Config
    backend      llm.Backend
    jobManager   jobs.JobManager
    permissions  PermissionEngine
    parentJobDir string
}

// Subagent context directory layout:
//   /tmp/pedrocli-jobs/<parent-id>/subagents/<child-id>/
//   ├── task.json          # input task envelope
//   ├── result.json        # output task result
//   ├── 001-prompt.txt     # inference history
//   └── artifacts/         # produced artifacts
```

---

## M6: Artifact Store

```go
package orchestration

import (
    "context"
    "io"
)

// ArtifactType identifies the kind of artifact.
type ArtifactType string

const (
    ArtifactRepoMap     ArtifactType = "repo_map"
    ArtifactTask        ArtifactType = "task"
    ArtifactPlan        ArtifactType = "plan"
    ArtifactDiff        ArtifactType = "diff"
    ArtifactTestResults ArtifactType = "test_results"
    ArtifactReview      ArtifactType = "review"
)

// Artifact represents a stored artifact.
type Artifact struct {
    ID        string       `json:"id"`
    Type      ArtifactType `json:"type"`
    Name      string       `json:"name"`
    Path      string       `json:"path"`
    Size      int64        `json:"size"`
    CreatedBy string       `json:"created_by"` // agent or subagent ID
    CreatedAt time.Time    `json:"created_at"`
}

// ArtifactStore manages the shared artifact workspace for a job.
type ArtifactStore interface {
    // Put stores an artifact and returns its ID.
    Put(ctx context.Context, artifactType ArtifactType, name string, reader io.Reader, createdBy string) (*Artifact, error)

    // PutJSON stores a JSON-serializable artifact.
    PutJSON(ctx context.Context, artifactType ArtifactType, name string, data interface{}, createdBy string) (*Artifact, error)

    // Get retrieves an artifact by ID.
    Get(ctx context.Context, id string) (*Artifact, io.ReadCloser, error)

    // GetByType retrieves the latest artifact of a given type.
    GetByType(ctx context.Context, artifactType ArtifactType) (*Artifact, io.ReadCloser, error)

    // List returns all artifacts, optionally filtered by type.
    List(ctx context.Context, filterType *ArtifactType) ([]Artifact, error)

    // Delete removes an artifact.
    Delete(ctx context.Context, id string) error
}

// FileArtifactStore implements ArtifactStore using the job directory filesystem.
// Storage layout:
//   /tmp/pedrocli-jobs/<job-id>/artifacts/
//   ├── manifest.json         # artifact index
//   ├── repo_map.json
//   ├── plan.md
//   ├── diff.patch
//   └── test_results.json
type FileArtifactStore struct {
    jobDir string
}
```

---

## M7: Permission Engine

```go
package orchestration

import "context"

// PermissionAction represents a tool/action category.
type PermissionAction string

const (
    PermRead       PermissionAction = "read"
    PermWrite      PermissionAction = "write"
    PermBash       PermissionAction = "bash"
    PermNetwork    PermissionAction = "network"
    PermGitPush    PermissionAction = "git_push"
    PermGitCommit  PermissionAction = "git_commit"
)

// PermissionDecision is the result of a permission check.
type PermissionDecision string

const (
    PermAllow PermissionDecision = "allow"
    PermDeny  PermissionDecision = "deny"
    PermAsk   PermissionDecision = "ask"   // requires user approval
)

// PermissionRequest describes what is being requested.
type PermissionRequest struct {
    Agent    string           `json:"agent"`
    Tool     string           `json:"tool"`
    Action   PermissionAction `json:"action"`
    Path     string           `json:"path,omitempty"`     // file path if applicable
    Command  string           `json:"command,omitempty"`  // bash command if applicable
    Reason   string           `json:"reason,omitempty"`   // why the agent needs this
}

// PermissionEngine evaluates permission requests against configured policies.
type PermissionEngine interface {
    // Check evaluates a permission request and returns a decision.
    Check(ctx context.Context, req PermissionRequest) (PermissionDecision, error)

    // RequestApproval prompts the user for approval (for "ask" decisions).
    // Returns true if approved, false if denied.
    RequestApproval(ctx context.Context, req PermissionRequest) (bool, error)

    // SetApprovalHandler sets the function that prompts users (REPL or HTTP).
    SetApprovalHandler(handler ApprovalHandler)
}

// ApprovalHandler is called when a permission check returns "ask".
// Implementations differ between REPL (terminal prompt) and HTTP (SSE + UI).
type ApprovalHandler func(ctx context.Context, req PermissionRequest) (approved bool, err error)

// PermissionConfig is the JSON configuration for permissions.
type PermissionConfig struct {
    Defaults map[PermissionAction]PermissionDecision            `json:"defaults"`
    Agents   map[string]map[PermissionAction]PermissionDecision `json:"agents"`
    Paths    PathPermissions                                     `json:"paths"`
}

// PathPermissions defines path-based access rules.
type PathPermissions struct {
    Deny []string `json:"deny"` // glob patterns always denied
    Ask  []string `json:"ask"`  // glob patterns requiring approval
}
```

---

## M8: Layered Prompt Builder

```go
package orchestration

// PromptLayer represents one layer of the prompt stack.
type PromptLayer struct {
    Name     string // "identity", "mode", "phase", "task", "skills", "output_contract"
    Content  string
    Priority int    // lower = earlier in prompt
}

// PromptBuilder composes system prompts from multiple layers.
type PromptBuilder interface {
    // WithIdentity sets the base identity layer (Pedro personality).
    WithIdentity(prompt string) PromptBuilder

    // WithMode sets the mode constraint layer.
    WithMode(mode Mode) PromptBuilder

    // WithPhase sets the active phase layer.
    WithPhase(phase agents.Phase) PromptBuilder

    // WithTask sets the task envelope layer.
    WithTask(task TaskEnvelope) PromptBuilder

    // WithSkills adds repo-specific skills/conventions.
    WithSkills(skills string) PromptBuilder

    // WithOutputContract sets the expected output schema.
    WithOutputContract(schema map[string]string) PromptBuilder

    // Build composes all layers into a single system prompt.
    Build() string
}

// DefaultPromptBuilder implements PromptBuilder with template composition.
type DefaultPromptBuilder struct {
    layers []PromptLayer
}
```

---

## M9: Telemetry

```go
package orchestration

import "time"

// TelemetryEvent represents a single telemetry data point.
type TelemetryEvent struct {
    Timestamp   time.Time              `json:"timestamp"`
    JobID       string                 `json:"job_id"`
    AgentID     string                 `json:"agent_id"`
    Phase       string                 `json:"phase,omitempty"`
    Round       int                    `json:"round,omitempty"`
    EventType   string                 `json:"event_type"` // "inference", "tool_call", "phase_complete", "error"
    Data        map[string]interface{} `json:"data"`
}

// TelemetrySummary aggregates telemetry for a job.
type TelemetrySummary struct {
    JobID            string        `json:"job_id"`
    TotalTokens      int           `json:"total_tokens"`
    PromptTokens     int           `json:"prompt_tokens"`
    CompletionTokens int           `json:"completion_tokens"`
    TotalDuration    time.Duration `json:"total_duration"`
    LLMLatency       time.Duration `json:"llm_latency"`
    ToolCalls        int           `json:"tool_calls"`
    ToolFailures     int           `json:"tool_failures"`
    Rounds           int           `json:"rounds"`
    Phases           int           `json:"phases"`
    EstimatedCost    float64       `json:"estimated_cost_usd"` // based on token pricing
}

// TelemetryCollector records and aggregates telemetry events.
type TelemetryCollector interface {
    // Record logs a telemetry event.
    Record(event TelemetryEvent)

    // Summary returns aggregated telemetry for a job.
    Summary(jobID string) (*TelemetrySummary, error)

    // Events returns raw events for a job, optionally filtered.
    Events(jobID string, eventType string) ([]TelemetryEvent, error)
}

// FileTelemetryCollector stores telemetry in the job directory.
// Storage: /tmp/pedrocli-jobs/<job-id>/telemetry.jsonl
type FileTelemetryCollector struct {
    jobDir string
}
```

---

## Configuration Schema (.pedrocli.json additions)

```json
{
  "orchestration": {
    "default_mode": "chat",
    "intent_classification": "heuristic",
    "max_subagents": 4,
    "subagent_timeout_minutes": 10,
    "artifact_retention_hours": 24
  },
  "modes": {
    "chat": {
      "allowed_tools": ["search", "navigate", "file", "context"],
      "allow_write": false,
      "allow_bash": false,
      "max_rounds": 5
    },
    "plan": {
      "allowed_tools": ["search", "navigate", "file", "context", "git"],
      "allow_write": false,
      "allow_bash": false,
      "max_rounds": 10
    },
    "build": {
      "allowed_tools": ["*"],
      "allow_write": true,
      "allow_bash": true,
      "max_rounds": 30
    },
    "review": {
      "allowed_tools": ["search", "navigate", "file", "git", "github", "test"],
      "allow_write": false,
      "allow_bash": true,
      "max_rounds": 15
    }
  },
  "permissions": {
    "defaults": {
      "read": "allow",
      "write": "ask",
      "bash": "ask",
      "network": "deny",
      "git_push": "ask"
    },
    "agents": {
      "explorer": { "write": "deny" },
      "implementer": { "write": "allow", "bash": "allow" },
      "tester": { "bash": "allow" },
      "reviewer": { "write": "deny" }
    },
    "paths": {
      "deny": [".env", "*.key", "*.pem", "credentials.*"],
      "ask": ["go.mod", "go.sum", "Makefile", "Dockerfile"]
    }
  },
  "phases": {
    "analyze": {
      "tools": ["search", "navigate", "file", "git", "lsp"],
      "max_rounds": 10
    },
    "plan": {
      "tools": ["search", "navigate", "file", "context"],
      "max_rounds": 5
    },
    "implement": {
      "tools": ["file", "code_edit", "search", "git", "bash", "lsp"],
      "max_rounds": 30
    },
    "validate": {
      "tools": ["test", "bash", "file", "code_edit", "search"],
      "max_rounds": 15
    },
    "deliver": {
      "tools": ["git", "github"],
      "max_rounds": 5
    }
  },
  "telemetry": {
    "enabled": true,
    "log_tool_args": false,
    "retention_days": 30
  }
}
```

---

## Integration Points with Existing Code

### Where `QueryEngine` plugs in

**REPL** (`pkg/repl/session.go`): Replace direct agent dispatch with:
```go
// Before: switch on slash command, create agent, execute
// After:
result, err := session.queryEngine.Execute(ctx, userInput)
```

**HTTP Bridge** (`pkg/httpbridge/handlers.go`): Replace `CreateJobRequest` switch:
```go
// Before: switch req.Type { case "builder": ... }
// After:
result, err := app.queryEngine.ExecuteWithMode(ctx, req.Description, mode)
```

### Where `SubagentManager` plugs in

**PhasedExecutor** (`pkg/agents/phased_executor.go`):
```go
// In the "implement" phase, spawn explorer + implementer subagents
// instead of running everything in a single inference loop
```

**BlogOrchestratorAgent** (`pkg/agents/blog_orchestrator.go`):
```go
// Research phase spawns parallel subagents:
// - RSS subagent
// - Calendar subagent  
// - Web search subagent
// Results aggregated into artifacts before outline phase
```

### Where `PermissionEngine` plugs in

**InferenceExecutor** (`pkg/agents/executor.go`):
```go
// In executeTool(), before calling tool.Execute():
decision, err := permissions.Check(ctx, PermissionRequest{
    Agent: e.agent.Name(),
    Tool:  toolName,
    Action: classifyToolAction(toolName, args),
})
```

This extends the existing `middleware.PolicyEvaluator` pattern already in the executor.
