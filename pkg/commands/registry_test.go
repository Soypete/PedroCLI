package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCommandRegistry(t *testing.T) {
	registry := NewCommandRegistry("/tmp")

	// Check that builtins are registered
	builtins := []string{"help", "clear", "undo", "redo", "compact", "status"}
	for _, name := range builtins {
		if !registry.IsBuiltin(name) {
			t.Errorf("expected builtin command %s to be registered", name)
		}
	}
}

func TestCommandRegistry_Register(t *testing.T) {
	registry := NewCommandRegistry("/tmp")

	cmd := &Command{
		Name:        "test",
		Description: "Test command",
		Template:    "Test template: $ARGUMENTS",
	}
	registry.Register("test", cmd)

	retrieved, ok := registry.Get("test")
	if !ok {
		t.Fatal("expected test command to be registered")
	}
	if retrieved.Template != "Test template: $ARGUMENTS" {
		t.Error("expected template to match")
	}
}

func TestCommandRegistry_ExpandTemplate(t *testing.T) {
	registry := NewCommandRegistry("/tmp")

	tests := []struct {
		name     string
		template string
		args     []string
		want     string
	}{
		{
			name:     "replace ARGUMENTS",
			template: "Review: $ARGUMENTS",
			args:     []string{"file.go", "main.go"},
			want:     "Review: file.go main.go",
		},
		{
			name:     "replace positional args",
			template: "Compare $1 with $2",
			args:     []string{"old.go", "new.go"},
			want:     "Compare old.go with new.go",
		},
		{
			name:     "mixed replacements",
			template: "File: $1, All: $ARGUMENTS",
			args:     []string{"main.go", "extra"},
			want:     "File: main.go, All: main.go extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := registry.ExpandTemplate(tt.template, tt.args, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.want {
				t.Errorf("got %q, want %q", result, tt.want)
			}
		})
	}
}

func TestCommandRegistry_ExpandFileReferences(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("file content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	registry := NewCommandRegistry(tmpDir)

	template := "Content: @test.txt"
	result, err := registry.ExpandTemplate(template, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "file content") {
		t.Errorf("expected file content in result, got: %s", result)
	}
}

func TestCommandRegistry_LoadMarkdownCommands(t *testing.T) {
	tmpDir := t.TempDir()
	cmdContent := `---
description: Test command from markdown
agent: build
---

Execute the test: $ARGUMENTS
`
	cmdPath := filepath.Join(tmpDir, "test-cmd.md")
	if err := os.WriteFile(cmdPath, []byte(cmdContent), 0644); err != nil {
		t.Fatalf("failed to create test command file: %v", err)
	}

	registry := NewCommandRegistry("/tmp")
	err := registry.LoadMarkdownCommands(tmpDir)
	if err != nil {
		t.Fatalf("failed to load markdown commands: %v", err)
	}

	cmd, ok := registry.Get("test-cmd")
	if !ok {
		t.Fatal("expected test-cmd to be loaded")
	}
	if cmd.Description != "Test command from markdown" {
		t.Errorf("expected description to match, got %q", cmd.Description)
	}
	if cmd.Agent != "build" {
		t.Errorf("expected agent 'build', got %q", cmd.Agent)
	}
	if cmd.Source != "markdown" {
		t.Errorf("expected source 'markdown', got %q", cmd.Source)
	}
}

func TestCommandRegistry_LoadFromConfig(t *testing.T) {
	registry := NewCommandRegistry("/tmp")

	configs := map[string]CommandConfig{
		"config-cmd": {
			Template:    "Config template: $1",
			Description: "Command from config",
			Agent:       "plan",
		},
	}

	err := registry.LoadFromConfig(configs)
	if err != nil {
		t.Fatalf("failed to load from config: %v", err)
	}

	cmd, ok := registry.Get("config-cmd")
	if !ok {
		t.Fatal("expected config-cmd to be loaded")
	}
	if cmd.Source != "config" {
		t.Errorf("expected source 'config', got %q", cmd.Source)
	}
}

func TestCommandRegistry_List(t *testing.T) {
	registry := NewCommandRegistry("/tmp")

	// Add a custom command
	registry.Register("custom", &Command{
		Name:     "custom",
		Template: "test",
	})

	list := registry.List()
	if len(list) < 7 { // 6 builtins + 1 custom
		t.Errorf("expected at least 7 commands, got %d", len(list))
	}

	// Check that both builtins and custom are in the list
	hasBuiltin := false
	hasCustom := false
	for _, cmd := range list {
		if cmd.Name == "help" {
			hasBuiltin = true
		}
		if cmd.Name == "custom" {
			hasCustom = true
		}
	}
	if !hasBuiltin {
		t.Error("expected builtin 'help' in list")
	}
	if !hasCustom {
		t.Error("expected 'custom' in list")
	}
}

func TestBuiltinCommands(t *testing.T) {
	registry := NewCommandRegistry("/tmp")

	tests := []struct {
		name   string
		args   []string
		expect string
	}{
		{"help", nil, "Tab"},
		{"clear", nil, "__CLEAR__"},
		{"undo", nil, "__UNDO__"},
		{"redo", nil, "__REDO__"},
		{"compact", nil, "__COMPACT__"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := registry.ExecuteBuiltin(tt.name, tt.args, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result, tt.expect) {
				t.Errorf("expected result to contain %q, got %q", tt.expect, result)
			}
		})
	}
}
