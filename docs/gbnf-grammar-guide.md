# GBNF Grammar Guide for Tool Calling

## Overview

PedroCLI uses GBNF (GGML BNF) grammars with llama.cpp to constrain model output at the logit level, ensuring valid tool calls. This dramatically improves accuracy for smaller models (7B-13B) by preventing hallucinated tool names or malformed JSON.

## What is GBNF?

GBNF is llama.cpp's implementation of Backus-Naur Form (BNF) grammars. It allows you to define the exact structure of valid output, and llama.cpp enforces this by masking invalid tokens during generation.

**Benefits:**
- ✅ Prevents hallucinated tool names
- ✅ Enforces valid JSON structure
- ✅ Ensures enum values match exactly
- ✅ Eliminates parsing failures
- ✅ Makes smaller models (7B) viable for tool calling

## How PedroCLI Uses GBNF

### Automatic Grammar Generation

PedroCLI automatically generates GBNF grammars from tool schemas:

1. **Tool Registry** (`pkg/tools/registry.go`):
   - Centralizes all tool definitions
   - Each tool provides metadata via `Metadata()` method
   - Schemas define parameter types, enums, required fields

2. **Grammar Generation** (`pkg/logits/schema.go`):
   - Converts JSON schemas to GBNF rules
   - Creates `oneOf` rules for tool selection
   - Generates nested schemas for complex parameters

3. **Application** (`pkg/agents/base.go`):
   - Grammar applied before each inference
   - llama.cpp enforces at logit level
   - Transparent to the model - it just can't generate invalid tokens

### Grammar Application Flow

```
Agent.executeInference()
  ├─> Check if registry exists
  ├─> Type assert to LlamaCppClient
  ├─> Generate grammar: registry.GenerateToolCallGrammar()
  ├─> Apply grammar: llamacppBackend.SetGrammar(grammar.String())
  ├─> Configure deterministic mode: ConfigureForToolCalls()
  └─> Perform inference with grammar enforced
```

## Tool Call Format

### Expected JSON Structure

```json
{
  "tool": "tool_name",
  "args": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

### Example Tool Calls

**Navigate tool:**
```json
{
  "tool": "navigate",
  "args": {
    "action": "list_directory",
    "directory": "pkg"
  }
}
```

**Code edit tool:**
```json
{
  "tool": "code_edit",
  "args": {
    "file": "main.go",
    "operation": "edit",
    "line_number": 42,
    "new_content": "func main() {"
  }
}
```

**Search tool:**
```json
{
  "tool": "search",
  "args": {
    "action": "grep",
    "pattern": "func.*Execute",
    "path": "pkg/agents"
  }
}
```

## GBNF Grammar Structure

### Basic Grammar Rules

GBNF uses these core constructs:

```gbnf
# Rule definition
rulename ::= <definition>

# Literals (strings)
greeting ::= "hello" | "hi"

# Character classes
digit ::= [0-9]
letter ::= [a-zA-Z]

# Sequences
name ::= letter+

# Optional elements
optional ::= "required" (" " "optional")?

# Repetition
list ::= item ("," item)*

# Alternation (choice)
choice ::= "option1" | "option2" | "option3"
```

### Tool Call Grammar Example

Here's a simplified grammar for a single tool:

```gbnf
root ::= ws "{" ws "\"tool\"" ws ":" ws tool-name ws "," ws "\"args\"" ws ":" ws args ws "}"

tool-name ::= "\"navigate\""

args ::= "{" ws "\"action\"" ws ":" ws action ws "," ws "\"directory\"" ws ":" ws directory ws "}"

action ::= "\"list_directory\"" | "\"get_file_outline\"" | "\"find_imports\""

directory ::= "\"" [^"]* "\""

ws ::= [ \t\n]*
```

**Key Points:**
- `root` defines the top-level structure
- `tool-name` is a literal (only "navigate" allowed)
- `action` uses alternation for enum values
- `directory` allows any string
- `ws` handles whitespace flexibly

### Multi-Tool Grammar

For multiple tools, use `oneOf`:

```gbnf
root ::= tool-call

tool-call ::= navigate-call | search-call | file-call

navigate-call ::= "{" ws "\"tool\"" ws ":" ws "\"navigate\"" ws "," ws "\"args\"" ws ":" ws navigate-args ws "}"

search-call ::= "{" ws "\"tool\"" ws ":" ws "\"search\"" ws "," ws "\"args\"" ws ":" ws search-args ws "}"

file-call ::= "{" ws "\"tool\"" ws ":" ws "\"file\"" ws "," ws "\"args\"" ws ":" ws file-args ws "}"

# ...args definitions for each tool...
```

## Implementing Tool Schemas

### Step 1: Define Tool Metadata

In your tool implementation:

```go
// Metadata returns JSON schema for the navigate tool
func (t *NavigateTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Name:        "navigate",
        Description: "Navigate code structure and list directories",
        Schema:      t.getSchema(),
    }
}

func (t *NavigateTool) getSchema() *logits.JSONSchema {
    return &logits.JSONSchema{
        Type: "object",
        Properties: map[string]*logits.JSONSchema{
            "action": {
                Type:        "string",
                Enum:        []interface{}{"list_directory", "get_file_outline", "find_imports"},
                Description: "Navigation operation",
            },
            "directory": {
                Type:        "string",
                Description: "Directory path",
            },
            "file": {
                Type:        "string",
                Description: "File path for outline/imports",
            },
        },
        Required: []string{"action"},
    }
}
```

### Step 2: Register the Tool

```go
registry := tools.NewToolRegistry()
registry.Register(navigateTool)
registry.Register(searchTool)
registry.Register(fileTool)
// ...
```

### Step 3: Generate Grammar

```go
grammar, err := registry.GenerateToolCallGrammar()
if err != nil {
    return err
}

// Grammar is automatically a *logits.GBNF with String() method
grammarString := grammar.String()
```

### Step 4: Apply to Backend

```go
if llamacppBackend, ok := backend.(*llm.LlamaCppClient); ok {
    llamacppBackend.SetGrammar(grammarString)
    llamacppBackend.ConfigureForToolCalls()
}
```

## Schema Types

### Supported JSON Schema Types

| JSON Schema Type | GBNF Equivalent | Example |
|------------------|-----------------|---------|
| `string` | `"\"" [^"]* "\""` | Any quoted string |
| `string` (enum) | `"val1" \| "val2"` | Fixed choices |
| `integer` | `"-"? [0-9]+` | Signed integers |
| `number` | `"-"? [0-9]+ ("." [0-9]+)?` | Floats |
| `boolean` | `"true" \| "false"` | Boolean literals |
| `object` | Nested rules | Structured objects |
| `array` | `"[" items "]"` | Lists |
| `null` | `"null"` | Null value |

### Enum Handling

Enums are critical for tool calling. They become alternations in GBNF:

**Schema:**
```go
{
    Type: "string",
    Enum: []interface{}{"list_directory", "get_file_outline", "find_imports"},
}
```

**Generated GBNF:**
```gbnf
action ::= "\"list_directory\"" | "\"get_file_outline\"" | "\"find_imports\""
```

**Why this matters:** The model CANNOT generate `"list_directories"` (plural) or any other variant. It must choose from the exact enum values.

## Grammar Generation Implementation

### Core Functions (`pkg/logits/schema.go`)

**1. SchemaToGBNF** - Converts JSON Schema to GBNF:
```go
func SchemaToGBNF(schema *JSONSchema) (string, error)
```

**2. ToolCallSchema** - Creates schema for single tool call:
```go
func ToolCallSchema(toolName string, argsSchema *JSONSchema) *JSONSchema
```

**3. MultiToolCallSchema** - Creates schema allowing any registered tool:
```go
func MultiToolCallSchema(tools map[string]*JSONSchema) *JSONSchema
```

### Example Generated Grammar

For tools `navigate`, `search`, `file`:

```gbnf
root ::= ws tool-call ws

tool-call ::= "{" ws
  "\"tool\"" ws ":" ws tool-name ws "," ws
  "\"args\"" ws ":" ws tool-args ws
"}"

tool-name ::=
  "\"navigate\"" |
  "\"search\"" |
  "\"file\""

tool-args ::= navigate-args | search-args | file-args

navigate-args ::= "{" ws
  "\"action\"" ws ":" ws navigate-action ws
  ("," ws "\"directory\"" ws ":" ws string-value ws)?
  ("," ws "\"file\"" ws ":" ws string-value ws)?
"}"

navigate-action ::=
  "\"list_directory\"" |
  "\"get_file_outline\"" |
  "\"find_imports\"" |
  "\"get_tree\""

search-args ::= "{" ws
  "\"action\"" ws ":" ws search-action ws
  ("," ws "\"pattern\"" ws ":" ws string-value ws)?
  ("," ws "\"path\"" ws ":" ws string-value ws)?
"}"

search-action ::=
  "\"grep\"" |
  "\"find_files\"" |
  "\"find_definition\""

file-args ::= "{" ws
  "\"action\"" ws ":" ws file-action ws
  "," ws "\"file\"" ws ":" ws string-value ws
  ("," ws "\"content\"" ws ":" ws string-value ws)?
"}"

file-action ::=
  "\"read\"" |
  "\"write\"" |
  "\"append\""

string-value ::= "\"" [^"]* "\""

ws ::= [ \t\n\r]*
```

## Testing Grammars

### Manual Grammar Testing

You can test grammars directly with llama-cli:

```bash
# Create grammar file
cat > test-grammar.gbnf <<'EOF'
root ::= ws "{" ws "\"tool\"" ws ":" ws tool-name ws "," ws "\"args\"" ws ":" ws "{" ws "}" ws "}"
tool-name ::= "\"navigate\"" | "\"search\"" | "\"file\""
ws ::= [ \t\n\r]*
EOF

# Test with llama.cpp
llama-cli \
  --model model.gguf \
  --grammar-file test-grammar.gbnf \
  --prompt "Generate a tool call:" \
  --n-predict 100
```

### Validating Generated Grammars

```go
// Generate grammar
grammar, err := registry.GenerateToolCallGrammar()
if err != nil {
    t.Fatalf("Grammar generation failed: %v", err)
}

// Save for inspection
os.WriteFile("generated-grammar.gbnf", []byte(grammar.String()), 0644)

// Parse back (validates syntax)
parsed, err := logits.ParseGBNF(grammar.String())
if err != nil {
    t.Fatalf("Generated grammar is invalid: %v", err)
}
```

## Common Issues and Debugging

### Issue: Grammar Too Restrictive

**Symptom:** Model output truncated early, only generates 5-10 tokens

**Cause:** Grammar doesn't allow the model to complete valid JSON

**Debug:**
1. Save grammar to file: `echo "$GRAMMAR" > debug.gbnf`
2. Test with simple prompt
3. Check for missing alternations or overly specific rules

**Fix:** Ensure grammar has rules for:
- Whitespace flexibility (`ws ::= [ \t\n\r]*`)
- Optional parameters (`param?`)
- String values (`[^"]*` for any string content)

### Issue: Model Generates Invalid Tool Names

**Symptom:** "unknown tool: navigat" or "tool not found: search_code"

**Cause:** Grammar not being applied or tool name not in enum

**Debug:**
1. Check stderr for `[DEBUG] Grammar applied` message
2. Verify tool is registered: `registry.List()`
3. Check grammar file: `/tmp/pedrocli-grammar.gbnf`

**Fix:**
- Ensure tool is registered before grammar generation
- Verify type assertion succeeds (llama.cpp backend, not Ollama)
- Check tool name spelling matches exactly

### Issue: Grammar File Not Found

**Symptom:** llama-cli error: "failed to load grammar file"

**Cause:** Temp file cleanup or permission issues

**Debug:**
1. Check if `/tmp/pedrocli-grammar.gbnf` exists during inference
2. Verify file permissions (should be 0600)

**Fix:**
- Grammar is written to temp file during `Infer()` call
- File is cleaned up after, which is expected
- If you need to inspect, copy during execution or modify cleanup logic

### Issue: No Tool Calls Generated

**Symptom:** Model outputs text explanation instead of JSON tool calls

**Cause:** Grammar not applied OR model doesn't understand task

**Debug:**
1. Check for `[DEBUG] Grammar applied` in logs
2. Verify backend type is llamacpp, not ollama
3. Check system prompt includes tool usage instructions

**Fix:**
- Grammars only work with llama.cpp, not Ollama
- Ensure system prompt explains tools and expected format
- Consider using a larger/better model if grammar is correctly applied

## Performance Considerations

### Grammar Size

- Large grammars (many tools, complex schemas) increase parse time
- Each token generation requires grammar validation
- Keep schemas minimal - only include used parameters

### Memory Usage

- llama.cpp loads grammar into memory
- Complex grammars with many alternations use more RAM
- ~1-5MB typical for PedroCLI's 7-tool grammar

### Generation Speed

- Grammar-constrained generation is slightly slower
- Overhead is minimal (< 10%) for well-designed grammars
- Benefit of avoiding re-tries far outweighs cost

## Best Practices

### 1. Define Minimal Schemas

Only include parameters actually used:

```go
// ❌ Bad: Includes unused parameters
{
    Properties: map[string]*logits.JSONSchema{
        "action": {...},
        "path": {...},
        "recursive": {...},      // Never used
        "follow_symlinks": {...}, // Never used
        "max_depth": {...},      // Never used
    },
}

// ✅ Good: Only what's needed
{
    Properties: map[string]*logits.JSONSchema{
        "action": {...},
        "path": {...},
    },
    Required: []string{"action"},
}
```

### 2. Use Enums for Fixed Values

```go
// ✅ Good: Enum for known operations
{
    Type: "string",
    Enum: []interface{}{"read", "write", "append"},
}

// ❌ Bad: Free-form string (model can hallucinate)
{
    Type: "string",
    Description: "Operation: read, write, or append",
}
```

### 3. Mark Required Fields

```go
{
    Properties: map[string]*logits.JSONSchema{
        "action": {...},
        "path": {...},
    },
    Required: []string{"action"}, // action must be present
}
```

### 4. Test with Smaller Models

If your grammar works well with 7B models, it will work great with larger models. Test early with small models to catch schema issues.

### 5. Validate Generated Grammars

```bash
# Generate and inspect
go test -v ./pkg/tools -run TestToolRegistry
cat /tmp/generated-grammar.gbnf

# Look for:
# - All tool names present
# - All enum values spelled correctly
# - Whitespace handling (ws rules)
# - Optional vs required parameters
```

## Advanced Topics

### Custom Grammar Rules

For special cases, you can define custom GBNF rules:

```go
customSchema := &logits.JSONSchema{
    Type: "string",
    // Use GBNF rule directly (advanced)
    CustomRule: `"\"" [a-zA-Z0-9_-]+ ".go" "\""`, // Must end in .go
}
```

### Grammar Caching

Grammars are generated on each inference. For performance, consider caching:

```go
type CachedGrammar struct {
    grammar    *logits.GBNF
    toolHashes map[string]string // tool name -> schema hash
}

func (c *CachedGrammar) GetOrGenerate(registry *tools.ToolRegistry) (*logits.GBNF, error) {
    if c.needsRegeneration(registry) {
        c.grammar, _ = registry.GenerateToolCallGrammar()
        c.updateHashes(registry)
    }
    return c.grammar, nil
}
```

### Hybrid Approaches

Combine grammar-constrained tool calling with free-form responses:

```go
// Option 1: Tool call mode
llamacppBackend.SetGrammar(toolGrammar)
response1, _ := llamacppBackend.Infer(ctx, toolPrompt)

// Option 2: Free-form response mode
llamacppBackend.ClearGrammar()
response2, _ := llamacppBackend.Infer(ctx, explanationPrompt)
```

## Resources

- [llama.cpp Grammar Documentation](https://github.com/ggerganov/llama.cpp/blob/master/grammars/README.md)
- [GBNF Examples](https://github.com/ggerganov/llama.cpp/tree/master/grammars)
- [PedroCLI Tool Registry](../pkg/tools/registry.go)
- [PedroCLI Schema Implementation](../pkg/logits/schema.go)

## See Also

- [Builder Agent Usage](./builder-agent-usage.md) - How to use the builder with grammar-enabled inference
- [Tool Documentation](../CLAUDE.md#package-structure) - Available tools and their schemas
- [Context Management](./pedroceli-context-guide.md) - Managing model context windows
