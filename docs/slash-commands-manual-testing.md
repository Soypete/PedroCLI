# Slash Commands Manual Testing Guide

## Prerequisites

1. Build the binaries:
```bash
make build
```

2. Ensure you have some command files in `.pedro/command/`:
```bash
ls .pedro/command/
# Should show: blog-outline.md, test.md, lint.md
```

## Test Scenarios

### 1. Start REPL and Check Help

```bash
./pedrocode
```

Once in REPL:
```
pedro> /help
```

**Expected:**
- Help text shows REPL commands
- Help text mentions "Slash Commands" section
- Examples include `/blog-outline`, `/test`, `/lint`

### 2. Test Slash Command Discovery

```
pedro> /unknown-command
```

**Expected:**
- Error message: "Command not found: /unknown-command"
- Shows list of available commands (builtin + custom)
- Suggests typing `/help` for REPL commands

### 3. Test Builtin Command

```
pedro> /status
```

**Expected:**
- Shows current agent, model, and work directory
- No prompt expansion (builtins execute directly)

### 4. Test Custom Command Without Arguments

```
pedro> /test
```

**Expected:**
1. Shows "Expanded prompt:" with a divider line
2. Prompt contains `go test` command output
3. Shows "Run with build agent? [y/n]:"
4. If you answer 'n': Shows "Cancelled"
5. If you answer 'y': Executes build agent with expanded prompt

### 5. Test Custom Command With Arguments

```
pedro> /blog-outline Building CLI Tools in Go
```

**Expected:**
1. Shows "Expanded prompt:"
2. Prompt contains "Building CLI Tools in Go" in the template
3. Shows "Run with blog agent? [y/n]:"
4. If you answer 'y': Executes blog agent

### 6. Test REPL Commands Still Work

```
pedro> /history
```

**Expected:**
- Shows command history
- Previous slash commands should be in history

```
pedro> /context
```

**Expected:**
- Shows session info (ID, mode, agent, duration, etc.)

### 7. Test Natural Language (Not Slash Command)

```
pedro> build a rate limiter for the API
```

**Expected:**
- Treated as natural language input
- Executed directly with current agent (no expansion)
- No "Expanded prompt" shown

### 8. Test Mixed Session

Full session example:

```bash
# Start REPL
./pedrocode

# Check help
pedro> /help

# Try a builtin
pedro> /status

# Try a slash command
pedro> /blog-outline Go Error Handling Best Practices
# Answer 'n' to cancel

# Try natural language
pedro> write a simple Go HTTP server

# Check history
pedro> /history

# Exit
pedro> /quit
```

## Integration Test Checklist

- [ ] REPL starts without errors
- [ ] `/help` shows updated help with slash commands section
- [ ] Builtin commands execute directly (`/status`, `/clear`, `/help`)
- [ ] Unknown slash commands show error + available commands
- [ ] Custom slash commands show expanded prompt
- [ ] Template expansion works (`$ARGUMENTS` replacement)
- [ ] Shell commands work (!`cmd` expansion) - `/test` command
- [ ] Confirmation prompt appears for commands with agents
- [ ] Answering 'n' cancels execution
- [ ] Answering 'y' executes agent
- [ ] REPL commands still work (`/history`, `/context`, `/logs`)
- [ ] Natural language input still works
- [ ] Command history includes slash commands
- [ ] Logs show slash command expansions

## Edge Cases

### Empty Arguments

```
pedro> /blog-outline
```

**Expected:**
- Expands with empty `$ARGUMENTS`
- Still asks for confirmation

### Very Long Arguments

```
pedro> /blog-outline This is a very long topic about building distributed systems with Go including microservices event-driven architecture and cloud-native patterns
```

**Expected:**
- All arguments joined in `$ARGUMENTS`
- Template expands correctly
- No truncation

### Special Characters in Arguments

```
pedro> /blog-outline Go's Error Handling: Best Practices & Patterns
```

**Expected:**
- Special characters passed through correctly
- No shell injection (commands are not executed)

### Command Not Found vs No Agent

Create a test command without an agent:

```markdown
---
description: Test without agent
---

This is a test prompt: $ARGUMENTS
```

```
pedro> /test-no-agent Hello
```

**Expected:**
- Shows expanded prompt
- Message: "No agent configured for this command"
- Suggests copying the prompt for manual use
- Does NOT execute any agent

## Performance Testing

### Large Command Output

The `/test` command executes `go test ./...` which can produce a lot of output.

**Expected:**
- Shell command executes within reasonable time
- Output is included in expanded prompt
- REPL doesn't freeze
- User can see progress

### Many Commands

Load 20+ custom commands in `.pedro/command/`.

**Expected:**
- REPL starts quickly (< 1 second)
- Command listing is fast
- No memory issues

## Debugging

If something doesn't work:

1. **Enable debug mode:**
   ```bash
   ./pedrocode --debug
   ```

2. **Check logs:**
   ```
   pedro> /logs
   ```
   Then check the session log files shown.

3. **Verify commands load:**
   ```
   pedro> /unknown
   ```
   This will list all available commands.

4. **Check working directory:**
   ```
   pedro> /status
   ```
   Verify the work directory is correct.

## Known Limitations

1. **No tab completion yet** - Must type full command names
2. **No command aliases** - Each command must be typed exactly
3. **No multi-line input for slash commands** - Must be single line
4. **Shell command execution** - Commands in !`...` run in shell, be careful with untrusted input

## Success Criteria

✅ All slash commands expand correctly
✅ Agent routing works for commands with `agent:` field
✅ Commands without agents just show expanded prompt
✅ REPL commands still work alongside slash commands
✅ Help text is updated and helpful
✅ Error messages are clear
✅ Confirmation prompts work
✅ Session logs include slash command activity
