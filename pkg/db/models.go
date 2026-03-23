// Package db provides the database layer for the RSS study engine,
// using pgx/v5 with native query patterns.
package db

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Enum types matching PostgreSQL enums.

type IngestStatus string

const (
	IngestStatusPending    IngestStatus = "pending"
	IngestStatusProcessing IngestStatus = "processing"
	IngestStatusDone       IngestStatus = "done"
	IngestStatusError      IngestStatus = "error"
)

type ArtifactType string

const (
	ArtifactTypeSummary    ArtifactType = "summary"
	ArtifactTypeFaq        ArtifactType = "faq"
	ArtifactTypeStudyGuide ArtifactType = "study_guide"
	ArtifactTypeTimeline   ArtifactType = "timeline"
	ArtifactTypeDigest     ArtifactType = "digest"
)

type TtsSourceType string

const (
	TtsSourceTypeArtifact TtsSourceType = "artifact"
	TtsSourceTypeDocRaw   TtsSourceType = "doc_raw"
)

type TtsStatus string

const (
	TtsStatusPending    TtsStatus = "pending"
	TtsStatusProcessing TtsStatus = "processing"
	TtsStatusDone       TtsStatus = "done"
	TtsStatusError      TtsStatus = "error"
)

type StudyJobType string

const (
	StudyJobTypeIngestText       StudyJobType = "ingest_text"
	StudyJobTypeIngestAudio      StudyJobType = "ingest_audio"
	StudyJobTypeGenerateArtifact StudyJobType = "generate_artifact"
	StudyJobTypeGenerateTts      StudyJobType = "generate_tts"
	StudyJobTypeGenerateDigest   StudyJobType = "generate_digest"
	StudyJobTypePollFeed         StudyJobType = "poll_feed"
)

type StudyJobStatus string

const (
	StudyJobStatusPending    StudyJobStatus = "pending"
	StudyJobStatusProcessing StudyJobStatus = "processing"
	StudyJobStatusDone       StudyJobStatus = "done"
	StudyJobStatusError      StudyJobStatus = "error"
	StudyJobStatusCancelled  StudyJobStatus = "cancelled"
)

type Feed struct {
	ID             uuid.UUID    `json:"id"`
	URL            string       `json:"url"`
	Title          string       `json:"title"`
	FeedType       string       `json:"feed_type"`
	ContentType    string       `json:"content_type"`
	PollInterval   string       `json:"poll_interval"`
	LastPolledAt   sql.NullTime `json:"last_polled_at"`
	LastError      *string      `json:"last_error"`
	PodcastEnabled bool         `json:"podcast_enabled"`
	PodcastSlug    *string      `json:"podcast_slug"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type Doc struct {
	ID           uuid.UUID       `json:"id"`
	FeedID       uuid.UUID       `json:"feed_id"`
	GUID         string          `json:"guid"`
	SourceURL    string          `json:"source_url"`
	ContentHash  string          `json:"content_hash"`
	Title        string          `json:"title"`
	Author       string          `json:"author"`
	PublishedAt  sql.NullTime    `json:"published_at"`
	RawContent   string          `json:"raw_content"`
	ContentType  string          `json:"content_type"`
	Meta         json.RawMessage `json:"meta"`
	Version      int32           `json:"version"`
	SupersededBy *uuid.UUID      `json:"superseded_by"`
	IsLatest     bool            `json:"is_latest"`
	IngestStatus IngestStatus    `json:"ingest_status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type Chunk struct {
	ID         uuid.UUID `json:"id"`
	DocID      uuid.UUID `json:"doc_id"`
	ChunkIndex int32     `json:"chunk_index"`
	ChunkHash  string    `json:"chunk_hash"`
	Text       string    `json:"text"`
	TokenCount int32     `json:"token_count"`
	StartTime  *float64  `json:"start_time"`
	EndTime    *float64  `json:"end_time"`
	CreatedAt  time.Time `json:"created_at"`
}

type Artifact struct {
	ID            uuid.UUID       `json:"id"`
	DocID         *uuid.UUID      `json:"doc_id"`
	FeedID        *uuid.UUID      `json:"feed_id"`
	ArtifactType  ArtifactType    `json:"artifact_type"`
	PromptVersion string          `json:"prompt_version"`
	Content       json.RawMessage `json:"content"`
	Model         string          `json:"model"`
	InputTokens   int32           `json:"input_tokens"`
	OutputTokens  int32           `json:"output_tokens"`
	GeneratedAt   time.Time       `json:"generated_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

type TtsJob struct {
	ID               uuid.UUID     `json:"id"`
	SourceType       TtsSourceType `json:"source_type"`
	ArtifactID       *uuid.UUID    `json:"artifact_id"`
	DocID            *uuid.UUID    `json:"doc_id"`
	ContentKey       string        `json:"content_key"`
	Model            string        `json:"model"`
	Voice            string        `json:"voice"`
	Speed            float64       `json:"speed"`
	Status           TtsStatus     `json:"status"`
	AudioPath        *string       `json:"audio_path"`
	AudioURL         *string       `json:"audio_url"`
	DurationSec      *float64      `json:"duration_sec"`
	FileSizeBytes    *int64        `json:"file_size_bytes"`
	IncludeInPodcast bool          `json:"include_in_podcast"`
	PodcastGUID      *string       `json:"podcast_guid"`
	QueuedAt         time.Time     `json:"queued_at"`
	StartedAt        sql.NullTime  `json:"started_at"`
	CompletedAt      sql.NullTime  `json:"completed_at"`
	CreatedAt        time.Time     `json:"created_at"`
}

type StudyJob struct {
	ID          uuid.UUID       `json:"id"`
	JobType     StudyJobType    `json:"job_type"`
	Status      StudyJobStatus  `json:"status"`
	Priority    int32           `json:"priority"`
	FeedID      *uuid.UUID      `json:"feed_id"`
	DocID       *uuid.UUID      `json:"doc_id"`
	ArtifactID  *uuid.UUID      `json:"artifact_id"`
	TtsJobID    *uuid.UUID      `json:"tts_job_id"`
	Payload     json.RawMessage `json:"payload"`
	Attempts    int32           `json:"attempts"`
	MaxAttempts int32           `json:"max_attempts"`
	LastError   *string         `json:"last_error"`
	RunAfter    time.Time       `json:"run_after"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type PodcastFeed struct {
	ID          uuid.UUID `json:"id"`
	FeedID      uuid.UUID `json:"feed_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	ImageURL    *string   `json:"image_url"`
	Language    string    `json:"language"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChunkResult struct {
	ID          uuid.UUID    `json:"id"`
	DocID       uuid.UUID    `json:"doc_id"`
	ChunkIndex  int32        `json:"chunk_index"`
	Text        string       `json:"text"`
	StartTime   *float64     `json:"start_time"`
	EndTime     *float64     `json:"end_time"`
	Rank        float64      `json:"rank"`
	DocTitle    string       `json:"doc_title"`
	SourceURL   string       `json:"source_url"`
	PublishedAt sql.NullTime `json:"published_at"`
}
