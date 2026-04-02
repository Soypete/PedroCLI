# Issue: Deprecate non-phased agent variants

## Summary

After implementing M3: Phase Registry, we identified that there are redundant agent implementations - both phased and non-phased versions exist.

## Current State

**Phased agents (use PhasedExecutor):**
- `BuilderPhasedAgent` ✅ Uses `phases.DefaultRegistry()`
- `DebuggerPhasedAgent` ✅ Uses `phases.DefaultRegistry()`
- `ReviewerPhasedAgent` ✅ Uses `phases.DefaultRegistry()`

**Legacy non-phased agents (direct execution):**
- `BuilderAgent` - non-phased version of BuilderPhasedAgent
- `DebuggerAgent` - non-phased version of DebuggerPhasedAgent
- `ReviewerAgent` - non-phased version of ReviewerPhasedAgent
- `TriagerAgent` - no phased equivalent
- `TechnicalWriterAgent` - no phased equivalent

## Proposed Action

1. **Deprecate legacy agents**: `BuilderAgent`, `DebuggerAgent`, `ReviewerAgent`
2. **Migrate callers** to use phased versions
3. **Optionally**: Create phased versions for `TriagerAgent`, `TechnicalWriterAgent`

## Files to Update

- Remove/deprecate `pkg/agents/builder.go`, `pkg/agents/debugger.go`, `pkg/agents/reviewer.go`
- Update `pkg/cli/bridge.go` to only register phased agents
- Update `pkg/httpbridge/app.go` agent factory methods

## Priority

Medium - Technical debt, not blocking functionality