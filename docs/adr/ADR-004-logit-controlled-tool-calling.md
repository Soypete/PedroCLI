# ADR-004: Logit-Controlled Tool Calling

## Status

Proposed

## Context

### Current Tool Call Parsing (`pkg/agents/executor.go:126-173`)

Tool calls are extracted from free-form LLM output using multiple parsing strategies:

```go
func (e *InferenceExecutor) parseToolCalls(text string) []llm.ToolCall {
    // Strategy 1: Entire response as JSON array
    var arrayOfCalls []llm.ToolCall
    if err := json.Unmarshal([]byte(text), &arrayOfCalls); err == nil {
        return e.filterValidCalls(arrayOfCalls)
    }

    // Strategy 2: Single JSON object
    var singleCall llm.ToolCall
    if err := json.Unmarshal([]byte(text), &singleCall); err == nil {
        return []llm.ToolCall{singleCall}
    }

    // Strategy 3: Extract from code blocks
    calls := e.extractFromCodeBlocks(text)

    // Strategy 4: Line-by-line JSON
    calls = e.extractFromLines(text)

    return calls
}
```

### Problems

1. **Hallucinated Tool Names**: LLM can invent tools that don't exist.

2. **Invalid Parameters**: LLM can use wrong parameter names or types.

3. **Malformed JSON**: LLM may produce syntactically invalid JSON.

4. **Mixed Content**: Explanation text mixed with tool calls complicates parsing.

5. **Inconsistent Format**: Different models format tool calls differently.

### Existing Logit Infrastructure (`pkg/logits/`)

PedroCLI already has sophisticated logit manipulation:

- **GBNF Grammars** (`grammar.go`): Parse and enforce grammar constraints
- **JSON Schema to GBNF** (`schema.go`): Convert schemas to grammars
- **Tool Call Filter** (`toolcall.go`): State machine for tool call format
- **Multi-Tool Grammar** (`schema.go:473-481`): Grammar allowing any registered tool

However, this infrastructure is not connected to the agent execution loop.

## Decision

### 1. Grammar-Constrained Tool Generation

Use GBNF grammars to ensure LLM output is always valid:

```go
// pkg/agents/constrained_executor.go

type ConstrainedExecutor struct {
    *DynamicExecutor
    grammarFilter *logits.ToolCallFilter
    useGrammar    bool  // Feature flag for gradual rollout
}

func NewConstrainedExecutor(agent *BaseAgent, registry *tools.ToolRegistry) (*ConstrainedExecutor, error) {
    // Build tool definitions from registry
    var toolDefs []*logits.ToolDefinition
    for _, tool := range registry.List() {
        meta := tool.Metadata()
        if meta != nil && meta.Schema != nil {
            toolDefs = append(toolDefs, &logits.ToolDefinition{
                Name:        tool.Name(),
                Description: tool.Description(),
                Parameters:  meta.Schema,
            })
        }
    }

    // Create filter for these tools
    filter, err := logits.NewToolCallFilter(toolDefs, backend.GetTokenizer())
    if err != nil {
        return nil, fmt.Errorf("create tool filter: %w", err)
    }

    return &ConstrainedExecutor{
        DynamicExecutor: NewDynamicExecutor(agent, registry),
        grammarFilter:   filter,
        useGrammar:      true,
    }, nil
}
```

### 2. Generation Modes

Support different generation modes based on backend capabilities:

```go
type GenerationMode string

const (
    // ModeUnconstrained - Parse tool calls from free-form output
    ModeUnconstrained GenerationMode = "unconstrained"

    // ModeGrammarConstrained - Use GBNF grammar to constrain output
    ModeGrammarConstrained GenerationMode = "grammar"

    // ModeSchemaConstrained - Use JSON Schema for validation
    ModeSchemaConstrained GenerationMode = "schema"

    // ModeHybrid - Try constrained, fall back to unconstrained
    ModeHybrid GenerationMode = "hybrid"
)

func (e *ConstrainedExecutor) infer(ctx context.Context, prompt string, state *ExecutionState) (*llm.InferenceResponse, error) {
    switch e.mode {
    case ModeGrammarConstrained:
        return e.inferWithGrammar(ctx, prompt, state)
    case ModeSchemaConstrained:
        return e.inferWithSchema(ctx, prompt, state)
    case ModeHybrid:
        return e.inferHybrid(ctx, prompt, state)
    default:
        return e.DynamicExecutor.infer(ctx, prompt, state)
    }
}
```

### 3. Grammar Generation from Registry

Generate grammar dynamically based on registered tools:

```go
// pkg/logits/registry_grammar.go

func GenerateToolCallGrammar(registry *tools.ToolRegistry) (*GBNF, error) {
    // Collect tool schemas
    schemas := make(map[string]*JSONSchema)
    for _, tool := range registry.List() {
        meta := tool.Metadata()
        if meta != nil && meta.Schema != nil {
            schemas[tool.Name()] = meta.Schema
        }
    }

    // Generate multi-tool schema
    schema := MultiToolCallSchema(schemas)

    // Convert to GBNF
    grammarStr, err := SchemaToGBNF(schema)
    if err != nil {
        return nil, err
    }

    return ParseGBNF(grammarStr)
}

// Also support text-with-tool-call format
func GenerateTextOrToolGrammar(registry *tools.ToolRegistry) (*GBNF, error) {
    toolGrammar, err := GenerateToolCallGrammar(registry)
    if err != nil {
        return nil, err
    }

    // Allow either text content or tool call
    combinedGrammar := fmt.Sprintf(`
root ::= text | tool_call | text tool_call

text ::= [^{]+

tool_call ::= %s
`, toolGrammar.String())

    return ParseGBNF(combinedGrammar)
}
```

### 4. Logit Bias for Tool Selection

Use logit bias to encourage/discourage specific tools:

```go
// pkg/logits/tool_bias.go

type ToolBiasConfig struct {
    // Encouragements increase probability of tool selection
    Encouragements map[string]float32

    // Discouragements decrease probability
    Discouragements map[string]float32

    // BlockedTools are completely prevented
    BlockedTools []string
}

func NewToolBiasFilter(config *ToolBiasConfig, tokenizer Tokenizer) *ToolBiasFilter {
    filter := &ToolBiasFilter{
        config:    config,
        tokenizer: tokenizer,
    }

    // Pre-compute token IDs for tool names
    for name, bias := range config.Encouragements {
        tokenIDs := tokenizer.Encode(fmt.Sprintf(`"%s"`, name))
        filter.encourageTokens[name] = tokenIDs
        filter.encourageBias[name] = bias
    }

    return filter
}

func (f *ToolBiasFilter) Apply(logits []float32, ctx *GenerationContext) []float32 {
    // Apply encouragements
    for name, tokenIDs := range f.encourageTokens {
        bias := f.encourageBias[name]
        for _, id := range tokenIDs {
            logits[id] += bias
        }
    }

    // Apply blocks (set to negative infinity)
    for _, name := range f.config.BlockedTools {
        tokenIDs := f.tokenizer.Encode(fmt.Sprintf(`"%s"`, name))
        for _, id := range tokenIDs {
            logits[id] = NegativeInfinity
        }
    }

    return logits
}
```

### 5. Context-Aware Biasing

Adjust biases based on execution state:

```go
func (e *ConstrainedExecutor) computeBias(state *ExecutionState) *ToolBiasConfig {
    config := &ToolBiasConfig{
        Encouragements:  make(map[string]float32),
        Discouragements: make(map[string]float32),
    }

    // Encourage tools whose inputs are available
    for _, tool := range e.registry.List() {
        meta := tool.Metadata()
        if meta != nil && e.areInputsAvailable(meta.Consumes, state) {
            config.Encouragements[tool.Name()] = 0.5
        }
    }

    // Discourage already-called tools (prevent loops)
    for name, count := range state.ToolsCalled {
        if count > 2 {
            config.Discouragements[name] = -1.0 * float32(count)
        }
    }

    // Encourage completion if criteria met
    if e.canComplete(state) {
        // Bias toward not calling tools (let LLM complete)
        for _, tool := range e.registry.List() {
            config.Discouragements[tool.Name()] = -0.3
        }
    }

    return config
}
```

### 6. Validation and Recovery

Handle edge cases gracefully:

```go
func (e *ConstrainedExecutor) inferWithGrammar(ctx context.Context, prompt string, state *ExecutionState) (*llm.InferenceResponse, error) {
    // Generate grammar for current tool set
    grammar, err := GenerateToolCallGrammar(e.registry)
    if err != nil {
        // Fall back to unconstrained
        return e.inferUnconstrained(ctx, prompt, state)
    }

    // Compute context-aware bias
    bias := e.computeBias(state)

    // Build inference request
    req := &llm.ConstrainedInferenceRequest{
        InferenceRequest: &llm.InferenceRequest{
            SystemPrompt: e.buildSystemPrompt(),
            UserPrompt:   prompt,
            Temperature:  e.config.Model.Temperature,
        },
        Grammar:   grammar.String(),
        LogitBias: bias,
    }

    resp, err := e.backend.InferConstrained(ctx, req)
    if err != nil {
        // Fall back to unconstrained
        return e.inferUnconstrained(ctx, prompt, state)
    }

    // Validate output (should always pass with grammar)
    if !e.isValidToolCall(resp.Text) {
        // This shouldn't happen, log for debugging
        log.Printf("Grammar produced invalid output: %s", resp.Text)
        return e.inferUnconstrained(ctx, prompt, state)
    }

    return resp, nil
}
```

### 7. Backend Integration

Extend LLM backend interface:

```go
// pkg/llm/interface.go

type ConstrainedInferenceRequest struct {
    *InferenceRequest

    // Grammar is GBNF grammar string
    Grammar string

    // JSONSchema is alternative to grammar
    JSONSchema *logits.JSONSchema

    // LogitBias adjusts token probabilities
    LogitBias *ToolBiasConfig
}

type Backend interface {
    Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)

    // InferConstrained performs inference with constraints
    InferConstrained(ctx context.Context, req *ConstrainedInferenceRequest) (*InferenceResponse, error)

    // SupportsConstraints indicates if backend supports constrained inference
    SupportsConstraints() bool

    GetContextWindow() int
    GetUsableContext() int
}
```

## Consequences

### Positive

1. **Guaranteed Valid Format**: Tool calls are always syntactically correct.

2. **No Hallucinated Tools**: Only registered tools can be invoked.

3. **Type Safety**: Parameter types are enforced by schema.

4. **Simplified Parsing**: No need for multiple parsing strategies.

5. **Intelligent Biasing**: Context-aware probability adjustments improve tool selection.

6. **Graceful Degradation**: Falls back to unconstrained if constraints fail.

### Negative

1. **Backend Requirements**: Not all backends support grammar constraints.

2. **Performance Overhead**: Grammar enforcement adds latency.

3. **Reduced Flexibility**: LLM cannot explain before/after tool calls (in pure grammar mode).

4. **Schema Maintenance**: All tools must have accurate schemas.

### Mitigation

1. **Hybrid Mode**: Combine grammar with fallback for maximum compatibility.

2. **Text-Plus-Tool Grammar**: Allow explanatory text before tool calls.

3. **Schema Validation**: CI checks ensure schemas match implementations.

4. **Caching**: Cache generated grammars for registered tool sets.

## Implementation

### Phase 1: Backend Support

1. Add `InferConstrained` to llama.cpp backend
2. Add `SupportsConstraints` capability detection
3. Pass grammar/bias to llama-server

### Phase 2: Grammar Generation

1. Implement `GenerateToolCallGrammar`
2. Implement `GenerateTextOrToolGrammar`
3. Add caching for generated grammars

### Phase 3: Executor Integration

1. Create `ConstrainedExecutor`
2. Implement generation modes
3. Add fallback logic

### Phase 4: Biasing

1. Implement `ToolBiasFilter`
2. Add context-aware bias computation
3. Integrate with execution loop

### Phase 5: Testing

1. Test with various model sizes
2. Measure latency impact
3. Verify fallback works correctly

## Example: Generated Grammar

For tools: `file`, `search`, `code_edit`

```
root ::= tool_call

tool_call ::= file_call | search_call | code_edit_call

file_call ::= "{" ws "\"name\"" ws ":" ws "\"file\"" ws "," ws "\"args\"" ws ":" ws file_args ws "}"

file_args ::= "{" ws "\"action\"" ws ":" ws file_action ws ("," ws "\"path\"" ws ":" ws string)? ("," ws "\"content\"" ws ":" ws string)? ws "}"

file_action ::= "\"read\"" | "\"write\"" | "\"list\""

search_call ::= "{" ws "\"name\"" ws ":" ws "\"search\"" ws "," ws "\"args\"" ws ":" ws search_args ws "}"

search_args ::= "{" ws "\"action\"" ws ":" ws search_action ws ("," ws "\"pattern\"" ws ":" ws string)? ("," ws "\"path\"" ws ":" ws string)? ws "}"

search_action ::= "\"grep\"" | "\"find_files\"" | "\"find_definitions\""

code_edit_call ::= ...

string ::= "\"" ([^"\\] | "\\" [\"\\/bfnrt] | "\\u" [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F] [0-9a-fA-F])* "\""

ws ::= [ \t\n\r]*
```

## Related ADRs

- **ADR-001**: Dynamic Tool Registry (provides schemas for grammar generation)
- **ADR-003**: Dynamic Tool Invocation (uses constrained executor)
- **ADR-005**: Agent Workflow Refactoring (applies constraints to agents)
