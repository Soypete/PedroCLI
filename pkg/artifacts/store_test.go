package artifacts

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryStore_Put(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	artifact := NewArtifact("art-1", ArtifactRepoMap, "repo_map.json", `{"files": []}`, "builder")
	err := store.Put(ctx, artifact)
	if err != nil {
		t.Errorf("Put() error = %v", err)
	}

	got, err := store.Get(ctx, "art-1")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if got.ID != "art-1" {
		t.Errorf("expected ID art-1, got %s", got.ID)
	}
}

func TestInMemoryStore_Put_Invalid(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	tests := []struct {
		name     string
		artifact *Artifact
		wantErr  bool
	}{
		{
			name:     "nil artifact",
			artifact: nil,
			wantErr:  true,
		},
		{
			name: "empty id",
			artifact: &Artifact{
				ID:   "",
				Type: ArtifactRepoMap,
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "empty type",
			artifact: &Artifact{
				ID:   "art-1",
				Type: "",
				Name: "test",
			},
			wantErr: true,
		},
		{
			name: "empty name",
			artifact: &Artifact{
				ID:   "art-1",
				Type: ArtifactRepoMap,
				Name: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Put(ctx, tt.artifact)
			if (err != nil) != tt.wantErr {
				t.Errorf("Put() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInMemoryStore_GetByType(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	store.Put(ctx, NewArtifact("art-1", ArtifactRepoMap, "repo_map.json", "{}", "builder"))
	store.Put(ctx, NewArtifact("art-2", ArtifactPlan, "plan.md", "# Plan", "builder"))
	store.Put(ctx, NewArtifact("art-3", ArtifactPlan, "plan2.md", "# Plan 2", "builder"))

	got, err := store.GetByType(ctx, ArtifactPlan)
	if err != nil {
		t.Errorf("GetByType() error = %v", err)
	}
	if got.ID != "art-3" {
		t.Errorf("expected latest artifact art-3, got %s", got.ID)
	}

	_, err = store.GetByType(ctx, ArtifactDiff)
	if err == nil {
		t.Error("expected error for non-existent type")
	}
}

func TestInMemoryStore_GetByName(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	store.Put(ctx, NewArtifact("art-1", ArtifactRepoMap, "repo_map.json", "{}", "builder"))
	store.Put(ctx, NewArtifact("art-2", ArtifactPlan, "plan.md", "# Plan", "builder"))

	got, err := store.GetByName(ctx, "plan.md")
	if err != nil {
		t.Errorf("GetByName() error = %v", err)
	}
	if got.ID != "art-2" {
		t.Errorf("expected artifact art-2, got %s", got.ID)
	}

	_, err = store.GetByName(ctx, "nonexistent.md")
	if err == nil {
		t.Error("expected error for non-existent name")
	}
}

func TestInMemoryStore_List(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	store.Put(ctx, NewArtifact("art-1", ArtifactRepoMap, "repo1.json", "{}", "builder"))
	store.Put(ctx, NewArtifact("art-2", ArtifactPlan, "plan1.md", "# Plan", "builder"))
	store.Put(ctx, NewArtifact("art-3", ArtifactPlan, "plan2.md", "# Plan 2", "debugger"))

	list, _ := store.List(ctx, nil)
	if len(list) != 3 {
		t.Errorf("expected 3 artifacts, got %d", len(list))
	}

	list, _ = store.List(ctx, &ArtifactFilter{Type: ArtifactPlan})
	if len(list) != 2 {
		t.Errorf("expected 2 plan artifacts, got %d", len(list))
	}

	list, _ = store.List(ctx, &ArtifactFilter{CreatedBy: "builder"})
	if len(list) != 2 {
		t.Errorf("expected 2 builder artifacts, got %d", len(list))
	}

	list, _ = store.List(ctx, &ArtifactFilter{Type: ArtifactPlan, CreatedBy: "builder"})
	if len(list) != 1 {
		t.Errorf("expected 1 artifact, got %d", len(list))
	}
}

func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	store.Put(ctx, NewArtifact("art-1", ArtifactRepoMap, "repo.json", "{}", "builder"))

	err := store.Delete(ctx, "art-1")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	_, err = store.Get(ctx, "art-1")
	if err == nil {
		t.Error("expected error after delete")
	}

	err = store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent artifact")
	}
}

func TestInMemoryStore_Clear(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	store.Put(ctx, NewArtifact("art-1", ArtifactRepoMap, "repo.json", "{}", "builder"))
	store.Put(ctx, NewArtifact("art-2", ArtifactPlan, "plan.md", "# Plan", "builder"))

	err := store.Clear(ctx)
	if err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	list, _ := store.List(ctx, nil)
	if len(list) != 0 {
		t.Errorf("expected 0 artifacts after clear, got %d", len(list))
	}
}

func TestArtifact_NewArtifact(t *testing.T) {
	now := time.Now()
	artifact := NewArtifact("art-1", ArtifactRepoMap, "repo.json", `{"key": "value"}`, "builder")

	if artifact.ID != "art-1" {
		t.Errorf("expected ID art-1, got %s", artifact.ID)
	}
	if artifact.Type != ArtifactRepoMap {
		t.Errorf("expected type ArtifactRepoMap, got %s", artifact.Type)
	}
	if artifact.Name != "repo.json" {
		t.Errorf("expected name repo.json, got %s", artifact.Name)
	}
	if artifact.Content != `{"key": "value"}` {
		t.Errorf("expected content, got %s", artifact.Content)
	}
	if artifact.CreatedBy != "builder" {
		t.Errorf("expected creator builder, got %s", artifact.CreatedBy)
	}
	if artifact.Size != 16 {
		t.Errorf("expected size 16, got %d", artifact.Size)
	}
	if artifact.CreatedAt.Before(now) {
		t.Error("expected CreatedAt to be after test start time")
	}
}

func TestArtifact_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		want     bool
	}{
		{
			name: "valid artifact",
			artifact: &Artifact{
				ID:   "art-1",
				Type: ArtifactRepoMap,
				Name: "repo.json",
			},
			want: true,
		},
		{
			name: "empty id",
			artifact: &Artifact{
				ID:   "",
				Type: ArtifactRepoMap,
				Name: "repo.json",
			},
			want: false,
		},
		{
			name: "empty type",
			artifact: &Artifact{
				ID:   "art-1",
				Type: "",
				Name: "repo.json",
			},
			want: false,
		},
		{
			name: "empty name",
			artifact: &Artifact{
				ID:   "art-1",
				Type: ArtifactRepoMap,
				Name: "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.IsValid()
			if got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArtifactTypes(t *testing.T) {
	expectedTypes := map[ArtifactType]string{
		ArtifactRepoMap:     "repo_map",
		ArtifactTask:        "task",
		ArtifactPlan:        "plan",
		ArtifactDiff:        "diff",
		ArtifactTestResults: "test_results",
		ArtifactReview:      "review",
		ArtifactOutput:      "output",
		ArtifactContext:     "context",
		ArtifactError:       "error",
	}

	for at, expected := range expectedTypes {
		if string(at) != expected {
			t.Errorf("ArtifactType %v = %s, want %s", at, string(at), expected)
		}
	}
}
