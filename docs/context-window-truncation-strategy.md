# Context Window Truncation Strategy

**Status**: ✅ Implemented
**Last Updated**: 2026-02-01
**Related**: ADR-008 (Context Window Management Strategy)

## Overview

This document describes the tool output truncation strategy implemented to prevent context window explosion in PedroCLI's autonomous agents.

## Problem Statement

### The Context Explosion

During tool-heavy workflows (especially blog research), the InferenceExecutor accumulated massive context that exceeded the model's context window:

```
Error: request (855,954 tokens) exceeds context (32,768 tokens)
```

**Root Cause**: Full tool outputs were added to feedback prompts without any truncation.

### Specific Scenario

**Blog Research Workflow** (20 rounds):
- Each web_search call returns 10K-50K tokens
- Each web_scraper call returns similar amounts
- All outputs accumulated in feedback prompts
- After 20 rounds: 800K+ tokens → **CRASH**

### Code Location of Problem

`pkg/agents/executor.go:323` (before fix):

```go
func (e *InferenceExecutor) buildFeedbackPrompt(calls []llm.ToolCall, results []*tools.Result) string {
    // ...
    for i, call := range calls {
        result := results[i]
        if result.Success {
            // PROBLEM: Full output added to prompt!
            prompt.WriteString(fmt.Sprintf("✅ %s: %s\n", call.Name, result.Output))
        }
    }
}
```

This accumulated 40K+ tokens per round × 20 rounds = 800K tokens!

## Solution: Proactive Truncation

### Strategy

**Truncate at the source** - prevent large outputs from entering the context in the first place.

**Key Insight**: Feedback prompts don't need full outputs. The LLM only needs:
- Enough info to decide next action (~250 tokens)
- Indication that full data is available if needed
- Clear success/failure status

Full results are preserved in context files for debugging and potential retrieval.

### Implementation Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Inference Loop                            │
├─────────────────────────────────────────────────────────────┤
│  1. LLM generates tool calls                                 │
│  2. Tools execute → Large outputs (10K-50K tokens)          │
│  3. Results saved to context files (FULL)  ✅               │
│  4. buildFeedbackPrompt() truncates outputs (1K chars)      │
│  5. Truncated feedback sent to LLM                          │
│  6. Repeat                                                   │
└─────────────────────────────────────────────────────────────┘
```

**Two-tier storage**:
1. **Context files**: Full, untruncated results (for debugging/retrieval)
2. **Feedback prompts**: Truncated outputs (for decision-making)

## How We Truncate

### 1. Enhanced `truncateOutput()` Function

**Location**: `pkg/agents/phased_executor.go:1047-1067`

**Shared by**: Both `InferenceExecutor` and `phaseInferenceExecutor`

```go
func truncateOutput(output string, maxLen int) string {
    if len(output) <= maxLen {
        return output  // Don't truncate short outputs
    }

    truncated := output[:maxLen]

    // Try to truncate at a newline to avoid mid-sentence cuts
    if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxLen/2 {
        truncated = truncated[:lastNewline]
    }

    // Count approximate tokens truncated
    truncatedChars := len(output) - len(truncated)
    truncatedTokens := truncatedChars / 4  // Rough estimation: 4 chars ≈ 1 token

    return fmt.Sprintf("%s\n\n[Output truncated: ~%d more tokens available. Full result saved to context files.]",
        truncated, truncatedTokens)
}
```

**Features**:
- ✅ Truncates at newline boundaries (readability)
- ✅ Estimates tokens saved (visibility)
- ✅ Informs LLM about full results (transparency)
- ✅ Handles short outputs gracefully (no overhead)

### 2. Updated `buildFeedbackPrompt()`

**Locations**:
- `pkg/agents/executor.go:323-346`
- `pkg/agents/phased_executor.go:920-934`

```go
func (e *InferenceExecutor) buildFeedbackPrompt(calls []llm.ToolCall, results []*tools.Result) string {
    var prompt strings.Builder
    prompt.WriteString("Tool execution results:\n\n")

    for i, call := range calls {
        result := results[i]

        if result.Success {
            // Truncate large outputs to prevent context window explosion
            truncated := truncateOutput(result.Output, 1000)
            prompt.WriteString(fmt.Sprintf("✅ %s: %s\n", call.Name, truncated))
        } else {
            // Errors are typically short, but truncate to be safe
            truncated := truncateOutput(result.Error, 500)
            prompt.WriteString(fmt.Sprintf("❌ %s failed: %s\n", call.Name, truncated))
        }
    }

    prompt.WriteString("\nBased on these results, what should we do next?...")
    return prompt.String()
}
```

**Truncation Limits**:
- **Success outputs**: 1000 chars (~250 tokens)
- **Error messages**: 500 chars (~125 tokens)

**Rationale**:
- 250 tokens is enough to understand what happened
- Error messages rarely exceed 500 chars anyway
- Keeps feedback prompts under 2K total

### 3. Context Budget Warning

**Location**: `pkg/agents/executor.go:95-103`

```go
// Check context budget and warn if nearing limit
if e.currentRound > 1 {
    stats, err := e.contextMgr.GetCompactionStats()
    if err == nil && stats.IsOverThreshold {
        fmt.Fprintf(os.Stderr, "⚠️  Context near limit: %d/%d tokens (%.0f%%)\n",
            stats.LastPromptTokens, stats.ContextLimit,
            float64(stats.LastPromptTokens)/float64(stats.ContextLimit)*100)
    }
}
```

**Purpose**: Early warning before hitting hard limit

## Impact & Results

### Before Fix

```
Round 1:  50K tokens (web_search result)
Round 2:  95K tokens (round 1 + new search)
Round 3: 135K tokens (rounds 1-2 + new scrape)
...
Round 20: 855K tokens → CRASH ❌
```

### After Fix

```
Round 1:  250 tokens (truncated feedback)
Round 2:  500 tokens (2 truncated feedbacks)
Round 3:  750 tokens (3 truncated feedbacks)
...
Round 20: 5K tokens → SUCCESS ✅
```

### Performance Metrics

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| **Tokens per round** | ~40K | ~250 | **160x reduction** |
| **20 rounds total** | 855K | 22K | **38x reduction** |
| **Context overflow** | YES ❌ | NO ✅ | Fixed |
| **Full data preserved** | YES | YES | No loss |

## Truncation Examples

### Example 1: Web Search (Large Output)

**Original output** (15,000 chars):
```
Found 10 search results for "Go context patterns"...

1. Go Context Best Practices - go.dev
   Full article text here with thousands of words about context usage,
   examples, code snippets, detailed explanations...
   [... 14,800 more chars ...]
```

**Truncated feedback** (1,000 chars + message):
```
Found 10 search results for "Go context patterns"...

1. Go Context Best Practices - go.dev
   Full article text here with thousands of words about context usage,
   examples, code snippets, detailed explanations...
   [... up to 1000 chars ...]

[Output truncated: ~3500 more tokens available. Full result saved to context files.]
```

**LLM sees**: Enough to know search succeeded and what topics were found
**Full data**: Available in `/tmp/pedrocli-jobs/<job-id>/NNN-tool-results.json`

### Example 2: Code File (Medium Output)

**Original output** (800 chars):
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, world!")
}
// ... 700 more chars of code ...
```

**Truncated feedback**: Same (under 1000 char limit)

No truncation occurs because output is already small.

### Example 3: Error Message (Small Output)

**Original error** (150 chars):
```
file not found: pkg/nonexistent/file.go - check the path and try again
```

**Truncated feedback**: Same (under 500 char limit)

Error messages are typically short and don't need truncation.

## Testing

### Test Coverage

**File**: `pkg/agents/executor_test.go`

1. **TestBuildFeedbackPrompt_Truncates** - 4 scenarios:
   - Small output not truncated
   - Large output truncated
   - Error message truncated
   - Multiple tools with mixed sizes

2. **TestBuildFeedbackPrompt_ContextExplosionPrevention**:
   - Simulates 20 tool calls with 40K output each
   - Verifies prompt stays under 30K chars
   - **Result**: 22,266 chars ✅

**File**: `pkg/agents/phased_executor_test.go`

3. **TestTruncateOutput** - 5 test cases:
   - Short output unchanged
   - Exact length unchanged
   - Long output truncated
   - Very large output heavily truncated
   - Output with newlines truncates at newline

### Running Tests

```bash
# Run truncation tests
go test ./pkg/agents -run TestTruncate -v

# Run feedback prompt tests
go test ./pkg/agents -run TestBuildFeedback -v

# Run all agent tests
go test ./pkg/agents -v

# Check coverage
go test ./pkg/agents -cover
# Result: buildFeedbackPrompt has 100% coverage
```

### Integration Testing

```bash
# Test with real blog workflow
./pedrocli blog -file test_transcript.txt

# Expected: Research phase completes without context overflow
# Check logs: No "exceeds context" errors
# Verify: Job completes successfully
```

## Why This Works

### 1. Feedback Prompts Don't Need Full Data

The LLM uses feedback to decide **what to do next**, not to process the full data:

```
❌ BAD: "Here are 50,000 tokens of search results, analyze them all in detail"
✅ GOOD: "Search found 10 results about Go contexts. What should we do next?"
```

### 2. Full Data Is Preserved

Context files contain complete results:
- `/tmp/pedrocli-jobs/<job-id>/004-tool-results.json` - Full outputs
- Can be retrieved if needed (future enhancement)
- Available for debugging and post-mortem analysis

### 3. LLM Knows About Truncation

The truncation message explicitly tells the LLM:
```
[Output truncated: ~3500 more tokens available. Full result saved to context files.]
```

This allows the LLM to:
- Understand it's seeing a summary
- Know full data exists if retrieval is needed
- Continue with informed decision-making

## Relationship to Other Strategies

### Complementary to Compaction

This truncation strategy is **complementary** to the compaction middleware (ADR-008):

| Strategy | When | Purpose | Status |
|----------|------|---------|--------|
| **Truncation** | Every round | Prevent explosion at source | ✅ Implemented |
| **Compaction** | When near limit | Summarize old rounds | 🚧 Proposed |

**Defense in Depth**:
1. **Truncation**: Stops problem before it starts (proactive)
2. **Compaction**: Recovers if it happens anyway (reactive)

Both together provide robust context management.

### Token Estimation vs Actual Tokenization

**Current approach**: Token estimation (`chars / 4`)

**Rationale**:
- Fast (no API calls)
- Good enough for truncation decisions
- Exact counts not needed for this use case

**Future enhancement**: Use llama-server `/tokenize` endpoint for exact counts (optional)

## Configuration

Currently hardcoded limits work well:

```go
const (
    SuccessOutputMaxChars = 1000  // ~250 tokens
    ErrorOutputMaxChars   = 500   // ~125 tokens
)
```

**Future enhancement**: Make configurable in `.pedrocli.json`:

```json
{
  "limits": {
    "tool_output_max_chars": 1000,
    "error_output_max_chars": 500,
    "enable_smart_truncation": true
  }
}
```

## Future Enhancements

### 1. File Path References in Truncation Message

Include cache file paths so LLM can retrieve if needed:

```
[Output truncated: ~3500 more tokens available.
 Full result in: /tmp/pedrocli-jobs/blog-research-123/015-tool-results.json
 Use file tool to read if needed.]
```

### 2. Per-Tool Truncation Limits

Different tools may need different limits:

```go
var toolTruncationLimits = map[string]int{
    "web_search":  1000,  // Summaries of search results
    "web_scraper": 500,   // Just enough to know what was found
    "file":        2000,  // Code files may need more context
    "test":        1500,  // Test output can be important
}
```

### 3. Smart Summarization

Use LLM to summarize large outputs instead of truncation:

```go
if len(output) > maxChars && config.SmartSummarization {
    summary := llm.Summarize(output, maxTokens: 200)
    return summary + "\n[Summarized by LLM. Full result in context files.]"
}
```

**Trade-off**: More expensive but preserves semantic content

### 4. Progressive Truncation

Truncate more aggressively as rounds increase:

```go
func (e *InferenceExecutor) getTruncationLimit() int {
    base := 1000
    if e.currentRound > 10 {
        return base / 2  // More aggressive after 10 rounds
    }
    return base
}
```

## Lessons Learned

### 1. Simple Solutions First

Started with simple character-based truncation instead of complex summarization. It worked perfectly.

### 2. Preserve Raw Data

Always keep full results in context files. Truncation is for **prompts only**.

### 3. Inform the LLM

Explicit truncation messages help LLM understand it's seeing a summary.

### 4. Test With Real Workloads

The "20 rounds of blog research" test case was critical for validating the fix.

### 5. Defense in Depth

Truncation + compaction + warnings = robust context management

## References

- **ADR-008**: Context Window Management Strategy
- **Implementation**:
  - `pkg/agents/phased_executor.go:1047-1067` (truncateOutput)
  - `pkg/agents/executor.go:323-346` (buildFeedbackPrompt)
- **Tests**:
  - `pkg/agents/executor_test.go`
  - `pkg/agents/phased_executor_test.go`
- **Issue**: Context explosion in blog research (855,954 tokens)
- **Fix verification**: PR #77 - InferenceExecutor integration

## Success Metrics

✅ **Research phase completes** without context overflow
✅ **38x reduction** in feedback prompt size
✅ **100% test coverage** on buildFeedbackPrompt
✅ **No data loss** - full results preserved
✅ **Backward compatible** - short outputs unchanged

---

**Last Validated**: 2026-02-01
**Next Review**: When implementing compaction middleware
