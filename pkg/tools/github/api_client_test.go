package github

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.token != "test-token" {
		t.Errorf("token = %v, want test-token", client.token)
	}
	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name      string
		repo      string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "valid repo format",
			repo:      "owner/repo",
			wantOwner: "owner",
			wantName:  "repo",
			wantErr:   false,
		},
		{
			name:      "valid with hyphens",
			repo:      "my-org/my-repo-name",
			wantOwner: "my-org",
			wantName:  "my-repo-name",
			wantErr:   false,
		},
		{
			name:    "invalid - no slash",
			repo:    "owner-repo",
			wantErr: true,
		},
		{
			name:    "invalid - too many parts",
			repo:    "owner/repo/extra",
			wantErr: true,
		},
		{
			name:    "invalid - empty",
			repo:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, name, err := ParseRepo(tt.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("owner = %v, want %v", owner, tt.wantOwner)
				}
				if name != tt.wantName {
					t.Errorf("name = %v, want %v", name, tt.wantName)
				}
			}
		})
	}
}

func TestPullRequestStruct(t *testing.T) {
	pr := PullRequest{
		Number:    1,
		Title:     "Test PR",
		Body:      "PR body",
		State:     "open",
		Head:      Branch{Ref: "feature", SHA: "abc123"},
		Base:      Branch{Ref: "main", SHA: "def456"},
		User:      User{Login: "testuser"},
		URL:       "https://github.com/owner/repo/pull/1",
		Additions: 100,
		Deletions: 50,
	}

	if pr.Number != 1 {
		t.Errorf("Number = %v, want 1", pr.Number)
	}
	if pr.Head.Ref != "feature" {
		t.Errorf("Head.Ref = %v, want feature", pr.Head.Ref)
	}
	if pr.User.Login != "testuser" {
		t.Errorf("User.Login = %v, want testuser", pr.User.Login)
	}
}

func TestFileStruct(t *testing.T) {
	file := File{
		SHA:       "abc123",
		Filename:  "src/main.go",
		Status:    "modified",
		Additions: 10,
		Deletions: 5,
		Patch:     "patch content",
	}

	if file.Filename != "src/main.go" {
		t.Errorf("Filename = %v, want src/main.go", file.Filename)
	}
	if file.Status != "modified" {
		t.Errorf("Status = %v, want modified", file.Status)
	}
}

func TestIssueStruct(t *testing.T) {
	issue := Issue{
		Number: 1,
		Title:  "Test Issue",
		Body:   "Issue body",
		State:  "open",
		User:   User{Login: "testuser"},
		Labels: []Label{{Name: "bug"}, {Name: "priority"}},
		URL:    "https://github.com/owner/repo/issues/1",
		Comments: []Comment{
			{ID: 1, Body: "comment", User: User{Login: "commenter"}, CreatedAt: "2024-01-01"},
		},
	}

	if issue.Number != 1 {
		t.Errorf("Number = %v, want 1", issue.Number)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("Labels length = %v, want 2", len(issue.Labels))
	}
	if len(issue.Comments) != 1 {
		t.Errorf("Comments length = %v, want 1", len(issue.Comments))
	}
}

func TestCreatePRRequestStruct(t *testing.T) {
	req := CreatePRRequest{
		Title: "New PR",
		Body:  "PR description",
		Head:  "feature-branch",
		Base:  "main",
		Draft: true,
	}

	if req.Title != "New PR" {
		t.Errorf("Title = %v, want New PR", req.Title)
	}
	if !req.Draft {
		t.Error("Draft should be true")
	}
}

func TestCreateCommentRequestStruct(t *testing.T) {
	req := CreateCommentRequest{
		Body: "Test comment",
	}

	if req.Body != "Test comment" {
		t.Errorf("Body = %v, want Test comment", req.Body)
	}
}
