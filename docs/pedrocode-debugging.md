# pedrocode Debugging Guide

## Overview

pedrocode includes a comprehensive logging system to help debug agent behavior, tool execution, and LLM interactions.

## Enabling Debug Mode

Start pedrocode with the `--debug` (or `-d`) flag:

```bash
./pedrocode --debug          # Debug mode in code mode (default)
./pedrocode --debug blog     # Debug mode in blog mode
./pedrocode -d podcast       # Short form (-d)
```

## What Debug Mode Does

1. **Verbose Logging** - Logs all interactions to files
2. **LLM Request Logging** - Captures full LLM API requests/responses
3. **Keeps Logs** - Doesn't auto-clean logs on exit
4. **Shows Log Path** - Displays log directory at startup

## Log Files

When debug mode is enabled, logs are saved to:

```
/tmp/pedrocode-sessions/<session-id>/
├── session.log         # Full REPL transcript
├── agent-calls.log     # Agent execution details
├── tool-calls.log      # Tool execution details
└── llm-requests.log    # LLM API calls (debug only)
```

### session.log

Full transcript of the REPL session:
- User inputs (>>> prefix)
- System outputs (<<< prefix)
- Session metadata
- Errors

Example:
```
Session started: code-a1b2c3d4-20240122-195430
Debug mode: true
Keep logs: true
Log directory: /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430

>>> add rate limiting to the API
<<< Starting build job...
<<< Job job-123 started
```

### agent-calls.log

Timestamped log of agent execution:
- Agent selected
- User prompts
- Agent responses
- Execution results

Example:
```
[19:54:35.123] Agent: build
[19:54:35.124] Prompt: add rate limiting to the API
[19:54:40.567] Result: Rate limiter implemented in middleware/ratelimit.go
```

### tool-calls.log

Timestamped log of tool execution:
- Tool name
- Arguments
- Results
- Success/failure status

Example:
```
[19:54:36.789] Tool: code_search
[19:54:36.790] Args: {"pattern":"middleware","path":"."}
[19:54:36.850] Result: Found 3 matches in pkg/middleware/
[19:54:36.851] Success: true
```

### llm-requests.log (debug only)

Full LLM API interactions:
- Request payloads
- Response bodies
- Token counts
- Timing information

Example:
```
[19:54:36.900] LLM Request:
{
  "model": "qwen2.5-coder:32b",
  "messages": [...],
  "temperature": 0.2
}

[19:54:40.123] LLM Response:
{
  "content": "...",
  "tokens": 1250
}
```

## Viewing Logs

### During Session

When debug mode is enabled, the welcome message shows the log directory:

```
╔═══════════════════════════════════════════╗
║   pedrocode - Interactive Coding Agent   ║
╚═══════════════════════════════════════════╝

Mode: code
Agent: build

🐛 Debug mode enabled
📁 Logs: /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430
   - session.log       : Full transcript
   - agent-calls.log   : Agent execution details
   - tool-calls.log    : Tool execution details
   - llm-requests.log  : LLM API calls

Logs will be kept after exit for debugging
```

### During Session (Live Tailing)

Open a new terminal and tail the logs while the REPL is running:

```bash
# Get the log directory from the welcome message
LOG_DIR="/tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430"

# Tail specific log
tail -f $LOG_DIR/agent-calls.log

# Tail all logs
tail -f $LOG_DIR/*.log
```

### After Session

Logs are kept in `/tmp/pedrocode-sessions/` when debug mode is enabled:

```bash
# List all sessions
ls -lh /tmp/pedrocode-sessions/

# View recent session
cat /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430/session.log

# Search across sessions
grep "error" /tmp/pedrocode-sessions/*/session.log
```

## Normal Mode (No Debug)

Without `--debug`, pedrocode still logs but:
- Only basic logging (no LLM requests)
- Logs are auto-cleaned on `/quit` or exit
- No log directory shown at startup

Logs are still created temporarily in `/tmp/pedrocode-sessions/` but are removed when you exit.

## Log Cleanup

### Automatic Cleanup

- **Normal mode**: Logs deleted on exit
- **Debug mode**: Logs kept indefinitely
- **Old sessions**: Auto-deleted after 24 hours (both modes)

### Manual Cleanup

```bash
# Remove all sessions
rm -rf /tmp/pedrocode-sessions/

# Remove specific session
rm -rf /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430

# Remove old sessions (24+ hours)
find /tmp/pedrocode-sessions -type d -mtime +1 -exec rm -rf {} +
```

## Debugging Workflow

### 1. Reproduce with Debug Enabled

```bash
./pedrocode --debug

pedro:build> add authentication middleware
[observe behavior]

/quit
```

### 2. Check Logs

```bash
# Find the session directory (shown in welcome message)
LOG_DIR="/tmp/pedrocode-sessions/code-..."

# Check agent behavior
cat $LOG_DIR/agent-calls.log

# Check tool execution
cat $LOG_DIR/tool-calls.log

# Check LLM requests (if needed)
cat $LOG_DIR/llm-requests.log
```

### 3. Identify Issue

Look for:
- **Errors** in session.log
- **Failed tool calls** in tool-calls.log
- **Unexpected agent responses** in agent-calls.log
- **Invalid LLM requests/responses** in llm-requests.log

### 4. Share Logs

When reporting issues, include relevant log snippets:

```bash
# Get last 50 lines of agent log
tail -50 $LOG_DIR/agent-calls.log > issue-log.txt

# Or compress entire session
tar -czf session-logs.tar.gz $LOG_DIR/
```

## Common Debug Scenarios

### Agent Not Responding

Check agent-calls.log for:
- Agent selection
- Prompt received
- Any error messages

### Tool Execution Failures

Check tool-calls.log for:
- Tool name and arguments
- Error messages
- Success/failure status

### LLM Issues

Check llm-requests.log for:
- Request format
- Token counts
- Response structure
- API errors

### Session Crashes

Check session.log for:
- Last command executed
- Any error messages
- Stack traces

## Best Practices

1. **Always use --debug when testing** - Keeps logs for analysis
2. **Tail logs in separate terminal** - See real-time activity
3. **Note the session ID** - Easy to find logs later
4. **Clean up old logs** - Prevent /tmp from filling up
5. **Include logs when reporting bugs** - Helps maintainers debug

## Example Debug Session

```bash
# Terminal 1: Start with debug
./pedrocode --debug

╔═══════════════════════════════════════════╗
║   pedrocode - Interactive Coding Agent   ║
╚═══════════════════════════════════════════╝

Mode: code
Agent: build

🐛 Debug mode enabled
📁 Logs: /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430

# Terminal 2: Tail logs
tail -f /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430/*.log

# Terminal 1: Execute command
pedro:build> add rate limiting to the API
[watch output in both terminals]

# Terminal 2: See real-time logs
[19:54:36.789] Tool: code_search
[19:54:36.790] Args: {"pattern":"middleware","path":"."}
...

# Terminal 1: Exit
pedro:build> /quit

# Terminal 3: Analyze logs
cd /tmp/pedrocode-sessions/code-a1b2c3d4-20240122-195430
ls -lh
cat agent-calls.log
```

## Environment Variables

You can also enable debug via environment variable:

```bash
export PEDROCODE_DEBUG=1
./pedrocode  # Debug mode enabled
```

## Configuration

Debug settings can be configured in `.pedrocli.json`:

```json
{
  "debug": {
    "enabled": true,           // Enable debug by default
    "keep_logs": true,         // Keep logs after exit
    "log_llm_requests": true   // Log LLM API calls
  }
}
```

## See Also

- [pedrocode REPL Guide](./pedrocode-repl.md)
- [CLAUDE.md](../CLAUDE.md) - Main documentation
- [Architecture Overview](./architecture.md)
