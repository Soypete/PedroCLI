package repos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultBasePath is the default base path for repository storage
	DefaultBasePath = "/var/pedro/repos"

	// SrcDir is the source directory within the base path (GOPATH-style)
	SrcDir = "src"
)

// DefaultManager implements Manager with GOPATH-style storage
type DefaultManager struct {
	basePath    string
	gitOps      GitOps
	store       Store
	hooksManager HooksManager // Will be set after hooks package is implemented
	mu          sync.RWMutex
}

// HooksManager is defined here to avoid circular imports
// The actual implementation is in the hooks package
type HooksManager interface {
	InstallHooks(repoPath string) error
	DetectProjectType(repoPath string) (string, error)
}

// ManagerOption is a functional option for configuring the manager
type ManagerOption func(*DefaultManager)

// WithBasePath sets the base path for repository storage
func WithBasePath(path string) ManagerOption {
	return func(m *DefaultManager) {
		m.basePath = path
	}
}

// WithGitOps sets a custom GitOps implementation
func WithGitOps(gitOps GitOps) ManagerOption {
	return func(m *DefaultManager) {
		m.gitOps = gitOps
	}
}

// WithStore sets a custom Store implementation
func WithStore(store Store) ManagerOption {
	return func(m *DefaultManager) {
		m.store = store
	}
}

// WithHooksManager sets the hooks manager
func WithHooksManager(hm HooksManager) ManagerOption {
	return func(m *DefaultManager) {
		m.hooksManager = hm
	}
}

// NewManager creates a new repository manager
func NewManager(opts ...ManagerOption) *DefaultManager {
	m := &DefaultManager{
		basePath: DefaultBasePath,
		gitOps:   NewGitOps(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// EnsureRepo clones or fetches a repository and returns its local info
func (m *DefaultManager) EnsureRepo(ctx context.Context, provider, owner, repo string) (*LocalRepo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	repoPath := m.getRepoPath(provider, owner, repo)

	// Check if repo exists
	if m.isGitRepo(repoPath) {
		// Fetch to update refs
		if err := m.gitOps.Fetch(ctx, repoPath); err != nil {
			return nil, fmt.Errorf("failed to fetch repo: %w", err)
		}

		return m.buildLocalRepo(ctx, provider, owner, repo, repoPath)
	}

	// Need to clone
	if err := m.ensureParentDir(repoPath); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone the repository
	cloneURL := m.buildCloneURL(provider, owner, repo)
	if err := m.gitOps.Clone(ctx, cloneURL, repoPath); err != nil {
		return nil, fmt.Errorf("failed to clone repo: %w", err)
	}

	// Install hooks if hooks manager is available
	if m.hooksManager != nil {
		if err := m.hooksManager.InstallHooks(repoPath); err != nil {
			// Log but don't fail - hooks are optional
			fmt.Printf("Warning: failed to install hooks: %v\n", err)
		}
	}

	return m.buildLocalRepo(ctx, provider, owner, repo, repoPath)
}

// GetRepoPath returns the local path for a repository without fetching
func (m *DefaultManager) GetRepoPath(provider, owner, repo string) string {
	return m.getRepoPath(provider, owner, repo)
}

// FreshClone removes existing repo and does a fresh clone
func (m *DefaultManager) FreshClone(ctx context.Context, provider, owner, repo string) (*LocalRepo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	repoPath := m.getRepoPath(provider, owner, repo)

	// Remove existing repo if it exists
	if m.isGitRepo(repoPath) {
		if err := os.RemoveAll(repoPath); err != nil {
			return nil, fmt.Errorf("failed to remove existing repo: %w", err)
		}
	}

	// Create parent directory
	if err := m.ensureParentDir(repoPath); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone
	cloneURL := m.buildCloneURL(provider, owner, repo)
	if err := m.gitOps.Clone(ctx, cloneURL, repoPath); err != nil {
		return nil, fmt.Errorf("failed to clone repo: %w", err)
	}

	// Install hooks if hooks manager is available
	if m.hooksManager != nil {
		if err := m.hooksManager.InstallHooks(repoPath); err != nil {
			fmt.Printf("Warning: failed to install hooks: %v\n", err)
		}
	}

	return m.buildLocalRepo(ctx, provider, owner, repo, repoPath)
}

// ListRepos returns all managed repositories
func (m *DefaultManager) ListRepos(ctx context.Context) ([]LocalRepo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var repos []LocalRepo

	srcPath := filepath.Join(m.basePath, SrcDir)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return repos, nil
	}

	// Walk the directory structure: src/{provider}/{owner}/{repo}
	providerDirs, err := os.ReadDir(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, providerDir := range providerDirs {
		if !providerDir.IsDir() {
			continue
		}
		provider := providerDir.Name()

		ownerDirs, err := os.ReadDir(filepath.Join(srcPath, provider))
		if err != nil {
			continue
		}

		for _, ownerDir := range ownerDirs {
			if !ownerDir.IsDir() {
				continue
			}
			owner := ownerDir.Name()

			repoDirs, err := os.ReadDir(filepath.Join(srcPath, provider, owner))
			if err != nil {
				continue
			}

			for _, repoDir := range repoDirs {
				if !repoDir.IsDir() {
					continue
				}
				repoName := repoDir.Name()

				repoPath := m.getRepoPath(provider, owner, repoName)
				if m.isGitRepo(repoPath) {
					localRepo, err := m.buildLocalRepo(ctx, provider, owner, repoName, repoPath)
					if err != nil {
						continue
					}
					repos = append(repos, *localRepo)
				}
			}
		}
	}

	return repos, nil
}

// GetRepo returns a specific repo by provider/owner/name
func (m *DefaultManager) GetRepo(ctx context.Context, provider, owner, repo string) (*LocalRepo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	repoPath := m.getRepoPath(provider, owner, repo)
	if !m.isGitRepo(repoPath) {
		return nil, fmt.Errorf("repository not found: %s/%s/%s", provider, owner, repo)
	}

	return m.buildLocalRepo(ctx, provider, owner, repo, repoPath)
}

// RemoveRepo removes a repository from local storage
func (m *DefaultManager) RemoveRepo(ctx context.Context, provider, owner, repo string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	repoPath := m.getRepoPath(provider, owner, repo)
	if !m.isGitRepo(repoPath) {
		return fmt.Errorf("repository not found: %s/%s/%s", provider, owner, repo)
	}

	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to remove repo: %w", err)
	}

	// Clean up empty parent directories
	m.cleanupEmptyDirs(filepath.Dir(repoPath))

	return nil
}

// UpdateRepoMeta updates repository metadata
func (m *DefaultManager) UpdateRepoMeta(ctx context.Context, repo *LocalRepo) error {
	if m.store == nil {
		return nil // No store configured, metadata is ephemeral
	}

	repo.UpdatedAt = time.Now()
	return m.store.SaveRepo(ctx, repo)
}

// SetBasePath changes the base storage path
func (m *DefaultManager) SetBasePath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.basePath = path
}

// GetBasePath returns the current base storage path
func (m *DefaultManager) GetBasePath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.basePath
}

// Internal helper methods

func (m *DefaultManager) getRepoPath(provider, owner, repo string) string {
	return filepath.Join(m.basePath, SrcDir, provider, owner, repo)
}

func (m *DefaultManager) isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (m *DefaultManager) ensureParentDir(repoPath string) error {
	parentDir := filepath.Dir(repoPath)
	return os.MkdirAll(parentDir, 0755)
}

func (m *DefaultManager) buildCloneURL(provider, owner, repo string) string {
	// TODO: Support SSH URLs based on credential configuration
	return fmt.Sprintf("https://%s/%s/%s.git", provider, owner, repo)
}

func (m *DefaultManager) buildLocalRepo(ctx context.Context, provider, owner, repo, repoPath string) (*LocalRepo, error) {
	// Get current branch
	currentBranch, err := m.gitOps.CurrentBranch(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Get default branch
	defaultBranch, err := m.gitOps.GetDefaultBranch(ctx, repoPath)
	if err != nil {
		defaultBranch = "main" // Fallback
	}

	// Detect project type
	projectType := m.detectProjectType(repoPath)

	// Get last modified time of .git directory
	gitDir := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitDir)
	var lastFetched time.Time
	if err == nil {
		lastFetched = info.ModTime()
	}

	localRepo := &LocalRepo{
		ID:            uuid.New().String(),
		Provider:      provider,
		Owner:         owner,
		Name:          repo,
		LocalPath:     repoPath,
		CurrentRef:    currentBranch,
		DefaultBranch: defaultBranch,
		ProjectType:   projectType,
		LastFetched:   lastFetched,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Try to load from store if available
	if m.store != nil {
		existing, err := m.store.GetRepo(ctx, provider, owner, repo)
		if err == nil && existing != nil {
			localRepo.ID = existing.ID
			localRepo.CreatedAt = existing.CreatedAt
		}
		// Save updated info
		if err := m.store.SaveRepo(ctx, localRepo); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to save repo to store: %v\n", err)
		}
	}

	return localRepo, nil
}

func (m *DefaultManager) detectProjectType(repoPath string) string {
	// Check for common project type indicators
	indicators := map[string][]string{
		"go":     {"go.mod", "go.sum"},
		"node":   {"package.json"},
		"python": {"setup.py", "pyproject.toml", "requirements.txt"},
		"rust":   {"Cargo.toml"},
		"java":   {"pom.xml", "build.gradle"},
		"ruby":   {"Gemfile"},
		"php":    {"composer.json"},
		"dotnet": {"*.csproj", "*.fsproj"},
	}

	for projectType, files := range indicators {
		for _, file := range files {
			matches, _ := filepath.Glob(filepath.Join(repoPath, file))
			if len(matches) > 0 {
				return projectType
			}
		}
	}

	return "unknown"
}

func (m *DefaultManager) cleanupEmptyDirs(path string) {
	srcPath := filepath.Join(m.basePath, SrcDir)

	// Don't go above src directory
	for path != srcPath && path != m.basePath {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(path); err != nil {
			break
		}
		path = filepath.Dir(path)
	}
}

// Ensure DefaultManager implements Manager
var _ Manager = (*DefaultManager)(nil)
