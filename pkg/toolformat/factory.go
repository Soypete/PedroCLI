package toolformat

import (
	"context"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/tools"
)

// ToolFactory creates and configures tool registries
type ToolFactory struct {
	config  *config.Config
	workDir string
}

// NewToolFactory creates a new tool factory
func NewToolFactory(cfg *config.Config, workDir string) *ToolFactory {
	return &ToolFactory{
		config:  cfg,
		workDir: workDir,
	}
}

// CreateRegistry creates a new registry with all available tools
func (f *ToolFactory) CreateRegistry() (*Registry, error) {
	registry := NewRegistry()

	// Register code tools
	if err := f.registerCodeTools(registry); err != nil {
		return nil, err
	}

	return registry, nil
}

// CreateRegistryForMode creates a registry with tools for a specific mode
func (f *ToolFactory) CreateRegistryForMode(mode ToolMode) (*Registry, error) {
	registry := NewRegistry()

	switch mode {
	case ModeCoding:
		if err := f.registerCodeTools(registry); err != nil {
			return nil, err
		}
	case ModeBlog, ModePodcast:
		if err := f.registerBlogTools(registry); err != nil {
			return nil, err
		}
	case ModeResearch:
		if err := f.registerResearchTools(registry); err != nil {
			return nil, err
		}
	case ModeAll:
		if err := f.registerCodeTools(registry); err != nil {
			return nil, err
		}
		if err := f.registerBlogTools(registry); err != nil {
			return nil, err
		}
		if err := f.registerResearchTools(registry); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// registerCodeTools registers the 7 core code tools
func (f *ToolFactory) registerCodeTools(registry *Registry) error {
	// File tool
	fileTool := tools.NewFileTool()
	if err := registry.Register(f.adaptTool(fileTool, CategoryCode, FileToolSchema())); err != nil {
		return err
	}

	// Code edit tool
	codeEditTool := tools.NewCodeEditTool()
	if err := registry.Register(f.adaptTool(codeEditTool, CategoryCode, CodeEditToolSchema())); err != nil {
		return err
	}

	// Search tool
	searchTool := tools.NewSearchTool(f.workDir)
	if err := registry.Register(f.adaptTool(searchTool, CategoryCode, SearchToolSchema())); err != nil {
		return err
	}

	// Navigate tool
	navigateTool := tools.NewNavigateTool(f.workDir)
	if err := registry.Register(f.adaptTool(navigateTool, CategoryCode, NavigateToolSchema())); err != nil {
		return err
	}

	// Git tool
	gitTool := tools.NewGitTool(f.workDir)
	if err := registry.Register(f.adaptTool(gitTool, CategoryCode, GitToolSchema())); err != nil {
		return err
	}

	// Bash tool (with config restrictions)
	bashTool := tools.NewBashTool(f.config, f.workDir)
	if err := registry.Register(f.adaptTool(bashTool, CategoryCode, BashToolSchema())); err != nil {
		return err
	}

	// Test tool
	testTool := tools.NewTestTool(f.workDir)
	if err := registry.Register(f.adaptTool(testTool, CategoryCode, TestToolSchema())); err != nil {
		return err
	}

	return nil
}

// registerBlogTools registers blog/content tools
func (f *ToolFactory) registerBlogTools(registry *Registry) error {
	// RSS feed tool
	rssTool := tools.NewRSSFeedTool(f.config)
	if err := registry.Register(f.adaptTool(rssTool, CategoryResearch, RSSToolSchema())); err != nil {
		return err
	}

	// Static links tool
	if f.config.Blog.Enabled {
		staticLinksTool := tools.NewStaticLinksTool(f.config)
		if err := registry.Register(f.adaptTool(staticLinksTool, CategoryResearch, StaticLinksToolSchema())); err != nil {
			return err
		}
	}

	return nil
}

// registerResearchTools registers research/web tools
func (f *ToolFactory) registerResearchTools(registry *Registry) error {
	// Web scrape tool - requires config and token manager, skip if not configured
	// This tool will be registered separately when token manager is available
	return nil
}

// adaptTool converts a legacy tool to a ToolDefinition
func (f *ToolFactory) adaptTool(tool tools.Tool, category ToolCategory, schema ParameterSchema) *ToolDefinition {
	return &ToolDefinition{
		Name:        tool.Name(),
		Description: tool.Description(),
		Category:    category,
		Parameters:  schema,
		Handler:     createHandler(tool),
	}
}

// createHandler creates a ToolHandler from a legacy tool
func createHandler(tool tools.Tool) ToolHandler {
	return func(args map[string]interface{}) (*ToolResult, error) {
		result, err := tool.Execute(context.Background(), args)
		if err != nil {
			return &ToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &ToolResult{
			Success:       result.Success,
			Output:        result.Output,
			Error:         result.Error,
			ModifiedFiles: result.ModifiedFiles,
			Data:          result.Data,
		}, nil
	}
}

// Quick factory functions for common use cases

// CreateCodeExecutor creates an executor configured for coding tasks
func CreateCodeExecutor(cfg *config.Config, workDir string, modelName string) (*ToolExecutor, error) {
	factory := NewToolFactory(cfg, workDir)
	registry, err := factory.CreateRegistryForMode(ModeCoding)
	if err != nil {
		return nil, err
	}

	executor := NewExecutorBuilder().
		WithRegistry(registry).
		WithFormatterForModel(modelName).
		WithMaxRounds(cfg.Limits.MaxInferenceRuns).
		Build()

	return executor, nil
}

// CreateBlogExecutor creates an executor configured for blog tasks
func CreateBlogExecutor(cfg *config.Config, workDir string, modelName string) (*ToolExecutor, error) {
	factory := NewToolFactory(cfg, workDir)
	registry, err := factory.CreateRegistryForMode(ModeBlog)
	if err != nil {
		return nil, err
	}

	executor := NewExecutorBuilder().
		WithRegistry(registry).
		WithFormatterForModel(modelName).
		WithMaxRounds(cfg.Limits.MaxInferenceRuns).
		Build()

	return executor, nil
}

// CreateFullExecutor creates an executor with all tools
func CreateFullExecutor(cfg *config.Config, workDir string, modelName string) (*ToolExecutor, error) {
	factory := NewToolFactory(cfg, workDir)
	registry, err := factory.CreateRegistryForMode(ModeAll)
	if err != nil {
		return nil, err
	}

	executor := NewExecutorBuilder().
		WithRegistry(registry).
		WithFormatterForModel(modelName).
		WithMaxRounds(cfg.Limits.MaxInferenceRuns).
		Build()

	return executor, nil
}
