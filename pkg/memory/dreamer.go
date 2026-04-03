package memory

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/llm"
)

type DreamerWorker struct {
	store        MemoryStore
	consolidator *Consolidator
	gitRepoPath  string
}

func NewDreamerWorker(store MemoryStore, consolidator *Consolidator, gitRepoPath string) *DreamerWorker {
	return &DreamerWorker{
		store:        store,
		consolidator: consolidator,
		gitRepoPath:  gitRepoPath,
	}
}

func (d *DreamerWorker) Run(ctx context.Context, sessionID string) error {
	log.Printf("[DreamerWorker] Starting consolidation for session %s", sessionID)

	session, err := d.store.LoadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	artifacts, err := d.store.LoadArtifactsForSession(ctx, sessionID)
	if err != nil {
		log.Printf("[DreamerWorker] Warning: failed to load artifacts: %v", err)
		artifacts = []Artifact{}
	}

	gitState, err := d.store.LoadGitSnapshot(ctx, sessionID)
	if err != nil {
		log.Printf("[DreamerWorker] Warning: failed to load git snapshot: %v", err)
		gitState = &GitSnapshot{
			Branch: d.getCurrentBranch(),
		}
	}

	priorFacts, err := d.store.LoadAllFacts(ctx, session.RepoID)
	if err != nil {
		log.Printf("[DreamerWorker] Warning: failed to load prior facts: %v", err)
		priorFacts = []MemoryFact{}
	}

	input := ConsolidationInput{
		Session:    *session,
		Artifacts:  artifacts,
		GitState:   *gitState,
		PriorFacts: priorFacts,
	}

	result, err := d.consolidator.Consolidate(ctx, input)
	if err != nil {
		return fmt.Errorf("consolidation failed: %w", err)
	}

	if err := d.store.SaveSessionSummary(ctx, result.Summary); err != nil {
		log.Printf("[DreamerWorker] Warning: failed to save summary: %v", err)
	}

	if len(result.Facts) > 0 {
		if err := d.store.SaveFacts(ctx, result.Facts); err != nil {
			log.Printf("[DreamerWorker] Warning: failed to save facts: %v", err)
		}
		log.Printf("[DreamerWorker] Saved %d facts", len(result.Facts))
	}

	if len(result.OpenTasks) > 0 {
		if err := d.store.SaveOpenTasks(ctx, result.OpenTasks); err != nil {
			log.Printf("[DreamerWorker] Warning: failed to save open tasks: %v", err)
		}
		log.Printf("[DreamerWorker] Saved %d open tasks", len(result.OpenTasks))
	}

	if err := d.store.SaveResumePacket(ctx, result.ResumePacket); err != nil {
		log.Printf("[DreamerWorker] Warning: failed to save resume packet: %v", err)
	}
	log.Printf("[DreamerWorker] Saved resume packet")

	session.Status = SessionStatusCompleted
	session.EndedAt = time.Now()
	session.SummaryID = result.Summary.ID
	session.Artifacts = make([]string, len(result.PrunedArtifactIDs))
	copy(session.Artifacts, result.PrunedArtifactIDs)

	if err := d.store.SaveSession(ctx, *session); err != nil {
		log.Printf("[DreamerWorker] Warning: failed to update session: %v", err)
	}

	if err := d.pruneOldData(); err != nil {
		log.Printf("[DreamerWorker] Warning: failed to prune old data: %v", err)
	}

	log.Printf("[DreamerWorker] Consolidation complete for session %s", sessionID)
	return nil
}

func (d *DreamerWorker) getCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = d.gitRepoPath
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func (d *DreamerWorker) pruneOldData() error {
	sevenDays := 7 * 24 * time.Hour
	if err := d.store.DeleteOldArtifacts(context.Background(), sevenDays); err != nil {
		return err
	}

	return nil
}

func (d *DreamerWorker) RevalidateFacts(ctx context.Context, repoID string) ([]MemoryFact, error) {
	facts, err := d.store.LoadAllFacts(ctx, repoID)
	if err != nil {
		return nil, err
	}

	var stale, valid []MemoryFact
	weekAgo := time.Now().AddDate(0, 0, -7)

	for _, fact := range facts {
		if fact.LastValidatedAt.Before(weekAgo) {
			stale = append(stale, fact)
		} else {
			valid = append(valid, fact)
		}
	}

	if len(stale) > 0 {
		var staleIDs []string
		for _, f := range stale {
			staleIDs = append(staleIDs, f.ID)
		}
		if err := d.store.MarkStale(ctx, staleIDs); err != nil {
			log.Printf("[DreamerWorker] Warning: failed to mark stale facts: %v", err)
		}
	}

	return valid, nil
}

type DreamerConfig struct {
	BaseDir     string
	RepoID      string
	GitRepoPath string
	LLMBackend  llm.Backend
}

func NewDreamerFromConfig(cfg DreamerConfig) (*DreamerWorker, error) {
	store, err := NewFileMemoryStore(cfg.BaseDir, cfg.RepoID)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory store: %w", err)
	}

	consolidator := NewConsolidator(cfg.LLMBackend)

	return NewDreamerWorker(store, consolidator, cfg.GitRepoPath), nil
}

func EnsureMemoryDirectory(baseDir string) error {
	dirs := []string{
		baseDir,
		baseDir + "/sessions",
		baseDir + "/memory",
		baseDir + "/memory/summaries",
		baseDir + "/memory/git_snapshots",
		baseDir + "/resume",
		baseDir + "/artifacts",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
