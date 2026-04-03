# M8: Prompt Architecture (Layered Prompts)

> Status: Completed | Completed: 2025-04-02

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, system prompts are monolithic. Adding mode-specific or phase-specific constraints requires modifying the base prompt. There's no clean way to compose prompts from multiple sources.

## Solution

<!-- How we solved it -->

Replace monolithic prompts with a layered system:

| Layer | Source | Purpose |
|-------|--------|---------|
| 1. Identity | `prompts/layers/identity.md` | Pedro personality, global rules |
| 2. Mode | `prompts/layers/mode_{name}.md` | What is allowed in this mode |
| 3. Phase | `prompts/layers/phase_{name}.md` | What phase is active, goals |
| 4. Task | Task envelope JSON | What to do specifically |
| 5. Skills | `.pedrocli.json` + repo context | Repo conventions, tech stack |
| 6. Output contract | Phase/task return schema | Expected output format |

## Implementation

### Core Components

**PromptBuilder** (`pkg/prompts/layered.go`):
- `NewPromptBuilder()` - Creates a new builder
- `SetIdentity()`, `SetMode()`, `SetPhase()`, `SetTask()`, `SetSkills()`, `SetOutputSchema()`
- `Build()` - Composes all layers into a single prompt

**Layer Files** (`pkg/prompts/layers/`):
- `identity.md` - Pedro's identity and global rules
- `mode_chat.md`, `mode_plan.md`, `mode_build.md`, `mode_review.md` - Mode constraints
- `phase_analyze.md`, `phase_plan.md`, `phase_implement.md` - Phase-specific guidance

### Wiring

**PhasedExecutor** (`pkg/agents/phased_executor.go`):
- Added `mode` field to track current execution mode
- Added `useLayeredPrompts` boolean flag
- `SetMode(mode string)` - Enables layered prompts with specified mode
- `buildLayeredPrompt()` - Composes layered prompt for each phase execution
- Falls back to embedded `.md` prompts if `SystemPrompt` is set on Phase

**BaseAgent** (`pkg/agents/base.go`):
- Added `BuildLayeredPrompt(mode, task string)` method for non-phased agents

**Agent Integration**:
- `BuilderPhasedAgent.Run()` - Calls `executor.SetMode("build")`
- `DebuggerPhasedAgent.Run()` - Calls `executor.SetMode("build")` (for fixes)
- `ReviewerPhasedAgent.Run()` - Calls `executor.SetMode("review")`

**REPL Integration** (`pkg/repl/interactive_sync.go`):
- Added `SetMode()` and `EnableLayeredPrompts()` calls for phased agents

## Key Decisions

1. **Decision**: Layered prompts are opt-in - Agents must call `SetMode()` to enable them
   - **Reason**: Backward compatibility with existing embedded prompts
2. **Decision**: Phase-level `SystemPrompt` takes precedence over layered prompts
   - **Reason**: Allows phased agents to use existing embedded prompts while migrating
3. **Decision**: Mode constraints are defined in code, layer files contain descriptive text
   - **Reason**: Mode constraints need programmatic access for tool filtering

## Files Changed

### New Files
- `pkg/prompts/layered.go` - PromptBuilder with 6-layer composition
- `pkg/prompts/layers/identity.md` - Pedro's identity
- `pkg/prompts/layers/mode_chat.md` - Chat mode constraints
- `pkg/prompts/layers/mode_plan.md` - Plan mode constraints
- `pkg/prompts/layers/mode_build.md` - Build mode constraints
- `pkg/prompts/layers/mode_review.md` - Review mode constraints
- `pkg/prompts/layers/phase_analyze.md` - Analyze phase guidance
- `pkg/prompts/layers/phase_plan.md` - Plan phase guidance
- `pkg/prompts/layers/phase_implement.md` - Implement phase guidance

### Modified Files
- `pkg/agents/phased_executor.go` - Added mode, useLayeredPrompts fields, buildLayeredPrompt method
- `pkg/agents/base.go` - Added BuildLayeredPrompt method
- `pkg/agents/builder_phased.go` - Added `executor.SetMode("build")`
- `pkg/agents/debugger_phased.go` - Added `executor.SetMode("build")`
- `pkg/agents/reviewer_phased.go` - Added `executor.SetMode("review")`
- `pkg/repl/interactive_sync.go` - Added mode configuration for phased agents

## Dependencies

- M2: Execution Modes
- M4: Task Envelope
- M6: Artifact Store (can run in parallel)

## Next Steps

- [M9: Telemetry](../M9-telemetry/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-8-prompt-architecture-layered-prompts)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m8-layered-prompt-builder)