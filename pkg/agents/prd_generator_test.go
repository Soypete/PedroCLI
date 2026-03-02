package agents

import (
	"context"
	"testing"
)

func TestGeneratePRD(t *testing.T) {
	mockResponse := `{
  "projectName": "rate-limiter",
  "mode": "code",
  "userStories": [
    {
      "id": "US-001",
      "title": "Add token bucket rate limiter",
      "description": "Implement a token bucket rate limiter for API endpoints",
      "acceptanceCriteria": ["Limits requests per second", "Returns 429 when exceeded"],
      "priority": 1,
      "passes": false
    },
    {
      "id": "US-002",
      "title": "Add rate limit middleware",
      "description": "Create HTTP middleware that applies the rate limiter",
      "acceptanceCriteria": ["Wraps any handler", "Configurable limit"],
      "priority": 2,
      "passes": false
    }
  ]
}`

	backend := &MockBackend{response: mockResponse}

	prd, err := GeneratePRD(context.Background(), backend, "Add rate limiting to the API", PRDModeCode)
	if err != nil {
		t.Fatalf("GeneratePRD failed: %v", err)
	}

	if prd.ProjectName != "rate-limiter" {
		t.Errorf("expected project name 'rate-limiter', got %q", prd.ProjectName)
	}
	if prd.Mode != PRDModeCode {
		t.Errorf("expected mode 'code', got %q", prd.Mode)
	}
	if len(prd.UserStories) != 2 {
		t.Fatalf("expected 2 stories, got %d", len(prd.UserStories))
	}
	if prd.UserStories[0].ID != "US-001" {
		t.Errorf("expected first story ID 'US-001', got %q", prd.UserStories[0].ID)
	}
}

func TestGeneratePRDWithCodeFence(t *testing.T) {
	// Test that the LLM wrapping response in code fences still works
	mockResponse := "```json\n" + `{
  "projectName": "my-blog",
  "mode": "blog",
  "outputFile": "content/post.md",
  "userStories": [
    {
      "id": "BLOG-001",
      "title": "Write intro",
      "description": "Write the opening section",
      "acceptanceCriteria": ["Has a hook"],
      "priority": 1,
      "passes": false
    }
  ]
}` + "\n```"

	backend := &MockBackend{response: mockResponse}

	prd, err := GeneratePRD(context.Background(), backend, "Write about Go contexts", PRDModeBlog)
	if err != nil {
		t.Fatalf("GeneratePRD with code fence failed: %v", err)
	}

	if prd.ProjectName != "my-blog" {
		t.Errorf("expected project name 'my-blog', got %q", prd.ProjectName)
	}
	// Mode should be overridden to match the requested mode
	if prd.Mode != PRDModeBlog {
		t.Errorf("expected mode 'blog', got %q", prd.Mode)
	}
}

func TestGeneratePRDModeOverride(t *testing.T) {
	// Even if LLM outputs wrong mode, it should be corrected
	mockResponse := `{
  "projectName": "test",
  "mode": "code",
  "userStories": [
    {
      "id": "POD-001",
      "title": "Episode structure",
      "description": "Define episode structure",
      "acceptanceCriteria": ["Has segments"],
      "priority": 1,
      "passes": false
    }
  ]
}`

	backend := &MockBackend{response: mockResponse}

	prd, err := GeneratePRD(context.Background(), backend, "Plan a podcast episode", PRDModePodcast)
	if err != nil {
		t.Fatalf("GeneratePRD failed: %v", err)
	}

	if prd.Mode != PRDModePodcast {
		t.Errorf("expected mode override to 'podcast', got %q", prd.Mode)
	}
}

func TestGeneratePRDNilBackend(t *testing.T) {
	_, err := GeneratePRD(context.Background(), nil, "test", PRDModeCode)
	if err == nil {
		t.Error("expected error with nil backend")
	}
}

func TestGeneratePRDInvalidResponse(t *testing.T) {
	backend := &MockBackend{response: "I cannot generate a PRD for that request."}

	_, err := GeneratePRD(context.Background(), backend, "test", PRDModeCode)
	if err == nil {
		t.Error("expected error for non-JSON response")
	}
}
