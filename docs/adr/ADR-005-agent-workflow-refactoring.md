# ADR-005: Agent Workflow Refactoring

## Status

Proposed

## Context

PedroCLI has multiple agent types with different workflow patterns:

### Code Agents (Dynamic)

- **Builder** (`pkg/agents/builder.go`)
- **Debugger** (`pkg/agents/debugger.go`)
- **Reviewer** (`pkg/agents/reviewer.go`)
- **Triager** (`pkg/agents/triager.go`)

These use `InferenceExecutor` with dynamic tool invocation.

### Content Agents (Rigid)

- **BlogOrchestrator** (`pkg/agents/blog_orchestrator.go`) - 7 hardcoded phases
- **Podcast** (`pkg/agents/podcast.go`) - Similar rigid structure

These use procedural phase execution with LLM calls at each phase.

### Problem

1. **Inconsistent Patterns**: Two fundamentally different execution models.

2. **Code Duplication**: Phase logic duplicates what tools could provide.

3. **Limited Flexibility**: Content agents can't adapt to request complexity.

4. **Maintenance Burden**: Changes require updating both patterns.

## Decision

### 1. Unified Agent Architecture

Refactor all agents to use `DynamicExecutor` from ADR-003:

```go
// pkg/agents/agent.go

type Agent interface {
    Name() string
    Description() string
    Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error)
}

type DynamicAgent struct {
    name        string
    description string
    config      *config.Config
    backend     llm.Backend
    jobManager  *jobs.Manager
    registry    *tools.ToolRegistry
    completion  *CompletionCriteria
    promptTmpl  string
}

func (a *DynamicAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
    job, err := a.jobManager.Create(a.name, a.description, input)
    if err != nil {
        return nil, err
    }

    go func() {
        executor := NewDynamicExecutor(a, a.registry)
        executor.SetCompletionCriteria(a.completion)

        result, err := executor.Execute(context.Background(), a.buildPrompt(input))
        // Handle result...
    }()

    return job, nil
}
```

### 2. Blog Agent Refactoring

Convert blog orchestrator to dynamic pattern:

```go
// pkg/agents/blog.go

func NewBlogAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *DynamicAgent {
    registry := tools.NewToolRegistry()

    // Register orchestration tools (converted from phases)
    registry.Register(blog.NewAnalyzePromptTool(backend))
    registry.Register(blog.NewGenerateOutlineTool(backend))
    registry.Register(blog.NewExpandSectionTool(backend))
    registry.Register(blog.NewAssembleContentTool())
    registry.Register(blog.NewGenerateSocialPostsTool(backend))

    // Register research tools
    registry.Register(tools.NewRSSFeedTool(cfg))
    registry.Register(tools.NewStaticLinksTool(cfg))
    if cfg.Blog.CalendarEnabled {
        registry.Register(tools.NewCalendarTool(cfg))
    }

    // Register publishing tools
    if cfg.Blog.NotionEnabled {
        registry.Register(tools.NewBlogNotionTool(cfg))
    }

    return &DynamicAgent{
        name:        "blog",
        description: "Create blog content with optional research and publishing",
        config:      cfg,
        backend:     backend,
        jobManager:  jobMgr,
        registry:    registry,
        completion:  blogCompletionCriteria(),
        promptTmpl:  blogPromptTemplate,
    }
}

func blogCompletionCriteria() *CompletionCriteria {
    return &CompletionCriteria{
        MinimumArtifacts: []string{"content"},
        ExplicitSignal:   "CONTENT_COMPLETE",
        CustomCheck: func(state *ExecutionState) (bool, error) {
            content, _ := state.Artifacts["content"].(string)
            return len(content) >= 500, nil
        },
    }
}

const blogPromptTemplate = `
You are a blog content creator. Create content based on this request:

{{.Prompt}}

## Available Tools

### Content Creation (Core)
- analyze_prompt: Understand complex prompts. Use for detailed requests.
- generate_outline: Create structured outline. Use for long-form content.
- expand_section: Write one section. Use iteratively for each section.
- assemble_content: Combine sections into final post.

### Research (Optional)
- rss_feed: Get recent posts for references and links.
- calendar: Get upcoming events to mention.
- static_links: Get social media links for newsletter section.

### Publishing (Optional)
- blog_notion: Publish to Notion drafts database.
- generate_social_posts: Create promotional posts for social media.

## Workflow Guidance

For simple requests (< 500 words):
1. Write content directly
2. Add social posts if relevant

For complex requests (multi-section, newsletter):
1. Analyze the prompt
2. Gather research (if needed)
3. Generate outline
4. Expand each section
5. Assemble final content
6. Generate social posts
7. Publish if requested

Respond with "CONTENT_COMPLETE" when the content is finalized.
`
```

### 3. Orchestration Tools

Convert blog phases to standalone tools:

```go
// pkg/tools/blog/analyze.go

type AnalyzePromptTool struct {
    backend llm.Backend
}

func (t *AnalyzePromptTool) Name() string { return "analyze_prompt" }

func (t *AnalyzePromptTool) Description() string {
    return "Analyze a complex blog prompt to identify structure and research needs"
}

func (t *AnalyzePromptTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "content",
        Optionality: tools.ToolOptional,
        UsageHint:   "Use for complex prompts with multiple topics or sections",
        Produces:    []string{"analysis"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "prompt": {Type: "string", Description: "The blog prompt to analyze"},
            },
            Required: []string{"prompt"},
        },
    }
}

func (t *AnalyzePromptTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
    prompt := args["prompt"].(string)

    // Use LLM to analyze (simplified)
    analysis, err := t.analyzeWithLLM(ctx, prompt)
    if err != nil {
        return tools.ErrorResult(err.Error()), nil
    }

    analysisJSON, _ := json.Marshal(analysis)

    return &tools.Result{
        Success: true,
        Output:  string(analysisJSON),
        Data: map[string]interface{}{
            "analysis": analysis,
        },
    }, nil
}

// pkg/tools/blog/outline.go

type GenerateOutlineTool struct {
    backend llm.Backend
}

func (t *GenerateOutlineTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "content",
        Optionality: tools.ToolOptional,
        UsageHint:   "Use for posts with 3+ sections. Provides structure for expand_section.",
        Consumes:    []string{"analysis"},  // Can use analysis if available
        Produces:    []string{"outline"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "topic":        {Type: "string", Description: "Main topic for the outline"},
                "sections":     {Type: "array", Description: "Suggested sections (optional)"},
                "research":     {Type: "object", Description: "Research data to incorporate (optional)"},
                "target_words": {Type: "integer", Description: "Target word count (optional)"},
            },
            Required: []string{"topic"},
        },
    }
}

// pkg/tools/blog/expand.go

type ExpandSectionTool struct {
    backend llm.Backend
}

func (t *ExpandSectionTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "content",
        Optionality: tools.ToolOptional,
        UsageHint:   "Expands one outline section into full prose. Call per section.",
        Consumes:    []string{"outline"},
        Produces:    []string{"section_content"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "section_title": {Type: "string", Description: "The section to expand"},
                "outline":       {Type: "string", Description: "Full outline for context"},
                "research":      {Type: "object", Description: "Research data (optional)"},
                "tone":          {Type: "string", Description: "Writing tone (optional)"},
            },
            Required: []string{"section_title"},
        },
    }
}

// pkg/tools/blog/assemble.go

type AssembleContentTool struct{}

func (t *AssembleContentTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "content",
        Optionality: tools.ToolOptional,
        UsageHint:   "Combines expanded sections into final post with transitions.",
        Consumes:    []string{"section_content"},
        Produces:    []string{"content"},
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "sections": {Type: "array", Description: "Array of section content strings"},
                "intro":    {Type: "string", Description: "Introduction (optional)"},
                "outro":    {Type: "string", Description: "Conclusion/CTA (optional)"},
            },
            Required: []string{"sections"},
        },
    }
}
```

### 4. Podcast Agent Refactoring

Apply same pattern to podcast agent:

```go
// pkg/agents/podcast.go

func NewPodcastAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *DynamicAgent {
    registry := tools.NewToolRegistry()

    // Podcast-specific tools
    registry.Register(podcast.NewTranscriptProcessorTool())
    registry.Register(podcast.NewShowNotesGeneratorTool(backend))
    registry.Register(podcast.NewChapterMarkerTool())
    registry.Register(podcast.NewSocialClipsTool(backend))

    // Shared research tools
    registry.Register(tools.NewRSSFeedTool(cfg))
    registry.Register(tools.NewStaticLinksTool(cfg))

    return &DynamicAgent{
        name:        "podcast",
        description: "Process podcast transcripts and generate show content",
        config:      cfg,
        backend:     backend,
        jobManager:  jobMgr,
        registry:    registry,
        completion:  podcastCompletionCriteria(),
        promptTmpl:  podcastPromptTemplate,
    }
}
```

### 5. Code Agent Standardization

Update code agents to use registry:

```go
// pkg/agents/builder.go

func NewBuilderAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *DynamicAgent {
    registry := tools.NewToolRegistry()

    // Core code tools
    registry.Register(tools.NewFileTool())
    registry.Register(tools.NewCodeEditTool())
    registry.Register(tools.NewSearchTool(cfg.Project.Workdir))
    registry.Register(tools.NewNavigateTool(cfg.Project.Workdir))
    registry.Register(tools.NewGitTool(cfg.Project.Workdir))
    registry.Register(tools.NewTestTool(cfg.Project.Workdir))

    // Optional tools
    registry.Register(tools.NewBashTool(cfg, cfg.Project.Workdir))

    return &DynamicAgent{
        name:        "builder",
        description: "Build new features autonomously",
        config:      cfg,
        backend:     backend,
        jobManager:  jobMgr,
        registry:    registry,
        completion:  builderCompletionCriteria(),
        promptTmpl:  builderPromptTemplate,
    }
}

func builderCompletionCriteria() *CompletionCriteria {
    return &CompletionCriteria{
        ExplicitSignal: "TASK_COMPLETE",
        CustomCheck: func(state *ExecutionState) (bool, error) {
            // Must have run tests successfully
            if testResult, ok := state.Artifacts["test_result"]; ok {
                return testResult.(bool), nil
            }
            return false, nil
        },
    }
}
```

### 6. Shared Infrastructure

Extract common patterns:

```go
// pkg/agents/common.go

// PromptBuilder constructs initial prompts from templates
type PromptBuilder struct {
    template string
}

func (b *PromptBuilder) Build(input map[string]interface{}) string {
    tmpl := template.Must(template.New("prompt").Parse(b.template))
    var buf bytes.Buffer
    tmpl.Execute(&buf, input)
    return buf.String()
}

// CompletionChecker validates task completion
type CompletionChecker struct {
    criteria *CompletionCriteria
}

func (c *CompletionChecker) IsComplete(state *ExecutionState) (bool, error) {
    // Check explicit signal
    if c.criteria.ExplicitSignal != "" {
        if strings.Contains(state.LastResponse, c.criteria.ExplicitSignal) {
            return true, nil
        }
    }

    // Check minimum artifacts
    for _, artifact := range c.criteria.MinimumArtifacts {
        if _, ok := state.Artifacts[artifact]; !ok {
            return false, nil
        }
    }

    // Run custom check
    if c.criteria.CustomCheck != nil {
        return c.criteria.CustomCheck(state)
    }

    return false, nil
}
```

### 7. Backward Compatibility

Maintain existing agent names and interfaces:

```go
// pkg/agents/compat.go

// BlogOrchestratorAgent is deprecated, use NewBlogAgent
// Kept for backward compatibility
type BlogOrchestratorAgent = DynamicAgent

func NewBlogOrchestratorAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *BlogOrchestratorAgent {
    return NewBlogAgent(cfg, backend, jobMgr)
}

// Add deprecation warning in Execute
func (a *BlogOrchestratorAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
    log.Println("DEPRECATED: BlogOrchestratorAgent is deprecated, use BlogAgent")
    return a.DynamicAgent.Execute(ctx, input)
}
```

## Consequences

### Positive

1. **Consistent Architecture**: All agents use same execution pattern.

2. **Tool Reuse**: Orchestration logic becomes reusable tools.

3. **Flexible Workflows**: LLM adapts workflow to request.

4. **Easier Testing**: Tools can be tested independently.

5. **Simpler Maintenance**: One pattern to maintain.

6. **Better Observability**: Artifact tracking shows workflow decisions.

### Negative

1. **Migration Risk**: Existing workflows may behave differently.

2. **Tool Explosion**: Many small tools to maintain.

3. **Performance Variance**: Dynamic workflows may take more/fewer steps.

4. **Learning Curve**: Users accustomed to predictable phases.

### Mitigation

1. **Feature Flags**: Gradual rollout with ability to revert.

2. **Comparison Testing**: Run both patterns, compare outputs.

3. **Documentation**: Clear guidance on new workflow behavior.

4. **Guardrails**: Completion criteria ensure quality.

## Implementation

### Phase 1: Infrastructure

1. Create `DynamicAgent` base type
2. Implement `PromptBuilder` and `CompletionChecker`
3. Add integration with `DynamicExecutor`

### Phase 2: Blog Agent

1. Create blog orchestration tools
2. Define blog completion criteria
3. Create `NewBlogAgent` using new pattern
4. Add feature flag for old vs new
5. Compare outputs

### Phase 3: Podcast Agent

1. Create podcast tools
2. Refactor to dynamic pattern
3. Test thoroughly

### Phase 4: Code Agents

1. Update to use registry
2. Standardize completion criteria
3. Verify no regressions

### Phase 5: Cleanup

1. Remove deprecated code
2. Update documentation
3. Remove feature flags

## Migration Path

| Week | Task |
|------|------|
| 1 | Infrastructure (DynamicAgent, tools base) |
| 2 | Blog tools implementation |
| 3 | Blog agent refactoring with feature flag |
| 4 | Testing and comparison |
| 5 | Podcast agent refactoring |
| 6 | Code agent updates |
| 7 | Documentation and cleanup |
| 8 | Production rollout |

## Related ADRs

- **ADR-001**: Dynamic Tool Registry (provides tool infrastructure)
- **ADR-002**: LLM Tool Awareness Protocol (generates tool prompts)
- **ADR-003**: Dynamic Tool Invocation (execution pattern)
- **ADR-004**: Logit-Controlled Tool Calling (ensures valid calls)
