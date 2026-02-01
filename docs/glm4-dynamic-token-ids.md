# GLM-4 Support with Dynamic Token ID System

## Overview

This document describes the GLM-4 model family support and the new dynamic token ID system for logit bias.

## Problem

Previously, logit bias used hardcoded Llama 3.x token IDs. When using GLM-4 or other models with different tokenizers, these hardcoded IDs were incorrect, leading to:
- Poor tool calling performance
- Incorrect biasing behavior
- Model-specific coupling

## Solution

### 1. GLM-4 Model Family Support

Added GLM-4 as a recognized model family with its own tool calling formatter.

**Files:**
- `pkg/toolformat/glm4.go` - GLM-4 formatter implementation
- `pkg/toolformat/formatter.go` - Updated to detect GLM-4 models

**Supported model names:**
- `glm-4*` (e.g., `glm-4-9b-chat`, `glm-4-flash`)
- `glm4*` (without dash)
- `chatglm*` (e.g., `chatglm3-6b`)

**Tool format:**
- OpenAI-compatible API format
- Native tool calling support
- Handles `reasoning_content` field (CoT-style reasoning)

### 2. Dynamic Token ID System

Replaced hardcoded token IDs with dynamic lookup from the model's actual tokenizer.

**Architecture:**

```
┌─────────────────────────────────────────┐
│         TokenIDProvider Interface       │
│  - GetTokenIDs(phrases) -> token IDs    │
│  - GetSingleTokenID(phrase) -> int      │
└─────────────────────────────────────────┘
                    ▲
                    │
        ┌───────────┴───────────┐
        │                       │
┌───────────────────┐   ┌──────────────────┐
│ HTTPTokenIDProvider│   │StaticTokenIDProvider│
│ (uses /tokenize)   │   │ (hardcoded maps) │
└───────────────────┘   └──────────────────┘
```

**HTTPTokenIDProvider:**
- Fetches token IDs from LLM backend's `/tokenize` endpoint
- Caches results for performance (one HTTP call per phrase)
- Thread-safe with `sync.RWMutex`
- Gracefully degrades to empty bias if tokenization fails

**StaticTokenIDProvider:**
- Fallback for models without `/tokenize` support
- Contains hardcoded token maps for Llama 3.x, Qwen, GLM-4
- Used for testing

**NullTokenIDProvider:**
- Returns empty bias (no token IDs)
- Used when logit bias is not needed

### 3. Refactored Logit Bias Functions

**Before:**
```go
func GetAntiHallucinationBias() map[int]float32 {
    bias := make(map[int]float32)
    bias[13249] = -50.0  // ``` - HARDCODED Llama 3.x token!
    bias[2285] = -30.0   // json - WRONG for GLM-4!
    return bias
}
```

**After:**
```go
func GetAntiHallucinationBias(provider TokenIDProvider) map[int]float32 {
    patterns := []BiasPattern{
        {"```", -50.0},   // Dynamic lookup
        {"json", -30.0},  // Works with any model
    }

    phrases := extractPhrases(patterns)
    tokenIDs, _ := provider.GetTokenIDs(phrases)

    bias := make(map[int]float32)
    for _, pattern := range patterns {
        for _, tokenID := range tokenIDs[pattern.Phrase] {
            bias[tokenID] = pattern.Bias
        }
    }
    return bias
}
```

**Key changes:**
- `GetAntiHallucinationBias(provider TokenIDProvider)` - now requires provider
- `GetToolResultValidationBias(provider TokenIDProvider)` - now requires provider
- Bias values (floats) remain the same - only token IDs change

### 4. Integration with Agents

**BaseAgent:**
- Added `tokenIDProvider` field
- Automatically initialized with `HTTPTokenIDProvider` in `NewBaseAgent()`
- Can be overridden with `SetTokenIDProvider()`

**Phased Executor:**
- Updated `executeInference()` to pass provider to bias functions
- Applies bias in "validate" phase when processing tool results

**Example:**
```go
// Create agent (provider is auto-initialized)
agent := NewBuilderAgent(cfg, backend, jobMgr)

// Use custom provider (optional)
agent.SetTokenIDProvider(NewStaticTokenIDProvider("qwen"))

// Bias is applied automatically in phased execution
bias := GetToolResultValidationBias(agent.tokenIDProvider)
req.LogitBias = bias
```

## Performance

**Tokenization overhead:**
- ~10-50ms per HTTP call to `/tokenize`
- Cached per phrase (only 1 call per unique phrase)
- ~20-30 unique phrases in bias patterns
- **Total startup overhead: <1 second**

**Cache behavior:**
- Thread-safe with `sync.RWMutex`
- Persists for agent lifetime
- No disk persistence (intentional - keeps code simple)

## Testing

**New test files:**
- `pkg/toolformat/glm4_test.go` - GLM-4 formatter tests
- `pkg/agents/logit_bias_test.go` - Dynamic bias tests

**Test coverage:**
- GLM-4 model detection
- Tool formatting and parsing
- Dynamic token ID lookup
- Bias pattern application
- Static provider fallback
- Null provider behavior

**Run tests:**
```bash
# GLM-4 formatter tests
go test github.com/soypete/pedrocli/pkg/toolformat -run "TestGLM4"

# Logit bias tests
go test github.com/soypete/pedrocli/pkg/agents -run "TestGetAntiHallucinationBias|TestGetToolResultValidationBias"

# All tests
make test-quick
```

## Migration Guide

### For Existing Code

**No changes required** if using standard agent constructors:
```go
// This automatically gets HTTPTokenIDProvider
agent := NewBuilderAgent(cfg, backend, jobMgr)
```

**If calling bias functions directly:**
```go
// OLD (doesn't compile anymore)
bias := GetAntiHallucinationBias()

// NEW
provider := NewHTTPTokenIDProvider(backend)
bias := GetAntiHallucinationBias(provider)
```

### Adding New Bias Patterns

**To add new bias patterns:**
1. Add to `BiasPattern` list in `GetAntiHallucinationBias()`
2. Token IDs are fetched automatically

**Example:**
```go
patterns := []BiasPattern{
    {"new_phrase", -25.0},  // Add your pattern here
    {"another_phrase", 15.0},
}
```

### Adding New Model Families

**To add support for new models:**
1. Create formatter: `pkg/toolformat/newmodel.go`
2. Add to `ModelFamily` enum in `formatter.go`
3. Update `DetectModelFamily()` to recognize model names
4. Update `GetFormatter()` to return new formatter
5. Add token IDs to `NewStaticTokenIDProvider()` (optional)

## Verification

### Test GLM-4 Detection
```go
family := DetectModelFamily("glm-4-9b-chat")
// Returns: ModelFamilyGLM4

formatter := GetFormatter(family)
// Returns: GLM4Formatter
```

### Test Dynamic Token IDs
```bash
# 1. Start GLM-4 llama-server
llama-server \
  --hf-repo unsloth/GLM-4.7-Flash-GGUF \
  --hf-file GLM-4.7-Flash-Q4_K_M.gguf \
  --port 8082 \
  --ctx-size 16384 \
  --n-gpu-layers -1 \
  --jinja

# 2. Test tokenize endpoint
curl -X POST http://localhost:8082/tokenize \
  -H "Content-Type: application/json" \
  -d '{"content":"```"}' | jq

# 3. Run pedrocli with GLM-4
./pedrocli build -description "Add a simple test function"

# 4. Verify correct token IDs in job files
cat /tmp/pedrocli-jobs/job-*/002-prompt.txt
```

## Benefits

### 1. Model-Agnostic
- Works with any model that has a `/tokenize` endpoint
- No hardcoded token IDs
- Easy to add new models

### 2. Safer
- Empty bias is safer than wrong bias
- Graceful degradation on error
- No risk of incorrect biasing

### 3. Maintainable
- Centralized in `TokenIDProvider` interface
- Easy to test with mock provider
- Clear separation of concerns

### 4. Backward Compatible
- Existing agents work without changes
- Static provider available for models without `/tokenize`
- Llama 3.x token map preserved for reference

## Future Enhancements

1. **Vocabulary File Distribution**
   - Ship vocab files with pedrocli
   - Faster than HTTP (no network calls)

2. **Model Auto-Detection**
   - Auto-detect model family from endpoint
   - Reduce manual configuration

3. **Token ID Profiling Tool**
   - CLI command to test token IDs for a model
   - Help debug bias issues

4. **Performance Monitoring**
   - Track cache hit rates
   - Monitor tokenization latency

5. **Multi-Model Support**
   - Support multiple models in single session
   - Per-model token ID caches

## References

- **Issue**: GLM-4 was using hardcoded Llama 3.x token IDs
- **Root cause**: Token IDs like `13249` for "\`\`\`" are model-specific
- **Solution**: Dynamic lookup via `/tokenize` endpoint
- **Files changed**:
  - `pkg/toolformat/glm4.go` (new)
  - `pkg/toolformat/formatter.go` (updated)
  - `pkg/agents/token_ids.go` (new)
  - `pkg/agents/logit_bias.go` (refactored)
  - `pkg/agents/base.go` (added tokenIDProvider)
  - `pkg/agents/phased_executor.go` (pass provider to bias functions)
