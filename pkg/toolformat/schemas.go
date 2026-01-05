package toolformat

// This file defines canonical schemas for all built-in tools.
// These schemas are used by the model-specific formatters to generate
// tool definitions in the appropriate format for each model.

// FileToolSchema returns the parameter schema for the file tool
func FileToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The file operation to perform",
		"read", "write", "replace", "append", "delete",
	), true)

	schema.AddProperty("path", StringProperty("The file path to operate on"), true)
	schema.AddProperty("content", StringProperty("Content to write or append (for write/append actions)"), false)
	schema.AddProperty("old", StringProperty("Text to replace (for replace action)"), false)
	schema.AddProperty("new", StringProperty("Replacement text (for replace action)"), false)

	return schema
}

// CodeEditToolSchema returns the parameter schema for the code_edit tool
func CodeEditToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The editing action to perform",
		"get_lines", "edit_lines", "insert_at_line", "delete_lines",
	), true)

	schema.AddProperty("path", StringProperty("The file path to edit"), true)
	schema.AddProperty("start_line", NumberProperty("Starting line number (1-indexed)"), false)
	schema.AddProperty("end_line", NumberProperty("Ending line number (1-indexed)"), false)
	schema.AddProperty("line_number", NumberProperty("Line number for insert operation"), false)
	schema.AddProperty("new_content", StringProperty("New content for edit_lines action"), false)
	schema.AddProperty("content", StringProperty("Content for insert_at_line action"), false)

	return schema
}

// SearchToolSchema returns the parameter schema for the search tool
func SearchToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The search action to perform",
		"grep", "find_files", "find_in_file", "find_definition",
	), true)

	schema.AddProperty("pattern", StringProperty("Search pattern (regex for grep/find_in_file, glob for find_files)"), false)
	schema.AddProperty("path", StringProperty("File path for find_in_file action"), false)
	schema.AddProperty("directory", StringProperty("Directory to search in (optional)"), false)
	schema.AddProperty("file_pattern", StringProperty("Glob pattern to filter files (for grep)"), false)
	schema.AddProperty("case_insensitive", BoolProperty("Perform case-insensitive search"), false)
	schema.AddProperty("max_results", NumberProperty("Maximum number of results to return"), false)
	schema.AddProperty("name", StringProperty("Name to find definition for"), false)
	schema.AddProperty("language", StringEnumProperty(
		"Language hint for find_definition",
		"go", "python", "javascript", "typescript",
	), false)

	return schema
}

// NavigateToolSchema returns the parameter schema for the navigate tool
func NavigateToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The navigation action to perform",
		"list_directory", "get_file_outline", "find_imports", "get_tree",
	), true)

	schema.AddProperty("path", StringProperty("Path to the file or directory"), false)
	schema.AddProperty("max_depth", NumberProperty("Maximum depth for tree traversal"), false)
	schema.AddProperty("show_hidden", BoolProperty("Include hidden files in listing"), false)

	return schema
}

// GitToolSchema returns the parameter schema for the git tool
func GitToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The git action to perform",
		"status", "diff", "add", "commit", "push", "checkout", "create_branch", "log",
	), true)

	schema.AddProperty("files", ArrayProperty("Files to add/stage", StringProperty("file path")), false)
	schema.AddProperty("message", StringProperty("Commit message"), false)
	schema.AddProperty("branch", StringProperty("Branch name for checkout/create_branch"), false)
	schema.AddProperty("remote", StringProperty("Remote name for push (default: origin)"), false)
	schema.AddProperty("path", StringProperty("Path for diff operation"), false)
	schema.AddProperty("count", NumberProperty("Number of commits for log"), false)

	return schema
}

// BashToolSchema returns the parameter schema for the bash tool
func BashToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("command", StringProperty("The shell command to execute"), true)
	schema.AddProperty("timeout", NumberProperty("Timeout in seconds (default: 30)"), false)

	return schema
}

// TestToolSchema returns the parameter schema for the test tool
func TestToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("type", StringEnumProperty(
		"The test framework to use",
		"go", "npm", "python",
	), true)

	schema.AddProperty("pattern", StringProperty("Test pattern or path"), false)
	schema.AddProperty("verbose", BoolProperty("Enable verbose output"), false)
	schema.AddProperty("timeout", NumberProperty("Timeout in seconds"), false)

	return schema
}

// RSSToolSchema returns the parameter schema for the rss_feed tool
func RSSToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The RSS action to perform",
		"parse_feed", "list_items",
	), true)

	schema.AddProperty("url", StringProperty("URL of the RSS/Atom feed"), false)
	schema.AddProperty("max_items", NumberProperty("Maximum number of items to return"), false)

	return schema
}

// StaticLinksToolSchema returns the parameter schema for the static_links tool
func StaticLinksToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The action to perform",
		"get_all", "get_social", "get_custom", "get_youtube_placeholder",
	), true)

	return schema
}

// BlogPublishToolSchema returns the parameter schema for the blog_publish tool
func BlogPublishToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("title", StringProperty("Title of the blog post"), true)
	schema.AddProperty("content", StringProperty("Markdown content of the blog post"), true)
	schema.AddProperty("summary", StringProperty("Short summary for the post"), false)
	schema.AddProperty("tags", ArrayProperty("Tags for the post", StringProperty("tag")), false)

	return schema
}

// CalendarToolSchema returns the parameter schema for the calendar tool
func CalendarToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The calendar action to perform",
		"list_events", "create_event", "update_event", "delete_event", "check_availability",
	), true)

	schema.AddProperty("start_date", StringProperty("Start date (ISO 8601 format)"), false)
	schema.AddProperty("end_date", StringProperty("End date (ISO 8601 format)"), false)
	schema.AddProperty("title", StringProperty("Event title"), false)
	schema.AddProperty("description", StringProperty("Event description"), false)
	schema.AddProperty("event_id", StringProperty("Event ID for update/delete operations"), false)

	return schema
}

// WebScrapeToolSchema returns the parameter schema for the web_scrape tool
func WebScrapeToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The web action to perform",
		"fetch_url", "search_web", "fetch_github_file", "search_github_code", "fetch_stackoverflow_question",
	), true)

	schema.AddProperty("url", StringProperty("URL to fetch"), false)
	schema.AddProperty("query", StringProperty("Search query"), false)
	schema.AddProperty("repo", StringProperty("GitHub repository (owner/repo)"), false)
	schema.AddProperty("path", StringProperty("File path in repository"), false)
	schema.AddProperty("branch", StringProperty("Branch name (default: main)"), false)

	return schema
}

// NotionToolSchema returns the parameter schema for the notion tool
func NotionToolSchema() ParameterSchema {
	schema := NewParameterSchema()

	schema.AddProperty("action", StringEnumProperty(
		"The Notion action to perform",
		"query_database", "create_page", "update_page", "append_blocks",
	), true)

	schema.AddProperty("database_id", StringProperty("Notion database ID"), false)
	schema.AddProperty("page_id", StringProperty("Notion page ID"), false)
	schema.AddProperty("title", StringProperty("Page title"), false)
	schema.AddProperty("content", StringProperty("Page content"), false)
	schema.AddProperty("properties", ObjectProperty("Page properties", nil), false)

	return schema
}

// JobToolSchemas returns schemas for job management tools
func GetJobStatusToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("job_id", StringProperty("ID of the job to check"), true)
	return schema
}

func ListJobsToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("status", StringEnumProperty(
		"Filter by status",
		"pending", "running", "completed", "failed", "cancelled",
	), false)
	return schema
}

func CancelJobToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("job_id", StringProperty("ID of the job to cancel"), true)
	return schema
}

// AgentToolSchemas returns schemas for agent invocation tools
func BuilderAgentToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("description", StringProperty("Description of the feature to build"), true)
	schema.AddProperty("files", ArrayProperty("Specific files to focus on", StringProperty("file path")), false)
	return schema
}

func DebuggerAgentToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("symptoms", StringProperty("Description of the bug symptoms"), true)
	schema.AddProperty("error_logs", StringProperty("Relevant error logs or stack traces"), false)
	schema.AddProperty("files", ArrayProperty("Files where the bug might be", StringProperty("file path")), false)
	return schema
}

func ReviewerAgentToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("branch", StringProperty("Branch to review"), true)
	schema.AddProperty("base_branch", StringProperty("Base branch to compare against (default: main)"), false)
	return schema
}

func TriagerAgentToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("issue", StringProperty("Description of the issue to triage"), true)
	schema.AddProperty("context", StringProperty("Additional context about the issue"), false)
	return schema
}

func BlogOrchestratorToolSchema() ParameterSchema {
	schema := NewParameterSchema()
	schema.AddProperty("prompt", StringProperty("Blog post prompt or topic"), true)
	schema.AddProperty("research", BoolProperty("Enable research phase"), false)
	schema.AddProperty("publish", BoolProperty("Publish to Notion when complete"), false)
	return schema
}

// GetSchemaForTool returns the schema for a tool by name
func GetSchemaForTool(toolName string) ParameterSchema {
	switch toolName {
	case "file":
		return FileToolSchema()
	case "code_edit":
		return CodeEditToolSchema()
	case "search":
		return SearchToolSchema()
	case "navigate":
		return NavigateToolSchema()
	case "git":
		return GitToolSchema()
	case "bash":
		return BashToolSchema()
	case "test":
		return TestToolSchema()
	case "rss_feed":
		return RSSToolSchema()
	case "static_links":
		return StaticLinksToolSchema()
	case "blog_publish":
		return BlogPublishToolSchema()
	case "calendar":
		return CalendarToolSchema()
	case "web_scrape":
		return WebScrapeToolSchema()
	case "notion":
		return NotionToolSchema()
	case "get_job_status":
		return GetJobStatusToolSchema()
	case "list_jobs":
		return ListJobsToolSchema()
	case "cancel_job":
		return CancelJobToolSchema()
	case "builder":
		return BuilderAgentToolSchema()
	case "debugger":
		return DebuggerAgentToolSchema()
	case "reviewer":
		return ReviewerAgentToolSchema()
	case "triager":
		return TriagerAgentToolSchema()
	case "blog_orchestrator":
		return BlogOrchestratorToolSchema()
	default:
		return NewParameterSchema()
	}
}

// GetCategoryForTool returns the default category for a tool by name
func GetCategoryForTool(toolName string) ToolCategory {
	switch toolName {
	case "file", "code_edit", "search", "navigate", "git", "bash", "test":
		return CategoryCode
	case "rss_feed", "static_links", "web_scrape":
		return CategoryResearch
	case "blog_publish", "notion", "calendar":
		return CategoryBlog
	case "get_job_status", "list_jobs", "cancel_job":
		return CategoryJob
	case "builder", "debugger", "reviewer", "triager", "blog_orchestrator":
		return CategoryAgent
	default:
		return CategoryCode
	}
}
