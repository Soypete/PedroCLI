# M4: Task Envelope + Structured I/O

> Status: **Completed** | Started: 2026-04-02 | Completed: 2026-04-02

## Problem

Currently, phases and agents pass unstructured strings between each other. This makes it hard to:
- Parse agent outputs reliably
- Chain agents together
- Validate what an agent produced
- Debug execution flow

## Solution

Introduced typed task envelopes that flow through the system:

```go
type TaskEnvelope struct {
    ID           string
    Agent        string
    Goal         string
    Mode         Mode
    Context      TaskContext
    ToolsAllowed []string
    MaxSteps     int
    ReturnSchema map[string]string
}

type TaskContext struct {
    Workspace  string
    WorkingDir string
    Files      []string
    Metadata   map[string]interface{}
}

type TaskResult struct {
    ID         string
    Success    bool
    Output     string
    Parsed     map[string]interface{}
    Error      string
    RoundsUsed int
    Finished   bool
}
```

## Key Decisions

1. **Types in orchestration package** - TaskEnvelope lives in `pkg/orchestration/task.go` alongside QueryEngine and Mode for cohesive orchestration layer
2. **Helper constructors** - Added `NewTaskEnvelope()` factory and fluent methods (`SetToolsAllowed()`, `SetReturnSchema()`, `AddFile()`, `SetMetadata()`)
3. **Validation method** - Added `Validate()` method on TaskEnvelope to ensure required fields before execution
4. **Parsed output support** - TaskResult includes `Parsed` map for structured output based on ReturnSchema

## Files Changed

### New Files
- `pkg/orchestration/task.go` - TaskEnvelope, TaskContext, TaskResult types with validation and helpers
- `pkg/orchestration/task_test.go` - Unit tests for task types

### Modified Files
- None (phased executor integration deferred to M5)

## Usage

```go
import "github.com/soypete/pedrocli/pkg/orchestration"

// Create a task envelope
envelope := orchestration.NewTaskEnvelope(
    "builder",
    "Add user authentication",
    orchestration.ModeCode,
    "/workspace/project",
)

// Configure tools and schema
envelope.SetToolsAllowed([]string{"file", "code_edit", "search"})
envelope.SetReturnSchema(map[string]string{
    "files_modified": "[]string",
    "success":        "bool",
})

// Validate before execution
if err := envelope.Validate(); err != nil {
    return err
}

// Execute and get structured result
result, err := engine.ExecuteTask(ctx, envelope)
if result.Parsed["success"] == true {
    files := result.Parsed["files_modified"].([]string)
}
```

## Dependencies

- M1: Query Engine
- M3: Phase Registry

## Next Steps

- [M5: Subagent Manager](../M5-subagent-manager/) - Use TaskEnvelope for parent-child agent communication

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-4-task-envelope--structured-io)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m4-task-envelope)