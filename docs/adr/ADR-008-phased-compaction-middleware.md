# ADR-008: Context Window Management Strategy

## Status
Implemented (Partially)

**Last Updated**: 2026-02-01

## Context

The phased executor (`pkg/agents/phased_executor.go`) doesn't use the existing compaction framework from `pkg/llmcontext/manager.go`. This causes context overflow when the agent makes many tool calls, as seen in job-1769879754:

```
Error: request (46,808 tokens) exceeds available context (16,384 tokens)
```

**Specific Case**: Blog research workflow accumulated 855,954 tokens (26x over 32K limit) due to web_search and web_scraper returning 10K-50K tokens per result, with all outputs accumulating in feedback prompts.

The old agent (before phased workflow) had compaction logic that would:
1. Check token usage vs context window
2. Compact history when approaching threshold (75%)
3. Summarize old rounds, keep recent rounds

## Decision

Use **two complementary strategies** for context window management:

### Strategy 1: Tool Output Truncation (Implemented)
**Prevent** context explosion at the source by truncating large tool outputs in feedback prompts.

**Rationale**:
- Web search/scraping tools return 10K-50K tokens per call
- Feedback prompts don't need full outputs - just enough to decide next action
- Full results are preserved in context files for debugging
- Immediate fix with minimal code changes

**Implementation**: Enhanced `truncateOutput()` function used in `buildFeedbackPrompt()`

### Strategy 2: Historical Compaction (Proposed)
**Recover** from context growth by summarizing old rounds when approaching limit.

**Rationale**:
- Long-running tasks accumulate many rounds
- Recent context is most important
- Old rounds can be summarized without losing critical info
- Enables unlimited task duration

**Implementation**: Functional programming middleware pattern to wrap inference execution with compaction logic.

### Architecture

```go
// InferenceFunc is the signature for inference execution
type InferenceFunc func(ctx context.Context, systemPrompt, userPrompt string) (*llm.InferenceResponse, error)

// WithCompaction wraps an inference function with automatic compaction
func WithCompaction(
    fn InferenceFunc,
    contextMgr *llmcontext.Manager,
    config *config.Config,
) InferenceFunc {
    return func(ctx context.Context, systemPrompt, userPrompt string) (*llm.InferenceResponse, error) {
        // BEFORE inference: check if compaction is needed
        if contextMgr.ShouldCompact() {
            // Compact history (keep recent 2-3 rounds)
            _, err := contextMgr.CompactHistory(3)
            if err != nil {
                return nil, fmt.Errorf("compaction failed: %w", err)
            }

            // Log compaction stats
            if config.Debug.Enabled {
                stats, _ := contextMgr.GetCompactionStats()
                fmt.Fprintf(os.Stderr, "   📦 Compacted history: %d rounds → %d recent\n",
                    stats.TotalRounds, stats.RecentRounds)
            }
        }

        // Execute the wrapped inference
        return fn(ctx, systemPrompt, userPrompt)
    }
}
```

### Benefits of Functional Approach

1. **Separation of Concerns**: Compaction logic is separate from inference logic
2. **Composability**: Can stack multiple middlewares (compaction, logging, metrics)
3. **Testability**: Easy to test compaction logic independently
4. **DRY**: Reusable across different executors (phased, simple, etc.)
5. **No Code Duplication**: Compaction logic isn't scattered in multiple places

### Integration with Phased Executor

```go
// In phaseInferenceExecutor.executeInference()

func (pie *phaseInferenceExecutor) executeInference(ctx context.Context, systemPrompt, userPrompt string) (*llm.InferenceResponse, error) {
    // Wrap the actual inference with compaction middleware
    inferenceWithCompaction := WithCompaction(
        pie.doInference,        // The actual inference function
        pie.contextMgr,         // Context manager
        pie.agent.config,       // Config
    )

    return inferenceWithCompaction(ctx, systemPrompt, userPrompt)
}

// doInference is the unwrapped inference logic
func (pie *phaseInferenceExecutor) doInference(ctx context.Context, systemPrompt, userPrompt string) (*llm.InferenceResponse, error) {
    // Budget calculation, tool definitions, logit bias, etc.
    // (existing logic from executeInference)
    ...
}
```

### Alternative: Inline Compaction Check

For simpler implementation without functional wrappers:

```go
func (pie *phaseInferenceExecutor) executeInference(ctx context.Context, systemPrompt, userPrompt string) (*llm.InferenceResponse, error) {
    // Check compaction BEFORE building request
    if pie.contextMgr.ShouldCompact() {
        if err := pie.performCompaction(); err != nil {
            return nil, fmt.Errorf("compaction failed: %w", err)
        }
    }

    // Existing inference logic
    budget := llm.CalculateBudget(...)
    // ...
}

func (pie *phaseInferenceExecutor) performCompaction() error {
    _, err := pie.contextMgr.CompactHistory(3)
    if err != nil {
        return err
    }

    // Log compaction
    if pie.agent.config.Debug.Enabled {
        stats, _ := pie.contextMgr.GetCompactionStats()
        fmt.Fprintf(os.Stderr, "   📦 Compacted: %d→%d rounds (%d tokens)\n",
            stats.TotalRounds, stats.RecentRounds, stats.LastPromptTokens)
    }

    // Record compaction stats if store is available
    if pie.agent.compactionStatsStore != nil {
        stats, _ := pie.contextMgr.GetCompactionStats()
        // ... record to DB
    }

    return nil
}
```

## Compaction Strategy

### When to Compact
- **Threshold**: 75% of context window (configurable via `compactionThreshold`)
- **Check timing**: Before EVERY inference call
- **Token counting**: Use `contextMgr.RecordPromptTokens()` to track usage

### What to Keep
- **Recent rounds**: Keep last 2-3 rounds as-is (full detail)
- **Older rounds**: Summarize into "Previous Work Summary"
- **Critical info**: Always preserve:
  - File modifications
  - Test results
  - Error messages
  - Completion signals

### Compaction Format

```
=== Previous Work Summary ===
Rounds 1-5:
- Analyzed codebase structure
- Found 3 relevant files: auth.go, course.go, handlers.go
- Identified testing patterns using testify
- Created test files: auth_test.go, course_test.go

=== Recent Context ===
=== 006-prompt.txt ===
[full content of round 6]

=== 007-prompt.txt ===
[full content of round 7]
```

## Implementation Plan

### Phase 1: Add Compaction Check (Simple)
1. Add `performCompaction()` method to `phaseInferenceExecutor`
2. Call `contextMgr.ShouldCompact()` before inference
3. Perform compaction if needed
4. Log compaction events

### Phase 2: Middleware (Optional Enhancement)
1. Create `pkg/agents/middleware.go`
2. Implement `WithCompaction()` wrapper
3. Implement `WithLogging()`, `WithMetrics()` wrappers
4. Compose middlewares for different execution modes

### Phase 3: Testing
1. Unit test compaction logic
2. Integration test with large codebase
3. Verify context stays under limit
4. Verify compaction preserves critical info

## Consequences

### Positive
- ✅ Prevents context overflow errors
- ✅ Enables longer-running tasks
- ✅ Functional programming makes code testable
- ✅ Reusable across executors
- ✅ Easy to add more middlewares (metrics, logging)

### Negative
- ⚠️ Compaction loses some history detail
- ⚠️ Summarization quality depends on LLM
- ⚠️ Slight complexity from functional wrappers

### Mitigation
- Keep recent rounds uncompacted (preserve detail)
- Log compaction events for debugging
- Store compaction stats in database for analysis

## Implementation Status (2026-02-01)

### ✅ Implemented: Tool Output Truncation

A complementary strategy was implemented that prevents context explosion **at the source** by truncating tool outputs before adding them to feedback prompts.

**Location**: `pkg/agents/phased_executor.go:1047-1067`, `pkg/agents/executor.go:323-346`

**Key Changes**:

1. **Enhanced `truncateOutput()` function**:
   - Truncates at newline boundaries for readability
   - Adds informative message about truncated content
   - Estimates tokens saved
   - Shared by both `InferenceExecutor` and `phaseInferenceExecutor`

2. **Updated `buildFeedbackPrompt()`**:
   - Success outputs: 1000 char limit (~250 tokens)
   - Error messages: 500 char limit (~125 tokens)
   - Preserves full results in context files

3. **Added context monitoring**:
   - Warns when approaching 75% context limit
   - Provides visibility into token usage per round

**Impact**:
- Before: 20 tool calls × 40K chars = 800K tokens → **CRASH**
- After: 20 tool calls × 1K chars = 20K tokens → **SUCCESS**
- Reduction: **38x smaller feedback prompts**

**Test Coverage**:
- `TestTruncateOutput` - 5 test cases for truncation logic
- `TestBuildFeedbackPrompt_Truncates` - 4 scenarios
- `TestBuildFeedbackPrompt_ContextExplosionPrevention` - Real-world simulation

**Rationale**:
This approach is **complementary** to compaction middleware:
- **Truncation** (implemented): Prevents large outputs from entering context
- **Compaction** (proposed): Summarizes old rounds when context grows
- Together they provide defense in depth against context overflow

### 🚧 Pending: Compaction Middleware

The middleware pattern described below remains unimplemented but is still valuable for:
- Long-running tasks with many rounds
- Summarizing historical context
- Composable middleware stack (logging, metrics, compaction)

**Next Steps**:
1. Implement `performCompaction()` for phased executor
2. Add middleware pattern for composability (optional)
3. Test with large codebases and long-running workflows

## References
- Context overflow issue: job-1769879754 (fixed by truncation)
- Blog research context explosion: 855,954 tokens → 22,266 chars (fixed)
- Existing compaction: `pkg/llmcontext/manager.go:404-435`
- Compaction checking: `pkg/llmcontext/manager.go:453-460`
- Truncation implementation: `pkg/agents/phased_executor.go:1047-1067`
- Tests: `pkg/agents/executor_test.go`, `pkg/agents/phased_executor_test.go`
