package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	SessionsDir     = "sessions"
	MemoryDir       = "memory"
	ResumeDir       = "resume"
	ArtifactsDir    = "artifacts"
	FactsFile       = "facts.jsonl"
	OpenTasksFile   = "open_tasks.jsonl"
	SummariesDir    = "summaries"
	GitSnapshotsDir = "git_snapshots"
)

type FileMemoryStore struct {
	baseDir  string
	repoID   string
	mu       sync.RWMutex
	facts    []MemoryFact
	tasks    []OpenTask
	sessions []SessionRecord
	resumes  []ResumePacket
}

func NewFileMemoryStore(baseDir, repoID string) (*FileMemoryStore, error) {
	store := &FileMemoryStore{
		baseDir:  baseDir,
		repoID:   repoID,
		facts:    []MemoryFact{},
		tasks:    []OpenTask{},
		sessions: []SessionRecord{},
		resumes:  []ResumePacket{},
	}

	if err := store.ensureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to ensure directories: %w", err)
	}

	if err := store.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load from disk: %w", err)
	}

	return store, nil
}

func (s *FileMemoryStore) ensureDirectories() error {
	dirs := []string{
		filepath.Join(s.baseDir, SessionsDir),
		filepath.Join(s.baseDir, MemoryDir),
		filepath.Join(s.baseDir, ResumeDir),
		filepath.Join(s.baseDir, MemoryDir, SummariesDir),
		filepath.Join(s.baseDir, MemoryDir, GitSnapshotsDir),
		filepath.Join(s.baseDir, ArtifactsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (s *FileMemoryStore) loadFromDisk() error {
	factsFile := filepath.Join(s.baseDir, MemoryDir, FactsFile)
	if data, err := os.ReadFile(factsFile); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var fact MemoryFact
			if err := json.Unmarshal([]byte(line), &fact); err == nil {
				s.facts = append(s.facts, fact)
			}
		}
	}

	tasksFile := filepath.Join(s.baseDir, MemoryDir, OpenTasksFile)
	if data, err := os.ReadFile(tasksFile); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var task OpenTask
			if err := json.Unmarshal([]byte(line), &task); err == nil {
				s.tasks = append(s.tasks, task)
			}
		}
	}

	sessionsDir := filepath.Join(s.baseDir, SessionsDir)
	if entries, err := os.ReadDir(sessionsDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
				if err != nil {
					continue
				}
				var session SessionRecord
				if err := json.Unmarshal(data, &session); err == nil {
					s.sessions = append(s.sessions, session)
				}
			}
		}
	}

	resumeDir := filepath.Join(s.baseDir, ResumeDir)
	if entries, err := os.ReadDir(resumeDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(resumeDir, entry.Name()))
				if err != nil {
					continue
				}
				var resume ResumePacket
				if err := json.Unmarshal(data, &resume); err == nil {
					s.resumes = append(s.resumes, resume)
				}
			}
		}
	}

	return nil
}

func (s *FileMemoryStore) SaveSession(ctx context.Context, session SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionFile := filepath.Join(s.baseDir, SessionsDir, fmt.Sprintf("%s.json", session.SessionID))
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	updated := false
	for i, sess := range s.sessions {
		if sess.SessionID == session.SessionID {
			s.sessions[i] = session
			updated = true
			break
		}
	}
	if !updated {
		s.sessions = append(s.sessions, session)
	}

	return nil
}

func (s *FileMemoryStore) LoadSession(ctx context.Context, sessionID string) (*SessionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if session.SessionID == sessionID {
			return &session, nil
		}
	}

	sessionFile := filepath.Join(s.baseDir, SessionsDir, fmt.Sprintf("%s.json", sessionID))
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var session SessionRecord
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

func (s *FileMemoryStore) LoadSessionsForRepo(ctx context.Context, repoID string) ([]SessionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []SessionRecord
	for _, session := range s.sessions {
		if session.RepoID == repoID {
			result = append(result, session)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartedAt.After(result[j].StartedAt)
	})

	return result, nil
}

func (s *FileMemoryStore) SaveSessionSummary(ctx context.Context, summary SessionSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	summaryFile := filepath.Join(s.baseDir, MemoryDir, SummariesDir, fmt.Sprintf("%s.json", summary.ID))
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	if err := os.WriteFile(summaryFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write summary file: %w", err)
	}

	return nil
}

func (s *FileMemoryStore) LoadSessionSummary(ctx context.Context, summaryID string) (*SessionSummary, error) {
	summaryFile := filepath.Join(s.baseDir, MemoryDir, SummariesDir, fmt.Sprintf("%s.json", summaryID))
	data, err := os.ReadFile(summaryFile)
	if err != nil {
		return nil, fmt.Errorf("summary not found: %s", summaryID)
	}

	var summary SessionSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal summary: %w", err)
	}

	return &summary, nil
}

func (s *FileMemoryStore) SaveFacts(ctx context.Context, facts []MemoryFact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, fact := range facts {
		fact.CreatedAt = time.Now()
		updated := false
		for i, f := range s.facts {
			if f.ID == fact.ID {
				s.facts[i] = fact
				updated = true
				break
			}
		}
		if !updated {
			s.facts = append(s.facts, fact)
		}
	}

	return s.persistFacts()
}

func (s *FileMemoryStore) persistFacts() error {
	factsFile := filepath.Join(s.baseDir, MemoryDir, FactsFile)
	var lines []string
	for _, fact := range s.facts {
		data, err := json.Marshal(fact)
		if err != nil {
			return fmt.Errorf("failed to marshal fact: %w", err)
		}
		lines = append(lines, string(data))
	}

	if err := os.WriteFile(factsFile, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write facts file: %w", err)
	}

	return nil
}

func (s *FileMemoryStore) LoadFacts(ctx context.Context, repoID string, factType FactType) ([]MemoryFact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MemoryFact
	for _, fact := range s.facts {
		if fact.Scope == FactScopeRepo || strings.HasPrefix(fact.Subject, repoID) {
			if factType == "" || fact.Type == factType {
				result = append(result, fact)
			}
		}
	}

	return result, nil
}

func (s *FileMemoryStore) LoadAllFacts(ctx context.Context, repoID string) ([]MemoryFact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []MemoryFact
	for _, fact := range s.facts {
		if fact.Scope == FactScopeRepo || strings.Contains(fact.Subject, repoID) {
			result = append(result, fact)
		}
	}

	return result, nil
}

func (s *FileMemoryStore) MarkStale(ctx context.Context, factIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]bool)
	for _, id := range factIDs {
		idSet[id] = true
	}

	for i := range s.facts {
		if idSet[s.facts[i].ID] {
			s.facts[i].Confidence = FactConfidenceLow
		}
	}

	return s.persistFacts()
}

func (s *FileMemoryStore) DeleteFact(ctx context.Context, factID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newFacts []MemoryFact
	for _, fact := range s.facts {
		if fact.ID != factID {
			newFacts = append(newFacts, fact)
		}
	}
	s.facts = newFacts

	return s.persistFacts()
}

func (s *FileMemoryStore) SaveOpenTasks(ctx context.Context, tasks []OpenTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range tasks {
		task.LastUpdatedAt = time.Now()
		updated := false
		for i, t := range s.tasks {
			if t.ID == task.ID {
				s.tasks[i] = task
				updated = true
				break
			}
		}
		if !updated {
			task.CreatedAt = time.Now()
			s.tasks = append(s.tasks, task)
		}
	}

	return s.persistTasks()
}

func (s *FileMemoryStore) persistTasks() error {
	tasksFile := filepath.Join(s.baseDir, MemoryDir, OpenTasksFile)
	var lines []string
	for _, task := range s.tasks {
		data, err := json.Marshal(task)
		if err != nil {
			return fmt.Errorf("failed to marshal task: %w", err)
		}
		lines = append(lines, string(data))
	}

	if err := os.WriteFile(tasksFile, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

func (s *FileMemoryStore) LoadOpenTasks(ctx context.Context, repoID string) ([]OpenTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []OpenTask
	for _, task := range s.tasks {
		if task.Status == TaskStatusOpen || task.Status == TaskStatusInProgress || task.Status == TaskStatusBlocked {
			if task.Scope == "" || strings.Contains(task.Scope, repoID) {
				result = append(result, task)
			}
		}
	}

	return result, nil
}

func (s *FileMemoryStore) UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.tasks {
		if s.tasks[i].ID == taskID {
			s.tasks[i].Status = status
			s.tasks[i].LastUpdatedAt = time.Now()
			break
		}
	}

	return s.persistTasks()
}

func (s *FileMemoryStore) DeleteTask(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newTasks []OpenTask
	for _, task := range s.tasks {
		if task.ID != taskID {
			newTasks = append(newTasks, task)
		}
	}
	s.tasks = newTasks

	return s.persistTasks()
}

func (s *FileMemoryStore) SaveResumePacket(ctx context.Context, packet ResumePacket) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	packet.CreatedAt = time.Now()
	resumeFile := filepath.Join(s.baseDir, ResumeDir, fmt.Sprintf("resume_%s.json", packet.SessionID))
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resume packet: %w", err)
	}

	if err := os.WriteFile(resumeFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write resume file: %w", err)
	}

	latestFile := filepath.Join(s.baseDir, ResumeDir, fmt.Sprintf("%s.latest.json", s.repoID))
	if err := os.WriteFile(latestFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write latest resume: %w", err)
	}

	updated := false
	for i, r := range s.resumes {
		if r.SessionID == packet.SessionID {
			s.resumes[i] = packet
			updated = true
			break
		}
	}
	if !updated {
		s.resumes = append(s.resumes, packet)
	}

	return nil
}

func (s *FileMemoryStore) LoadLatestResumePacket(ctx context.Context, repoID string) (*ResumePacket, error) {
	latestFile := filepath.Join(s.baseDir, ResumeDir, fmt.Sprintf("%s.latest.json", repoID))
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("no resume packet found for repo: %s", repoID)
	}

	var resume ResumePacket
	if err := json.Unmarshal(data, &resume); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resume: %w", err)
	}

	return &resume, nil
}

func (s *FileMemoryStore) LoadResumePackets(ctx context.Context, repoID string, limit int) ([]ResumePacket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sort.Slice(s.resumes, func(i, j int) bool {
		return s.resumes[i].CreatedAt.After(s.resumes[j].CreatedAt)
	})

	if limit > 0 && len(s.resumes) > limit {
		return s.resumes[:limit], nil
	}

	return s.resumes, nil
}

func (s *FileMemoryStore) SaveGitSnapshot(ctx context.Context, sessionID string, snapshot GitSnapshot) error {
	snapshotFile := filepath.Join(s.baseDir, MemoryDir, GitSnapshotsDir, fmt.Sprintf("%s.json", sessionID))
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal git snapshot: %w", err)
	}

	if err := os.WriteFile(snapshotFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write git snapshot: %w", err)
	}

	return nil
}

func (s *FileMemoryStore) LoadGitSnapshot(ctx context.Context, sessionID string) (*GitSnapshot, error) {
	snapshotFile := filepath.Join(s.baseDir, MemoryDir, GitSnapshotsDir, fmt.Sprintf("%s.json", sessionID))
	data, err := os.ReadFile(snapshotFile)
	if err != nil {
		return nil, fmt.Errorf("git snapshot not found for session: %s", sessionID)
	}

	var snapshot GitSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal git snapshot: %w", err)
	}

	return &snapshot, nil
}

func (s *FileMemoryStore) SaveArtifact(ctx context.Context, artifact Artifact) error {
	artifactFile := filepath.Join(s.baseDir, ArtifactsDir, fmt.Sprintf("%s.json", artifact.ID))
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal artifact: %w", err)
	}

	if err := os.WriteFile(artifactFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write artifact: %w", err)
	}

	return nil
}

func (s *FileMemoryStore) LoadArtifact(ctx context.Context, artifactID string) (*Artifact, error) {
	artifactFile := filepath.Join(s.baseDir, ArtifactsDir, fmt.Sprintf("%s.json", artifactID))
	data, err := os.ReadFile(artifactFile)
	if err != nil {
		return nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("failed to unmarshal artifact: %w", err)
	}

	return &artifact, nil
}

func (s *FileMemoryStore) LoadArtifactsForSession(ctx context.Context, sessionID string) ([]Artifact, error) {
	artifactsDir := filepath.Join(s.baseDir, ArtifactsDir)
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifacts directory: %w", err)
	}

	var result []Artifact
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(artifactsDir, entry.Name()))
		if err != nil {
			continue
		}

		var artifact Artifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			continue
		}

		if artifact.SessionID == sessionID {
			result = append(result, artifact)
		}
	}

	return result, nil
}

func (s *FileMemoryStore) DeleteOldArtifacts(ctx context.Context, olderThan time.Duration) error {
	artifactsDir := filepath.Join(s.baseDir, ArtifactsDir)
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		return fmt.Errorf("failed to read artifacts directory: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	var deleted int
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(artifactsDir, entry.Name())); err == nil {
				deleted++
			}
		}
	}

	return nil
}

func (s *FileMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.persistFacts(); err != nil {
		return err
	}

	if err := s.persistTasks(); err != nil {
		return err
	}

	return nil
}

func GetDefaultMemoryDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".pedro"
	}
	return filepath.Join(homeDir, ".pedro")
}
