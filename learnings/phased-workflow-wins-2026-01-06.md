# Phased Workflow Implementation - Immediate Wins

**Date:** 2026-01-06
**Context:** Switched HTTP server agents from unstructured inference loops to structured 5-phase workflows

## What We Did

### 1. Integrated Phased Agents into HTTP Server
Updated `pkg/httpbridge/app.go` to use the new phased agent implementations:

**Before:**
```go
func (ctx *AppContext) NewBuilderAgent() *agents.BuilderAgent {
    agent := agents.NewBuilderAgent(ctx.Config, ctx.Backend, ctx.JobManager)
    // ...
}
```

**After:**
```go
func (ctx *AppContext) NewBuilderAgent() *agents.BuilderPhasedAgent {
    agent := agents.NewBuilderPhasedAgent(ctx.Config, ctx.Backend, ctx.JobManager)
    // ...
}
```

**Changed Agents:**
- `BuilderAgent` → `BuilderPhasedAgent`
- `DebuggerAgent` → `DebuggerPhasedAgent`
- `ReviewerAgent` → `ReviewerPhasedAgent`

### 2. Fixed Migration Numbering Conflict
During rebase, discovered duplicate migration version 011:
- `011_compaction_stats.sql` (from feature branch)
- `011_add_workflow_tracking.sql` (from main)

**Resolution:** Renumbered `011_compaction_stats.sql` → `012_compaction_stats.sql`

### 3. Fixed Context Manager Calls
Updated phased agents to include missing `contextSize` parameter:
```go
// Before (broken)
contextMgr, err := llmcontext.NewManager(job.ID, b.config.Debug.Enabled)

// After (fixed)
contextMgr, err := llmcontext.NewManager(job.ID, b.config.Debug.Enabled, b.config.Model.ContextSize)
```

**Files Fixed:**
- `pkg/agents/builder_phased.go`
- `pkg/agents/debugger_phased.go`
- `pkg/agents/reviewer_phased.go`

## Immediate Wins

### ✅ 1. Structured Progress Tracking
Jobs now show **clear phase progression** with visible milestones:

```
📋 Phase 1/5: analyze
   Analyze the request, evaluate repo state, gather requirements
  ✅ Phase analyze completed in 1 rounds

📋 Phase 2/5: plan
   Create a detailed implementation plan with numbered steps
  ✅ Phase plan completed in 1 rounds

📋 Phase 3/5: implement
   Write code following the plan, chunk by chunk
  🔄 Round 1/30
```

**Before:** Just saw "Inference round 3/25" with no context
**After:** Know exactly what the agent is doing at each step

### ✅ 2. Better Resource Management
Each phase has **limited iterations** appropriate to the task:
- **Analyze:** 10 rounds (exploratory)
- **Plan:** 5 rounds (focused planning)
- **Implement:** 30 rounds (code writing needs more iterations)
- **Validate:** 15 rounds (iterative test fixing)
- **Deliver:** 5 rounds (git operations)

**Benefit:** Prevents runaway inference loops while giving enough room for complex tasks

### ✅ 3. Tool Specialization Per Phase
Phases only have access to relevant tools:

| Phase | Tools Available |
|-------|----------------|
| Analyze | search, navigate, file, git, github, lsp |
| Plan | search, navigate, file, context |
| Implement | file, code_edit, search, navigate, git, bash, lsp, context |
| Validate | test, bash, file, code_edit, lsp |
| Deliver | git, github |

**Benefit:** Reduces token usage and prevents phase confusion (e.g., agent won't try to commit during analysis)

### ✅ 4. JSON Output Expectations
Phases like `analyze` and `plan` expect structured JSON output:
```go
ExpectsJSON: true,
Validator: func(result *PhaseResult) error {
    if result.Data == nil || result.Data["analysis"] == nil {
        return fmt.Errorf("analysis phase produced no output")
    }
    return nil
}
```

**Benefit:** Enables programmatic consumption of agent analysis and plans

### ✅ 5. Automatic PR Creation
The **Deliver** phase automatically:
1. Commits changes with proper git messages
2. Creates draft PR with description
3. Includes Claude Code attribution

**From PR #54 (first phased workflow PR):**
```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

**Benefit:** Complete end-to-end automation from task description to reviewable PR

## Test Case: Prometheus Observability (Issue #32)

**Job ID:** `dde0a067-ab67-44b7-ad35-824e388ec2d6`
**Task:** Implement Prometheus metrics for Kubernetes deployment

**Progress (as of 2026-01-06 19:57):**
1. ✅ **Analyze** - Completed in 1 round
2. ✅ **Plan** - Completed in 1 round
3. 🔄 **Implement** - In progress (Round 1/30)
4. ⏳ **Validate** - Pending
5. ⏳ **Deliver** - Pending

**Expected Outcome:** Full Prometheus implementation with tests and PR

## Tools Used in Phased Workflows

### Phase-Specific Tools
- **LSP (Language Server Protocol):** Type information, definitions, references
- **Context Tool:** Manages phase state and cross-phase data sharing
- **GitHub Tool:** Creates issues, PRs, manages labels
- **Git Tool:** Commits, branches, status checks
- **Code Edit Tool:** Precise line-based editing
- **Test Tool:** Runs Go tests, npm tests, Python tests
- **Bash Tool:** Build commands, linters, formatters

### Workflow Infrastructure
- **PhasedExecutor:** Orchestrates phase transitions
- **PhaseResult:** Captures phase output and metadata
- **Workflow Tracking DB:** Stores current_phase and phase_results in PostgreSQL

## Key Learnings

### 1. Phase Boundaries Matter
Clear separation between analyze/plan/implement prevents:
- Premature code writing during analysis
- Analysis paralysis during implementation
- Test failures from incomplete implementation

### 2. Iteration Limits Force Focus
Setting `MaxRounds` per phase creates natural checkpoints:
- Forces agents to produce results within constraints
- Prevents infinite loops from context window issues
- Makes progress measurable and predictable

### 3. Tool Access Shapes Behavior
Restricting tools per phase guides the LLM:
- Can't commit without git tool (only in Deliver phase)
- Can't run tests during planning (only in Validate phase)
- Can't edit code during analysis (only in Implement phase)

### 4. JSON Expectations Enable Composition
Structured output from early phases feeds later phases:
- Analysis JSON → Planning input
- Plan JSON → Implementation checklist
- Test results JSON → Validation summary

## Prior Art: Claude Code PRs

**PR #54:** [feat: Add phased workflow infrastructure for agentic code agents](https://github.com/Soypete/PedroCLI/pull/54)
- First PR implementing the phased workflow system
- Added workflow tracking migrations (011_add_workflow_tracking.sql)
- Introduced PhasedExecutor and phase-specific prompts
- Status: **MERGED** on 2026-01-06

**PR #46:** [Add Language Server Protocol integration to Pedro CLI](https://github.com/Soypete/PedroCLI/pull/46)
- Added LSP tool for code intelligence
- Enables goto-definition, find-references, hover info
- Critical for code-writing phases
- Status: **MERGED** on 2026-01-04

## Next Steps

1. **Monitor Prometheus Job:** Watch first phased workflow complete end-to-end
2. **Gather Metrics:** Track phase completion times and iteration counts
3. **SQLite Removal:** Clean up database layer (separate PR)
4. **Phase Optimization:** Adjust iteration limits based on real usage

## References

- Phased workflow implementation: `pkg/agents/phased_executor.go`
- Builder phases: `pkg/agents/builder_phased.go`
- Workflow migrations: `pkg/database/migrations/011_add_workflow_tracking.sql`
- HTTP bridge updates: `pkg/httpbridge/app.go` (this commit)

---

**Author:** Miriah Peterson + Claude Sonnet 4.5
**Branch:** `fix/context-compaction-51`
**Related Issues:** #32 (Prometheus observability), #57 (SQLite removal)
