# M7: Permission Engine

> Status: Planned | Started: TBD | Completed: TBD

## Problem

<!-- Why does this milestone exist? What problem does it solve? -->

Currently, tool access is controlled via middleware but there's no granular per-agent, per-tool, per-path permission system. The REPL has approval prompts but no unified permission engine.

## Solution

<!-- How we solved it -->

Implement a permission engine with config-driven policies:

```json
{
  "permissions": {
    "defaults": {
      "read": "allow",
      "write": "ask",
      "bash": "ask",
      "network": "deny"
    },
    "agents": {
      "explorer": { "read": "allow", "write": "deny" },
      "implementer": { "read": "allow", "write": "allow" }
    },
    "paths": {
      "deny": [".env", "*.key", "*.pem"],
      "ask": ["go.mod", "go.sum"]
    }
  }
}
```

Actions: `allow`, `deny`, `ask` (prompt user)

## Key Decisions

<!-- Important choices made during implementation - fill in as we code -->

1. **Decision**: TODO - Reason
2. **Decision**: TODO - Reason

## Files Changed

### New Files
- `pkg/orchestration/permissions.go` - PermissionEngine interface
- `pkg/orchestration/default_permissions.go` - Default implementation
- `pkg/orchestration/permissions_config.go` - Config loading

### Modified Files
- `pkg/agents/executor.go` - Check permissions before tool execution

## Dependencies

- M1: Query Engine (can start after M1)

## Next Steps

- [M2: Execution Modes](../M2-execution-modes/)

## Reference

- [Implementation Plan](../../pedrocode-vnext-implementation-plan.md#milestone-7-permission-engine)
- [Interface Definitions](../../pedrocode-vnext-interfaces.md#m7-permission-engine)