# GBNF Grammar Debug Plan - URGENT FIX NEEDED

## Current Status (2026-01-04)

### What's Working
- ✅ Grammar generation (2954 bytes)
- ✅ Type assertion to LlamaCppClient succeeds
- ✅ `SetGrammar()` called successfully
- ✅ `ConfigureForToolCalls()` applied (temp=0, topK=1)
- ✅ Debug logging confirms all steps execute

### What's Broken
- ❌ Model outputs malformed JSON: `{"args": {"action": "list_directory"`
- ❌ Missing `"tool"` field entirely - starts with `{"args":`
- ❌ Only generates ~10 tokens before `[end of text]`
- ❌ Tool call parser gets 0 parsed calls every round
- ❌ Test stuck in loop (round 6/30, no progress)

### Test Configuration
**Model**: Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf
**Config**: `.pedrocli.json` (llamacpp type)
**Job ID**: 23283e6f-1bba-41cc-b7c7-b02b360a7539
**Job Dir**: `/tmp/pedroceli-jobs/23283e6f-1bba-41cc-b7c7-b02b360a7539-20260104-101620/`

## Root Cause Analysis

### Hypothesis 1: Grammar Doesn't Match Expected Format
**Evidence**:
- Model outputs `{"args":` instead of `{"tool":`
- Grammar might be defining args-first structure
- Schema generation in `pkg/logits/schema.go` may be incorrect

**To Verify**:
```bash
# 1. Modify llamacpp.go to NOT delete grammar file
# 2. Run single inference
# 3. Inspect /tmp/pedrocli-grammar.gbnf
# 4. Check if it starts with:
#    root ::= "{" ws "\"tool\"" ws ":" ...
# OR:
#    root ::= "{" ws "\"args\"" ws ":" ...  (WRONG)
```

### Hypothesis 2: Grammar Is Applied But Wrong Format
**Evidence**:
- Debug logs show grammar applied
- Model still produces invalid output
- 10 token limit suggests grammar terminates early

**To Verify**:
```bash
# Test grammar directly with llama-cli
llama-cli \
  --model ~/.cache/huggingface/.../Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf \
  --grammar-file /tmp/test-grammar.gbnf \
  --prompt "Generate a tool call to list files in pkg directory" \
  --n-predict 100
```

### Hypothesis 3: Parser Expects Different Format
**Evidence**:
- Parser in `pkg/toolformat/generic.go` may expect different JSON
- Model might be following a different tool call convention

**To Verify**:
```bash
# Check what parser expects
grep -A 20 "func.*Parse" pkg/toolformat/generic.go
# Check against actual model output
```

## Step-by-Step Debug Plan

### Phase 1: Inspect Grammar (15 minutes)

**File**: `pkg/llm/llamacpp.go`

**Change**:
```go
// Line 124-130, modify writeGrammarToTempFile:
func (l *LlamaCppClient) writeGrammarToTempFile(grammar string) (string, error) {
    tmpDir := os.TempDir()
    tmpFile := filepath.Join(tmpDir, "pedrocli-grammar.gbnf")

    // DEBUG: Also write to a permanent location
    debugFile := "/tmp/pedrocli-grammar-debug.gbnf"
    os.WriteFile(debugFile, []byte(grammar), 0644)
    fmt.Fprintf(os.Stderr, "[DEBUG] Grammar written to: %s\n", debugFile)

    if err := os.WriteFile(tmpFile, []byte(grammar), 0600); err != nil {
        return "", err
    }
    return tmpFile, nil
}
```

**Run**:
```bash
make build-cli
./pedrocli build -description "test" -issue "1"
# Ctrl+C after first inference
cat /tmp/pedrocli-grammar-debug.gbnf
```

**Expected Output**:
```gbnf
root ::= ws "{" ws "\"tool\"" ws ":" ws tool-name ws "," ws "\"args\"" ws ":" ws tool-args ws "}"
tool-name ::= "\"navigate\"" | "\"search\"" | "\"file\"" | ...
```

**If Wrong** (args-first):
```gbnf
root ::= ws "{" ws "\"args\"" ws ":" ...  # BUG!
```

### Phase 2: Fix Grammar Generation (30 minutes)

**File**: `pkg/logits/schema.go`

**Functions to Check**:
1. `ToolCallSchema()` - Line ~456
2. `MultiToolCallSchema()` - Line ~472
3. `SchemaToGBNF()` - Line ~52

**Expected Structure**:
```go
func ToolCallSchema(toolName string, argsSchema *JSONSchema) *JSONSchema {
    return &JSONSchema{
        Type: "object",
        Properties: map[string]*JSONSchema{
            "tool": {  // Must come first!
                Type: "const",
                Const: toolName,
            },
            "args": argsSchema,
        },
        Required: []string{"tool", "args"},
        // CRITICAL: Ensure "tool" comes before "args" in GBNF output
    }
}
```

**Likely Bug**:
- `SchemaToGBNF()` may be iterating `Properties` map in random order
- Maps in Go are unordered!
- Need to sort keys or use explicit ordering

**Fix**:
```go
func SchemaToGBNF(schema *JSONSchema) (string, error) {
    // ... existing code ...

    // When generating object rules, ensure "tool" comes first
    keys := make([]string, 0, len(schema.Properties))
    for k := range schema.Properties {
        keys = append(keys, k)
    }

    // Sort to ensure consistent order: "args", "tool" alphabetically
    // OR better: hardcode order for tool calls
    if hasKey(keys, "tool") && hasKey(keys, "args") {
        keys = []string{"tool", "args"}  // Force this order
    } else {
        sort.Strings(keys)
    }

    // Then iterate over keys in order
    for _, key := range keys {
        prop := schema.Properties[key]
        // ... generate GBNF ...
    }
}
```

### Phase 3: Test Grammar Directly (15 minutes)

**Create Test Grammar**:
```bash
cat > /tmp/test-tool-grammar.gbnf <<'EOF'
root ::= ws "{" ws "\"tool\"" ws ":" ws tool-name ws "," ws "\"args\"" ws ":" ws "{" ws "}" ws "}"

tool-name ::= "\"navigate\""

ws ::= [ \t\n\r]*
EOF
```

**Test**:
```bash
llama-cli \
  --model ~/.cache/huggingface/hub/models--bartowski--Qwen2.5-Coder-7B-Instruct-GGUF/snapshots/*/Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf \
  --grammar-file /tmp/test-tool-grammar.gbnf \
  --prompt "Call the navigate tool" \
  --temp 0 \
  --top-k 1 \
  --n-predict 50
```

**Expected Output**:
```json
{"tool": "navigate", "args": {}}
```

**If Still Wrong**:
- Grammar syntax error
- llama.cpp version incompatibility
- Model doesn't understand grammar constraints

### Phase 4: Compare Against Working Example (10 minutes)

**Find Working Grammar**:
```bash
# llama.cpp repo has examples
git clone --depth 1 https://github.com/ggerganov/llama.cpp.git /tmp/llamacpp
cat /tmp/llamacpp/grammars/json.gbnf
cat /tmp/llamacpp/grammars/json_arr.gbnf
```

**Adapt to Tool Calls**:
```gbnf
# Based on json.gbnf
root ::= object

object ::= "{" ws members ws "}"

members ::= pair ("," ws pair)*

pair ::= string-key ws ":" ws value

string-key ::= "\"tool\"" | "\"args\""

value ::= string | object | array | "true" | "false" | "null"

string ::= "\"" [^"]* "\""

# ... etc
```

### Phase 5: Verify Parser Compatibility (10 minutes)

**Check Parser**:
```bash
grep -A 30 "func.*Parse" pkg/toolformat/generic.go
```

**Expected Format**:
```go
type ToolCall struct {
    Name string                 `json:"name"` // or "tool"?
    Args map[string]interface{} `json:"args"`
}
```

**Possible Mismatch**:
- Parser expects `"name"` but we generate `"tool"`
- Parser expects different structure

**Quick Test**:
```go
// Add to executor.go
fmt.Fprintf(os.Stderr, "[DEBUG] Raw LLM output: %s\n", response.Text)

// Then manually parse
var toolCall struct {
    Tool string                 `json:"tool"`
    Args map[string]interface{} `json:"args"`
}
err := json.Unmarshal([]byte(response.Text), &toolCall)
fmt.Fprintf(os.Stderr, "[DEBUG] Parse result: %+v, error: %v\n", toolCall, err)
```

## Quick Win Alternatives

### Option A: Disable Grammar Temporarily
```go
// pkg/agents/base.go line 223
if a.registry != nil {
    // TEMP DISABLE for debugging
    fmt.Fprintf(os.Stderr, "[DEBUG] Grammar generation disabled for debugging\n")
    return

    if llamacppBackend, ok := a.llm.(*llm.LlamaCppClient); ok {
        // ...
    }
}
```

**Test without grammar to confirm model can make tool calls**

### Option B: Use Ollama Instead
```bash
# Switch to Ollama for the test
mv .pedrocli.json .pedrocli-llamacpp.json
mv .pedrocli.json.backup .pedrocli.json

# Ollama doesn't support grammar but has better 30B model
./pedrocli build -description "test" -issue "32"
```

### Option C: Test with Larger Model
```bash
# Try 32B model which may not need grammar
# Update .pedrocli.json to use Qwen2.5-Coder-32B-Instruct
```

## Critical Files

### Must Inspect:
1. **pkg/logits/schema.go**
   - `SchemaToGBNF()` - Line 52
   - `ToolCallSchema()` - Line 456
   - `MultiToolCallSchema()` - Line 472
   - Property ordering in object generation

2. **pkg/llm/llamacpp.go**
   - `writeGrammarToTempFile()` - Line 124
   - `Infer()` - Line 54 (where grammar is applied)

3. **pkg/toolformat/generic.go**
   - Tool call parsing logic
   - Expected JSON structure

4. **pkg/agents/executor.go**
   - Where responses are parsed
   - Error handling for malformed output

### Test Files:
- `/tmp/pedroceli-jobs/23283e6f-1bba-41cc-b7c7-b02b360a7539-20260104-101620/002-response.txt`
- `/tmp/pedrocli-grammar-debug.gbnf` (after Phase 1)

## Success Criteria

### Minimal Success:
- ✅ Grammar generates with `"tool"` field first
- ✅ Model outputs `{"tool": "navigate", "args": {...}}`
- ✅ Parser successfully extracts tool call
- ✅ At least 1 tool call executes in test

### Full Success:
- ✅ All tools work with grammar
- ✅ No malformed outputs
- ✅ 7B model completes feature implementation
- ✅ PR created successfully

## Timeline Estimate

- **Phase 1 (Inspect)**: 15 minutes
- **Phase 2 (Fix)**: 30 minutes
- **Phase 3 (Test)**: 15 minutes
- **Phase 4 (Compare)**: 10 minutes
- **Phase 5 (Verify)**: 10 minutes
- **Total**: ~90 minutes

## Immediate Next Steps

1. Add debug output to save grammar file permanently
2. Rebuild and run single inference
3. Inspect `/tmp/pedrocli-grammar-debug.gbnf`
4. Identify if problem is grammar generation or application
5. Fix and retest

## Commands to Run After Compact

```bash
# 1. Stop current test
pkill -f "pedrocli build"

# 2. Add debug logging to save grammar
# (See Phase 1 changes above)

# 3. Rebuild
make build-cli

# 4. Run single inference test
./pedrocli build -description "List files in pkg" -issue "test" &
PID=$!
sleep 20
kill $PID

# 5. Inspect grammar
cat /tmp/pedrocli-grammar-debug.gbnf

# 6. Check model output
find /tmp/pedroceli-jobs -name "*-20260104-*" -type d | tail -1 | xargs ls -la
# Look at 002-response.txt

# 7. Compare to expected format
# Should start with: {"tool": "navigate", "args": ...
# Currently shows: {"args": {"action": ...

# 8. Fix schema.go property ordering (see Phase 2)

# 9. Retest
make build-cli
./pedrocli build -description "List files in pkg" -issue "test2"
```

## Reference Links

- Grammar guide: `docs/gbnf-grammar-guide.md`
- Builder usage: `docs/builder-agent-usage.md`
- llama.cpp grammars: https://github.com/ggerganov/llama.cpp/tree/master/grammars
- Current job logs: `/tmp/pedroceli-jobs/23283e6f-*/`

## Notes

- Grammar IS being applied (verified via debug logs)
- Model output is wrong format (missing "tool" field)
- Most likely cause: Property ordering bug in `SchemaToGBNF()`
- Go maps are unordered - need explicit ordering
- Fix: Force "tool" before "args" in GBNF generation
