# Reasoning Models Guide

## Overview

Modern LLMs like GLM-4, DeepSeek R1, and Qwen-QwQ have built-in reasoning capabilities that use a dual-token system:
- **Reasoning tokens**: Internal thinking/planning (chain-of-thought)
- **Output tokens**: Final response with tool calls or content

Without proper configuration, reasoning models can exhaust their token budget on thinking before producing useful output.

## The Problem

**Example from production:**
```
Research Phase with GLM-4.7-Flash:
- MaxTokens: 8192
- Reasoning consumed: 8192 tokens (planning which tools to call)
- Output produced: 0 tokens (truncated before making tool calls)
- Result: No tools executed, workflow fails
```

## Solution: Control Reasoning Budget

llama-server (and compatible backends) support reasoning control via CLI flags.

### llama-server Flags

```bash
--reasoning-format FORMAT
```
Controls how reasoning is handled:
- `deepseek`: Puts thoughts in `message.reasoning_content` (separate from output)
- `deepseek-legacy`: Keeps `<think>` tags in content AND populates reasoning_content
- Default: No special reasoning handling

```bash
--reasoning-budget N
```
Limits thinking tokens:
- `-1`: Unlimited reasoning (default) ⚠️ Can exhaust token budget
- `0`: Disable reasoning completely
- `N`: Limit to N tokens of reasoning

### Recommended Configuration

#### For Tool-Heavy Workflows (Research, Code Search)

**Priority**: Tool execution over deep reasoning

```bash
llama-server \
  --model ~/.cache/huggingface/.../GLM-4.7-Flash-Q4_K_M.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers -1 \
  --reasoning-format deepseek \
  --reasoning-budget 4096 \        # 4K thinking, rest for output
  --jinja \
  --no-webui \
  --metrics
```

**Token allocation:**
- Prompt: ~8K tokens
- Reasoning: 4K tokens (limited)
- Output: ~18K tokens (plenty for tool calls)
- Total: 30K tokens (fits in 32K context)

#### For Planning/Architecture Workflows

**Priority**: Deep reasoning

```bash
llama-server \
  --model ~/.cache/huggingface/.../GLM-4.7-Flash-Q4_K_M.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers -1 \
  --reasoning-format deepseek \
  --reasoning-budget 12288 \       # 12K thinking for complex analysis
  --jinja \
  --no-webui \
  --metrics
```

**Token allocation:**
- Prompt: ~8K tokens
- Reasoning: 12K tokens (extensive thinking)
- Output: ~10K tokens (structured plan)
- Total: 30K tokens

#### For Content Generation

**Priority**: Balanced

```bash
llama-server \
  --reasoning-budget 6144          # 6K thinking, balanced
```

## Parsing Reasoning Content

Our code already handles `reasoning_content` (see `pkg/llm/server.go:190`):

```go
type Message struct {
    Content          string `json:"content"`
    ReasoningContent string `json:"reasoning_content,omitempty"` // GLM-4, DeepSeek
}

// Parse response
if content == "" && message.ReasoningContent != "" {
    content = message.ReasoningContent  // Fallback to reasoning
}
```

### Best Practice: Separate Reasoning from Output

With `--reasoning-format deepseek`, the API returns:
- `message.reasoning_content`: The thinking process
- `message.content`: The actual tool calls/response

**Benefits:**
1. Can log reasoning separately for debugging
2. Don't include thinking in context (saves tokens)
3. Can analyze reasoning quality independently
4. Better metrics (track thinking vs output tokens)

## Configuration Integration

Add to `.pedrocli.json` (future enhancement):

```json
{
  "model": {
    "type": "llamacpp",
    "model_name": "GLM-4.7-Flash",
    "server_url": "http://localhost:8082",
    "context_size": 32768,
    "reasoning": {
      "enabled": true,
      "format": "deepseek",
      "budget": 4096,
      "budget_by_phase": {
        "research": 4096,
        "planning": 12288,
        "content": 6144
      }
    }
  }
}
```

## Model-Specific Recommendations

### GLM-4.7-Flash
- **Default budget**: 4096 tokens
- **Format**: `deepseek`
- **Context**: 32K
- **Best for**: Balanced reasoning + tool execution

### DeepSeek R1 (Future)
- **Default budget**: 8192 tokens
- **Format**: `deepseek`
- **Context**: 64K+
- **Best for**: Complex reasoning tasks

### Qwen-QwQ (Future)
- **Default budget**: 6144 tokens
- **Format**: `deepseek` (if compatible)
- **Context**: 32K
- **Best for**: Mathematical/logical reasoning

## Testing Reasoning Budget

### Before (Unlimited Reasoning)
```bash
# Start llama-server without reasoning control
llama-server --model model.gguf --ctx-size 32768 --port 8082

# Result: 8K tokens reasoning, 0 tokens output (truncated)
```

### After (Limited Reasoning)
```bash
# Start llama-server WITH reasoning control
llama-server \
  --model model.gguf \
  --ctx-size 32768 \
  --port 8082 \
  --reasoning-format deepseek \
  --reasoning-budget 4096

# Result: 4K tokens reasoning, 16K tokens output (successful tool calls)
```

### Verification

Check llama-server metrics at `http://localhost:8082/metrics`:

```bash
curl -s http://localhost:8082/metrics | grep -E "prompt_tokens|predicted_tokens"
```

Look for token distribution:
- Prompt tokens: Context input
- Predicted tokens: Reasoning + output combined
- With budget: Reasoning capped, more output tokens

## Troubleshooting

### Problem: Still Getting Truncated Responses

**Cause**: Reasoning budget too high for available context

**Fix**: Calculate safe budget
```
Context: 32768
Prompt: ~10000
Safety: ~2000
Available: 20768

Max reasoning budget: 4096-8192 (leave room for output)
```

### Problem: Tool Calls Not Being Made

**Cause**: All tokens spent on reasoning

**Fix**: Lower reasoning budget to 2048-4096 for tool execution phases

### Problem: Reasoning Quality Degraded

**Cause**: Reasoning budget too restrictive

**Fix**: Increase budget to 8192-12288 for planning phases

## Future: Dynamic Reasoning Budget

Automatically adjust reasoning budget based on phase:

```go
func (a *BaseAgent) getReasoningBudget(phaseName string) int {
    budgets := map[string]int{
        "research":    4096,   // Quick thinking, fast tool calls
        "planning":    12288,  // Deep analysis
        "debugging":   8192,   // Moderate analysis
        "content":     6144,   // Balanced
    }

    if budget, ok := budgets[phaseName]; ok {
        return budget
    }
    return 4096  // Safe default
}
```

## References

- [GLM-4.7 Agentic Coding Guide](https://docs.z.ai/guides/llm/glm-4.7#agentic-coding)
- [llama.cpp Reasoning Support](https://github.com/ggerganov/llama.cpp)
- [DeepSeek R1 Documentation](https://github.com/deepseek-ai/DeepSeek-R1)
- Issue #90: Phase-Aware Token Budget System

## Quick Start

**Stop current llama-server:**
```bash
pkill llama-server
```

**Restart with reasoning control:**
```bash
llama-server \
  --hf-repo unsloth/GLM-4.7-Flash-GGUF \
  --hf-file GLM-4.7-Flash-Q4_K_M.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers -1 \
  --reasoning-format deepseek \
  --reasoning-budget 4096 \
  --jinja \
  --no-webui \
  --metrics
```

**Update config:**
```json
{
  "model": {
    "context_size": 32768,
    "model_name": "GLM-4.7-Flash"
  }
}
```

**Test:**
```bash
./pedrocli blog -file test.txt
# Should see much faster tool calls with less reasoning overhead
```

---

**Last Updated**: 2026-02-01
**Next Review**: After testing with DeepSeek R1 and other reasoning models
