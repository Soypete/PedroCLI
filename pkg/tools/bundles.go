package tools

// ToolBundle defines a named set of tools for a specific agent type
type ToolBundle struct {
	Name        string   // Bundle name (e.g., "code_agent", "blog_agent")
	Description string   // What this bundle is for
	Required    []string // Tool names that must be available
	Optional    []string // Tool names that can be used if available
}

// ApplyBundle registers all tools from a bundle with a registry
// Returns list of tools that couldn't be registered (not found in source registry)
func (b *ToolBundle) ApplyBundle(source, target *ToolRegistry) []string {
	var missing []string

	// Register required tools
	for _, toolName := range b.Required {
		if tool, ok := source.Get(toolName); ok {
			_ = target.RegisterExtended(tool) // Error only if already registered, which is fine
		} else {
			missing = append(missing, toolName)
		}
	}

	// Register optional tools (don't track as missing)
	for _, toolName := range b.Optional {
		if tool, ok := source.Get(toolName); ok {
			_ = target.RegisterExtended(tool) // Error only if already registered, which is fine
		}
	}

	return missing
}

// AllToolNames returns all tool names in this bundle (required + optional)
func (b *ToolBundle) AllToolNames() []string {
	all := make([]string, 0, len(b.Required)+len(b.Optional))
	all = append(all, b.Required...)
	all = append(all, b.Optional...)
	return all
}

// HasTool checks if a tool is part of this bundle
func (b *ToolBundle) HasTool(name string) bool {
	for _, n := range b.Required {
		if n == name {
			return true
		}
	}
	for _, n := range b.Optional {
		if n == name {
			return true
		}
	}
	return false
}

// Predefined bundles for different agent types

// CodeAgentBundle contains tools for code-focused agents (builder, debugger, reviewer, triager)
var CodeAgentBundle = &ToolBundle{
	Name:        "code_agent",
	Description: "Tools for code exploration, modification, and version control",
	Required:    []string{"file", "code_edit", "search", "navigate", "git"},
	Optional:    []string{"bash", "test"},
}

// BlogAgentBundle contains tools for blog writing agents (writer, editor)
var BlogAgentBundle = &ToolBundle{
	Name:        "blog_agent",
	Description: "Tools for blog research, writing, and publishing",
	Required:    []string{},
	Optional:    []string{"rss_feed", "static_links", "blog_publish", "calendar"},
}

// BlogOrchestratorBundle contains all research and publishing tools for multi-phase blog generation
var BlogOrchestratorBundle = &ToolBundle{
	Name:        "blog_orchestrator",
	Description: "Comprehensive tools for multi-phase blog generation with research",
	Required:    []string{},
	Optional:    []string{"rss_feed", "static_links", "blog_publish", "calendar", "web_scrape"},
}

// ResearchBundle contains tools for gathering information
var ResearchBundle = &ToolBundle{
	Name:        "research",
	Description: "Tools for web scraping and information gathering",
	Required:    []string{},
	Optional:    []string{"web_scrape", "rss_feed"},
}

// UtilityBundle contains job management tools
var UtilityBundle = &ToolBundle{
	Name:        "utility",
	Description: "Tools for job management and utilities",
	Required:    []string{},
	Optional:    []string{"get_job_status", "list_jobs", "cancel_job"},
}

// AllBundles returns all predefined bundles
func AllBundles() []*ToolBundle {
	return []*ToolBundle{
		CodeAgentBundle,
		BlogAgentBundle,
		BlogOrchestratorBundle,
		ResearchBundle,
		UtilityBundle,
	}
}

// GetBundle returns a bundle by name, or nil if not found
func GetBundle(name string) *ToolBundle {
	for _, b := range AllBundles() {
		if b.Name == name {
			return b
		}
	}
	return nil
}
