# GLM-4 Chat Template for llama-server

## Problem

When using GLM-4 models with llama-server, you may see this warning:

```
Template supports tool calls but does not natively describe tools. The
fallback behaviour used may produce bad results, inspect prompt w/ --verbose
& consider overriding the template.
```

This causes:
- **Slow inference** - Model struggles to understand tool definitions
- **Missing parameters** - Tools called without required arguments
- **Inefficient prompts** - Fallback template is suboptimal

## Solution

Use the custom GLM-4 chat template that properly formats tool definitions.

## Usage

### Start llama-server with Custom Template

```bash
llama-server \
  --model ~/.cache/huggingface/hub/.../GLM-4-Flash-GGUF/glm-4-9b-chat-q8_0.gguf \
  --port 8082 \
  --ctx-size 32768 \
  --n-gpu-layers -1 \
  --chat-template templates/glm4-tools-template.jinja \
  --jinja \
  --no-webui \
  --metrics
```

**Key flags:**
- `--chat-template templates/glm4-tools-template.jinja` - Use custom template
- `--jinja` - Enable Jinja2 template processing
- `--ctx-size 32768` - GLM-4-9B supports 32K context
- `--n-gpu-layers -1` - Offload all layers to GPU (Metal on Mac)

### Makefile Target

Update your `Makefile` to include the chat template:

```makefile
llama-server:
	llama-server \
	  --model $(GLM4_MODEL_PATH) \
	  --port 8082 \
	  --ctx-size 32768 \
	  --n-gpu-layers -1 \
	  --chat-template templates/glm4-tools-template.jinja \
	  --jinja \
	  --no-webui \
	  --metrics
```

### Verify Template is Working

1. **Check for warning**:
   ```bash
   # Start llama-server with template
   # Warning should NOT appear now
   ```

2. **Test with verbose mode**:
   ```bash
   llama-server \
     --chat-template templates/glm4-tools-template.jinja \
     --verbose \
     # ... other flags
   ```

   You should see properly formatted tool definitions in the prompt.

3. **Test tool calling**:
   ```bash
   ./pedrocli blog -file test.txt
   ```

   Should see:
   - ✅ Faster inference
   - ✅ Correct parameter usage (no "inferred action" warnings after logit bias is added)
   - ✅ Proper completion signals

## Template Format

The template uses GLM-4's native format:

### Special Tokens
- `<|system|>` - System prompt
- `<|user|>` - User message
- `<|assistant|>` - Assistant response
- `<|observation|>` - Tool result

### Tool Call Format
```json
{
  "name": "tool_name",
  "arguments": {
    "param1": "value1"
  }
}
```

### Tool Definitions
Tools are described in the system prompt with:
- Function name
- Description
- JSON Schema for parameters

## Benefits

**Before (fallback template):**
- ⚠️ Warning about template compatibility
- 🐌 Slow inference (model confused by bad formatting)
- ❌ Missing required parameters
- 🔄 More inference rounds (model keeps trying to use tools)

**After (custom template):**
- ✅ No warnings
- ⚡ Faster inference (clean tool definitions)
- ✅ Correct parameter usage
- 🎯 Fewer rounds needed (model understands tools immediately)

## Troubleshooting

### Warning Still Appears

Check that:
1. Path to template is correct (relative to where you run llama-server)
2. `--jinja` flag is present
3. Template file has `.jinja` extension

### Tool Calls Still Malformed

1. Check llama-server logs with `--verbose`
2. Verify tools are being passed in OpenAI-compatible format
3. Ensure PedroCLI is using GLM-4 formatter (auto-detected from model name)

### Template Not Found

Use absolute path:
```bash
--chat-template /absolute/path/to/pedrocli/templates/glm4-tools-template.jinja
```

## Related

- Issue #92 - Logit bias for strict tool calling
- `pkg/toolformat/glm4.go` - GLM-4 tool formatter
- llama.cpp docs: https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md#chat-template

## Testing

```bash
# 1. Start llama-server with template
make llama-server

# 2. Run blog workflow
./pedrocli blog -file test_ontology_blog.txt

# 3. Verify:
# - No template warning
# - Faster inference
# - Clean tool calls
# - Research phase completes (with our fixes)
```
