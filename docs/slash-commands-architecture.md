# Slash Commands Architecture

## Clear Separation: CLI Subcommands vs REPL Slash Commands

### CLI (Non-Interactive) - Traditional Subcommands

For non-interactive, script-friendly operations:

```bash
# Traditional CLI subcommands
pedrocli build -description "Add rate limiting"
pedrocli debug -symptoms "Memory leak"
pedrocli review -branch feature/new-api
pedrocli blog -file transcript.txt
pedrocli podcast script -outline outline.md
pedrocli status job-123
pedrocli list
```

**Characteristics:**
- Standard CLI flags and arguments
- Scriptable and pipeable
- Exit codes for automation
- No user interaction required
- Direct execution

### REPL (Interactive) - Slash Commands

For interactive, exploratory development:

```bash
# Start interactive session
pedrocode

# Use slash commands
pedro> /blog-outline Building CLIs in Go
pedro> /test
pedro> /lint
pedro> /commit
```

**Characteristics:**
- Interactive prompts and confirmations
- Shows expanded prompts before execution
- Tab completion (future)
- Command history
- Context-aware suggestions
- Requires user confirmation

## Template Syntax

Slash commands use a simple template system:

### Variables

```markdown
$ARGUMENTS   - All arguments joined with spaces
$1, $2, $3   - Positional arguments
```

Example:
```markdown
---
description: Generate a blog outline
agent: blog
---

Write a blog post about: $ARGUMENTS

Focus on these aspects: $1, $2, $3
```

Usage: `/blog-outline Go contexts, error handling, testing`
Expands to:
```
Write a blog post about: Go contexts, error handling, testing

Focus on these aspects: Go contexts, error handling, testing
```

### File Inclusion

```markdown
@filepath    - Include file contents
```

Example:
```markdown
Review this code:

@src/main.go

Suggest improvements
```

### Shell Command Execution

```markdown
!`command`   - Execute shell command and include output
```

Example:
```markdown
---
description: Run tests
---

Test results:

!`go test -v ./...`

Analyze the above results and suggest fixes.
```

## Testing Strategy

### Unit Tests ✅

Located in `pkg/cli/commands_test.go`:

- ✅ Command parsing
- ✅ Template expansion
- ✅ Builtin commands
- ✅ Custom markdown commands
- ✅ Error handling

Run tests:
```bash
go test ./pkg/cli/... -v
```

### Coverage

```bash
go test ./pkg/cli/... -cover
```

Current coverage: Good baseline established

### Test Structure

Similar to HTTP handler tests:
```go
func TestCommandRunner_Feature(t *testing.T) {
    // Setup
    testDir := setupTestDir(t)
    cfg := setupTestConfig()

    // Change to test directory (commands load from CWD)
    oldWd, _ := os.Getwd()
    defer os.Chdir(oldWd)
    os.Chdir(testDir)

    runner := NewCommandRunner(cfg, testDir)

    // Test cases
    t.Run("scenario", func(t *testing.T) {
        result, err := runner.RunCommand(ctx, "/command arg")
        assert.NoError(t, err)
        assert.Contains(t, result, "expected")
    })
}
```

## Integration with REPL

The REPL integration is in `pkg/repl/repl.go:handleSlashCommand()`:

**Current status:** Stub implementation (line 161-167)

**Next PR:** Replace stub with:
1. CommandRunner initialization
2. Template expansion
3. Show expanded prompt to user
4. Ask for confirmation
5. Route to agent if configured
6. Or just show expanded prompt for manual use

## Command Discovery

Commands are loaded from:

1. **Builtins** (in code):
   - `/help` - Show help
   - `/clear` - Clear conversation
   - `/undo` - Undo last change
   - `/redo` - Redo change
   - `/compact` - Compact context
   - `/status` - Show status

2. **Project commands** (`.pedro/command/*.md`):
   - `/blog-outline` - Generate blog outline
   - `/test` - Run tests
   - `/lint` - Run linters

3. **User commands** (`~/.config/pedrocli/command/*.md`):
   - User-defined custom commands

## Creating Custom Commands

### 1. Create `.pedro/command/mycommand.md`

```markdown
---
description: What this command does
agent: blog          # Optional: auto-route to agent
model: qwen2.5:32b   # Optional: use specific model
---

Your prompt template here.

Use $ARGUMENTS for all args.
Use $1, $2 for specific args.
Use @file.txt to include files.
Use !`cmd` to run shell commands.
```

### 2. Test it

```bash
# In REPL
pedro> /mycommand arg1 arg2

# Or test expansion (future)
pedrocli run /mycommand arg1 arg2 --dry-run
```

## Why This Separation?

### CLI Subcommands
- **For**: Scripts, CI/CD, automation
- **When**: You know exactly what you want
- **How**: Direct execution, no interaction

### REPL Slash Commands
- **For**: Development, exploration, learning
- **When**: You want to see what will happen first
- **How**: Interactive approval, iterative refinement

## Example Workflows

### CLI Workflow (Non-Interactive)
```bash
#!/bin/bash
# CI/CD script
pedrocli build -description "Implement OAuth" || exit 1
pedrocli test || exit 1
git commit -m "Automated build"
```

### REPL Workflow (Interactive)
```bash
$ pedrocode

pedro> /test
📝 Expanded prompt:
─────────────────────────────
Run the full test suite with coverage:

[test output]

Analyze the test results above:
1. Identify any failing tests
2. Suggest fixes for failures
...
─────────────────────────────

Run with build agent? [y/n]: y

🤖 Running build agent...
[agent executes, shows progress, asks for approval]
```

## Future Enhancements

1. **Tab completion** - Type `/` and tab to see available commands
2. **Command aliases** - `/t` → `/test`
3. **Command composition** - `/test && /lint`
4. **Dry-run mode** - See expansion without execution
5. **Command history** - Up arrow to repeat commands
6. **Smart suggestions** - Based on current context/mode

## Migration Path

For users currently using `pedrocli run /command`:

**Option 1:** Keep it for testing/development
- Add `--allow-non-interactive` flag
- Warn users to prefer `pedrocode`

**Option 2:** Make it launch REPL in single-command mode
- `pedrocli run /test` → launches REPL, runs command, exits
- Still interactive (shows expansion, asks confirmation)
- Best of both worlds

**Option 3:** Remove entirely
- Force users to use `pedrocode` for slash commands
- Cleanest separation

**Recommendation:** Option 2 - provides convenience without breaking the interactive model.
