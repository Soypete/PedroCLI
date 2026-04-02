package orchestration

import "github.com/soypete/pedrocli/pkg/config"

var defaultModeConstraints = map[Mode]config.ModeConfig{
	ModeChat: {
		AllowedTools: []string{
			"search",
			"navigate",
			"file",
			"context",
		},
		DeniedTools: []string{
			"code_edit",
			"bash",
			"git",
		},
		AllowWrites:      false,
		AgentTypes:       []string{"builder", "debugger", "reviewer", "triager"},
		Description:      "Read-only mode for chatting about code",
		MaxInferenceRuns: 10,
	},
	ModePlan: {
		AllowedTools: []string{
			"search",
			"navigate",
			"file",
			"context",
			"git",
		},
		DeniedTools: []string{
			"code_edit",
			"bash",
		},
		AllowWrites:      false,
		AgentTypes:       []string{"triager", "reviewer"},
		Description:      "Planning mode - analyze and plan without writing",
		MaxInferenceRuns: 15,
	},
	ModeBuild: {
		AllowedTools:     []string{},
		DeniedTools:      []string{},
		AllowWrites:      true,
		AgentTypes:       []string{"builder", "debugger"},
		Description:      "Full build mode - can write code and run commands",
		MaxInferenceRuns: 30,
	},
	ModeReview: {
		AllowedTools: []string{
			"search",
			"navigate",
			"file",
			"git",
			"github",
			"test",
		},
		DeniedTools: []string{
			"code_edit",
			"bash",
		},
		AllowWrites:      false,
		AgentTypes:       []string{"reviewer"},
		Description:      "Code review mode - read and analyze code",
		MaxInferenceRuns: 20,
	},
	ModeCode: {
		AllowedTools:     []string{},
		DeniedTools:      []string{},
		AllowWrites:      true,
		AgentTypes:       []string{"builder", "debugger", "reviewer", "triager"},
		Description:      "Default code mode - full tool access",
		MaxInferenceRuns: 20,
	},
	ModeBlog: {
		AllowedTools: []string{
			"file",
			"rss",
			"web_search",
			"web_scraper",
			"context",
		},
		DeniedTools: []string{
			"code_edit",
			"bash",
			"git",
		},
		AllowWrites:      true,
		AgentTypes:       []string{"blog_content"},
		Description:      "Blog writing mode",
		MaxInferenceRuns: 25,
	},
	ModePodcast: {
		AllowedTools: []string{
			"file",
			"web_search",
			"web_scraper",
			"context",
			"notion",
			"cal_com",
		},
		DeniedTools: []string{
			"code_edit",
			"bash",
			"git",
		},
		AllowWrites:      true,
		AgentTypes:       []string{"podcast"},
		Description:      "Podcast mode for outlines and scripts",
		MaxInferenceRuns: 25,
	},
	ModeTechnicalWriter: {
		AllowedTools: []string{
			"search",
			"web_scraper",
			"file",
			"context",
			"code_search",
		},
		DeniedTools: []string{
			"code_edit",
			"bash",
			"git",
		},
		AllowWrites:      true,
		AgentTypes:       []string{"technical_writer"},
		Description:      "Technical writer mode for documentation",
		MaxInferenceRuns: 20,
	},
}

func GetModeConfig(mode Mode) config.ModeConfig {
	if cfg, ok := defaultModeConstraints[mode]; ok {
		return cfg
	}
	return defaultModeConstraints[ModeCode]
}

func MergeModeConfig(mode Mode, customCfg config.ModeConfig) config.ModeConfig {
	defaultCfg := GetModeConfig(mode)

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

func GetAllowedToolsForMode(mode Mode) ([]string, []string, bool) {
	cfg := GetModeConfig(mode)
	allowWrites := cfg.AllowWrites

	deniedTools := cfg.DeniedTools
	if len(cfg.AllowedTools) == 0 && len(deniedTools) == 0 {
		return nil, nil, allowWrites
	}

	return cfg.AllowedTools, deniedTools, allowWrites
}

func GetModeConfigWithCustom(mode Mode, customModes config.ModeConfigMap) config.ModeConfig {
	defaultCfg := GetModeConfig(mode)

	if customCfg, exists := customModes[string(mode)]; exists {
		return MergeModeConfig(mode, customCfg)
	}

	return defaultCfg
}

func GetAllowedToolsForModeWithCustom(mode Mode, customModes config.ModeConfigMap) ([]string, []string, bool) {
	cfg := GetModeConfigWithCustom(mode, customModes)
	allowWrites := cfg.AllowWrites

	deniedTools := cfg.DeniedTools
	if len(cfg.AllowedTools) == 0 && len(deniedTools) == 0 {
		return nil, nil, allowWrites
	}

	return cfg.AllowedTools, deniedTools, allowWrites
}

func GetMaxInferenceRunsForMode(mode Mode, customModes config.ModeConfigMap) int {
	cfg := GetModeConfigWithCustom(mode, customModes)
	return cfg.MaxInferenceRuns
}

func ApplyModeConstraintsToExecutor(executor interface {
	SetModeConstraints(allowedTools, deniedTools []string, allowWrites bool)
	SetMaxRounds(maxRounds int)
}, mode Mode, customModes config.ModeConfigMap) {
	allowed, denied, allowWrites := GetAllowedToolsForModeWithCustom(mode, customModes)
	executor.SetModeConstraints(allowed, denied, allowWrites)

	maxRuns := GetMaxInferenceRunsForMode(mode, customModes)
	if maxRuns > 0 {
		executor.SetMaxRounds(maxRuns)
	}
}
