# Slash Commands Testing Strategy

## Overview

Slash commands should be **interactive-only** features in PedroCLI. They are designed for the REPL experience where users can iteratively expand prompts and see results before committing to agent execution.

## Architecture

```
User in REPL → /command args → CommandRunner → CommandRegistry → Expanded Prompt → Agent (optional)
                    ↑                                                    ↓
                    └────────────── Show to user, ask confirmation ──────┘
```

## Restriction Strategy

### Phase 1: Make Slash Commands Interactive-Only

**Goal**: Remove or restrict the `pedrocli run` command to prevent non-interactive use.

**Options**:

1. **Option A (Recommended)**: Remove `run` and `commands` from main CLI
   - Keep slash commands exclusive to REPL
   - Remove from `cmd/pedrocli/main.go`
   - Users must use `pedrocode` (REPL) to access slash commands

2. **Option B**: Keep `run` but make it explicit it's for testing only
   - Add `--allow-non-interactive` flag with warning
   - Document as internal/testing feature
   - Default behavior: error with "Use pedrocode REPL for interactive commands"

3. **Option C**: Make `run` command launch REPL in single-command mode
   - `pedrocli run /blog-outline "topic"` → launches REPL, executes command, exits
   - Still interactive (shows expanded prompt, asks for confirmation)
   - Best of both worlds

**Recommendation**: Go with **Option C** - it preserves the CLI convenience while maintaining interactivity.

### Phase 2: Integration with REPL

Update `pkg/repl/repl.go:handleSlashCommand()` to use the CommandRunner:

```go
func (r *REPL) handleSlashCommand(cmd *Command) error {
    // Create command runner
    runner := cli.NewCommandRunner(r.session.Config, r.session.WorkDir)

    // Build input string
    input := "/" + cmd.Name
    if len(cmd.Args) > 0 {
        args := make([]string, 0, len(cmd.Args))
        for i := 0; ; i++ {
            key := fmt.Sprintf("arg%d", i)
            if val, ok := cmd.Args[key].(string); ok {
                args = append(args, val)
            } else {
                break
            }
        }
        input = input + " " + strings.Join(args, " ")
    }

    // Expand command
    expanded, err := runner.RunCommand(r.ctx, input)
    if err != nil {
        return fmt.Errorf("command expansion failed: %w", err)
    }

    // Show expanded prompt
    r.output.PrintMessage("\n📝 Expanded prompt:\n")
    r.output.PrintMessage("─────────────────────────────────────────\n")
    r.output.PrintMessage("%s\n", expanded)
    r.output.PrintMessage("─────────────────────────────────────────\n\n")

    // Check if command has an agent
    name, _, _ := cli.ParseSlashCommand(input)
    command, ok := runner.GetCommand(name)

    if !ok || command.Agent == "" {
        r.output.PrintMessage("ℹ️  No agent configured for this command\n")
        r.output.PrintMessage("   Copy the expanded prompt to use manually\n")
        return nil
    }

    // Ask for confirmation
    r.output.PrintMessage("Run with %s agent? [y/n]: ", command.Agent)
    line, err := r.input.ReadLine()
    if err != nil {
        return err
    }

    response := strings.TrimSpace(strings.ToLower(line))
    if response != "y" && response != "yes" {
        r.output.PrintWarning("❌ Cancelled\n")
        return nil
    }

    // Execute with agent
    r.output.PrintMessage("\n🤖 Running %s agent...\n", command.Agent)
    return r.handleBackground(command.Agent, expanded)
}
```

## Testing Strategy

### Unit Tests

#### 1. Command Registry Tests (`pkg/commands/registry_test.go`)

Already exists! Enhance with:

```go
func TestCommandRegistry_LoadMarkdownCommands(t *testing.T) {
    // Test loading from .pedro/command/
}

func TestCommandRegistry_ExpandTemplate(t *testing.T) {
    // Test template expansion with variables
}

func TestCommandRegistry_Builtins(t *testing.T) {
    // Test /test, /lint builtins
}
```

#### 2. CLI Command Runner Tests (`pkg/cli/commands_test.go`)

Create new test file:

```go
func TestCommandRunner_ListCommands(t *testing.T)
func TestCommandRunner_ExpandCommand(t *testing.T)
func TestCommandRunner_ParseSlashCommand(t *testing.T)
func TestCommandRunner_InvalidCommands(t *testing.T)
```

#### 3. REPL Slash Command Tests (`pkg/repl/commands_test.go`)

Create new test file:

```go
func TestParseCommand_SlashCommands(t *testing.T)
func TestIsREPLCommand(t *testing.T)
func TestSlashCommandExecution(t *testing.T)
```

### Integration Tests

#### 1. REPL Integration Test

```go
// pkg/repl/integration_test.go
func TestREPL_SlashCommandWorkflow(t *testing.T) {
    // Setup REPL with mock backend
    // Send slash command: /blog-outline "test topic"
    // Verify expansion
    // Verify agent routing
}
```

#### 2. End-to-End Test

```bash
# e2e/slash_commands_test.sh

# Test 1: Verify slash commands work in REPL
echo "/test" | pedrocode --mode code --non-interactive | grep "Expanded prompt"

# Test 2: Verify slash commands are restricted in CLI (if we go with Option A/B)
pedrocli run /test 2>&1 | grep "Use pedrocode REPL"

# Test 3: Verify custom commands load from .pedro/command/
echo "/blog-outline Test Post" | pedrocode --mode blog | grep "Write a blog post"
```

### Manual Testing Checklist

- [ ] Start REPL: `pedrocode`
- [ ] List commands: `/help` (should show slash commands section)
- [ ] List all slash commands: type `/` and press Tab (autocomplete)
- [ ] Run builtin command: `/test`
- [ ] Run custom command: `/blog-outline "My Topic"`
- [ ] Verify expansion shown before execution
- [ ] Verify confirmation prompt appears
- [ ] Test with agent-linked commands (should auto-route to agent)
- [ ] Test with non-agent commands (should just show expanded prompt)

## Implementation Plan

### Step 1: Write Core Tests

```bash
# Create test files
touch pkg/cli/commands_test.go
touch pkg/repl/integration_test.go

# Run tests
go test ./pkg/cli/...
go test ./pkg/repl/...
go test ./pkg/commands/...
```

### Step 2: Implement REPL Integration

Update `pkg/repl/repl.go:handleSlashCommand()` with the code above.

### Step 3: Restrict CLI Access (Choose Option A, B, or C)

If **Option C** (recommended):
```go
// cmd/pedrocli/main.go
func runSlashCommand(cfg *config.Config, args []string) {
    // Launch single-command REPL mode
    session, _ := repl.NewSession(...)
    r, _ := repl.NewREPL(session)

    // Execute command in interactive mode
    cmd := repl.ParseCommand(strings.Join(args, " "))
    r.handleSlashCommand(cmd)
}
```

### Step 4: Documentation

Update docs:
- `README.md` - Add slash commands section
- `docs/slash-commands.md` - Comprehensive guide
- `.pedro/command/README.md` - Template guide

### Step 5: E2E Tests

Create `test/e2e/slash_commands_test.sh` with automated tests.

## Test Coverage Goals

- **Unit Tests**: 80%+ coverage for command parsing, expansion, execution
- **Integration Tests**: Cover all builtin commands + 2-3 custom examples
- **E2E Tests**: Cover REPL workflow end-to-end

## Example Test Cases

### Test Case 1: Builtin Command

```go
func TestBuiltinCommand_Test(t *testing.T) {
    runner := cli.NewCommandRunner(testConfig, "/tmp/test-project")

    expanded, err := runner.RunCommand(ctx, "/test")
    assert.NoError(t, err)
    assert.Contains(t, expanded, "go test")
    assert.Contains(t, expanded, "Report results")
}
```

### Test Case 2: Custom Command with Arguments

```go
func TestCustomCommand_BlogOutline(t *testing.T) {
    // Setup .pedro/command/blog-outline.md
    runner := cli.NewCommandRunner(testConfig, testDir)

    expanded, err := runner.RunCommand(ctx, "/blog-outline Building CLIs in Go")
    assert.NoError(t, err)
    assert.Contains(t, expanded, "Building CLIs in Go")
    assert.Contains(t, expanded, "Write a blog post")
}
```

### Test Case 3: Agent Routing

```go
func TestSlashCommand_AgentRouting(t *testing.T) {
    runner := cli.NewCommandRunner(testConfig, testDir)

    cmd, ok := runner.GetCommand("blog-outline")
    assert.True(t, ok)
    assert.Equal(t, "blog", cmd.Agent)
}
```

## Rollout Plan

1. **PR #1**: Core tests + REPL integration (no breaking changes)
2. **PR #2**: CLI restriction (breaking change - requires communication)
3. **PR #3**: Documentation + E2E tests
4. **PR #4**: Additional builtin commands (/commit, /review-pr, /search)

## Success Criteria

- ✅ All slash commands only work in REPL (or with explicit flag)
- ✅ REPL shows expanded prompts before execution
- ✅ Users can confirm/cancel before agent execution
- ✅ Custom commands load from `.pedro/command/`
- ✅ Builtin commands work without setup
- ✅ Tab completion for slash commands (future enhancement)
- ✅ 80%+ test coverage for command system
