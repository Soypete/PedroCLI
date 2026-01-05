# ADR-003: Dynamic Tool Invocation Pattern

## Status

Implemented (v0.3.0)

## Context

### Current Workflow Patterns

PedroCLI has two distinct workflow patterns:

#### 1. Code Agents: Dynamic Loop (`pkg/agents/executor.go`)

```go
func (e *InferenceExecutor) Execute(ctx context.Context, initialPrompt string) error {
    currentPrompt := initialPrompt

    for e.currentRound < e.maxRounds {
        // 1. Send prompt to LLM
        response, err := e.agent.executeInference(ctx, e.contextMgr, currentPrompt)

        // 2. Parse tool calls from response
        toolCalls := e.parseToolCalls(response.Text)

        // 3. Execute tools
        results, err := e.executeTools(ctx, toolCalls)

        // 4. Build feedback prompt with results
        currentPrompt = e.buildFeedbackPrompt(toolCalls, results)

        // 5. Repeat until done or max iterations
    }
}
```

The LLM has full agency to select which tools to use and when.

#### 2. Blog Orchestrator: Rigid Phases (`pkg/agents/blog_orchestrator.go`)

```go
func (o *BlogOrchestratorAgent) runOrchestration(ctx context.Context, ...) (*BlogOrchestratorOutput, error) {
    // Phase 1: Always analyze prompt
    analysis, err := o.analyzePrompt(ctx, contextMgr, prompt)

    // Phase 2: Always execute research
    researchData, err := o.executeResearch(ctx, analysis.ResearchTasks)

    // Phase 3: Always generate outline
    outline, err := o.generateOutline(ctx, contextMgr, prompt, analysis, researchData)

    // Phase 4: Always expand sections
    expandedContent, err := o.expandSections(ctx, contextMgr, outline, analysis, researchData)

    // Phase 5: Build newsletter (conditional)
    if analysis.IncludeNewsletter {
        newsletter := o.buildNewsletter(researchData)
        fullContent = expandedContent + "\n\n---\n\n" + newsletter
    }

    // Phase 6: Always generate social posts
    socialPosts, err := o.generateSocialPosts(ctx, contextMgr, expandedContent)

    // Phase 7: Publish if requested
    if shouldPublish {
        notionURL, pageID, err := o.publishToNotion(ctx, result)
    }

    return result, nil
}
```

The LLM has no agency over which phases execute or in what order.

### Problem

The blog orchestrator pattern:
1. Executes all phases even when not needed
2. Cannot adapt based on intermediate results
3. Cannot skip phases that add no value for a specific request
4. Cannot add phases that might be helpful for complex requests

## Decision

### 1. Unified Dynamic Invocation Pattern

Replace rigid phase execution with a dynamic tool-based pattern for all agents:

```go
// pkg/agents/dynamic_executor.go

type DynamicExecutor struct {
    agent      *BaseAgent
    registry   *tools.ToolRegistry
    contextMgr *llmcontext.Manager
    maxRounds  int

    // Tool configuration
    requiredTools  []string  // Must call before completion
    optionalTools  []string  // Can call if helpful
    completionFunc func(ctx context.Context, state *ExecutionState) (bool, error)
}

type ExecutionState struct {
    Round           int
    ToolsCalled     map[string]int  // Tool name -> call count
    Artifacts       map[string]interface{}
    LastResponse    string
    IntermediateOutputs []string
}

func (e *DynamicExecutor) Execute(ctx context.Context, initialPrompt string) (*ExecutionResult, error) {
    state := &ExecutionState{
        ToolsCalled: make(map[string]int),
        Artifacts:   make(map[string]interface{}),
    }

    currentPrompt := e.buildInitialPrompt(initialPrompt, state)

    for state.Round < e.maxRounds {
        state.Round++

        // 1. Infer with tool awareness
        response, err := e.infer(ctx, currentPrompt, state)
        state.LastResponse = response.Text

        // 2. Parse tool calls or completion signal
        toolCalls := e.parseToolCalls(response.Text)

        if len(toolCalls) == 0 {
            // Check if completion criteria are met
            complete, err := e.checkCompletion(ctx, state)
            if complete {
                return e.buildResult(state), nil
            }

            // Prompt LLM to continue
            currentPrompt = e.buildContinuationPrompt(state)
            continue
        }

        // 3. Execute tools and update state
        results := e.executeToolsWithState(ctx, toolCalls, state)

        // 4. Build next prompt
        currentPrompt = e.buildFeedbackPrompt(toolCalls, results, state)
    }

    return nil, fmt.Errorf("max rounds reached without completion")
}
```

### 2. Tool-Based Phases

Convert orchestrator phases to optional tools:

```go
// pkg/tools/blog/phases.go

// AnalyzePromptTool replaces Phase 1
type AnalyzePromptTool struct{}

func (t *AnalyzePromptTool) Name() string { return "analyze_prompt" }

func (t *AnalyzePromptTool) Description() string {
    return "Analyze a complex prompt to identify main topic, sections, and research needs"
}

func (t *AnalyzePromptTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "orchestration",
        Optionality: tools.ToolRequired,
        UsageHint:   "Call first to understand the structure of complex prompts",
        Schema: &logits.JSONSchema{
            Type: "object",
            Properties: map[string]*logits.JSONSchema{
                "prompt": {Type: "string", Description: "The prompt to analyze"},
            },
            Required: []string{"prompt"},
        },
    }
}

func (t *AnalyzePromptTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
    // Returns structured analysis that can inform subsequent tool calls
}

// GenerateOutlineTool replaces Phase 3
type GenerateOutlineTool struct{}

func (t *GenerateOutlineTool) Name() string { return "generate_outline" }

func (t *GenerateOutlineTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        Category:    "orchestration",
        Optionality: tools.ToolOptional,  // Not always needed for simple prompts
        UsageHint:   "Use for complex posts with multiple sections. Skip for simple content.",
    }
}
```

### 3. Completion Criteria

Define what "done" means for each agent type:

```go
// pkg/agents/completion.go

type CompletionCriteria struct {
    // RequiredTools that must be called at least once
    RequiredTools []string

    // MinimumArtifacts that must be produced
    MinimumArtifacts []string

    // ExplicitSignal - look for this in LLM output
    ExplicitSignal string

    // CustomCheck - additional validation
    CustomCheck func(state *ExecutionState) (bool, error)
}

func BlogCompletionCriteria() *CompletionCriteria {
    return &CompletionCriteria{
        RequiredTools: []string{},  // No strictly required tools
        MinimumArtifacts: []string{"content"},  // Must produce content
        ExplicitSignal: "CONTENT_COMPLETE",
        CustomCheck: func(state *ExecutionState) (bool, error) {
            // Check if content meets minimum length
            if content, ok := state.Artifacts["content"].(string); ok {
                return len(content) > 500, nil
            }
            return false, nil
        },
    }
}
```

### 4. Artifact Tracking

Track what tools produce and consume:

```go
// pkg/tools/interface.go (extended)

type ToolMetadata struct {
    // ... existing fields

    // Produces lists artifacts this tool can create
    Produces []string `json:"produces,omitempty"`

    // Consumes lists artifacts this tool needs
    Consumes []string `json:"consumes,omitempty"`
}

// Example for GenerateOutlineTool
func (t *GenerateOutlineTool) Metadata() *tools.ToolMetadata {
    return &tools.ToolMetadata{
        // ...
        Produces: []string{"outline"},
        Consumes: []string{"analysis", "research_data"},  // Optional inputs
    }
}
```

The executor uses artifact tracking to:
1. Include relevant artifacts in prompts
2. Suggest tools based on available artifacts
3. Warn if required inputs are missing

### 5. Dynamic Prompt Construction

Build prompts based on current state:

```go
func (e *DynamicExecutor) buildFeedbackPrompt(
    toolCalls []llm.ToolCall,
    results []*tools.Result,
    state *ExecutionState,
) string {
    var prompt strings.Builder

    // Tool results
    prompt.WriteString("## Tool Execution Results\n\n")
    for i, call := range toolCalls {
        result := results[i]
        prompt.WriteString(formatToolResult(call, result))

        // Track artifacts produced
        e.updateArtifacts(state, call.Name, result)
    }

    // Current artifacts
    prompt.WriteString("\n## Available Artifacts\n\n")
    for name, artifact := range state.Artifacts {
        prompt.WriteString(fmt.Sprintf("- %s: %s\n", name, summarizeArtifact(artifact)))
    }

    // Remaining tools
    prompt.WriteString("\n## Available Tools\n\n")
    for _, tool := range e.registry.List() {
        meta := tool.Metadata()
        if meta != nil {
            // Indicate if tool's inputs are available
            available := e.areInputsAvailable(meta.Consumes, state)
            prompt.WriteString(formatToolOption(tool, available, state))
        }
    }

    // Guidance
    prompt.WriteString("\n## Next Steps\n\n")
    prompt.WriteString(e.generateGuidance(state))

    return prompt.String()
}

func (e *DynamicExecutor) generateGuidance(state *ExecutionState) string {
    // Check completion criteria
    if e.canComplete(state) {
        return "You have enough artifacts to complete the task. Either call additional tools to enhance the output, or respond with 'CONTENT_COMPLETE' if satisfied."
    }

    // Suggest next tools based on artifacts
    suggestions := e.suggestNextTools(state)
    if len(suggestions) > 0 {
        return fmt.Sprintf("Consider calling: %v to progress toward completion.", suggestions)
    }

    return "Continue using available tools to complete the task."
}
```

### 6. Refactored Blog Agent

```go
// pkg/agents/blog_dynamic.go

func NewDynamicBlogAgent(cfg *config.Config, backend llm.Backend, jobMgr *jobs.Manager) *DynamicBlogAgent {
    agent := &DynamicBlogAgent{
        BaseAgent: NewBaseAgent("blog", "Dynamic blog content creation", cfg, backend, jobMgr),
    }

    // Register all blog tools as available
    agent.RegisterTools([]tools.ExtendedTool{
        blog.NewAnalyzePromptTool(),
        blog.NewGenerateOutlineTool(),
        blog.NewExpandSectionTool(),
        blog.NewGenerateSocialPostsTool(),
        tools.NewRSSFeedTool(cfg),
        tools.NewCalendarTool(cfg),
        tools.NewStaticLinksTool(cfg),
        tools.NewBlogNotionTool(cfg),
    })

    return agent
}

func (a *DynamicBlogAgent) Execute(ctx context.Context, input map[string]interface{}) (*jobs.Job, error) {
    prompt := input["prompt"].(string)

    job, err := a.jobManager.Create("blog", "Blog: "+extractTitle(prompt), input)
    if err != nil {
        return nil, err
    }

    go func() {
        executor := NewDynamicExecutor(a.BaseAgent, a.registry)
        executor.SetCompletionCriteria(BlogCompletionCriteria())

        result, err := executor.Execute(context.Background(), a.buildInitialPrompt(prompt, input))
        // ... handle result
    }()

    return job, nil
}

func (a *DynamicBlogAgent) buildInitialPrompt(prompt string, input map[string]interface{}) string {
    return fmt.Sprintf(`Create blog content based on this request:

%s

You have access to various tools. Use them as needed:
- analyze_prompt: Understand complex prompts (recommended for detailed requests)
- generate_outline: Create structure for long content
- expand_section: Write individual sections
- rss_feed: Get recent posts for references
- calendar: Get upcoming events
- static_links: Get social media links
- blog_notion: Publish to Notion when ready

Start by understanding the request. For simple requests, you may write content directly.
For complex requests, analyze the prompt first, then gather research, then write.

Respond with "CONTENT_COMPLETE" when the content is ready.`, prompt)
}
```

## Consequences

### Positive

1. **LLM Agency**: The LLM decides which tools to use based on the request.

2. **Efficiency**: Simple requests skip unnecessary phases.

3. **Flexibility**: Complex requests can use more tools.

4. **Adaptability**: The LLM can adjust based on intermediate results.

5. **Unified Pattern**: All agents use the same execution pattern.

6. **Debuggability**: Artifact tracking shows decision flow.

### Negative

1. **Non-Determinism**: Same input may produce different tool sequences.

2. **Potential Omissions**: LLM might skip helpful tools.

3. **More Tokens**: Dynamic prompts are larger than static phase prompts.

4. **Complexity**: More moving parts than rigid orchestration.

### Mitigation

1. **Guardrails**: Required tools ensure critical steps happen.

2. **Suggestions**: Prompt includes tool recommendations based on state.

3. **Limits**: Max rounds and artifact requirements prevent runaway execution.

4. **Logging**: Full execution trace enables debugging and refinement.

## Implementation

### Phase 1: DynamicExecutor

1. Implement `DynamicExecutor` with basic loop
2. Add artifact tracking
3. Implement completion criteria checking

### Phase 2: Blog Tool Conversion

1. Convert blog phases to tools
2. Define blog completion criteria
3. Create `DynamicBlogAgent`

### Phase 3: Testing

1. Compare outputs between rigid and dynamic patterns
2. Tune prompts for optimal tool selection
3. Measure token efficiency

### Phase 4: Rollout

1. Add feature flag for dynamic mode
2. Migrate podcast agent
3. Refine based on production usage

## Related ADRs

- **ADR-001**: Dynamic Tool Registry (provides tool metadata for prompts)
- **ADR-002**: LLM Tool Awareness Protocol (generates tool sections)
- **ADR-004**: Logit-Controlled Tool Calling (ensures valid tool calls)
