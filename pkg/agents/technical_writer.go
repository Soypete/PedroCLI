package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

type TechnicalWriterAgent struct {
	*BaseAgent
	cfg        *config.Config
	backend    llm.Backend
	jobManager jobs.JobManager
	jobID      string
}

func NewTechnicalWriterAgent(cfg *config.Config, backend llm.Backend, jobManager jobs.JobManager) *TechnicalWriterAgent {
	base := NewBaseAgent(
		"technical_writer",
		"Creates technical documentation, tutorials, and guides",
		cfg,
		backend,
		jobManager,
	)

	agent := &TechnicalWriterAgent{
		BaseAgent:  base,
		cfg:        cfg,
		backend:    backend,
		jobManager: jobManager,
	}

	agent.registerTools()

	return agent
}

func (a *TechnicalWriterAgent) registerTools() {
	searchTool := tools.NewWebSearchTool()
	a.RegisterTool(searchTool)

	scraperTool := tools.NewWebScraperTool()
	a.RegisterTool(scraperTool)

	workDir := "."
	if a.cfg != nil && a.cfg.Project.Workdir != "" {
		workDir = a.cfg.Project.Workdir
	}
	codeSearchTool := tools.NewSearchTool(workDir)
	a.RegisterTool(codeSearchTool)

	fileTool := tools.NewFileTool()
	a.RegisterTool(fileTool)
}

func (a *TechnicalWriterAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	content := getStringArg(input, "content", "")
	title := getStringArg(input, "title", "")
	contentType := getStringArg(input, "type", "technical")

	job, err := a.jobManager.Create(ctx, "technical_writer", title, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	a.jobID = job.ID
	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	go func() {
		bgCtx := context.Background()

		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled, a.config.Model.ContextSize)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		contextMgr.SetModelName(a.config.Model.ModelName)

		result, err := a.writeContent(bgCtx, contextMgr, content, title, contentType)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, result, nil)
	}()

	return job, nil
}

func (a *TechnicalWriterAgent) writeContent(ctx context.Context, contextMgr *llmcontext.Manager, content, title, contentType string) (map[string]interface{}, error) {
	systemPrompt := a.buildSystemPrompt(contentType)
	userPrompt := a.buildUserPrompt(content, title, contentType)

	executor := NewInferenceExecutor(a.BaseAgent, contextMgr)
	executor.SetSystemPrompt(systemPrompt)

	ApplyModeConstraintsToExecutor(executor, "technical_writer", a.config.Modes)

	err := executor.Execute(ctx, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Get final response from context manager
	history, err := contextMgr.GetHistory()
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %w", err)
	}

	output := strings.TrimSpace(history)
	if idx := strings.Index(output, "TASK_COMPLETE"); idx >= 0 {
		output = strings.TrimSpace(output[idx+len("TASK_COMPLETE"):])
	}

	return map[string]interface{}{
		"content": output,
		"title":   title,
		"type":    contentType,
	}, nil
}

func (a *TechnicalWriterAgent) buildSystemPrompt(contentType string) string {
	var prompt string

	switch contentType {
	case "tutorial":
		prompt = `You are a technical writer specializing in programming tutorials.
Create clear, step-by-step tutorials that are easy to follow.
Structure your content with:
- Introduction (what you'll learn, prerequisites)
- Step-by-step instructions
- Code examples where applicable
- Summary

`
	case "guide":
		prompt = `You are a technical writer creating comprehensive guides.
Cover all aspects of the topic thoroughly.
Structure with:
- Overview, detailed sections, examples, best practices, troubleshooting

`
	case "podcast_script":
		prompt = `You are a technical podcast script writer.
Create engaging scripts with segment markers:
- [INTRO], [MAIN], [TRANSITION], [OUTRO]
Keep it conversational but informative.

`
	default:
		prompt = `You are a technical writer creating high-quality technical content.
Structure with: hook, core concepts, examples, conclusion with CTA.

`
	}

	prompt += `Available tools: search, web_scraper, file, code_search

When complete, respond with "TASK_COMPLETE" followed by your content.`

	return prompt
}

func (a *TechnicalWriterAgent) buildUserPrompt(content, title, contentType string) string {
	prompt := fmt.Sprintf("Create %s content", contentType)
	if title != "" {
		prompt += fmt.Sprintf(" titled: %s", title)
	}
	prompt += fmt.Sprintf("\n\nContent to work from:\n%s\n\nWrite the complete content now.", content)
	return prompt
}

func getStringArg(args map[string]interface{}, key, defaultValue string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}
