package agents

import (
	"github.com/soypete/pedrocli/pkg/config"
)

var defaultModeConstraints = map[string]config.ModeConfig{
	"code": {
		AllowedTools:     []string{},
		DeniedTools:      []string{},
		AllowWrites:      true,
		AgentTypes:       []string{"builder", "debugger", "reviewer", "triager"},
		Description:      "Default code mode - full tool access",
		MaxInferenceRuns: 20,
	},
	"chat": {
		AllowedTools: []string{
			"search", "navigate", "file", "context",
		},
		DeniedTools:      []string{"code_edit", "bash", "git"},
		AllowWrites:      false,
		AgentTypes:       []string{"builder", "debugger", "reviewer", "triager"},
		Description:      "Read-only mode for chatting about code",
		MaxInferenceRuns: 10,
	},
	"plan": {
		AllowedTools: []string{
			"search", "navigate", "file", "context", "git",
		},
		DeniedTools:      []string{"code_edit", "bash"},
		AllowWrites:      false,
		AgentTypes:       []string{"triager", "reviewer"},
		Description:      "Planning mode - analyze and plan without writing",
		MaxInferenceRuns: 15,
	},
	"build": {
		AllowedTools:     []string{},
		DeniedTools:      []string{},
		AllowWrites:      true,
		AgentTypes:       []string{"builder", "debugger"},
		Description:      "Full build mode - can write code and run commands",
		MaxInferenceRuns: 30,
	},
	"review": {
		AllowedTools: []string{
			"search", "navigate", "file", "git", "github", "test",
		},
		DeniedTools:      []string{"code_edit", "bash"},
		AllowWrites:      false,
		AgentTypes:       []string{"reviewer"},
		Description:      "Code review mode - read and analyze code",
		MaxInferenceRuns: 20,
	},
	"blog": {
		AllowedTools: []string{
			"file", "rss", "web_search", "web_scraper", "context",
		},
		DeniedTools:      []string{"code_edit", "bash", "git"},
		AllowWrites:      true,
		AgentTypes:       []string{"blog_content"},
		Description:      "Blog writing mode",
		MaxInferenceRuns: 25,
	},
	"podcast": {
		AllowedTools: []string{
			"file", "web_search", "web_scraper", "context", "notion", "cal_com",
		},
		DeniedTools:      []string{"code_edit", "bash", "git"},
		AllowWrites:      true,
		AgentTypes:       []string{"podcast"},
		Description:      "Podcast mode for outlines and scripts",
		MaxInferenceRuns: 25,
	},
	"technical_writer": {
		AllowedTools: []string{
			"search", "web_scraper", "file", "context", "code_search",
		},
		DeniedTools:      []string{"code_edit", "bash", "git"},
		AllowWrites:      true,
		AgentTypes:       []string{"technical_writer"},
		Description:      "Technical writer mode for documentation",
		MaxInferenceRuns: 20,
	},
}

func GetModeConfigForAgent(agentType string, customModes config.ModeConfigMap) config.ModeConfig {
	modeStr := agentTypeToMode(agentType)
	return getModeConfigForModeString(modeStr, customModes)
}

func agentTypeToMode(agentType string) string {
	switch agentType {
	case "builder", "debugger", "reviewer", "triager":
		return "code"
	case "blog_content", "dynamic_blog":
		return "blog"
	case "podcast":
		return "podcast"
	case "technical_writer":
		return "technical_writer"
	default:
		return "code"
	}
}

func getModeConfigForModeString(modeStr string, customModes config.ModeConfigMap) config.ModeConfig {
	defaultCfg, exists := defaultModeConstraints[modeStr]
	if !exists {
		defaultCfg = defaultModeConstraints["code"]
	}

	if customModes != nil {
		if customCfg, exists := customModes[modeStr]; exists {
			return mergeModeConfig(defaultCfg, customCfg)
		}
	}

	return defaultCfg
}

func mergeModeConfig(defaultCfg, customCfg config.ModeConfig) config.ModeConfig {
	if len(customCfg.AllowedTools) > 0 {
		defaultCfg.AllowedTools = customCfg.AllowedTools
	}
	if len(customCfg.DeniedTools) > 0 {
		defaultCfg.DeniedTools = customCfg.DeniedTools
	}
	if customCfg.AllowWrites {
		defaultCfg.AllowWrites = customCfg.AllowWrites
	}
	if len(customCfg.AgentTypes) > 0 {
		defaultCfg.AgentTypes = customCfg.AgentTypes
	}
	if customCfg.MaxInferenceRuns > 0 {
		defaultCfg.MaxInferenceRuns = customCfg.MaxInferenceRuns
	}
	if customCfg.SystemPrompt != "" {
		defaultCfg.SystemPrompt = customCfg.SystemPrompt
	}
	if customCfg.Description != "" {
		defaultCfg.Description = customCfg.Description
	}

	return defaultCfg
}

func ApplyModeConstraintsToExecutor(executor interface {
	SetModeConstraints(allowedTools, deniedTools []string, allowWrites bool)
	SetMaxRounds(maxRounds int)
}, agentType string, customModes config.ModeConfigMap) {
	cfg := GetModeConfigForAgent(agentType, customModes)

	executor.SetModeConstraints(cfg.AllowedTools, cfg.DeniedTools, cfg.AllowWrites)

	if cfg.MaxInferenceRuns > 0 {
		executor.SetMaxRounds(cfg.MaxInferenceRuns)
	}
}

func GetAllowedToolsForAgent(agentType string, customModes config.ModeConfigMap) ([]string, []string, bool) {
	cfg := GetModeConfigForAgent(agentType, customModes)
	return cfg.AllowedTools, cfg.DeniedTools, cfg.AllowWrites
}

func GetModeStringForAgentType(agentType string) string {
	return agentTypeToMode(agentType)
}
