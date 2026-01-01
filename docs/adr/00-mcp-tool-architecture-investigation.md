# MCP Tool Architecture Investigation

This document captures the investigation findings for the dynamic MCP tool architecture initiative. It serves as the foundation for ADRs 001-006.

## Current Tool Registration

### File Locations

| Component | Location | Purpose |
|-----------|----------|---------|
| Tool Interface | `pkg/tools/interface.go` | Defines `Tool` interface |
| MCP Server | `pkg/mcp/server.go` | JSON-RPC server, tool registry |
| Agent Tool Wrapper | `pkg/mcp/agent_tool.go` | Wraps agents as MCP tools |
| Tool Implementations | `pkg/tools/*.go` | Individual tool implementations |
| Server Entry Point | `cmd/mcp-server/main.go` | Tool and agent registration |
| Agent Base | `pkg/agents/base.go` | Agent tool registry |
| Inference Executor | `pkg/agents/executor.go` | Tool call parsing and execution |

### Registration Pattern

Tools are registered at two levels:

#### 1. MCP Server Level (`pkg/mcp/server.go:52-55`)

```go
// Server maintains a tool map
type Server struct {
    tools  map[string]tools.Tool
    stdin  io.Reader
    stdout io.Writer
}

// RegisterTool adds tool to server's registry
func (s *Server) RegisterTool(tool tools.Tool) {
    s.tools[tool.Name()] = tool
}
```

#### 2. Agent Level (`pkg/agents/base.go:57-60`)

```go
// BaseAgent maintains its own tool map
type BaseAgent struct {
    tools map[string]tools.Tool
    // ... other fields
}

func (a *BaseAgent) RegisterTool(tool tools.Tool) {
    a.tools[tool.Name()] = tool
}
```

#### Registration in `cmd/mcp-server/main.go`

```go
// Create tools
fileTool := tools.NewFileTool()
gitTool := tools.NewGitTool(workDir)
bashTool := tools.NewBashTool(cfg, workDir)
// ... more tools

// Register to MCP server
server.RegisterTool(fileTool)
server.RegisterTool(gitTool)
// ...

// Create agent and register same tools
builderAgent := agents.NewBuilderAgent(cfg, backend, jobManager)
builderAgent.RegisterTool(fileTool)
builderAgent.RegisterTool(gitTool)
// ... same tools registered to each agent
```

### Tool Schema Format

Current tool interface (`pkg/tools/interface.go`):

```go
type Tool interface {
    Name() string
    Description() string
    Execute(ctx context.Context, args map[string]interface{}) (*Result, error)
}
```

**Key Observation**: No parameter schema is exposed. Tools are described only by name and description. Parameter validation happens inside `Execute()`.

### MCP Protocol for Tool Discovery

The `tools/list` method (`pkg/mcp/server.go:112-125`) returns available tools:

```go
func (s *Server) handleToolsList(req *Request) {
    toolsList := make([]map[string]interface{}, 0, len(s.tools))
    for _, tool := range s.tools {
        toolsList = append(toolsList, map[string]interface{}{
            "name":        tool.Name(),
            "description": tool.Description(),
        })
    }
    s.sendResponse(req.ID, map[string]interface{}{"tools": toolsList})
}
```

**Missing**: Parameter schemas are not exposed to clients.

### Tool Call Format

LLMs produce tool calls as JSON (`pkg/agents/executor.go:126-173`):

```json
{"tool": "tool_name", "args": {"key": "value"}}
// or
{"name": "tool_name", "args": {"key": "value"}}
```

The executor parses these with multiple strategies:
1. Full response as JSON array
2. Single JSON object
3. Extract from markdown code blocks
4. Line-by-line JSON parsing

### Tool Result Format

```go
type Result struct {
    Success       bool                   `json:"success"`
    Output        string                 `json:"output"`
    Error         string                 `json:"error,omitempty"`
    ModifiedFiles []string               `json:"modified_files,omitempty"`
    Data          map[string]interface{} `json:"data,omitempty"`
}
```

Results are fed back to the LLM in the next prompt iteration.

---

## Current Workflow Analysis

### Blog Orchestrator Workflow (`pkg/agents/blog_orchestrator.go:155-231`)

**7 Hardcoded Phases:**

```
Phase 1: analyzePrompt()      - Parse user prompt into structured BlogPromptAnalysis
Phase 2: executeResearch()    - Run research tools (calendar, RSS, static links)
Phase 3: generateOutline()    - Create markdown outline
Phase 4: expandSections()     - Expand each section independently
Phase 5: (inline)             - Build newsletter from research data
Phase 6: generateSocialPosts() - Create Twitter/LinkedIn/Bluesky posts
Phase 7: publishToNotion()    - Publish to Notion if requested
```

**Problems:**
- Phases are sequential and hardcoded - no dynamic selection
- Research tools are invoked procedurally, not by LLM choice
- LLM generates content but doesn't control workflow
- No optional tool invocation based on context

### Code Agent Workflow (`pkg/agents/builder.go`, `pkg/agents/executor.go`)

Uses `InferenceExecutor.Execute()` - an iterative loop:

```
1. Send prompt to LLM
2. Parse tool calls from response
3. Execute tools
4. Feed results back to LLM
5. Repeat until TASK_COMPLETE or max iterations
```

**Better but still limited:**
- LLM has agency to choose which tools to use
- But all registered tools are always available
- No concept of required vs optional tools
- No dynamic tool discovery mid-session

### Tool Registration Per Agent Type

| Agent | Tools |
|-------|-------|
| Builder | file, code_edit, search, navigate, git, bash, test |
| Reviewer | file, code_edit, search, navigate, git, bash, test |
| Debugger | file, code_edit, search, navigate, git, bash, test |
| Triager | file, code_edit, search, navigate, git, bash, test |
| BlogOrchestrator | rss_feed, static_links, (calendar), blog_notion |

All code agents have identical tool sets. No differentiation.

---

## Logit Manipulation Integration

### Package Structure (`pkg/logits/`)

| File | Purpose |
|------|---------|
| `grammar.go` | GBNF grammar parsing (llama.cpp format) |
| `schema.go` | JSON Schema → GBNF conversion |
| `toolcall.go` | Tool call format enforcement |
| `filter.go` | Logit filter interface |
| `sampler.go` | Sampling configuration |
| `chain.go` | Filter chaining |
| `safety.go` | Safety-related filtering |

### GBNF Grammar for Tool Calls (`pkg/logits/grammar.go:599-607`)

```
root       ::= "{" ws "\"name\"" ws ":" ws string ws "," ws "\"args\"" ws ":" ws object ws "}"
object     ::= "{" ws (string ws ":" ws value (ws "," ws string ws ":" ws value)*)? ws "}"
// ... JSON primitives
```

### JSON Schema to GBNF (`pkg/logits/schema.go:51-104`)

Converts JSON Schema definitions to GBNF grammars for constrained generation:

```go
func SchemaToGBNF(schema *JSONSchema) (string, error) {
    // Recursively converts schema types to GBNF rules
    // Supports: object, array, string, number, integer, boolean, null
    // Handles: enum, const, oneOf, anyOf, $ref
}
```

### Multi-Tool Grammar (`pkg/logits/schema.go:473-481`)

Creates a grammar that allows any of the registered tools:

```go
func MultiToolCallSchema(tools map[string]*JSONSchema) *JSONSchema {
    var oneOf []*JSONSchema
    for name, argsSchema := range tools {
        oneOf = append(oneOf, ToolCallSchema(name, argsSchema))
    }
    return &JSONSchema{OneOf: oneOf}
}
```

### ToolCallFilter (`pkg/logits/toolcall.go:86-103`)

Stateful filter that enforces tool call format:

```go
type ToolCallFilter struct {
    tools     map[string]*ToolDefinition
    tokenizer Tokenizer
    parseState    ToolParseState  // State machine tracking
    currentTool   *ToolDefinition
    argsGrammarFilter *GrammarFilter
    multiToolGrammar *GBNF
}
```

State machine:
1. `StateExpectingStart` - Only allow `{`
2. `StateExpectingToolName` - Only allow `"name"`
3. `StateExpectingNameValue` - Only allow valid tool names
4. `StateExpectingArgs` - Only allow `"args"`
5. `StateParsingArgs` - Delegate to tool's args grammar
6. `StateExpectingEnd` - Only allow `}`
7. `StateComplete` - Only allow EOS

### LogitTool (`pkg/tools/logit.go`)

Exposes logit control as a tool:
- `generate` - Generate with logit control
- `generate_structured` - Generate JSON matching schema
- `generate_tool_call` - Generate guaranteed-format tool call
- `test_config` - Test logit configurations
- `list_presets` - List available presets

---

## Problem Statement

### Current State

```
User Request → Agent → Step 1 → Step 2 → Step 3 → ... → Step 7 → Output
                        (rigid, predetermined sequence)
```

For blog orchestrator, all 7 phases execute in order regardless of whether they're needed.

### Desired State

```
User Request → Agent → [Thinks about what's needed]
                     → Calls Tool A (required)
                     → Calls Tool B (optional, decided by LLM)
                     → Skips Tool C (not needed)
                     → Calls Tool D (optional, decided by LLM)
                     → Output
```

The LLM should:
1. Know all available tools (required and optional)
2. Understand when optional tools would be helpful
3. Be able to invoke tools dynamically during generation
4. Handle tool results and continue reasoning

---

## Key Gaps Identified

### 1. No Tool Metadata

Tools lack:
- Parameter schemas (for LLM understanding)
- Required vs optional classification
- Category/grouping information
- Usage context hints

### 2. No Dynamic Tool Discovery

- Tools are registered at startup
- No way to add/remove tools mid-session
- No per-request tool customization

### 3. Rigid Workflow Orchestration

- Blog orchestrator hardcodes phase sequence
- No LLM agency over workflow decisions
- Research tools invoked procedurally, not dynamically

### 4. Disconnected Logit Control

- Logit manipulation layer exists but isn't integrated
- Tool call grammars can constrain output but aren't used by agents
- No connection between tool registry and grammar generation

### 5. No Tool Context in Prompts

- System prompts list tools statically
- No guidance on when to use optional tools
- No examples of tool invocation patterns

---

## Recommendations Summary

1. **ADR-001**: Extend tool interface with metadata (schema, optionality, category)
2. **ADR-002**: Enhance system prompts to communicate available tools with context
3. **ADR-003**: Replace rigid phases with dynamic tool invocation loop
4. **ADR-004**: Integrate logit constraints to ensure valid tool calls
5. **ADR-005**: Refactor blog/podcast/code agents to dynamic pattern
6. **ADR-006**: Define catalog of optional tools with usage guidelines
