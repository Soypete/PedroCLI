package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/storage/blog"
)

// MockBlogStorage implements blog.BlogStorage for testing
type MockBlogStorage struct {
	posts    map[string]*blog.BlogPost
	versions map[string][]*blog.PostVersion
}

func NewMockBlogStorage() *MockBlogStorage {
	return &MockBlogStorage{
		posts:    make(map[string]*blog.BlogPost),
		versions: make(map[string][]*blog.PostVersion),
	}
}

func (m *MockBlogStorage) CreatePost(ctx context.Context, post *blog.BlogPost) error {
	m.posts[post.ID.String()] = post
	return nil
}

func (m *MockBlogStorage) UpdatePost(ctx context.Context, post *blog.BlogPost) error {
	m.posts[post.ID.String()] = post
	return nil
}

func (m *MockBlogStorage) GetPost(ctx context.Context, id uuid.UUID) (*blog.BlogPost, error) {
	return m.posts[id.String()], nil
}

func (m *MockBlogStorage) ListPosts(ctx context.Context, status blog.PostStatus) ([]*blog.BlogPost, error) {
	return nil, nil
}

func (m *MockBlogStorage) DeletePost(ctx context.Context, id uuid.UUID) error {
	delete(m.posts, id.String())
	return nil
}

func (m *MockBlogStorage) CreateVersion(ctx context.Context, version *blog.PostVersion) error {
	postID := version.PostID.String()
	m.versions[postID] = append(m.versions[postID], version)
	return nil
}

func (m *MockBlogStorage) GetVersion(ctx context.Context, postID uuid.UUID, versionNumber int) (*blog.PostVersion, error) {
	return nil, nil
}

func (m *MockBlogStorage) ListVersions(ctx context.Context, postID uuid.UUID) ([]*blog.PostVersion, error) {
	return m.versions[postID.String()], nil
}

func (m *MockBlogStorage) GetNextVersionNumber(ctx context.Context, postID uuid.UUID) (int, error) {
	return len(m.versions[postID.String()]) + 1, nil
}

func (m *MockBlogStorage) Close() error {
	return nil
}

// MockBackend implements llm.Backend for testing
type MockBlogBackend struct {
	responses []string
	callCount int
}

func NewMockBlogBackend(responses []string) *MockBlogBackend {
	return &MockBlogBackend{
		responses: responses,
		callCount: 0,
	}
}

func (m *MockBlogBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	if m.callCount >= len(m.responses) {
		return &llm.InferenceResponse{
			Text:       "RESEARCH_COMPLETE\n\nNo more responses available.",
			TokensUsed: 50,
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++

	return &llm.InferenceResponse{
		Text:       response,
		TokensUsed: len(response) / 4,
		ToolCalls:  []llm.ToolCall{}, // No tool calls in mock
	}, nil
}

func (m *MockBlogBackend) GetContextWindow() int {
	return 32000
}

func (m *MockBlogBackend) GetUsableContext() int {
	return 24000 // 75% of context window
}

func TestBlogContentAgent_ExtractResearchSummary(t *testing.T) {
	agent := &BlogContentAgent{}

	tests := []struct {
		name     string
		history  string
		contains string // Check if result contains this string (more flexible than exact match)
	}{
		{
			name: "with RESEARCH_COMPLETE marker",
			history: `User: Do research
Assistant: I'll search for information
RESEARCH_COMPLETE

Summary:
- Found code in pkg/agents/executor.go
- Found tests in pkg/agents/executor_test.go`,
			contains: "Found code in pkg/agents/executor.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.extractResearchSummary(tt.history)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("extractResearchSummary() result does not contain %q.\nGot: %q", tt.contains, result)
			}
		})
	}
}

func TestBlogContentAgent_ExtractSectionContent(t *testing.T) {
	agent := &BlogContentAgent{}

	tests := []struct {
		name     string
		history  string
		contains string // Check if result contains this string (more flexible than exact match)
	}{
		{
			name: "with SECTION_COMPLETE marker",
			history: `User: Write section
Assistant: I'll write the section
SECTION_COMPLETE

The InferenceExecutor is the heart of autonomous operation. It runs an iterative loop that:
1. Sends prompts to the LLM
2. Parses tool calls
3. Executes tools
4. Feeds results back`,
			contains: "InferenceExecutor is the heart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.extractSectionContent(tt.history)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("extractSectionContent() result does not contain %q.\nGot: %q", tt.contains, result)
			}
		})
	}
}

func TestNewBlogContentAgent(t *testing.T) {
	mockBackend := NewMockBlogBackend([]string{"Test response"})
	mockStorage := NewMockBlogStorage()

	cfg := BlogContentAgentConfig{
		Backend:       mockBackend,
		Storage:       mockStorage,
		WorkingDir:    ".",
		MaxIterations: 20,
		Transcription: "Test transcription about Go code",
		Title:         "Test Blog Post",
		Config: &config.Config{
			Model: config.ModelConfig{
				Type:        "ollama",
				ModelName:   "qwen2.5-coder:32b",
				ContextSize: 32000,
			},
			Project: config.ProjectConfig{
				Name:    "test-project",
				Workdir: ".",
			},
			Limits: config.LimitsConfig{
				MaxInferenceRuns: 20,
			},
		},
	}

	agent := NewBlogContentAgent(cfg)

	// Verify agent was created properly
	if agent == nil {
		t.Fatal("NewBlogContentAgent() returned nil")
	}

	if agent.backend == nil {
		t.Error("backend is nil")
	}

	if agent.storage == nil {
		t.Error("storage is nil")
	}

	if agent.baseAgent == nil {
		t.Error("baseAgent is nil")
	}

	if agent.progress == nil {
		t.Error("progress tracker is nil")
	}

	// Verify tools were registered
	if len(agent.toolsList) == 0 {
		t.Error("no tools registered")
	}

	// Check that baseAgent has tools registered
	if len(agent.baseAgent.tools) == 0 {
		t.Error("baseAgent has no tools registered")
	}

	// Verify progress tracker has phases
	phases := agent.progress.GetPhases()
	expectedPhases := []string{
		"Transcribe",
		"Research",
		"Outline",
		"Generate Sections",
		"Assemble",
		"Editor Review",
		"Publish",
	}

	if len(phases) != len(expectedPhases) {
		t.Errorf("progress has %d phases, expected %d", len(phases), len(expectedPhases))
	}

	for i, phase := range phases {
		if phase.Name != expectedPhases[i] {
			t.Errorf("phase %d: got %q, expected %q", i, phase.Name, expectedPhases[i])
		}
	}
}
