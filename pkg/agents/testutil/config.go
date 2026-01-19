package testutil

import (
	"github.com/soypete/pedrocli/pkg/config"
)

// NewTestConfig creates a minimal configuration for testing.
func NewTestConfig() *config.Config {
	return &config.Config{
		Model: config.ModelConfig{
			Type:          "ollama",
			ModelName:     "test-model",
			ContextSize:   32768,
			UsableContext: 24576,
			Temperature:   0.2,
			ServerURL:     "http://localhost:11434",
			EnableTools:   true,
		},
		Limits: config.LimitsConfig{
			MaxTaskDurationMinutes: 30,
			MaxInferenceRuns:       20,
		},
		Debug: config.DebugConfig{
			Enabled:       false,
			KeepTempFiles: false,
			LogLevel:      "error", // Minimize output in tests
		},
		Tools: config.ToolsConfig{
			AllowedBashCommands: []string{"echo", "ls"},
			ForbiddenCommands:   []string{"rm", "sudo"},
		},
		Project: config.ProjectConfig{
			Name:    "test-project",
			Workdir: "/tmp/test-workdir",
		},
		Git: config.GitConfig{
			Remote:       "origin",
			BranchPrefix: "test/",
		},
	}
}

// NewTestConfigWithMaxRounds creates a test config with a specific max rounds limit.
func NewTestConfigWithMaxRounds(maxRounds int) *config.Config {
	cfg := NewTestConfig()
	cfg.Limits.MaxInferenceRuns = maxRounds
	return cfg
}

// NewTestConfigWithDebug creates a test config with debug enabled.
func NewTestConfigWithDebug() *config.Config {
	cfg := NewTestConfig()
	cfg.Debug.Enabled = true
	cfg.Debug.KeepTempFiles = true
	cfg.Debug.LogLevel = "debug"
	return cfg
}
