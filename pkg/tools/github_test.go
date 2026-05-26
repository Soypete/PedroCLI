package tools

import (
	"context"
	"testing"
)

func TestGitHubToolName(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	if tool.Name() != "github" {
		t.Errorf("Name() = %v, want github", tool.Name())
	}
}

func TestGitHubToolDescription(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	if len(desc) < 100 {
		t.Error("Description() should be comprehensive")
	}
}

func TestGitHubToolExecute_MissingAction(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for missing action")
	}
	if result.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestGitHubToolExecute_UnknownAction(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "unknown_action",
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for unknown action")
	}
	if result.Error == "" {
		t.Error("Error should not be empty")
	}
}

func TestGitHubTool_NewGitHubToolWithToken(t *testing.T) {
	tool := NewGitHubToolWithToken("/tmp", "test-token")
	if tool.githubToken != "test-token" {
		t.Errorf("githubToken = %v, want test-token", tool.githubToken)
	}
	if tool.httpClient == nil {
		t.Error("httpClient should not be nil when token is provided")
	}
}

func TestGitHubTool_NewGitHubToolWithEmptyToken(t *testing.T) {
	tool := NewGitHubToolWithToken("/tmp", "")
	if tool.httpClient != nil {
		t.Error("httpClient should be nil when token is empty")
	}
}

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantNum   int
		wantErr   bool
	}{
		{
			name:      "standard https URL",
			url:       "https://github.com/owner/repo/pull/123",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   123,
			wantErr:   false,
		},
		{
			name:      "URL without https",
			url:       "github.com/owner/repo/pull/456",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   456,
			wantErr:   false,
		},
		{
			name:      "URL with files suffix",
			url:       "https://github.com/owner/repo/pull/789/files",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   789,
			wantErr:   false,
		},
		{
			name:      "URL with diff suffix",
			url:       "https://github.com/owner/repo/pull/100/diff",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantNum:   100,
			wantErr:   false,
		},
		{
			name:    "invalid URL - not a PR",
			url:     "https://github.com/owner/repo/issues/123",
			wantErr: true,
		},
		{
			name:    "invalid URL - wrong format",
			url:     "https://github.com/owner/repo",
			wantErr: true,
		},
		{
			name:    "invalid URL - missing PR number",
			url:     "https://github.com/owner/repo/pull/abc",
			wantErr: true,
		},
		{
			name:    "invalid URL - empty",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, num, err := parsePRURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePRURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("repo = %v, want %v", repo, tt.wantRepo)
				}
				if num != tt.wantNum {
					t.Errorf("num = %v, want %v", num, tt.wantNum)
				}
			}
		})
	}
}

func TestGitHubToolMetadata(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	metadata := tool.Metadata()

	if metadata == nil {
		t.Fatal("Metadata() returned nil")
	}

	if metadata.Schema == nil {
		t.Error("Schema should not be nil")
	}

	if metadata.Category != CategoryVCS {
		t.Errorf("Category = %v, want vcs", metadata.Category)
	}

	if len(metadata.Produces) == 0 {
		t.Error("Produces should not be empty")
	}
}

func TestGitHubToolExecute_PRCheckout_MissingNumber(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "pr_checkout",
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for missing pr_number")
	}
}

func TestGitHubToolExecute_IssueFetch_MissingNumber(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "issue_fetch",
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for missing issue_number")
	}
}

func TestGitHubToolExecute_PRCreate_MissingTitle(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "pr_create",
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for missing title")
	}
}

func TestGitHubToolExecute_PRComment_MissingNumber(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "pr_comment",
		"body":   "test comment",
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for missing pr_number")
	}
}

func TestGitHubToolExecute_PRComment_MissingBody(t *testing.T) {
	tool := NewGitHubTool("/tmp")
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":    "pr_comment",
		"pr_number": 123,
	})
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result.Success {
		t.Error("Success = true, want false for missing body")
	}
}

func TestGitHubTool_PRInfoStruct(t *testing.T) {
	prInfo := PRInfo{
		Number:     1,
		Title:      "Test PR",
		Body:       "Test body",
		State:      "open",
		HeadBranch: "feature",
		BaseBranch: "main",
		Author:     "testuser",
		URL:        "https://github.com/owner/repo/pull/1",
		Files:      []string{"file1.go", "file2.go"},
		Additions:  100,
		Deletions:  50,
		Diff:       "diff content",
	}

	if prInfo.Number != 1 {
		t.Errorf("Number = %v, want 1", prInfo.Number)
	}
	if len(prInfo.Files) != 2 {
		t.Errorf("Files length = %v, want 2", len(prInfo.Files))
	}
}

func TestGitHubTool_IssueInfoStruct(t *testing.T) {
	issueInfo := IssueInfo{
		Number:   1,
		Title:    "Test Issue",
		Body:     "Test body",
		State:    "open",
		Author:   "testuser",
		Labels:   []string{"bug", "priority"},
		URL:      "https://github.com/owner/repo/issues/1",
		Comments: []IssueComment{{Author: "commenter", Body: "comment", CreatedAt: "2024-01-01"}},
	}

	if issueInfo.Number != 1 {
		t.Errorf("Number = %v, want 1", issueInfo.Number)
	}
	if len(issueInfo.Labels) != 2 {
		t.Errorf("Labels length = %v, want 2", len(issueInfo.Labels))
	}
	if len(issueInfo.Comments) != 1 {
		t.Errorf("Comments length = %v, want 1", len(issueInfo.Comments))
	}
}
