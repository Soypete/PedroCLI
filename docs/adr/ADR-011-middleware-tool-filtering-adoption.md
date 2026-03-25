# ADR-011: Middleware Library Adoption for Comprehensive Agent Middleware Capabilities

## Status
Accepted

**Last Updated**: 2026-03-25

## Summary

This ADR documents the adoption of the middleware library (`github.com/soypete/pedro-agentware/middleware`) for comprehensive agent middleware capabilities including policy evaluation, result filtering, tool tracking, format/formatter support, and context/window management with compaction strategies. The ADR supersedes the original narrow scope of "tool filtering" to reflect the holistic middleware capabilities discovered during exploration.

## Context

The phased executor (`pkg/agents/phased_executor.go`) currently implements phase-based tool filtering with two standalone functions:

1. **`filterToolCalls()`** (lines 725-749): Filters tool calls to only allowed tools for the current phase using a whitelist approach
2. **`filterToolDefinitions()`** (lines 751-789): Filters tool definitions to only allowed tools for the current phase

These functions are called at lines 618 and 691 respectively.

A middleware library (`github.com/soypete/pedro-agentware/middleware`) exists that provides significantly more comprehensive middleware capabilities beyond tool filtering. This ADR documents the full scope of these capabilities and their applicability to PedroCLI.

## Middleware Library Comprehensive Analysis

The middleware library provides five core capability areas:

### 1. Policy Evaluation (`middleware/policy.go`)

The `PolicyEvaluator` interface provides fine-grained control over agent behavior:

```go
type PolicyEvaluator interface {
    Evaluate(ctx context.Context, req *EvaluationRequest) (*EvaluationResult, error)
    FilterResult(ctx context.Context, result *Result) (*Result, error)
    FilterToolList(ctx context.Context, tools []Tool) ([]Tool, error)
}
```

**Key Features**:
- **Rate Limiting**: Throttle tool calls per time window
- **Max Turns**: Limit total inference iterations
- **Iteration Limits**: Per-phase or global iteration caps
- **Deny/Filter Actions**: Two action types - `Deny` blocks entirely, `Filter` removes from list
- **Field Redaction**: Remove sensitive fields from results (e.g., redact API keys from tool outputs)

**EvaluationRequest Fields**:
- `CallerID`: Unique identifier for the caller
- `Phase`: Current execution phase
- `ToolName`: Tool being evaluated
- `Iteration`: Current iteration number
- `TurnCount`: Total turns consumed

### 2. Result Filtering (`middleware/policy.go:140-160`)

The `FilterResult()` method can redact fields based on configured rules:

```go
func (pe *PolicyEvaluator) FilterResult(ctx context.Context, result *Result) (*Result, error) {
    // Redact sensitive fields based on policy rules
    for _, rule := range pe.redactionRules {
        if rule.Matches(result) {
            result = rule.Apply(result)
        }
    }
    return result, nil
}
```

**Use Cases**:
- Remove API keys/credentials from tool outputs
- Filter sensitive user data
- Sanitize error messages for public consumption

### 3. Tool Tracking / CallHistory (`middleware/middleware.go:15-95`)

The `CallHistory` struct provides comprehensive tool usage tracking:

```go
type CallHistory struct {
    mu           sync.RWMutex
    CalledTools  map[string]bool  // "phaseName:toolName" -> true
    FailedTools  map[string]int   // "phaseName:toolName" -> failure count
}
```

**Key Features**:
- **Per-Phase Tracking**: Uses "phaseName:toolName" keys for phase-specific tracking
- **Failed Tool Exclusion**: Automatically excludes tools that failed 3+ times from retry
- **Called Tools Map**: Tracks which tools have been invoked per phase
- **Failure Counting**: Counts failures per tool per phase for smart retry logic

**Automatic Behavior**:
- When a tool fails, `FailedTools["phase:tool"]` increments
- Before executing a tool, checks if `FailedTools["phase:tool"] >= 3`
- If threshold exceeded, tool is skipped with warning

### 4. Format/Formatter Support (`middleware/format/formatter.go`)

The `ToolFormatter` interface supports multiple model families with model-specific tool call formatting:

```go
type ToolFormatter interface {
    FormatTools(tools []ToolDefinition) (string, error)
    FormatResult(result *Result) (string, error)
    ParseToolCalls(response string) ([]ToolCall, error)
    ModelFamily() ModelFamily
}

type ModelFamily string

const (
    ModelFamilyLlama3  ModelFamily = "llama3"
    ModelFamilyQwen    ModelFamily = "qwen"
    ModelFamilyMistral ModelFamily = "mistral"
    ModelFamilyClaude  ModelFamily = "claude"
    ModelFamilyOpenAI  ModelFamily = "openai"
    ModelFamilyGLM4    ModelFamily = "glm4"
)
```

**Capabilities**:
- **Model-Specific Formatting**: Each model family has distinct tool call syntax
  - **Llama3**: Uses `<|python_tag|>` format
  - **Qwen**: Uses `<tool_call>` XML-style tags
  - **Mistral**: Uses `[TOOL_CALLS]` JSON array format
  - **Claude**: Uses XML tool_use blocks
  - **OpenAI**: Uses standard function calling JSON
  - **GLM4**: Similar to OpenAI with Chinese-optimized prompts

- **Bidirectional**: Formats tools FOR the model AND parses tool calls FROM model responses
- **Extensible**: New model families can be added by implementing the interface

### 5. Context/Window Management with Compaction (`middleware/windowmanager/windowmanager.go`)

The `ContextWindowManager` provides intelligent context management:

```go
type ContextWindowManager struct {
    model     ModelSpec
    strategy  CompactionStrategy
    counter   TokenCounter
    mu        sync.RWMutex
    lastCheck time.Time
}

type CompactionStrategy interface {
    Compact(messages []Message, targetTokens int, counter TokenCounter) ([]Message, error)
    Name() string
}
```

**Compaction Strategies**:

1. **LastNCompaction**: Keeps the most recent N messages
   - Configurable `KeepCount`
   - Iteratively reduces until target tokens met
   - Always keeps at least 1 message

2. **SummaryCompaction**: Summarizes old messages using LLM
   - Extracts system message separately
   - Creates summarized "[Previous conversation summarized: ...]" message
   - Preserves recent messages for context

3. **PriorityBasedCompaction**: Keeps messages by priority
   - **PrioritySystem** (highest): System prompts
   - **PriorityHigh**: Assistant messages
   - **PriorityMedium**: User messages  
   - **PriorityLow**: Other messages
   - Bubble sorts by priority, keeps highest until token limit

**Token Management**:
- `TokenCounter` interface for custom tokenization
- `DefaultCounter`: Simple character-based estimation (chars/4 + overhead)
- `ContextStatus` with warning levels: None, Low, Medium, High, Critical
- Auto-triggers compaction at 25% remaining threshold

## Current Implementation (Phased Executor)

```go
// filterToolCalls filters tool calls to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolCalls(calls []llm.ToolCall) []llm.ToolCall {
    if len(pie.phase.Tools) == 0 {
        return calls
    }

    allowedSet := make(map[string]bool)
    for _, t := range pie.phase.Tools {
        allowedSet[t] = true
    }

    filtered := make([]llm.ToolCall, 0)
    for _, call := range calls {
        if allowedSet[call.Name] {
            filtered = append(filtered, call)
        } else {
            fmt.Fprintf(os.Stderr, "   Tool %s not allowed in phase %s, skipping\n", call.Name, pie.phase.Name)
        }
    }

    return filtered
}

// filterToolDefinitions filters tool definitions to only allowed tools for this phase
func (pie *phaseInferenceExecutor) filterToolDefinitions(defs []llm.ToolDefinition) []llm.ToolDefinition {
    if len(pie.phase.Tools) == 0 {
        return defs
    }

    allowedSet := make(map[string]bool)
    for _, toolName := range pie.phase.Tools {
        allowedSet[toolName] = true
    }

    filtered := make([]llm.ToolDefinition, 0, len(pie.phase.Tools))
    for _, def := range defs {
        if allowedSet[def.Name] {
            filtered = append(filtered, def)
        }
    }

    return filtered
}
```

## Decision

**Adopt the middleware library for NEW agents/features while maintaining existing functions for phased executor.**

### Rationale

1. **Comprehensive capabilities**: The middleware library provides value beyond tool filtering - policy evaluation, result filtering, context management, and multi-model formatter support.

2. **Migration complexity**: The middleware library doesn't have direct function equivalents for our current use case. Wrapping would require:
   - Converting between `llm.ToolDefinition` and middleware's `Tool` types
   - Implementing a custom `PolicyEvaluator` that mimics current phase-based restrictions
   - Significant refactoring with marginal benefit for existing agents

3. **Existing ADR-008**: The middleware pattern was already proposed for compaction (ADR-008), but only partial implementation (truncation) was completed. This suggests conservative adoption is appropriate.

4. **Consistency across agents**: New agents can use the middleware pattern directly, while existing agents retain their current implementation.

5. **Model-family flexibility**: The formatter support enables easier model switching without rewriting tool-calling logic.

## Holistic Impact Analysis

### Pros of Middleware Adoption

1. **Policy Enforcement**: Centralized policy evaluation enables consistent enforcement of rate limits, turn limits, and access controls across all agents.

2. **Result Filtering**: Built-in field redaction improves security posture by preventing credential leakage in logs/outputs.

3. **Tool Tracking**: Automatic failure tracking and exclusion reduces agent loops from broken tools.

4. **Multi-Model Support**: Format/formatter abstraction enables quick switching between model families (Llama3, Qwen, Mistral, Claude, OpenAI, GLM4) without code changes.

5. **Context Compaction**: Three distinct strategies (LastN, Summary, Priority) provide flexibility for different use cases.

6. **Composable**: Middleware can be stacked (logging, metrics, auth) for cross-cutting concerns.

7. **Consistency**: New agents follow the same pattern as ADR-008's middleware proposal.

### Cons of Middleware Adoption

1. **Dual Patterns**: Current functions in phased executor, middleware in new agents - two patterns to maintain.

2. **No Direct Replacement**: Middleware requires wrapping/custom policy evaluator for simple whitelist logic.

3. **Overhead**: PolicyEvaluator adds complexity for simple use cases.

4. **Type Conversion**: Requires conversion between `llm.ToolDefinition` and middleware's `Tool` type.

5. **Learning Curve**: Team must understand middleware patterns vs. direct implementation.

### Shortcomings / Risks

1. **Incomplete Integration Path**: No clear migration guide from current implementation to middleware.

2. **Testing Burden**: New middleware components require comprehensive test coverage.

3. **Dependency**: Adds external dependency (`github.com/soypete/pedro-agentware/middleware`) that must be maintained.

4. **Over-Engineering Risk**: For simple agents, middleware overhead may exceed benefit.

5. **Window Manager Limitations**: 
   - SummaryCompaction requires external LLM call (not included)
   - TokenCounter is simplistic (character-based estimation)
   - No streaming support for large context management

## Alternative Approaches Considered

### Option A: Full Middleware Adoption
Replace current functions with middleware library, requiring custom PolicyEvaluator.

**Pros**: Single pattern, all benefits of middleware
**Cons**: High migration effort, marginal benefit for simple whitelist

### Option B: Custom Policy Evaluator
Create a policy evaluator that delegates to current phase logic.

**Pros**: Reuses existing logic, maintains behavior
**Cons**: Added abstraction layer, still requires type conversion

### Option C: Status Quo (Selected)
Keep current functions, adopt middleware for new agents.

**Pros**: Low risk, maintains existing behavior, gradual adoption
**Cons**: Two patterns in codebase

## Implementation Plan

### Phase 1: Documentation & Discovery (Complete)
- Document all middleware capabilities in this ADR
- Identify gaps between current implementation and middleware features

### Phase 2: New Agent Adoption
- New agents (post-ADR-011) use middleware library pattern directly
- Start with PolicyEvaluator for rate limiting and max turns
- Use CallHistory for tool tracking

### Phase 3: Formatter Integration
- Integrate ToolFormatter for model-family specific tool calling
- Enable Qwen format for existing Qwen-based agents
- Add model family detection to auto-select formatter

### Phase 4: Context Management
- Evaluate ContextWindowManager for long-running agents
- Test compaction strategies with real workloads
- Consider SummaryCompaction for very long conversations

### Phase 5: Evaluation (Future)
- Assess middleware effectiveness after 3 months
- Decide on full migration or status quo in ADR-012

## Implementation Notes

### Current phased_executor.go
- Keep `filterToolCalls()` and `filterToolDefinitions()` unchanged
- Add code comments explaining dual-pattern approach

### New Agent Patterns
```go
// Example: Using middleware in new agent
func NewAgentWithMiddleware() *Agent {
    middleware := middleware.NewMiddleware(
        middleware.WithPolicyEvaluator(policy),
        middleware.WithCallHistory(),
        middleware.WithFormatter(formatter),
    )
    
    return &Agent{
        middleware: middleware,
        // ...
    }
}
```

### Context Window Integration
```go
// Example: Using window manager
wm, _ := windowmanager.NewContextWindowManager(
    windowmanager.ModelSpec{
        Name:           "qwen2.5-coder:32b",
        MaxTokens:      32768,
        ReservedTokens: 4096,
    },
    windowmanager.NewPriorityBasedCompaction(),
    nil, // uses DefaultCounter
)

status, _ := wm.Check(ctx, messages)
if shouldCompact, _ := wm.ShouldCompact(ctx, messages); shouldCompact {
    messages, _ = wm.Compact(ctx, messages)
}
```

## References

- Current functions: `pkg/agents/phased_executor.go:725-789`
- Call sites: `pkg/agents/phased_executor.go:618, 691`
- Middleware library: `github.com/soypete/pedro-agentware/middleware`
  - Core: `middleware/middleware.go`
  - Policy: `middleware/policy.go`
  - Format: `middleware/format/formatter.go`
  - Window: `middleware/windowmanager/windowmanager.go`
- Related ADR-008: Context Window Management Strategy
- ADR-003: Dynamic Tool Invocation
- ADR-007: MCP Removal Migration

## Future Work

1. **ADR-012**: Re-evaluate middleware adoption after new agents prove the pattern
2. **Migration Guide**: Document steps to convert existing agents to middleware
3. **Testing**: Add comprehensive tests for middleware components
4. **Monitoring**: Add metrics for policy evaluations, compaction events, tool failure rates