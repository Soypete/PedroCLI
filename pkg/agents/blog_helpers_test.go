package agents

import (
	"context"
	"testing"

	"github.com/soypete/pedrocli/pkg/llm"
)

// MockBackend for testing
type MockBackend struct {
	response string
}

func (m *MockBackend) Infer(ctx context.Context, req *llm.InferenceRequest) (*llm.InferenceResponse, error) {
	return &llm.InferenceResponse{
		Text:       m.response,
		TokensUsed: 50,
	}, nil
}

func (m *MockBackend) GetContextWindow() int {
	return 8192
}

func (m *MockBackend) GetUsableContext() int {
	return 6144
}

func TestGenerateTLDR(t *testing.T) {
	mockBackend := &MockBackend{
		response: "- First key point about the topic\n- Second important takeaway\n- Third critical insight",
	}

	opts := GenerateTLDROptions{
		Outline:  "# Introduction\n\nTest outline content",
		Research: "Test research data",
	}

	tldr, err := GenerateTLDR(context.Background(), mockBackend, opts)
	if err != nil {
		t.Fatalf("GenerateTLDR failed: %v", err)
	}

	if tldr == "" {
		t.Error("Expected non-empty TLDR")
	}

	if len(tldr) > 1000 {
		t.Errorf("TLDR too long: %d characters", len(tldr))
	}
}

func TestGenerateTLDR_Defaults(t *testing.T) {
	mockBackend := &MockBackend{
		response: "- Test point",
	}

	opts := GenerateTLDROptions{
		Outline: "Test outline",
	}

	// Test that defaults are applied
	_, err := GenerateTLDR(context.Background(), mockBackend, opts)
	if err != nil {
		t.Fatalf("GenerateTLDR with defaults failed: %v", err)
	}
}

func TestGenerateSocialMediaPost_Twitter(t *testing.T) {
	mockBackend := &MockBackend{
		response: "Great blog post about Go programming! https://example.com/post #golang",
	}

	opts := SocialMediaPostOptions{
		Platform: PlatformTwitter,
		Content:  "Test blog content about Go programming",
		Link:     "https://example.com/post",
	}

	post, err := GenerateSocialMediaPost(context.Background(), mockBackend, opts)
	if err != nil {
		t.Fatalf("GenerateSocialMediaPost failed: %v", err)
	}

	if post == "" {
		t.Error("Expected non-empty post")
	}

	if len(post) > 280 {
		t.Errorf("Twitter post too long: %d characters (max 280)", len(post))
	}
}

func TestGenerateSocialMediaPost_Bluesky(t *testing.T) {
	mockBackend := &MockBackend{
		response: "Interesting insights on cloud native development https://example.com/post #cloudnative",
	}

	opts := SocialMediaPostOptions{
		Platform: PlatformBluesky,
		Content:  "Test blog about cloud native",
		Link:     "https://example.com/post",
	}

	post, err := GenerateSocialMediaPost(context.Background(), mockBackend, opts)
	if err != nil {
		t.Fatalf("GenerateSocialMediaPost failed: %v", err)
	}

	if len(post) > 300 {
		t.Errorf("Bluesky post too long: %d characters (max 300)", len(post))
	}
}

func TestGenerateSocialMediaPost_LinkedIn(t *testing.T) {
	mockBackend := &MockBackend{
		response: "Deep dive into Kubernetes operators and how they simplify complex deployments. https://example.com/post #kubernetes",
	}

	opts := SocialMediaPostOptions{
		Platform: PlatformLinkedIn,
		Content:  "Test blog about Kubernetes operators with lots of detail and technical content",
		Link:     "https://example.com/post",
	}

	post, err := GenerateSocialMediaPost(context.Background(), mockBackend, opts)
	if err != nil {
		t.Fatalf("GenerateSocialMediaPost failed: %v", err)
	}

	if len(post) > 3000 {
		t.Errorf("LinkedIn post too long: %d characters (max 3000)", len(post))
	}
}

func TestGenerateSocialMediaPost_UnsupportedPlatform(t *testing.T) {
	mockBackend := &MockBackend{
		response: "test",
	}

	opts := SocialMediaPostOptions{
		Platform: "invalid",
		Content:  "test",
		Link:     "https://example.com",
	}

	_, err := GenerateSocialMediaPost(context.Background(), mockBackend, opts)
	if err == nil {
		t.Error("Expected error for unsupported platform")
	}
}

func TestGenerateBulletListGrammar(t *testing.T) {
	grammar := generateBulletListGrammar(5)
	if grammar == "" {
		t.Error("Expected non-empty grammar")
	}
}

func TestGenerateSocialPostGrammar(t *testing.T) {
	testCases := []struct {
		platform SocialMediaPlatform
		link     string
	}{
		{PlatformTwitter, "https://example.com"},
		{PlatformBluesky, "https://example.com"},
		{PlatformLinkedIn, "https://example.com"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.platform), func(t *testing.T) {
			grammar := generateSocialPostGrammar(tc.platform, tc.link)
			if grammar == "" {
				t.Errorf("Expected non-empty grammar for %s", tc.platform)
			}
		})
	}
}
