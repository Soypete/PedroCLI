# Build Agent Prometheus Observability Testing

**Date**: 2026-01-17
**Agent**: BuilderPhasedAgent (5-phase workflow)
**Task**: Add Prometheus observability for Kubernetes deployment (GitHub Issue #32)
**Model**: Qwen 2.5 Coder 32B (via llama.cpp)
**Mode**: CLI (background job)

## Objective

Test the BuilderPhasedAgent on a real feature request (adding Prometheus metrics) to verify the 5-phase workflow while running in parallel with the blog agent test.

## Task Description

Add Prometheus observability to PedroCLI's Kubernetes deployment, including:
- Metrics endpoint configuration
- Service monitor setup
- Dashboard configuration
- Alerting rules (if applicable)

## Workflow Phases (BuilderPhasedAgent)

The BuilderPhasedAgent uses a 5-phase workflow:
1. **Analyze** (max 10 rounds) - Understand the task and explore codebase
2. **Plan** (max 5 rounds) - Create implementation plan
3. **Implement** (max 30 rounds) - Write code and make changes
4. **Validate** (max 15 rounds) - Run tests and verify changes
5. **Deliver** (max 5 rounds) - Create PR and finalize

## Test Results

### Final Outcome: ❌ FAILED

**Status**: Failed in Analyze phase
**Error**: `max inference rounds (25) reached without completion`
**Reason**: Consistent search tool failures due to missing 'action' parameter
**Job Directory**: `/tmp/pedroceli-jobs/job-1768708813-20260117-210013`

### Phase 1: Analyze ❌
**Status**: Failed (incomplete after 25 rounds)
**Duration**: ~8 minutes
**Rounds Used**: 25/25 (max reached)
**Tool Uses**: ~25 (all search tool calls)
**Tool Success Rate**: ~13% (3 successes out of ~23 attempts)

**Tool Failure Pattern**:
```
Round 1:  Multiple tool failures (unknown actions)
Round 2:  ❌ search failed - missing 'action' parameter
Round 3:  ✅ search succeeded
Round 4:  ✅ search succeeded
Round 5:  ❌ search failed - missing 'action' parameter
Round 6:  ❌ search failed - missing 'action' parameter
Round 7:  ❌ search failed - missing 'action' parameter
Round 8:  ❌ search failed - missing 'action' parameter
Round 9:  ❌ search failed - missing 'action' parameter
Round 10: ❌ search failed - missing 'action' parameter
Round 11: ❌ search failed - missing 'action' parameter
Round 12: ❌ search failed - missing 'action' parameter
Round 13: ❌ search failed - missing 'action' parameter
Round 14: ❌ search failed - missing 'action' parameter
Round 15: ❌ search failed - missing 'action' parameter
Round 16: ❌ search failed - missing 'action' parameter
Round 17: ❌ search failed - missing 'action' parameter
Round 18: ✅ search succeeded
Round 19: ❌ search failed - missing 'action' parameter
Round 20: ❌ search failed - missing 'action' parameter
Round 21: ❌ search failed - missing 'action' parameter
Round 22: ❌ search failed - missing 'action' parameter
Round 23: ❌ search failed - missing 'action' parameter
Round 24: ❌ search failed - missing 'action' parameter
Round 25: ❌ search failed - missing 'action' parameter
```

**Successful Rounds**: 3, 4, 18 (only 3 out of ~23 search attempts)
**Success Rate**: ~13%
**Failure Mode**: Intermittent - successes surrounded by many failures

### Phases 2-5: Not Reached
- **Plan**: Not started (stuck in Analyze)
- **Implement**: Not started
- **Validate**: Not started
- **Deliver**: Not started

---

## Root Cause Analysis

### The Problem: Missing Required Parameter

The search tool requires an `action` parameter to specify what kind of search to perform:
- `grep` - Search file contents for pattern
- `find_files` - Find files by pattern
- `find_in_file` - Search within specific file
- `find_definition` - Find code definitions

**Expected Tool Call**:
```json
{
  "tool": "search",
  "args": {
    "action": "grep",
    "pattern": "deployment",
    "path": "deployments/kubernetes"
  }
}
```

**Actual Tool Call** (failing):
```json
{
  "tool": "search",
  "args": {
    "pattern": "deployment|deploy|k8s",
    "path": "deployments/kubernetes"
  }
}
```

Notice the missing `"action"` field in args.

### Why This Happened

1. **LLM Probabilistic Behavior**: The model generates tokens based on probability distributions
2. **No Structural Enforcement**: JSON schema validation happens after generation, not during
3. **Insufficient Attention**: In long context (5000+ tokens for Analyze phase), model may not consistently attend to parameter requirements
4. **Example Presence**: System prompt has examples, but they don't guarantee compliance

### Evidence from Job Files

**File**: `/tmp/pedroceli-jobs/job-1768708813-20260117-210013/034-response.txt`
```
Let's proceed by finding the relevant Kubernetes deployment file. Since the search for `deployment.yaml`
did not yield results, we need to search more broadly for Kubernetes deployment files.

We'll search for files with patterns like `deployment`, `deploy`, or `k8s` in the `deployments/kubernetes`
directory to identify the correct deployment file.

{"tool": "search", "args": {"pattern": "deployment|deploy|k8s", "path": "deployments/kubernetes"}}
```

The LLM's reasoning was sound, but it forgot to include the required `action` parameter.

---

## Impact on Workflow

### Analyze Phase Stalled
- Agent spent all 25 rounds trying to search for files
- Never progressed past initial exploration
- Could not gather enough context to move to Plan phase

### Cascading Failure
```
Analyze (stuck) → Plan (not reached) → Implement (not reached) → Validate (not reached) → Deliver (not reached)
```

The entire 5-phase workflow failed because the first phase couldn't complete.

### Wasted Resources
- **Time**: ~8 minutes
- **Inference Rounds**: 25/25 used
- **Token Usage**: Unknown (estimated ~25k+ tokens across 25 rounds)
- **Outcome**: No usable output

---

## Comparison to Blog Agent

Running in parallel, we tested both agents on the same model:

| Metric | Blog Agent (9 phases) | Build Agent (5 phases) |
|--------|---------------------|---------------------|
| **Outcome** | ✅ Success | ❌ Failure |
| **Duration** | ~5 minutes | ~8 minutes |
| **Phases Completed** | 9/9 (100%) | 0/5 (0%) |
| **Token Usage** | 47.6k tokens | ~25k+ tokens (estimated) |
| **Tool Success Rate** | N/A (no tool-heavy phases) | ~13% (search tool) |
| **Output Quality** | Excellent (2545 word blog post) | None (stuck in Analyze) |

**Key Difference**: Blog agent didn't rely on tools during critical phases; build agent needed search tool to gather context.

---

## Solution: Logit Bias

This failure directly validates the need for the logit bias fix documented in `learnings/2026-01-17_llm-tool-parameter-reliability.md`.

### Proposed Fix

**Implementation**:
```go
// In pkg/agents/executor.go or pkg/llm/server.go
req := llm.ChatCompletionRequest{
    Messages:    messages,
    Temperature: 0.2,
    MaxTokens:   2048,
    LogitBias: map[string]float64{
        "action": 5.0,  // Boost "action" token probability
        "args":   2.0,  // Also boost "args" presence
    },
}
```

**Expected Impact**:
- Increase search tool success rate from ~13% to >80%
- Allow Analyze phase to complete within 10 rounds
- Enable progression to Plan → Implement → Validate → Deliver phases
- Successful feature implementation

### Alternative Fixes Considered

1. **Increase Max Rounds**: Would waste more time on failed searches, not solve root cause
2. **Simplify Tool**: Splitting search into 4 separate tools increases prompt size, doesn't guarantee compliance
3. **Retry Logic**: Wastes inference rounds, creates confusing feedback loops
4. **Better Prompts**: Already have clear examples; adding more doesn't guarantee compliance

---

## Lessons Learned

### 1. Tool Reliability is Critical for Coding Agents
Blog agents can succeed without tools (pure content generation), but coding agents MUST have reliable tool calls to:
- Search codebases
- Read files
- Edit code
- Run tests
- Create PRs

### 2. Phase Isolation Helps Debug Failures
Because the agent failed in Analyze phase, we know exactly where the problem occurred. Without phased workflows, debugging would be much harder.

### 3. Max Rounds Should Match Phase Complexity
Analyze phase has max 10 rounds, but the agent used 25 rounds because it was configured in CLI mode (which overrode phase limits). This suggests:
- Phase-specific limits should be enforced strictly
- CLI should respect phase configurations

### 4. Probabilistic Failures Compound
3 successes out of 23 attempts = 13% success rate
If a task requires 5 sequential successful tool calls:
- Probability of success = 0.13^5 = 0.0000371 (~0.004%)
- Practically guaranteed failure

### 5. Tool Parameter Validation Should Be Visible
The error message `"missing 'action' parameter"` was clear, but the agent didn't learn from it. Better feedback might include:
- "Required parameter 'action' missing. Valid actions: grep, find_files, find_in_file, find_definition"
- "Example: {\"action\": \"grep\", \"pattern\": \"...\", \"path\": \"...\"}"

---

## Recommendations

### Immediate (PR #1)
1. ✅ **Implement Logit Bias**: Boost probability of required parameters
2. ✅ **Document Issue**: This learning + tool parameter reliability learning
3. ⏳ **Test Fix**: Re-run build agent with logit bias enabled

### Short-Term (PR #2-5)
1. **Enhanced Error Messages**: Include examples in tool failure messages
2. **Tool Call Validation**: Pre-generation schema hints (if possible)
3. **Success Rate Metrics**: Track tool call success rates per agent type

### Long-Term (Future Work)
1. **Adaptive Prompting**: If tool fails 3x in a row, inject additional examples
2. **Tool Call Recovery**: Automatic parameter inference from context
3. **Model-Specific Tuning**: Different logit bias values for different models

---

## Acceptance Criteria (For Fix Validation)

When testing the logit bias fix, the build agent should:
- [ ] Complete Analyze phase within 10 rounds
- [ ] Achieve >80% search tool success rate
- [ ] Progress to Plan phase
- [ ] Create implementation plan for Prometheus observability
- [ ] Implement at least partial solution (even if not perfect)
- [ ] Not hit max rounds in any phase

---

## Artifacts

### Job Directory
`/tmp/pedroceli-jobs/job-1768708813-20260117-210013`

**Key Files**:
- `001-prompt.txt` - Initial system prompt + task description
- `002-response.txt` through `074-response.txt` - All 25 inference rounds
- Tool call JSON files showing failure pattern
- Tool result JSON files with error messages

### Final Status Output
```
Job job-1768708813 (build):
Status: failed
Description: Add Prometheus observability for Kubernetes deployment
Error: max inference rounds (25) reached without completion

❌ Job failed!
```

---

## Related Learnings

- `learnings/2026-01-17_llm-tool-parameter-reliability.md` - Root cause analysis + logit bias solution
- `learnings/2026-01-17_blog-agent-testing.md` - Parallel successful test showing contrast

---

## References

- **BuilderPhasedAgent**: `pkg/agents/builder_phased.go`
- **Search Tool**: `pkg/tools/search.go`
- **InferenceExecutor**: `pkg/agents/executor.go`
- **GitHub Issue**: #32 (Add Prometheus observability)
- **Job Files**: `/tmp/pedroceli-jobs/job-1768708813-20260117-210013/`
