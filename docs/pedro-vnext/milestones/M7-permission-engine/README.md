# M7: Permission Engine

> Status: Completed | Started: 2026-04 | Completed: 2026-04

## Problem

Currently, tool access is controlled via middleware but there's no granular per-agent, per-tool, per-path permission system. The REPL has approval prompts but no unified permission engine.

## Solution

Extended the existing `PermissionManager` in `pkg/permission/` with agent-specific permissions and path-based restrictions.

### New Features

**Agent-specific permissions**:
- `SetAgentPermission(agent, tool, permission)` - Set permission for specific agent+tool combo
- `SetDefaultAgentPermission(agent, permission)` - Set default permission for an agent
- `CheckAgentTool(agent, tool)` - Check if tool is allowed for specific agent

**Path-based restrictions**:
- `SetPathRestriction(restriction)` - Add path pattern restrictions
- `CheckPath(tool, path)` - Check if operation allowed on path
- `CheckAgentToolAndPath(agent, tool, path)` - Combined agent+tool+path check

**Permission levels**: `allow`, `deny`, `ask` (prompt user)

## Key Decisions

1. **Extended existing permission manager**: Instead of creating new packages (`pkg/orchestration/permissions.go`), we extended the existing `pkg/permission/manager.go` which already had tool permissions, bash commands, and patterns.

2. **Agent defaults with override**: Agent-specific permissions use defaults that can be overridden per-tool. If no agent-specific rule exists, falls back to global tool permissions.

3. **Path restrictions apply last**: Path restrictions are checked after agent+tool permissions. A deny on path will always block, even if the tool is allowed for that agent.

4. **CLI-only (no MCP)**: Per user requirement, PedroCode focuses on CLI only - no external MCP support.

## Files Changed

### Modified Files
- `pkg/permission/manager.go` - Added agent permissions, path restrictions, and new Check methods

### Integration Points
- PermissionManager is already used by `pkg/opencode/manager.go` via `CheckPermission()`
- Can now pass agent name to `CheckAgentTool()` for per-agent permissions

## Usage Example

```go
pm := permission.NewPermissionManager()

// Agent-specific permissions
pm.SetDefaultAgentPermission("explorer", permission.PermissionAllow)
pm.SetAgentPermission("explorer", "write", permission.PermissionDeny)
pm.SetAgentPermission("explorer", "bash", permission.PermissionDeny)

// Path restrictions
pm.SetPathRestriction(permission.PathRestriction{
    Pattern:   ".env",
    Action:    permission.PermissionDeny,
    ToolMatch: "file", // Only applies to file tool
})
pm.SetPathRestriction(permission.PathRestriction{
    Pattern: "*.key",
    Action:  permission.PermissionDeny,
})

// Check permissions
perm := pm.CheckAgentToolAndPath("explorer", "file", ".env/my密钥.pem")
// Returns: PermissionDeny
```

## Dependencies

- M1: Query Engine (can start after M1)

## Next Steps

- [M8: Prompt Architecture](../M8-prompt-architecture/)
- [M9: Telemetry](../M9-telemetry/)
- [M10: Kairos Memory](../M10-kairos-memory/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-7-permission-engine)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m7-permission-engine)