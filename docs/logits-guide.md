# Logit Manipulation for Reliable Structured Outputs

## Why PedroCLI Needs Logit Control

**Self-hosted models lack enterprise safety guardrails.** Unlike API-based services (OpenAI, Anthropic), models running via llama.cpp or Ollama don't have:

- Built-in output format enforcement
- Enterprise safety fine-tuning
- Guaranteed JSON/tool call formatting

**Agent retry loops are wasteful and unreliable.** Without logit control:
- Tool calls may have malformed JSON
- Structured outputs require post-hoc validation
- Failed generations waste compute

**Grammar constraints beat prompt engineering.** Prompts are persuasion; logits are enforcement.

| Approach | Reliability | Compute Cost |
|----------|-------------|--------------|
| Prompt engineering | Low | Low |
| Fine-tuning | Medium | High (training) |
| Logit manipulation | High | Low |

---

## What Logits Are

Logits are the raw, unnormalized scores a language model produces for every possible next token.

```
prompt/context
→ model forward pass
→ logits (one score per token in vocab)
→ softmax
→ probabilities
→ sampling
→ chosen token
```

**Key properties:**
- Logits are real numbers (not probabilities)
- Only relative differences matter
- Small logit changes can produce large probability shifts
- Setting a logit to -∞ bans that token

---

## PedroCLI Execution Model

### One-Shot Subprocess Execution

PedroCLI uses **one-shot subprocess execution** with llama.cpp, NOT a persistent HTTP server:

```
PedroCLI (Go)
    │
    ├─→ Builds prompt from context files
    │
    ├─→ Spawns llama.cpp subprocess with CLI flags
    │       llama-cli -m model.gguf -p "prompt" --grammar-file tool.gbnf
    │
    ├─→ Reads stdout for response
    │
    └─→ Parses tool calls, updates context files
```

**Why one-shot?**
- Simple: No server management, no connection pooling
- Stateless: Each inference is independent
- Context management: PedroCLI handles context via files, not KV cache

### Package Structure

```
pkg/llm/llamacpp.go     # LlamaCppClient - subprocess execution with grammar support
pkg/logits/
├── grammar.go          # GBNF grammar parser
├── schema.go           # JSON Schema → GBNF converter
├── sampler.go          # Sampler configuration and presets
├── filter.go           # LogitFilter interface (for future use)
├── chain.go            # FilterChain for composing filters
├── safety.go           # Safety categories
├── toolcall.go         # Tool call schemas
├── tokenizer.go        # Vocabulary access interface
├── backend.go          # LlamaHTTPBackend (alternative, optional)
└── testing.go          # Test harness for configurations
```

### How Grammar Constraints Work

Grammar constraints are passed to llama.cpp via CLI flags:

```go
// llama.cpp command with grammar
args := []string{
    "-m", l.modelPath,
    "-p", fullPrompt,
    "--grammar-file", "/tmp/tool-call.gbnf",  // Grammar enforced at logit level
    "--temp", "0.0",                           // Deterministic
    "--top-k", "1",
}
cmd := exec.CommandContext(ctx, l.llamacppPath, args...)
```

The grammar is enforced **inside llama.cpp** at the logit level - invalid tokens get -∞ logits before sampling.

---

## Ollama vs llama.cpp

**Important:** Logit control is designed for **llama.cpp backend only**.

| Feature | Ollama | llama.cpp |
|---------|--------|-----------|
| Logit access | No | Yes (via CLI flags) |
| Grammar support | No | Yes (GBNF via `--grammar-file`) |
| Sampling control | Limited | Full (`--temp`, `--top-k`, etc.) |
| Execution model | HTTP API | Subprocess (one-shot) |

When using Ollama, rely on:
- Prompt engineering
- Post-hoc validation
- Retry loops

When using llama.cpp, use:
- GBNF grammars for structure (`--grammar-file`)
- Sampling parameters for control (`--temp`, `--top-k`, `--top-p`)
- Repetition penalties (`--repeat-penalty`)

---

## GBNF Grammar Constraints

GBNF (GGML BNF) is the grammar format used by llama.cpp.

### Built-in Grammars

```go
// JSON object grammar
const JSONObjectGrammar = `
root   ::= object
object ::= "{" ws (string ":" ws value ("," ws string ":" ws value)*)? "}"
array  ::= "[" ws (value ("," ws value)*)? "]"
value  ::= object | array | string | number | "true" | "false" | "null"
string ::= "\"" ([^"\\] | "\\" ["\\/bfnrt])* "\""
number ::= "-"? ([0-9] | [1-9] [0-9]*) ("." [0-9]+)?
ws     ::= [ \t\n\r]*
`

// Tool call grammar
const ToolCallGrammar = `
root   ::= "{" ws "\"name\"" ws ":" ws string ws "," ws "\"args\"" ws ":" ws object ws "}"
...
`
```

### Using Grammar Filters

```go
import "github.com/soypete/pedrocli/pkg/logits"

// Parse and create filter
grammar, _ := logits.ParseGBNF(logits.JSONObjectGrammar)
filter := logits.NewGrammarFilter(grammar, tokenizer)

// Apply during generation
chain := logits.NewFilterChain()
chain.Add(filter)
```

---

## JSON Schema Enforcement

Convert JSON Schema directly to GBNF for type-safe outputs.

```go
// Define expected output schema
schema := &logits.JSONSchema{
    Type: "object",
    Properties: map[string]*logits.JSONSchema{
        "name": {Type: "string"},
        "args": {
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "path": {Type: "string"},
            },
            Required: []string{"path"},
        },
    },
    Required: []string{"name", "args"},
}

// Convert to grammar
grammarStr, _ := logits.SchemaToGBNF(schema)

// Create filter
filter, _ := logits.NewJSONSchemaFilter(schema, tokenizer)
```

### Tool Call Schemas

```go
// Create schema for specific tool
toolSchema := logits.ToolCallSchema("read_file", &logits.JSONSchema{
    Type: "object",
    Properties: map[string]*logits.JSONSchema{
        "path": {Type: "string"},
    },
    Required: []string{"path"},
})

// Multi-tool support
multiSchema := logits.MultiToolCallSchema(map[string]*logits.JSONSchema{
    "read_file":  readFileParams,
    "write_file": writeFileParams,
    "search":     searchParams,
})
```

---

## Safety Filters

Block unsafe content at the token level. Categories are modular and can be enabled/disabled independently.

### Safety Categories

| Category | Blocks |
|----------|--------|
| `CategoryCodeInjection` | Shell injection patterns (`rm -rf`, `sudo`, etc.) |
| `CategoryCredentials` | API keys, tokens, passwords |
| `CategoryPII` | SSN patterns, credit card numbers |
| `CategoryProfanity` | Vulgar language (word list required) |
| `CategoryViolence` | Violent content (word list required) |
| `CategoryDangerous` | Dangerous instructions |

### Using Safety Filters

```go
// Create filter with tokenizer
safety := logits.NewSafetyFilter(tokenizer)

// Enable specific categories
safety.EnableCategory(logits.CategoryCodeInjection)
safety.EnableCategory(logits.CategoryCredentials)

// Or use a preset
safety.ApplyPreset("standard")  // code_injection + credentials
safety.ApplyPreset("strict")    // + PII, violence, dangerous
safety.ApplyPreset("maximum")   // all categories

// Add custom patterns
safety.AddCustomBannedPattern("rm -rf /")
```

### Safety Presets

| Preset | Categories |
|--------|------------|
| `minimal` | Code injection only |
| `standard` | Code injection + credentials |
| `strict` | + PII, violence, dangerous |
| `maximum` | All categories |

---

## Sampler Configuration

Control the sampling process with full parameter access.

### Configuration Options

```go
type SamplerConfig struct {
    Temperature       float32  // Randomness (0 = deterministic)
    TopK              int      // Consider top K tokens
    TopP              float32  // Nucleus sampling threshold
    MinP              float32  // Minimum probability threshold
    RepetitionPenalty float32  // Penalize repeated tokens
    RepetitionWindow  int      // How far back to check
    Mirostat          int      // Mirostat sampling (0, 1, or 2)
    MirostatTau       float32  // Target entropy
    MirostatEta       float32  // Learning rate
    LogitBias         map[int]float32  // Per-token biases
    MaxTokens         int      // Generation limit
    StopSequences     []string // Stop on these strings
}
```

### Built-in Presets

```go
// For tool calls and structured output
logits.DeterministicConfig  // temp=0, top_k=1

// For JSON generation
logits.StructuredOutputConfig  // temp=0.1, top_k=40

// For code generation
logits.CodeGenerationConfig  // temp=0.2, repetition_penalty=1.1

// For creative text
logits.CreativeConfig  // temp=0.8, top_k=100

// For chat
logits.ChatConfig  // temp=0.7, balanced settings
```

### Generation Presets

```go
// Get a preset
preset := logits.GetPreset("tool_call")

// Use in generation
req := &logits.GenerateRequest{
    Prompt:        "Call the read_file tool for /etc/hosts",
    SamplerConfig: preset.Config,
    Grammar:       preset.Grammar,
}

// Or build custom presets
custom := logits.NewPresetBuilder("my_preset").
    Description("Custom JSON generation").
    Temperature(0.2).
    TopK(50).
    Grammar(logits.JSONObjectGrammar).
    SafetyPreset("standard").
    BuildAndRegister()
```

---

## Using LlamaCppClient with Grammar

The primary approach uses `LlamaCppClient` with CLI-based grammar enforcement.

### Basic Usage

```go
import "github.com/soypete/pedrocli/pkg/llm"

// Create client from config
client := llm.NewLlamaCppClient(cfg)

// Set grammar for structured output
client.SetGrammar(logits.ToolCallGrammar)

// Configure for deterministic tool calls
client.ConfigureForToolCalls()

// Run inference
resp, err := client.Infer(ctx, &llm.InferenceRequest{
    SystemPrompt: "You are a helpful assistant.",
    UserPrompt:   "Read the file /etc/hosts",
    MaxTokens:    256,
    Temperature:  0.0,
})
```

### Grammar Methods

```go
// Set inline GBNF grammar (written to temp file)
client.SetGrammar(`
root ::= "{" ws "\"name\"" ws ":" ws string ws "}"
string ::= "\"" [^"]* "\""
ws ::= [ \t\n]*
`)

// Or use a grammar file directly
client.SetGrammarFile("/path/to/grammar.gbnf")

// Clear grammar for unconstrained generation
client.ClearGrammar()
```

### Sampling Control

```go
// Set individual parameters
client.SetTopK(40)
client.SetTopP(0.9)
client.SetMinP(0.05)
client.SetRepeatPenalty(1.1)
client.SetRepeatLastN(64)

// Or use convenience methods
client.ConfigureForStructuredOutput()  // Low temp, tight sampling
client.ConfigureForToolCalls()         // Deterministic (temp=0, top_k=1)
```

### Alternative: LlamaHTTPBackend

For llama-server (HTTP API) instead of CLI, use `LlamaHTTPBackend`:

```go
import "github.com/soypete/pedrocli/pkg/logits"

backend := logits.NewLlamaHTTPBackend("http://localhost:8080")
resp, _ := backend.GenerateWithPreset(ctx, prompt, "tool_call")
```

This is optional and not the primary execution model.

---

## MCP Tool Integration

The `LogitTool` provides MCP-compatible operations.

### Available Actions

| Action | Description |
|--------|-------------|
| `generate` | Generate with logit control |
| `generate_structured` | Generate JSON matching schema |
| `generate_tool_call` | Generate tool call with format guarantee |
| `test_config` | Test a configuration |
| `list_presets` | List available presets |
| `analyze_vocabulary` | Analyze tokenizer vocabulary |

### Example Tool Calls

```json
{
  "tool": "logit",
  "args": {
    "action": "generate_structured",
    "prompt": "Generate user info",
    "json_schema": {
      "type": "object",
      "properties": {
        "name": {"type": "string"},
        "email": {"type": "string"}
      },
      "required": ["name", "email"]
    }
  }
}
```

---

## Testing Configurations

Use the test harness to validate logit configurations.

```go
// Create test harness
harness := logits.NewLogitTestHarness(backend)

// Add test cases
harness.AddTestCase(&logits.LogitTestCase{
    Name:           "json_output",
    Prompt:         "Generate a user object",
    PresetName:     "json_strict",
    ExpectedJSON:   true,
    ExpectedFormat: `^\s*\{.*\}\s*$`,
    Iterations:     10,
})

// Run tests
results := harness.RunTests(ctx)

// Print summary
fmt.Println(harness.PrintResults())
```

### Standard Test Cases

The package includes standard test cases:
- `json_object_basic` - Simple JSON generation
- `tool_call_format` - Tool call structure
- `deterministic_output` - Consistent output
- `code_safety` - Injection prevention

---

## Database Schema

Logit configurations and logs are persisted in SQLite (Migration V3).

### Tables

| Table | Purpose |
|-------|---------|
| `logit_presets` | Custom preset configurations |
| `logit_test_runs` | Test run history |
| `generation_logs` | Individual generation tracking |
| `vocab_cache` | Tokenizer vocabulary cache |

---

## Common Patterns

### Pattern 1: Guaranteed Tool Calls

```go
// Ensure tool calls are always valid JSON
chain := logits.NewFilterChain()
chain.Add(logits.NewGrammarFilter(toolGrammar, tokenizer))
chain.Add(logits.NewSafetyFilter(tokenizer))

req := &logits.GenerateRequest{
    Prompt:        prompt,
    SamplerConfig: logits.DeterministicConfig,
    Filters:       chain,
}
```

### Pattern 2: Safe Code Generation

```go
safety := logits.NewSafetyFilter(tokenizer)
safety.ApplyPreset("minimal")  // Just code injection

req := &logits.GenerateRequest{
    Prompt:        "Write a bash script to...",
    SamplerConfig: logits.CodeGenerationConfig,
    Filters:       logits.NewFilterChainWithFilters(safety),
}
```

### Pattern 3: Deterministic JSON

```go
req := &logits.GenerateRequest{
    Prompt:        prompt,
    SamplerConfig: logits.DeterministicConfig,
    Grammar:       grammarFromSchema,
}

// Output is guaranteed to be valid JSON matching schema
```

---

## When NOT to Use Logit Control

- **Ollama backend**: Ollama handles sampling internally
- **Creative generation**: Heavy constraints reduce quality
- **Long-form text**: Grammar constraints slow generation
- **Unknown schemas**: Dynamic schemas need runtime compilation

For these cases, use:
- Post-hoc validation
- Retry loops with exponential backoff
- Hybrid approaches (constrain structure, not content)

---

## Further Reading

- [llama.cpp Grammar Documentation](https://github.com/ggerganov/llama.cpp/blob/master/grammars/README.md)
- [GBNF Grammar Specification](https://github.com/ggerganov/llama.cpp/blob/master/grammars/grammar.gbnf)
- [JSON Schema Specification](https://json-schema.org/)
- [Hugging Face Logits Processors](https://huggingface.co/docs/transformers/internal/generation_utils)
