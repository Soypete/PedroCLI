# PedroCLI Agent Guidelines

This file provides guidance for AI agents working on the PedroCLI codebase.

## Build Commands

```bash
# Build all binaries (CLI + HTTP server)
make build

# Build only CLI
make build-cli

# Build only HTTP server
make build-http

# Cross-compile for macOS (arm64 + amd64)
make build-mac

# Cross-compile for Linux (amd64)
make build-linux
```

Binaries are output to the project root:
- `pedrocli` - CLI client (cmd/pedrocli/main.go)
- `pedrocli-http-server` - HTTP server with web UI (cmd/http-server/main.go)

## Test Commands

```bash
# Run all tests (requires PostgreSQL running)
make test

# Run tests without database (faster)
make test-quick

# Run with coverage report
make test-coverage

# Run specific test
go test -v -run TestName ./path/to/package

# Run tests in specific package
go test ./pkg/tools/...
go test ./pkg/agents/...
```

## Lint & Format Commands

```bash
# Format all code
make fmt

# Run linter (golangci-lint)
make lint

# Tidy dependencies
make tidy
```

The linter is configured in `.golangci.yml`. Enabled linters:
- errcheck, govet, ineffassign, staticcheck, unused
- gofmt, goimports, misspell

Go version: 1.24

## Code Style Guidelines

### Imports

Group imports in three sections separated by blank lines:
1. Standard library (stdlib)
2. External packages (e.g., `github.com/...`)
3. Internal packages (e.g., `github.com/soypete/pedrocli/...`)

```go
import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/some/external/pkg"

    "github.com/soypete/pedrocli/pkg/config"
    "github.com/soypete/pedrocli/pkg/llm"
)
```

### Formatting

- Use `go fmt` or `gofmt` for formatting
- Maximum line length: not strictly enforced, but keep under 120 chars when reasonable
- Use blank lines to separate logical code sections within functions

### Types & Interfaces

Use interfaces to define abstractions. Common patterns:

```go
// Tool interface - all tools implement this
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args map[string]interface{}) (*Result, error)
}

// Backend interface - for LLM backends
type Backend interface {
    Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)
    GetContextWindow() int
    GetUsableContext() int
}
```

Structs use JSON tags for serialization:

```go
type InferenceRequest struct {
    SystemPrompt string            `json:"system_prompt"`
    UserPrompt   string            `json:"user_prompt"`
    Temperature  float64           `json:"temperature"`
    MaxTokens    int               `json:"max_tokens,omitempty"`
    Tools        []ToolDefinition  `json:"tools,omitempty"`
}
```

### Naming Conventions

- **Variables & Functions**: camelCase (e.g., `maxFileSize`, `executeTool`)
- **Exported Types & Functions**: PascalCase (e.g., `Config`, `NewCodeEditTool`)
- **Constants**: PascalCase for exported, camelCase for unexported (e.g., `MaxRetries`, `defaultTimeout`)
- **Package Names**: short, lowercase, no underscores (e.g., `pkg/tools`, not `pkg/tools_package`)
- **File Names**: lowercase with underscores (e.g., `code_edit.go`, not `codeEdit.go`)

### Error Handling

Use `fmt.Errorf` with `%w` for error wrapping:

```go
// Simple error
return nil, fmt.Errorf("missing 'action' parameter")

// Wrapped error with context
return nil, fmt.Errorf("failed to execute tool %s: %w", toolName, err)

// Error with additional info
return &Result{Success: false, Error: fmt.Sprintf("unknown action: %s", action)}, nil
```

Avoid generic errors like `err != nil { return err }`. Provide context:

```go
if err != nil {
    return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
}
```

### Struct Initialization

Prefer functional options for constructors with optional parameters:

```go
// Simple constructor
func NewCodeEditTool() *CodeEditTool {
    return &CodeEditTool{
        maxFileSize: 10 * 1024 * 1024,
        fs:          fileio.NewFileSystem(),
    }
}

// Constructor with options
func NewToolWithConfig(cfg *Config) *Tool {
    return &Tool{
        maxRetries: cfg.MaxRetries,
    }
}
```

### Context Usage

Always pass `context.Context` as first parameter for functions that may timeout or be cancelled:

```go
func (t *Tool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error)
```

Check for cancellation in long-running operations:

```go
select {
case <-ctx.Done():
    return nil, ctx.Err()
default:
    // continue operation
}
```

### Result Pattern

Tools return a `Result` struct with `Success`, `Output`, `Error`, and `ModifiedFiles` fields:

```go
return &Result{
    Success:        true,
    Output:         "file modified successfully",
    ModifiedFiles:  []string{"main.go"},
}, nil

// Error case
return &Result{Success: false, Error: "file not found"}, nil
```

### Testing

Follow Go testing conventions:
- Test files: `*_test.go` suffix
- Test functions: `Test` prefix (e.g., `TestCodeEditTool_Execute`)
- Use table-driven tests for multiple test cases:

```go
func TestCodeEditTool_Execute(t *testing.T) {
    tests := []struct {
        name    string
        args    map[string]interface{}
        wantErr bool
    }{
        {"missing action", map[string]interface{}{}, true},
        {"valid edit", map[string]interface{}{"action": "get_lines", "path": "foo.go"}, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tool := NewCodeEditTool()
            _, err := tool.Execute(context.Background(), tt.args)
            if (err != nil) != tt.wantErr {
                t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Key Packages

- `pkg/agents/` - Autonomous agents (Builder, Debugger, Reviewer, Triager)
- `pkg/tools/` - Tools agents use (file, codeedit, search, git, bash, test)
- `pkg/llm/` - LLM backend abstraction (Ollama, llama.cpp, vllm)
- `pkg/config/` - Configuration management
- `pkg/httpbridge/` - HTTP server and API
- `pkg/jobs/` - Job management

## Critical Files

- `pkg/agents/executor.go` - The inference loop (heart of autonomous operation)
- `pkg/agents/base.go` - Base agent and system prompts
- `pkg/llmcontext/manager.go` - File-based context management

## Design Documents

For implementation details and context preservation during compaction, see:

### Pedro vNext Orchestration
- [Overview](../docs/pedro-vnext/README.md) - Milestone navigation
- [Architecture](../docs/pedro-vnext/architecture/overview.md) - System diagrams
- [Integration Points](../docs/pedro-vnext/architecture/integration.md) - REPL, HTTP, CLI integration

### Milestones (Implementation Documentation)
- [M1: Query Engine](../docs/pedro-vnext/milestones/M1-query-engine/) - Intent routing
- [M2: Execution Modes](../docs/pedro-vnext/milestones/M2-execution-modes/) - chat/plan/build/review
- [M3: Phase Registry](../docs/pedro-vnext/milestones/M3-phase-registry/) - Reusable phases
- [M4: Task Envelope](../docs/pedro-vnext/milestones/M4-task-envelope/) - Structured I/O
- [M5: Subagent Manager](../docs/pedro-vnext/milestones/M5-subagent-manager/) - Child agents
- [M6: Artifact Store](../docs/pedro-vnext/milestones/M6-artifact-store/) - Structured workspace
- [M7: Permission Engine](../docs/pedro-vnext/milestones/M7-permission-engine/) - Per-agent permissions
- [M8: Prompt Architecture](../docs/pedro-vnext/milestones/M8-prompt-architecture/) - Layered prompts
- [M9: Telemetry](../docs/pedro-vnext/milestones/M9-telemetry/) - Cost tracking
- [M10: Kairos Memory](../docs/pedro-vnext/milestones/M10-kairos-memory/) - Session continuity

### Technical References
- [Implementation Plan](../docs/pedrocode-vnext-implementation-plan.md) - Full technical plan
- [Interface Definitions](../docs/pedrocode-vnext-interfaces.md) - Go interface specs
- [ADR-012: Memory](../docs/adr/ADR-012-kairos-memory-consolidation.md) - Memory system design
- [ADR-013: Orchestration](../docs/adr/ADR-013-pedrocode-vnext-orchestration.md) - Architecture
