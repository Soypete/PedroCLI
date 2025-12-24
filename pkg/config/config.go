package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the Pedroceli configuration
type Config struct {
	Model     ModelConfig     `json:"model"`
	Execution ExecutionConfig `json:"execution"`
	Git       GitConfig       `json:"git"`
	Tools     ToolsConfig     `json:"tools"`
	Project   ProjectConfig   `json:"project"`
	Limits    LimitsConfig    `json:"limits"`
	Debug     DebugConfig     `json:"debug"`
	Platform  PlatformConfig  `json:"platform"`
	Init      InitConfig      `json:"init"`
	LSP       LSPConfig       `json:"lsp"`
	FileIO    FileIOConfig    `json:"fileio"`
	Web       WebConfig       `json:"web"`
}

// ModelConfig contains model configuration
type ModelConfig struct {
	Type          string  `json:"type"` // "llamacpp" or "ollama"
	ModelPath     string  `json:"model_path,omitempty"`
	LlamaCppPath  string  `json:"llamacpp_path,omitempty"`
	ModelName     string  `json:"model_name,omitempty"` // for Ollama
	OllamaURL     string  `json:"ollama_url,omitempty"` // Ollama API URL
	ContextSize   int     `json:"context_size"`
	UsableContext int     `json:"usable_context,omitempty"`
	NGpuLayers    int     `json:"n_gpu_layers,omitempty"`
	Temperature   float64 `json:"temperature"`
	Threads       int     `json:"threads,omitempty"`
}

// ExecutionConfig contains execution settings
type ExecutionConfig struct {
	RunOnSpark bool   `json:"run_on_spark"`
	SparkSSH   string `json:"spark_ssh,omitempty"`
}

// GitConfig contains git settings
type GitConfig struct {
	AlwaysDraftPR bool   `json:"always_draft_pr"`
	BranchPrefix  string `json:"branch_prefix"`
	Remote        string `json:"remote"`
}

// ToolsConfig contains tool settings
type ToolsConfig struct {
	AllowedBashCommands []string `json:"allowed_bash_commands"`
	ForbiddenCommands   []string `json:"forbidden_commands"`
}

// ProjectConfig contains project settings
type ProjectConfig struct {
	Name      string   `json:"name"`
	Workdir   string   `json:"workdir"`
	TechStack []string `json:"tech_stack"`
}

// LimitsConfig contains execution limits
type LimitsConfig struct {
	MaxTaskDurationMinutes int `json:"max_task_duration_minutes"`
	MaxInferenceRuns       int `json:"max_inference_runs"`
}

// DebugConfig contains debug settings
type DebugConfig struct {
	Enabled       bool   `json:"enabled"`
	KeepTempFiles bool   `json:"keep_temp_files"`
	LogLevel      string `json:"log_level"`
}

// PlatformConfig contains platform settings
type PlatformConfig struct {
	OS    string `json:"os"`    // "auto", "darwin", "linux", "windows"
	Shell string `json:"shell"` // "/bin/sh", "/bin/bash", etc.
}

// InitConfig contains initialization settings
type InitConfig struct {
	SkipChecks bool `json:"skip_checks"`
	Verbose    bool `json:"verbose"`
}

// LSPConfig contains LSP (Language Server Protocol) settings
type LSPConfig struct {
	Enabled      bool                    `json:"enabled"`
	AutoDiscover bool                    `json:"auto_discover"`
	Timeout      int                     `json:"timeout"`
	Servers      map[string]LSPServerDef `json:"servers,omitempty"`
}

// LSPServerDef defines an LSP server configuration
type LSPServerDef struct {
	Command     string                 `json:"command"`
	Args        []string               `json:"args,omitempty"`
	Languages   []string               `json:"languages"`
	RootURI     string                 `json:"root_uri,omitempty"`
	InitOptions map[string]interface{} `json:"init_options,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
	Enabled     bool                   `json:"enabled"`
}

// FileIOConfig contains file I/O settings
type FileIOConfig struct {
	MaxFileSize   int64  `json:"max_file_size"`
	EnableBackup  bool   `json:"enable_backup"`
	BackupDir     string `json:"backup_dir,omitempty"`
	AtomicWrites  bool   `json:"atomic_writes"`
	PreservePerms bool   `json:"preserve_permissions"`
}

// WebConfig contains web server settings
type WebConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Host    string `json:"host"`
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	config.setDefaults()

	// Validate
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadDefault attempts to load .pedrocli.json from current directory or home
func LoadDefault() (*Config, error) {
	// Try current directory
	if _, err := os.Stat(".pedrocli.json"); err == nil {
		return Load(".pedrocli.json")
	}

	// Try home directory
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(home, ".pedrocli.json")
		if _, err := os.Stat(homePath); err == nil {
			return Load(homePath)
		}
	}

	return nil, fmt.Errorf("no .pedrocli.json found in current directory or home")
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() {
	// Model defaults
	if c.Model.Temperature == 0 {
		c.Model.Temperature = 0.2
	}
	if c.Model.UsableContext == 0 && c.Model.ContextSize > 0 {
		c.Model.UsableContext = c.Model.ContextSize * 3 / 4
	}
	if c.Model.Threads == 0 {
		c.Model.Threads = 8
	}

	// Git defaults
	if c.Git.Remote == "" {
		c.Git.Remote = "origin"
	}
	if c.Git.BranchPrefix == "" {
		c.Git.BranchPrefix = "pedroceli/"
	}

	// Limits defaults
	if c.Limits.MaxTaskDurationMinutes == 0 {
		c.Limits.MaxTaskDurationMinutes = 30
	}
	if c.Limits.MaxInferenceRuns == 0 {
		c.Limits.MaxInferenceRuns = 20
	}

	// Debug defaults
	if c.Debug.LogLevel == "" {
		c.Debug.LogLevel = "info"
	}

	// Platform defaults
	if c.Platform.OS == "" {
		c.Platform.OS = "auto"
	}
	if c.Platform.Shell == "" {
		c.Platform.Shell = "/bin/sh"
	}

	// Tools defaults
	if len(c.Tools.AllowedBashCommands) == 0 {
		c.Tools.AllowedBashCommands = []string{
			"git", "gh", "go", "cat", "ls", "head", "tail", "wc", "sort", "uniq",
		}
	}
	if len(c.Tools.ForbiddenCommands) == 0 {
		c.Tools.ForbiddenCommands = []string{
			"sed", "grep", "find", "xargs", "rm", "mv", "dd", "sudo",
		}
	}

	// LSP defaults
	if c.LSP.Timeout == 0 {
		c.LSP.Timeout = 30
	}
	if c.LSP.Servers == nil {
		c.LSP.Servers = make(map[string]LSPServerDef)
	}

	// FileIO defaults
	if c.FileIO.MaxFileSize == 0 {
		c.FileIO.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}
	if !c.FileIO.AtomicWrites {
		c.FileIO.AtomicWrites = true // Enable by default
	}

	// Web defaults
	if c.Web.Port == 0 {
		c.Web.Port = 8080
	}
	if c.Web.Host == "" {
		c.Web.Host = "0.0.0.0" // Bind to all interfaces for Tailscale/remote access
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate model type
	if c.Model.Type != "llamacpp" && c.Model.Type != "ollama" {
		return fmt.Errorf("invalid model type: %s (must be 'llamacpp' or 'ollama')", c.Model.Type)
	}

	// Validate llama.cpp config
	if c.Model.Type == "llamacpp" {
		if c.Model.ModelPath == "" {
			return fmt.Errorf("model_path is required for llamacpp backend")
		}
		if c.Model.LlamaCppPath == "" {
			return fmt.Errorf("llamacpp_path is required for llamacpp backend")
		}
		if c.Model.ContextSize < 2048 {
			return fmt.Errorf("context_size too small: %d (minimum 2048)", c.Model.ContextSize)
		}
		if c.Model.ContextSize > 200000 {
			return fmt.Errorf("context_size suspiciously large: %d", c.Model.ContextSize)
		}
	}

	// Validate Ollama config
	if c.Model.Type == "ollama" {
		if c.Model.ModelName == "" {
			return fmt.Errorf("model_name is required for ollama backend")
		}
	}

	// Validate LSP config
	if c.LSP.Enabled {
		if c.LSP.Timeout < 1 || c.LSP.Timeout > 300 {
			return fmt.Errorf("LSP timeout must be between 1 and 300 seconds")
		}
		for name, server := range c.LSP.Servers {
			if server.Command == "" {
				return fmt.Errorf("LSP server %s has no command specified", name)
			}
			if len(server.Languages) == 0 {
				return fmt.Errorf("LSP server %s has no languages specified", name)
			}
		}
	}

	// Validate FileIO config (only if explicitly set to a non-default value)
	if c.FileIO.MaxFileSize != 0 && c.FileIO.MaxFileSize < 1024 {
		return fmt.Errorf("max_file_size must be at least 1KB")
	}
	if c.FileIO.MaxFileSize > 100*1024*1024 {
		return fmt.Errorf("max_file_size cannot exceed 100MB")
	}

	return nil
}
