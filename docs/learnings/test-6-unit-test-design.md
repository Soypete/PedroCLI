# Unit Test Design: Phased Workflow Fixes

**Related:** `test-6-analysis-phased-workflow-bugs.md`
**Goal:** Make phased workflow testable and prevent regressions

## Testing Philosophy

**Key Principle:** If we can't test it, we can't trust it.

Current state:
- ❌ Phase transition logic embedded in executor (not testable)
- ❌ Prompt construction opaque (can't verify inputs)
- ❌ Feedback generation has no assertions
- ❌ Tool tracking doesn't exist

Target state:
- ✅ Extract pure functions for testing
- ✅ Test prompt construction in isolation
- ✅ Test feedback generation with different scenarios
- ✅ Test tool tracking and loop detection
- ✅ Integration tests for phase transitions

---

## Test Structure

```
pkg/agents/
├── phased_executor.go          # Implementation
├── phased_executor_test.go     # Unit tests
├── phased_executor_internal.go # Extracted testable functions (NEW)
├── phased_executor_integration_test.go  # Integration tests (NEW)
└── testdata/
    ├── phase_outputs/          # Sample phase outputs
    └── expected_inputs/        # Expected sanitized inputs
```

---

## Unit Tests

### 1. Prompt Construction Tests

**File:** `pkg/agents/phased_executor_test.go`

#### Test: `TestBuildNextPhaseInput_RemovesJSONExamples`

**Purpose:** Verify that JSON code blocks don't leak into next phase

**Input:**
```go
result := &PhaseResult{
    PhaseName: "implement",
    Output: `# Implementation Complete

Created pkg/metrics/metrics.go with Prometheus counters.

## Next Steps:
Run LSP diagnostics:

` + "```json\n{\"tool\": \"lsp\", \"args\": {\"operation\": \"diagnostics\", \"file\": \"pkg/metrics/metrics.go\"}}\n```" + `

PHASE_COMPLETE`,
    ModifiedFiles: []string{"pkg/metrics/metrics.go", "pkg/metrics/metrics_test.go"},
}
```

**Expected Output:**
```
# Previous Phase: implement

## Summary
Implementation Complete

Created pkg/metrics/metrics.go with Prometheus counters.

PHASE_COMPLETE

## Modified Files
- pkg/metrics/metrics.go
- pkg/metrics/metrics_test.go
```

**Assertion:**
```go
func TestBuildNextPhaseInput_RemovesJSONExamples(t *testing.T) {
    result := &PhaseResult{
        PhaseName: "implement",
        Output: "# Implementation Complete\n\nCreated pkg/metrics/metrics.go.\n\n```json\n{\"tool\": \"lsp\", \"args\": {\"operation\": \"diagnostics\"}}\n```\n\nPHASE_COMPLETE",
        ModifiedFiles: []string{"pkg/metrics/metrics.go"},
    }

    input := buildNextPhaseInput(result)

    // Should NOT contain raw JSON that looks like tool calls
    assert.NotContains(t, input, `{"tool": "lsp"`)
    assert.NotContains(t, input, `"operation": "diagnostics"`)

    // Should contain sanitized version
    assert.Contains(t, input, "Previous Phase: implement")
    assert.Contains(t, input, "PHASE_COMPLETE")
    assert.Contains(t, input, "Modified Files")
    assert.Contains(t, input, "pkg/metrics/metrics.go")
}
```

---

#### Test: `TestSanitizePhaseOutput_VariousFormats`

**Purpose:** Test sanitization with different JSON formats

**Test Cases:**

1. **JSON code blocks with tool calls:**
   ```markdown
   Input: "```json\n{\"tool\": \"file\", \"args\": {\"path\": \"test.go\"}}\n```"
   Output: "[JSON example removed]"
   ```

2. **Inline JSON (not in code blocks):**
   ```markdown
   Input: "The config is {\"enabled\": true}"
   Output: "The config is {\"enabled\": true}" (unchanged - not a tool call)
   ```

3. **Multiple JSON blocks:**
   ```markdown
   Input: "```json\n{\"tool\": \"lsp\"}\n```\n\nAnd\n\n```json\n{\"tool\": \"git\"}\n```"
   Output: "[JSON example removed]\n\nAnd\n\n[JSON example removed]"
   ```

4. **PHASE_COMPLETE with instructions:**
   ```markdown
   Input: "### Step 3: Run tests\n```bash\ngo test\n```\nPHASE_COMPLETE"
   Output: "PHASE_COMPLETE" (strip procedural instructions)
   ```

**Implementation:**
```go
func TestSanitizePhaseOutput_RemovesJSONCodeBlocks(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        want     string
        notWant  []string
    }{
        {
            name:  "removes json code blocks with tool calls",
            input: "Created file\n```json\n{\"tool\": \"lsp\", \"args\": {\"file\": \"test.go\"}}\n```\nDone",
            want:  "Created file",
            notWant: []string{`{"tool"`, `"lsp"`, `"args"`},
        },
        {
            name:  "preserves inline json config",
            input: "Config: {\"enabled\": true, \"timeout\": 30}",
            want:  "Config: {\"enabled\": true, \"timeout\": 30}",
        },
        {
            name:  "removes multiple json blocks",
            input: "```json\n{\"tool\": \"a\"}\n```\nText\n```json\n{\"tool\": \"b\"}\n```",
            notWant: []string{`"tool"`, `"a"`, `"b"`},
        },
        {
            name:  "extracts phase complete and summary",
            input: "### Step 1\nDo this\n### Step 2\nDo that\n\nSummary: Done!\n\nPHASE_COMPLETE",
            want:  "Summary: Done!\n\nPHASE_COMPLETE",
            notWant: []string{"### Step", "Do this", "Do that"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := sanitizePhaseOutput(tt.input)

            if tt.want != "" {
                assert.Contains(t, got, tt.want)
            }

            for _, notWant := range tt.notWant {
                assert.NotContains(t, got, notWant)
            }
        })
    }
}
```

---

### 2. Feedback Generation Tests

**File:** `pkg/agents/phased_executor_test.go`

#### Test: `TestBuildFeedbackPrompt_GitStatus_Guidance`

**Purpose:** Verify git status interpretation and next-step guidance

**Test Cases:**

1. **Untracked files (`??`):**
   ```go
   Input:  git status → "?? pkg/metrics/\n"
   Output: "Git shows UNTRACKED files... NEXT STEP: Stage them with: {\"tool\": \"git\", \"args\": {\"action\": \"add\", \"files\": [...]}}"
   ```

2. **Modified unstaged (`M `):**
   ```go
   Input:  git status → " M pkg/server.go\n"
   Output: "Git shows MODIFIED files... NEXT STEP: Stage them with git add."
   ```

3. **Clean working tree:**
   ```go
   Input:  git status → "nothing to commit, working tree clean"
   Output: "Working tree is clean. No changes to deliver."
   ```

4. **After git add:**
   ```go
   Input:  git add → "Added files to staging area"
   Output: "👉 NEXT: Create commit with: {\"tool\": \"git\", \"args\": {\"action\": \"commit\", ...}}"
   ```

5. **After git commit:**
   ```go
   Input:  git commit → "Created commit abc123"
   Output: "👉 NEXT: Push branch with: {\"tool\": \"git\", \"args\": {\"action\": \"push\", ...}}"
   ```

6. **After git push:**
   ```go
   Input:  git push → "Pushed to origin/feature-branch"
   Output: "👉 NEXT: Create PR with: {\"tool\": \"github\", \"args\": {\"action\": \"pr_create\", ...}}"
   ```

**Implementation:**
```go
func TestBuildFeedbackPrompt_GitWorkflowGuidance(t *testing.T) {
    tracker := NewPhaseTracker()

    tests := []struct {
        name         string
        toolCall     llm.ToolCall
        result       *tools.Result
        wantContains []string
    }{
        {
            name: "git status with untracked files guides to git add",
            toolCall: llm.ToolCall{
                Name: "git",
                Args: map[string]interface{}{"action": "status"},
            },
            result: &tools.Result{
                Success: true,
                Output:  "?? pkg/metrics/\n?? pkg/metrics_test.go\n",
            },
            wantContains: []string{
                "UNTRACKED",
                "NEXT STEP",
                "git add",
                `"action": "add"`,
                `"files"`,
            },
        },
        {
            name: "git add guides to git commit",
            toolCall: llm.ToolCall{
                Name: "git",
                Args: map[string]interface{}{"action": "add", "files": []string{"pkg/metrics/metrics.go"}},
            },
            result: &tools.Result{
                Success: true,
                Output:  "Added pkg/metrics/metrics.go",
            },
            wantContains: []string{
                "NEXT",
                "commit",
                `"action": "commit"`,
                `"message"`,
            },
        },
        {
            name: "git commit guides to git push",
            toolCall: llm.ToolCall{
                Name: "git",
                Args: map[string]interface{}{"action": "commit", "message": "feat: Add metrics"},
            },
            result: &tools.Result{
                Success: true,
                Output:  "[main abc123] feat: Add metrics\n 2 files changed",
            },
            wantContains: []string{
                "NEXT",
                "push",
                `"action": "push"`,
                `"branch"`,
            },
        },
        {
            name: "git push guides to github pr_create",
            toolCall: llm.ToolCall{
                Name: "git",
                Args: map[string]interface{}{"action": "push", "branch": "feat/metrics"},
            },
            result: &tools.Result{
                Success: true,
                Output:  "Pushed to origin/feat/metrics",
            },
            wantContains: []string{
                "NEXT",
                "PR",
                "github",
                `"action": "pr_create"`,
            },
        },
        {
            name: "clean working tree indicates nothing to deliver",
            toolCall: llm.ToolCall{
                Name: "git",
                Args: map[string]interface{}{"action": "status"},
            },
            result: &tools.Result{
                Success: true,
                Output:  "nothing to commit, working tree clean",
            },
            wantContains: []string{
                "clean",
                "No changes to deliver",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            feedback := buildFeedbackPrompt([]llm.ToolCall{tt.toolCall}, []*tools.Result{tt.result}, tracker)

            for _, want := range tt.wantContains {
                assert.Contains(t, feedback, want, "Feedback should guide to next step")
            }
        })
    }
}
```

---

### 3. Tool Tracking Tests

**File:** `pkg/agents/phased_executor_test.go`

#### Test: `TestPhaseTracker_RecordAndDetectLoop`

**Purpose:** Verify loop detection works correctly

**Test Cases:**

1. **No loop (different calls):**
   ```go
   RecordCall("git", {"action": "status"})
   RecordCall("git", {"action": "add"})
   RecordCall("git", {"action": "commit"})
   DetectLoop() → false
   ```

2. **Loop detected (3 identical calls):**
   ```go
   RecordCall("git", {"action": "status"})
   RecordCall("git", {"action": "status"})
   RecordCall("git", {"action": "status"})
   DetectLoop() → true
   ```

3. **Loop with different args (not a loop):**
   ```go
   RecordCall("file", {"path": "a.go"})
   RecordCall("file", {"path": "b.go"})
   RecordCall("file", {"path": "c.go"})
   DetectLoop() → false
   ```

**Implementation:**
```go
func TestPhaseTracker_LoopDetection(t *testing.T) {
    tests := []struct {
        name       string
        calls      []struct {
            tool string
            args map[string]interface{}
        }
        expectLoop bool
    }{
        {
            name: "no loop with different tools",
            calls: []struct {
                tool string
                args map[string]interface{}
            }{
                {"git", map[string]interface{}{"action": "status"}},
                {"git", map[string]interface{}{"action": "add"}},
                {"git", map[string]interface{}{"action": "commit"}},
            },
            expectLoop: false,
        },
        {
            name: "loop detected with 3 identical calls",
            calls: []struct {
                tool string
                args map[string]interface{}
            }{
                {"git", map[string]interface{}{"action": "status"}},
                {"git", map[string]interface{}{"action": "status"}},
                {"git", map[string]interface{}{"action": "status"}},
            },
            expectLoop: true,
        },
        {
            name: "no loop with same tool but different args",
            calls: []struct {
                tool string
                args map[string]interface{}
            }{
                {"file", map[string]interface{}{"path": "a.go"}},
                {"file", map[string]interface{}{"path": "b.go"}},
                {"file", map[string]interface{}{"path": "c.go"}},
            },
            expectLoop: false,
        },
        {
            name: "loop resets after different call",
            calls: []struct {
                tool string
                args map[string]interface{}
            }{
                {"git", map[string]interface{}{"action": "status"}},
                {"git", map[string]interface{}{"action": "status"}},
                {"git", map[string]interface{}{"action": "add"}},
                {"git", map[string]interface{}{"action": "status"}},
            },
            expectLoop: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tracker := NewPhaseTracker()

            for _, call := range tt.calls {
                tracker.RecordCall(call.tool, call.args)
            }

            got := tracker.DetectLoop()
            assert.Equal(t, tt.expectLoop, got)
        })
    }
}
```

---

#### Test: `TestPhaseTracker_RequiredTools`

**Purpose:** Verify required tool tracking

**Test Cases:**

1. **All required tools called:**
   ```go
   required: ["bash", "test"]
   called: ["bash", "test", "file"]
   HasRequiredTools() → true
   ```

2. **Missing required tool:**
   ```go
   required: ["bash", "test", "lsp"]
   called: ["bash", "test"]
   HasRequiredTools() → false
   ```

3. **No required tools:**
   ```go
   required: []
   called: ["git"]
   HasRequiredTools() → true (vacuously true)
   ```

**Implementation:**
```go
func TestPhaseTracker_RequiredTools(t *testing.T) {
    tests := []struct {
        name         string
        required     []string
        called       []string
        expectResult bool
    }{
        {
            name:         "all required tools called",
            required:     []string{"bash", "test"},
            called:       []string{"bash", "test", "file", "lsp"},
            expectResult: true,
        },
        {
            name:         "missing required tool",
            required:     []string{"bash", "test", "lsp"},
            called:       []string{"bash", "test"},
            expectResult: false,
        },
        {
            name:         "no required tools is always satisfied",
            required:     []string{},
            called:       []string{"git", "github"},
            expectResult: true,
        },
        {
            name:         "required tool called multiple times still counts",
            required:     []string{"bash"},
            called:       []string{"bash", "bash", "bash"},
            expectResult: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tracker := NewPhaseTracker()

            for _, tool := range tt.called {
                tracker.RecordCall(tool, map[string]interface{}{})
            }

            got := tracker.HasRequiredTools(tt.required)
            assert.Equal(t, tt.expectResult, got)
        })
    }
}
```

---

### 4. Git Status Interpretation Tests

**File:** `pkg/agents/phased_executor_test.go`

#### Test: `TestInterpretGitStatus_AllScenarios`

**Purpose:** Verify all git status output formats are handled

**Implementation:**
```go
func TestInterpretGitStatus_AllFormats(t *testing.T) {
    tests := []struct {
        name         string
        gitOutput    string
        wantContains []string
    }{
        {
            name:      "untracked files",
            gitOutput: "?? pkg/metrics/\n?? test.go\n",
            wantContains: []string{
                "UNTRACKED",
                "git add",
                "pkg/metrics/",
            },
        },
        {
            name:      "modified unstaged",
            gitOutput: " M pkg/server.go\n M cmd/main.go\n",
            wantContains: []string{
                "MODIFIED",
                "git add",
            },
        },
        {
            name:      "staged files",
            gitOutput: "A  pkg/metrics/metrics.go\nA  pkg/metrics/metrics_test.go\n",
            wantContains: []string{
                "STAGED",
                "commit",
            },
        },
        {
            name:      "clean working tree",
            gitOutput: "nothing to commit, working tree clean",
            wantContains: []string{
                "clean",
                "No changes",
            },
        },
        {
            name:      "mixed status",
            gitOutput: " M pkg/server.go\n?? test.go\nA  new_file.go\n",
            wantContains: []string{
                "MODIFIED",
                "UNTRACKED",
                "git add",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            guidance := interpretGitStatus(tt.gitOutput)

            for _, want := range tt.wantContains {
                assert.Contains(t, guidance, want)
            }
        })
    }
}
```

---

## Integration Tests

**File:** `pkg/agents/phased_executor_integration_test.go`

### Test: `TestPhaseTransition_ImplementToValidate_NoJSONLeakage`

**Purpose:** Verify full phase transition with sanitization

**Setup:**
```go
func TestPhaseTransition_ImplementToValidate(t *testing.T) {
    // Create mock Implement phase result
    implementResult := &PhaseResult{
        PhaseName: "implement",
        Output: `# Implementation Phase Complete

Created pkg/metrics package with Prometheus counters.

## Files Created:
- pkg/metrics/metrics.go
- pkg/metrics/metrics_test.go

## LSP Diagnostics (example):
` + "```json\n{\"tool\": \"lsp\", \"args\": {\"operation\": \"diagnostics\", \"file\": \"pkg/metrics/metrics.go\"}}\n```" + `

All checks passed. PHASE_COMPLETE`,
        ModifiedFiles: []string{
            "pkg/metrics/metrics.go",
            "pkg/metrics/metrics_test.go",
        },
    }

    // Build input for Validate phase
    validateInput := buildNextPhaseInput(implementResult)

    // Assertions
    t.Run("no json leakage", func(t *testing.T) {
        assert.NotContains(t, validateInput, `{"tool":`)
        assert.NotContains(t, validateInput, `"lsp"`)
        assert.NotContains(t, validateInput, `"operation": "diagnostics"`)
    })

    t.Run("includes summary", func(t *testing.T) {
        assert.Contains(t, validateInput, "Previous Phase: implement")
        assert.Contains(t, validateInput, "PHASE_COMPLETE")
    })

    t.Run("includes modified files", func(t *testing.T) {
        assert.Contains(t, validateInput, "Modified Files")
        assert.Contains(t, validateInput, "pkg/metrics/metrics.go")
        assert.Contains(t, validateInput, "pkg/metrics/metrics_test.go")
    })

    t.Run("does not include procedural instructions", func(t *testing.T) {
        assert.NotContains(t, validateInput, "## Files Created:")
        assert.NotContains(t, validateInput, "## LSP Diagnostics (example):")
    })
}
```

---

### Test: `TestValidatePhase_RequiresToolCalls`

**Purpose:** Verify Validate phase fails without required tools

**Setup:**
```go
func TestValidatePhase_MustCallRequiredTools(t *testing.T) {
    // Create test executor
    cfg := &config.Config{
        Limits: config.LimitsConfig{
            MaxInferenceRuns: 5,
        },
    }

    backend := &mockLLMBackend{
        responses: []string{
            "PHASE_COMPLETE", // Tries to complete immediately
        },
    }

    agent := NewCodingBaseAgent("test", "test", cfg, backend, nil)
    contextMgr, _ := llmcontext.NewManager("test-job", false, 8000)

    validatePhase := Phase{
        Name:          "validate",
        RequiredTools: []string{"bash", "test"},
        MaxRounds:     5,
    }

    executor := NewPhasedExecutor(agent, contextMgr, []Phase{validatePhase})

    // Execute
    _, err := executor.executePhase(context.Background(), validatePhase, "Validate the code")

    // Should fail because no tools were called
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "required validation tools not executed")
    assert.Contains(t, err.Error(), "bash")
    assert.Contains(t, err.Error(), "test")
}
```

---

### Test: `TestDeliverPhase_DetectsLoop`

**Purpose:** Verify Deliver phase detects and fails on infinite loops

**Setup:**
```go
func TestDeliverPhase_FailsOnGitStatusLoop(t *testing.T) {
    // Create test executor
    cfg := &config.Config{
        Limits: config.LimitsConfig{
            MaxInferenceRuns: 5,
        },
    }

    backend := &mockLLMBackend{
        // Mock LLM that keeps calling git status
        toolCalls: []llm.ToolCall{
            {Name: "git", Args: map[string]interface{}{"action": "status"}},
        },
        repeatCount: 5, // Call git status 5 times
    }

    agent := NewCodingBaseAgent("test", "test", cfg, backend, nil)
    contextMgr, _ := llmcontext.NewManager("test-job", false, 8000)

    deliverPhase := Phase{
        Name:      "deliver",
        MaxRounds: 5,
    }

    executor := NewPhasedExecutor(agent, contextMgr, []Phase{deliverPhase})

    // Execute
    _, err := executor.executePhase(context.Background(), deliverPhase, "Deliver the code")

    // Should fail due to loop detection
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "loop") // OR "repeated tool calls" OR "max rounds"
}
```

---

## Test Utilities

**File:** `pkg/agents/phased_executor_testutil.go`

```go
package agents

// Mock LLM backend for testing
type mockLLMBackend struct {
    responses   []string // Text responses to return
    toolCalls   []llm.ToolCall // Tool calls to return
    repeatCount int      // How many times to repeat toolCalls
    callCount   int      // Track how many times Infer was called
}

func (m *mockLLMBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
    m.callCount++

    response := &llm.InferenceResponse{
        Text: "",
        ToolCalls: nil,
    }

    // Return text response if available
    if len(m.responses) > 0 {
        idx := m.callCount - 1
        if idx >= len(m.responses) {
            idx = len(m.responses) - 1 // Repeat last response
        }
        response.Text = m.responses[idx]
    }

    // Return tool calls if configured
    if len(m.toolCalls) > 0 && (m.repeatCount == 0 || m.callCount <= m.repeatCount) {
        response.ToolCalls = m.toolCalls
    }

    return response, nil
}

// Helper to create test PhaseResult
func newTestPhaseResult(name, output string, files []string) *PhaseResult {
    return &PhaseResult{
        PhaseName:     name,
        Success:       true,
        Output:        output,
        ModifiedFiles: files,
    }
}

// Helper to create test Phase
func newTestPhase(name string, required []string, maxRounds int) Phase {
    return Phase{
        Name:          name,
        RequiredTools: required,
        MaxRounds:     maxRounds,
    }
}
```

---

## Running Tests

```bash
# Run all unit tests
go test ./pkg/agents/ -v -run Test.*Phased.*

# Run specific test
go test ./pkg/agents/ -v -run TestBuildNextPhaseInput_RemovesJSONExamples

# Run with coverage
go test ./pkg/agents/ -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run integration tests
go test ./pkg/agents/ -v -run TestPhaseTransition.*

# Run all tests in parallel
go test ./pkg/agents/ -v -parallel 4
```

---

## Success Criteria

✅ All unit tests pass
✅ Integration tests pass
✅ Code coverage > 80% for new functions
✅ Tests catch all known bugs (Bug #11, Bug #12)
✅ Tests prevent regressions

---

## Implementation Order

1. **Phase 1: Extract testable functions**
   - Move `buildNextPhaseInput` to separate file
   - Extract `sanitizePhaseOutput`
   - Extract `interpretGitStatus`
   - Extract `buildFeedbackPrompt`

2. **Phase 2: Write unit tests**
   - Test prompt sanitization
   - Test git status interpretation
   - Test feedback generation
   - Test tool tracking

3. **Phase 3: Implement PhaseTracker**
   - Add tool call recording
   - Add loop detection
   - Add required tool checking

4. **Phase 4: Write integration tests**
   - Test phase transitions
   - Test validation enforcement
   - Test loop detection in full workflow

5. **Phase 5: Verify with Test 6 reproduction**
   - Re-run Test 6 scenario
   - Verify both bugs are fixed
   - Confirm tests catch the issues
