// Package github provides a GitHub API client for programmatic access to GitHub.
//
// This package is used by the GitHub tool to interact with GitHub's REST API.
// It supports fetching PRs, issues, files, and creating PRs/comments.
//
// # Authentication
//
// Provide a personal access token (PAT) or GitHub App token to NewClient().
// The token should have appropriate scopes:
//
//   - repo: Full read/write access to repositories
//   - read:org: Read organization information
//
// # Usage
//
//	client := github.NewClient("ghp_...")
//
//	// Fetch a PR
//	pr, err := client.FetchPR(ctx, "owner", "repo", 123)
//
//	// Fetch PR files
//	files, err := client.FetchPRFiles(ctx, "owner", "repo", 123)
//
//	// Fetch issue with comments
//	issue, err := client.FetchIssue(ctx, "owner", "repo", 456)
//	comments, err := client.FetchIssueComments(ctx, "owner", "repo", 456)
//
//	// Create a PR
//	pr, err := client.CreatePR(ctx, "owner", "repo", github.CreatePRRequest{
//	    Title: "Add feature",
//	    Body:  "Description",
//	    Head:  "feature-branch",
//	    Base:  "main",
//	    Draft: true,
//	})
//
// # Rate Limiting
//
// The GitHub API imposes rate limits (5000 requests/hour for authenticated requests).
// This client does not implement rate limiting - add a rate limiter if needed.
//
// # Error Handling
//
// All methods return errors that include the HTTP status code and response body
// for debugging failed requests.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	APIBaseURL = "https://api.github.com"
)

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, APIBaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

type PullRequest struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	Head      Branch `json:"head"`
	Base      Branch `json:"base"`
	User      User   `json:"user"`
	URL       string `json:"html_url"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Files     []File `json:"files"`
}

type Branch struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type User struct {
	Login string `json:"login"`
}

type File struct {
	SHA       string `json:"sha"`
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

type Issue struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	State    string    `json:"state"`
	User     User      `json:"user"`
	Labels   []Label   `json:"labels"`
	URL      string    `json:"html_url"`
	Comments []Comment `json:"comments"`
}

type Label struct {
	Name string `json:"name"`
}

type Comment struct {
	ID        int    `json:"id"`
	Body      string `json:"body"`
	User      User   `json:"user"`
	CreatedAt string `json:"created_at"`
}

type CreatePRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Draft bool   `json:"draft"`
}

type CreateCommentRequest struct {
	Body string `json:"body"`
}

func (c *Client) FetchPR(ctx context.Context, owner, repo string, prNum int) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, prNum)
	respBody, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var pr PullRequest
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PR response: %w", err)
	}

	return &pr, nil
}

func (c *Client) FetchPRFiles(ctx context.Context, owner, repo string, prNum int) ([]File, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files", owner, repo, prNum)
	respBody, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var files []File
	if err := json.Unmarshal(respBody, &files); err != nil {
		return nil, fmt.Errorf("failed to parse files response: %w", err)
	}

	return files, nil
}

func (c *Client) FetchPRDiff(ctx context.Context, owner, repo string, prNum int) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, prNum)

	req, err := http.NewRequestWithContext(ctx, "GET", APIBaseURL+path, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3.diff")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
	}

	diffBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(diffBody), nil
}

func (c *Client) FetchIssue(ctx context.Context, owner, repo string, issueNum int) (*Issue, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, issueNum)
	respBody, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var issue Issue
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return nil, fmt.Errorf("failed to parse issue response: %w", err)
	}

	return &issue, nil
}

func (c *Client) FetchIssueComments(ctx context.Context, owner, repo string, issueNum int) ([]Comment, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, issueNum)
	respBody, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var comments []Comment
	if err := json.Unmarshal(respBody, &comments); err != nil {
		return nil, fmt.Errorf("failed to parse comments response: %w", err)
	}

	return comments, nil
}

func (c *Client) CreatePR(ctx context.Context, owner, repo string, req CreatePRRequest) (*PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	respBody, err := c.doRequest(ctx, "POST", path, req)
	if err != nil {
		return nil, err
	}

	var pr PullRequest
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse PR response: %w", err)
	}

	return &pr, nil
}

func (c *Client) AddComment(ctx context.Context, owner, repo string, issueNum int, body string) (*Comment, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, issueNum)
	respBody, err := c.doRequest(ctx, "POST", path, CreateCommentRequest{Body: body})
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := json.Unmarshal(respBody, &comment); err != nil {
		return nil, fmt.Errorf("failed to parse comment response: %w", err)
	}

	return &comment, nil
}

func ParseRepo(repo string) (owner, name string, err error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %s (expected owner/repo)", repo)
	}
	return parts[0], parts[1], nil
}
