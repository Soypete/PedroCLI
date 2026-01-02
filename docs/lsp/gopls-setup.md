# Setting Up gopls for Pedro CLI

gopls is the official Go language server, providing IDE features like code navigation, auto-completion, and diagnostics. This guide explains how to set up gopls for use with Pedro CLI's LSP integration.

## Prerequisites

- Go 1.21 or later installed
- `GOPATH` and `GOROOT` environment variables set

## Installation

### Option 1: Using go install (Recommended)

```bash
go install golang.org/x/tools/gopls@latest
```

Verify installation:

```bash
gopls version
# Output: golang.org/x/tools/gopls v0.x.x
```

### Option 2: Using a Package Manager

**macOS (Homebrew):**

```bash
brew install gopls
```

**Linux (via Go):**

```bash
go install golang.org/x/tools/gopls@latest
```

**Make sure gopls is in your PATH:**

```bash
# Add to your ~/.bashrc or ~/.zshrc
export PATH=$PATH:$(go env GOPATH)/bin
```

## Configuration

### Enabling LSP in Pedro CLI

Add the following to your `.pedrocli.json` configuration file:

```json
{
  "lsp": {
    "enabled": true,
    "timeout": 30,
    "servers": {
      "gopls": {
        "command": "gopls",
        "args": ["serve"],
        "languages": ["go"],
        "enabled": true,
        "init_options": {
          "gofumpt": true,
          "staticcheck": true
        }
      }
    }
  }
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable/disable LSP globally |
| `timeout` | int | 30 | Connection timeout in seconds |
| `servers` | object | {} | LSP server configurations |

### Server Configuration Options

| Option | Type | Description |
|--------|------|-------------|
| `command` | string | Path to the language server binary |
| `args` | array | Arguments to pass to the server |
| `languages` | array | Language IDs this server handles |
| `enabled` | bool | Enable/disable this server |
| `init_options` | object | Initialization options for the server |

## Default Servers

Pedro CLI includes default configurations for common language servers. If the server binary is found in your PATH, it will be used automatically:

| Language | Server | Command |
|----------|--------|---------|
| Go | gopls | `gopls serve` |
| TypeScript/JavaScript | typescript-language-server | `typescript-language-server --stdio` |
| Python | pylsp | `pylsp` |
| Rust | rust-analyzer | `rust-analyzer` |
| C/C++ | clangd | `clangd` |
| Shell | bash-language-server | `bash-language-server start` |
| YAML | yaml-language-server | `yaml-language-server --stdio` |

## Using LSP in Pedro CLI

Once configured, the LSP tool becomes available to agents. Here's how to use it:

### Go to Definition

Find where a function, type, or variable is defined:

```json
{"tool": "lsp", "args": {"operation": "definition", "file": "main.go", "line": 42, "column": 15}}
```

### Find References

Find all places where a symbol is used:

```json
{"tool": "lsp", "args": {"operation": "references", "file": "pkg/server.go", "line": 25, "column": 6}}
```

### Get Hover Information

Get type information and documentation:

```json
{"tool": "lsp", "args": {"operation": "hover", "file": "internal/api.go", "line": 10, "column": 12}}
```

### Get Diagnostics

Get compiler errors and warnings for a file:

```json
{"tool": "lsp", "args": {"operation": "diagnostics", "file": "cmd/main.go"}}
```

### List Symbols

Get all symbols in a file:

```json
{"tool": "lsp", "args": {"operation": "symbols", "file": "pkg/models/user.go"}}
```

Get all symbols in the workspace:

```json
{"tool": "lsp", "args": {"operation": "symbols", "file": "any.go", "scope": "workspace"}}
```

## Agent Workflow Best Practices

When using LSP with Pedro CLI agents:

1. **Before editing code**: Use `diagnostics` to understand existing issues
2. **When exploring unfamiliar code**: Use `definition` and `hover` to understand symbols
3. **Before refactoring**: Use `references` to find all usages
4. **After making changes**: Use `diagnostics` to verify no errors were introduced

Example agent workflow:

```
1. Agent receives task: "Refactor the handleRequest function"
2. Agent uses lsp:hover to understand the function signature
3. Agent uses lsp:references to find all callers
4. Agent makes changes
5. Agent uses lsp:diagnostics to verify no errors
```

## Troubleshooting

### gopls not found

Ensure gopls is in your PATH:

```bash
which gopls

# If not found, add Go bin to PATH:
export PATH=$PATH:$(go env GOPATH)/bin
```

### Slow startup on large projects

For large monorepos, gopls may take time to index. Consider:

1. Using a more powerful machine or allocating more memory
2. Excluding directories with `.gopls.mod` file
3. Using workspace mode for multi-module projects

### Module errors

Ensure your project has valid go.mod:

```bash
go mod tidy
```

### gopls crashes

Check for known issues:

```bash
gopls version
# Upgrade to latest version if needed:
go install golang.org/x/tools/gopls@latest
```

## Advanced Configuration

### Custom gopls settings

For advanced users, pass additional initialization options:

```json
{
  "lsp": {
    "servers": {
      "gopls": {
        "command": "gopls",
        "args": ["serve"],
        "languages": ["go"],
        "enabled": true,
        "init_options": {
          "gofumpt": true,
          "staticcheck": true,
          "analyses": {
            "unusedparams": true,
            "shadow": true,
            "nilness": true
          },
          "codelenses": {
            "gc_details": true,
            "generate": true,
            "test": true
          },
          "diagnosticsDelay": "500ms",
          "usePlaceholders": true,
          "completionDocumentation": true
        }
      }
    }
  }
}
```

### Using a different gopls binary

If you have multiple versions installed:

```json
{
  "lsp": {
    "servers": {
      "gopls": {
        "command": "/custom/path/to/gopls",
        "args": ["serve"],
        "languages": ["go"],
        "enabled": true
      }
    }
  }
}
```

### Environment variables

Pass environment variables to the server:

```json
{
  "lsp": {
    "servers": {
      "gopls": {
        "command": "gopls",
        "args": ["serve"],
        "languages": ["go"],
        "enabled": true,
        "settings": {
          "env": {
            "GOPATH": "/custom/gopath",
            "GOPROXY": "https://proxy.golang.org,direct"
          }
        }
      }
    }
  }
}
```

## See Also

- [gopls documentation](https://pkg.go.dev/golang.org/x/tools/gopls)
- [LSP Specification](https://microsoft.github.io/language-server-protocol/)
- [Pedro CLI Documentation](../00-README.md)
