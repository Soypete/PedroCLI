# ADR-001: Dynamic Tool Registry Architecture

## Status

Proposed

## Context

The current tool registration system in PedroCLI has several limitations:

1. **No Parameter Schema**: Tools expose only `Name()` and `Description()`. There's no structured way to communicate parameter types, required fields, or valid values to the LLM or clients.

2. **No Tool Classification**: All tools are treated equally. There's no distinction between required tools (must be available) and optional tools (nice-to-have for enhanced output).

3. **Static Registration**: Tools are registered at startup in `cmd/mcp-server/main.go` and cannot be added, removed, or modified during runtime.

4. **No Grouping/Categories**: Tools can't be organized by function (e.g., "research tools", "code tools", "publishing tools").

5. **Duplicate Registration**: The same tools are registered to both the MCP server and each agent, requiring careful synchronization.

### Current Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args map[string]interface{}) (*Result, error)
}
```

### Current Registration Pattern

```go
// In cmd/mcp-server/main.go
fileTool := tools.NewFileTool()
server.RegisterTool(fileTool)
builderAgent.RegisterTool(fileTool)
reviewerAgent.RegisterTool(fileTool)
// ... repeated for every agent
```

## Decision

### 1. Extended Tool Interface

Create a new `ToolMetadata` structure and extend the `Tool` interface:

```go
// pkg/tools/interface.go

// ToolMetadata provides extended information about a tool
type ToolMetadata struct {
    // Schema is the JSON Schema for the tool's parameters
    Schema *logits.JSONSchema `json:"schema,omitempty"`

    // Category groups related tools (e.g., "research", "code", "publish")
    Category string `json:"category,omitempty"`

    // Optionality indicates when the tool should be used
    Optionality ToolOptionality `json:"optionality"`

    // UsageHint provides context for when to use this tool
    UsageHint string `json:"usage_hint,omitempty"`

    // Examples shows sample invocations
    Examples []ToolExample `json:"examples,omitempty"`

    // RequiresCapabilities lists required system capabilities
    RequiresCapabilities []string `json:"requires_capabilities,omitempty"`
}

type ToolOptionality string

const (
    // ToolRequired - Tool must be available for agent to function
    ToolRequired ToolOptionality = "required"

    // ToolOptional - Tool enhances output but isn't necessary
    ToolOptional ToolOptionality = "optional"

    // ToolConditional - Tool is required if certain conditions are met
    ToolConditional ToolOptionality = "conditional"
)

type ToolExample struct {
    Description string                 `json:"description"`
    Input       map[string]interface{} `json:"input"`
    Output      string                 `json:"output,omitempty"`
}

// ExtendedTool adds metadata support to the basic Tool interface
type ExtendedTool interface {
    Tool
    Metadata() *ToolMetadata
}
```

### 2. Centralized Tool Registry

Create a registry that manages tool lifecycle and provides discovery:

```go
// pkg/tools/registry.go

type ToolRegistry struct {
    mu       sync.RWMutex
    tools    map[string]ExtendedTool
    byCategory map[string][]string
    listeners []ToolRegistryListener
}

type ToolRegistryListener interface {
    OnToolRegistered(tool ExtendedTool)
    OnToolUnregistered(name string)
}

func NewToolRegistry() *ToolRegistry

// Registration
func (r *ToolRegistry) Register(tool ExtendedTool) error
func (r *ToolRegistry) Unregister(name string) error

// Discovery
func (r *ToolRegistry) Get(name string) (ExtendedTool, bool)
func (r *ToolRegistry) List() []ExtendedTool
func (r *ToolRegistry) ListByCategory(category string) []ExtendedTool
func (r *ToolRegistry) ListByOptionality(opt ToolOptionality) []ExtendedTool

// For LLM consumption
func (r *ToolRegistry) GetToolDescriptions() []ToolDescription
func (r *ToolRegistry) GetToolSchemas() map[string]*logits.JSONSchema

// Event subscription
func (r *ToolRegistry) Subscribe(listener ToolRegistryListener)
func (r *ToolRegistry) Unsubscribe(listener ToolRegistryListener)
```

### 3. Tool Categories

Define standard categories:

| Category | Description | Example Tools |
|----------|-------------|---------------|
| `code` | Code manipulation and analysis | file, code_edit, search, navigate |
| `vcs` | Version control operations | git |
| `build` | Build and test operations | bash, test |
| `research` | Information gathering | rss_feed, calendar, web_search |
| `publish` | Content publishing | blog_notion, static_links |
| `utility` | General utilities | logit |

### 4. Tool Bundles

Pre-configured sets of tools for common use cases:

```go
// pkg/tools/bundles.go

type ToolBundle struct {
    Name        string
    Description string
    Required    []string  // Tool names that must be available
    Optional    []string  // Tool names that enhance functionality
}

var CodeAgentBundle = &ToolBundle{
    Name:        "code_agent",
    Description: "Standard tools for code manipulation agents",
    Required:    []string{"file", "code_edit", "search", "navigate", "git"},
    Optional:    []string{"bash", "test", "web_search"},
}

var BlogAgentBundle = &ToolBundle{
    Name:        "blog_agent",
    Description: "Tools for blog content creation",
    Required:    []string{},
    Optional:    []string{"rss_feed", "calendar", "static_links", "blog_notion", "web_search"},
}
```

### 5. Integration with MCP Server

Update MCP server to use the registry:

```go
// pkg/mcp/server.go

type Server struct {
    registry *tools.ToolRegistry
    // ... other fields
}

func (s *Server) handleToolsList(req *Request) {
    toolsList := make([]map[string]interface{}, 0)

    for _, tool := range s.registry.List() {
        toolInfo := map[string]interface{}{
            "name":        tool.Name(),
            "description": tool.Description(),
        }

        if meta := tool.Metadata(); meta != nil {
            if meta.Schema != nil {
                toolInfo["inputSchema"] = meta.Schema
            }
            toolInfo["category"] = meta.Category
            toolInfo["optionality"] = meta.Optionality
        }

        toolsList = append(toolsList, toolInfo)
    }

    s.sendResponse(req.ID, map[string]interface{}{"tools": toolsList})
}
```

## Consequences

### Positive

1. **Schema-Driven Validation**: Tools can validate inputs before execution using JSON Schema.

2. **LLM-Friendly Discovery**: The LLM receives structured information about what tools do and how to use them.

3. **Grammar Generation**: Tool schemas enable automatic GBNF grammar generation for constrained output.

4. **Flexible Configuration**: Tool sets can be configured per-agent or per-request via bundles.

5. **Runtime Flexibility**: Tools can be registered/unregistered during runtime.

6. **Event-Driven Updates**: Components can subscribe to registry changes.

### Negative

1. **Migration Effort**: All existing tools need to be updated to implement `Metadata()`.

2. **Schema Maintenance**: Parameter schemas must be kept in sync with tool implementations.

3. **Increased Complexity**: Registry adds abstraction layer over direct tool maps.

### Mitigation

1. **Gradual Migration**: Create `SimpleExtendedTool` wrapper that returns nil metadata for legacy tools.

2. **Schema Validation**: Add CI checks that validate tool schemas match implementations.

3. **Clear Documentation**: Document the registry pattern and provide migration guide.

## Implementation

### Phase 1: Core Registry

1. Define `ToolMetadata` and `ExtendedTool` interface
2. Implement `ToolRegistry` with basic CRUD operations
3. Create `SimpleExtendedTool` wrapper for backward compatibility

### Phase 2: Tool Migration

1. Update each tool to implement `Metadata()`
2. Add JSON Schema definitions for tool parameters
3. Assign categories and optionality to each tool

### Phase 3: Integration

1. Update MCP server to use registry
2. Update agents to receive tools from registry
3. Implement tool bundles for common configurations

### Phase 4: Advanced Features

1. Add event subscription for dynamic updates
2. Implement tool capability checking
3. Add registry persistence for configuration management

## Example: Migrated File Tool

```go
// pkg/tools/file.go

func (t *FileTool) Metadata() *ToolMetadata {
    return &ToolMetadata{
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "action": {
                    Type: "string",
                    Enum: []interface{}{"read", "write", "list"},
                    Description: "The file operation to perform",
                },
                "path": {
                    Type: "string",
                    Description: "Absolute or relative file path",
                },
                "content": {
                    Type: "string",
                    Description: "Content to write (for write action)",
                },
            },
            Required: []string{"action", "path"},
        },
        Category:    "code",
        Optionality: ToolRequired,
        UsageHint:   "Use for reading, writing, and listing files. Always read before modifying.",
        Examples: []ToolExample{
            {
                Description: "Read a Go source file",
                Input:       map[string]interface{}{"action": "read", "path": "main.go"},
            },
        },
    }
}
```

## Related ADRs

- **ADR-002**: LLM Tool Awareness Protocol (consumes registry data)
- **ADR-003**: Dynamic Tool Invocation Pattern (uses registry for tool lookup)
- **ADR-004**: Logit-Controlled Tool Calling (generates grammars from schemas)
