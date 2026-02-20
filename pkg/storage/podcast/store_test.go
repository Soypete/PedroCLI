package podcast

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_CreateAndGet(t *testing.T) {
	store := NewMemoryStore(nil)
	ctx := context.Background()

	ep := &Episode{
		EpisodeNumber: "S01E01",
		Title:         "Pilot Episode",
		RecordDate:    time.Now(),
		Status:        StatusUploaded,
	}

	if err := store.CreateEpisode(ctx, ep); err != nil {
		t.Fatalf("CreateEpisode failed: %v", err)
	}

	if ep.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := store.GetEpisode(ctx, ep.ID)
	if err != nil {
		t.Fatalf("GetEpisode failed: %v", err)
	}

	if got.Title != "Pilot Episode" {
		t.Errorf("expected title 'Pilot Episode', got '%s'", got.Title)
	}
	if got.EpisodeNumber != "S01E01" {
		t.Errorf("expected episode number 'S01E01', got '%s'", got.EpisodeNumber)
	}
	if got.Status != StatusUploaded {
		t.Errorf("expected status 'uploaded', got '%s'", got.Status)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore(nil)
	ctx := context.Background()

	ep := &Episode{
		Title:  "Original",
		Status: StatusUploaded,
	}
	_ = store.CreateEpisode(ctx, ep)

	ep.Title = "Updated"
	ep.Status = StatusTranscribed
	ep.Transcript = "Hello world transcript"

	if err := store.UpdateEpisode(ctx, ep); err != nil {
		t.Fatalf("UpdateEpisode failed: %v", err)
	}

	got, _ := store.GetEpisode(ctx, ep.ID)
	if got.Title != "Updated" {
		t.Errorf("expected title 'Updated', got '%s'", got.Title)
	}
	if got.Status != StatusTranscribed {
		t.Errorf("expected status 'transcribed', got '%s'", got.Status)
	}
	if got.Transcript != "Hello world transcript" {
		t.Errorf("expected transcript content, got '%s'", got.Transcript)
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore(nil)
	ctx := context.Background()

	// Empty list
	episodes, _ := store.ListEpisodes(ctx)
	if len(episodes) != 0 {
		t.Errorf("expected 0 episodes, got %d", len(episodes))
	}

	// Add two episodes
	_ = store.CreateEpisode(ctx, &Episode{Title: "Episode 1", Status: StatusUploaded})
	_ = store.CreateEpisode(ctx, &Episode{Title: "Episode 2", Status: StatusTranscribed})

	episodes, _ = store.ListEpisodes(ctx)
	if len(episodes) != 2 {
		t.Errorf("expected 2 episodes, got %d", len(episodes))
	}

	// List should not include transcript content (large field)
	for _, ep := range episodes {
		if ep.Transcript != "" {
			t.Error("expected empty transcript in list view")
		}
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore(nil)
	ctx := context.Background()

	ep := &Episode{Title: "To Delete", Status: StatusUploaded}
	_ = store.CreateEpisode(ctx, ep)

	if err := store.DeleteEpisode(ctx, ep.ID); err != nil {
		t.Fatalf("DeleteEpisode failed: %v", err)
	}

	_, err := store.GetEpisode(ctx, ep.ID)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestMemoryStore_UpdateNonExistent(t *testing.T) {
	store := NewMemoryStore(nil)
	ctx := context.Background()

	err := store.UpdateEpisode(ctx, &Episode{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent episode")
	}
}

func TestMemoryStore_FactChecksAndShowNotes(t *testing.T) {
	store := NewMemoryStore(nil)
	ctx := context.Background()

	ep := &Episode{
		Title:  "With Facts",
		Status: StatusTranscribed,
	}
	_ = store.CreateEpisode(ctx, ep)

	// Add fact checks
	ep.FactChecks = []FactCheck{
		{ID: "fc-1", Timestamp: "05:30", Claim: "Go is the fastest language", Status: "unchecked"},
		{ID: "fc-2", Timestamp: "12:00", Claim: "Kubernetes was made by Google", Status: "verified"},
	}
	ep.Status = StatusFactChecked
	_ = store.UpdateEpisode(ctx, ep)

	got, _ := store.GetEpisode(ctx, ep.ID)
	if len(got.FactChecks) != 2 {
		t.Errorf("expected 2 fact checks, got %d", len(got.FactChecks))
	}

	// Add show notes
	ep.ShowNotes = &ShowNotes{
		Summary: "Episode about Go and Kubernetes",
		Links: []Link{
			{URL: "https://golang.org", Title: "Go Website", Timestamp: "03:00"},
		},
		Chapters: []Chapter{
			{Timestamp: "00:00", Title: "Intro", Summary: "Welcome"},
			{Timestamp: "05:00", Title: "Go Performance", Summary: "Discussing speed"},
		},
	}
	ep.Status = StatusShowNotesDone
	_ = store.UpdateEpisode(ctx, ep)

	got, _ = store.GetEpisode(ctx, ep.ID)
	if got.ShowNotes == nil {
		t.Fatal("expected show notes, got nil")
	}
	if got.ShowNotes.Summary != "Episode about Go and Kubernetes" {
		t.Errorf("unexpected summary: %s", got.ShowNotes.Summary)
	}
	if len(got.ShowNotes.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(got.ShowNotes.Links))
	}
	if len(got.ShowNotes.Chapters) != 2 {
		t.Errorf("expected 2 chapters, got %d", len(got.ShowNotes.Chapters))
	}
}
