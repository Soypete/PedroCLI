package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPRDValidation(t *testing.T) {
	tests := []struct {
		name    string
		prd     PRD
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid code PRD",
			prd: PRD{
				ProjectName: "test-project",
				Mode:        PRDModeCode,
				UserStories: []UserStory{
					{ID: "US-001", Title: "Add feature", Priority: 1},
				},
			},
			wantErr: false,
		},
		{
			name: "valid blog PRD",
			prd: PRD{
				ProjectName: "my-blog",
				Mode:        PRDModeBlog,
				OutputFile:  "content/post.md",
				UserStories: []UserStory{
					{ID: "BLOG-001", Title: "Write intro", Priority: 1},
					{ID: "BLOG-002", Title: "Write body", Priority: 2},
				},
			},
			wantErr: false,
		},
		{
			name: "valid podcast PRD",
			prd: PRD{
				ProjectName: "podcast-ep1",
				Mode:        PRDModePodcast,
				OutputFile:  "episodes/ep1.md",
				UserStories: []UserStory{
					{ID: "POD-001", Title: "Cold open", Priority: 1},
				},
			},
			wantErr: false,
		},
		{
			name: "missing project name",
			prd: PRD{
				Mode: PRDModeCode,
				UserStories: []UserStory{
					{ID: "US-001", Title: "Add feature"},
				},
			},
			wantErr: true,
			errMsg:  "projectName is required",
		},
		{
			name: "missing mode",
			prd: PRD{
				ProjectName: "test",
				UserStories: []UserStory{
					{ID: "US-001", Title: "Add feature"},
				},
			},
			wantErr: true,
			errMsg:  "mode is required",
		},
		{
			name: "invalid mode",
			prd: PRD{
				ProjectName: "test",
				Mode:        "invalid",
				UserStories: []UserStory{
					{ID: "US-001", Title: "Add feature"},
				},
			},
			wantErr: true,
			errMsg:  "mode must be one of",
		},
		{
			name: "no user stories",
			prd: PRD{
				ProjectName: "test",
				Mode:        PRDModeCode,
				UserStories: []UserStory{},
			},
			wantErr: true,
			errMsg:  "at least one user story",
		},
		{
			name: "missing story ID",
			prd: PRD{
				ProjectName: "test",
				Mode:        PRDModeCode,
				UserStories: []UserStory{
					{Title: "Missing ID"},
				},
			},
			wantErr: true,
			errMsg:  "must have an ID",
		},
		{
			name: "duplicate story IDs",
			prd: PRD{
				ProjectName: "test",
				Mode:        PRDModeCode,
				UserStories: []UserStory{
					{ID: "US-001", Title: "First"},
					{ID: "US-001", Title: "Duplicate"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate story ID",
		},
		{
			name: "missing story title",
			prd: PRD{
				ProjectName: "test",
				Mode:        PRDModeCode,
				UserStories: []UserStory{
					{ID: "US-001"},
				},
			},
			wantErr: true,
			errMsg:  "must have a title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.prd.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPRDIncompleteStories(t *testing.T) {
	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-003", Title: "Third", Priority: 3, Passes: false},
			{ID: "US-001", Title: "First", Priority: 1, Passes: true},
			{ID: "US-002", Title: "Second", Priority: 2, Passes: false},
		},
	}

	incomplete := prd.IncompleteStories()
	if len(incomplete) != 2 {
		t.Fatalf("expected 2 incomplete stories, got %d", len(incomplete))
	}

	// Should be sorted by priority
	if incomplete[0].ID != "US-002" {
		t.Errorf("expected first incomplete to be US-002 (priority 2), got %s", incomplete[0].ID)
	}
	if incomplete[1].ID != "US-003" {
		t.Errorf("expected second incomplete to be US-003 (priority 3), got %s", incomplete[1].ID)
	}
}

func TestPRDNextStory(t *testing.T) {
	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-001", Title: "First", Priority: 1, Passes: true},
			{ID: "US-002", Title: "Second", Priority: 2, Passes: false},
			{ID: "US-003", Title: "Third", Priority: 3, Passes: false},
		},
	}

	next := prd.NextStory()
	if next == nil {
		t.Fatal("expected a next story, got nil")
	}
	if next.ID != "US-002" {
		t.Errorf("expected US-002, got %s", next.ID)
	}

	// Mark US-002 complete
	prd.MarkStoryComplete("US-002")
	next = prd.NextStory()
	if next == nil {
		t.Fatal("expected a next story after completing US-002, got nil")
	}
	if next.ID != "US-003" {
		t.Errorf("expected US-003, got %s", next.ID)
	}

	// Mark US-003 complete
	prd.MarkStoryComplete("US-003")
	next = prd.NextStory()
	if next != nil {
		t.Errorf("expected nil (all complete), got %s", next.ID)
	}
}

func TestPRDAllComplete(t *testing.T) {
	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-001", Title: "First", Priority: 1, Passes: false},
		},
	}

	if prd.AllComplete() {
		t.Error("expected not all complete")
	}

	prd.MarkStoryComplete("US-001")

	if !prd.AllComplete() {
		t.Error("expected all complete after marking")
	}
}

func TestPRDCompletedCount(t *testing.T) {
	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-001", Title: "First", Priority: 1, Passes: true},
			{ID: "US-002", Title: "Second", Priority: 2, Passes: false},
			{ID: "US-003", Title: "Third", Priority: 3, Passes: true},
		},
	}

	if count := prd.CompletedCount(); count != 2 {
		t.Errorf("expected 2 completed, got %d", count)
	}
}

func TestPRDStatusSummary(t *testing.T) {
	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-001", Title: "Done task", Priority: 1, Passes: true},
			{ID: "US-002", Title: "Pending task", Priority: 2, Passes: false},
		},
	}

	summary := prd.StatusSummary()
	if !contains(summary, "[DONE] US-001") {
		t.Error("expected DONE for US-001 in summary")
	}
	if !contains(summary, "[TODO] US-002") {
		t.Error("expected TODO for US-002 in summary")
	}
}

func TestPRDMarkStoryComplete(t *testing.T) {
	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-001", Title: "First", Passes: false},
		},
	}

	// Mark existing story
	if !prd.MarkStoryComplete("US-001") {
		t.Error("expected true when marking existing story")
	}
	if !prd.UserStories[0].Passes {
		t.Error("story should be marked as passing")
	}

	// Try marking non-existent story
	if prd.MarkStoryComplete("US-999") {
		t.Error("expected false when marking non-existent story")
	}
}

func TestPRDLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-prd.json")

	// Create and save a PRD
	prd := &PRD{
		ProjectName: "roundtrip-test",
		Mode:        PRDModeBlog,
		OutputFile:  "output.md",
		UserStories: []UserStory{
			{
				ID:                 "BLOG-001",
				Title:              "Write intro",
				Description:        "Write the introduction section",
				AcceptanceCriteria: []string{"Clear hook", "Under 200 words"},
				Priority:           1,
				Passes:             false,
			},
		},
	}

	if err := prd.SavePRD(path); err != nil {
		t.Fatalf("failed to save PRD: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("PRD file was not created")
	}

	// Load it back
	loaded, err := LoadPRD(path)
	if err != nil {
		t.Fatalf("failed to load PRD: %v", err)
	}

	if loaded.ProjectName != prd.ProjectName {
		t.Errorf("project name mismatch: got %q, want %q", loaded.ProjectName, prd.ProjectName)
	}
	if loaded.Mode != prd.Mode {
		t.Errorf("mode mismatch: got %q, want %q", loaded.Mode, prd.Mode)
	}
	if loaded.OutputFile != prd.OutputFile {
		t.Errorf("output file mismatch: got %q, want %q", loaded.OutputFile, prd.OutputFile)
	}
	if len(loaded.UserStories) != 1 {
		t.Fatalf("expected 1 story, got %d", len(loaded.UserStories))
	}
	if loaded.UserStories[0].ID != "BLOG-001" {
		t.Errorf("story ID mismatch: got %q", loaded.UserStories[0].ID)
	}
	if len(loaded.UserStories[0].AcceptanceCriteria) != 2 {
		t.Errorf("expected 2 acceptance criteria, got %d", len(loaded.UserStories[0].AcceptanceCriteria))
	}
}

func TestParsePRD(t *testing.T) {
	json := []byte(`{
		"projectName": "test-project",
		"mode": "code",
		"branchName": "feature/test",
		"userStories": [
			{
				"id": "US-001",
				"title": "Add endpoint",
				"description": "Add GET /health endpoint",
				"acceptanceCriteria": ["Returns 200", "Has test"],
				"priority": 1,
				"passes": false
			}
		]
	}`)

	prd, err := ParsePRD(json)
	if err != nil {
		t.Fatalf("failed to parse PRD: %v", err)
	}

	if prd.ProjectName != "test-project" {
		t.Errorf("expected project name 'test-project', got %q", prd.ProjectName)
	}
	if prd.BranchName != "feature/test" {
		t.Errorf("expected branch name 'feature/test', got %q", prd.BranchName)
	}
	if len(prd.UserStories) != 1 {
		t.Fatalf("expected 1 story, got %d", len(prd.UserStories))
	}
}

func TestParsePRDInvalid(t *testing.T) {
	_, err := ParsePRD([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	_, err = ParsePRD([]byte(`{"mode": "code"}`))
	if err == nil {
		t.Error("expected error for missing projectName")
	}
}
