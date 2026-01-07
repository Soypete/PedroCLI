package agents

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

//go:embed prompts/debugger_phased_reproduce.md
var debuggerReproducePrompt string

//go:embed prompts/debugger_phased_investigate.md
var debuggerInvestigatePrompt string

//go:embed prompts/debugger_phased_isolate.md
var debuggerIsolatePrompt string

//go:embed prompts/debugger_phased_fix.md
var debuggerFixPrompt string

//go:embed prompts/debugger_phased_verify.md
var debuggerVerifyPrompt string

//go:embed prompts/debugger_phased_commit.md
var debuggerCommitPrompt string

// DebuggerPhasedAgent debugs issues using a 6-phase workflow
type DebuggerPhasedAgent struct {
	*CodingBaseAgent
	contextTool *tools.ContextTool
}

// NewDebuggerPhasedAgent creates a new phased debugger agent
func NewDebuggerPhasedAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *DebuggerPhasedAgent {
	base := NewCodingBaseAgent(
		"debugger_phased",
		"Debug issues using a structured 6-phase workflow: Reproduce, Investigate, Isolate, Fix, Verify, Commit",
		cfg,
		backend,
		jobMgr,
	)

	contextTool := tools.NewContextTool()
	base.RegisterTool(contextTool)

	return &DebuggerPhasedAgent{
		CodingBaseAgent: base,
		contextTool:     contextTool,
	}
}

// GetPhases returns the workflow phases for the debugger agent
func (d *DebuggerPhasedAgent) GetPhases() []Phase {
	return []Phase{
		{
			Name:         "reproduce",
			Description:  "Reproduce the issue consistently",
			SystemPrompt: debuggerReproducePrompt,
			Tools:        []string{"test", "bash", "file", "search"},
			MaxRounds:    8,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				if result.Output == "" {
					return fmt.Errorf("reproduce phase produced no output")
				}
				return nil
			},
		},
		{
			Name:         "investigate",
			Description:  "Gather evidence about the root cause",
			SystemPrompt: debuggerInvestigatePrompt,
			Tools:        []string{"search", "file", "lsp", "git", "navigate", "context"},
			MaxRounds:    12,
			ExpectsJSON:  false,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "isolate",
			Description:  "Narrow down to the exact root cause",
			SystemPrompt: debuggerIsolatePrompt,
			Tools:        []string{"file", "lsp", "search", "bash", "context"},
			MaxRounds:    10,
			ExpectsJSON:  true,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "fix",
			Description:  "Implement a targeted fix",
			SystemPrompt: debuggerFixPrompt,
			Tools:        []string{"file", "code_edit", "search", "lsp"},
			MaxRounds:    10,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "verify",
			Description:  "Verify the fix works and doesn't break anything",
			SystemPrompt: debuggerVerifyPrompt,
			Tools:        []string{"test", "bash", "lsp", "file", "code_edit"},
			MaxRounds:    15, // Allow iterations for fix adjustments
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
		{
			Name:         "commit",
			Description:  "Commit the fix with a clear message",
			SystemPrompt: debuggerCommitPrompt,
			Tools:        []string{"git"},
			MaxRounds:    3,
			Validator: func(result *PhaseResult) error {
				return nil
			},
		},
	}
}

// Execute executes the phased debugger agent
func (d *DebuggerPhasedAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	// Get description
	description, ok := input["description"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'description' in input")
	}

	// Register research_links tool if provided
	if researchLinks, ok := input["research_links"].([]tools.ResearchLink); ok && len(researchLinks) > 0 {
		plainNotes, _ := input["plain_notes"].(string)
		researchLinksTool := tools.NewResearchLinksToolFromLinks(researchLinks, plainNotes)
		d.RegisterTool(researchLinksTool)
	}

	// Create job
	job, err := d.jobManager.Create(ctx, "debugger_phased", description, input)
	if err != nil {
		return nil, err
	}

	d.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run the phased workflow in background
	go func() {
		bgCtx := context.Background()

		// Create context manager
		contextMgr, err := llmcontext.NewManager(job.ID, d.config.Debug.Enabled, d.config.Model.ContextSize)
		if err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		if err := d.jobManager.SetContextDir(bgCtx, job.ID, contextMgr.GetJobDir()); err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set context_dir: %w", err))
			return
		}

		workDir, err := d.setupWorkDirectory(bgCtx, job.ID, input)
		if err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		if err := d.jobManager.SetWorkDir(bgCtx, job.ID, workDir); err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, fmt.Errorf("failed to set work_dir: %w", err))
			return
		}

		// Build initial prompt
		initialPrompt := d.buildInitialPrompt(input)

		// Create phased executor
		executor := NewPhasedExecutor(d.BaseAgent, contextMgr, d.GetPhases())

		// Execute all phases
		err = executor.Execute(bgCtx, initialPrompt)
		if err != nil {
			d.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		// Collect results
		output := map[string]interface{}{
			"status":        "completed",
			"workflow_type": "debugger_phased",
			"job_dir":       contextMgr.GetJobDir(),
			"work_dir":      workDir,
			"phases":        executor.GetAllResults(),
		}

		d.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, output, nil)
	}()

	return job, nil
}

// buildInitialPrompt builds the initial prompt for the reproduce phase
func (d *DebuggerPhasedAgent) buildInitialPrompt(input map[string]interface{}) string {
	description := input["description"].(string)

	prompt := fmt.Sprintf(`# Bug Report

%s

## Instructions

You are starting a 6-phase debugging workflow:
1. **Reproduce** - Reproduce the issue consistently
2. **Investigate** - Gather evidence about the root cause
3. **Isolate** - Narrow down to the exact root cause
4. **Fix** - Implement a targeted fix
5. **Verify** - Verify the fix works
6. **Commit** - Commit the fix

Start with the **Reproduce** phase:
1. Run the failing test or command
2. Capture the exact error message
3. Verify the issue is reproducible

When you've confirmed reproduction, document:
- Exact steps to reproduce
- Error message/behavior observed
- Expected behavior

Then say PHASE_COMPLETE.`, description)

	// Add optional context
	if errorLog, ok := input["error_log"].(string); ok && errorLog != "" {
		prompt += fmt.Sprintf("\n\n## Error Log\n```\n%s\n```", errorLog)
	}

	if stackTrace, ok := input["stack_trace"].(string); ok && stackTrace != "" {
		prompt += fmt.Sprintf("\n\n## Stack Trace\n```\n%s\n```", stackTrace)
	}

	if failingTest, ok := input["failing_test"].(string); ok && failingTest != "" {
		prompt += fmt.Sprintf("\n\n## Failing Test\n%s", failingTest)
	}

	if reproduction, ok := input["reproduction_steps"].(string); ok && reproduction != "" {
		prompt += fmt.Sprintf("\n\n## Known Reproduction Steps\n%s", reproduction)
	}

	return prompt
}
