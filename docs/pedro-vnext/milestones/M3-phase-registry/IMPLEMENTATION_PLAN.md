# M3: Phase Registry - Implementation Plan

## Overview

This document outlines the implementation plan for M3: Phase Registry. The goal is to make phases reusable across agents instead of hardcoded per agent.

## Current State

### What's Already Built

| Component | Location | Status |
|-----------|----------|--------|
| `Phase` struct | `pkg/agents/phased_executor.go:19-30` | Exists |
| `PhasedExecutor` | `pkg/agents/phased_executor.go` | Exists |
| `BuilderPhasedAgent.GetPhases()` | `pkg/agents/builder_phased.go:56-121` | Hardcoded |
| `DebuggerPhasedAgent.GetPhases()` | `pkg/agents/debugger_phased.go:59-128` | Hardcoded |
| `ReviewerPhasedAgent.GetPhases()` | `pkg/agents/reviewer_phased.go:56-119` | Hardcoded |

### Problem

Each phased agent defines its phases independently:
- Similar phases (analyze, plan, implement, validate) have duplicated definitions
- Tool lists are repeated across agents
- Phase validators are copy-pasted
- No way to compose phases from a shared catalog

## Implementation

### Phase 1: Create PhaseRegistry

**File**: `pkg/orchestration/phase_registry.go`

```go
package orchestration

// PhaseDefinition defines a reusable phase template
type PhaseDefinition struct {
    Name        string   // Phase identifier (e.g., "analyze", "plan")
    Description string   // Human-readable description
    Tools       []string // Allowed tools for this phase
    MaxRounds   int      // Default max rounds
    ExpectsJSON bool     // Whether phase expects JSON output
}

// PhaseRegistry provides reusable phase definitions
type PhaseRegistry interface {
    GetPhase(name string) (*PhaseDefinition, error)
    GetPhases(names []string) ([]PhaseDefinition, error)
    ListPhases() []string
    RegisterPhase(def PhaseDefinition)
}

// DefaultPhaseRegistry returns the built-in phase registry
func DefaultPhaseRegistry() PhaseRegistry
```

### Phase 2: Define Standard Phases

**Standard phases** (to be defined in `phase_registry.go`):

| Phase | Tools | Description |
|-------|-------|-------------|
| `analyze` | search, navigate, file, git, lsp | Analyze request, evaluate repo state |
| `plan` | search, navigate, file, context | Create detailed implementation plan |
| `implement` | file, code_edit, search, git, bash, lsp | Write code following the plan |
| `validate` | test, bash, file, code_edit, search | Run tests, verify implementation |
| `deliver` | git, github | Commit changes, create PR |
| `review` | search, navigate, file, git, github, lsp | Code review analysis |
| `reproduce` | test, bash, file, search | Reproduce the issue |
| `investigate` | search, file, lsp, git, navigate | Gather evidence about root cause |
| `isolate` | file, lsp, search, bash | Narrow down to exact root cause |
| `fix` | file, code_edit, search, lsp | Implement targeted fix |
| `verify` | test, bash, lsp, file, code_edit | Verify fix works |
| `commit` | git | Commit the fix |
| `gather` | github, git, lsp, search, navigate, file | Fetch PR details, checkout branch |
| `security` | search, file, lsp, context | Analyze security vulnerabilities |
| `quality` | search, file, lsp, navigate, context | Review code quality |
| `compile` | context | Compile findings into structured output |
| `publish` | github | Post review to GitHub |

### Phase 3: Agent Refactoring

**Modified files**:
- `pkg/agents/builder_phased.go`
- `pkg/agents/debugger_phased.go`
- `pkg/agents/reviewer_phased.go`

**Changes**:
1. Add `PhaseRegistry` dependency to agents
2. Refactor `GetPhases()` to compose from registry
3. Support phase overrides (custom prompt, custom validator)

```go
// Example: BuilderPhasedAgent using registry
func (b *BuilderPhasedAgent) GetPhases() []Phase {
    registry := orchestration.DefaultPhaseRegistry()
    
    analyzePhase, _ := registry.GetPhase("analyze")
    planPhase, _ := registry.GetPhase("plan")
    implementPhase, _ := registry.GetPhase("implement")
    validatePhase, _ := registry.GetPhase("validate")
    deliverPhase, _ := registry.GetPhase("deliver")
    
    return []Phase{
        {
            Name:         analyzePhase.Name,
            Description:  analyzePhase.Description,
            SystemPrompt: builderAnalyzePrompt, // Custom prompt
            Tools:        analyzePhase.Tools,
            MaxRounds:    analyzePhase.MaxRounds,
            ExpectsJSON:  analyzePhase.ExpectsJSON,
            Validator:    defaultValidator,
        },
        // ... etc
    }
}
```

### Phase 4: Phase Dependencies (Optional)

Support declaring phase dependencies:
- `implement` requires `plan` completed first
- `validate` requires `implement` completed first

```go
type PhaseDefinition struct {
    // ... existing fields
    Dependencies []string // Required phases that must complete first
}
```

### Phase 5: Phase Configuration (Optional)

Support YAML/JSON phase configuration:

```yaml
# .pedrocli-phases.yaml
phases:
  custom_validate:
    description: "Custom validation with specific test commands"
    tools:
      - test
      - bash
    max_rounds: 20
```

## Files to Create

| File | Description |
|------|-------------|
| `pkg/orchestration/phase_registry.go` | PhaseRegistry interface and default implementation |

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/agents/builder_phased.go` | Use PhaseRegistry in GetPhases() |
| `pkg/agents/debugger_phased.go` | Use PhaseRegistry in GetPhases() |
| `pkg/agents/reviewer_phased.go` | Use PhaseRegistry in GetPhases() |

## Testing

1. Unit tests for `PhaseRegistry` methods
2. Integration tests for phased agents using registry
3. Verify backward compatibility (existing agents work unchanged)

## Estimated Effort

- Phase 1-2: ~2 hours (create registry + define phases)
- Phase 3: ~1 hour per agent (3 agents = 3 hours)
- Phase 4-5: ~2 hours (optional)
- Testing: ~2 hours

**Total**: ~7-9 hours

## Dependencies

- None (can start independently)
- Depends on: M1 (Query Engine), M2 (Execution Modes) - but can be implemented standalone