package repos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultPRTracker implements PRTracker using the GitHub API
type DefaultPRTracker struct {
	httpClient *http.Client
	store      Store
	gitOps     GitOps
}

// NewPRTracker creates a new PR tracker
func NewPRTracker(store Store, gitOps GitOps) *DefaultPRTracker {
	return &DefaultPRTracker{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		store:      store,
		gitOps:     gitOps,
	}
}

// CreatePR creates a new pull request using the GitHub API
func (t *DefaultPRTracker) CreatePR(ctx context.Context, repo *LocalRepo, title, body, head, base string, draft bool) (*TrackedPR, error) {
	// Build API URL based on provider
	apiURL, err := t.getAPIURL(repo.Provider, repo.Owner, repo.Name, "pulls")
	if err != nil {
		return nil, err
	}

	// Get auth token
	token := t.getToken(repo.Provider)
	if token == "" {
		return nil, fmt.Errorf("no authentication token found for %s", repo.Provider)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"title": title,
		"body":  body,
		"head":  head,
		"base":  base,
		"draft": draft,
	}

	reqJSON, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create PR: %s - %s", resp.Status, string(respBody))
	}

	// Parse response
	var prResp struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			SHA string `json:"sha"`
		} `json:"head"`
		State string `json:"state"`
	}

	if err := json.Unmarshal(respBody, &prResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Get local commit hash
	localHash, _ := t.gitOps.GetHeadHash(ctx, repo.LocalPath)

	// Create tracked PR
	pr := &TrackedPR{
		RepoID:           repo.ID,
		PRNumber:         prResp.Number,
		BranchName:       head,
		BaseBranch:       base,
		Title:            title,
		Body:             body,
		Status:           PRStatusOpen,
		LocalCommitHash:  localHash,
		RemoteCommitHash: prResp.Head.SHA,
		HTMLURL:          prResp.HTMLURL,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if draft {
		pr.Status = PRStatusDraft
	}

	// Save to store
	if t.store != nil {
		if err := t.store.SavePR(ctx, pr); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to save PR to store: %v\n", err)
		}
	}

	return pr, nil
}

// GetPR retrieves a tracked PR by number
func (t *DefaultPRTracker) GetPR(ctx context.Context, repoID string, prNumber int) (*TrackedPR, error) {
	if t.store != nil {
		return t.store.GetPR(ctx, repoID, prNumber)
	}
	return nil, fmt.Errorf("no store configured")
}

// UpdatePRStatus updates the status of a tracked PR
func (t *DefaultPRTracker) UpdatePRStatus(ctx context.Context, pr *TrackedPR) error {
	if t.store == nil {
		return fmt.Errorf("no store configured")
	}

	pr.UpdatedAt = time.Now()
	return t.store.SavePR(ctx, pr)
}

// ListPRs lists all tracked PRs for a repo
func (t *DefaultPRTracker) ListPRs(ctx context.Context, repoID string) ([]TrackedPR, error) {
	if t.store != nil {
		return t.store.ListPRs(ctx, repoID)
	}
	return nil, fmt.Errorf("no store configured")
}

// SyncPRStatus syncs PR status from remote
func (t *DefaultPRTracker) SyncPRStatus(ctx context.Context, repo *LocalRepo, prNumber int) (*TrackedPR, error) {
	// Get API URL
	apiURL, err := t.getAPIURL(repo.Provider, repo.Owner, repo.Name, "pulls/"+strconv.Itoa(prNumber))
	if err != nil {
		return nil, err
	}

	// Get auth token
	token := t.getToken(repo.Provider)
	if token == "" {
		return nil, fmt.Errorf("no authentication token found for %s", repo.Provider)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get PR: %s", resp.Status)
	}

	respBody, _ := io.ReadAll(resp.Body)

	var prResp struct {
		Number   int    `json:"number"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		State    string `json:"state"`
		Merged   bool   `json:"merged"`
		MergedAt string `json:"merged_at"`
		HTMLURL  string `json:"html_url"`
		Head     struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}

	if err := json.Unmarshal(respBody, &prResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Determine status
	var status PRStatus
	if prResp.Merged {
		status = PRStatusMerged
	} else if prResp.State == "closed" {
		status = PRStatusClosed
	} else {
		status = PRStatusOpen
	}

	// Get or create tracked PR
	var pr *TrackedPR
	if t.store != nil {
		pr, _ = t.store.GetPR(ctx, repo.ID, prNumber)
	}

	if pr == nil {
		pr = &TrackedPR{
			RepoID:    repo.ID,
			PRNumber:  prNumber,
			CreatedAt: time.Now(),
		}
	}

	pr.Title = prResp.Title
	pr.Body = prResp.Body
	pr.BranchName = prResp.Head.Ref
	pr.BaseBranch = prResp.Base.Ref
	pr.Status = status
	pr.RemoteCommitHash = prResp.Head.SHA
	pr.HTMLURL = prResp.HTMLURL
	pr.UpdatedAt = time.Now()

	if prResp.MergedAt != "" {
		mergedAt, _ := time.Parse(time.RFC3339, prResp.MergedAt)
		pr.MergedAt = &mergedAt
	}

	// Save to store
	if t.store != nil {
		if err := t.store.SavePR(ctx, pr); err != nil {
			fmt.Printf("Warning: failed to save PR to store: %v\n", err)
		}
	}

	return pr, nil
}

// IsMerged checks if a PR is merged by comparing commits
func (t *DefaultPRTracker) IsMerged(ctx context.Context, repo *LocalRepo, pr *TrackedPR) (bool, error) {
	// Sync from remote to get latest status
	syncedPR, err := t.SyncPRStatus(ctx, repo, pr.PRNumber)
	if err != nil {
		return false, err
	}

	return syncedPR.Status == PRStatusMerged, nil
}

// Helper methods

func (t *DefaultPRTracker) getAPIURL(provider, owner, repo, path string) (string, error) {
	switch provider {
	case "github.com":
		return fmt.Sprintf("https://api.github.com/repos/%s/%s/%s", owner, repo, path), nil
	case "gitlab.com":
		// GitLab uses a different API format
		return fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/%s", owner, repo, path), nil
	default:
		// Assume GitHub-compatible API
		return fmt.Sprintf("https://api.%s/repos/%s/%s/%s", provider, owner, repo, path), nil
	}
}

func (t *DefaultPRTracker) getToken(provider string) string {
	// Check environment variables for tokens
	// TODO: Support credential configuration from config file

	switch provider {
	case "github.com":
		// Try multiple common env var names
		for _, envVar := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITHUB_PAT"} {
			if token := os.Getenv(envVar); token != "" {
				return token
			}
		}
	case "gitlab.com":
		for _, envVar := range []string{"GITLAB_TOKEN", "GL_TOKEN"} {
			if token := os.Getenv(envVar); token != "" {
				return token
			}
		}
	default:
		// Try generic patterns
		upperProvider := strings.ToUpper(strings.ReplaceAll(provider, ".", "_"))
		for _, suffix := range []string{"_TOKEN", "_PAT"} {
			if token := os.Getenv(upperProvider + suffix); token != "" {
				return token
			}
		}
	}

	return ""
}

// Ensure DefaultPRTracker implements PRTracker
var _ PRTracker = (*DefaultPRTracker)(nil)
