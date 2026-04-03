package memory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ResumeLoader struct {
	store       MemoryStore
	gitRepoPath string
}

func NewResumeLoader(store MemoryStore, gitRepoPath string) *ResumeLoader {
	return &ResumeLoader{
		store:       store,
		gitRepoPath: gitRepoPath,
	}
}

func (r *ResumeLoader) LoadResume(ctx context.Context, repoID string) (*ResumePacket, error) {
	packet, err := r.store.LoadLatestResumePacket(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("no resume packet found: %w", err)
	}

	return packet, nil
}

func (r *ResumeLoader) ValidateResume(ctx context.Context, packet *ResumePacket) ([]string, error) {
	var errors []string

	if packet.Branch != "" {
		currentBranch, err := r.getCurrentBranch()
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to get current branch: %v", err))
		} else if currentBranch != packet.Branch {
			errors = append(errors, fmt.Sprintf("branch mismatch: expected %s, got %s", packet.Branch, currentBranch))
		}
	}

	for _, file := range packet.ChangedFiles {
		absPath := filepath.Join(r.gitRepoPath, file)
		info, err := os.Stat(absPath)
		if os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("file not found: %s", file))
		} else if err != nil {
			errors = append(errors, fmt.Sprintf("error checking file %s: %v", file, err))
		} else if info.IsDir() {
			errors = append(errors, fmt.Sprintf("expected file but found directory: %s", file))
		}
	}

	for _, blocker := range packet.Warnings {
		if strings.Contains(blocker, "test") || strings.Contains(blocker, "fail") {
			hasFailing, err := r.checkForFailingTests()
			if err == nil && hasFailing {
				errors = append(errors, fmt.Sprintf("previous failure still present: %s", blocker))
			}
		}
	}

	packet.Validated = len(errors) == 0
	packet.ValidationErrors = errors

	return errors, nil
}

func (r *ResumeLoader) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = r.gitRepoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git command failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *ResumeLoader) checkForFailingTests() (bool, error) {
	cmd := exec.Command("go", "test", "./...", "-count=1", "-failfast")
	cmd.Dir = r.gitRepoPath
	output, err := cmd.CombinedOutput()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 0 {
				return true, nil
			}
		}
		return false, fmt.Errorf("test command failed: %s", string(output))
	}

	return false, nil
}

func (r *ResumeLoader) GetResumeWithValidation(ctx context.Context, repoID string) (*ResumePacket, error) {
	packet, err := r.LoadResume(ctx, repoID)
	if err != nil {
		return nil, err
	}

	errors, err := r.ValidateResume(ctx, packet)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if len(errors) > 0 {
		return packet, fmt.Errorf("resume validation failed: %s", strings.Join(errors, "; "))
	}

	return packet, nil
}

func (r *ResumeLoader) GetOpenTasks(ctx context.Context, repoID string) ([]OpenTask, error) {
	return r.store.LoadOpenTasks(ctx, repoID)
}

func (r *ResumeLoader) GetRelevantFacts(ctx context.Context, repoID string, scope string) ([]MemoryFact, error) {
	if scope != "" {
		factScope := FactScope(scope)
		return r.store.LoadFacts(ctx, repoID, FactType(factScope))
	}
	return r.store.LoadAllFacts(ctx, repoID)
}

func (r *ResumeLoader) GetSessionHistory(ctx context.Context, repoID string, limit int) ([]SessionRecord, error) {
	sessions, err := r.store.LoadSessionsForRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(sessions) > limit {
		return sessions[:limit], nil
	}

	return sessions, nil
}

type ResumeConfig struct {
	BaseDir     string
	RepoID      string
	GitRepoPath string
}

func NewResumeLoaderFromConfig(cfg ResumeConfig) (*ResumeLoader, error) {
	store, err := NewFileMemoryStore(cfg.BaseDir, cfg.RepoID)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory store: %w", err)
	}

	return NewResumeLoader(store, cfg.GitRepoPath), nil
}

func GetRepoIDFromPath(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	absPath := strings.TrimSpace(string(output))
	dirName := filepath.Base(absPath)

	return dirName, nil
}
