# ADR-008: Dual File Editing Strategy (CLI Direct vs HTTP Bridge Isolated)

**Status:** Accepted

**Date:** 2026-01-10

**Authors:** @soypete, Claude Sonnet 4.5

## Context

PedroCLI has two execution modes with fundamentally different requirements:

### CLI Mode
- **Environment:** Single-user local development
- **Concurrency:** Synchronous (one task at a time)
- **Safety:** User controls the environment
- **Speed:** Critical (developers expect fast iteration)

### HTTP Bridge Mode
- **Environment:** Multi-user web/API server
- **Concurrency:** Multiple concurrent jobs
- **Safety:** Jobs must not interfere with each other
- **Isolation:** Different repos, branches, users

The original implementation used a single approach (direct repository editing) which:
- ✅ Worked great for CLI mode (fast, simple)
- ❌ Failed for HTTP bridge (concurrent jobs conflicted)
- ❌ Limited to Go file tools only (no regex support)
- ❌ Prevented multi-file transformations

### Problem Statement

We needed a solution that:
1. Preserves CLI simplicity and speed
2. Enables concurrent job isolation for HTTP bridge
3. Provides both simple (Go tools) and powerful (bash sed/awk) file editing
4. Supports SSH URLs for password-less git authentication
5. Reuses workspaces to minimize cloning overhead

## Decision

We implemented a **dual file editing strategy** with mode-specific workflows and tool variants:

### 1. Workflow Strategy

**CLI Workflow (Direct Repository Editing):**
```
┌──────────────────────────────────────┐
│ Current Repository                   │
│ /Users/dev/code/my-project          │
├──────────────────────────────────────┤
│ 1. Create branch: feat/feature-name  │
│ 2. Edit files in place               │
│ 3. Commit changes                    │
│ 4. Push using SSH URL                │
│ 5. Create PR via gh CLI              │
└──────────────────────────────────────┘
```

**HTTP Bridge Workflow (Isolated Workspaces):**
```
┌────────────────────────────────────────────────┐
│ Isolated Workspace                             │
│ ~/.cache/pedrocli/jobs/<job-id>/workspace/    │
├────────────────────────────────────────────────┤
│ 1. Check if already cloned:                   │
│    - Exists: git fetch && git pull            │
│    - New: git clone <ssh-url> workspace/      │
│ 2. Create branch: feat/feature-name           │
│ 3. Edit files in workspace                    │
│ 4. Commit changes                             │
│ 5. Push from workspace using SSH              │
│ 6. Create PR via gh CLI                       │
│ 7. Optional cleanup (configurable)            │
└────────────────────────────────────────────────┘
```

### 2. Tool Strategy

Expose **BOTH** Go library tools AND bash file editing tools:

**Go Tools (Cross-Platform, Safe):**
- `FileTool`: Simple string replacements, read/write files
- `CodeEditTool`: Precise line-based editing, preserves indentation

**Bash Tools (Powerful, Platform-Specific):**
- `BashExploreTool`: grep, find for searching (Analyze/Plan phases)
- `BashEditTool`: sed, awk for complex regex and multi-file edits (Implement/Validate phases)

### 3. Phase-Specific Tool Access

```
Analyze Phase   → bash_explore (grep, find)   → Search, no editing
Plan Phase      → bash_explore (grep, find)   → Search, no editing
Implement Phase → bash_edit (sed, awk)        → Edit, use SearchTool for searching
Validate Phase  → bash_edit (sed, awk)        → Edit, run tests/linters
Deliver Phase   → git, github                 → Commit, push, PR
```

### 4. Workspace Management

**HTTP Bridge Only:**
- Base path: `~/.cache/pedrocli/jobs/` (XDG Base Directory spec)
- Per-job isolation: Each job gets `~/.cache/pedrocli/jobs/<job-id>/workspace/`
- Workspace reuse: Check for `.git`, do `git fetch && git pull` if exists
- SSH by default: Auto-convert HTTPS URLs to SSH format
- Configurable cleanup: `cleanup_on_complete` flag (default: false)

### 5. Configuration

```json
{
  "http_bridge": {
    "workspace_path": "~/.cache/pedrocli/jobs",
    "cleanup_on_complete": false
  },
  "tools": {
    "allowed_bash_commands": ["git", "gh", "go", "cat", "ls", "head", "tail", "wc", "sort", "uniq", "sed", "awk", "tee", "cut", "tr"],
    "forbidden_commands": ["grep", "find", "rm", "mv", "dd", "sudo", "xargs"]
  }
}
```

Note: grep/find are allowed in `bash_explore` but forbidden in `bash_edit` to enforce use of SearchTool during implementation.

## Consequences

### Positive

#### CLI Workflow
- ✅ **Fast**: No cloning overhead, work directly in repo
- ✅ **Simple**: Straightforward mental model
- ✅ **Familiar**: Standard git workflow developers expect
- ✅ **Visible**: Changes immediately visible in working directory

#### HTTP Bridge Workflow
- ✅ **Isolated**: Jobs don't interfere with each other
- ✅ **Concurrent**: Multiple jobs can run simultaneously
- ✅ **Safe**: Each job has its own clean workspace
- ✅ **Cached**: Workspaces reused (git pull vs re-clone saves time)
- ✅ **Debuggable**: Preserved workspaces enable inspection after completion

#### Tool Flexibility
- ✅ **Simple cases**: Use Go tools (file, code_edit) for cross-platform safety
- ✅ **Complex cases**: Use bash tools (sed, awk) for regex and multi-file transforms
- ✅ **Agent choice**: Agents select the best tool for each operation
- ✅ **LSP integration**: Type checking and diagnostics catch errors immediately

#### SSH URLs
- ✅ **Password-less**: SSH keys enable automated git operations
- ✅ **Secure**: No token management required
- ✅ **Standard**: Industry best practice
- ✅ **Auto-conversion**: Works with HTTPS URLs (auto-converts)

### Negative

#### Complexity
- ❌ **Two workflows**: Developers must understand both modes
- ❌ **Workspace management**: Additional HTTP bridge infrastructure
- ❌ **Tool variants**: Three bash tool types (bash, bash_explore, bash_edit)

#### Disk Usage
- ❌ **Workspace accumulation**: Workspaces grow over time if cleanup disabled
- ❌ **Monitoring required**: Need to track disk usage
- ⚠️ Mitigation: Configurable cleanup, manual cleanup commands

#### Platform Differences
- ❌ **sed/awk behavior**: macOS vs Linux differences
- ⚠️ Mitigation: Document platform differences, test on both

### Neutral

- ℹ️ **Debugging**: Preserved workspaces helpful for debugging
- ℹ️ **SSH setup**: Users must configure SSH keys (one-time setup)

## Implementation Details

### Core Components

1. **WorkspaceManager** (`pkg/httpbridge/workspace.go`)
   - `SetupWorkspace()`: Clone or update workspace
   - `ConvertToSSH()`: HTTPS → SSH URL conversion
   - `CreateBranchInWorkspace()`: Feature branch creation
   - `PushAndCreatePR()`: Push and create GitHub PR
   - `CleanupWorkspace()`: Optional workspace deletion

2. **Bash Tool Variants** (`pkg/tools/bash.go`)
   - `BashTool`: Original tool (deprecated for phases)
   - `BashExploreTool`: grep, find allowed; sed, awk forbidden
   - `BashEditTool`: sed, awk allowed; grep, find forbidden

3. **Phase Definitions** (`pkg/agents/*_phased.go`)
   - Analyze/Plan phases: Use `bash_explore`
   - Implement/Validate phases: Use `bash_edit`
   - Tools specified by string name in phase definitions

4. **AppContext Integration** (`pkg/httpbridge/app.go`)
   - Register WorkspaceManager
   - Initialize all tool variants
   - Register tools with agents

### Testing Strategy

1. **Unit Tests**
   - WorkspaceManager SSH URL conversion
   - Bash tool command validation (allowed/forbidden)

2. **Integration Tests (Planned)**
   - CLI workflow: Issue #32 (Prometheus endpoints)
   - HTTP bridge workflow: Issue #39 (slog migration across 17 files)

3. **Manual Validation**
   - Workspace reuse (git pull vs clone)
   - SSH URL usage (verify remote URLs)
   - Tool selection (agents choose appropriately)

## Alternatives Considered

### Alternative 1: Single Workspace for All Jobs
**Rejected:** Concurrent jobs would conflict

### Alternative 2: Always Re-Clone (No Workspace Reuse)
**Rejected:** Wasteful, slow for repeated jobs on same repo

### Alternative 3: Only Go Tools (No Bash sed/awk)
**Rejected:** Limited functionality, no regex support, can't do multi-file transforms efficiently

### Alternative 4: Only Bash Tools (No Go Tools)
**Rejected:** Platform-specific, harder to debug, less structured

### Alternative 5: HTTPS URLs with Tokens
**Rejected:** Token management complexity, less secure, not industry standard

### Alternative 6: Single Bash Tool (No Phase Variants)
**Rejected:** Exploration phases could accidentally edit, implementation phases could inefficiently search

## Future Considerations

### Short Term
1. Monitor disk usage metrics
2. Add workspace size limits (prevent disk exhaustion)
3. Implement workspace cleanup cron job

### Long Term
1. Workspace caching layer (share clones across jobs for same repo)
2. Remote workspace support (run jobs on different machines)
3. Workspace snapshots (save/restore workspace state)

## References

- [File Editing Strategy Documentation](../file-editing-strategy.md)
- [WorkspaceManager Implementation](../../pkg/httpbridge/workspace.go)
- [Bash Tool Variants](../../pkg/tools/bash.go)
- [Builder Phased Agent](../../pkg/agents/builder_phased.go)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)

## Related ADRs

- ADR-002: Dynamic Prompt Generation
- ADR-003: Dynamic Blog Agent Architecture
- ADR-007: Model-Specific Tool Formatting

## Approval

**Decision approved by:** @soypete

**Implementation status:** ✅ Complete

**Validation status:** ⏳ In Progress (waiting for Issue #32 and #39 tests)
