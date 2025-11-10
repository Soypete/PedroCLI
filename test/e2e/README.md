# End-to-End Tests

This directory contains comprehensive end-to-end (E2E) tests for PedroCLI that verify full workflows, including agent execution, CLI commands, and MCP server integration.

## Running E2E Tests

E2E tests are **expensive and slow** - they build binaries, start servers, and execute complete workflows. Therefore, they only run when explicitly enabled.

### Skip by Default

By default, E2E tests are skipped:

```bash
# These commands will skip all E2E tests
go test ./test/e2e
go test ./test/e2e -v
```

Output: `SKIP: Skipping E2E test - set RUN_E2E_TESTS=1 to run`

### Run Manually

To run E2E tests locally during development:

```bash
RUN_E2E_TESTS=1 go test ./test/e2e -v
```

### Run in CI/CD

E2E tests should run only when:
- A pull request is marked as "ready for review"
- A manual trigger button is pressed
- On release builds

**Example GitHub Actions workflows are provided in `docs/examples/github-workflows/`:**
- `e2e-tests.yml` - E2E test workflow (runs on PR ready for review + manual trigger)
- `ci.yml` - Regular CI workflow (runs unit tests on every push/PR)

**To activate the workflows, run the setup script:**

```bash
./scripts/setup-github-workflows.sh
```

This will copy the workflow files to `.github/workflows/` and provide instructions for committing them.

## Test Coverage

### Agent Tests (`agents_test.go`)
- **TestBuilderAgent_E2E**: Verifies agent can create files and complete tasks
- **TestDebuggerAgent_E2E**: Verifies agent can identify and fix bugs
- **TestReviewerAgent_E2E**: Verifies agent can review code and provide feedback
- **TestTriagerAgent_E2E**: Verifies agent can analyze issues and provide diagnoses

### CLI Tests (`cli_test.go`)
- **TestCLI_Build_Command**: Tests `pedrocli build` command execution
- **TestCLI_Debug_Command**: Tests `pedrocli debug` command execution
- **TestCLI_Review_Command**: Tests `pedrocli review` command execution
- **TestCLI_Triage_Command**: Tests `pedrocli triage` command execution
- **TestCLI_Help_Command**: Tests `pedrocli --help` displays all commands
- **TestCLI_Version_Command**: Tests `pedrocli --version` displays version info

### MCP Tests (`mcp_test.go`)
- **TestMCP_Initialize**: Tests MCP server initialization and handshake
- **TestMCP_ToolsList**: Tests MCP server exposes all expected tools
- **TestMCP_ToolCall_FileRead**: Tests file reading through MCP protocol
- **TestMCP_ToolCall_FileWrite**: Tests file writing through MCP protocol
- **TestMCP_ToolCall_SearchGrep**: Tests code search through MCP protocol

## Test Infrastructure

### TestEnvironment (`helpers.go`)
Provides isolated test environments with:
- Temporary workspace directories
- Test configuration
- Mock LLM backend
- Job manager
- File utilities and assertions

### MockBackend
Implements `llm.Backend` interface with canned responses for deterministic testing without requiring actual LLM API calls.

## Requirements

E2E tests require:
- Go 1.21+
- Git (for repository operations)
- Sufficient disk space for temporary workspaces
- 5-30 minutes execution time (full suite)

## Troubleshooting

**Tests hang or timeout:**
- Increase timeout: `go test ./test/e2e -v -timeout 30m`
- Check that MCP server builds successfully
- Verify no port conflicts (if testing web server)

**Tests fail with "binary not found":**
- Ensure `go build` works for `cmd/pedrocli` and `cmd/mcp-server`
- Check Go build cache: `go clean -cache`

**Permission errors:**
- Tests use `t.TempDir()` which should have proper permissions
- Check that test files are not gitignored

## Best Practices

1. **Always skip by default**: Never remove the `SkipUnlessE2E(t)` check
2. **Keep tests isolated**: Each test should create its own environment
3. **Clean up resources**: Use `defer env.Cleanup()` and `defer server.Stop()`
4. **Add timeouts**: Use `context.WithTimeout()` for agent operations
5. **Log failures**: Use `t.Logf()` to debug issues in CI
