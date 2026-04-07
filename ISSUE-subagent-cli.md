# Subagent Support for CLI Blog Command

## Summary
Add subagent support to CLI to enable parallel research and analysis in the blog workflow.

## Current State
- Subagent infrastructure exists in `pkg/orchestration/default_subagent_manager.go`
- `phased_executor.go` has `SubagentSpawner` interface
- Agent registry has `research` subagent defined
- **CLI does NOT wire up subagents** - no references in `cmd/pedrocli/`
- **HTTP server does NOT wire up subagents** - no references in `cmd/http-server/`

## Benefits
1. Parallel web searches during research phase
2. Concurrent analysis tasks
3. Faster blog generation workflow

## Implementation Steps
1. Wire `SubagentManager` to blog command in `cmd/pedrocli/main.go`
2. Pass `SubagentManager` to `BlogContentAgent`
3. Update research phase to spawn subagents for parallel searches
4. Handle subagent results in main workflow

## Related Files
- `pkg/orchestration/default_subagent_manager.go` - subagent manager implementation
- `pkg/agents/phased_executor.go` - has SubagentSpawner interface
- `pkg/agentregistry/registry.go` - has research subagent
- `pkg/agents/blog_content.go` - blog content agent (needs subagent integration)