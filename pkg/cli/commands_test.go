package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDir creates a temporary test directory with sample commands
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .pedro/command directory
	cmdDir := filepath.Join(tmpDir, ".pedro", "command")
	require.NoError(t, os.MkdirAll(cmdDir, 0755))

	// Create a sample custom command
	blogOutline := `---
name: blog-outline
description: Generate a blog post outline
agent: blog
---

Write a detailed blog post outline about: $ARGUMENTS

The outline should include:
- Introduction hook
- 3-5 main sections
- Conclusion with call to action
- Suggested word count: 1500-2000 words

Make it engaging and well-structured for technical readers.
`
	err := os.WriteFile(filepath.Join(cmdDir, "blog-outline.md"), []byte(blogOutline), 0644)
	require.NoError(t, err)

	return tmpDir
}

// setupTestConfig creates a minimal test config
func setupTestConfig() *config.Config {
	return &config.Config{
		Model: config.ModelConfig{
			Type:      "ollama",
			ModelName: "qwen2.5-coder:32b",
		},
		Project: config.ProjectConfig{
			Name:    "test-project",
			Workdir: "/tmp/test",
		},
	}
}

func TestNewCommandRunner(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()
	cfg.Project.Workdir = testDir

	runner := NewCommandRunner(cfg, testDir)

	assert.NotNil(t, runner)
	assert.NotNil(t, runner.registry)
	assert.Equal(t, cfg, runner.config)
	assert.Equal(t, testDir, runner.workDir)
}

func TestCommandRunner_ListCommands(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()

	// Change to test directory so relative paths work
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(testDir)

	runner := NewCommandRunner(cfg, testDir)
	commands := runner.ListCommands()

	// Should have builtins at minimum
	assert.GreaterOrEqual(t, len(commands), 1)

	// Check for builtin commands
	foundBuiltin := false
	for _, cmd := range commands {
		if cmd.Name == "help" && cmd.Source == "builtin" {
			foundBuiltin = true
			break
		}
	}
	assert.True(t, foundBuiltin, "Builtin commands should be loaded")

	// Check for our custom command
	found := false
	for _, cmd := range commands {
		if cmd.Name == "blog-outline" {
			found = true
			assert.Equal(t, "Generate a blog post outline", cmd.Description)
			assert.Equal(t, "blog", cmd.Agent)
			break
		}
	}
	assert.True(t, found, "Custom command 'blog-outline' should be loaded")
}

func TestCommandRunner_GetCommand(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()

	// Change to test directory so relative paths work
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(testDir)

	runner := NewCommandRunner(cfg, testDir)

	t.Run("existing command", func(t *testing.T) {
		cmd, ok := runner.GetCommand("blog-outline")
		assert.True(t, ok)
		assert.NotNil(t, cmd)
		assert.Equal(t, "blog-outline", cmd.Name)
		assert.Equal(t, "blog", cmd.Agent)
	})

	t.Run("non-existing command", func(t *testing.T) {
		cmd, ok := runner.GetCommand("does-not-exist")
		assert.False(t, ok)
		assert.Nil(t, cmd)
	})
}

func TestCommandRunner_ExpandCommand(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()

	// Change to test directory so relative paths work
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(testDir)

	runner := NewCommandRunner(cfg, testDir)

	tests := []struct {
		name           string
		commandName    string
		args           []string
		expectedSubstr []string
		expectError    bool
	}{
		{
			name:        "blog-outline with argument",
			commandName: "blog-outline",
			args:        []string{"Building", "CLI", "Tools"},
			expectedSubstr: []string{
				"Write a detailed blog post outline about: Building",
				"Introduction hook",
				"3-5 main sections",
			},
			expectError: false,
		},
		{
			name:        "non-existent command",
			commandName: "does-not-exist",
			args:        []string{"arg"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded, err := runner.ExpandCommand(tt.commandName, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			for _, substr := range tt.expectedSubstr {
				assert.Contains(t, expanded, substr,
					"Expanded prompt should contain: %s", substr)
			}
		})
	}
}

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedName  string
		expectedArgs  []string
		expectedValid bool
	}{
		{
			name:          "simple command",
			input:         "/test",
			expectedName:  "test",
			expectedArgs:  []string{},
			expectedValid: true,
		},
		{
			name:          "command with single arg",
			input:         "/blog-outline Go Contexts",
			expectedName:  "blog-outline",
			expectedArgs:  []string{"Go", "Contexts"},
			expectedValid: true,
		},
		{
			name:          "command with multiple args",
			input:         "/blog-outline Building CLI Tools in Go",
			expectedName:  "blog-outline",
			expectedArgs:  []string{"Building", "CLI", "Tools", "in", "Go"},
			expectedValid: true,
		},
		{
			name:          "not a slash command",
			input:         "regular text",
			expectedName:  "",
			expectedArgs:  nil,
			expectedValid: false,
		},
		{
			name:          "slash only",
			input:         "/",
			expectedName:  "",
			expectedArgs:  nil,
			expectedValid: false,
		},
		{
			name:          "with extra whitespace",
			input:         "  /test   arg1   arg2  ",
			expectedName:  "test",
			expectedArgs:  []string{"arg1", "arg2"},
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args, isValid := ParseSlashCommand(tt.input)

			assert.Equal(t, tt.expectedValid, isValid)
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestCommandRunner_RunCommand(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()

	// Change to test directory so relative paths work
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(testDir)

	runner := NewCommandRunner(cfg, testDir)
	ctx := context.Background()

	t.Run("valid slash command", func(t *testing.T) {
		expanded, err := runner.RunCommand(ctx, "/blog-outline Go Patterns")
		assert.NoError(t, err)
		assert.Contains(t, expanded, "Go Patterns")
		assert.Contains(t, expanded, "blog post outline")
	})

	t.Run("invalid input - not a slash command", func(t *testing.T) {
		_, err := runner.RunCommand(ctx, "not a slash command")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a slash command")
	})

	t.Run("command not found", func(t *testing.T) {
		_, err := runner.RunCommand(ctx, "/nonexistent command")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command not found")
	})
}

func TestCommandRunner_PrintHelp(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()

	runner := NewCommandRunner(cfg, testDir)

	// This test just ensures PrintHelp doesn't panic
	// In a real scenario, you might capture stdout and verify output
	assert.NotPanics(t, func() {
		runner.PrintHelp()
	})
}

// Integration test: Verify builtin commands work
func TestCommandRunner_BuiltinCommands(t *testing.T) {
	testDir := setupTestDir(t)
	cfg := setupTestConfig()

	runner := NewCommandRunner(cfg, testDir)

	// Test /test builtin (if it exists)
	cmd, ok := runner.GetCommand("test")
	if ok {
		assert.Equal(t, "builtin", cmd.Source)

		// Try expanding it
		expanded, err := runner.ExpandCommand("test", []string{})
		assert.NoError(t, err)
		assert.NotEmpty(t, expanded)
	}
}
