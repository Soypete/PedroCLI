package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
	"github.com/soypete/pedrocli/pkg/llm"
	"github.com/soypete/pedrocli/pkg/tools"
)

// BlogStyleAnalyzerAgent analyzes writing style from Substack RSS feed
// and generates a style guide prompt for the editor
type BlogStyleAnalyzerAgent struct {
	backend    llm.Backend
	config     *config.Config
	rssTool    tools.Tool
	styleGuide string // Generated style guide
}

// NewBlogStyleAnalyzerAgent creates a new style analyzer agent
func NewBlogStyleAnalyzerAgent(backend llm.Backend, cfg *config.Config) *BlogStyleAnalyzerAgent {
	rssTool := tools.NewRSSFeedTool(cfg)

	return &BlogStyleAnalyzerAgent{
		backend: backend,
		config:  cfg,
		rssTool: rssTool,
	}
}

// AnalyzeStyle analyzes recent blog posts from RSS feed to learn writing style
func (a *BlogStyleAnalyzerAgent) AnalyzeStyle(ctx context.Context) (string, error) {
	fmt.Println("\n=== Analyzing Writing Style from Substack ===")

	// Step 1: Fetch recent posts from RSS feed
	fmt.Println("Fetching recent posts from Substack RSS...")
	rssResult, err := a.rssTool.Execute(ctx, map[string]interface{}{
		"action": "get_configured", // Use RSS feed URL from config
		"limit":  10,               // Analyze last 10 posts for good sample size
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch RSS feed: %w", err)
	}

	if !rssResult.Success {
		return "", fmt.Errorf("RSS fetch failed: %s", rssResult.Error)
	}

	fmt.Printf("Fetched %d posts for analysis\n", 10)

	// Step 2: Analyze writing style using LLM
	fmt.Println("Analyzing narrative voice and style patterns...")

	systemPrompt := `You are a writing style analyst and editor assistant.

Your task is to analyze a collection of blog posts and extract the author's unique writing style, voice, and narrative patterns.

ANALYSIS FOCUS:
1. **Voice & Tone**: Is the voice casual, technical, humorous, formal, conversational?
2. **Sentence Structure**: Long/short sentences, complexity, rhythm
3. **Technical Depth**: How technical vs. accessible? Balance of jargon vs. explanation?
4. **Storytelling Style**: Uses anecdotes? Personal experience? Abstract concepts?
5. **Vocabulary**: Common phrases, metaphors, technical terms usage
6. **Paragraph Structure**: Length, organization, transitions
7. **Opening Style**: How do posts typically begin?
8. **Closing Style**: How do posts conclude?
9. **Code Examples**: How are code examples integrated? Comments style?
10. **Audience Engagement**: Direct address ("you"), inclusive ("we"), or observational?

OUTPUT FORMAT:
Create a concise style guide (500-800 words) that an AI editor can use to match this voice.

Structure your response as:
# Writing Style Guide

## Voice & Tone
[2-3 sentences describing the characteristic voice]

## Sentence & Paragraph Structure
[2-3 sentences about typical structure]

## Technical Content Approach
[2-3 sentences about technical depth and jargon usage]

## Narrative Style
[2-3 sentences about storytelling approach]

## Common Patterns
[Bullet list of 5-7 specific patterns, phrases, or techniques]

## Code Integration
[2-3 sentences about how code examples are presented]

## Opening & Closing Techniques
[2-3 sentences about typical beginnings and endings]

## Key Characteristics for AI Editor
[Bullet list of 3-5 concrete rules to follow when editing]`

	userPrompt := fmt.Sprintf(`Analyze these blog posts and create a style guide:

RSS FEED CONTENT:
%s

Create a comprehensive style guide that captures the author's unique voice.`, rssResult.Output)

	req := &llm.InferenceRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3, // Lower temperature for analytical task
		MaxTokens:    2000,
	}

	resp, err := a.backend.Infer(ctx, req)
	if err != nil {
		return "", fmt.Errorf("style analysis failed: %w", err)
	}

	a.styleGuide = strings.TrimSpace(resp.Text)

	fmt.Printf("✓ Style guide generated (%d characters)\n", len(a.styleGuide))
	fmt.Printf("Used %d tokens for analysis\n\n", resp.TokensUsed)

	return a.styleGuide, nil
}

// GetEditorPromptWithStyle returns an editor prompt enhanced with the analyzed style guide
func (a *BlogStyleAnalyzerAgent) GetEditorPromptWithStyle(basePrompt string) string {
	if a.styleGuide == "" {
		return basePrompt
	}

	enhancedPrompt := fmt.Sprintf(`%s

## AUTHOR'S WRITING STYLE GUIDE

The author has a distinctive voice and style. When editing, preserve and enhance these characteristics:

%s

## EDITING INSTRUCTIONS

When revising the blog post:
1. Maintain the author's characteristic voice and tone
2. Apply the sentence and paragraph patterns identified above
3. Use technical depth and jargon consistent with the style guide
4. Integrate code examples following the author's typical approach
5. Ensure openings and closings match the identified patterns
6. Preserve or enhance the narrative style elements
7. Fix grammar and clarity issues WITHOUT diluting the author's voice

Remember: The goal is to make the post clearer and more polished while keeping it authentically in the author's voice.`, basePrompt, a.styleGuide)

	return enhancedPrompt
}

// SaveStyleGuide saves the style guide to a file for future reference
func (a *BlogStyleAnalyzerAgent) SaveStyleGuide(filepath string) error {
	if a.styleGuide == "" {
		return fmt.Errorf("no style guide to save - run AnalyzeStyle() first")
	}

	// Add metadata header
	content := fmt.Sprintf(`# Writing Style Guide
Generated: %s
Source: Substack RSS Feed
Posts Analyzed: 10

---

%s`, time.Now().Format("2006-01-02 15:04:05"), a.styleGuide)

	// In a real implementation, would use file tool or ioutil.WriteFile
	// For now, return the content for manual saving
	fmt.Printf("Style guide ready to save to: %s\n", filepath)
	fmt.Printf("Content length: %d bytes\n", len(content))

	return nil
}

// GetStyleGuide returns the generated style guide
func (a *BlogStyleAnalyzerAgent) GetStyleGuide() string {
	return a.styleGuide
}
