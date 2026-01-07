# Ollama Context Compaction Test - January 5, 2026

## Test Configuration

- **Backend**: Ollama
- **Model**: Qwen 3 30B Coder (`qwen3-coder:30b`)
- **Context Window**: 16,384 tokens (16K)
- **Usable Context**: 12,288 tokens (75% of total)
- **Compaction Threshold**: 9,216 tokens (75% of usable)
- **Job ID**: job-1767673628
- **Task**: Build feature for issue #39 (migrate fmt to slog)
- **Config File**: `configs/ollama/qwen3-30b-coder/.pedrocli.json`

## Test Command

```bash
./pedrocli --config configs/ollama/qwen3-30b-coder/.pedrocli.json build \
  --issue 39 \
  --description 'look at this issue and make a plan and write code to migrate form fmt to slog'
```

## Token Usage Progression

| Round | Prompt Size | % Threshold | Summary Sections | Status |
|-------|-------------|-------------|------------------|--------|
| 001   | 173 tokens  | 1%          | 0                | Normal |
| 003   | 283 tokens  | 3%          | 0                | Normal |
| 005   | 438 tokens  | 4%          | 0                | Normal |
| 007   | 877 tokens  | 9%          | 0                | Normal |
| 009   | 1,637 tokens| 17%         | 1                | Growing |
| 011   | 3,086 tokens| 33%         | 2                | Growing |
| 013   | 5,766 tokens| 62%         | 4                | ⚡ Warning |
| 015   | 7,373 tokens| 80%         | 5                | ⚡ Warning |
| 017   | 9,080 tokens| **98%**     | 7                | 🔥 **PEAK #1** |
| 019   | 6,024 tokens| 65%         | 5                | ✅ **COMPACTED** |
| 021   | 7,893 tokens| 85%         | 6                | ⚡ Warning |
| 023   | 9,401 tokens| **102%**    | 8                | 🔥 **PEAK #2** (exceeded) |
| 025   | 6,377 tokens| 69%         | 6                | ✅ **COMPACTED** |
| 027   | 5,594 tokens| 60%         | 6                | ✅ Stable |
| 028   | 5,650 tokens| 61%         | 6                | 🏁 **COMPLETED** |

**Key Metrics**:
- **Total rounds**: 14 (28 prompt files due to odd numbering)
- **Peak usage**: 9,401 tokens (102% of threshold) at round 23
- **First compaction**: Round 19 (from 98% → 65%)
- **Second compaction**: Round 25 (from 102% → 69%)
- **Final usage**: 5,650 tokens (61% of threshold)
- **Compaction triggered**: 2 times
- **Summary sections**: Grew from 0 → 8 → 6 (dynamic adjustment)

## What Worked ✅

### 1. Context Compaction Prevented Crash
- **Without compaction**: llamacpp test crashed at round 8 (18,427 tokens, 112% over limit)
- **With compaction**: Ollama test ran 14 rounds without crash
- **Exceeded threshold twice** (98%, 102%) and recovered both times

### 2. Dynamic Summarization
Compaction system intelligently adjusted summary sections:
- Started with 0 sections (no history to compress)
- Grew to 8 sections at peak usage (round 23)
- Reduced to 6 sections after compaction (rounds 25-28)
- Kept most recent 2-3 rounds in full detail

### 3. Threshold Detection
- ⚡ Warning at 80% (7,373 tokens) - round 15
- 🔥 Critical at 98% (9,080 tokens) - round 17
- 🔥 Exceeded at 102% (9,401 tokens) - round 23
- ✅ Recovery to 65-69% after compaction

### 4. Monitoring Script
Real-time monitoring script successfully tracked:
- Token usage and threshold percentage
- Summary section count (compaction indicator)
- Tool calls and file modifications
- Warnings at 80% and 90% thresholds

## What Didn't Work ❌

### Tool Execution Still Broken

**Critical Issue**: Same problem as llamacpp test - tools are NOT executing.

**Evidence**:
```bash
# No tool-calls.json files exist
$ ls /tmp/pedroceli-jobs/job-1767673628-20260105-212708/*-tool-calls.json
(eval):1: no matches found

# Git shows no changes from the job
$ git status
On branch fix/context-compaction-51
# ... only our config changes, no agent changes

# No PR was created
$ gh pr list --head pedrocli/
# (empty)
```

**What the agent did**:
1. Output tool calls as JSON text in responses (e.g., `{"tool": "search", "args": {...}}`)
2. Waited for tool results that never came
3. Eventually gave up and provided example code instead of actual migration

**Impact**:
- ✅ Compaction prevented crash (SUCCESS)
- ❌ Task not completed (FAILURE)
- ❌ No code changes made
- ❌ No PR created

This is the SAME root cause as the llamacpp test. The difference:
- **llamacpp**: Crashed before completing (8 rounds, context overflow)
- **Ollama**: Didn't crash but still didn't complete (14 rounds, tools broken)

## Comparison: Ollama vs llamacpp

### Ollama (Qwen 3 30B) - This Test
- **Rounds before crash**: 14 (no crash)
- **Peak usage**: 102% (9,401 tokens at round 23)
- **Compaction triggers**: 2 times (rounds 19, 25)
- **Recovery level**: 65-69% after compaction
- **Summary sections**: 0 → 8 → 6 (dynamic)
- **Tool execution**: ❌ Broken
- **Task completion**: ❌ Failed

### llamacpp (Qwen 2.5 32B) - Previous Test
- **Rounds before crash**: 8 (crashed at 18,427 tokens)
- **Peak usage**: 112% (18,427 tokens at round 25)
- **Compaction triggers**: 1 time (round 15)
- **Recovery level**: 75% after compaction
- **Summary sections**: 4 → 10 (kept growing)
- **Tool execution**: ❌ Broken
- **Task completion**: ❌ Failed (crashed)

### Key Differences
1. **Ollama hit threshold earlier** (round 17 vs round 15)
2. **Ollama recovered more aggressively** (65% vs 75%)
3. **Ollama compacted twice** (more aggressive)
4. **llamacpp crashed**, Ollama didn't
5. **Both have broken tool execution**

## Database Schema Used

Compaction statistics were tracked in PostgreSQL:

```sql
CREATE TABLE IF NOT EXISTS compaction_stats (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(255) NOT NULL,
    round_number INT NOT NULL,
    prompt_size_bytes INT NOT NULL,
    prompt_tokens INT NOT NULL,
    summary_sections INT NOT NULL,
    threshold_tokens INT NOT NULL,
    context_limit INT NOT NULL,
    is_over_threshold BOOLEAN NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_compaction_stats_job_id ON compaction_stats(job_id);
CREATE INDEX idx_compaction_stats_created_at ON compaction_stats(created_at);
```

## Recommendations

### 1. Fix Tool Execution FIRST
**Priority**: CRITICAL

Context compaction is working as designed. The blocker is tool execution.

**Action items**:
- Investigate why tool calls aren't being parsed from responses
- Check if tool call format matches what parser expects
- Add debug logging to tool execution path
- Verify tool registry is properly wired to agents

### 2. Compaction Improvements (After Tool Execution Fixed)
**Priority**: LOW (compaction is working)

Once tools work, we can fine-tune:
- Adjust threshold from 75% if needed
- Tune summary section reduction strategy
- Add more detailed compaction metrics

### 3. Testing Strategy
**Priority**: HIGH

Create integration test that:
1. Uses small context window (4K or 8K)
2. Forces many rounds to trigger compaction
3. **Verifies tool execution works** (checks for tool-calls.json files)
4. Verifies task completion (checks for git commits, PR creation)

### 4. Monitor Both Backends
Continue testing with both Ollama and llamacpp to ensure:
- Compaction works consistently across backends
- Tool execution works with both (once fixed)
- Performance characteristics are understood

## Next Steps

1. **Immediate**: Debug tool execution system
   - Add logging to `pkg/agents/executor.go`
   - Check tool call parsing in responses
   - Verify tool registry wiring

2. **Short-term**: Create failing integration test
   - Test should verify tool calls ARE executed
   - Test should verify task completion (commits/PRs)
   - Use this to validate tool execution fix

3. **Long-term**: Full compaction validation
   - Once tools work, re-run these tests
   - Verify compaction + tool execution both work
   - Document production-ready configuration

## Conclusion

**Context compaction is WORKING** ✅
- Prevented crash (14 rounds vs 8-round crash)
- Recovered from 102% usage twice
- Dynamic summarization adjusted appropriately

**Tool execution is BROKEN** ❌
- Same issue as llamacpp test
- Agent outputs JSON but tools don't execute
- No code changes made, no PR created

**The blocker is tool execution, not compaction.**

Fix tool execution first, then re-validate compaction with working tools.

## Files Referenced

- Config: `configs/ollama/qwen3-30b-coder/.pedrocli.json`
- Job directory: `/tmp/pedroceli-jobs/job-1767673628-20260105-212708/`
- Monitoring script: `/tmp/monitor_job_1767673628.sh`
- Previous learnings: `learnings/context-compaction-testing-2026-01-05.md` (llamacpp)
