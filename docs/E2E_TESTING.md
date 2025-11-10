# End-to-End Testing Guide

PedroCLI includes comprehensive end-to-end (E2E) tests that validate the entire system from the web server API to agent execution.

## Overview

The E2E test suite covers:
- ✅ Web server REST API endpoints
- ✅ WebSocket real-time communication
- ✅ Job creation and monitoring
- ✅ Concurrent WebSocket connections
- ✅ Static file serving
- ✅ Error handling and edge cases

## Running E2E Tests

### Environment Variable Control

E2E tests are **disabled by default** because they:
- Start actual web server processes
- Make real HTTP/WebSocket connections
- Can take longer to execute
- May require external dependencies (like Ollama)

To enable E2E tests, set the `RUN_E2E_TESTS` environment variable:

```bash
# Run all tests including E2E
RUN_E2E_TESTS=1 go test ./...

# Run only E2E tests
RUN_E2E_TESTS=1 go test ./test/e2e/...

# Run with verbose output
RUN_E2E_TESTS=1 go test -v ./test/e2e/...

# Run specific test
RUN_E2E_TESTS=1 go test -v ./test/e2e/ -run TestWebServer_APIEndpoints
```

### Without Environment Variable

If you run tests without the environment variable, E2E tests will be skipped:

```bash
$ go test ./test/e2e/...
ok  	github.com/soypete/pedrocli/test/e2e	0.123s

# All E2E tests are skipped
```

## Test Structure

### Test Files

- `test/e2e/webserver_test.go` - Web server API and WebSocket tests
- `test/e2e/helpers.go` - Test utilities and setup functions

### Test Cases

#### 1. REST API Endpoints (`TestWebServer_APIEndpoints`)

Tests all REST API endpoints:
- `GET /api/agents` - Returns list of 4 agents
- `GET /api/jobs` - Returns job list (initially empty)
- `GET /api/jobs/{id}` - Returns 404 for non-existent jobs
- `POST /api/transcribe` - Returns 503 when STT not configured
- `POST /api/speak` - Returns 503 when TTS not configured

#### 2. WebSocket Communication (`TestWebServer_WebSocket`)

Tests WebSocket functionality:
- Connection establishment
- Invalid message type handling
- Missing agent name error
- Unknown agent error
- Message format validation

#### 3. Job Execution (`TestWebServer_JobExecution`)

Tests job creation and monitoring:
- Sending `run_agent` messages
- Receiving `job_started` or error responses
- Receiving periodic `job_update` messages
- Graceful handling when Ollama is not available

#### 4. Concurrent Connections (`TestWebServer_ConcurrentConnections`)

Tests multiple simultaneous WebSocket clients:
- Connecting 5 clients simultaneously
- Each client can send/receive independently
- No interference between clients

#### 5. Static Files (`TestWebServer_StaticFiles`)

Tests static file serving:
- `GET /` serves index.html
- `GET /static/test.css` serves CSS files
- Correct content types and content

## Test Utilities

### `SetupTestEnvironment(t *testing.T)`

Creates a clean test environment:
- Temporary working directory
- Test configuration file
- Temporary jobs directory
- Automatic cleanup on test completion

### `SkipUnlessE2E(t *testing.T)`

Skips the test unless `RUN_E2E_TESTS` is set:

```go
func TestSomething(t *testing.T) {
    SkipUnlessE2E(t)  // Test will be skipped unless RUN_E2E_TESTS=1
    // ... test code ...
}
```

### `startWebServer(t *testing.T, env *TestEnvironment)`

Starts a web server process for testing:
- Builds the web-server binary
- Finds an available port
- Creates test configuration
- Returns `*WebServerProcess` with URL and cleanup function
- Automatically retries if port binding fails

### `waitForServer(t *testing.T, url string, timeout time.Duration)`

Waits for the server to be ready:
- Polls the `/api/agents` endpoint
- Fails the test if server doesn't start within timeout

## Configuration

E2E tests use a test configuration that points to Ollama:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:7b",
    "ollama_url": "http://localhost:11434",
    "temperature": 0.2
  },
  "project": {
    "name": "TestProject",
    "workdir": "/tmp/test-workdir",
    "tech_stack": ["Go"]
  },
  "tools": {
    "allowed_bash_commands": ["echo", "ls", "cat", "pwd"],
    "forbidden_commands": ["rm", "mv", "dd", "sudo"]
  },
  "limits": {
    "max_task_duration_minutes": 5,
    "max_inference_runs": 5
  }
}
```

## Writing New E2E Tests

### Basic Structure

```go
func TestMyFeature(t *testing.T) {
    SkipUnlessE2E(t)  // Skip unless RUN_E2E_TESTS=1

    env := SetupTestEnvironment(t)
    defer env.Cleanup()

    server := startWebServer(t, env)
    defer server.Stop()

    waitForServer(t, server.URL, 10*time.Second)

    // Your test code here
    resp, err := http.Get(server.URL + "/api/agents")
    // ... assertions ...
}
```

### WebSocket Tests

```go
func TestMyWebSocketFeature(t *testing.T) {
    SkipUnlessE2E(t)

    env := SetupTestEnvironment(t)
    defer env.Cleanup()

    server := startWebServer(t, env)
    defer server.Stop()

    waitForServer(t, server.URL, 10*time.Second)

    // Connect to WebSocket
    wsURL := strings.Replace(server.URL, "http://", "ws://", 1) + "/ws"
    conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    if err != nil {
        t.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // Send message
    msg := map[string]interface{}{
        "type": "my_message_type",
        "data": "test",
    }
    conn.WriteJSON(msg)

    // Read response
    var response map[string]interface{}
    conn.ReadJSON(&response)

    // ... assertions ...
}
```

## CI/CD Integration

### GitHub Actions

E2E tests are designed to work in CI environments. Here's an example workflow:

```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install Ollama
        run: |
          curl -fsSL https://ollama.com/install.sh | sh
          ollama serve &
          sleep 5
          ollama pull qwen2.5-coder:7b

      - name: Run E2E Tests
        run: RUN_E2E_TESTS=1 go test -v ./test/e2e/...
        env:
          RUN_E2E_TESTS: "1"
```

### Local Development

For local development, you can create a shell alias:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias test-e2e='RUN_E2E_TESTS=1 go test -v ./test/e2e/...'

# Usage
test-e2e
```

## Test Isolation

Each test is isolated:
- **Separate working directory**: Created via `t.TempDir()`
- **Unique port**: Automatically finds available port
- **Independent jobs directory**: Each server gets its own jobs directory
- **Clean state**: No shared state between tests

## Handling External Dependencies

### Ollama

Job execution tests gracefully handle missing Ollama:

```go
// Test accepts both success and "Ollama not running" errors
if responseType == "job_started" {
    t.Log("✓ Job started successfully")
    // ... test job execution ...
} else if responseType == "error" {
    if strings.Contains(errorMsg, "connection refused") {
        t.Log("⚠ Ollama not available (expected in test)")
        t.Log("✓ Error handling works correctly")
    } else {
        t.Errorf("Unexpected error: %s", errorMsg)
    }
}
```

This allows tests to pass in environments without Ollama while still validating error handling.

### Whisper.cpp / Piper

Voice-related tests check for `503 Service Unavailable` when STT/TTS is not configured, which is the expected behavior.

## Debugging Failed Tests

### Enable Verbose Output

```bash
RUN_E2E_TESTS=1 go test -v ./test/e2e/...
```

### Inspect Server Output

The test helper captures server stdout/stderr:

```go
cmd.Stdout = os.Stdout  // Server output visible in test logs
cmd.Stderr = os.Stderr
```

### Check Temporary Files

Test directories are cleaned up automatically, but you can prevent this:

```go
// In your test
tmpDir := t.TempDir()
t.Logf("Test directory: %s", tmpDir)
// Add a sleep or breakpoint to inspect before cleanup
```

### Port Conflicts

If tests fail with "address already in use":

```bash
# Find process using port
lsof -ti:8080 | xargs kill -9

# Or let the test find a free port (default behavior)
```

## Test Coverage

Check test coverage including E2E tests:

```bash
RUN_E2E_TESTS=1 go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Best Practices

1. **Always use `SkipUnlessE2E(t)`**: Ensures tests are opt-in
2. **Use `SetupTestEnvironment(t)`**: Provides clean, isolated test state
3. **Always defer cleanup**: `defer env.Cleanup()`, `defer server.Stop()`
4. **Wait for server**: Use `waitForServer()` before making requests
5. **Set read deadlines**: Prevent hanging on WebSocket reads
6. **Handle both success and expected failures**: Tests should pass even when optional dependencies are missing

## Common Issues

### Test Hangs

- Set timeouts on WebSocket reads: `conn.SetReadDeadline()`
- Set timeouts on HTTP requests: `context.WithTimeout()`
- Ensure server is started before making requests

### Port Already in Use

- The test automatically retries with a new port
- If it still fails, check for leaked processes

### Tests Pass But Server Fails

- Run the server manually to debug: `go run cmd/web-server/main.go`
- Check configuration file syntax
- Verify Ollama is accessible

### Flaky Tests

- Use proper synchronization (e.g., `waitForServer()`)
- Avoid hardcoded timeouts - use polling instead
- Ensure test isolation (no shared state)

## Future E2E Tests

Potential additions:
- [ ] Testing config file hot-reload
- [ ] Testing job cancellation via API
- [ ] Testing file upload for log analysis
- [ ] Testing rate limiting (if implemented)
- [ ] Testing authentication (if implemented)
- [ ] Load testing with many concurrent jobs

## See Also

- [Web Server API Reference](WEB_SERVER_API.md) - API documentation
- [Contributing Guide](../CONTRIBUTING.md) - How to contribute tests
- [Main README](../README.md) - Project overview
