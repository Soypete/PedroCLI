# M8: Prompt Architecture (Layered Prompts)

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, system prompts are monolithic. Adding mode-specific or phase-specific constraints requires modifying the base prompt. There's no clean way to compose prompts from multiple sources.

## Solution

<!-- How we solved it -->

Replace monolithic prompts with a layered system:

| Layer | Source | Purpose |
|-------|--------|---------|
| 1. Identity | `prompts/identity.md` | Pedro personality, global rules |
| 2. Mode | `prompts/mode_{name}.md` | What is allowed in this mode |
| 3. Phase | `prompts/phase_{name}.md` | What phase is active, goals |
| 4. Task | Task envelope JSON | What to do specifically |
| 5. Skills | `.pedrocli.json` + repo context | Repo conventions, tech stack |
| 6. Output contract | Phase/task return schema | Expected output format |

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/prompt_layers.go` - PromptBuilder interface
- `pkg/orchestration/default_prompt_builder.go` - Implementation
- `pkg/agents/prompts/` - Restructure prompt files

### Modified Files
- `pkg/agents/base.go` - Use layered prompt builder
- `pkg/agents/executor.go` - Build prompts via PromptBuilder

## Dependencies

- M2: Execution Modes
- M4: Task Envelope
- M6: Artifact Store (can run in parallel)

## Next Steps

- [M9: Telemetry](../M9-telemetry/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-8-prompt-architecture-layered-prompts)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m8-layered-prompt-builder)