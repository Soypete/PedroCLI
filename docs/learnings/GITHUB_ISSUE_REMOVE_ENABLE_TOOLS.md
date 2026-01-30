# Remove `enable_tools` Config Option - Make Tool Calling a Prerequisite

## Problem

Agents fail silently when `enable_tools` is not explicitly set to `true`:

```bash
Phase 1/5: analyze
   [DEBUG] LLM returned 0 tool calls
   [DEBUG] Phase completing with 0 tool calls made, 0 files modified
   ✅ Phase analyze completed in 1 rounds
```

The LLM doesn't receive tool definitions, so it just responds with text and says "PHASE_COMPLETE" without actually analyzing, planning, or implementing anything.

## Root Cause

`enable_tools` in `.pedrocli.json` defaults to `null`/`false`:

```json
{
  "model": {
    "model_name": "qwen2.5-coder:32b",
    "enable_tools": null  // ← This breaks everything
  }
}
```

## Solution

**Remove the config option entirely.** Tool calling is a prerequisite for Pedro to function.

### Code Changes

1. **Remove from config struct:**
```go
// pkg/config/config.go
type ModelConfig struct {
    Type        string  `json:"type"`
    ModelName   string  `json:"model_name"`
    // ... other fields ...
    // REMOVE: EnableTools bool `json:"enable_tools,omitempty"`
}
```

2. **Always enable in LLM backend:**
```go
// pkg/llm/server.go, pkg/llm/llamacpp.go, pkg/llm/ollama.go
// Always include tool definitions in requests (when tools are registered)
if len(req.Tools) > 0 {
    // Include in API request
}
```

3. **Update example configs:**
```bash
# Remove enable_tools from all example .pedrocli.json files
find . -name ".pedrocli*.json" -exec sed -i '' '/enable_tools/d' {} \;
```

### Validation (Optional)

If a model/backend truly doesn't support tools, detect and fail early:

```go
func ValidateBackend(backend llm.Backend) error {
    // Try a simple tool call
    // If it fails, return clear error about tool support requirement
}
```

## Benefits

- **Eliminates confusing failure mode** (agents doing nothing)
- **Simplifies config** (one less field to understand)
- **Documents the requirement** (tools aren't optional)

## Migration

Existing configs with `enable_tools` will continue to work (field is ignored). No breaking changes.

## Implementation

- [ ] Remove `EnableTools` from `pkg/config/config.go`
- [ ] Remove references in backend implementations
- [ ] Update example configs to remove the field
- [ ] Update documentation
- [ ] Add migration note to CHANGELOG

## Testing

After the fix, agents should work without needing `enable_tools` in config:

```bash
# Before: Required this
{
  "model": {
    "model_name": "qwen2.5-coder:32b",
    "enable_tools": true  // ← Had to remember this
  }
}

# After: Just works
{
  "model": {
    "model_name": "qwen2.5-coder:32b"
  }
}
```
