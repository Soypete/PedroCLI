# Test 7 Plan: Validate Prompt Improvements

**Date:** 2026-01-13
**Phase:** Phase 1 - Prompt Updates
**Related:** `test-6-analysis-phased-workflow-bugs.md`

## Changes Made

### 1. Validate Phase Prompt (`builder_phased_validate.md`)

**Changes:**
- ✅ Added "⚠️ CRITICAL: REQUIRED VALIDATION STEPS" section at top
- ✅ Made build + test execution REQUIRED before PHASE_COMPLETE
- ✅ Added explicit warning: "YOU CANNOT SKIP THESE STEPS"
- ✅ Emphasized: "You must actually execute these tool calls... Simply claiming 'I already validated' is NOT acceptable"
- ✅ Added completion checklist with checkboxes

**Expected Impact:**
- Prevents hallucination (Bug #12)
- Forces agent to run validation tools
- Makes it clear that previous phase documentation doesn't count

### 2. Deliver Phase Prompt (`builder_phased_deliver.md`)

**Changes:**
- ✅ Added "⚠️ Understanding Git Status Output" table with symbols
- ✅ Added "⚠️ Sequential Workflow - DO NOT REPEAT STEPS" section
- ✅ Added step-by-step next actions (status → add → commit → push → PR)
- ✅ Added "DO THIS ONCE" and "DO THIS AFTER X" annotations
- ✅ Added "Common Mistakes to Avoid" section
- ✅ Added loop detection guidance: "If you're stuck in a loop... STOP and analyze"

**Expected Impact:**
- Agent understands `??` symbol (Bug #11)
- Sequential enforcement prevents loops
- Clear next-step guidance

---

## Test 7 Execution Plan

### Objective
Validate that prompt improvements fix Bug #11 (Deliver loop) and Bug #12 (Validate skip).

### Test Scenario
**Same as Test 6:** Issue #32 - Implement Prometheus observability metrics

### Steps

#### 1. Prepare Environment

```bash
# Ensure you're on the updated branch
git status
git log -1  # Should show commit 831b7dd "fix(agents): Add enforcement..."

# Clean up Test 6 artifacts if any
rm -rf pkg/metrics/  # Remove untracked files from Test 6
git status  # Should be clean
```

#### 2. Run Test 7

```bash
# Run the same command as Test 6
./pedrocli build \
  -issue "32" \
  -description "Implement Prometheus observability metrics. Create pkg/metrics package with HTTP, job, LLM, and tool metrics. Instrument server.go, handlers.go. Add /metrics endpoint to HTTP bridge. Write tests. Create PR when done."
```

#### 3. Monitor Execution

**Watch for these phases:**

**Analyze Phase (should pass):**
- ✅ Explores codebase
- ✅ Identifies files to modify
- ✅ Completes with PHASE_COMPLETE

**Plan Phase (should pass):**
- ✅ Creates implementation plan
- ✅ Breaks down into steps
- ✅ Completes with PHASE_COMPLETE

**Implement Phase (should pass - worked in Test 6):**
- ✅ Creates `pkg/metrics/metrics.go`
- ✅ Creates `pkg/metrics/metrics_test.go`
- ✅ May claim to run LSP in text (that's okay, Validate will actually run it)
- ✅ Completes with PHASE_COMPLETE

**Validate Phase (CRITICAL - Bug #12 test):**
- ⚠️ **Expected Fix:** Should call `bash` (go build) AND `test` tools
- ⚠️ **Should NOT:** Complete in 1 round with 0 tool calls
- ✅ Should run: `go build ./...`
- ✅ Should run: `go test` or test tool
- ✅ Should show actual test results
- ✅ May run: LSP diagnostics (recommended but not required)

**Deliver Phase (CRITICAL - Bug #11 test):**
- ⚠️ **Expected Fix:** Should NOT loop on `git status`
- ⚠️ **Should call exactly 5 tools:** git status (1x) → git add → git commit → git push → github pr_create
- ✅ Round 1: `git status` → sees `?? pkg/metrics/`
- ✅ Round 2: `git add` → stages files
- ✅ Round 3: `git commit` → creates commit
- ✅ Round 4: `git push` → pushes to remote
- ✅ Round 5: `github pr_create` → creates draft PR
- ✅ Completes with PHASE_COMPLETE and PR URL

#### 4. Collect Evidence

**Check job context directory:**
```bash
# Find the job ID from the CLI output
JOB_ID="job-<timestamp>"

# Navigate to job context
cd /tmp/pedrocli-jobs/$JOB_ID

# Count phases and rounds
ls -la *.txt | wc -l  # Total files

# Check Validate phase
grep -l "Validate" *-prompt.txt  # Find Validate phase files
# Read the files around Validate phase
```

**Validate Phase Evidence:**
```bash
# Find Validate phase (usually around files 013-014 in Test 6)
# Look for phase transition from Implement to Validate

# Check if tools were called
grep -c '"name": "bash"' *-tool-calls.json  # Should be > 0
grep -c '"name": "test"' *-tool-calls.json  # Should be > 0

# Check round count
# Validate phase should use 2-5 rounds (not just 1)
```

**Deliver Phase Evidence:**
```bash
# Find Deliver phase (usually starts around file 015 in Test 6)

# Count git status calls
grep -c '"action": "status"' *-tool-calls.json  # Should be exactly 1

# Check tool sequence
grep '"name"' *-tool-calls.json | grep -A1 -B1 git  # Should show git, git, github

# Verify no loops
# Should NOT see 5 consecutive git status calls
```

#### 5. Check Git Repository

```bash
# Return to project root
cd /Users/miriahpeterson/Code/go-projects/pedrocli

# Check git status
git status
# Should see a new branch like feat/32-prometheus-metrics or similar

# Check git log
git log -1
# Should see a commit about Prometheus metrics

# Check for PR (if Deliver succeeded)
gh pr list
# Should see a draft PR

# Check modified files
ls -la pkg/metrics/
# Should see metrics.go and metrics_test.go

# Check git diff
git diff main
# Should show the new files
```

---

## Success Criteria

### ✅ Phase 1 Success (Prompt Fixes)

**Validate Phase:**
- [ ] Calls `bash` tool with `go build ./...` command
- [ ] Calls `test` tool to run tests
- [ ] Completes in 2-5 rounds (not 1 round)
- [ ] Does NOT claim validation without running tools

**Deliver Phase:**
- [ ] Calls `git status` exactly ONCE (not 5 times)
- [ ] Calls `git add` after git status
- [ ] Calls `git commit` after git add
- [ ] Calls `git push` after git commit
- [ ] Calls `github pr_create` after git push
- [ ] Completes within 5 rounds
- [ ] Creates actual PR on GitHub

**Overall:**
- [ ] Job completes successfully (no "max rounds reached" error)
- [ ] PR is created
- [ ] All files are committed and pushed

---

## Expected Results

### If Prompts Work (Success)

**Console Output:**
```
📋 Phase 1/5: analyze
   Analyze the request, evaluate repo state, gather requirements
   ✅ Phase analyze completed in 3 rounds

📋 Phase 2/5: plan
   Create a detailed implementation plan with numbered steps
   ✅ Phase plan completed in 2 rounds

📋 Phase 3/5: implement
   Write code following the plan, chunk by chunk
   ✅ Phase implement completed in 6 rounds

📋 Phase 4/5: validate
   Run tests, linter, verify the implementation works
   🔄 Round 1/15
   🔧 bash
   ✅ bash
   🔄 Round 2/15
   🔧 test
   ✅ test
   ✅ Phase validate completed in 2 rounds

📋 Phase 5/5: deliver
   Commit changes and create draft PR
   🔄 Round 1/5
   🔧 git
   ✅ git
   🔄 Round 2/5
   🔧 git
   ✅ git
   🔄 Round 3/5
   🔧 git
   ✅ git
   🔄 Round 4/5
   🔧 git
   ✅ git
   🔄 Round 5/5
   🔧 github
   ✅ github
   ✅ Phase deliver completed in 5 rounds

✅ All 5 phases completed successfully!
```

### If Prompts Don't Work (Failure)

**Validate Phase Failure:**
```
📋 Phase 4/5: validate
   🔄 Round 1/15
   ✅ Phase validate completed in 1 rounds  # ❌ Too fast, no tools called
```

**Deliver Phase Failure:**
```
📋 Phase 5/5: deliver
   🔄 Round 1/5
   🔧 git
   ✅ git
   🔄 Round 2/5
   🔧 git  # ❌ git again (loop)
   ✅ git
   🔄 Round 3/5
   🔧 git  # ❌ git again (loop)
   ✅ git
   ...
Error: phase deliver failed: max rounds (5) reached without phase completion
```

---

## Failure Analysis

### If Validate Phase Still Skips

**Possible causes:**
1. Agent ignoring prompt instructions
2. Context pollution still happening (need Phase 2 fix)
3. Need code-level enforcement (Phase 3)

**Next steps:**
- Move to Phase 2: Extract testable functions
- Implement code-level validation tracking

### If Deliver Phase Still Loops

**Possible causes:**
1. Agent not reading the git status guide
2. Generic feedback still not guiding well (need Phase 2 fix)
3. Need state-aware feedback (Phase 3)

**Next steps:**
- Move to Phase 2: Implement interpretGitStatus function
- Add state-aware feedback in buildFeedbackPrompt

---

## Post-Test Actions

### If Test 7 Succeeds

1. **Document success:**
   ```bash
   echo "Test 7: SUCCESS - Both bugs fixed with prompts" >> docs/learnings/test-results.md
   ```

2. **Clean up:**
   ```bash
   # Merge the generated PR or close it
   gh pr close <pr-number>

   # Reset branch if needed
   git reset --hard main
   git clean -fd
   ```

3. **Decision point:**
   - If bugs are fully fixed: Skip Phase 2 & 3, move to next feature
   - If partially fixed: Continue to Phase 2 for robustness
   - If not fixed: Immediately start Phase 2

### If Test 7 Fails

1. **Capture failure evidence:**
   ```bash
   # Copy job context for analysis
   JOB_ID="<failed-job-id>"
   cp -r /tmp/pedrocli-jobs/$JOB_ID ~/test-7-failure/

   # Document which phase failed
   echo "Test 7: FAILED - <phase> still has issues" >> docs/learnings/test-results.md
   ```

2. **Analyze failure:**
   - Read the job context files
   - Identify if it's the same bug or a new issue
   - Check if agent even saw the new prompt instructions

3. **Proceed to Phase 2:**
   - Cannot rely on prompts alone
   - Need code-level enforcement
   - Move forward with architectural changes

---

## Metrics to Track

| Metric | Test 6 | Test 7 Target |
|--------|--------|---------------|
| Validate rounds | 1 | 2-5 |
| Validate tool calls | 0 | ≥2 (bash + test) |
| Deliver rounds | 5 (failed) | 5 (success) |
| Deliver git status calls | 5 | 1 |
| Overall success | Failed | Completed |
| PR created | No | Yes |

---

## Next Steps After Test 7

**If successful:**
- Commit test results
- Update learnings document
- Decide: Continue to Phase 2 for robustness, or skip to next feature

**If failed:**
- Document failure mode
- Start Phase 2 immediately
- Extract testable functions
- Add code-level enforcement

---

## Running the Test

**You can run Test 7 now with:**

```bash
./pedrocli build \
  -issue "32" \
  -description "Implement Prometheus observability metrics. Create pkg/metrics package with HTTP, job, LLM, and tool metrics. Instrument server.go, handlers.go. Add /metrics endpoint to HTTP bridge. Write tests. Create PR when done."
```

**Watch the console output and look for the patterns described above.**
