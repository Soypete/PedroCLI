package memory

import (
	"context"
	"time"
)

type MemoryStore interface {
	SaveSession(ctx context.Context, session SessionRecord) error
	LoadSession(ctx context.Context, sessionID string) (*SessionRecord, error)
	LoadSessionsForRepo(ctx context.Context, repoID string) ([]SessionRecord, error)

	SaveSessionSummary(ctx context.Context, summary SessionSummary) error
	LoadSessionSummary(ctx context.Context, summaryID string) (*SessionSummary, error)

	SaveFacts(ctx context.Context, facts []MemoryFact) error
	LoadFacts(ctx context.Context, repoID string, factType FactType) ([]MemoryFact, error)
	LoadAllFacts(ctx context.Context, repoID string) ([]MemoryFact, error)
	MarkStale(ctx context.Context, factIDs []string) error
	DeleteFact(ctx context.Context, factID string) error

	SaveOpenTasks(ctx context.Context, tasks []OpenTask) error
	LoadOpenTasks(ctx context.Context, repoID string) ([]OpenTask, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error
	DeleteTask(ctx context.Context, taskID string) error

	SaveResumePacket(ctx context.Context, packet ResumePacket) error
	LoadLatestResumePacket(ctx context.Context, repoID string) (*ResumePacket, error)
	LoadResumePackets(ctx context.Context, repoID string, limit int) ([]ResumePacket, error)

	SaveGitSnapshot(ctx context.Context, sessionID string, snapshot GitSnapshot) error
	LoadGitSnapshot(ctx context.Context, sessionID string) (*GitSnapshot, error)

	SaveArtifact(ctx context.Context, artifact Artifact) error
	LoadArtifact(ctx context.Context, artifactID string) (*Artifact, error)
	LoadArtifactsForSession(ctx context.Context, sessionID string) ([]Artifact, error)
	DeleteOldArtifacts(ctx context.Context, olderThan time.Duration) error

	Close() error
}

type ArtifactStore interface {
	SaveArtifact(ctx context.Context, artifact Artifact) error
	LoadArtifact(ctx context.Context, artifactID string) (*Artifact, error)
	LoadArtifactsForSession(ctx context.Context, sessionID string) ([]Artifact, error)
	DeleteOldArtifacts(ctx context.Context, olderThan time.Duration) error
}
