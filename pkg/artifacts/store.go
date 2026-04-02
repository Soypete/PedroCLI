package artifacts

import (
	"context"
	"fmt"
	"sync"
)

type ArtifactStore interface {
	Put(ctx context.Context, artifact *Artifact) error
	Get(ctx context.Context, id string) (*Artifact, error)
	GetByType(ctx context.Context, artifactType ArtifactType) (*Artifact, error)
	GetByName(ctx context.Context, name string) (*Artifact, error)
	List(ctx context.Context, filter *ArtifactFilter) ([]*Artifact, error)
	Delete(ctx context.Context, id string) error
	Clear(ctx context.Context) error
}

type InMemoryArtifactStore struct {
	mu        sync.RWMutex
	artifacts map[string]*Artifact
	byType    map[ArtifactType][]string
	byName    map[string]string
}

func NewInMemoryStore() *InMemoryArtifactStore {
	return &InMemoryArtifactStore{
		artifacts: make(map[string]*Artifact),
		byType:    make(map[ArtifactType][]string),
		byName:    make(map[string]string),
	}
}

func (s *InMemoryArtifactStore) Put(ctx context.Context, artifact *Artifact) error {
	if artifact == nil {
		return fmt.Errorf("artifact is nil")
	}
	if !artifact.IsValid() {
		return fmt.Errorf("invalid artifact: missing id, type, or name")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.artifacts[artifact.ID] = artifact

	s.byType[artifact.Type] = append(s.byType[artifact.Type], artifact.ID)

	s.byName[artifact.Name] = artifact.ID

	return nil
}

func (s *InMemoryArtifactStore) Get(ctx context.Context, id string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	artifact, ok := s.artifacts[id]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %s", id)
	}
	return artifact, nil
}

func (s *InMemoryArtifactStore) GetByType(ctx context.Context, artifactType ArtifactType) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.byType[artifactType]
	if !ok || len(ids) == 0 {
		return nil, fmt.Errorf("no artifacts of type %s", artifactType)
	}

	return s.artifacts[ids[len(ids)-1]], nil
}

func (s *InMemoryArtifactStore) GetByName(ctx context.Context, name string) (*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byName[name]
	if !ok {
		return nil, fmt.Errorf("artifact not found with name: %s", name)
	}

	return s.artifacts[id], nil
}

func (s *InMemoryArtifactStore) List(ctx context.Context, filter *ArtifactFilter) ([]*Artifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Artifact

	for _, artifact := range s.artifacts {
		if filter != nil {
			if filter.Type != "" && artifact.Type != filter.Type {
				continue
			}
			if filter.Name != "" && artifact.Name != filter.Name {
				continue
			}
			if filter.CreatedBy != "" && artifact.CreatedBy != filter.CreatedBy {
				continue
			}
			if !filter.Since.IsZero() && artifact.CreatedAt.Before(filter.Since) {
				continue
			}
		}
		result = append(result, artifact)
	}

	return result, nil
}

func (s *InMemoryArtifactStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	artifact, ok := s.artifacts[id]
	if !ok {
		return fmt.Errorf("artifact not found: %s", id)
	}

	delete(s.artifacts, id)

	ids := s.byType[artifact.Type]
	for i, aid := range ids {
		if aid == id {
			s.byType[artifact.Type] = append(ids[:i], ids[i+1:]...)
			break
		}
	}

	delete(s.byName, artifact.Name)

	return nil
}

func (s *InMemoryArtifactStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.artifacts = make(map[string]*Artifact)
	s.byType = make(map[ArtifactType][]string)
	s.byName = make(map[string]string)

	return nil
}
