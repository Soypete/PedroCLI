# M3: Phase Registry

> Status: **Completed** | Started: 2026-04-02 | Completed: 2026-04-02

## Problem

Phases (analyze, plan, implement, validate, deliver) were hardcoded per agent. The BuilderPhasedAgent, DebuggerPhasedAgent, and ReviewerPhasedAgent each defined their own phase lists with duplicated definitions.

## Solution

Created a reusable phase registry (`pkg/phases/registry.go`) where phases are defined once and can be composed by any agent:

| Phase | Tools | Used By |
|-------|-------|---------|
| `analyze` | search, navigate, file, git, lsp | builder |
| `plan` | search, navigate, file, context | builder |
| `implement` | file, code_edit, search, git, bash, lsp, context | builder |
| `validate` | test, bash, file, code_edit, lsp, search, navigate | builder |
| `deliver` | git, github | builder |
| `reproduce` | test, bash, file, search | debugger |
| `investigate` | search, file, lsp, git, navigate, context | debugger |
| `isolate` | file, lsp, search, bash, context | debugger |
| `fix` | file, code_edit, search, lsp | debugger |
| `verify` | test, bash, lsp, file, code_edit | debugger |
| `commit` | git | debugger |
| `gather` | github, git, lsp, search, navigate, file | reviewer |
| `security` | search, file, lsp, context | reviewer |
| `quality` | search, file, lsp, navigate, context | reviewer |
| `compile` | context | reviewer |
| `publish` | github | reviewer |

## Key Decisions

1. **Created separate `pkg/phases` package** - Avoids import cycle with `pkg/orchestration` (which imports `pkg/agents`)
2. **Agents customize prompts/validators but inherit tools from registry** - Registry defines tools, agents provide custom prompts and validators
3. **Phase dependencies tracked but not enforced** - The `DependsOn` field exists for documentation/future validation but current implementation runs phases sequentially

## Files Changed

### New Files
- `pkg/phases/registry.go` - Registry interface and default implementation with 16 standard phases

### Modified Files
- `pkg/agents/builder_phased.go` - Refactored to use `phases.DefaultRegistry()`
- `pkg/agents/debugger_phased.go` - Refactored to use `phases.DefaultRegistry()`
- `pkg/agents/reviewer_phased.go` - Refactored to use `phases.DefaultRegistry()`

### Documentation
- `docs/pedro-vnext/milestones/M3-phase-registry/IMPLEMENTATION_PLAN.md` - Detailed implementation plan

## Usage

```go
import "github.com/soypete/pedrocli/pkg/phases"

// Get the default registry
registry := phases.DefaultRegistry()

// Get a specific phase
analyzePhase, _ := registry.GetPhase("analyze")
fmt.Println(analyzePhase.Tools) // ["search", "navigate", "file", "git", "lsp"]

// List all available phases
phases := registry.ListPhases()
```

## Dependencies

- None (standalone package)

## Next Steps

- [M4: Task Envelope](../M4-task-envelope/)
- [M1: Query Engine](../M1-query-engine/)

## Reference

- [Implementation Plan](./IMPLEMENTATION_PLAN.md)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m3-phase-registry)