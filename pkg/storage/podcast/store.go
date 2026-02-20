package podcast

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	s3storage "github.com/soypete/pedrocli/pkg/storage/s3"
)

// Store defines the interface for podcast episode storage.
type Store interface {
	CreateEpisode(ctx context.Context, ep *Episode) error
	GetEpisode(ctx context.Context, id string) (*Episode, error)
	UpdateEpisode(ctx context.Context, ep *Episode) error
	ListEpisodes(ctx context.Context) ([]*Episode, error)
	DeleteEpisode(ctx context.Context, id string) error
}

// MemoryStore is an in-memory implementation of Store, backed by S3 for large files.
type MemoryStore struct {
	mu       sync.RWMutex
	episodes map[string]*Episode
	s3       *s3storage.Client // Optional: for persisting transcripts/recordings
}

// NewMemoryStore creates a new in-memory podcast store.
// If s3Client is nil, large file storage is disabled.
func NewMemoryStore(s3Client *s3storage.Client) *MemoryStore {
	return &MemoryStore{
		episodes: make(map[string]*Episode),
		s3:       s3Client,
	}
}

func (s *MemoryStore) CreateEpisode(ctx context.Context, ep *Episode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ep.ID == "" {
		ep.ID = uuid.New().String()
	}
	now := time.Now()
	ep.CreatedAt = now
	ep.UpdatedAt = now

	s.episodes[ep.ID] = ep
	return nil
}

func (s *MemoryStore) GetEpisode(ctx context.Context, id string) (*Episode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ep, ok := s.episodes[id]
	if !ok {
		return nil, fmt.Errorf("episode not found: %s", id)
	}

	// Return a copy to avoid concurrent modification
	copied := *ep
	return &copied, nil
}

func (s *MemoryStore) UpdateEpisode(ctx context.Context, ep *Episode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.episodes[ep.ID]; !ok {
		return fmt.Errorf("episode not found: %s", ep.ID)
	}

	ep.UpdatedAt = time.Now()
	s.episodes[ep.ID] = ep
	return nil
}

func (s *MemoryStore) ListEpisodes(ctx context.Context) ([]*Episode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Episode, 0, len(s.episodes))
	for _, ep := range s.episodes {
		copied := *ep
		// Don't include large fields in list view
		copied.Transcript = ""
		copied.FactChecks = nil
		copied.ShowNotes = nil
		copied.Template = nil
		result = append(result, &copied)
	}
	return result, nil
}

func (s *MemoryStore) DeleteEpisode(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.episodes[id]; !ok {
		return fmt.Errorf("episode not found: %s", id)
	}

	delete(s.episodes, id)
	return nil
}

// SaveTranscriptToS3 persists a transcript to S3 and updates the episode's transcript key.
func (s *MemoryStore) SaveTranscriptToS3(ctx context.Context, episodeID string, transcript string) error {
	if s.s3 == nil {
		return nil // S3 not configured, transcript stays in memory only
	}

	s.mu.Lock()
	ep, ok := s.episodes[episodeID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("episode not found: %s", episodeID)
	}

	key := fmt.Sprintf("episodes/%s/transcript.txt", episodeID)
	_, err := s.s3.Upload(ctx, key, strings.NewReader(transcript), "text/plain")
	if err != nil {
		return fmt.Errorf("failed to upload transcript to S3: %w", err)
	}

	s.mu.Lock()
	ep.TranscriptKey = key
	ep.UpdatedAt = time.Now()
	s.mu.Unlock()

	return nil
}

// SaveShowNotesToS3 persists show notes to S3.
func (s *MemoryStore) SaveShowNotesToS3(ctx context.Context, episodeID string, notes *ShowNotes) error {
	if s.s3 == nil {
		return nil
	}

	data, err := json.Marshal(notes)
	if err != nil {
		return fmt.Errorf("failed to marshal show notes: %w", err)
	}

	key := fmt.Sprintf("episodes/%s/shownotes.json", episodeID)
	_, err = s.s3.Upload(ctx, key, strings.NewReader(string(data)), "application/json")
	if err != nil {
		return fmt.Errorf("failed to upload show notes to S3: %w", err)
	}

	return nil
}

