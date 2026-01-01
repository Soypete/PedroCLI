package agents

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/jobs"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/llmcontext"
	"github.com/soypete/pedrocli/pkg/tools"
)

//go:embed prompts/blog_dynamic_system.md
var blogDynamicSystemPrompt string

// DynamicBlogAgent is a tool-driven blog creation agent that lets the LLM
// decide which tools to use and when, rather than following rigid phases.
// This implements ADR-003: Dynamic tool selection for blog generation.
type DynamicBlogAgent struct {
	*BaseAgent
}

// NewDynamicBlogAgent creates a new dynamic blog agent
func NewDynamicBlogAgent(cfg *config.Config, backend llm.Backend, jobMgr jobs.JobManager) *DynamicBlogAgent {
	base := NewBaseAgent(
		"blog_dynamic",
		"Dynamic LLM-driven blog creation with tool selection",
		cfg,
		backend,
		jobMgr,
	)

	return &DynamicBlogAgent{
		BaseAgent: base,
	}
}

// RegisterResearchTool registers a research tool (rss_feed, calendar, static_links)
func (a *DynamicBlogAgent) RegisterResearchTool(tool tools.Tool) {
	a.RegisterTool(tool)
}

// RegisterNotionTool registers the Notion publishing tool
func (a *DynamicBlogAgent) RegisterNotionTool(tool tools.Tool) {
	a.RegisterTool(tool)
}

// Execute runs the dynamic blog agent using the standard inference executor
func (a *DynamicBlogAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
	prompt, ok := input["prompt"].(string)
	if !ok {
		// Also accept "dictation" as input key
		prompt, ok = input["dictation"].(string)
		if !ok {
			return nil, fmt.Errorf("missing 'prompt' or 'dictation' in input")
		}
	}

	title, _ := input["title"].(string)
	if title == "" {
		title = "Blog Post"
	}

	// Check if publish is requested
	shouldPublish, _ := input["publish"].(bool)

	// Create job
	job, err := a.jobManager.Create(ctx, "blog_dynamic", "Dynamic Blog: "+title, input)
	if err != nil {
		return nil, err
	}

	a.jobManager.Update(ctx, job.ID, jobs.StatusRunning, nil, nil)

	// Run in background
	go func() {
		bgCtx := context.Background()

		contextMgr, err := llmcontext.NewManager(job.ID, a.config.Debug.Enabled)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}
		defer contextMgr.Cleanup()

		result, err := a.runDynamic(bgCtx, contextMgr, prompt, title, shouldPublish)
		if err != nil {
			a.jobManager.Update(bgCtx, job.ID, jobs.StatusFailed, nil, err)
			return
		}

		a.jobManager.Update(bgCtx, job.ID, jobs.StatusCompleted, result, nil)
	}()

	return job, nil
}

// runDynamic executes the dynamic inference loop
func (a *DynamicBlogAgent) runDynamic(ctx context.Context, contextMgr *llmcontext.Manager, prompt, title string, shouldPublish bool) (map[string]interface{}, error) {
	// Create executor with model-specific formatting
	modelName := a.config.Model.ModelName
	executor := NewInferenceExecutorWithModel(a.BaseAgent, contextMgr, modelName)
	executor.SetSystemPrompt(blogDynamicSystemPrompt)

	// Build initial prompt
	initialPrompt := a.buildInitialPrompt(prompt, title, shouldPublish)

	// Run the inference loop - LLM decides which tools to use
	err := executor.Execute(ctx, initialPrompt)
	if err != nil {
		return nil, err
	}

	// Read the final content from the last response file
	finalContent := a.readLastResponse(contextMgr.GetJobDir())

	// Build output from result
	output := map[string]interface{}{
		"status":  "completed",
		"content": finalContent,
		"job_dir": contextMgr.GetJobDir(),
		"title":   title,
		"prompt":  prompt,
	}

	return output, nil
}

// readLastResponse reads the last response file from the job directory
func (a *DynamicBlogAgent) readLastResponse(jobDir string) string {
	// Find all response files
	files, err := os.ReadDir(jobDir)
	if err != nil {
		return ""
	}

	var responseFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "-response.txt") {
			responseFiles = append(responseFiles, f.Name())
		}
	}

	if len(responseFiles) == 0 {
		return ""
	}

	// Sort to get the last one
	sort.Strings(responseFiles)
	lastFile := responseFiles[len(responseFiles)-1]

	// Read the file
	content, err := os.ReadFile(filepath.Join(jobDir, lastFile))
	if err != nil {
		return ""
	}

	return string(content)
}

// buildInitialPrompt constructs the initial prompt for the LLM
func (a *DynamicBlogAgent) buildInitialPrompt(prompt, title string, shouldPublish bool) string {
	publishInstructions := ""
	if shouldPublish {
		publishInstructions = `

After completing the blog post, use the blog_publish tool to publish it to Notion.`
	} else {
		publishInstructions = `

Do NOT publish the blog post. Just write the content and return it.`
	}

	return fmt.Sprintf(`Write a blog post based on the following instructions:

## Title
%s

## Instructions
%s

## Requirements
1. Analyze what the user is asking for
2. If research would be helpful (calendar events, RSS posts, static links), use the appropriate tools
3. Write a complete, polished blog post
4. Include social media posts (Twitter, LinkedIn, Bluesky) at the end
%s

When the blog post is complete, output CONTENT_COMPLETE followed by the final content.`, title, prompt, publishInstructions)
}
