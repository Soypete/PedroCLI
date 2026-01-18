# LLM Tool Parameter Reliability Issue

**Date**: 2026-01-17
**Context**: Build agent testing on GitHub issue #32 (Add Prometheus observability)
**Model**: Qwen 2.5 Coder 32B (via llama.cpp)

## Problem Statement

The BuilderPhasedAgent consistently fails to provide the required `action` parameter when calling the `search` tool, despite clear examples and documentation in the system prompt.

### Observed Pattern

During the Analyze phase of the build agent (max 10 rounds), the search tool failed repeatedly:

```
Round 2:  ❌ Tool search failed: missing 'action' parameter
Round 3:  ✅ Tool search succeeded
Round 4:  ✅ Tool search succeeded
Round 5:  ❌ Tool search failed: missing 'action' parameter
Round 6:  ❌ Tool search failed: missing 'action' parameter
Round 7:  ❌ Tool search failed: missing 'action' parameter
Round 8:  ❌ Tool search failed: missing 'action' parameter
Round 9:  ❌ Tool search failed: missing 'action' parameter
Round 10: ❌ Tool search failed: missing 'action' parameter
Round 11: ❌ Tool search failed: missing 'action' parameter
...continuing through Round 19
```

**Success Rate**: ~15% (2 successes out of ~17 attempts in rounds 2-19)

### Tool Call Format

**Expected**:
```json
{
  "tool": "search",
  "args": {
    "action": "grep",
    "pattern": "deployment.yaml",
    "path": "."
  }
}
```

**Actual (failing)**:
```json
{
  "tool": "search",
  "args": {
    "pattern": "deployment|deploy|k8s",
    "path": "deployments/kubernetes"
  }
}
```

Notice the missing `"action"` field in args.

### Search Tool Definition

From `pkg/tools/search.go:65-71`:
```go
func (s *SearchTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
    action, ok := args["action"].(string)
    if !ok {
        return &Result{Success: false, Error: "missing 'action' parameter"}, nil
    }
    // ...
}
```

Valid actions: `grep`, `find_files`, `find_in_file`, `find_definition`

## Root Cause Analysis

### Why This Happens

1. **Probabilistic Nature**: LLMs generate tokens probabilistically - even with clear examples, there's no guarantee of specific output
2. **Attention Distribution**: In long system prompts (5000+ tokens for Analyze phase), the model may not consistently attend to parameter requirements
3. **Schema Complexity**: The search tool has 4 different actions, each with different parameter requirements
4. **No Structural Enforcement**: JSON schema validation happens **after** generation, not during

### Evidence from Job Files

File: `/tmp/pedroceli-jobs/job-1768708813-20260117-210013/034-response.txt`
```
Let's proceed by finding the relevant Kubernetes deployment file. Since the search for `deployment.yaml`
did not yield results, we need to search more broadly for Kubernetes deployment files.

We'll search for files with patterns like `deployment`, `deploy`, or `k8s` in the `deployments/kubernetes`
directory to identify the correct deployment file.

{"tool": "search", "args": {"pattern": "deployment|deploy|k8s", "path": "deployments/kubernetes"}}
```

The LLM's reasoning is sound, but it forgot to include `"action": "grep"` in the args.

## Solution Options Considered

### 1. ❌ Improve Prompt Examples
**Approach**: Add more explicit examples of search tool calls with action parameter
**Why Rejected**: Already have clear examples; adding more increases prompt length without guarantees

### 2. ✅ **Logit Bias (SELECTED)**
**Approach**: Boost probability of `"action"` token appearing after `"args": {` in tool call JSON
**Why Selected**:
- Directly addresses the probabilistic generation issue
- No prompt changes needed
- Works at token level during generation
- Low overhead (single bias entry)

**Implementation Plan**:
```go
// In pkg/llm/server.go or llamacpp.go
type ChatCompletionRequest struct {
    // ... existing fields
    LogitBias map[string]float64 `json:"logit_bias,omitempty"`
}

// When generating tool call responses:
req.LogitBias = map[string]float64{
    "action": 5.0,  // Boost probability when in args context
}
```

**Acceptance Criteria**:
- [ ] Search tool success rate improves to >80% in Analyze phase
- [ ] No regression in other tool call reliability
- [ ] Logit bias only applied during tool call generation (not regular text)
- [ ] Configurable bias strength (default: 5.0)

### 3. ❌ Schema-Guided Generation
**Approach**: Use constrained decoding (grammar-based generation) to enforce JSON schema
**Why Rejected**:
- Not supported by llama.cpp chat completions API
- Would require switching to raw completion mode
- Loses chat template benefits

### 4. ❌ Tool Call Retry Logic
**Approach**: Automatically retry failed tool calls with parameter hints
**Why Rejected**:
- Wastes inference rounds (already limited to 10-30 per phase)
- Doesn't fix root cause
- Creates confusing feedback loops

### 5. ❌ Simplify Tool Interface
**Approach**: Split search tool into 4 separate tools (grep, find_files, find_in_file, find_definition)
**Why Rejected**:
- Increases system prompt size (4x tool descriptions)
- Doesn't guarantee parameter compliance on other tools
- Makes tool registry more complex

## Implementation Details

### Phase 1: Add Logit Bias Support (PR #1)

**Files to Modify**:

1. **`pkg/llm/interface.go`** - Add LogitBias field to request interface:
```go
type ChatCompletionRequest struct {
    Model       string                 `json:"model"`
    Messages    []ChatMessage          `json:"messages"`
    Temperature float64                `json:"temperature,omitempty"`
    MaxTokens   int                    `json:"max_tokens,omitempty"`
    LogitBias   map[string]float64     `json:"logit_bias,omitempty"`  // NEW
    // ... other fields
}
```

2. **`pkg/llm/server.go`** - Forward logit bias to backend:
```go
func (b *ServerBackend) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
    payload := map[string]interface{}{
        "model":       b.ModelName,
        "messages":    req.Messages,
        "temperature": req.Temperature,
        "max_tokens":  req.MaxTokens,
    }

    if req.LogitBias != nil && len(req.LogitBias) > 0 {
        payload["logit_bias"] = req.LogitBias  // NEW
    }
    // ... rest of implementation
}
```

3. **`pkg/agents/executor.go`** - Apply logit bias during tool call generation:
```go
func (e *InferenceExecutor) Execute(ctx context.Context, maxRounds int) error {
    for round := 1; round <= maxRounds; round++ {
        // Build request with logit bias for tool calls
        req := llm.ChatCompletionRequest{
            Messages:    messages,
            Temperature: e.temperature,
            MaxTokens:   e.maxTokens,
            LogitBias:   map[string]float64{
                "action": 5.0,     // Boost "action" token in args
                "args":   2.0,     // Also boost "args" presence
            },
        }

        resp, err := e.backend.ChatCompletion(ctx, req)
        // ... rest of execution
    }
}
```

4. **`pkg/config/config.go`** - Add configuration options:
```go
type ModelConfig struct {
    // ... existing fields
    LogitBias map[string]float64 `json:"logit_bias,omitempty"`
}
```

Example `.pedrocli.json`:
```json
{
  "model": {
    "type": "llamacpp",
    "model_name": "qwen2.5-coder-32b",
    "logit_bias": {
      "action": 5.0,
      "args": 2.0
    }
  }
}
```

### Phase 2: Testing and Validation

**Test Cases**:

1. **Baseline Test** (without logit bias):
   - Run BuilderPhasedAgent on simple feature request
   - Track search tool success rate over 20 rounds
   - Expected: ~15% success rate

2. **Logit Bias Test** (with bias):
   - Same feature request, logit_bias enabled
   - Track search tool success rate over 20 rounds
   - Expected: >80% success rate

3. **Regression Test**:
   - Test other tools (code_edit, file, navigate, git)
   - Ensure no degradation in their parameter compliance
   - Expected: No change in success rates

4. **Different Bias Strengths**:
   - Test bias values: 1.0, 2.0, 5.0, 10.0
   - Find optimal balance (success rate vs. text quality)
   - Document recommended value

**Metrics to Track**:
```
Tool Call Success Metrics:
- search tool success rate (% with action parameter)
- code_edit tool success rate (% with correct action/params)
- Average inference rounds per phase (should not increase)
- Total token usage per phase (should not increase significantly)
```

### Phase 3: Learnings from Testing

**When Testing Completes** (to be added):
- Actual success rate improvement (target: >80%)
- Optimal logit bias values
- Any unexpected side effects
- Model-specific differences (Qwen vs Llama vs Mistral)

## Related Issues

- **GitHub Issue #32**: Build agent test case that exposed this issue
- **BuilderPhasedAgent Analyze Phase**: Where failures were most frequent (10 max rounds, all search)
- **Plan File**: `/Users/miriahpeterson/.claude/plans/playful-cuddling-feather.md` - PR #1 includes this fix

## Next Steps

1. ✅ **Document the problem** (this file)
2. [ ] **Implement logit bias support** in PR #1 (unified architecture foundation)
3. [ ] **Test with different models** (Qwen 32B, Llama 3.x, Mistral)
4. [ ] **Validate success rate** >80% on build agent
5. [ ] **Document optimal bias values** per model
6. [ ] **Update this learning** with test results

## References

- **llama.cpp Logit Bias**: https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md#api-endpoints
- **OpenAI Logit Bias**: https://platform.openai.com/docs/api-reference/chat/create#chat-create-logit_bias
- **Tool Definition**: `pkg/tools/search.go`
- **Executor**: `pkg/agents/executor.go`
- **BuilderPhasedAgent**: `pkg/agents/builder_phased.go`
