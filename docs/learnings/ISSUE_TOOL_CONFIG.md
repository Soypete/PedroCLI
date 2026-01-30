# Remove `enable_tools` from Config - Make Tool Calling Always Enabled

## Problem

The `enable_tools` config option creates a confusing failure mode:

1. **User creates config** with `enable_tools: false` (or it defaults to `null`)
2. **Agent starts** but LLM doesn't receive tool definitions
3. **Agent fails silently** - says "PHASE_COMPLETE" without doing any work
4. **No obvious error** - it just looks like the agent is lazy

**Example from user testing:**
```
Phase 1/5: analyze
   Round 1/10
   [DEBUG] LLM returned 0 tool calls
   [DEBUG] Phase completing with 0 tool calls made, 0 files modified
   ✅ Phase analyze completed in 1 rounds
```

The analyze phase completed without searching for files, reading code, or doing any analysis because `enable_tools` was not explicitly set to `true`.

## Why This Option Exists

Originally added to support:
1. **Legacy text-based tool calling** (before native tool support)
2. **Models without tool calling support** (base models, older models)
3. **Backends without tool support** (llama.cpp without --jinja)

However, in practice:
- **Pedro requires tools to function** - agents can't analyze/implement without them
- **All supported models have tool calling** (Qwen 2.5+, Llama 3.1+, Mistral)
- **All supported backends have tool calling** (llamacpp with --jinja, Ollama, OpenAI)

## Proposed Solution

### Option 1: Remove Config Option (Recommended)

**Always enable tool calling** and remove `enable_tools` from config:

```go
// pkg/config/config.go
type ModelConfig struct {
    Type        string  `json:"type"`
    ModelName   string  `json:"model_name"`
    // ... other fields ...
    // REMOVED: EnableTools bool `json:"enable_tools,omitempty"`
}
```

**Hardcode it in the LLM backend:**

```go
// pkg/llm/server.go
func (c *ServerClient) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
    // Always send tool definitions if registry is set
    if len(req.Tools) > 0 {
        // Include tools in request
    }
}
```

**Benefits:**
- Eliminates a common failure mode
- Simplifies config (one less option to understand)
- Makes tool calling the default (as it should be)

**Migration:**
- Existing configs with `enable_tools` will ignore the field (backward compatible)
- No breaking changes

### Option 2: Default to `true` Instead of `null`

Keep the option but change the default:

```go
type ModelConfig struct {
    EnableTools bool `json:"enable_tools,omitempty"` // Default: true
}

// In config loading:
if cfg.Model.EnableTools == nil {
    cfg.Model.EnableTools = true // Default to enabled
}
```

**Benefits:**
- Allows disabling tools if needed (edge cases)
- Less breaking

**Drawbacks:**
- Doesn't solve the problem (users can still disable it and break things)

### Option 3: Add Validation with Clear Error

Keep the option but validate on startup:

```go
func ValidateConfig(cfg *Config) error {
    if !cfg.Model.EnableTools {
        return fmt.Errorf("enable_tools must be true for agents to function - set 'enable_tools: true' in .pedrocli.json")
    }
    return nil
}
```

**Benefits:**
- Clear error message instead of silent failure
- Documents that tools are required

**Drawbacks:**
- Still allows misconfiguration (just with better error message)

## Recommendation

**Option 1** is best:
- Tools are **required** for Pedro to work
- No legitimate use case for disabling them in production
- Simplifies the codebase and user experience

If there's a future need to support non-tool models, it can be re-added with proper validation and documentation.

## Implementation Checklist

- [ ] Remove `EnableTools` field from `config.ModelConfig`
- [ ] Remove references in config loading/validation
- [ ] Update default `.pedrocli.json` example to remove the field
- [ ] Update documentation (remove mentions of `enable_tools`)
- [ ] Add migration note to CHANGELOG
- [ ] Test with existing configs (ensure backward compatibility)

## Related Issues

- #XXX - Context manager logging (discovered this issue during testing)
- #XXX - Validate phase hitting max rounds (caused by this issue)

## User Impact

**Before (Confusing):**
```json
{
  "model": {
    "model_name": "qwen2.5-coder:32b",
    "enable_tools": null  // ← Breaks everything silently
  }
}
```

Agent appears to work but does nothing.

**After (Clear):**
```json
{
  "model": {
    "model_name": "qwen2.5-coder:32b"
    // Tools always enabled - no config needed
  }
}
```

Agent works out of the box.

---

## Alternative: Keep for Advanced Use Cases

If we decide to keep `enable_tools`, at minimum we need:

1. **Default to `true`** instead of `null`
2. **Validate on startup** with clear error if disabled
3. **Document when to disable it** (only for non-agent use cases)
4. **Add warning** if agent tries to run with tools disabled

But this adds complexity without clear benefit.
