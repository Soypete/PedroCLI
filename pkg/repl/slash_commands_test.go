package repl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soypete/pedrocli/pkg/cli"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestCommandDir creates a test directory with sample commands
func setupTestCommandDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .pedro/command directory
	cmdDir := filepath.Join(tmpDir, ".pedro", "command")
	require.NoError(t, os.MkdirAll(cmdDir, 0755))

	// Create test command
	testCmd := `---
description: Test command for REPL
agent: build
---

Run tests and analyze:

!` + "`go test ./...`" + `

Fix any failing tests.
`
	err := os.WriteFile(filepath.Join(cmdDir, "test.md"), []byte(testCmd), 0644)
	require.NoError(t, err)

	// Create blog outline command
	blogCmd := `---
description: Generate blog outline
agent: blog
---

Create a blog outline about: $ARGUMENTS

Include:
- Title
- TLDR (3-5 bullets)
- Main sections
- Conclusion
`
	err = os.WriteFile(filepath.Join(cmdDir, "blog-outline.md"), []byte(blogCmd), 0644)
	require.NoError(t, err)

	return tmpDir
}

func TestParseCommand_SlashCommands(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedType CommandType
		expectedName string
		numArgs      int
	}{
		{
			name:         "REPL command",
			input:        "/help",
			expectedType: CommandTypeREPL,
			expectedName: "help",
			numArgs:      0,
		},
		{
			name:         "Slash command - no args",
			input:        "/test",
			expectedType: CommandTypeSlash,
			expectedName: "test",
			numArgs:      0,
		},
		{
			name:         "Slash command - with args",
			input:        "/blog-outline Building CLI Tools in Go",
			expectedType: CommandTypeSlash,
			expectedName: "blog-outline",
			numArgs:      5, // Building, CLI, Tools, in, Go
		},
		{
			name:         "Natural language",
			input:        "build a rate limiter",
			expectedType: CommandTypeNatural,
			expectedName: "",
			numArgs:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ParseCommand(tt.input)

			assert.Equal(t, tt.expectedType, cmd.Type, "Command type mismatch")
			assert.Equal(t, tt.expectedName, cmd.Name, "Command name mismatch")
			assert.Equal(t, tt.numArgs, len(cmd.Args), "Args count mismatch")
		})
	}
}

func TestIsREPLCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{"help", "help", true},
		{"quit", "quit", true},
		{"history", "history", true},
		{"clear", "clear", true},
		{"mode", "mode", true},
		{"context", "context", true},
		{"logs", "logs", true},
		{"interactive", "interactive", true},
		{"background", "background", true},
		{"unknown", "unknown-command", false},
		{"test", "test", false}, // Not a REPL command, it's a slash command
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isREPLCommand(tt.command)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlashCommandIntegration(t *testing.T) {
	// Setup test directory
	testDir := setupTestCommandDir(t)

	// Change to test directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(testDir)

	// Create test config
	cfg := &config.Config{
		Model: config.ModelConfig{
			Type:      "ollama",
			ModelName: "qwen2.5-coder:32b",
		},
		Project: config.ProjectConfig{
			Name:    "test-project",
			Workdir: testDir,
		},
	}

	// Create command runner
	runner := cli.NewCommandRunner(cfg, testDir)

	t.Run("list commands", func(t *testing.T) {
		commands := runner.ListCommands()

		// Should have at least our 2 custom commands + builtins
		assert.GreaterOrEqual(t, len(commands), 2)

		// Find our custom commands
		foundTest := false
		foundBlog := false

		for _, cmd := range commands {
			if cmd.Name == "test" {
				foundTest = true
				assert.Equal(t, "Test command for REPL", cmd.Description)
				assert.Equal(t, "build", cmd.Agent)
			}
			if cmd.Name == "blog-outline" {
				foundBlog = true
				assert.Equal(t, "Generate blog outline", cmd.Description)
				assert.Equal(t, "blog", cmd.Agent)
			}
		}

		assert.True(t, foundTest, "test command should be loaded")
		assert.True(t, foundBlog, "blog-outline command should be loaded")
	})

	t.Run("parse slash command input", func(t *testing.T) {
		input := "/blog-outline Go Patterns"

		name, args, isCmd := cli.ParseSlashCommand(input)

		assert.True(t, isCmd)
		assert.Equal(t, "blog-outline", name)
		assert.Equal(t, []string{"Go", "Patterns"}, args)
	})
}

func TestGetREPLHelp(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		expectedSubstrs []string
	}{
		{
			name: "code mode",
			mode: "code",
			expectedSubstrs: []string{
				"REPL Commands",
				"/help",
				"/quit",
				"Code Mode Agents",
				"build",
				"debug",
				"Slash Commands",
			},
		},
		{
			name: "blog mode",
			mode: "blog",
			expectedSubstrs: []string{
				"REPL Commands",
				"Blog Mode",
				"Slash Commands",
			},
		},
		{
			name: "podcast mode",
			mode: "podcast",
			expectedSubstrs: []string{
				"REPL Commands",
				"Podcast Mode",
				"Slash Commands",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			help := GetREPLHelp(tt.mode)

			for _, substr := range tt.expectedSubstrs {
				assert.Contains(t, help, substr,
					"Help text should contain: %s", substr)
			}
		})
	}
}
