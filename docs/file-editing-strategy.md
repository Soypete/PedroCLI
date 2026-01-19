# File Editing Strategy

PedroCLI supports two distinct workflows for file editing: **CLI Direct Editing** and **HTTP Bridge Isolated Workspaces**. Additionally, agents have access to both **Go-based file tools** and **Bash-based file editing tools** (sed/awk).

## CLI vs HTTP Bridge Workflows

### CLI Workflow (Direct Repository Editing)

**When to use:**
- Single-user development environment
- Working on your local machine
- Testing changes before committing
- Iterating quickly on features

**How it works:**
```
1. Run CLI command: ./pedrocli build -description "Add feature X"
2. Agent works directly in current repository (cfg.Project.Workdir)
3. Creates feature branch: feat/<description>
4. Makes file edits in place
5. Commits changes to current repo
6. Pushes branch using SSH (auto-converts HTTPS → SSH)
7. Creates PR via gh CLI
```

**Key characteristics:**
- **Fast**: No cloning overhead
- **Simple**: Work directly where you are
- **Synchronous**: One task at a time
- **Git remote**: Automatically uses SSH URLs

**Example:**
```bash
cd /path/to/my-project
./pedrocli build -issue 32 -description "Add Prometheus metrics"

# Agent edits files in /path/to/my-project
# Creates branch feat/32-prometheus-metrics
# git push -u origin feat/32-prometheus-metrics
# gh pr create --title "..." --body "..."
```

### HTTP Bridge Workflow (Isolated Workspaces)

**When to use:**
- Multi-user environment (web UI, API)
- Concurrent jobs on different repos
- Production/server deployments
- Want isolation between jobs

**How it works:**
```
1. HTTP API request: POST /api/jobs {"type": "build", ...}
2. WorkspaceManager creates isolated workspace: ~/.cache/pedrocli/jobs/<job-id>/
3. Checks if repo already cloned:
   - If .git exists: git fetch && git pull (reuse workspace)
   - If not: git clone <ssh-url> workspace/ (fresh clone)
4. Agent works in isolated workspace
5. Creates feature branch in workspace
6. Makes file edits in workspace
7. Commits and pushes from workspace
8. Creates PR via gh CLI
9. Optional: Cleanup workspace after PR created (configurable)
```

**Key characteristics:**
- **Isolated**: Each job has its own workspace
- **Concurrent**: Multiple jobs can run simultaneously
- **Cached**: Workspaces are reused (git pull vs re-clone)
- **Configurable cleanup**: Preserve or delete after completion
- **Git remote**: Automatically uses SSH URLs

**Example:**
```bash
# Start HTTP server
./pedrocli-http-server

# Submit job via API
curl -X POST http://localhost:8080/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "build",
    "description": "Migrate to slog logging",
    "issue_number": "39"
  }'

# Agent works in ~/.cache/pedrocli/jobs/<job-id>/workspace/
# Edits happen in isolation
# PR created from workspace branch
```

**Workspace lifecycle:**
```
Job Created
   ↓
SetupWorkspace()
   ↓
Check ~/.cache/pedrocli/jobs/<job-id>/workspace/.git
   ↓
┌─────────────────┬─────────────────┐
│   Exists        │   Not Exists    │
│   ↓             │   ↓             │
│ git fetch       │ git clone       │
│ git pull        │ <ssh-url>       │
└─────────────────┴─────────────────┘
   ↓
Work in workspace
   ↓
Create branch, edit files, commit
   ↓
Push branch to remote
   ↓
Create PR
   ↓
Job Complete
   ↓
CleanupWorkspace() [if cleanup_on_complete: true]
   ↓
Delete ~/.cache/pedrocli/jobs/<job-id>/ [optional]
```

## Configuration

### HTTP Bridge Config (.pedrocli.json)

```json
{
  "http_bridge": {
    "workspace_path": "~/.cache/pedrocli/jobs",
    "cleanup_on_complete": false
  }
}
```

**Fields:**
- `workspace_path`: Base directory for all job workspaces (default: `~/.cache/pedrocli/jobs`)
- `cleanup_on_complete`: Delete workspace after successful PR creation (default: `false`)

**When cleanup happens:**
- Only when `cleanup_on_complete: true`
- After Deliver phase completes successfully (PR created)
- Entire job directory deleted: `~/.cache/pedrocli/jobs/<job-id>/`
- Cleanup failures don't block job completion (logged as warnings)

**Manual cleanup:**
```bash
# List all workspaces
ls -la ~/.cache/pedrocli/jobs/

# Delete all workspaces
rm -rf ~/.cache/pedrocli/jobs/*

# Delete specific workspace
rm -rf ~/.cache/pedrocli/jobs/job-1234567890-20260110-120000/
```

## File Editing Tools

Agents have access to **three categories** of file editing tools:

### 1. Go File Tool (Cross-Platform, Simple)

**Tool name:** `file`

**Best for:**
- Simple string replacements
- Creating new files
- Reading files
- Appending content
- Full file rewrites

**Actions:**
- `read`: Read file contents
- `write`: Write entire file (create or overwrite)
- `replace`: Find/replace string (must be unique)
- `append`: Add content to end of file
- `delete`: Delete file

**Examples:**
```json
// Simple replacement
{"tool": "file", "args": {"action": "replace", "path": "config.go", "old": "Port: 8080", "new": "Port: 8081"}}

// Create new file
{"tool": "file", "args": {"action": "write", "path": "pkg/metrics/metrics.go", "content": "package metrics\n..."}}

// Read before editing
{"tool": "file", "args": {"action": "read", "path": "main.go"}}
```

**Advantages:**
- Cross-platform (works everywhere)
- Structured results
- Safe (validates inputs)
- Simple API

**Limitations:**
- Replacement must be unique (only first match if not using replace_all)
- No regex support
- No multi-file operations

### 2. Code Edit Tool (Precise, Line-Based)

**Tool name:** `code_edit`

**Best for:**
- Precise line-based edits
- Preserving indentation
- Surgical changes
- Inserting at specific line numbers
- Deleting specific lines

**Actions:**
- `get_lines`: Read specific line range
- `edit_lines`: Replace line range with new content
- `insert_at_line`: Insert content at line number
- `delete_lines`: Delete line range

**Examples:**
```json
// Edit specific lines
{"tool": "code_edit", "args": {"action": "edit_lines", "path": "handler.go", "start_line": 42, "end_line": 45, "new_content": "if err != nil {\n    return fmt.Errorf(\"handler error: %w\", err)\n}\n"}}

// Insert at line
{"tool": "code_edit", "args": {"action": "insert_at_line", "path": "server.go", "line": 100, "content": "// New middleware\nfunc LoggingMiddleware() {}\n"}}

// Delete lines
{"tool": "code_edit", "args": {"action": "delete_lines", "path": "deprecated.go", "start_line": 10, "end_line": 50}}
```

**Advantages:**
- Preserves indentation
- Line numbers are explicit (easier to reason about)
- Surgical precision
- Great for adding/removing code blocks

**Limitations:**
- Single file only
- Need to know exact line numbers
- Not good for pattern-based changes

### 3. Bash Edit Tool (Regex, Multi-File)

**Tool name:** `bash_edit`

**Best for:**
- Complex regex find/replace patterns
- Multi-file transformations (same change across many files)
- Stream editing (sed)
- Field-based text processing (awk)

**Available commands:**
- `sed`: Stream editor for regex-based replacements
- `awk`: Field/column-based text processing
- `tee`, `cut`, `tr`: Text manipulation utilities
- Build/test commands: `go`, `git`, `gh`, `npm`, etc.

**Examples:**
```json
// Multi-file regex replacement
{"tool": "bash_edit", "args": {"command": "sed -i 's/fmt\\.Printf(/slog.Info(/g' pkg/**/*.go"}}

// Complex regex with capture groups
{"tool": "bash_edit", "args": {"command": "sed -i 's/oldFunc(\\([^)]*\\))/newFunc(\\1, nil)/g' pkg/tools/*.go"}}

// Field extraction with awk
{"tool": "bash_edit", "args": {"command": "awk '{print $1, $3}' data.txt > output.txt"}}

// Run formatter
{"tool": "bash_edit", "args": {"command": "goimports -w pkg/**/*.go"}}
```

**Advantages:**
- Regex support (complex patterns)
- Multi-file operations (wildcards)
- Powerful stream editing
- Unix text processing tools

**Limitations:**
- Platform-specific behavior (macOS vs Linux sed differences)
- Less structured (command-line strings)
- Harder to debug than Go tools
- Forbidden commands: grep, find (use SearchTool instead)

## Tool Selection Decision Tree

```
Need to edit files?
│
├─ Simple string replacement (unique)?
│  └─► Use `file` tool (replace action)
│     Example: Change port number, update version string
│
├─ Precise line-based edit?
│  └─► Use `code_edit` tool
│     Example: Add null check, modify function signature
│
├─ Complex regex pattern?
│  └─► Use `bash_edit` with sed
│     Example: Change all fmt.Printf to slog.Info
│
├─ Multi-file transformation (same change in many files)?
│  └─► Use `bash_edit` with sed + wildcards
│     Example: Rename function across 20 files
│
└─ Field/column processing?
   └─► Use `bash_edit` with awk
      Example: Extract specific columns from CSV
```

## Phase-Specific Tool Access

Different workflow phases have access to different bash tool variants:

### Explore Phases (Analyze, Plan)
- **Tool:** `bash_explore`
- **Allowed:** grep, find (for searching)
- **Forbidden:** sed, awk (no editing in exploration)

### Edit Phases (Implement, Validate)
- **Tool:** `bash_edit`
- **Allowed:** sed, awk, tee, cut, tr (for editing)
- **Forbidden:** grep, find (use SearchTool instead)

This separation ensures:
- Exploration phases can search but not edit
- Implementation phases can edit but should use SearchTool for searching
- Clear separation of concerns

## LSP Diagnostics (Format Awareness)

**CRITICAL:** Always run LSP diagnostics after file edits to catch errors immediately.

### Workflow

```
1. Edit file with any tool (file, code_edit, bash_edit)
2. Run LSP diagnostics to check for errors
3. If errors found:
   a. Read error messages
   b. Make another edit to fix
   c. Re-run diagnostics
4. Repeat until clean
5. Proceed to next file
```

### Example

```json
// After editing
{"tool": "code_edit", "args": {"action": "edit_lines", "path": "handler.go", "start_line": 42, "end_line": 42, "new_content": "..."}}

// Check for errors
{"tool": "lsp", "args": {"operation": "diagnostics", "file": "handler.go"}}

// If error: "unused import 'fmt'"
{"tool": "code_edit", "args": {"action": "delete_lines", "path": "handler.go", "start_line": 5, "end_line": 5}}

// Re-check
{"tool": "lsp", "args": {"operation": "diagnostics", "file": "handler.go"}}
// Clean! ✓
```

### Available LSP Operations

- `diagnostics`: Get compiler errors/warnings (use after edits)
- `definition`: Jump to symbol definition
- `references`: Find all usages
- `hover`: Get type info and documentation
- `symbols`: List file structure

## SSH URLs by Default

Both CLI and HTTP bridge workflows use **SSH URLs** for git operations by default.

### Why SSH?

- Password-less authentication with SSH keys
- More secure than HTTPS tokens
- Standard in professional environments
- No need to manage tokens/passwords

### URL Conversion

PedroCLI automatically converts HTTPS URLs to SSH format:

```
HTTPS → SSH Conversion:

https://github.com/user/repo.git     → git@github.com:user/repo.git
https://github.com/user/repo         → git@github.com:user/repo.git
https://gitlab.com/user/repo.git     → git@gitlab.com:user/repo.git
https://bitbucket.org/user/repo.git  → git@bitbucket.org:user/repo.git
git@github.com:user/repo.git         → git@github.com:user/repo.git (unchanged)
```

### CLI: Remote Update

```bash
# CLI automatically updates git remote to SSH
# Before:
git remote get-url origin
# https://github.com/user/repo.git

# After pedrocli run:
git remote get-url origin
# git@github.com:user/repo.git
```

### HTTP Bridge: Clone with SSH

```bash
# HTTP bridge clones with SSH URL automatically
# User provides: https://github.com/user/repo.git
# Agent clones: git clone git@github.com:user/repo.git workspace/
```

## Examples

### Example 1: CLI Workflow - Add Prometheus Metrics (Issue #32)

```bash
# In project directory
cd /path/to/pedrocli

# Run CLI agent
./pedrocli build -issue 32 -description "Add Prometheus observability metrics. Create pkg/metrics package with HTTP, job, LLM, and tool metrics. Instrument server.go, handlers.go, db_manager.go, executor.go, ollama.go. Add /metrics and /api/ready endpoints to HTTP bridge only (not CLI). Write tests. Create PR when done."

# Agent workflow:
# 1. Analyze: Explores codebase, finds HTTP bridge files
# 2. Plan: Creates step-by-step implementation plan
# 3. Implement:
#    - Uses file tool to create pkg/metrics/metrics.go
#    - Uses code_edit to add instrumentation to HTTP files
#    - Uses bash_edit for multi-file imports: sed -i 's/import (/import (\n\t"github.com\/soypete\/pedrocli\/pkg\/metrics"/' pkg/httpbridge/*.go
# 4. Validate: Runs tests, checks build
# 5. Deliver: Commits, creates PR

# Result:
# Branch: feat/32-prometheus-metrics
# Files edited in /path/to/pedrocli (current directory)
# PR created: https://github.com/user/repo/pull/123
```

### Example 2: HTTP Bridge Workflow - slog Migration (Issue #39)

```bash
# Start HTTP server
./pedrocli-http-server

# Submit job via API
curl -X POST http://localhost:8080/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "build",
    "description": "Migrate from fmt to slog. Replace all fmt.Print/Printf/Println with slog.Info/Warn/Error/Debug. Create centralized logger in pkg/logger/logger.go. Update all 17 files. Use structured logging fields. Create PR when done.",
    "issue_number": "39"
  }'

# Agent workflow:
# 1. SetupWorkspace: Creates ~/.cache/pedrocli/jobs/<job-id>/workspace/
#    - Checks if .git exists (first run: no)
#    - Clones: git clone git@github.com:user/repo.git workspace/
# 2. Analyze: Identifies all 17 files with fmt.Print
# 3. Plan: Creates migration plan (centralize logger, update files)
# 4. Implement:
#    - Uses file tool to create pkg/logger/logger.go
#    - Uses bash_edit for multi-file regex: sed -i 's/fmt\.Printf(/slog.Info(/g' pkg/**/*.go cmd/**/*.go
#    - Uses code_edit for specific imports and structured fields
# 5. Validate: Builds, runs tests
# 6. Deliver: Commits, pushes from workspace, creates PR

# Result:
# Workspace: ~/.cache/pedrocli/jobs/job-1234567890-20260110-120000/workspace/
# Branch: feat/39-slog-migration
# PR created: https://github.com/user/repo/pull/124
# Workspace preserved (cleanup_on_complete: false)
```

### Example 3: Workspace Reuse

```bash
# First job on repo
curl -X POST http://localhost:8080/api/jobs -d '{"type":"build","description":"Task 1"}'
# Clones: git clone git@github.com:user/repo.git workspace/

# Second job on same repo (later)
curl -X POST http://localhost:8080/api/jobs -d '{"type":"build","description":"Task 2"}'
# Reuses: cd workspace && git fetch origin && git pull
# (No re-clone, much faster!)
```

## Best Practices

### Tool Selection

1. **Start simple**: Use `file` tool for basic replacements
2. **Upgrade to precision**: Use `code_edit` when line numbers matter
3. **Scale to complexity**: Use `bash_edit` with sed/awk for regex or multi-file

### LSP Usage

1. **Always check after edits**: Run diagnostics after every file change
2. **Fix errors immediately**: Don't proceed with broken code
3. **Iterate until clean**: Re-run diagnostics until no errors

### Workspace Management

1. **Preserve workspaces**: Keep `cleanup_on_complete: false` for debugging
2. **Monitor disk usage**: Workspaces can accumulate, clean periodically
3. **SSH keys**: Ensure SSH keys are set up for password-less git operations

### Git Workflow

1. **Descriptive branches**: Use feat/, fix/, refactor/ prefixes
2. **Atomic commits**: Commit logical chunks, not entire features
3. **Clear PR descriptions**: Include context, testing, and impact

## Troubleshooting

### Workspace Issues

**Problem:** Workspace cloning fails
```
Error: failed to clone repo: exit status 128
Output: Permission denied (publickey)
```

**Solution:** Set up SSH keys
```bash
# Generate SSH key
ssh-keygen -t ed25519 -C "your_email@example.com"

# Add to ssh-agent
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519

# Add public key to GitHub
cat ~/.ssh/id_ed25519.pub
# Copy and paste to GitHub Settings → SSH Keys
```

### Disk Space

**Problem:** Running out of disk space
```bash
df -h ~/.cache/pedrocli/jobs
# 15G used, 5G available
```

**Solution:** Clean old workspaces
```bash
# List workspaces by size
du -sh ~/.cache/pedrocli/jobs/* | sort -rh

# Delete old workspaces (older than 7 days)
find ~/.cache/pedrocli/jobs -type d -mtime +7 -exec rm -rf {} \;

# Or enable auto-cleanup
# In .pedrocli.json:
{
  "http_bridge": {
    "cleanup_on_complete": true
  }
}
```

### Tool Selection Confusion

**Problem:** Not sure which tool to use

**Decision flowchart:**
```
1. Is it a simple string replacement?
   → Yes: Use `file` tool
   → No: Continue

2. Do you know the exact line numbers?
   → Yes: Use `code_edit` tool
   → No: Continue

3. Is it a regex pattern or multi-file change?
   → Yes: Use `bash_edit` with sed
   → No: Read the file, figure out approach, start with `file` tool
```

## References

- [ADR-008: Dual File Editing Strategy](../docs/adr/ADR-008-dual-file-editing.md)
- [CLAUDE.md](../CLAUDE.md) - Project instructions
- [Workspace Manager Implementation](../pkg/httpbridge/workspace.go)
- [Bash Tool Variants](../pkg/tools/bash.go)
