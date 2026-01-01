# ADR-002: LLM Tool Awareness Protocol

## Status

Proposed

## Context

Currently, available tools are communicated to the LLM through static system prompts:

### Current System Prompt Pattern (`pkg/agents/base.go:63-108`)

```
You are an autonomous coding agent...

# Available Tools

- file: Read, write, and modify files. ALWAYS read files before modifying them.
- code_edit: Precise line-based editing...
- search: Search code with regex patterns...
...

## Tool Call Format
Use tools by providing JSON objects: {"tool": "tool_name", "args": {"key": "value"}}
```

### Problems

1. **Static Descriptions**: Tool capabilities are hardcoded in prompts, not derived from tool metadata.

2. **No Parameter Details**: LLM doesn't know valid parameter names, types, or constraints.

3. **No Optionality Hints**: All tools appear equally important; no guidance on when to use optional tools.

4. **No Examples**: LLM must infer correct invocation format from description only.

5. **No Grouping**: Tools are listed flat; no logical organization by function.

6. **Version Drift**: Prompt descriptions can become stale as tools evolve.

## Decision

### 1. Dynamic Tool Description Generation

Generate tool descriptions from registry metadata instead of static strings:

```go
// pkg/prompts/tool_prompts.go

type ToolPromptGenerator struct {
    registry *tools.ToolRegistry
}

func (g *ToolPromptGenerator) GenerateToolSection() string {
    var sb strings.Builder

    // Group tools by category
    categories := g.registry.GetCategories()
    for _, category := range categories {
        tools := g.registry.ListByCategory(category)
        sb.WriteString(g.formatCategory(category, tools))
    }

    // Add invocation format
    sb.WriteString(g.generateInvocationGuide())

    return sb.String()
}
```

### 2. Tool Description Format

Each tool section includes:

```markdown
## [Category Name] Tools

### tool_name (required|optional)
Description from tool metadata.

**Parameters:**
- `param1` (type, required): Description
- `param2` (type, optional): Description, default: value

**When to use:** Usage hint from metadata.

**Example:**
```json
{"tool": "tool_name", "args": {"param1": "value"}}
```
```

### 3. Required vs Optional Tool Presentation

Different presentation for tool optionality:

```markdown
# Required Tools
These tools are essential for completing your task. Use them as needed.

## Code Tools
### file (required)
...

# Optional Tools
These tools can enhance your output but are not required. Consider using them when:
- You need current information (web_search)
- You're creating content that benefits from external data (rss_feed, calendar)

## Research Tools
### web_search (optional)
Use when you need up-to-date information not in your training data.
...
```

### 4. Context-Aware Tool Selection

Generate prompts based on task context:

```go
// pkg/prompts/context_prompts.go

type TaskContext struct {
    TaskType    string   // "build", "debug", "blog", etc.
    InputHints  []string // Detected from user input
    Preferences *UserPreferences
}

func (g *ToolPromptGenerator) GenerateForContext(ctx *TaskContext) string {
    // Determine which optional tools are relevant
    relevantOptional := g.determineRelevantTools(ctx)

    // Generate focused tool section
    return g.formatToolSection(
        g.registry.ListByOptionality(tools.ToolRequired),
        relevantOptional,
        ctx,
    )
}

func (g *ToolPromptGenerator) determineRelevantTools(ctx *TaskContext) []tools.ExtendedTool {
    var relevant []tools.ExtendedTool

    optionalTools := g.registry.ListByOptionality(tools.ToolOptional)
    for _, tool := range optionalTools {
        meta := tool.Metadata()

        // Check if tool is relevant to task type
        if g.isRelevantToTask(tool, ctx.TaskType) {
            relevant = append(relevant, tool)
            continue
        }

        // Check if input hints suggest this tool
        if g.matchesInputHints(tool, ctx.InputHints) {
            relevant = append(relevant, tool)
        }
    }

    return relevant
}
```

### 5. Schema-Driven Parameter Documentation

Generate parameter documentation from JSON Schema:

```go
func formatParameters(schema *logits.JSONSchema) string {
    var sb strings.Builder
    sb.WriteString("**Parameters:**\n")

    for name, prop := range schema.Properties {
        required := contains(schema.Required, name)
        reqStr := "optional"
        if required {
            reqStr = "required"
        }

        sb.WriteString(fmt.Sprintf("- `%s` (%s, %s): %s\n",
            name,
            prop.Type,
            reqStr,
            prop.Description,
        ))

        // Add enum values if present
        if len(prop.Enum) > 0 {
            sb.WriteString(fmt.Sprintf("  Valid values: %v\n", prop.Enum))
        }
    }

    return sb.String()
}
```

### 6. Example Generation

Include invocation examples in prompts:

```go
func formatExamples(tool tools.ExtendedTool) string {
    meta := tool.Metadata()
    if meta == nil || len(meta.Examples) == 0 {
        return ""
    }

    var sb strings.Builder
    sb.WriteString("**Examples:**\n")

    for _, ex := range meta.Examples {
        sb.WriteString(fmt.Sprintf("- %s:\n", ex.Description))
        sb.WriteString("```json\n")

        call := map[string]interface{}{
            "tool": tool.Name(),
            "args": ex.Input,
        }
        jsonBytes, _ := json.MarshalIndent(call, "", "  ")
        sb.Write(jsonBytes)
        sb.WriteString("\n```\n")
    }

    return sb.String()
}
```

### 7. Decision Guidance Prompt Section

Add explicit guidance for tool selection:

```markdown
# Tool Selection Guidelines

## When to Use Optional Tools

### Research Tools (rss_feed, calendar, web_search)
Consider using when:
- The task involves current events or recent information
- User mentions dates, events, or schedules
- Content would benefit from external references
- You're unsure about current state of something

### Publishing Tools (blog_notion, static_links)
Consider using when:
- User explicitly requests publishing
- Content is finalized and ready for distribution
- User mentions Notion, blog, or newsletter

## Tool Selection Process

1. Identify the core task requirements
2. Select required tools needed to complete the task
3. Consider if optional tools would enhance the output:
   - Would current data improve the result?
   - Would external references add value?
   - Did the user hint at wanting additional functionality?
4. Call selected tools in logical order
5. Re-evaluate tool needs after each tool result
```

## Consequences

### Positive

1. **Always Current**: Tool descriptions are generated from live metadata, never stale.

2. **Complete Information**: LLM receives parameter schemas, examples, and usage hints.

3. **Better Tool Selection**: LLM understands when optional tools add value.

4. **Reduced Errors**: Schema information helps LLM construct valid tool calls.

5. **Context Awareness**: Different tasks receive appropriately focused tool sections.

6. **Maintainability**: Tool documentation lives with tool code, not in prompts.

### Negative

1. **Longer Prompts**: More detailed tool sections consume more context tokens.

2. **Generation Overhead**: Prompts are dynamically generated per request.

3. **Complexity**: Prompt generation logic is more complex than static strings.

### Mitigation

1. **Lazy Generation**: Cache generated tool sections, regenerate only on registry changes.

2. **Selective Detail**: Include full examples only for complex or commonly misused tools.

3. **Tiered Information**: Provide summary view for familiar tools, detailed view on request.

## Implementation

### Phase 1: Basic Generation

1. Implement `ToolPromptGenerator` with basic formatting
2. Replace static tool sections in `buildSystemPrompt()` and `buildCodingSystemPrompt()`
3. Generate parameter documentation from schemas

### Phase 2: Context Awareness

1. Implement `TaskContext` detection from user input
2. Add logic to determine relevant optional tools
3. Generate context-aware prompts

### Phase 3: Examples and Hints

1. Add examples to tool metadata (ADR-001)
2. Generate example sections in prompts
3. Add tool selection guidelines section

### Phase 4: Optimization

1. Implement prompt caching
2. Add tiered detail levels
3. Measure and optimize context usage

## Example: Generated Tool Section

```markdown
# Available Tools

## Code Tools (Required)

### file
Read, write, and modify entire files.

**Parameters:**
- `action` (string, required): The operation to perform.
  Valid values: ["read", "write", "list"]
- `path` (string, required): File path (absolute or relative).
- `content` (string, optional): Content for write action.

**When to use:** Reading files before modification, writing new files, listing directory contents.

**Example:**
```json
{"tool": "file", "args": {"action": "read", "path": "main.go"}}
```

### code_edit
Precise line-based editing with edit/insert/delete operations.

**Parameters:**
- `action` (string, required): Edit operation type.
  Valid values: ["edit", "insert", "delete"]
- `file` (string, required): File to edit.
- `line` (integer, required): Line number to operate on.
- `content` (string, optional): New content for edit/insert.
- `end_line` (integer, optional): End line for multi-line operations.

**When to use:** Making targeted changes to specific lines. Preferred over file write for modifications.

## Research Tools (Optional)

### rss_feed
Fetch recent posts from configured RSS/Atom feeds.

**Parameters:**
- `action` (string, required): The operation to perform.
  Valid values: ["get_configured", "fetch"]
- `limit` (integer, optional): Max posts to return. Default: 5.
- `url` (string, optional): Custom feed URL for fetch action.

**When to use:** When content benefits from references to recent posts, or user mentions newsletters/updates.

---

# Tool Invocation Format

Call tools using JSON:
```json
{"tool": "tool_name", "args": {"param": "value"}}
```

Multiple tools can be called in sequence. After each tool result, decide if additional tools are needed.

When complete, respond with "TASK_COMPLETE".
```

## Related ADRs

- **ADR-001**: Dynamic Tool Registry Architecture (provides tool metadata)
- **ADR-003**: Dynamic Tool Invocation Pattern (uses prompts for tool guidance)
- **ADR-006**: Optional Tool Catalog (defines optional tools and their usage)
