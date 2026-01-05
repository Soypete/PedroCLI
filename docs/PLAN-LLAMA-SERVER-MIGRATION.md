# Migration Plan: llama-cli → llama-server (HTTP API)

## Objective

Migrate from llama-cli one-shot subprocess execution to llama-server HTTP API to enable:
- **Native tool calling** via Qwen's chat template
- **Better logit control** (grammars, sampling parameters)
- **Unified LLM interface** across ollama/llama.cpp/vllm/lm studio
- **Better performance** (persistent model loading, no subprocess overhead)

## Architecture Overview

### Current State
```
pkg/llm/
├── interface.go       # Backend interface
├── llamacpp.go        # One-shot CLI subprocess (DEPRECATED)
├── ollama.go          # HTTP API client
└── factory.go         # Backend factory
```

### Target State
```
pkg/llm/
├── interface.go       # Generic Backend interface (UPDATED)
├── server.go          # Generic HTTP server client (NEW)
├── ollama.go          # Ollama-specific implementation
├── llamacpp.go        # llama-server implementation (REWRITTEN)
├── vllm.go            # vLLM implementation (FUTURE)
├── lmstudio.go        # LM Studio implementation (FUTURE)
└── factory.go         # Backend factory (UPDATED)
```

## Phase 1: Infrastructure Setup

### 1.1 Add Makefile Targets

**File**: `Makefile`

```makefile
# llama-server configuration
LLAMA_SERVER_PORT ?= 8081
LLAMA_MODEL ?= ~/.cache/huggingface/hub/models--bartowski--Qwen2.5-Coder-32B-Instruct-GGUF/snapshots/*/Qwen2.5-Coder-32B-Instruct-Q4_K_M.gguf
LLAMA_CTX_SIZE ?= 32768
LLAMA_N_GPU_LAYERS ?= 35
LLAMA_THREADS ?= 8

.PHONY: llama-server
llama-server: ## Start llama-server with configured model
	@echo "Starting llama-server on port $(LLAMA_SERVER_PORT)..."
	llama-server \
		--model $(LLAMA_MODEL) \
		--port $(LLAMA_PORT) \
		--ctx-size $(LLAMA_CTX_SIZE) \
		--n-gpu-layers $(LLAMA_N_GPU_LAYERS) \
		--threads $(LLAMA_THREADS) \
		--chat-template qwen \
		--log-disable \
		--metrics

.PHONY: llama-server-tools
llama-server-tools: ## Start llama-server with tool calling enabled
	@echo "Starting llama-server with tool calling support..."
	llama-server \
		--model $(LLAMA_MODEL) \
		--port $(LLAMA_PORT) \
		--ctx-size $(LLAMA_CTX_SIZE) \
		--n-gpu-layers $(LLAMA_N_GPU_LAYERS) \
		--threads $(LLAMA_THREADS) \
		--chat-template qwen \
		--log-disable \
		--metrics \
		--jinja

.PHONY: llama-health
llama-health: ## Check llama-server health
	@curl -s http://localhost:$(LLAMA_SERVER_PORT)/health || echo "Server not running"

.PHONY: stop-llama
stop-llama: ## Stop llama-server
	@pkill -f llama-server || echo "No llama-server running"
```

### 1.2 Update Configuration Schema

**File**: `pkg/config/config.go`

```go
type ModelConfig struct {
    Type          string  `json:"type"`           // "ollama" | "llamacpp" | "vllm" | "lmstudio"

    // Generic server settings (for all HTTP-based backends)
    ServerURL     string  `json:"server_url"`     // e.g., "http://localhost:8081" or "http://localhost:11434"
    ModelName     string  `json:"model_name"`     // Model identifier

    // Context settings
    ContextSize   int     `json:"context_size"`
    UsableContext int     `json:"usable_context"`

    // Generation parameters
    Temperature   float64 `json:"temperature"`
    MaxTokens     int     `json:"max_tokens"`

    // Tool calling
    EnableTools   bool    `json:"enable_tools"`   // Enable native tool calling via chat template

    // DEPRECATED - for backward compatibility only
    LlamaCppPath  string  `json:"llamacpp_path,omitempty"` // DEPRECATED: use ServerURL
    ModelPath     string  `json:"model_path,omitempty"`    // DEPRECATED: use ModelName
    Threads       int     `json:"threads,omitempty"`       // DEPRECATED: server setting
    NGpuLayers    int     `json:"n_gpu_layers,omitempty"`  // DEPRECATED: server setting
    EnableGrammar bool    `json:"enable_grammar,omitempty"` // DEPRECATED: use EnableTools
}
```

### 1.3 Example Configurations

**File**: `.pedrocli-llamacpp-server.json.example`

```json
{
  "model": {
    "type": "llamacpp",
    "server_url": "http://localhost:8081",
    "model_name": "qwen2.5-coder-32b",
    "context_size": 32768,
    "usable_context": 24576,
    "temperature": 0.2,
    "max_tokens": 8192,
    "enable_tools": true
  },
  "database": { ... },
  "project": { ... }
}
```

**File**: `.pedrocli-ollama.json.example`

```json
{
  "model": {
    "type": "ollama",
    "server_url": "http://localhost:11434",
    "model_name": "qwen2.5-coder:32b",
    "context_size": 32768,
    "temperature": 0.2,
    "enable_tools": true
  },
  "database": { ... },
  "project": { ... }
}
```

## Phase 2: Unified LLM Interface

### 2.1 Define Generic Backend Interface

**File**: `pkg/llm/interface.go` (UPDATED)

```go
package llm

import (
    "context"
)

// Backend represents a generic LLM backend
type Backend interface {
    // Infer performs inference with optional tool definitions
    Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)

    // GetContextWindow returns the total context window size
    GetContextWindow() int

    // GetUsableContext returns the usable context size (usually 75% of total)
    GetUsableContext() int

    // SupportsNativeTools returns true if backend supports native tool calling
    SupportsNativeTools() bool

    // Close closes any persistent connections
    Close() error
}

// InferenceRequest represents a request for LLM inference
type InferenceRequest struct {
    // Prompts
    SystemPrompt string
    UserPrompt   string

    // Generation parameters
    Temperature float64
    MaxTokens   int

    // Tool calling (optional)
    Tools []ToolDefinition // If provided and backend supports it, enables tool calling

    // Advanced parameters (optional)
    TopP          float64
    TopK          int
    RepeatPenalty float64
}

// InferenceResponse represents the LLM's response
type InferenceResponse struct {
    Text       string      // Raw text response
    ToolCalls  []ToolCall  // Parsed tool calls (if any)
    NextAction string      // "CONTINUE" | "COMPLETE"
    TokensUsed int         // Estimated tokens used
}

// ToolDefinition defines a tool for native tool calling
type ToolDefinition struct {
    Name        string
    Description string
    Parameters  map[string]interface{} // JSON Schema
}

// ToolCall represents a parsed tool call from the LLM
type ToolCall struct {
    Name string
    Args map[string]interface{}
}
```

### 2.2 Implement Generic HTTP Server Client

**File**: `pkg/llm/server.go` (NEW)

```go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// ServerClient is a generic HTTP-based LLM client
// It handles OpenAI-compatible chat completion APIs
type ServerClient struct {
    baseURL       string
    modelName     string
    contextSize   int
    usableSize    int
    enableTools   bool
    httpClient    *http.Client
    parseToolCall ToolCallParser // Model-specific tool call parser
}

// ToolCallParser extracts tool calls from LLM response
type ToolCallParser func(response string) ([]ToolCall, error)

// NewServerClient creates a new generic server client
func NewServerClient(baseURL, modelName string, contextSize int, enableTools bool, parser ToolCallParser) *ServerClient {
    return &ServerClient{
        baseURL:       baseURL,
        modelName:     modelName,
        contextSize:   contextSize,
        usableSize:    int(float64(contextSize) * 0.75),
        enableTools:   enableTools,
        httpClient:    &http.Client{Timeout: 5 * time.Minute},
        parseToolCall: parser,
    }
}

// Infer performs inference using OpenAI-compatible chat completions API
func (c *ServerClient) Infer(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
    // Build chat completion request
    chatReq := map[string]interface{}{
        "model": c.modelName,
        "messages": []map[string]string{
            {"role": "system", "content": req.SystemPrompt},
            {"role": "user", "content": req.UserPrompt},
        },
        "temperature": req.Temperature,
        "max_tokens":  req.MaxTokens,
        "stream":      false,
    }

    // Add tools if supported and provided
    if c.enableTools && len(req.Tools) > 0 {
        chatReq["tools"] = c.formatTools(req.Tools)
    }

    // Make HTTP request
    body, err := json.Marshal(chatReq)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(bodyBytes))
    }

    // Parse response
    var chatResp struct {
        Choices []struct {
            Message struct {
                Content   string `json:"content"`
                ToolCalls []struct {
                    Function struct {
                        Name      string `json:"name"`
                        Arguments string `json:"arguments"`
                    } `json:"function"`
                } `json:"tool_calls,omitempty"`
            } `json:"message"`
        } `json:"choices"`
        Usage struct {
            TotalTokens int `json:"total_tokens"`
        } `json:"usage"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    if len(chatResp.Choices) == 0 {
        return nil, fmt.Errorf("no choices in response")
    }

    message := chatResp.Choices[0].Message

    // Parse tool calls
    var toolCalls []ToolCall
    if len(message.ToolCalls) > 0 {
        // Native tool calls from API
        for _, tc := range message.ToolCalls {
            var args map[string]interface{}
            if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
                continue
            }
            toolCalls = append(toolCalls, ToolCall{
                Name: tc.Function.Name,
                Args: args,
            })
        }
    } else if c.parseToolCall != nil {
        // Parse from text using model-specific parser
        toolCalls, _ = c.parseToolCall(message.Content)
    }

    return &InferenceResponse{
        Text:       message.Content,
        ToolCalls:  toolCalls,
        NextAction: "CONTINUE",
        TokensUsed: chatResp.Usage.TotalTokens,
    }, nil
}

// formatTools converts tool definitions to OpenAI format
func (c *ServerClient) formatTools(tools []ToolDefinition) []map[string]interface{} {
    result := make([]map[string]interface{}, len(tools))
    for i, tool := range tools {
        result[i] = map[string]interface{}{
            "type": "function",
            "function": map[string]interface{}{
                "name":        tool.Name,
                "description": tool.Description,
                "parameters":  tool.Parameters,
            },
        }
    }
    return result
}

// GetContextWindow returns the context window size
func (c *ServerClient) GetContextWindow() int {
    return c.contextSize
}

// GetUsableContext returns the usable context size
func (c *ServerClient) GetUsableContext() int {
    return c.usableSize
}

// SupportsNativeTools returns whether native tool calling is enabled
func (c *ServerClient) SupportsNativeTools() bool {
    return c.enableTools
}

// Close closes the HTTP client
func (c *ServerClient) Close() error {
    c.httpClient.CloseIdleConnections()
    return nil
}
```

### 2.3 Rewrite llama.cpp Backend

**File**: `pkg/llm/llamacpp.go` (REWRITTEN)

```go
package llm

import (
    "github.com/soypete/pedrocli/pkg/config"
)

// LlamaCppClient implements Backend for llama-server HTTP API
type LlamaCppClient struct {
    *ServerClient
}

// NewLlamaCppClient creates a new llama-server client
func NewLlamaCppClient(cfg *config.Config) *LlamaCppClient {
    modelCfg := cfg.Model

    // Default to localhost:8081 if not specified
    serverURL := modelCfg.ServerURL
    if serverURL == "" {
        serverURL = "http://localhost:8081"
    }

    // Use model_name or fallback to a default
    modelName := modelCfg.ModelName
    if modelName == "" {
        modelName = "qwen2.5-coder-32b"
    }

    // Create base server client with Qwen-specific tool call parser
    serverClient := NewServerClient(
        serverURL,
        modelName,
        modelCfg.ContextSize,
        modelCfg.EnableTools,
        parseQwenToolCalls, // Model-specific parser
    )

    return &LlamaCppClient{
        ServerClient: serverClient,
    }
}

// parseQwenToolCalls parses Qwen-style <tool_call> tags from text
func parseQwenToolCalls(response string) ([]ToolCall, error) {
    // Implementation using regex to extract <tool_call>...</tool_call>
    // This is a fallback if native tool calling isn't used
    // ... (similar to existing parsing logic)
    return nil, nil
}
```

### 2.4 Update Ollama Backend

**File**: `pkg/llm/ollama.go` (UPDATED to use ServerClient)

```go
package llm

import (
    "github.com/soypete/pedrocli/pkg/config"
)

// OllamaClient implements Backend for Ollama HTTP API
type OllamaClient struct {
    *ServerClient
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(cfg *config.Config) *OllamaClient {
    modelCfg := cfg.Model

    serverURL := modelCfg.ServerURL
    if serverURL == "" {
        serverURL = "http://localhost:11434"
    }

    serverClient := NewServerClient(
        serverURL,
        modelCfg.ModelName,
        modelCfg.ContextSize,
        modelCfg.EnableTools,
        nil, // Ollama uses native tool calling
    )

    return &OllamaClient{
        ServerClient: serverClient,
    }
}
```

### 2.5 Update Factory

**File**: `pkg/llm/factory.go` (UPDATED)

```go
package llm

import (
    "fmt"
    "github.com/soypete/pedrocli/pkg/config"
)

// NewBackend creates a new LLM backend based on config
func NewBackend(cfg *config.Config) (Backend, error) {
    switch cfg.Model.Type {
    case "ollama":
        return NewOllamaClient(cfg), nil

    case "llamacpp":
        return NewLlamaCppClient(cfg), nil

    case "vllm":
        // Future: return NewVLLMClient(cfg), nil
        return nil, fmt.Errorf("vllm backend not yet implemented")

    case "lmstudio":
        // Future: return NewLMStudioClient(cfg), nil
        return nil, fmt.Errorf("lmstudio backend not yet implemented")

    default:
        return nil, fmt.Errorf("unknown backend type: %s (supported: ollama, llamacpp)", cfg.Model.Type)
    }
}
```

## Phase 3: Agent Integration

### 3.1 Remove Manual Tool Formatting

**Files to Update**:
- `pkg/agents/executor.go` - Remove `buildSystemPromptWithFormatter()` and related helpers
- `pkg/agents/coding.go` - Simplify `buildCodingSystemPrompt()` to just return base prompt
- `pkg/prompts/tool_generator.go` - DEPRECATED (tools now in API request)

**Why**: When using native tool calling, tools are passed in the API request, not in the prompt.

### 3.2 Update InferenceRequest Creation

**File**: `pkg/agents/base.go`

```go
func (a *BaseAgent) executeInference(ctx context.Context, contextMgr *llmcontext.Manager, userPrompt string) (*llm.InferenceResponse, error) {
    // Build inference request
    req := &llm.InferenceRequest{
        SystemPrompt: a.buildSystemPrompt(), // Just the base prompt, no tools
        UserPrompt:   userPrompt,
        Temperature:  a.config.Model.Temperature,
        MaxTokens:    8192,
    }

    // Add tools if backend supports native tool calling
    if a.llm.SupportsNativeTools() && a.registry != nil {
        req.Tools = a.convertToolsToDefinitions()
    }

    // Execute inference
    return a.llm.Infer(ctx, req)
}

func (a *BaseAgent) convertToolsToDefinitions() []llm.ToolDefinition {
    var tools []llm.ToolDefinition

    for _, tool := range a.registry.List() {
        tools = append(tools, llm.ToolDefinition{
            Name:        tool.Name(),
            Description: tool.Description(),
            Parameters:  convertToJSONSchema(tool), // Helper to extract schema
        })
    }

    return tools
}
```

### 3.3 Simplify Tool Call Parsing

**File**: `pkg/agents/executor.go`

```go
// parseToolCalls extracts tool calls from response
// With native tool calling, these come from InferenceResponse.ToolCalls
func (e *InferenceExecutor) parseToolCalls(response *llm.InferenceResponse) []llm.ToolCall {
    // If using native tool calling, tool calls are already parsed
    if len(response.ToolCalls) > 0 {
        return response.ToolCalls
    }

    // Fallback: parse from text using formatter (for non-native backends)
    if e.formatter != nil {
        parsed, _ := e.formatter.ParseToolCalls(response.Text)
        calls := make([]llm.ToolCall, len(parsed))
        for i, tc := range parsed {
            calls[i] = llm.ToolCall{Name: tc.Name, Args: tc.Args}
        }
        return calls
    }

    return nil
}
```

## Phase 4: Testing & Validation

### 4.1 Update Tests

**Files**:
- `pkg/llm/llamacpp_test.go` - Update to test HTTP API instead of CLI
- `pkg/llm/server_test.go` - New tests for ServerClient
- `pkg/agents/executor_test.go` - Update mocks for native tool calling

### 4.2 Integration Test

**File**: `test-llama-server.sh` (NEW)

```bash
#!/bin/bash
# Integration test for llama-server backend

set -e

echo "=== Testing llama-server Backend ==="

# Start llama-server
echo "Starting llama-server..."
make llama-server-tools &
SERVER_PID=$!

# Wait for server to be ready
sleep 10

# Check health
echo "Checking server health..."
curl -s http://localhost:8081/health || (echo "Server not ready" && exit 1)

# Run test
echo "Running builder test..."
./pedrocli build -issue "test" -description "Test native tool calling" \
    -config .pedrocli-llamacpp-server.json

# Cleanup
echo "Stopping server..."
kill $SERVER_PID

echo "=== Test Complete ==="
```

### 4.3 Performance Comparison

Create benchmarks to compare:
- **llama-cli (one-shot)**: Current approach
- **llama-server (HTTP)**: New approach

Expected improvements:
- 5-10x faster after first request (model stays loaded)
- Better tool call accuracy (native chat template)
- Lower memory overhead (no subprocess spawning)

## Phase 5: Documentation & Cleanup

### 5.1 Update Documentation

**Files to Update**:
- `README.md` - Update setup instructions
- `CLAUDE.md` - Update architecture section
- `docs/pedrocli-context-guide.md` - Update LLM backend section
- `docs/builder-agent-usage.md` - Update configuration examples

**Files to Create**:
- `docs/llama-server-setup.md` - Complete guide for llama-server setup
- `docs/backends-comparison.md` - Compare ollama/llamacpp/vllm backends

### 5.2 Remove Deprecated Code

**Files to Delete**:
- Old one-shot llama-cli code (after confirming HTTP works)
- Grammar-related code (superseded by native tool calling)
- Manual tool prompt generation (superseded by API tools param)

**Deprecation Notices**:
```go
// DEPRECATED: Use ServerURL instead
// This field is only used for backward compatibility
LlamaCppPath string `json:"llamacpp_path,omitempty"`
```

### 5.3 Migration Guide

**File**: `docs/MIGRATION-LLAMA-SERVER.md`

```markdown
# Migration Guide: llama-cli → llama-server

## For Users

### Before (llama-cli)
```json
{
  "model": {
    "type": "llamacpp",
    "llamacpp_path": "/opt/homebrew/bin/llama-cli",
    "model_path": "~/.cache/huggingface/.../model.gguf",
    "context_size": 32768,
    "threads": 8,
    "n_gpu_layers": 35
  }
}
```

### After (llama-server)
```json
{
  "model": {
    "type": "llamacpp",
    "server_url": "http://localhost:8081",
    "model_name": "qwen2.5-coder-32b",
    "context_size": 32768,
    "enable_tools": true
  }
}
```

### Setup Steps

1. Start llama-server:
   ```bash
   make llama-server-tools MODEL=path/to/model.gguf
   ```

2. Update config to use `server_url`

3. Run pedrocli commands normally
```

## Implementation Timeline

### Week 1: Infrastructure
- [ ] Add Makefile targets for llama-server
- [ ] Update config schema with ServerURL
- [ ] Create example configs

### Week 2: Core Implementation
- [ ] Implement ServerClient (generic HTTP client)
- [ ] Rewrite LlamaCppClient to use ServerClient
- [ ] Update OllamaClient to use ServerClient
- [ ] Update factory.go

### Week 3: Agent Integration
- [ ] Remove manual tool formatting from executor
- [ ] Update agents to use native tool calling
- [ ] Simplify tool call parsing

### Week 4: Testing & Cleanup
- [ ] Write integration tests
- [ ] Performance benchmarks
- [ ] Update documentation
- [ ] Remove deprecated code

## Success Criteria

- ✅ llama-server starts successfully via Makefile
- ✅ Native tool calling works with Qwen 2.5
- ✅ All agents work with new backend
- ✅ Tests pass
- ✅ Performance improves (faster inference after warmup)
- ✅ Documentation complete

## Rollback Plan

Since this is local development only:
- Keep old configs as `.pedrocli-llamacpp-legacy.json.example`
- Tag current commit before migration
- Can revert to old one-shot approach if needed

## Future Enhancements

After migration complete:
1. Add vLLM backend support
2. Add LM Studio backend support
3. Add streaming support for real-time output
4. Add multi-turn conversation support
5. Add fine-tuned model support
