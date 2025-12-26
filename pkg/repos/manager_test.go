package repos

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultManager_GetRepoPath(t *testing.T) {
	manager := NewManager(WithBasePath("/tmp/test-repos"))

	tests := []struct {
		provider string
		owner    string
		repo     string
		expected string
	}{
		{
			provider: "github.com",
			owner:    "soypete",
			repo:     "pedro-cli",
			expected: "/tmp/test-repos/src/github.com/soypete/pedro-cli",
		},
		{
			provider: "gitlab.com",
			owner:    "myorg",
			repo:     "myproject",
			expected: "/tmp/test-repos/src/gitlab.com/myorg/myproject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.owner+"/"+tt.repo, func(t *testing.T) {
			result := manager.GetRepoPath(tt.provider, tt.owner, tt.repo)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDefaultManager_DetectProjectType(t *testing.T) {
	// Create temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "test-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "go project",
			files:    []string{"go.mod", "main.go"},
			expected: "go",
		},
		{
			name:     "node project",
			files:    []string{"package.json", "index.js"},
			expected: "node",
		},
		{
			name:     "python project",
			files:    []string{"requirements.txt", "app.py"},
			expected: "python",
		},
		{
			name:     "rust project",
			files:    []string{"Cargo.toml", "src/main.rs"},
			expected: "rust",
		},
		{
			name:     "unknown project",
			files:    []string{"README.md"},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Create test files
			for _, f := range tt.files {
				filePath := filepath.Join(testDir, f)
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			}

			manager := NewManager()
			result := manager.detectProjectType(testDir)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestLocalRepo_FullName(t *testing.T) {
	repo := &LocalRepo{
		Provider: "github.com",
		Owner:    "soypete",
		Name:     "pedro-cli",
	}

	expected := "github.com/soypete/pedro-cli"
	if repo.FullName() != expected {
		t.Errorf("expected %s, got %s", expected, repo.FullName())
	}
}

func TestLocalRepo_CloneURL(t *testing.T) {
	repo := &LocalRepo{
		Provider: "github.com",
		Owner:    "soypete",
		Name:     "pedro-cli",
	}

	expected := "https://github.com/soypete/pedro-cli.git"
	if repo.CloneURL() != expected {
		t.Errorf("expected %s, got %s", expected, repo.CloneURL())
	}
}

func TestLocalRepo_SSHCloneURL(t *testing.T) {
	repo := &LocalRepo{
		Provider: "github.com",
		Owner:    "soypete",
		Name:     "pedro-cli",
	}

	expected := "git@github.com:soypete/pedro-cli.git"
	if repo.SSHCloneURL() != expected {
		t.Errorf("expected %s, got %s", expected, repo.SSHCloneURL())
	}
}

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		url      string
		provider string
		owner    string
		repo     string
		hasError bool
	}{
		{
			url:      "https://github.com/soypete/pedro-cli.git",
			provider: "github.com",
			owner:    "soypete",
			repo:     "pedro-cli",
			hasError: false,
		},
		{
			url:      "git@github.com:soypete/pedro-cli.git",
			provider: "github.com",
			owner:    "soypete",
			repo:     "pedro-cli",
			hasError: false,
		},
		{
			url:      "https://gitlab.com/myorg/myproject.git",
			provider: "gitlab.com",
			owner:    "myorg",
			repo:     "myproject",
			hasError: false,
		},
		{
			url:      "invalid-url",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			provider, owner, repo, err := ParseRepoURL(tt.url)

			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if provider != tt.provider {
				t.Errorf("provider: expected %s, got %s", tt.provider, provider)
			}
			if owner != tt.owner {
				t.Errorf("owner: expected %s, got %s", tt.owner, owner)
			}
			if repo != tt.repo {
				t.Errorf("repo: expected %s, got %s", tt.repo, repo)
			}
		})
	}
}

func TestDefaultManager_SetBasePath(t *testing.T) {
	manager := NewManager()

	originalPath := manager.GetBasePath()
	newPath := "/custom/repos"

	manager.SetBasePath(newPath)
	if manager.GetBasePath() != newPath {
		t.Errorf("expected %s, got %s", newPath, manager.GetBasePath())
	}

	// Reset
	manager.SetBasePath(originalPath)
}

func TestDefaultManager_ListRepos_EmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-repos-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager(WithBasePath(tmpDir))

	repos, err := manager.ListRepos(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}
