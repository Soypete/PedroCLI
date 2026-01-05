# Tool Format Guide

This guide explains how to use the `pkg/toolformat` package for unified tool calling across different LLM backends.

## Overview

The toolformat package provides:
- **Model-agnostic tool definitions** with JSON Schema parameters
- **Model-specific formatters** for generating prompts and parsing responses
- **Unified registry** for tool management
- **Migration bridge** for gradual adoption

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Application                          │
│                  (HTTP Server / CLI)                         │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      ToolBridge                              │
│         (DirectBridge / HybridBridge / MCPAdapter)          │
└─────────────────────────┬───────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  Registry   │   │  Executor   │   │  Formatter  │
│   (tools)   │   │   (loop)    │   │   (model)   │
└─────────────┘   └─────────────┘   └─────────────┘
```

## Quick Start

### 1. Create a Registry with Tools

```go
import (
    "github.com/soypete/pedrocli/pkg/config"
    "github.com/soypete/pedrocli/pkg/toolformat"
)

// Load config
cfg, _ := config.LoadDefault()

// Create factory and registry
factory := toolformat.NewToolFactory(cfg, "/path/to/workdir")
registry, _ := factory.CreateRegistryForMode(toolformat.ModeCoding)
```

### 2. Get the Right Formatter for Your Model

```go
// Auto-detect from model name
formatter := toolformat.GetFormatterForModel("qwen2.5-coder:32b")

// Or explicitly choose
formatter := toolformat.NewQwenFormatter()
formatter := toolformat.NewLlama3Formatter()
formatter := toolformat.NewClaudeFormatter()
```

### 3. Generate Tool Prompts

```go
// Get tool definitions
tools := registry.GetToolsForMode(toolformat.ModeCoding)

// Generate prompt section for your model
toolsPrompt := formatter.FormatToolsPrompt(tools)

// Add to your system prompt
systemPrompt := basePrompt + "\n\n" + toolsPrompt
```

### 4. Parse Tool Calls from Responses

```go
// Parse model response
calls, err := formatter.ParseToolCalls(llmResponse)
if err != nil {
    // handle error
}

// Execute tools
for _, call := range calls {
    result, _ := registry.Execute(ctx, call.Name, call.Args)
    // Process result...
}
```

---

## Model-Specific Tool Call Formats

### Qwen 2.5 Format

Qwen uses XML-style `<tool_call>` tags:

**Prompt Format:**
```xml
<tools>
{"type": "function", "function": {"name": "file", "description": "...", "parameters": {...}}}
{"type": "function", "function": {"name": "search", "description": "...", "parameters": {...}}}
</tools>

For each function call return a json object with function name and arguments within <tool_call></tool_call> XML tags:
<tool_call>
{"name": "<function-name>", "arguments": <args-json-object>}
</tool_call>
```

**Model Output:**
```xml
I'll read the file first.
<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "main.go"}}
</tool_call>
```

**Result Format:**
```xml
<tool_response>
{"name": "file", "content": "package main\n\nfunc main() {...}"}
</tool_response>
```

---

### Llama 3.x Format

Llama 3 uses `<|python_tag|>` for tool calls:

**Prompt Format:**
```
Environment: ipython
Tools: file, search, git, bash

# Tool Definitions

## file
Read, write, and modify files...

Parameters:
  - action (string) [required]: The file operation to perform
  - path (string) [required]: The file path

# Tool Call Format

When you need to call a tool, use the following format:

<|python_tag|>
{"name": "tool_name", "parameters": {"param1": "value1"}}
```

**Model Output:**
```
<|python_tag|>
{"name": "file", "parameters": {"action": "read", "path": "main.go"}}
<|eom_id|>
```

**Result Format:**
```
<|start_header_id|>ipython<|end_header_id|>

Tool: file
Status: Success
Output:
package main

func main() {...}
```

---

### Mistral/Mixtral Format

Mistral uses `[TOOL_CALLS]` format:

**Prompt Format:**
```
[AVAILABLE_TOOLS]
{"type": "function", "function": {"name": "file", "description": "...", "parameters": {...}}}
{"type": "function", "function": {"name": "search", "description": "...", "parameters": {...}}}
[/AVAILABLE_TOOLS]

To call a tool, use the following format:
[TOOL_CALLS] [{"name": "tool_name", "arguments": {"param": "value"}}]
```

**Model Output:**
```
[TOOL_CALLS] [{"name": "file", "arguments": {"action": "read", "path": "main.go"}}]
```

**Result Format:**
```
[TOOL_RESULTS]
{"name": "file", "content": "package main\n\nfunc main() {...}"}
[/TOOL_RESULTS]
```

---

### Hermes/Nous Format

Hermes uses XML-style function tags:

**Prompt Format:**
```xml
You are a function calling AI model. You are provided with function signatures within <tools></tools> XML tags.

<tools>
<tool>
  <name>file</name>
  <description>Read, write, and modify files.</description>
  <parameters>
    <parameter name="action" type="string" required="true">
      <description>The file operation to perform</description>
      <enum>read, write, replace, append, delete</enum>
    </parameter>
    <parameter name="path" type="string" required="true">
      <description>The file path to operate on</description>
    </parameter>
  </parameters>
</tool>
</tools>

For each function call return a json object with function name and arguments within <tool_call></tool_call> XML tags:
<tool_call>
{"name": "<function-name>", "arguments": <args-dict>}
</tool_call>
```

**Model Output:**
```xml
<tool_call>
{"name": "file", "arguments": {"action": "read", "path": "main.go"}}
</tool_call>
```

**Result Format:**
```xml
<tool_response>
  <name>file</name>
  <status>success</status>
  <output>package main

func main() {...}</output>
</tool_response>
```

---

### Claude API Format

Claude uses native tool_use content blocks:

**API Request (tools parameter):**
```json
{
  "tools": [
    {
      "name": "file",
      "description": "Read, write, and modify files.",
      "input_schema": {
        "type": "object",
        "properties": {
          "action": {"type": "string", "enum": ["read", "write", "replace", "append", "delete"]},
          "path": {"type": "string", "description": "The file path"}
        },
        "required": ["action", "path"]
      }
    }
  ]
}
```

**Model Response:**
```json
{
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_01A09q90qw90lq917835lhl",
      "name": "file",
      "input": {"action": "read", "path": "main.go"}
    }
  ]
}
```

**Result Format (user message):**
```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_01A09q90qw90lq917835lhl",
      "content": "package main\n\nfunc main() {...}"
    }
  ]
}
```

---

### OpenAI-Compatible Format

For vLLM, llama.cpp server, and other OpenAI-compatible APIs:

**API Request (tools parameter):**
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "file",
        "description": "Read, write, and modify files.",
        "parameters": {
          "type": "object",
          "properties": {
            "action": {"type": "string", "enum": ["read", "write"]},
            "path": {"type": "string"}
          },
          "required": ["action", "path"]
        }
      }
    }
  ]
}
```

**Model Response:**
```json
{
  "choices": [{
    "message": {
      "tool_calls": [
        {
          "id": "call_abc123",
          "type": "function",
          "function": {
            "name": "file",
            "arguments": "{\"action\": \"read\", \"path\": \"main.go\"}"
          }
        }
      ]
    }
  }]
}
```

**Result Format:**
```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "package main\n\nfunc main() {...}"
}
```

---

### Generic JSON Format (Fallback)

For models without specific tool calling formats:

**Prompt Format:**
```
# Available Tools

## file
Read, write, and modify files.

Parameters:
- action (string) (required): The file operation to perform
  Allowed values: read, write, replace, append, delete
- path (string) (required): The file path to operate on

# Tool Call Format

To call a tool, output a JSON object:
```json
{"tool": "tool_name", "args": {"param1": "value1"}}
```

You can call multiple tools by outputting multiple JSON objects on separate lines.
```

**Model Output:**
```json
{"tool": "file", "args": {"action": "read", "path": "main.go"}}
```

**Result Format:**
```
Tool: file
Status: Success
Output:
package main

func main() {...}
```

---

## Tool Categories and Modes

Tools are organized into categories:

| Category | Tools | Use Case |
|----------|-------|----------|
| `CategoryCode` | file, code_edit, search, navigate, git, bash, test | Code manipulation |
| `CategoryResearch` | rss_feed, static_links, web_scrape | Research/web |
| `CategoryBlog` | blog_publish, notion, calendar | Content creation |
| `CategoryJob` | get_job_status, list_jobs, cancel_job | Job management |
| `CategoryAgent` | builder, debugger, reviewer, triager | Agent invocation |

Modes provide pre-defined tool sets:

```go
// Get tools for coding tasks
tools := registry.GetToolsForMode(toolformat.ModeCoding)

// Get tools for blog writing
tools := registry.GetToolsForMode(toolformat.ModeBlog)

// Get all tools
tools := registry.GetToolsForMode(toolformat.ModeAll)
```

---

## Migration Guide

### Option 1: Direct Migration

Replace MCP client entirely with toolformat:

```go
// Before (MCP)
result, _ := mcpClient.CallTool(ctx, "file", args)
output := result.Content[0].Text

// After (toolformat)
bridge := toolformat.NewDirectBridge(registry, formatter)
result, _ := bridge.CallTool(ctx, "file", args)
output := result.Output
```

### Option 2: Hybrid Migration

Use toolformat for some tools, MCP for others:

```go
// Create hybrid bridge
bridge := toolformat.NewHybridBridge(
    registry,
    []string{"file", "search", "git"},  // Direct execution
    mcpFallback,                          // MCP for others
)

// All calls go through same interface
result, _ := bridge.CallTool(ctx, "file", args)      // Direct
result, _ := bridge.CallTool(ctx, "builder", args)   // MCP fallback
```

### Option 3: MCP Adapter

Wrap existing MCP client with ToolBridge interface:

```go
adapter := &toolformat.MCPClientAdapter{
    MCPCaller: func(ctx context.Context, name string, args map[string]interface{}) (string, bool, error) {
        result, err := mcpClient.CallTool(ctx, name, args)
        if err != nil {
            return "", true, err
        }
        return result.Content[0].Text, result.IsError, nil
    },
    MCPHealthy: func() bool {
        return mcpClient.IsRunning()
    },
}
```

---

## Adding Custom Tools

### Define Tool Schema

```go
func MyToolSchema() toolformat.ParameterSchema {
    schema := toolformat.NewParameterSchema()

    schema.AddProperty("action", toolformat.StringEnumProperty(
        "The action to perform",
        "action1", "action2",
    ), true)

    schema.AddProperty("value", toolformat.StringProperty(
        "Some value",
    ), false)

    return schema
}
```

### Register Tool

```go
registry.Register(&toolformat.ToolDefinition{
    Name:        "my_tool",
    Description: "Does something useful",
    Category:    toolformat.CategoryCode,
    Parameters:  MyToolSchema(),
    Handler: func(args map[string]interface{}) (*toolformat.ToolResult, error) {
        action := args["action"].(string)
        // ... implement tool logic
        return &toolformat.ToolResult{
            Success: true,
            Output:  "Result here",
        }, nil
    },
})
```

---

## Best Practices

1. **Use the right formatter**: Match the formatter to your model family for best results
2. **Auto-detect when possible**: Use `GetFormatterForModel(modelName)` for automatic detection
3. **Handle parsing failures**: Formatters fall back to generic parsing if model-specific format fails
4. **Truncate long outputs**: Tool results should be truncated to fit context windows
5. **Validate tool calls**: Check that required parameters are present before execution

---

## Supported Models

| Model | Formatter | Detection Pattern |
|-------|-----------|-------------------|
| Qwen 2.5 | `QwenFormatter` | `qwen`, `qwen2` |
| Llama 3.x | `Llama3Formatter` | `llama3`, `llama-3` |
| Mistral/Mixtral | `MistralFormatter` | `mistral`, `mixtral` |
| Hermes/Nous | `HermesFormatter` | `hermes`, `nous` |
| Claude | `ClaudeFormatter` | `claude` |
| GPT-4/3.5 | `OpenAIFormatter` | `gpt-4`, `gpt-3.5`, `openai` |
| Other | `GenericFormatter` | (fallback) |

---

## Execution Modes

PedroCLI supports two execution modes:

### 1. Direct Mode (Default)

Tools and agents run directly in the CLI process using goroutines. This provides:
- Faster startup (no subprocess spawn)
- Single binary operation
- Better resource sharing
- Simpler deployment

This is the default mode. No configuration needed.

### 2. MCP Subprocess Mode (Deprecated)

The CLI spawns `pedrocli-server` as a subprocess and communicates via JSON-RPC over stdio. This mode is deprecated but still available for:
- Backward compatibility during migration
- Debugging tool isolation issues

To use MCP subprocess mode (not recommended):
```json
{
  "execution": {
    "direct_mode": false
  }
}
```

> **Note**: MCP subprocess mode will be removed in a future release. Please migrate to direct mode.

### Third-Party MCP Servers

You can configure additional MCP servers to connect to:

```json
{
  "execution": {
    "direct_mode": true,
    "mcp_servers": [
      {
        "name": "calendar",
        "command": "/path/to/calendar-mcp-server",
        "args": ["--port", "9000"],
        "env": ["GOOGLE_CREDENTIALS=/path/to/creds.json"]
      }
    ]
  }
}
```

When in direct mode, built-in tools execute in-process while third-party MCP servers are called via the HybridBridge
