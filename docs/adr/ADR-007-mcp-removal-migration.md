# ADR-007: MCP Removal and Unified Tool Architecture Migration

## Status

Proposed

## Context

The current PedroCLI architecture uses a custom MCP (Model Context Protocol) server for tool communication:

```
CLI/HTTP → MCP Client → (stdio JSON-RPC) → MCP Server → Tools
```

This architecture was designed to be compatible with the MCP standard, but introduces unnecessary complexity:

1. **Subprocess overhead**: MCP server runs as a separate process
2. **JSON-RPC serialization**: All tool calls go through stdio serialization
3. **Duplicate registration**: Tools registered to both MCP server AND agents
4. **Limited metadata**: MCP protocol exposes only name/description, no schemas
5. **No model awareness**: Tool formatting is not model-specific

### Current Pain Points

| Issue | Impact |
|-------|--------|
| Two-binary system | Complex deployment, debugging |
| Stdio transport | Latency, error handling complexity |
| Duplicate tool registration | Maintenance burden, sync bugs |
| Generic JSON format | Models parse tool calls differently |
| No parameter schemas | LLM must guess parameter format |

### ADRs 001-006 Reference

The existing ADRs (001-006) propose dynamic tool architecture improvements but assume MCP remains as the transport layer. This ADR supersedes that assumption and proposes removing internal MCP entirely.

**What we keep from ADRs 001-006:**
- Extended Tool interface with Metadata (ADR-001)
- Dynamic tool registry (ADR-001)
- LLM tool awareness via prompts (ADR-002)
- Dynamic execution loop (ADR-003)
- Logit-controlled generation (ADR-004)
- Unified agent architecture (ADR-005)
- Optional tool catalog (ADR-006)

**What we remove:**
- MCP server (`pkg/mcp/server.go`)
- MCP client (`pkg/mcp/client.go`)
- MCP server binary (`cmd/mcp-server/`)
- AgentTool wrapper (`pkg/mcp/agent_tool.go`)
- JSON-RPC transport layer

## Decision

### 1. New Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Entry Points                             │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   CLI Binary    │   HTTP Server   │   (Future: Library Import)  │
│  cmd/pedrocli/  │ cmd/http-server/│                             │
└────────┬────────┴────────┬────────┴─────────────────────────────┘
         │                 │
         ▼                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    pkg/toolbox (NEW)                             │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                     ToolRegistry                            ││
│  │  - Centralized tool management                              ││
│  │  - Category/mode-based filtering                            ││
│  │  - Capability detection                                     ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   ToolDefinition                            ││
│  │  - Name, Description, Handler                               ││
│  │  - JSONSchema for parameters                                ││
│  │  - Category, Optionality                                    ││
│  │  - Usage hints, Examples                                    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                  pkg/toolformat (NEW)                            │
│  ┌─────────────────┬─────────────────┬─────────────────────────┐│
│  │  LlamaFormatter │  QwenFormatter  │  ClaudeAPIFormatter     ││
│  │  (Llama 3.x)    │  (Qwen 2.5)     │  (Anthropic API)        ││
│  ├─────────────────┼─────────────────┼─────────────────────────┤│
│  │  MistralFormatter│ HermesFormatter │  OpenAIFormatter       ││
│  │  ([TOOL_CALLS]) │  (XML tags)     │  (Generic/vLLM)        ││
│  └─────────────────┴─────────────────┴─────────────────────────┘│
│                                                                  │
│  Each formatter implements:                                      │
│  - FormatTools([]ToolDefinition) → string/struct                │
│  - ParseToolCall(response) → (*ToolCall, error)                 │
│  - FormatToolResult(result) → string                            │
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                     pkg/agents (UPDATED)                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                  DynamicExecutor                            ││
│  │  - Uses ToolRegistry directly (no MCP)                      ││
│  │  - Selects ToolFormatter based on model                     ││
│  │  - Executes tools in-process                                ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                     pkg/tools (UPDATED)                          │
│  - All existing tools remain                                     │
│  - Each tool implements ExtendedTool interface                   │
│  - Metadata() returns full JSONSchema                            │
└─────────────────────────────────────────────────────────────────┘
```

### 2. Core Interfaces

#### 2.1 Extended Tool Definition

```go
// pkg/toolbox/definition.go

// ToolDefinition is the canonical tool representation
type ToolDefinition struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Category    ToolCategory      `json:"category"`
    Optionality ToolOptionality   `json:"optionality"`
    Schema      *JSONSchema       `json:"schema"`
    UsageHint   string            `json:"usage_hint,omitempty"`
    Examples    []ToolExample     `json:"examples,omitempty"`
    Produces    []string          `json:"produces,omitempty"`
    Consumes    []string          `json:"consumes,omitempty"`
    Handler     ToolHandler       `json:"-"` // Not serialized
}

type ToolHandler func(ctx context.Context, args map[string]interface{}) (*ToolResult, error)

type ToolCategory string

const (
    CategoryCode      ToolCategory = "code"
    CategoryVCS       ToolCategory = "vcs"
    CategoryBuild     ToolCategory = "build"
    CategoryResearch  ToolCategory = "research"
    CategoryPublish   ToolCategory = "publish"
    CategoryOrchestration ToolCategory = "orchestration"
    CategoryUtility   ToolCategory = "utility"
)

type ToolOptionality string

const (
    ToolRequired    ToolOptionality = "required"
    ToolOptional    ToolOptionality = "optional"
    ToolConditional ToolOptionality = "conditional"
)

type JSONSchema struct {
    Type        string                 `json:"type"`
    Description string                 `json:"description,omitempty"`
    Properties  map[string]*JSONSchema `json:"properties,omitempty"`
    Required    []string               `json:"required,omitempty"`
    Enum        []interface{}          `json:"enum,omitempty"`
    Items       *JSONSchema            `json:"items,omitempty"`
    OneOf       []*JSONSchema          `json:"oneOf,omitempty"`
    Default     interface{}            `json:"default,omitempty"`
}
```

#### 2.2 Tool Registry

```go
// pkg/toolbox/registry.go

type ToolRegistry struct {
    mu         sync.RWMutex
    tools      map[string]*ToolDefinition
    byCategory map[ToolCategory][]*ToolDefinition
    bundles    map[string]*ToolBundle
}

func NewToolRegistry() *ToolRegistry

// Registration
func (r *ToolRegistry) Register(tool *ToolDefinition) error
func (r *ToolRegistry) RegisterBundle(bundle *ToolBundle) error

// Lookup
func (r *ToolRegistry) Get(name string) (*ToolDefinition, bool)
func (r *ToolRegistry) List() []*ToolDefinition
func (r *ToolRegistry) ListByCategory(cat ToolCategory) []*ToolDefinition
func (r *ToolRegistry) ListForMode(mode string) []*ToolDefinition

// Execution
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)

// Tool Bundles (predefined tool sets)
type ToolBundle struct {
    Name        string
    Description string
    Tools       []string // Tool names
}

var (
    BundleCoding = &ToolBundle{
        Name: "coding",
        Tools: []string{"file", "code_edit", "search", "navigate", "git", "bash", "test"},
    }
    BundleBlog = &ToolBundle{
        Name: "blog",
        Tools: []string{"rss_feed", "static_links", "calendar", "blog_notion", "webscrape"},
    }
    BundleDBMigrations = &ToolBundle{
        Name: "db-migrations",
        Tools: []string{"file", "bash", "git", "db_migrate", "db_status"},
    }
)
```

#### 2.3 Tool Formatter Interface

```go
// pkg/toolformat/formatter.go

// ToolFormatter handles model-specific tool call formatting
type ToolFormatter interface {
    // FormatToolsForPrompt generates the tool description section for the system prompt
    FormatToolsForPrompt(tools []*toolbox.ToolDefinition) string

    // FormatToolsForAPI returns tool definitions in API-specific format (for API-based models)
    FormatToolsForAPI(tools []*toolbox.ToolDefinition) interface{}

    // ParseToolCalls extracts tool calls from LLM response
    ParseToolCalls(response string) ([]ToolCall, error)

    // FormatToolResult formats a tool result for the next prompt
    FormatToolResult(call ToolCall, result *toolbox.ToolResult) string

    // SupportsNativeToolUse returns true if model has native tool API (like Claude)
    SupportsNativeToolUse() bool
}

type ToolCall struct {
    Name string                 `json:"name"`
    Args map[string]interface{} `json:"args"`
}
```

### 3. Model-Specific Formatters

#### 3.1 Llama 3.x Formatter

Llama 3.x uses a special `<|python_tag|>` format for tool calls:

```go
// pkg/toolformat/llama.go

type LlamaFormatter struct {
    version string // "3.1", "3.2", "3.3"
}

func (f *LlamaFormatter) FormatToolsForPrompt(tools []*toolbox.ToolDefinition) string {
    // Llama 3.x expects tools in a specific JSON format in the system prompt
    var sb strings.Builder
    sb.WriteString("You have access to the following tools:\n\n")

    for _, tool := range tools {
        sb.WriteString(fmt.Sprintf("### %s\n", tool.Name))
        sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))
        if tool.Schema != nil {
            schemaJSON, _ := json.MarshalIndent(tool.Schema, "", "  ")
            sb.WriteString(fmt.Sprintf("Parameters:\n```json\n%s\n```\n\n", schemaJSON))
        }
    }

    sb.WriteString(`
When you need to call a tool, use the following format:
<|python_tag|>{"name": "tool_name", "arguments": {"param": "value"}}
`)
    return sb.String()
}

func (f *LlamaFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
    var calls []ToolCall

    // Look for <|python_tag|> markers
    re := regexp.MustCompile(`<\|python_tag\|>\s*(\{[^}]+\})`)
    matches := re.FindAllStringSubmatch(response, -1)

    for _, match := range matches {
        var call struct {
            Name      string                 `json:"name"`
            Arguments map[string]interface{} `json:"arguments"`
        }
        if err := json.Unmarshal([]byte(match[1]), &call); err == nil {
            calls = append(calls, ToolCall{Name: call.Name, Args: call.Arguments})
        }
    }

    // Fallback to generic JSON parsing if no python_tag found
    if len(calls) == 0 {
        calls = parseGenericToolCalls(response)
    }

    return calls, nil
}
```

#### 3.2 Qwen 2.5 Formatter

Qwen 2.5 uses XML-style `<tool_call>` tags:

```go
// pkg/toolformat/qwen.go

type QwenFormatter struct {
    version string // "2.5", "2.5-coder"
}

func (f *QwenFormatter) FormatToolsForPrompt(tools []*toolbox.ToolDefinition) string {
    var sb strings.Builder
    sb.WriteString("# Tools\n\n")
    sb.WriteString("You have access to the following tools:\n\n")

    for _, tool := range tools {
        sb.WriteString(fmt.Sprintf("## %s\n", tool.Name))
        sb.WriteString(fmt.Sprintf("%s\n\n", tool.Description))
        // Qwen works well with inline parameter descriptions
        if tool.Schema != nil && tool.Schema.Properties != nil {
            sb.WriteString("**Parameters:**\n")
            for name, prop := range tool.Schema.Properties {
                req := ""
                for _, r := range tool.Schema.Required {
                    if r == name {
                        req = " (required)"
                        break
                    }
                }
                sb.WriteString(fmt.Sprintf("- `%s`%s: %s\n", name, req, prop.Description))
            }
            sb.WriteString("\n")
        }
    }

    sb.WriteString(`
To call a tool, use the following XML format:
<tool_call>
{"name": "tool_name", "arguments": {"param": "value"}}
</tool_call>
`)
    return sb.String()
}

func (f *QwenFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
    var calls []ToolCall

    // Look for <tool_call>...</tool_call> blocks
    re := regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)
    matches := re.FindAllStringSubmatch(response, -1)

    for _, match := range matches {
        var call struct {
            Name      string                 `json:"name"`
            Arguments map[string]interface{} `json:"arguments"`
        }
        if err := json.Unmarshal([]byte(match[1]), &call); err == nil {
            calls = append(calls, ToolCall{Name: call.Name, Args: call.Arguments})
        }
    }

    return calls, nil
}
```

#### 3.3 Mistral/Mixtral Formatter

Mistral models use `[TOOL_CALLS]` format:

```go
// pkg/toolformat/mistral.go

type MistralFormatter struct{}

func (f *MistralFormatter) FormatToolsForPrompt(tools []*toolbox.ToolDefinition) string {
    // Mistral function calling format
    var sb strings.Builder
    sb.WriteString("[AVAILABLE_TOOLS]\n")

    toolDefs := make([]map[string]interface{}, len(tools))
    for i, tool := range tools {
        toolDefs[i] = map[string]interface{}{
            "type": "function",
            "function": map[string]interface{}{
                "name":        tool.Name,
                "description": tool.Description,
                "parameters":  tool.Schema,
            },
        }
    }

    toolJSON, _ := json.Marshal(toolDefs)
    sb.Write(toolJSON)
    sb.WriteString("\n[/AVAILABLE_TOOLS]\n")

    return sb.String()
}

func (f *MistralFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
    var calls []ToolCall

    // Look for [TOOL_CALLS] blocks
    re := regexp.MustCompile(`(?s)\[TOOL_CALLS\]\s*(\[.*?\])\s*`)
    matches := re.FindAllStringSubmatch(response, -1)

    for _, match := range matches {
        var toolCalls []struct {
            Name      string                 `json:"name"`
            Arguments map[string]interface{} `json:"arguments"`
        }
        if err := json.Unmarshal([]byte(match[1]), &toolCalls); err == nil {
            for _, tc := range toolCalls {
                calls = append(calls, ToolCall{Name: tc.Name, Args: tc.Arguments})
            }
        }
    }

    return calls, nil
}
```

#### 3.4 Claude API Formatter

For direct Anthropic API integration (when using Claude as backend):

```go
// pkg/toolformat/claude.go

type ClaudeAPIFormatter struct{}

func (f *ClaudeAPIFormatter) SupportsNativeToolUse() bool {
    return true
}

func (f *ClaudeAPIFormatter) FormatToolsForAPI(tools []*toolbox.ToolDefinition) interface{} {
    // Return Anthropic API tool format
    claudeTools := make([]map[string]interface{}, len(tools))

    for i, tool := range tools {
        claudeTools[i] = map[string]interface{}{
            "name":        tool.Name,
            "description": tool.Description,
            "input_schema": tool.Schema,
        }
    }

    return claudeTools
}

// ParseToolCalls handles Claude's native tool_use content blocks
func (f *ClaudeAPIFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
    // For API responses, this would parse the structured tool_use blocks
    // The actual parsing depends on the API response format
    return nil, fmt.Errorf("use ParseAPIResponse for Claude API responses")
}

func (f *ClaudeAPIFormatter) ParseAPIResponse(response *anthropic.MessageResponse) ([]ToolCall, error) {
    var calls []ToolCall

    for _, block := range response.Content {
        if block.Type == "tool_use" {
            calls = append(calls, ToolCall{
                Name: block.Name,
                Args: block.Input,
            })
        }
    }

    return calls, nil
}
```

#### 3.5 OpenAI-Compatible Formatter

Generic fallback for vLLM, llama.cpp server, and other OpenAI-compatible APIs:

```go
// pkg/toolformat/openai.go

type OpenAIFormatter struct{}

func (f *OpenAIFormatter) FormatToolsForAPI(tools []*toolbox.ToolDefinition) interface{} {
    // OpenAI function calling format
    openaiTools := make([]map[string]interface{}, len(tools))

    for i, tool := range tools {
        openaiTools[i] = map[string]interface{}{
            "type": "function",
            "function": map[string]interface{}{
                "name":        tool.Name,
                "description": tool.Description,
                "parameters":  tool.Schema,
            },
        }
    }

    return openaiTools
}
```

### 4. Formatter Selection

```go
// pkg/toolformat/selector.go

// GetFormatter returns the appropriate formatter for a model
func GetFormatter(modelName string) ToolFormatter {
    modelLower := strings.ToLower(modelName)

    switch {
    case strings.Contains(modelLower, "llama-3") || strings.Contains(modelLower, "llama3"):
        return &LlamaFormatter{version: detectLlamaVersion(modelName)}

    case strings.Contains(modelLower, "qwen"):
        return &QwenFormatter{version: detectQwenVersion(modelName)}

    case strings.Contains(modelLower, "mistral") || strings.Contains(modelLower, "mixtral"):
        return &MistralFormatter{}

    case strings.Contains(modelLower, "hermes") || strings.Contains(modelLower, "nous"):
        return &HermesFormatter{}

    case strings.Contains(modelLower, "claude"):
        return &ClaudeAPIFormatter{}

    default:
        // Generic JSON formatter for unknown models
        return &GenericFormatter{}
    }
}

// GenericFormatter uses simple JSON format that most models understand
type GenericFormatter struct{}

func (f *GenericFormatter) FormatToolsForPrompt(tools []*toolbox.ToolDefinition) string {
    var sb strings.Builder
    sb.WriteString("# Available Tools\n\n")

    for _, tool := range tools {
        sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", tool.Name, tool.Description))
    }

    sb.WriteString(`
Call tools using JSON format:
{"tool": "tool_name", "args": {"param": "value"}}
`)
    return sb.String()
}

func (f *GenericFormatter) ParseToolCalls(response string) ([]ToolCall, error) {
    return parseGenericToolCalls(response)
}

// parseGenericToolCalls handles multiple JSON formats
func parseGenericToolCalls(text string) []ToolCall {
    // Try multiple strategies (similar to current executor.go)
    // 1. Full JSON array
    // 2. Single JSON object
    // 3. Code block extraction
    // 4. Line-by-line parsing
    // ... (existing logic from executor.go)
}
```

### 5. Third-Party MCP Client (Keep)

While removing our internal MCP server, keep the ability to connect to external MCP servers as a client:

```go
// pkg/mcpclient/client.go (renamed from pkg/mcp/client.go)

// ExternalMCPClient connects to third-party MCP servers
type ExternalMCPClient struct {
    // ... existing client implementation
}

// ToToolDefinitions converts external MCP tools to our ToolDefinition format
func (c *ExternalMCPClient) ToToolDefinitions() ([]*toolbox.ToolDefinition, error) {
    mcpTools, err := c.ListTools(context.Background())
    if err != nil {
        return nil, err
    }

    definitions := make([]*toolbox.ToolDefinition, len(mcpTools))
    for i, tool := range mcpTools {
        definitions[i] = &toolbox.ToolDefinition{
            Name:        tool.Name,
            Description: tool.Description,
            Category:    CategoryUtility, // Default category for external tools
            Optionality: ToolOptional,
            Schema:      convertMCPSchema(tool.InputSchema),
            Handler:     c.createHandler(tool.Name),
        }
    }

    return definitions, nil
}

func (c *ExternalMCPClient) createHandler(toolName string) toolbox.ToolHandler {
    return func(ctx context.Context, args map[string]interface{}) (*toolbox.ToolResult, error) {
        resp, err := c.CallTool(ctx, toolName, args)
        if err != nil {
            return nil, err
        }
        return convertMCPResponse(resp), nil
    }
}
```

### 6. Updated Agent Architecture

```go
// pkg/agents/executor.go (UPDATED)

type DynamicExecutor struct {
    registry    *toolbox.ToolRegistry
    formatter   toolformat.ToolFormatter
    backend     llm.Backend
    contextMgr  *llmcontext.Manager
    maxRounds   int
    completion  *CompletionCriteria
}

func NewDynamicExecutor(
    registry *toolbox.ToolRegistry,
    backend llm.Backend,
    contextMgr *llmcontext.Manager,
) *DynamicExecutor {
    // Select formatter based on model
    modelName := backend.ModelName()
    formatter := toolformat.GetFormatter(modelName)

    return &DynamicExecutor{
        registry:   registry,
        formatter:  formatter,
        backend:    backend,
        contextMgr: contextMgr,
        maxRounds:  20,
    }
}

func (e *DynamicExecutor) Execute(ctx context.Context, initialPrompt string) error {
    // Get tools for this execution
    tools := e.registry.List()

    // Build system prompt with tool descriptions
    toolSection := e.formatter.FormatToolsForPrompt(tools)
    systemPrompt := e.buildSystemPrompt(toolSection)

    currentPrompt := initialPrompt

    for round := 0; round < e.maxRounds; round++ {
        // Execute inference
        response, err := e.backend.Infer(ctx, &llm.InferenceRequest{
            SystemPrompt: systemPrompt,
            UserPrompt:   currentPrompt,
        })
        if err != nil {
            return err
        }

        // Parse tool calls using model-specific formatter
        toolCalls, err := e.formatter.ParseToolCalls(response.Text)
        if err != nil {
            // Log but continue - might just be explanation text
        }

        if len(toolCalls) == 0 {
            if e.isDone(response.Text) {
                return nil
            }
            currentPrompt = "Please use tools to complete the task."
            continue
        }

        // Execute tools directly (no MCP!)
        results := e.executeTools(ctx, toolCalls)

        // Build feedback prompt
        currentPrompt = e.buildFeedbackPrompt(toolCalls, results)
    }

    return fmt.Errorf("max rounds reached")
}

func (e *DynamicExecutor) executeTools(ctx context.Context, calls []toolformat.ToolCall) []*toolbox.ToolResult {
    results := make([]*toolbox.ToolResult, len(calls))

    for i, call := range calls {
        // Direct execution via registry - no MCP!
        result, err := e.registry.Execute(ctx, call.Name, call.Args)
        if err != nil {
            results[i] = &toolbox.ToolResult{
                Success: false,
                Error:   err.Error(),
            }
        } else {
            results[i] = result
        }
    }

    return results
}
```

## Consequences

### Positive

1. **Simplified Architecture**: Single binary, no subprocess communication
2. **Lower Latency**: Direct function calls vs JSON-RPC over stdio
3. **Better Debugging**: Tools execute in-process, full stack traces
4. **Model-Aware Formatting**: Each model gets optimized tool formatting
5. **Rich Metadata**: Full JSON schemas available to LLM
6. **Cleaner Codebase**: Remove ~500 lines of MCP server/client code
7. **Easier Testing**: Tools can be unit tested directly

### Negative

1. **Breaking Change**: Removes MCP server binary
2. **Migration Effort**: Tools need Metadata() implementations
3. **Formatter Maintenance**: Each model format must be maintained

### Mitigation

1. **Deprecation Period**: Keep MCP server as deprecated option initially
2. **Gradual Migration**: Tools can return nil Metadata() during transition
3. **Formatter Testing**: Comprehensive tests for each model format

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1-2)

1. Create `pkg/toolbox/` package
   - `definition.go` - ToolDefinition struct
   - `registry.go` - ToolRegistry implementation
   - `bundles.go` - Predefined tool bundles
   - `result.go` - ToolResult struct

2. Create `pkg/toolformat/` package
   - `formatter.go` - ToolFormatter interface
   - `generic.go` - Generic JSON formatter
   - `selector.go` - Formatter selection logic

3. Update `pkg/tools/interface.go`
   - Add ExtendedTool interface
   - Add backward-compatible wrapper

### Phase 2: Tool Migration (Week 2-3)

1. Update each tool to implement Metadata():
   - `file.go`
   - `code_edit.go`
   - `search.go`
   - `navigate.go`
   - `git.go`
   - `bash.go`
   - `test.go`
   - `rss.go`
   - `calendar.go`
   - `static_links.go`
   - `blog_notion.go`
   - `webscrape.go`

2. Define JSON schemas for each tool's parameters

3. Assign categories and optionality

### Phase 3: Model Formatters (Week 3-4)

1. Implement formatters:
   - `qwen.go` (primary - most used)
   - `llama.go` (Llama 3.x)
   - `mistral.go`
   - `claude.go` (for API mode)
   - `openai.go` (generic fallback)

2. Add formatter tests with real model outputs

### Phase 4: Agent Integration (Week 4-5)

1. Create `DynamicExecutor` using new architecture
2. Update `BaseAgent` to use registry
3. Update agent constructors (Builder, Debugger, etc.)
4. Update HTTP handlers to use new architecture
5. Update CLI commands

### Phase 5: MCP Removal (Week 5-6)

1. Rename `pkg/mcp/client.go` to `pkg/mcpclient/` (for external MCP)
2. Delete `pkg/mcp/server.go`
3. Delete `pkg/mcp/agent_tool.go`
4. Delete `cmd/mcp-server/`
5. Update Makefile to remove mcp-server build
6. Update documentation

### Phase 6: Testing & Cleanup (Week 6-7)

1. Integration tests with each supported model
2. Performance benchmarks (before/after MCP)
3. Update CLAUDE.md
4. Update docs/
5. Remove deprecated code

## File Changes Summary

### New Files

```
pkg/toolbox/
├── definition.go     # ToolDefinition, JSONSchema
├── registry.go       # ToolRegistry
├── bundles.go        # Tool bundles
└── result.go         # ToolResult

pkg/toolformat/
├── formatter.go      # ToolFormatter interface
├── generic.go        # Generic JSON formatter
├── llama.go          # Llama 3.x formatter
├── qwen.go           # Qwen 2.5 formatter
├── mistral.go        # Mistral formatter
├── claude.go         # Claude API formatter
├── openai.go         # OpenAI-compatible formatter
├── hermes.go         # Hermes/Nous formatter
└── selector.go       # Formatter selection

pkg/mcpclient/
└── client.go         # External MCP client (renamed)
```

### Modified Files

```
pkg/tools/interface.go       # Add ExtendedTool interface
pkg/tools/*.go               # Add Metadata() to each tool
pkg/agents/executor.go       # Use new architecture
pkg/agents/base.go           # Use registry
pkg/agents/builder.go        # Update constructor
pkg/agents/debugger.go       # Update constructor
pkg/agents/reviewer.go       # Update constructor
pkg/agents/triager.go        # Update constructor
pkg/agents/blog_orchestrator.go  # Update to dynamic
cmd/pedrocli/main.go         # Remove MCP client usage
cmd/http-server/main.go      # Remove MCP client usage
```

### Deleted Files

```
pkg/mcp/server.go            # MCP server
pkg/mcp/agent_tool.go        # Agent wrapper
cmd/mcp-server/main.go       # MCP server binary
```

## Migration Path for Existing Users

1. **v0.x → v1.0**: MCP server deprecated but still works
2. **v1.0 → v1.1**: MCP server removed, use direct tool calls
3. **External MCP**: Use `--mcp-server` flag to connect to external servers

## Related ADRs

This ADR supersedes the MCP-related portions of:
- **ADR-001**: Dynamic Tool Registry (keep registry, remove MCP integration)
- **ADR-002**: LLM Tool Awareness (keep prompt generation, add formatters)
- **ADR-003**: Dynamic Tool Invocation (keep executor, remove MCP calls)
- **ADR-004**: Logit Control (unchanged)
- **ADR-005**: Agent Workflow Refactoring (unchanged)
- **ADR-006**: Optional Tool Catalog (unchanged)
