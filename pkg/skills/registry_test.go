package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSkillRegistry(t *testing.T) {
	registry := NewSkillRegistry()

	if registry == nil {
		t.Fatal("expected registry to be created")
	}

	if len(registry.skills) != 0 {
		t.Error("expected empty skills map initially")
	}
}

func TestSkillRegistry_Discover(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	skillContent := `---
name: test-skill
description: A test skill for testing
category: testing
tags:
  - test
  - example
---

## What This Skill Does

This is a test skill content.

## How to Use It

Use it for testing purposes.
`
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to create SKILL.md: %v", err)
	}

	registry := NewSkillRegistry()
	registry.AddSearchPath(tmpDir)

	if err := registry.Discover(); err != nil {
		t.Fatalf("failed to discover skills: %v", err)
	}

	skill, ok := registry.Get("test-skill")
	if !ok {
		t.Fatal("expected test-skill to be discovered")
	}

	if skill.Description != "A test skill for testing" {
		t.Errorf("expected description, got %q", skill.Description)
	}
	if skill.Category != "testing" {
		t.Errorf("expected category 'testing', got %q", skill.Category)
	}
	if len(skill.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(skill.Tags))
	}
}

func TestSkillRegistry_Load(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "load-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	skillContent := `---
name: load-skill
---

Skill body content here.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to create SKILL.md: %v", err)
	}

	registry := NewSkillRegistry()
	registry.AddSearchPath(tmpDir)
	registry.Discover()

	content, err := registry.Load("load-skill")
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	if content != "\nSkill body content here.\n" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestSkillRegistry_ListByCategory(t *testing.T) {
	registry := NewSkillRegistry()

	registry.Register(&Skill{
		Name:     "skill1",
		Category: "git",
	})
	registry.Register(&Skill{
		Name:     "skill2",
		Category: "testing",
	})
	registry.Register(&Skill{
		Name:     "skill3",
		Category: "git",
	})

	gitSkills := registry.ListByCategory("git")
	if len(gitSkills) != 2 {
		t.Errorf("expected 2 git skills, got %d", len(gitSkills))
	}

	testingSkills := registry.ListByCategory("testing")
	if len(testingSkills) != 1 {
		t.Errorf("expected 1 testing skill, got %d", len(testingSkills))
	}
}

func TestSkillRegistry_Search(t *testing.T) {
	registry := NewSkillRegistry()

	registry.Register(&Skill{
		Name:        "git-release",
		Description: "Create releases with changelogs",
		Tags:        []string{"git", "release"},
	})
	registry.Register(&Skill{
		Name:        "api-design",
		Description: "Design REST APIs",
		Tags:        []string{"api", "design"},
	})

	// Search by name
	results := registry.Search("git")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'git', got %d", len(results))
	}

	// Search by description
	results = registry.Search("rest")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'rest', got %d", len(results))
	}

	// Search by tag
	results = registry.Search("release")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'release' tag, got %d", len(results))
	}
}

func TestSkillRegistry_GetToolDescription(t *testing.T) {
	registry := NewSkillRegistry()

	registry.Register(&Skill{
		Name:        "test-skill",
		Description: "Test description",
		Category:    "test",
	})

	desc := registry.GetToolDescription()
	if desc == "" {
		t.Error("expected non-empty tool description")
	}
	if !contains(desc, "<available_skills>") {
		t.Error("expected XML format in description")
	}
	if !contains(desc, "test-skill") {
		t.Error("expected skill name in description")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
