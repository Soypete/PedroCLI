# M5: Subagent Manager - Implementation

## What Was Built

### Core Components

**1. SubagentManager Interface** (`pkg/orchestration/subagent.go`)

```go
type SubagentManager interface {
    Spawn(ctx context.Context, task TaskEnvelope) (SubagentHandle, error)
    SpawnAll(ctx context.Context, tasks []TaskEnvelope, parallel bool) ([]SubagentHandle, error)
    Wait(ctx context.Context, handle SubagentHandle) (*TaskResult, error)
    WaitAll(ctx context.Context, handles []SubagentHandle) ([]*TaskResult, error)
    Cancel(handle SubagentHandle) error
    List(ctx context.Context) ([]SubagentHandle, error)
}
```

**2. DefaultSubagentManager** (`pkg/orchestration/default_subagent_manager.go`)

- Spawns subagents with isolated context directories
- Supports sequential and parallel execution
- Manages lifecycle: spawn → execute → wait → result
- Thread-safe with RWMutex for handle/result maps

**3. Subagent Types** (`pkg/orchestration/subagent.go`)

| Type | Role | Tools | Max Rounds |
|------|------|-------|------------|
| `explorer` | Search and map codebase | search, navigate, file, lsp | 10 |
| `implementer` | Write code changes | file, code_edit, bash | 20 |
| `tester` | Run and fix tests | test, bash, file, code_edit | 15 |
| `reviewer` | Validate changes | search, file, git, test | 10 |
| `doc-writer` | Generate documentation | file, search, navigate | 10 |

### Key Design Decisions

1. **Same LLM Backend**: Subagents reuse parent's backend (future: per-subagent config)
2. **Sequential by Default**: CLI pattern - use `parallel=true` for HTTP
3. **Context Inheritance**: Parent passes specific files via `TaskEnvelope.ContextFiles`
4. **Pointer Handles**: `map[string]*SubagentHandle` for mutation safety
5. **Thread-Safe Errors**: Mutex protects error slice in parallel SpawnAll

### Files Created

```
pkg/orchestration/
├── subagent.go                      # Interface + types + helpers
├── default_subagent_manager.go      # Implementation (337 lines)
└── subagent_test.go                 # Unit tests
```

### Files Modified

- `pkg/orchestration/default_subagent_manager.go` - Bug fixes for race conditions

## Running

```go
// Create manager
mgr := NewDefaultSubagentManager(cfg, backend, jobMgr, toolRegistry, parentJobDir, workspaceDir)

// Spawn single subagent
handle, err := mgr.Spawn(ctx, TaskEnvelope{
    ID:     "task-1",
    Agent:  "explorer",
    Goal:   "Find all files related to auth",
})

// Wait for result
result, err := mgr.Wait(ctx, handle)

// Or spawn multiple (sequential)
handles, err := mgr.SpawnAll(ctx, tasks, false)

// Or spawn multiple (parallel)
handles, err := mgr.SpawnAll(ctx, tasks, true)
```

## Known Issues

1. **Cancel doesn't stop execution** - Only marks status, doesn't cancel context
2. **Hardcoded timeout** - 5-minute wait timeout should be configurable
3. **Limited test coverage** - No integration tests for Spawn/Wait lifecycle

## Next Steps (M6)

- [M6: Artifact Store](../M6-artifact-store/) - Structured shared workspace for subagent coordination