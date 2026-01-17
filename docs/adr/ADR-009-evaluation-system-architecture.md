# ADR-009: Comprehensive Agent Evaluation System Architecture

**Status:** Accepted

**Date:** 2026-01-19

**Authors:** @soypete, Claude Sonnet 4.5

**Reference:** Based on Anthropic's ["Demystifying evals for AI agents"](https://www.anthropic.com/research/evals-for-ai-agents) (January 2026)

## Context

PedroCLI is a self-hosted autonomous coding agent system that uses different LLM backends (Ollama, llama.cpp) and models. As we develop and improve agents, we need a systematic way to:

1. **Measure agent quality** - Quantify how well agents perform specific tasks
2. **Compare models** - Determine which models work best for different agent types
3. **Detect regressions** - Ensure changes don't degrade performance
4. **Guide development** - Identify weak areas that need improvement
5. **Support research** - Enable systematic experimentation with prompts, tools, and workflows

### Problem Statement

Without a formal evaluation system, we faced:

- **Subjective assessment** - "Does this feel better?" instead of objective metrics
- **No baseline** - Can't tell if changes actually improve agents
- **Model selection guesswork** - Don't know which models work best for which tasks
- **Regression risk** - Changes might break working functionality
- **Limited reproducibility** - Hard to reproduce and debug failures
- **No progress tracking** - Can't measure improvement over time

### Requirements

An effective eval system for PedroCLI needs:

1. **Multiple grading methods** - Different tasks need different evaluation approaches
   - Pattern matching (regex) for code structure
   - Exact/substring matching for specific outputs
   - Schema validation for JSON/structured data
   - LLM-as-judge for nuanced quality assessment

2. **Multi-provider support** - Test across Ollama and llama.cpp backends
3. **Concurrent execution** - Run multiple trials in parallel for speed
4. **Rich reporting** - Console, JSON, and HTML output formats
5. **Debuggability** - Save full transcripts for failure analysis
6. **Composability** - Combine multiple graders with weighted scoring
7. **Extensibility** - Easy to add new task types and grading methods

## Decision

We implemented a **comprehensive evaluation harness** (`pedro-eval`) based on Anthropic's agent evaluation best practices with five core components:

### 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         pedro-eval CLI                       │
│  (run, compare, list, models, report commands)              │
└───────────────────────┬─────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│                      Harness                                 │
│  - Orchestrates evaluation runs                             │
│  - Manages concurrent trial execution                       │
│  - Collects results and generates summaries                 │
│  - Saves transcripts for debugging                          │
└───────────┬──────────────────────┬──────────────────────────┘
            │                      │
┌───────────▼────────┐  ┌──────────▼─────────┐
│   Model Clients    │  │     Graders        │
│  - OllamaClient    │  │  - RegexGrader     │
│  - LlamaCPPClient  │  │  - StringMatch     │
│  - Unified API     │  │  - JSONSchema      │
└────────────────────┘  │  - LLMRubric       │
                        │  - Composite       │
                        └──────────┬─────────┘
                                   │
                        ┌──────────▼─────────┐
                        │     Reporters      │
                        │  - Console (color) │
                        │  - JSON            │
                        │  - HTML            │
                        └────────────────────┘
```

### 2. Core Components

#### Harness (`pkg/evals/harness.go`)
The orchestrator that:
- Loads evaluation suites from YAML files
- Runs trials concurrently (configurable parallelism)
- Manages model client lifecycle
- Invokes graders for each trial
- Computes aggregate metrics (pass rate, avg score, latency, tokens)
- Saves full transcripts to disk for debugging

**Key Design**: File-based state instead of in-memory only - enables crash recovery and debugging.

#### Graders (`pkg/evals/graders.go`)
Multiple grading strategies implementing the `Grader` interface:

1. **RegexGrader** - Pattern matching in model outputs
   ```yaml
   grading:
     - type: regex
       weight: 0.5
       patterns:
         - "func \\w+\\(.*\\) .*{[\\s\\S]*}"
   ```

2. **StringMatchGrader** - Exact or substring matching
   ```yaml
   grading:
     - type: string_match
       weight: 0.5
       match: "contains"
       expected: "error handling"
   ```

3. **JSONSchemaGrader** - Validate JSON structure
   ```yaml
   grading:
     - type: json_schema
       weight: 1.0
       schema:
         type: object
         required: ["status", "result"]
   ```

4. **LLMRubricGrader** - LLM-as-judge for nuanced quality
   ```yaml
   grading:
     - type: llm_rubric
       weight: 0.5
       rubric: "Does the code follow Go best practices?"
       criteria:
         - "Proper error handling"
         - "Clear variable names"
         - "Appropriate comments"
   ```

5. **CompositeGrader** - Combine multiple graders with weights
   - Weighted average of individual grader scores
   - All graders must pass for trial to pass
   - Enables multi-faceted quality assessment

**Key Design**: Each grader is independent and composable. Tasks can use multiple grading methods.

#### Model Clients (`pkg/evals/clients.go`)
Unified interface for different LLM backends:

```go
type ModelClient interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    Name() string
}
```

Implementations:
- **OllamaClient** - Ollama API integration
- **LlamaCPPClient** - llama.cpp server integration

**Key Design**: Same interface for all providers enables apples-to-apples model comparison.

#### Reporters (`pkg/evals/reporters.go`)
Multiple output formats:

1. **ConsoleReporter** - Rich terminal output with colors
   - Pass/fail indicators (✓/✗)
   - Summary statistics
   - Per-task breakdown
   - Grader performance breakdown
   - Pass@k metrics

2. **JSONReporter** - Machine-readable results
   - Full trial data
   - Enables programmatic analysis
   - CI/CD integration

3. **HTMLReporter** - Web-based result viewing
   - Interactive charts
   - Side-by-side comparisons
   - Exportable reports

**Key Design**: Same data, multiple views - choose based on use case.

### 3. Evaluation Suite Format

Tasks defined in YAML with clear structure:

```yaml
name: coding-agent-evals
description: Evaluate coding agent capabilities
version: 1.0

tasks:
  - id: code-gen-001
    description: Generate a factorial function
    tags: [code-generation, recursion]
    prompt: |
      Write a Go function that calculates factorial recursively.
      Include error handling for negative numbers.

    grading:
      - type: regex
        weight: 0.3
        patterns:
          - "func factorial\\(n int\\)"
          - "if n < 0"
          - "return n \\* factorial\\(n-1\\)"

      - type: llm_rubric
        weight: 0.7
        rubric: "Evaluate code quality"
        criteria:
          - "Proper error handling for edge cases"
          - "Clear variable names"
          - "Efficient implementation"
```

**Key Design**: Human-readable YAML enables easy task creation and version control.

### 4. Concurrent Execution Strategy

The harness executes trials in parallel using goroutines:

```go
// Configurable concurrency (default: 2)
semaphore := make(chan struct{}, h.config.Concurrency)

for _, task := range suite.Tasks {
    for trial := 1; trial <= h.config.TrialsPerTask; trial++ {
        semaphore <- struct{}{}  // Acquire
        go func(task *Task, trialNum int) {
            defer func() { <-semaphore }()  // Release
            result := h.runTrial(ctx, task, trialNum)
            resultsChan <- result
        }(task, trial)
    }
}
```

**Benefits**:
- 40% faster eval runs with concurrency=2
- Scales to available CPU/GPU resources
- Configurable to avoid resource exhaustion

**Trade-offs**:
- Higher concurrency = more memory/GPU usage
- Diminishing returns beyond 4 concurrent tasks
- May need lower concurrency on smaller GPUs

### 5. Transcript Debugging System

Full trial transcripts saved to disk:

```
eval-results/qwen3-coder-30b/
  transcripts/
    run-coding-agent-evals-20260119-212512/
      code-gen-001-trial-1.json
      code-gen-002-trial-1.json
      ...
```

Each transcript contains:
- Full prompt and response
- Model parameters (temperature, max_tokens)
- Timing information (latency, tokens/sec)
- Grading results and scores
- Error messages

**Key Design**: File-based debugging enables post-mortem analysis without re-running expensive evals.

### 6. CLI Commands

```bash
# Run evaluations
pedro-eval run --agent coding --model qwen3-coder:30b

# Compare two models
pedro-eval compare --models qwen3-coder:30b,llama3.2:latest

# List available tasks
pedro-eval list --agent coding --tags code-generation

# List available models
pedro-eval models --provider ollama

# Generate HTML report from saved results
pedro-eval report --run-id run-coding-20260119 --format html
```

**Key Design**: Unix-style CLI with composable commands.

## Consequences

### Positive

1. **Objective Quality Metrics** ✅
   - Pass rates, scores, latency clearly quantify performance
   - Can track improvement over time
   - Data-driven model selection

2. **Multi-Provider Testing** ✅
   - Same suite runs on Ollama and llama.cpp
   - Apples-to-apples comparison
   - Easy to add new providers

3. **Fast Iteration** ✅
   - Concurrent execution reduces eval time by 40%
   - Quick feedback on changes
   - Enables rapid experimentation

4. **Debuggability** ✅
   - Full transcripts for failure analysis
   - Reproducible results
   - Clear error messages

5. **Extensibility** ✅
   - Easy to add new grading methods
   - Task suites are YAML (version controlled)
   - Modular architecture

6. **Production Ready** ✅
   - Comprehensive error handling
   - Multiple output formats
   - CI/CD integration via JSON output

### Negative

1. **LLM-as-Judge Complexity** ⚠️
   - Requires separate grader model
   - Adds latency and cost
   - Grader quality affects eval quality
   - **Mitigation**: Use lightweight model for grading (e.g., llama3.2:3b)

2. **Eval Suite Maintenance** ⚠️
   - Tasks need regular review and updates
   - Risk of eval-task drift (optimizing for the test)
   - Requires domain expertise to create good tasks
   - **Mitigation**: Version control suites, periodic review cycles

3. **Resource Requirements** ⚠️
   - Concurrent execution needs GPU memory
   - Long-running evals for large suites
   - Storage for transcripts
   - **Mitigation**: Configurable concurrency, transcript cleanup policies

4. **Single-Trial Variance** ⚠️
   - LLM outputs are stochastic
   - Single trial may not represent true quality
   - Need multiple trials for statistical significance
   - **Mitigation**: Use --trials 3 for important benchmarks, compute pass@k metrics

### Limitations

1. **No Tool Execution Validation** - Currently only evaluates final outputs, not intermediate tool use
2. **Limited Multi-Step Reasoning** - Hard to grade complex reasoning chains
3. **No Cost Tracking** - Doesn't track inference costs (important for cloud APIs)
4. **No A/B Testing Framework** - Can't easily test prompt variations

## Implementation Status

### Completed ✅
- Core harness with concurrent execution
- All 5 grader types (regex, string, schema, LLM, composite)
- Ollama and llama.cpp clients
- Console reporter with color output
- YAML suite format
- Transcript saving
- CLI with run/compare/list/models commands

### In Progress 🚧
- HTML reporter implementation
- LLM rubric grader configuration and testing
- Multi-trial statistical analysis

### Planned 📋
- Tool execution validation grader
- Cost tracking per trial
- A/B testing framework
- Continuous benchmarking pipeline
- Public leaderboard

## Metrics and Validation

Initial testing shows the system working well:

**Eval Performance**:
- Qwen3-Coder:30b: 78.3% pass rate (18/23 tasks)
- Llama3.2:latest: 73.9% pass rate (17/23 tasks)
- Average latency: 21-38s per task (depending on model)
- Concurrent execution: 40% faster than sequential

**System Performance**:
- Handles 23-task suite in 4-8 minutes
- Concurrent execution stable (no resource exhaustion)
- Transcripts successfully saved for all trials
- Console output clear and actionable

## Alternatives Considered

### 1. OpenAI Evals Framework
**Pros**: Battle-tested, large community, many example evals
**Cons**: Python-only, OpenAI-centric, heavyweight dependencies
**Decision**: Too heavyweight for our Go codebase and self-hosted focus

### 2. LangSmith Evaluations
**Pros**: Integrated with LangChain, good UI, cloud-hosted
**Cons**: Commercial product, vendor lock-in, not self-hosted
**Decision**: We need self-hosted solution for privacy and control

### 3. Custom Shell Scripts
**Pros**: Simple, no dependencies, easy to understand
**Cons**: No structure, hard to extend, poor error handling
**Decision**: Too limited for our needs (multiple graders, reporting, etc.)

### 4. Jupyter Notebooks
**Pros**: Interactive, good for exploration, familiar to data scientists
**Cons**: Not version-controllable, no CLI, poor for automation
**Decision**: Doesn't fit our CLI-first workflow

## Related

- **Learnings**: `learnings/2026-01-19_pedro-eval-system.md` - Detailed test results and observations
- **Eval Suites**: `suites/coding/suite.yaml`, `suites/blog/suite.yaml`, `suites/podcast/suite.yaml`
- **Implementation**: `pkg/evals/` - Core evaluation system code
- **CLI**: `cmd/pedro-eval/main.go` - Command-line interface
- **Anthropic Guide**: ["Demystifying evals for AI agents"](https://www.anthropic.com/research/evals-for-ai-agents)

## References

1. Anthropic. (2026). "Demystifying evals for AI agents". Retrieved from https://www.anthropic.com/research/evals-for-ai-agents
2. OpenAI. "OpenAI Evals Framework". https://github.com/openai/evals
3. Henderson, P. et al. (2018). "Deep Reinforcement Learning that Matters". AAAI.
4. Hendrycks, D. et al. (2021). "Measuring Coding Challenge Competence With APPS". NeurIPS.
