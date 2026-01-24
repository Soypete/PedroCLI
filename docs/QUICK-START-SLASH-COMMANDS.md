# Quick Start: Testing Slash Commands in REPL

## 🚀 Quick Setup (30 seconds)

```bash
# Build the binaries
make build

# Start the REPL
./pedrocode
```

## ✅ 5-Minute Test Checklist

### 1. Check Help (Shows slash commands section)

```
pedro> /help
```

**✅ Expected:** Help text includes "Slash Commands" section with examples

---

### 2. List Available Commands (Error shows all commands)

```
pedro> /unknown
```

**✅ Expected:**
- Error: "Command not found: /unknown"
- Lists available commands (builtin + custom)
- Shows: `/help`, `/status`, `/clear`, `/test`, `/blog-outline`, `/lint`

---

### 3. Try a Builtin Command (Direct execution)

```
pedro> /status
```

**✅ Expected:**
- Shows current agent, model, work directory
- No prompt expansion (executes directly)

---

### 4. Try Slash Command WITHOUT Arguments

```
pedro> /test
```

**✅ Expected:**
1. Shows "📝 Expanded prompt:" header
2. Prompt contains output from `go test ./...`
3. Shows "Run with build agent? [y/n]:"
4. Type `n` → Shows "❌ Cancelled"

---

### 5. Try Slash Command WITH Arguments

```
pedro> /blog-outline Building Microservices in Go
```

**✅ Expected:**
1. Shows "📝 Expanded prompt:" header
2. Prompt includes "Building Microservices in Go"
3. Shows "Run with blog agent? [y/n]:"
4. Type `n` → Shows "❌ Cancelled"

---

### 6. Check Command History

```
pedro> /history
```

**✅ Expected:** Shows all previous commands including slash commands

---

### 7. Natural Language Still Works

```
pedro> what is the current agent?
```

**✅ Expected:** Executes with current agent (no expansion shown)

---

### 8. REPL Commands Still Work

```
pedro> /context
```

**✅ Expected:** Shows session info (ID, mode, agent, duration)

```
pedro> /clear
```

**✅ Expected:** Clears the screen

---

### 9. Exit

```
pedro> /quit
```

**✅ Expected:** Exits cleanly with "Goodbye!" message

---

## 🎯 Advanced: Execute an Agent

### Full Flow Test

```bash
# Start REPL
./pedrocode

# Try a slash command and execute it
pedro> /blog-outline Go Context Best Practices

# Review the expanded prompt
# Type 'y' when asked "Run with blog agent? [y/n]:"

# Watch agent execute
# Check results
```

**✅ Expected:**
1. Expanded prompt shown
2. Confirmation requested
3. Agent executes with expanded prompt
4. Job created and runs
5. Results displayed

---

## 🐛 Troubleshooting

### Commands not found?

```bash
# Check commands directory exists
ls .pedro/command/

# Should show:
# blog-outline.md
# test.md
# lint.md
```

If missing, commands are in this repo at `.pedro/command/`

### Slash command shows wrong output?

Check working directory:
```
pedro> /status
```

Should show correct `Work Dir`. Commands load from `.pedro/command/` relative to work dir.

### Want to see debug output?

```bash
# Start with debug mode
./pedrocode --debug

# Check logs
pedro> /logs
```

---

## 📊 Success Criteria

After testing, you should have verified:

- ✅ `/help` shows slash commands section
- ✅ Unknown slash commands show helpful error
- ✅ Builtin commands execute directly
- ✅ Custom slash commands show expanded prompts
- ✅ Template expansion works (`$ARGUMENTS` replaced)
- ✅ Shell commands execute (!`go test ./...`)
- ✅ Confirmation prompts work
- ✅ Can cancel execution
- ✅ Can execute with agent
- ✅ REPL commands still functional
- ✅ Natural language still works
- ✅ History includes slash commands
- ✅ Session logs capture expansions

---

## 🔍 What to Look For

### Good Signs ✅
- Expanded prompts show before execution
- Clear divider lines around expanded prompts
- Confirmation prompts appear for commands with agents
- Error messages are helpful and actionable
- Commands load from `.pedro/command/`
- Template variables get replaced correctly

### Bad Signs ❌
- Raw template variables in output (e.g., `$ARGUMENTS` not replaced)
- No confirmation prompt when agent configured
- Commands not loading from `.pedro/command/`
- REPL commands stopped working
- Error messages are cryptic
- Agent executes without showing expanded prompt

---

## 📝 Example Session

```
$ ./pedrocode

╔═══════════════════════════════════════════╗
║   pedrocode - Interactive Coding Agent   ║
╚═══════════════════════════════════════════╝

Mode: code
Agent: build

Type /help for available commands
Type /quit to exit

pedro> /help
[Help text appears with slash commands section]

pedro> /test

📝 Expanded prompt:
─────────────────────────────────────────
Run the full test suite with coverage:

[test output from `go test -v -cover ./...`]

Analyze the test results above:
1. Identify any failing tests
2. Suggest fixes for failures
3. Highlight any tests with low coverage
4. Note any slow tests (>1s)
─────────────────────────────────────────

Run with build agent? [y/n]: n
❌ Cancelled

pedro> /blog-outline Effective Go Error Handling

📝 Expanded prompt:
─────────────────────────────────────────
Create a detailed blog outline about: Effective Go Error Handling

Generate a structured outline with:
1. A compelling title
2. TLDR summary (3-5 bullet points)
3. Main sections (4-8 sections) with:
   - Section title
   - Key points to cover
   - Estimated word count
4. Conclusion section
5. Call-to-action suggestions

Format the outline in Markdown.
─────────────────────────────────────────

Run with blog agent? [y/n]: y

🤖 Running blog agent...

[Agent executes and shows progress]

pedro> /history
Command History:
  1: /help
  2: /test
  3: /blog-outline Effective Go Error Handling
  4: /history

pedro> /quit

Goodbye!
```

---

## 🎓 Next Steps

### Create Your Own Command

1. Create `.pedro/command/my-command.md`:

```markdown
---
description: My custom command
agent: build
---

Do something with: $ARGUMENTS

Explain it clearly and provide examples.
```

2. Test it:

```
pedro> /my-command testing this
```

### Template Syntax

- `$ARGUMENTS` - All arguments joined
- `$1, $2, $3` - Individual arguments
- `@file.txt` - Include file contents
- !`command` - Run shell command

### Learn More

- Full testing guide: `docs/slash-commands-manual-testing.md`
- Architecture: `docs/slash-commands-architecture.md`
- Strategy: `docs/slash-commands-testing-strategy.md`

---

## 💡 Tips

1. **Start simple** - Try builtin commands first
2. **Read expansions** - Always review before executing
3. **Cancel freely** - Press 'n' if unsure
4. **Check logs** - Use `/logs` when debugging
5. **Keep commands small** - One task per command

---

## ✨ That's It!

You now have slash commands working in the REPL! They provide:
- **Interactive exploration** - See what will run before it runs
- **Reusable prompts** - Save common workflows as commands
- **Template power** - Dynamic prompts with arguments and shell commands
- **Agent routing** - Commands know which agent to use

Happy coding! 🚀
