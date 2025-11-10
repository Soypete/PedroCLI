# Pedroceli: Context Window Management

## The Critical Challenge

**Different models have different context windows. We must be acutely aware of these limits.**

## Context Window Sizes

| Model | Context | Usable (75%) | What Fits |
|-------|---------|--------------|-----------|
| Qwen 2.5 Coder 7B | 32k | ~24k | ~96KB code |
| Qwen 2.5 Coder 32B | 32k | ~24k | ~96KB code |
| Qwen 2.5 Coder 72B | 128k | ~96k | ~384KB code |
| Llama 3.1 70B | 128k | ~96k | ~384KB code |
| DeepSeek Coder 33B | 16k | ~12k | ~48KB code |

**Rule**: Use 75% of stated context (leave room for response)

## Configuration

### llama.cpp (User Responsible)

User must know their model's limits:

```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen2.5-coder-32b.gguf",
    "context_size": 32768,
    "usable_context": 24576
  }
}
```

**Assumption**: Users with llama.cpp know what they're doing.

### Ollama (Auto-Detected)

Pedroceli knows common Ollama models:

```go
var ollamaContexts = map[string]int{
    "qwen2.5-coder:7b":   32768,
    "qwen2.5-coder:32b":  32768,
    "qwen2.5-coder:72b":  131072,
    "deepseek-coder:33b": 16384,
    "codellama:34b":      16384,
    "llama3.1:70b":       131072,
}
```

```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b"
    // context_size auto-detected: 32768
  }
}
```

## Token Estimation

```go
// Rough: 1 token ≈ 4 characters
tokens = len(text) / 4

// Better: 1.3 tokens per word
tokens = word_count * 1.3
```

### Context Budget

```
Total Context: 32,768 tokens
├─ System Prompt: 2,048 tokens (6%)
├─ Task Prompt: 512 tokens (2%)
├─ Tool Definitions: 1,024 tokens (3%)
├─ Available: 20,992 tokens (64%)
└─ Reserved for Response: 8,192 tokens (25%)
```

**Available for code**: ~21k tokens = ~84KB text

## Strategic File Loading

### Priority Order

1. **Explicitly mentioned files** (from task description)
2. **Related files** (imports, dependencies)
3. **Test files** (for context)
4. **Documentation** (if room)

### Loading Strategy

```go
func LoadRelevantFiles(task Task, maxTokens int) map[string]string {
    files := make(map[string]string)
    tokenCount := 0
    
    // Priority 1: Files mentioned in task
    for _, path := range task.Files {
        content := readFile(path)
        tokens := estimateTokens(content)
        
        if tokenCount + tokens > maxTokens {
            // Too large - summarize it
            files[path] = summarizeFile(content)
        } else {
            files[path] = content
            tokenCount += tokens
        }
    }
    
    // Priority 2: Related files (if room)
    related := findRelatedFiles(task.Files)
    for _, path := range related {
        if tokenCount >= maxTokens {
            break
        }
        
        content := readFile(path)
        tokens := estimateTokens(content)
        
        if tokenCount + tokens <= maxTokens {
            files[path] = content
            tokenCount += tokens
        }
    }
    
    return files
}
```

### File Summarization

For large files, include only:
- Package declaration
- Imports
- Type definitions
- Function signatures
- Key constants/variables

```go
func summarizeFile(content string) string {
    lines := strings.Split(content, "\n")
    var summary []string
    
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        
        // Keep declarations
        if strings.HasPrefix(trimmed, "package ") ||
           strings.HasPrefix(trimmed, "import ") ||
           strings.HasPrefix(trimmed, "type ") ||
           strings.HasPrefix(trimmed, "func ") ||
           strings.HasPrefix(trimmed, "const ") ||
           strings.HasPrefix(trimmed, "var ") {
            summary = append(summary, line)
        }
    }
    
    return strings.Join(summary, "\n")
}
```

## History Compaction

### Strategy

- **Always keep**: Last 3 inference rounds (full)
- **Summarize**: Older rounds (key facts only)
- **Discard**: If still too large

```go
func GetHistoryWithinBudget(budget int) string {
    files := getAllHistoryFiles()
    
    // Keep recent (last 3 inference rounds)
    keepRecent := 3
    recentFiles := files[len(files)-keepRecent*2:]
    
    // Estimate tokens
    recentTokens := estimateTokens(readFiles(recentFiles))
    
    if recentTokens > budget {
        // Even recent doesn't fit - truncate
        return truncateToFit(recentFiles, budget)
    }
    
    // Summarize older history
    oldFiles := files[:len(files)-len(recentFiles)]
    summary := summarizeHistory(oldFiles)
    summaryTokens := estimateTokens(summary)
    
    if recentTokens + summaryTokens <= budget {
        return summary + "\n\n" + readFiles(recentFiles)
    }
    
    // Just return recent
    return readFiles(recentFiles)
}
```

### History Summary

Extract key facts from old inferences:
- Files modified
- Tests run (pass/fail)
- Errors encountered
- Commits made

```
=== Previous Work Summary ===
Steps 1-5: 
  - Modified: pkg/api/handler.go, pkg/db/queries.go
  - Tests: 12 passed, 0 failed
  - Committed: "Add rate limiting middleware"

Steps 6-8:
  - Modified: pkg/api/middleware.go
  - Tests: 1 failed (TestRateLimit)
  - Error: Redis connection timeout
  - Fixed: Added connection retry logic

=== Recent Context ===
[Full last 3 inference rounds]
```

## Repository Size Strategies

### Small Repo (<20k tokens)
**Strategy**: Load everything

```
✓ All files fit in context
✓ Full codebase awareness
✓ Best results
```

### Medium Repo (20k-50k tokens)
**Strategy**: Load most files, summarize largest

```
✓ Load all files under 2k tokens each
✓ Summarize files over 2k tokens
⚠️ Some implementation details missing
```

### Large Repo (>50k tokens)
**Strategy**: Task-based loading

```
✓ Load files mentioned in task
✓ Load direct dependencies
✓ Summarize everything else
⚠️ Limited context, multiple iterations needed
```

## CLI Warnings

```bash
$ pedroceli build --description "Refactor database layer"

📊 Context Budget Analysis:
  Model: qwen2.5-coder:32b
  Total Context: 32,768 tokens
  Usable: 24,576 tokens (75%)
  
  Budget Allocation:
  ├─ System Prompt: 2,048 tokens
  ├─ Task Description: 512 tokens
  ├─ Tool Definitions: 1,024 tokens
  └─ Available for code: 20,992 tokens (~84KB)

📁 Repository Analysis:
  Total files: 127
  Total tokens: ~45,000
  Strategy: Load task-relevant files, summarize others

⚠️  Large repository - will load strategically

Job started: job-1699401234
```

## Temp File Benefits

The temp file system helps with context:

```
/tmp/pedroceli-jobs/job-123/
├── 001-prompt.txt          # Pre-parsed, token counted
├── 002-response.txt         # Can reference without re-reading
├── 003-tool-calls.json      # Structured, easy to summarize
├── 004-tool-results.json    # Know what changed
└── ...
```

**Benefits**:
1. Know exact token counts from past inferences
2. Can summarize efficiently (JSON is structured)
3. Don't need to re-parse on retry
4. Easy to see what fits in budget

## Config Validation

```bash
$ pedroceli build --description "Test"

❌ Configuration Error:

context_size is too large: 500000
Model qwen2.5-coder:32b has max context of 32768

Fix: Update .pedroceli.json:
{
  "model": {
    "context_size": 32768,
    "usable_context": 24576
  }
}
```

## Token Tracking

```go
type InferenceMetrics struct {
    PromptTokens    int
    ResponseTokens  int
    TotalTokens     int
    ContextUsed     float64  // Percentage
}

func RecordInference(prompt, response string, contextSize int) {
    promptTokens := EstimateTokens(prompt)
    responseTokens := EstimateTokens(response)
    total := promptTokens + responseTokens
    
    usagePercent := float64(total) / float64(contextSize) * 100
    
    log.Printf("Tokens: prompt=%d response=%d total=%d (%.1f%% of %d)", 
        promptTokens, responseTokens, total, usagePercent, contextSize)
    
    if usagePercent > 90 {
        log.Printf("⚠️  High context usage! Consider compaction.")
    }
}
```

## Example: Real Project

**Pedro Bot**: ~50 Go files, ~15k lines, ~60k tokens

**With 32k context model:**
```
Strategy: Task-based loading
├─ Load task-mentioned files: ~15k tokens
├─ Load dependencies: ~5k tokens  
├─ Summarize remaining: ~2k tokens
├─ History: ~3k tokens
└─ Available: 20k tokens
Result: ✓ Fits comfortably
```

**With 16k context model:**
```
Strategy: Aggressive filtering
├─ Load task-mentioned files: ~15k tokens
├─ Summarize dependencies: ~2k tokens
├─ History (truncated): ~2k tokens
└─ Available: 12k tokens
Result: ⚠️ Tight fit, may need multiple iterations
```

## Future: Hugging Face Integration

Stretch goal - auto-detect context from HF:

```go
// Query model card for context_length
contextSize := queryHuggingFace(modelPath)
```

This would require HF MCP server or API access.

**For now**: Assume llama.cpp users know their models.

## Key Takeaways

1. **Different models = different limits**
2. **Use 75% rule** (leave room for response)
3. **Load strategically** (task-relevant first)
4. **Summarize large files** (signatures > details)
5. **Compact history** (recent full, old summarized)
6. **Track token usage** (warn if getting close)
7. **Temp files help** (pre-parsed, easy to measure)

## Config Examples

**Small model:**
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/deepseek-33b.gguf",
    "context_size": 16384,
    "usable_context": 12288
  }
}
```

**Large model:**
```json
{
  "model": {
    "type": "llamacpp",
    "model_path": "/models/qwen-72b.gguf",
    "context_size": 131072,
    "usable_context": 98304
  }
}
```

**Ollama (auto):**
```json
{
  "model": {
    "type": "ollama",
    "model_name": "qwen2.5-coder:32b"
  }
}
```

Context is managed automatically! 🎯
