package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/llm"
)

// GenerateTLDROptions configures TLDR generation
type GenerateTLDROptions struct {
	Outline     string  // Blog post outline
	Research    string  // Research data
	MaxBullets  int     // Max number of bullet points (default: 5)
	MaxTokens   int     // Max tokens for TLDR (default: 200)
	Temperature float64 // Temperature for generation (default: 0.3)
	UseGrammar  bool    // Use grammar constraints (default: true)
}

// GenerateTLDR generates a concise TLDR section for a blog post
// Uses grammar constraints and low temperature for focused, brief output
func GenerateTLDR(ctx context.Context, backend llm.Backend, opts GenerateTLDROptions) (string, error) {
	// Set defaults
	if opts.MaxBullets == 0 {
		opts.MaxBullets = 5
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 200
	}
	if opts.Temperature == 0 {
		opts.Temperature = 0.3
	}

	// Build system prompt for TLDR generation
	systemPrompt := fmt.Sprintf(`You are writing a TLDR (Too Long; Didn't Read) section for a technical blog post.

REQUIREMENTS:
- Exactly %d bullet points or fewer
- Each bullet should be 1-2 sentences maximum
- Focus on KEY takeaways, not details
- Use clear, concise language
- NO emojis or special formatting
- Start each bullet with a dash (-)

OUTPUT FORMAT:
- First key takeaway
- Second key takeaway
- Third key takeaway`, opts.MaxBullets)

	// Build user prompt
	userPrompt := fmt.Sprintf(`Create a TLDR section for this blog post.

OUTLINE:
%s

%s

Write the TLDR now (max %d bullet points):`,
		opts.Outline,
		addResearchSection(opts.Research),
		opts.MaxBullets,
	)

	// Build grammar constraint for bullet list format
	var grammar string
	if opts.UseGrammar {
		grammar = generateBulletListGrammar(opts.MaxBullets)
	}

	// Perform inference
	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  opts.Temperature,
		MaxTokens:    opts.MaxTokens,
		Grammar:      grammar,
	}

	resp, err := backend.Infer(ctx, req)
	if err != nil {
		return "", fmt.Errorf("TLDR generation failed: %w", err)
	}

	return strings.TrimSpace(resp.Text), nil
}

// SocialMediaPlatform represents a social media platform
type SocialMediaPlatform string

const (
	PlatformTwitter  SocialMediaPlatform = "twitter"
	PlatformBluesky  SocialMediaPlatform = "bluesky"
	PlatformLinkedIn SocialMediaPlatform = "linkedin"
)

// SocialMediaPostOptions configures social media post generation
type SocialMediaPostOptions struct {
	Platform    SocialMediaPlatform
	Content     string  // Blog content or summary
	Link        string  // Link to blog post
	Temperature float64 // Temperature (default: 0.4)
	UseGrammar  bool    // Use grammar constraints (default: true)
}

// GenerateSocialMediaPost generates a platform-specific social media post
// Enforces character limits via max_tokens and grammar constraints
func GenerateSocialMediaPost(ctx context.Context, backend llm.Backend, opts SocialMediaPostOptions) (string, error) {
	var maxTokens int
	var maxChars int
	var platformName string

	// Set platform-specific limits
	switch opts.Platform {
	case PlatformTwitter:
		maxTokens = 70 // ~280 chars
		maxChars = 280
		platformName = "Twitter/X"
	case PlatformBluesky:
		maxTokens = 75 // ~300 chars
		maxChars = 300
		platformName = "Bluesky"
	case PlatformLinkedIn:
		maxTokens = 750 // ~3000 chars
		maxChars = 3000
		platformName = "LinkedIn"
	default:
		return "", fmt.Errorf("unsupported platform: %s", opts.Platform)
	}

	// Set defaults
	if opts.Temperature == 0 {
		opts.Temperature = 0.4
	}

	// Build system prompt
	systemPrompt := fmt.Sprintf(`You are writing a %s post about a blog post.

REQUIREMENTS:
- Maximum %d characters
- Include the blog link: %s
- Add ONE relevant hashtag at the end
- Keep it concise and engaging
- NO emojis

FORMAT:
[Brief compelling summary] [Link] #hashtag`, platformName, maxChars, opts.Link)

	// Build user prompt with content summary
	contentSummary := opts.Content
	if len(contentSummary) > 500 {
		contentSummary = contentSummary[:500] + "..."
	}

	userPrompt := fmt.Sprintf(`Write a %s post for this blog content:

%s

Write the post now:`, platformName, contentSummary)

	// Build grammar constraint for social post format
	var grammar string
	if opts.UseGrammar {
		grammar = generateSocialPostGrammar(opts.Platform, opts.Link)
	}

	// Perform inference
	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  opts.Temperature,
		MaxTokens:    maxTokens,
		Grammar:      grammar,
	}

	resp, err := backend.Infer(ctx, req)
	if err != nil {
		return "", fmt.Errorf("social post generation failed: %w", err)
	}

	post := strings.TrimSpace(resp.Text)

	// Validate character limit
	if len(post) > maxChars {
		post = post[:maxChars-3] + "..."
	}

	return post, nil
}

// addResearchSection adds research data to prompt if provided
func addResearchSection(research string) string {
	if research == "" {
		return ""
	}
	return fmt.Sprintf("RESEARCH DATA:\n%s\n", research)
}

// generateBulletListGrammar creates a GBNF grammar for bullet list format
func generateBulletListGrammar(maxBullets int) string {
	// This is a simplified grammar - can be enhanced
	// For now, we rely more on system prompt than strict grammar
	return fmt.Sprintf(`root ::= bullet bullet? bullet? bullet? bullet?
bullet ::= "- " [^\n]+ "\n"`)
}

// generateSocialPostGrammar creates a GBNF grammar for social media post format
func generateSocialPostGrammar(platform SocialMediaPlatform, link string) string {
	// Simplified grammar - relies on system prompt for most constraints
	// Full GBNF implementation would be more complex
	switch platform {
	case PlatformTwitter, PlatformBluesky:
		return `root ::= summary " " link " " hashtag
summary ::= [^\n]{10,180}
link ::= "https://" [a-zA-Z0-9./_\-]+
hashtag ::= "#" [a-zA-Z0-9]+`
	case PlatformLinkedIn:
		return `root ::= summary "\n\n" link "\n\n" hashtag
summary ::= [^\n]{50,2800}
link ::= "https://" [a-zA-Z0-9./_\-]+
hashtag ::= "#" [a-zA-Z0-9]+`
	default:
		return ""
	}
}
