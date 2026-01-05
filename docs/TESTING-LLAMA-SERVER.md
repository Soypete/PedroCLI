# llama-server Testing Guide

This document describes how to test the llama-server HTTP API integration.

## Overview

PedroCLI now uses llama-server (HTTP API) instead of llama-cli (one-shot subprocess) for better performance, native tool calling, and unified backend architecture.

## Test Results (2026-01-04)

**Unit Tests:** ✅ All 5 tests passing
```
TestServerClient_Infer_BasicResponse      PASS
TestServerClient_Infer_WithToolCalls      PASS
TestServerClient_ContextWindow            PASS
TestServerClient_Timeout                  PASS
TestServerClient_ErrorResponse            PASS
```

**Integration Tests:** ✅ Verified
- llama-server starts successfully on port 8082
- Health endpoint returns `{"status":"ok"}`
- Chat completions endpoint works with Qwen 2.5 Coder 32B
- Response format is OpenAI-compatible
- Includes usage stats and timing information

## Test Coverage

### 1. Unit Tests

Unit tests for `pkg/llm/server.go` test the generic HTTP client with mock servers.

**Run unit tests:**
```bash
go test -v ./pkg/llm/server_test.go ./pkg/llm/server.go ./pkg/llm/interface.go ./pkg/llm/tokens.go
```

**What's tested:**
- ✅ Basic inference with text responses
- ✅ Tool calling with OpenAI-compatible API
- ✅ Context window size calculations
- ✅ Timeout handling
- ✅ Error response handling

### 2. Integration Tests

Integration tests verify llama-server works end-to-end with a real model.

**Run integration tests:**
```bash
./test-llama-server.sh
```

**What's tested:**
- ✅ llama-server startup and health check
- ✅ Basic chat completions endpoint
- ✅ Tool calling via chat completions
- ✅ Models metadata endpoint

**Requirements:**
- llama-server installed (from llama.cpp)
- Qwen 2.5 Coder model downloaded
- `jq` for JSON parsing

## Manual Testing

### Step 1: Start llama-server

```bash
make llama-server
```

This starts llama-server with:
- Port: 8081
- Model: Qwen 2.5 Coder 32B (auto-detected from ~/.cache/huggingface)
- Context: 32768 tokens
- GPU layers: 35
- Chat template: Qwen 2.5 (from model metadata)

**Verify it's running:**
```bash
make llama-health
```

Expected output:
```json
{
  "status": "ok"
}
```

### Step 2: Test Basic Inference

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "default",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Say hello in 2 words."}
    ],
    "temperature": 0.1,
    "max_tokens": 10
  }' | jq .
```

Expected response:
```json
{
  "choices": [
    {
      "message": {
        "content": "Hello there!"
      }
    }
  ],
  "usage": {
    "total_tokens": 25
  }
}
```

### Step 3: Test Tool Calling

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "default",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant with access to tools."},
      {"role": "user", "content": "Search for information about Golang"}
    ],
    "temperature": 0.1,
    "max_tokens": 100,
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "search",
          "description": "Search for information",
          "parameters": {
            "type": "object",
            "properties": {
              "query": {
                "type": "string",
                "description": "The search query"
              }
            },
            "required": ["query"]
          }
        }
      }
    ]
  }' | jq .
```

Expected response (with tool call):
```json
{
  "choices": [
    {
      "message": {
        "content": "",
        "tool_calls": [
          {
            "function": {
              "name": "search",
              "arguments": "{\"query\":\"Golang\"}"
            }
          }
        ]
      }
    }
  ]
}
```

### Step 4: Stop the Server

```bash
make stop-llama
```

## Configuration Options

You can customize llama-server behavior via environment variables:

```bash
# Use different port
LLAMA_PORT=9090 make llama-server

# Use specific model
LLAMA_MODEL=/path/to/model.gguf make llama-server

# Adjust context size
LLAMA_CTX_SIZE=65536 make llama-server

# Add grammar constraint
LLAMA_GRAMMAR_FILE=/path/to/grammar.gbnf make llama-server

# Add logit bias (increase likelihood of specific tokens)
LLAMA_LOGIT_BIAS="--logit-bias 15043+1" make llama-server
```

## Troubleshooting

### Server won't start

**Check if port is in use:**
```bash
lsof -i :8081
```

**Check model path:**
```bash
find ~/.cache/huggingface/hub/models--bartowski--Qwen2.5-Coder-32B-Instruct-GGUF -name "*.gguf"
```

**View server logs:**
```bash
# Integration test logs
cat /tmp/llama-server-test.log

# Or start server manually to see output
llama-server --model ~/.cache/.../model.gguf --port 8081 --jinja
```

### Health check fails

Wait a few seconds for the model to load:
```bash
# Check every 2 seconds
watch -n 2 'curl -s http://localhost:8081/health | jq .'
```

### Tool calls not generated

1. **Verify chat template is loaded:**
   - Qwen 2.5 models should have chat template in metadata
   - `--jinja` flag enables template parsing

2. **Try explicit tool prompt:**
   ```bash
   curl -X POST http://localhost:8081/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "default",
       "messages": [
         {"role": "system", "content": "You MUST call the search tool."},
         {"role": "user", "content": "Search for Python"}
       ],
       "tools": [...]
     }'
   ```

3. **Check model supports tool calling:**
   - Qwen 2.5 Coder models support native tool calling
   - Other models may need different prompting

### Out of memory

Reduce GPU layers or context size:
```bash
LLAMA_N_GPU_LAYERS=20 LLAMA_CTX_SIZE=16384 make llama-server
```

## Performance Benchmarks

After starting llama-server, first request will be slow (model loading). Subsequent requests should be fast:

- **First request:** 5-10 seconds (model load)
- **Subsequent requests:** 1-3 seconds (inference only)
- **Context:** ~30k tokens/request supported

Compare to llama-cli (one-shot):
- **Every request:** 5-10 seconds (model reload each time)

## Next Steps

After verifying llama-server works:

1. Update agent code to use native tool calling (Phase 5)
2. Remove deprecated grammar generation code
3. Update documentation and migration guide

## Related Files

- `Makefile` - llama-server targets
- `pkg/llm/server.go` - Generic HTTP client
- `pkg/llm/llamacpp.go` - llama-server backend
- `.pedrocli-llamacpp-server.json.example` - Example config
- `test-llama-server.sh` - Integration test script
