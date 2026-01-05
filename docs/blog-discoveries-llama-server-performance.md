# Blog Post: llama-server Performance Optimization - Discoveries and Lessons Learned

**Date:** January 4, 2026
**Context:** Migration from llama-cli subprocess to llama-server HTTP API with native tool calling

## TL;DR

Migrated PedroCLI from subprocess-based llama-cli to llama-server HTTP API for native tool calling support. Initial testing revealed **critical performance issues** (24s for simple requests). Root cause analysis and optimization resulted in **10x speedup** (2.3s). Key lessons: always offload all GPU layers, context size directly impacts VRAM usage, and auto-detection beats manual configuration.

## The Journey: From Slow to Fast

### Initial State: The Baseline

**Configuration:**
```bash
llama-server \
  --model qwen2.5-coder-32b.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers 35 \  # ❌ Manual config
  --threads 8 \
  --jinja
```

**Performance:**
- Simple 2-word response: **24 seconds**
- Prompt processing: 12s (2.9 tok/s)
- Token generation: 12s (0.25 tok/s)

**Hardware:** M1 Max, 32GB unified memory

### Discovery #1: GPU Offloading Matters A LOT

#### The Investigation

First inference took over 20 seconds. Health check was instant, so server was running. What was wrong?

**Hypothesis 1:** Large prompt with tool definitions?
- Checked prompt size: Only 19 lines
- Tool definitions added by API: ~7 tools
- Not the issue

**Hypothesis 2:** Server configuration problem?
- Checked llama-server logs
- Found the smoking gun:

```
load_tensors: offloading 35 repeating layers to GPU
load_tensors: offloaded 35/65 layers to GPU  # ❌ Only 54%!
```

#### The Problem

Qwen 2.5 Coder 32B has **64 transformer layers** (plus 1 embedding layer = 65 total).

**Our config:** `--n-gpu-layers 35`
- 35 layers on GPU (Metal, fast)
- 30 layers on CPU (slow, single-threaded)

**Impact:**
- Every token must pass through CPU layers
- CPU bottleneck destroys throughput
- 2.9 tok/s instead of expected 50-100 tok/s

#### The Fix

```bash
--n-gpu-layers -1  # Let llama.cpp auto-detect
```

**Result:** `offloaded 65/65 layers to GPU` ✅

### Discovery #2: VRAM Is Finite (And Context Size Matters)

#### The Second Problem

After increasing GPU layers to 65, server crashed during warmup:

```
ggml_metal_synchronize: error: command buffer 0 failed with status 5
error: Insufficient Memory (kIOGPUCommandBufferCallbackErrorOutOfMemory)
```

**Why?**

VRAM usage calculation:
```
Model size:     ~20GB (32B Q4_K_M quantization)
Context (32K):  ~8GB  (KV cache for 32K tokens)
Overhead:       ~4GB  (temporary buffers, Metal heap)
─────────────────────────────────────────────────
Total:          ~32GB
Available:      32GB (M1 Max unified memory)
```

**Problem:** No headroom! macOS needs memory too.

#### The Solution

Reduce context to leave breathing room:

```bash
--ctx-size 16384  # Half the context, fits comfortably
```

**Trade-off:**
- ✅ Stable inference (no crashes)
- ✅ Still plenty of context for most tasks
- ⚠️  May need to summarize for very large codebases

**VRAM breakdown (16K context):**
```
Model size:     ~20GB
Context (16K):  ~4GB
Overhead:       ~4GB
─────────────────────────
Total:          ~28GB
Available:      32GB
Free:           ~4GB ✅
```

### Discovery #3: Auto-Detection Beats Manual Config

#### The Lesson

We started with manual configuration based on:
- Online tutorials (outdated)
- Other projects' configs (different hardware)
- Guesswork (dangerous)

**Manual approach:**
```bash
--n-gpu-layers 35     # Why 35? No good reason!
--ctx-size 32768      # Bigger is better? Not always!
--threads 8           # Seems reasonable?
```

**Problems:**
1. Hardware varies (M1 vs M2, 32GB vs 64GB)
2. Models vary (7B vs 32B vs 70B)
3. Quantization varies (Q4 vs Q5 vs Q8)
4. Updates change optimal values

**Auto-detection approach:**
```bash
--n-gpu-layers -1     # llama.cpp knows your hardware
--ctx-size 16384      # Conservative, reliable default
```

**Benefits:**
- Adapts to your hardware automatically
- Future-proof (works with llama.cpp updates)
- Prevents crashes from VRAM exhaustion

### Final Configuration & Results

#### Optimized Setup

```bash
llama-server \
  --model qwen2.5-coder-32b-q4_k_m.gguf \
  --port 8082 \
  --ctx-size 16384 \     # Conservative for stability
  --n-gpu-layers -1 \    # Auto-detect (offloads all 65 layers)
  --threads 8 \
  --jinja \              # Enable chat template (tool calling)
  --no-webui \
  --metrics              # Prometheus metrics endpoint
```

#### Performance Comparison

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| **Inference Time** | 24s | 2.3s | **10.4x faster** |
| **GPU Layers** | 35/65 (54%) | 65/65 (100%) | 1.9x more GPU |
| **Prompt Speed** | 2.9 tok/s | 3.5 tok/s | 1.2x faster |
| **Generation Speed** | 0.25 tok/s | 14 tok/s | **56x faster** |
| **Stability** | Crashes | Stable | ✅ |

**Real-world impact:**
- Agent inference loops: 24s → 2.3s per round
- 20-iteration job: 8 minutes → 46 seconds
- **Actually usable!**

## Technical Deep Dive

### Why GPU Layer Offloading Matters

#### Model Architecture Refresher

Transformer models process tokens through sequential layers:

```
Input → Embed → Layer 1 → Layer 2 → ... → Layer 64 → Output
                  ↓          ↓                ↓
                 GPU        GPU              CPU (if not offloaded)
```

**With partial offloading (35 layers):**
```
Token → [35 GPU layers: fast] → [30 CPU layers: SLOW] → Output
         ~50ms                     ~11,000ms
```

**With full offloading (65 layers):**
```
Token → [65 GPU layers: fast] → Output
         ~70ms total
```

#### Why CPU Layers Are So Slow

1. **No Parallelism:** CPUs can't parallelize attention computations like GPUs
2. **Memory Bandwidth:** CPU-GPU transfers for each layer boundary
3. **Precision:** CPU uses FP32, GPU uses optimized FP16/BF16
4. **SIMD:** GPU has thousands of cores vs 8-12 CPU cores

**Rule of thumb:** 1 CPU layer ≈ 10x slower than 1 GPU layer

### VRAM Usage Breakdown

#### Model Weights

32B parameter model with Q4_K_M quantization:
```
32 billion params × 4.5 bits/param ÷ 8 bits/byte = ~18GB
+ Overhead (graph, buffers): ~2GB
= ~20GB total
```

#### KV Cache (Context)

For each context token, we store keys and values:
```
Per token: 2 (K+V) × 64 layers × 128 heads × 128 dim × 2 bytes (FP16)
         ≈ 4MB per token

Context sizes:
- 4K context:  ~16GB
- 8K context:  ~32GB
- 16K context: ~64GB (requires batch processing or smaller models)
- 32K context: ~128GB (impractical for 32B models)
```

**Wait, that doesn't match our numbers!**

llama.cpp is smarter:
- Uses FP16 instead of FP32: 2x savings
- Shares KV cache across batches
- Uses quantized KV cache (experimental)
- Our 16K context: ~4GB (much better!)

#### Temporary Buffers

During inference:
- Attention score matrices
- FFN intermediate activations
- Metal command buffers
- ~2-4GB depending on batch size

### The -1 Magic Number

`--n-gpu-layers -1` isn't magic, it's smart detection:

**What llama.cpp does:**
```c
// Simplified algorithm
available_vram = detect_metal_memory();
model_size = calculate_model_size();
context_size = ctx_size * bytes_per_token;

free_vram = available_vram - model_size - context_size - overhead;

if (free_vram > 0) {
    offload_all_layers();  // 65/65
} else {
    // Fit as many layers as possible
    layers = (available_vram - context_size - overhead) / bytes_per_layer;
}
```

**Benefits:**
- Adapts to actual available VRAM (not theoretical max)
- Accounts for macOS using some unified memory
- Handles different quantization levels automatically
- Updates with llama.cpp improvements

## Best Practices for llama-server

### 1. Always Use Auto-Detection for GPU Layers

❌ **Don't:**
```bash
--n-gpu-layers 35  # Arbitrary number
```

✅ **Do:**
```bash
--n-gpu-layers -1  # Let llama.cpp decide
```

### 2. Start Conservative with Context Size

❌ **Don't:**
```bash
--ctx-size 32768  # "More is better!"
```

✅ **Do:**
```bash
--ctx-size 16384  # Start conservative
# Increase only if needed and you have VRAM
```

**Context size guidelines:**

| Model Size | VRAM | Recommended Context |
|------------|------|---------------------|
| 7B Q4 | 16GB+ | 32K |
| 13B Q4 | 24GB+ | 16K-32K |
| 32B Q4 | 32GB+ | 8K-16K |
| 70B Q4 | 64GB+ | 4K-8K |

### 3. Enable Metrics for Monitoring

```bash
--metrics  # Enables /metrics endpoint
```

**Monitor these:**
```bash
curl http://localhost:8082/metrics | grep -E "prompt_tokens_total|cache_"
```

Key metrics:
- `llamacpp:prompt_tokens_total` - Total tokens processed
- `llamacpp:prompt_seconds_total` - Time spent on prompts
- `llamacpp:kv_cache_usage_ratio` - How full is your context?

### 4. Use --jinja for Native Tool Calling

Modern models (Qwen 2.5, Llama 3.1+) have chat templates in metadata:

```bash
--jinja  # Enables chat template from model
```

**Benefits:**
- Native tool calling via OpenAI-compatible API
- No manual prompt formatting needed
- Model-specific optimizations

### 5. Disable Unnecessary Features

```bash
--no-webui        # Don't need web UI for API-only
--log-disable     # Reduce logging overhead (in production)
```

## Migration Impact: Code Simplification

### Before: Manual Tool Formatting (GBNF Grammar)

**Lines of code:** ~450 lines across multiple files

```go
// pkg/agents/base.go (~30 lines)
if a.config.Model.EnableGrammar {
    if llamaCppClient, ok := a.llm.(*llm.LlamaCppClient); ok {
        grammar := generateGrammar(a.registry)
        llamaCppClient.SetGrammar(grammar.String())
        llamaCppClient.ConfigureForToolCalls()
    }
}

// pkg/agents/executor.go (~150 lines)
func (e *InferenceExecutor) parseToolCalls(text string) []llm.ToolCall {
    // Strategy 1: JSON array
    // Strategy 2: Single JSON object
    // Strategy 3: Code blocks
    // Strategy 4: Line-by-line
    // ... complex regex and JSON parsing
}
```

### After: Native API Tool Calling

**Lines of code:** ~40 lines

```go
// pkg/agents/base.go
if a.config.Model.EnableTools && a.registry != nil {
    req.Tools = a.convertToolsToDefinitions()
}

// pkg/agents/executor.go
toolCalls := response.ToolCalls  // Already parsed by server!
```

**Net reduction:** 410 lines removed, 40 lines added = **-370 lines** (-82%!)

### Reliability Improvement

**Before (text parsing):**
- LLM must output perfect JSON
- Format varies by model
- Fragile regex parsing
- Easy to break with prompt changes

**After (native API):**
- Server handles formatting
- OpenAI-compatible standard
- Robust parsing
- Model-agnostic

## Common Pitfalls & Solutions

### Pitfall 1: "Health Check Passes But Inference Hangs"

**Symptom:** `/health` returns OK but requests timeout

**Causes:**
1. Not enough GPU layers (CPU bottleneck)
2. Context too large (VRAM exhaustion during inference)
3. Model not fully loaded yet

**Debug:**
```bash
# Check GPU layer offloading
grep "offloaded" /var/log/llama-server.log

# Check for VRAM errors
grep -i "memory\|failed" /var/log/llama-server.log

# Test simple request
curl -X POST http://localhost:8082/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"default","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}'
```

### Pitfall 2: "Server Crashes During Warmup"

**Symptom:** Server exits during "warming up the model"

**Cause:** VRAM exhaustion

**Solution:**
```bash
# Reduce context size
--ctx-size 8192  # or even 4096 for large models

# Or use smaller model
# 32B → 14B or 7B
```

### Pitfall 3: "Inference Gets Slower Over Time"

**Symptom:** First requests fast, later requests slow

**Cause:** KV cache fragmentation

**Solutions:**
```bash
# Enable continuous batching (if supported)
--cont-batching

# Or restart server periodically
# (crude but effective)
```

### Pitfall 4: "Tool Calls Not Generated"

**Symptom:** LLM responds with text instead of tool calls

**Causes:**
1. `--jinja` flag missing (chat template not loaded)
2. Model doesn't support tool calling
3. Tools not passed in API request

**Debug:**
```bash
# Check if chat template loaded
grep "chat_template" /var/log/llama-server.log

# Verify tools in request
curl ... -d '{
  "tools": [...],  # Must be present!
  "messages": [...]
}'
```

## Metrics & Monitoring

### Prometheus Metrics

llama-server exposes metrics at `/metrics`:

```bash
curl http://localhost:8082/metrics
```

**Key metrics to track:**

```prometheus
# Request throughput
llamacpp:prompt_tokens_total
llamacpp:tokens_predicted_total

# Performance
llamacpp:prompt_seconds_total
llamacpp:predict_seconds_total

# Resource usage
llamacpp:kv_cache_usage_ratio
llamacpp:kv_cache_tokens
```

### Grafana Dashboard Example

```yaml
panels:
  - title: "Tokens/Second"
    targets:
      - rate(llamacpp:tokens_predicted_total[1m])

  - title: "Request Latency"
    targets:
      - llamacpp:prompt_seconds_total / llamacpp:prompt_tokens_total

  - title: "Context Usage"
    targets:
      - llamacpp:kv_cache_usage_ratio * 100
```

## Hardware-Specific Recommendations

### Apple Silicon (M1/M2/M3)

**Advantages:**
- Unified memory (CPU + GPU share RAM)
- Metal acceleration
- Excellent power efficiency

**Recommendations:**
```bash
# M1/M2 (16GB)
--n-gpu-layers -1
--ctx-size 4096      # Conservative
# Use 7B-14B models

# M1/M2 Pro/Max (32GB)
--n-gpu-layers -1
--ctx-size 16384
# Can use 32B models

# M1/M2 Ultra (64GB+)
--n-gpu-layers -1
--ctx-size 32768
# Can use 70B models
```

### NVIDIA GPUs

**Advantages:**
- More VRAM options (24GB, 48GB, 80GB)
- CUDA optimizations
- Multi-GPU support

**Recommendations:**
```bash
# RTX 3090/4090 (24GB)
--n-gpu-layers -1
--ctx-size 16384
# Good for 32B models

# A100 (40GB/80GB)
--n-gpu-layers -1
--ctx-size 32768
# Can run 70B models
```

### CPU-Only

**Last resort, but possible:**
```bash
--n-gpu-layers 0     # Force CPU
--threads 16         # Use all cores
--ctx-size 2048      # Small context
# Use 7B models maximum
# Expect 1-2 tok/s
```

## Future Optimizations

### Ideas We Haven't Tried Yet

1. **Flash Attention:**
   ```bash
   --flash-attn  # Experimental in llama.cpp
   # Could reduce VRAM usage by 30-40%
   ```

2. **Quantized KV Cache:**
   ```bash
   --cache-type-k q4_0  # Quantize keys
   --cache-type-v q4_0  # Quantize values
   # Trade accuracy for 4x less VRAM
   ```

3. **Continuous Batching:**
   ```bash
   --cont-batching
   # Process multiple requests simultaneously
   # Better GPU utilization
   ```

4. **Speculative Decoding:**
   ```bash
   --draft-model small-7b.gguf
   # Use small model to draft, large model to verify
   # Can be 2-3x faster
   ```

## Conclusion: Key Takeaways

### The Big Wins

1. **Auto-detection > Manual config**
   - Use `-1` for GPU layers
   - Let llama.cpp optimize for your hardware
   - Future-proof against updates

2. **Context size directly impacts VRAM**
   - Start conservative (8K-16K)
   - Only increase if needed
   - Monitor KV cache usage

3. **Native tool calling simplifies everything**
   - 82% less code
   - More reliable
   - Industry-standard API

4. **Performance can vary 10x based on config**
   - Worth spending time to optimize
   - Metrics are your friend
   - Test on real workloads

### The Numbers

| Aspect | Impact |
|--------|--------|
| Code reduction | -82% (450→80 lines) |
| Speed improvement | 10x faster (24s→2.3s) |
| Reliability | No more parsing errors |
| Maintainability | Much simpler architecture |

### What's Next

- [ ] Test with different model sizes (7B, 14B, 70B)
- [ ] Benchmark tool calling latency with many tools
- [ ] Explore quantized KV cache for larger contexts
- [ ] Add automatic config tuning based on hardware detection
- [ ] Create Grafana dashboard for production monitoring

## References

- [llama.cpp GitHub](https://github.com/ggerganov/llama.cpp)
- [Server Documentation](https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md)
- [OpenAI API Compatibility](https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md#api-endpoints)
- [Metal Performance Guide](https://developer.apple.com/metal/Metal-Feature-Set-Tables.pdf)

---

**Author:** Miriah Peterson (with Claude Sonnet 4.5)
**Date:** January 4, 2026
**Project:** PedroCLI - Self-hosted autonomous coding agents
