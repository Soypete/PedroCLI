# PedroCLI Configuration Guide

Complete guide to configuring PedroCLI for optimal performance and reliability.

## Table of Contents

- [Config File Locations](#config-file-locations)
- [Configuration File Precedence](#configuration-file-precedence)
- [Essential Settings](#essential-settings)
- [Model Backend Configuration](#model-backend-configuration)
- [Context Size Guidelines](#context-size-guidelines)
- [Native Tool Calling](#native-tool-calling)
- [Complete Example Configs](#complete-example-configs)
- [Troubleshooting](#troubleshooting)

---

## Config File Locations

PedroCLI looks for configuration files in two locations:

1. **Current directory**: `./.pedrocli.json` (checked first)
2. **Home directory**: `~/.pedrocli.json` (fallback)

### Which Config File Is Being Used?

To verify which config file is loaded:

```bash
# Check current directory
ls -la .pedrocli.json

# Check home directory
ls -la ~/.pedrocli.json
```

**Important**: If you run `pedrocli` from a directory without a local config, it will use the home directory config as fallback.

---

## Configuration File Precedence

**Rule**: Current directory `.pedrocli.json` **takes precedence** over `~/.pedrocli.json`

### Example Scenario

```bash
# You have two config files:
# 1. ~/project-a/.pedrocli.json  (context_size: 16384)
# 2. ~/.pedrocli.json            (context_size: 32768)

# Running from project-a
cd ~/project-a
./pedrocli blog -file post.txt
# ✅ Uses: ~/project-a/.pedrocli.json (16K context)

# Running from project-b (no local config)
cd ~/project-b
./pedrocli blog -file post.txt
# ✅ Uses: ~/.pedrocli.json (32K context)
```

### Best Practice

- **Global defaults**: Put common settings in `~/.pedrocli.json`
- **Project overrides**: Create `./.pedrocli.json` in projects that need different settings

---

## Essential Settings

### Minimal Working Config

```json
{
  "model": {
    "type": "llamacpp",
    "model_name": "Qwen3-Coder-30B-A3B-Instruct",
    "server_url": "http://localhost:8082",
    "context_size": 16384,
    "usable_context": 12288,
    "temperature": 0.2,
    "enable_tools": false
  },
  "project": {
    "name": "MyProject",
    "workdir": "/path/to/project"
  }
}
```

---

## Model Backend Configuration

### llama.cpp (llama-server)

**Recommended configuration**:

```json
{
  "model": {
    "type": "llamacpp",
    "model_name": "Qwen3-Coder-30B-A3B-Instruct",
    "server_url": "http://localhost:8082",
    "context_size": 16384,
    "usable_context": 12288,
    "temperature": 0.2,
    "threads": 8,
    "n_gpu_layers": -1,
    "enable_grammar": false,
    "enable_tools": false
  }
}
```

**Key settings**:
- `enable_tools: false` - Use custom formatters (avoids grammar crashes)
- `context_size` - Match your llama-server `--ctx-size`
- `usable_context` - Set to 75% of `context_size`

### Ollama

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "server_url": "http://localhost:11434",
    "temperature": 0.2
  }
}
```

**Note**: Ollama auto-detects context size, so `context_size` and `usable_context` are optional.

---

## Context Size Guidelines

### The 75% Rule

**Always set `usable_context` to 75% of `context_size`** to leave room for:
- System prompts (~1-2K tokens)
- Tool definitions (~1-2K tokens)
- Response generation (2-10K tokens)

### Hardware-Specific Recommendations

| Hardware | Model Size | Recommended Context | `context_size` | `usable_context` |
|----------|-----------|---------------------|----------------|------------------|
| M1 Max (32GB) | 30B-32B | 16K | `16384` | `12288` |
| M1 Max (32GB) | 7B-14B | 32K | `32768` | `24576` |
| M2/M3 Max (64GB+) | 30B-32B | 32K | `32768` | `24576` |
| Desktop (64GB+ RAM) | 70B+ | 16K-32K | `16384` - `32768` | 75% of context |

### Symptoms of Context Too Large

If you see these errors, **reduce `context_size` by half**:

```
Error: Post "http://localhost:8082/v1/chat/completions": EOF
```

**Cause**: llama-server running out of VRAM/RAM

**Solution**:
```json
{
  "model": {
    "context_size": 16384,  // Was 32768
    "usable_context": 12288  // Was 24576
  }
}
```

### Symptoms of Context Too Small

If phases hit max rounds without completing:

```
Phase research failed: max rounds (20) exceeded
```

**Possible cause**: Not enough context to hold conversation history

**Solution**: Increase context if your hardware supports it, or rely on context compaction (automatic at 75% threshold).

---

## Native Tool Calling

### TL;DR: Use `"enable_tools": false`

**Recommended setting for llama.cpp**:

```json
{
  "model": {
    "enable_tools": false  // Use custom formatters
  }
}
```

### Why Disable Native Tool Calling?

**Problem**: llama.cpp's grammar system has compatibility issues with certain models:

```
libc++abi: terminating due to uncaught exception of type std::runtime_error:
Unexpected empty grammar stack after accepting piece: =n (21747)
```

**Affected models**:
- Qwen3-Coder-30B-A3B-Instruct
- Qwen2.5-Coder-32B (sometimes)
- Other models may also be affected

**Solution**: Custom tool formatters (QwenFormatter, LlamaFormatter, etc.) parse tool calls from the LLM's text output instead of using grammar constraints.

### When to Enable Native Tool Calling (`"enable_tools": true`)

**Use native tool calling (`true`) with:**

1. **Ollama** - Generally has better tool calling support than llama.cpp
   ```json
   {
     "model": {
       "type": "ollama",
       "enable_tools": true  // Ollama handles tools well
     }
   }
   ```

2. **OpenAI API** - Native tool calling is the standard
   ```json
   {
     "model": {
       "type": "openai",
       "enable_tools": true  // Required for OpenAI
     }
   }
   ```

3. **Other API providers** (Anthropic, Together, etc.) - Use their native tool calling
   ```json
   {
     "model": {
       "type": "server",  // Generic OpenAI-compatible
       "enable_tools": true
     }
   }
   ```

4. **llama.cpp (llama-server)** - Only if:
   - Using a model that's confirmed to work (test first!)
   - Using a newer llama.cpp version with grammar fixes
   - Willing to debug potential grammar crashes

**Summary table:**

| Backend | Recommended `enable_tools` | Notes |
|---------|---------------------------|-------|
| `ollama` | `true` | Better tool calling support |
| `llamacpp` | `false` | Grammar bugs with many models |
| `openai` | `true` | Native API format |
| `server` (generic) | `true` | For OpenAI-compatible APIs |

### How to Test

```bash
# Try with native tool calling disabled
# Edit config: "enable_tools": false
./pedrocli blog -file test.txt -title "Test"

# Should see in debug output:
# [DEBUG] Tool calling enabled: false
```

---

## Complete Example Configs

### Ollama Config - Native Tool Calling

Ollama generally has better tool calling support than llama.cpp:

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b",
    "server_url": "http://localhost:11434",
    "temperature": 0.2,
    "enable_tools": true  // Ollama handles tools well
  },
  "project": {
    "name": "MyProject",
    "workdir": "/Users/yourname/projects"
  },
  "limits": {
    "max_inference_runs": 25
  },
  "debug": {
    "enabled": true
  }
}
```

### Home Config (`~/.pedrocli.json`) - Global Defaults (llama.cpp)

```json
{
  "model": {
    "type": "llamacpp",
    "model_name": "Qwen3-Coder-30B-A3B-Instruct",
    "server_url": "http://localhost:8082",
    "context_size": 16384,
    "usable_context": 12288,
    "temperature": 0.2,
    "threads": 8,
    "n_gpu_layers": -1,
    "enable_grammar": false,
    "enable_tools": false
  },
  "project": {
    "name": "DefaultProject",
    "workdir": "/Users/yourname/projects"
  },
  "tools": {
    "allowed_bash_commands": [
      "go", "git", "cat", "ls", "head", "tail",
      "wc", "sort", "uniq", "make", "gh", "npm",
      "curl", "grep"
    ],
    "forbidden_commands": [
      "rm", "mv", "dd", "sudo"
    ]
  },
  "limits": {
    "max_task_duration_minutes": 60,
    "max_inference_runs": 25
  },
  "debug": {
    "enabled": true,
    "keep_temp_files": true,
    "log_level": "info"
  },
  "blog": {
    "enabled": true,
    "rss_feed_url": "https://yourblog.com/feed",
    "research": {
      "enabled": true,
      "calendar_enabled": true,
      "rss_enabled": true,
      "max_rss_posts": 5,
      "max_calendar_days": 30
    }
  }
}
```

### Project-Specific Config - Large Context

For projects where you need more context (smaller models or more RAM):

```json
{
  "model": {
    "context_size": 32768,
    "usable_context": 24576
  },
  "project": {
    "name": "LargeProject",
    "workdir": "/Users/yourname/large-project"
  }
}
```

### Project-Specific Config - Resource Constrained

For projects on machines with limited resources:

```json
{
  "model": {
    "context_size": 8192,
    "usable_context": 6144,
    "n_gpu_layers": 20
  },
  "project": {
    "name": "SmallProject",
    "workdir": "/Users/yourname/small-project"
  },
  "limits": {
    "max_inference_runs": 15
  }
}
```

---

## Troubleshooting

### Config Not Loading

**Symptom**: Changes to config file don't take effect

**Check which config is being used**:
```bash
# From your project directory
pwd
ls -la .pedrocli.json  # Local config (highest priority)
ls -la ~/.pedrocli.json  # Home config (fallback)
```

**Solution**: Either:
1. Create `.pedrocli.json` in your current directory
2. Verify you're editing the correct config file

### Grammar Crashes (EOF errors)

**Symptom**:
```
Error: Post "http://localhost:8082/v1/chat/completions": EOF
libc++abi: terminating due to uncaught exception...
Unexpected empty grammar stack after accepting piece: =n
```

**Solution 1 - Disable native tool calling**:
```json
{
  "model": {
    "enable_tools": false
  }
}
```

**Solution 2 - Reduce context size**:
```json
{
  "model": {
    "context_size": 16384,  // Was 32768
    "usable_context": 12288  // Was 24576
  }
}
```

### Slow Inference

**Symptom**: Each token takes >5 seconds to generate

**Check**:
1. **Context too large**: Reduce `context_size`
2. **Not using GPU**: Set `n_gpu_layers: -1` (use all GPU)
3. **Model too large**: Try smaller quantization (Q4_K_M instead of Q8_0)

### Tool Calls Not Working

**Symptom**: Agent doesn't call tools, just generates text

**Check debug output**:
```bash
./pedrocli blog -file test.txt -title "Test" 2>&1 | grep "Tool calling enabled"
```

**Expected**: `[DEBUG] Tool calling enabled: false` (using custom formatters)

**If tools still not working**, check:
1. llama-server is running: `curl http://localhost:8082/health`
2. Model supports tool calling (instruction-tuned models work best)
3. Debug logs show tool definitions being sent

---

## Quick Reference

### Safe Defaults for M1 Max + 30B Model

```json
{
  "model": {
    "type": "llamacpp",
    "context_size": 16384,
    "usable_context": 12288,
    "enable_tools": false
  }
}
```

### Safe Defaults for High-RAM Systems

```json
{
  "model": {
    "type": "llamacpp",
    "context_size": 32768,
    "usable_context": 24576,
    "enable_tools": false
  }
}
```

### Verify Your Config

```bash
# Show current settings
cat ~/.pedrocli.json | jq '{
  context_size: .model.context_size,
  usable_context: .model.usable_context,
  enable_tools: .model.enable_tools
}'
```

---

## Related Documentation

- [Blog Workflow](./blog-workflow.md) - Blog generation configuration
- [Context Management](./context-window-truncation-strategy.md) - How context compaction works
- [Tool Calling Issues](./issue-100-tool-calling-fix.md) - Debugging tool calling problems

---

## Getting Help

If you encounter configuration issues:

1. **Check debug logs**: Set `"debug": {"enabled": true}` in config
2. **Verify llama-server**: `curl http://localhost:8082/health`
3. **Test with minimal config**: Start with the "Safe Defaults" above
4. **Report issues**: https://github.com/Soypete/PedroCLI/issues

---

**Last updated**: 2026-02-07
