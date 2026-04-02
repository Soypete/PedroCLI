# M2: Execution Modes

> Status: Completed | Started: 2025-01 | Completed: 2025-04

## Problem

Currently there is no way to constrain what an agent can do. All queries execute with the same permissions and tool access, regardless of user intent.

## Solution

Introduce execution modes that define behavioral constraints:

| Mode | Allowed Tools | Writes | Agent Types |
|------|--------------|--------|-------------|
| `chat` | search, navigate, file, context | No | builder, debugger, reviewer, triager |
| `plan` | search, navigate, file, context, git | No | triager, reviewer |
| `build` | all | Yes | builder, debugger |
| `review` | search, navigate, file, git, github, test | No | reviewer |
| `code` | all (default) | Yes | builder, debugger, reviewer, triager |
| `blog` | file, rss, web_search, web_scraper, context | Yes | blog_content, dynamic_blog |
| `podcast` | file, web_search, web_scraper, context, notion, cal_com | Yes | podcast |
| `technical_writer` | search, web_scraper, file, context, code_search | Yes | technical_writer |

## Key Decisions

1. **Mode Constraints in agents pkg**: To avoid import cycles between `pkg/agents` and `pkg/orchestration`, mode constraints are defined in `pkg/agents/mode_constraints.go` rather than importing from orchestration.

2. **ApplyModeConstraintsToExecutor helper**: Created a helper function that applies mode constraints to any executor implementing `SetModeConstraints()` and `SetMaxRounds()`.

3. **Agent type to mode mapping**: Each agent type maps to a mode (e.g., "builder" → "code", "podcast" → "podcast").

## Files Changed

### New Files
- `pkg/orchestration/mode.go` - Mode type and ModeConfig struct
- `pkg/orchestration/mode_config.go` - Default mode configurations
- `pkg/agents/mode_constraints.go` - Agent-side mode constraint helpers (avoids import cycle)

### Modified Files
- `pkg/agents/executor.go` - Added `SetModeConstraints()`, `SetMaxRounds()`, `isToolAllowed()` methods
- `pkg/agents/builder.go` - Added mode constraints
- `pkg/agents/debugger.go` - Added mode constraints
- `pkg/agents/reviewer.go` - Added mode constraints
- `pkg/agents/triager.go` - Added mode constraints
- `pkg/agents/blog_dynamic.go` - Added mode constraints
- `pkg/agents/podcast.go` - Added mode constraints (5 executors)
- `pkg/agents/technical_writer.go` - Added mode constraints

## Dependencies

- M1: Query Engine (required)

## Next Steps

- [M3: Phase Registry](../M3-phase-registry/)
- [M7: Permission Engine](../M7-permission-engine/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-2-execution-modes)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m2-execution-modes)