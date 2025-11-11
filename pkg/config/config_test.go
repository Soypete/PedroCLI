package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		errMsg    string
		validate  func(*testing.T, *Config)
	}{
		{
			name: "valid llamacpp config",
			content: `{
				"model": {
					"type": "llamacpp",
					"model_path": "/models/test.gguf",
					"llamacpp_path": "/usr/local/bin/llama-cli",
					"context_size": 32768
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if c.Model.Type != "llamacpp" {
					t.Errorf("Model.Type = %v, want llamacpp", c.Model.Type)
				}
				if c.Model.ContextSize != 32768 {
					t.Errorf("Model.ContextSize = %v, want 32768", c.Model.ContextSize)
				}
				// Check defaults were set
				if c.Model.Temperature == 0 {
					t.Error("Temperature should have default value")
				}
				if c.Git.Remote != "origin" {
					t.Errorf("Git.Remote = %v, want origin", c.Git.Remote)
				}
			},
		},
		{
			name: "valid ollama config",
			content: `{
				"model": {
					"type": "ollama",
					"model_name": "qwen2.5-coder:32b"
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if c.Model.Type != "ollama" {
					t.Errorf("Model.Type = %v, want ollama", c.Model.Type)
				}
				if c.Model.ModelName != "qwen2.5-coder:32b" {
					t.Errorf("Model.ModelName = %v, want qwen2.5-coder:32b", c.Model.ModelName)
				}
			},
		},
		{
			name: "invalid model type",
			content: `{
				"model": {
					"type": "invalid"
				}
			}`,
			wantErr: true,
			errMsg:  "invalid model type",
		},
		{
			name: "llamacpp missing model_path",
			content: `{
				"model": {
					"type": "llamacpp",
					"llamacpp_path": "/usr/local/bin/llama-cli",
					"context_size": 32768
				}
			}`,
			wantErr: true,
			errMsg:  "model_path is required",
		},
		{
			name: "llamacpp missing llamacpp_path",
			content: `{
				"model": {
					"type": "llamacpp",
					"model_path": "/models/test.gguf",
					"context_size": 32768
				}
			}`,
			wantErr: true,
			errMsg:  "llamacpp_path is required",
		},
		{
			name: "context_size too small",
			content: `{
				"model": {
					"type": "llamacpp",
					"model_path": "/models/test.gguf",
					"llamacpp_path": "/usr/local/bin/llama-cli",
					"context_size": 1024
				}
			}`,
			wantErr: true,
			errMsg:  "context_size too small",
		},
		{
			name: "context_size too large",
			content: `{
				"model": {
					"type": "llamacpp",
					"model_path": "/models/test.gguf",
					"llamacpp_path": "/usr/local/bin/llama-cli",
					"context_size": 300000
				}
			}`,
			wantErr: true,
			errMsg:  "context_size suspiciously large",
		},
		{
			name: "ollama missing model_name",
			content: `{
				"model": {
					"type": "ollama"
				}
			}`,
			wantErr: true,
			errMsg:  "model_name is required",
		},
		{
			name:    "invalid json",
			content: `{invalid json`,
			wantErr: true,
			errMsg:  "failed to parse config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.json")

			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Load config
			got, err := Load(tmpFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Load() error = nil, want error containing %q", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Load() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Load() unexpected error = %v", err)
				return
			}

			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Error("Load() should error for nonexistent file")
	}
	if !contains(err.Error(), "failed to read config file") {
		t.Errorf("Load() error = %q, want error containing 'failed to read config file'", err.Error())
	}
}

func TestSetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		validate func(*testing.T, *Config)
	}{
		{
			name:   "empty config gets all defaults",
			config: Config{},
			validate: func(t *testing.T, c *Config) {
				if c.Model.Temperature != 0.2 {
					t.Errorf("Model.Temperature = %v, want 0.2", c.Model.Temperature)
				}
				if c.Model.Threads != 8 {
					t.Errorf("Model.Threads = %v, want 8", c.Model.Threads)
				}
				if c.Git.Remote != "origin" {
					t.Errorf("Git.Remote = %v, want origin", c.Git.Remote)
				}
				if c.Git.BranchPrefix != "pedrocli/" {
					t.Errorf("Git.BranchPrefix = %v, want pedrocli/", c.Git.BranchPrefix)
				}
				if c.Limits.MaxTaskDurationMinutes != 30 {
					t.Errorf("Limits.MaxTaskDurationMinutes = %v, want 30", c.Limits.MaxTaskDurationMinutes)
				}
				if c.Limits.MaxInferenceRuns != 20 {
					t.Errorf("Limits.MaxInferenceRuns = %v, want 20", c.Limits.MaxInferenceRuns)
				}
				if c.Debug.LogLevel != "info" {
					t.Errorf("Debug.LogLevel = %v, want info", c.Debug.LogLevel)
				}
				if c.Platform.OS != "auto" {
					t.Errorf("Platform.OS = %v, want auto", c.Platform.OS)
				}
				if c.Platform.Shell != "/bin/sh" {
					t.Errorf("Platform.Shell = %v, want /bin/sh", c.Platform.Shell)
				}
			},
		},
		{
			name: "usable_context calculated from context_size",
			config: Config{
				Model: ModelConfig{
					ContextSize: 32768,
				},
			},
			validate: func(t *testing.T, c *Config) {
				expected := 32768 * 3 / 4 // 75%
				if c.Model.UsableContext != expected {
					t.Errorf("Model.UsableContext = %v, want %v", c.Model.UsableContext, expected)
				}
			},
		},
		{
			name: "custom values not overridden",
			config: Config{
				Model: ModelConfig{
					Temperature:   0.7,
					Threads:       16,
					ContextSize:   32768,
					UsableContext: 20000, // custom value
				},
				Git: GitConfig{
					Remote:       "upstream",
					BranchPrefix: "custom/",
				},
			},
			validate: func(t *testing.T, c *Config) {
				if c.Model.Temperature != 0.7 {
					t.Errorf("Model.Temperature = %v, want 0.7 (custom)", c.Model.Temperature)
				}
				if c.Model.Threads != 16 {
					t.Errorf("Model.Threads = %v, want 16 (custom)", c.Model.Threads)
				}
				if c.Model.UsableContext != 20000 {
					t.Errorf("Model.UsableContext = %v, want 20000 (custom)", c.Model.UsableContext)
				}
				if c.Git.Remote != "upstream" {
					t.Errorf("Git.Remote = %v, want upstream (custom)", c.Git.Remote)
				}
			},
		},
		{
			name:   "allowed bash commands default",
			config: Config{},
			validate: func(t *testing.T, c *Config) {
				if len(c.Tools.AllowedBashCommands) == 0 {
					t.Error("Tools.AllowedBashCommands should have defaults")
				}
				// Check for a few expected commands
				hasGit := false
				for _, cmd := range c.Tools.AllowedBashCommands {
					if cmd == "git" {
						hasGit = true
					}
				}
				if !hasGit {
					t.Error("Tools.AllowedBashCommands should include 'git'")
				}
			},
		},
		{
			name:   "forbidden commands default",
			config: Config{},
			validate: func(t *testing.T, c *Config) {
				if len(c.Tools.ForbiddenCommands) == 0 {
					t.Error("Tools.ForbiddenCommands should have defaults")
				}
				// Check for a few expected forbidden commands
				hasSed := false
				for _, cmd := range c.Tools.ForbiddenCommands {
					if cmd == "sed" {
						hasSed = true
					}
				}
				if !hasSed {
					t.Error("Tools.ForbiddenCommands should include 'sed'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			config.setDefaults()
			tt.validate(t, &config)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid llamacpp config",
			config: Config{
				Model: ModelConfig{
					Type:         "llamacpp",
					ModelPath:    "/models/test.gguf",
					LlamaCppPath: "/usr/local/bin/llama-cli",
					ContextSize:  32768,
				},
			},
			wantErr: false,
		},
		{
			name: "valid ollama config",
			config: Config{
				Model: ModelConfig{
					Type:      "ollama",
					ModelName: "qwen2.5-coder:32b",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid model type",
			config: Config{
				Model: ModelConfig{
					Type: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid model type",
		},
		{
			name: "llamacpp missing model_path",
			config: Config{
				Model: ModelConfig{
					Type:         "llamacpp",
					LlamaCppPath: "/usr/local/bin/llama-cli",
					ContextSize:  32768,
				},
			},
			wantErr: true,
			errMsg:  "model_path is required",
		},
		{
			name: "llamacpp missing llamacpp_path",
			config: Config{
				Model: ModelConfig{
					Type:        "llamacpp",
					ModelPath:   "/models/test.gguf",
					ContextSize: 32768,
				},
			},
			wantErr: true,
			errMsg:  "llamacpp_path is required",
		},
		{
			name: "context_size too small",
			config: Config{
				Model: ModelConfig{
					Type:         "llamacpp",
					ModelPath:    "/models/test.gguf",
					LlamaCppPath: "/usr/local/bin/llama-cli",
					ContextSize:  1024,
				},
			},
			wantErr: true,
			errMsg:  "context_size too small",
		},
		{
			name: "context_size exactly minimum (2048)",
			config: Config{
				Model: ModelConfig{
					Type:         "llamacpp",
					ModelPath:    "/models/test.gguf",
					LlamaCppPath: "/usr/local/bin/llama-cli",
					ContextSize:  2048,
				},
			},
			wantErr: false,
		},
		{
			name: "context_size too large",
			config: Config{
				Model: ModelConfig{
					Type:         "llamacpp",
					ModelPath:    "/models/test.gguf",
					LlamaCppPath: "/usr/local/bin/llama-cli",
					ContextSize:  300000,
				},
			},
			wantErr: true,
			errMsg:  "context_size suspiciously large",
		},
		{
			name: "context_size exactly maximum (200000)",
			config: Config{
				Model: ModelConfig{
					Type:         "llamacpp",
					ModelPath:    "/models/test.gguf",
					LlamaCppPath: "/usr/local/bin/llama-cli",
					ContextSize:  200000,
				},
			},
			wantErr: false,
		},
		{
			name: "ollama missing model_name",
			config: Config{
				Model: ModelConfig{
					Type: "ollama",
				},
			},
			wantErr: true,
			errMsg:  "model_name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, want error containing %q", tt.errMsg)
					return
				}
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestLoadDefault(t *testing.T) {
	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Test 1: No config file exists
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	_, err = LoadDefault()
	if err == nil {
		t.Error("LoadDefault() should error when no config file exists")
	}
	if !contains(err.Error(), "no .pedrocli.json found") {
		t.Errorf("LoadDefault() error = %q, want error containing 'no .pedrocli.json found'", err.Error())
	}

	// Test 2: Config in current directory
	validConfig := `{
		"model": {
			"type": "llamacpp",
			"model_path": "/models/test.gguf",
			"llamacpp_path": "/usr/local/bin/llama-cli",
			"context_size": 32768
		}
	}`

	if err := os.WriteFile(".pedrocli.json", []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Errorf("LoadDefault() unexpected error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadDefault() returned nil config")
	}
	if cfg.Model.Type != "llamacpp" {
		t.Errorf("LoadDefault() Model.Type = %v, want llamacpp", cfg.Model.Type)
	}
}

func TestConfigJSONRoundTrip(t *testing.T) {
	// Create a config, marshal it, unmarshal it, and verify
	original := Config{
		Model: ModelConfig{
			Type:         "llamacpp",
			ModelPath:    "/models/test.gguf",
			LlamaCppPath: "/usr/local/bin/llama-cli",
			ContextSize:  32768,
			Temperature:  0.2,
		},
		Project: ProjectConfig{
			Name:      "TestProject",
			Workdir:   "/tmp/test",
			TechStack: []string{"go", "python"},
		},
		Git: GitConfig{
			Remote:       "origin",
			BranchPrefix: "test/",
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Unmarshal
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify key fields
	if decoded.Model.Type != original.Model.Type {
		t.Errorf("Model.Type = %v, want %v", decoded.Model.Type, original.Model.Type)
	}
	if decoded.Model.ContextSize != original.Model.ContextSize {
		t.Errorf("Model.ContextSize = %v, want %v", decoded.Model.ContextSize, original.Model.ContextSize)
	}
	if decoded.Project.Name != original.Project.Name {
		t.Errorf("Project.Name = %v, want %v", decoded.Project.Name, original.Project.Name)
	}
	if len(decoded.Project.TechStack) != len(original.Project.TechStack) {
		t.Errorf("Project.TechStack length = %v, want %v", len(decoded.Project.TechStack), len(original.Project.TechStack))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
