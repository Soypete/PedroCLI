package vision

import (
	"context"
	"fmt"
	"strings"
)

// AltTextGenerator generates accessible alt text for images.
type AltTextGenerator struct {
	visionModel *VisionModel
}

// AltTextConfig configures alt text generation.
type AltTextConfig struct {
	MaxLength      int      `json:"max_length"`      // Maximum alt text length (default: 125)
	IncludeContext bool     `json:"include_context"` // Include surrounding context
	AvoidPhrases   []string `json:"avoid_phrases"`   // Phrases to avoid (e.g., "image of")
	LanguageStyle  string   `json:"language_style"`  // "descriptive", "concise", "technical"
}

// DefaultAltTextConfig returns default configuration.
func DefaultAltTextConfig() *AltTextConfig {
	return &AltTextConfig{
		MaxLength:      125,
		IncludeContext: true,
		AvoidPhrases:   []string{"image of", "picture of", "photo of", "a image", "this image shows"},
		LanguageStyle:  "concise",
	}
}

// AltTextRequest contains parameters for alt text generation.
type AltTextRequest struct {
	ImagePath string `json:"image_path"`
	Context   string `json:"context"` // Surrounding context (blog post text, etc.)
	Purpose   string `json:"purpose"` // Purpose of the image (hero, diagram, etc.)
	MaxLength int    `json:"max_length"`
}

// AltTextResult contains the generated alt text and metadata.
type AltTextResult struct {
	AltText     string   `json:"alt_text"`
	Confidence  float64  `json:"confidence"`
	ImageType   string   `json:"image_type"`            // "photo", "illustration", "diagram", etc.
	Suggestions []string `json:"suggestions,omitempty"` // Alternative suggestions
}

// NewAltTextGenerator creates a new alt text generator.
func NewAltTextGenerator(vm *VisionModel) *AltTextGenerator {
	return &AltTextGenerator{
		visionModel: vm,
	}
}

// Generate generates alt text for an image.
func (g *AltTextGenerator) Generate(ctx context.Context, req *AltTextRequest) (*AltTextResult, error) {
	cfg := DefaultAltTextConfig()
	if req.MaxLength > 0 {
		cfg.MaxLength = req.MaxLength
	}

	prompt := g.buildPrompt(req, cfg)

	visionReq := &VisionRequest{
		Images: []ImageInput{
			{Path: req.ImagePath, MimeType: detectMimeType(req.ImagePath)},
		},
		Prompt:      prompt,
		MaxTokens:   200,
		Temperature: 0.3,
	}

	resp, err := g.visionModel.Analyze(ctx, visionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image: %w", err)
	}

	// Clean and validate the alt text
	altText := g.cleanAltText(resp.Text, cfg)

	return &AltTextResult{
		AltText:    altText,
		Confidence: 0.9, // TODO: Calculate actual confidence
		ImageType:  g.detectImageType(resp.Text),
	}, nil
}

// buildPrompt constructs the prompt for alt text generation.
func (g *AltTextGenerator) buildPrompt(req *AltTextRequest, cfg *AltTextConfig) string {
	var sb strings.Builder

	sb.WriteString("Generate a concise, accessible alt text for this image. ")
	sb.WriteString("Requirements:\n")
	sb.WriteString(fmt.Sprintf("- Maximum %d characters\n", cfg.MaxLength))
	sb.WriteString("- Do NOT start with 'image of', 'picture of', 'photo of', or similar phrases\n")
	sb.WriteString("- Focus on the purpose and meaning, not just description\n")
	sb.WriteString("- Be specific and informative\n")
	sb.WriteString("- Write in present tense\n")

	if req.Purpose != "" {
		sb.WriteString(fmt.Sprintf("\nImage purpose: %s\n", req.Purpose))
	}

	if req.Context != "" {
		sb.WriteString(fmt.Sprintf("\nContext (for relevance): %s\n", truncateText(req.Context, 500)))
	}

	sb.WriteString("\nRespond with ONLY the alt text, nothing else. No quotes, no explanations.")

	return sb.String()
}

// cleanAltText cleans and validates the generated alt text.
func (g *AltTextGenerator) cleanAltText(text string, cfg *AltTextConfig) string {
	// Remove common unwanted prefixes
	text = strings.TrimSpace(text)
	text = strings.Trim(text, "\"'")

	lower := strings.ToLower(text)
	for _, phrase := range cfg.AvoidPhrases {
		if strings.HasPrefix(lower, phrase) {
			text = strings.TrimSpace(text[len(phrase):])
			lower = strings.ToLower(text)
		}
	}

	// Capitalize first letter
	if len(text) > 0 {
		text = strings.ToUpper(text[:1]) + text[1:]
	}

	// Truncate if too long
	if len(text) > cfg.MaxLength {
		text = text[:cfg.MaxLength-3] + "..."
	}

	return text
}

// detectImageType determines the type of image from the description.
func (g *AltTextGenerator) detectImageType(description string) string {
	lower := strings.ToLower(description)

	switch {
	case strings.Contains(lower, "diagram") || strings.Contains(lower, "flowchart") || strings.Contains(lower, "chart"):
		return "diagram"
	case strings.Contains(lower, "screenshot"):
		return "screenshot"
	case strings.Contains(lower, "illustration") || strings.Contains(lower, "drawing") || strings.Contains(lower, "artwork"):
		return "illustration"
	case strings.Contains(lower, "photo") || strings.Contains(lower, "photograph"):
		return "photo"
	case strings.Contains(lower, "icon") || strings.Contains(lower, "logo"):
		return "icon"
	case strings.Contains(lower, "graph") || strings.Contains(lower, "visualization"):
		return "visualization"
	default:
		return "image"
	}
}

// GenerateBatch generates alt text for multiple images.
func (g *AltTextGenerator) GenerateBatch(ctx context.Context, requests []*AltTextRequest) ([]*AltTextResult, error) {
	results := make([]*AltTextResult, len(requests))

	for i, req := range requests {
		result, err := g.Generate(ctx, req)
		if err != nil {
			// Store error but continue with other images
			results[i] = &AltTextResult{
				AltText:    fmt.Sprintf("Image %d", i+1),
				Confidence: 0,
			}
			continue
		}
		results[i] = result
	}

	return results, nil
}

// ValidateAltText checks if alt text meets accessibility guidelines.
func (g *AltTextGenerator) ValidateAltText(altText string) []string {
	var issues []string

	if len(altText) == 0 {
		issues = append(issues, "Alt text is empty")
		return issues
	}

	if len(altText) > 125 {
		issues = append(issues, fmt.Sprintf("Alt text too long (%d chars, max 125)", len(altText)))
	}

	lower := strings.ToLower(altText)
	badPrefixes := []string{"image of", "picture of", "photo of", "graphic of"}
	for _, prefix := range badPrefixes {
		if strings.HasPrefix(lower, prefix) {
			issues = append(issues, fmt.Sprintf("Alt text should not start with '%s'", prefix))
		}
	}

	if strings.HasSuffix(altText, ".jpg") || strings.HasSuffix(altText, ".png") {
		issues = append(issues, "Alt text appears to be a filename")
	}

	if strings.Contains(altText, "click here") || strings.Contains(altText, "link to") {
		issues = append(issues, "Alt text should not contain action instructions")
	}

	return issues
}

// ImproveAltText takes existing alt text and improves it.
func (g *AltTextGenerator) ImproveAltText(ctx context.Context, imagePath, existingAltText, context string) (*AltTextResult, error) {
	prompt := fmt.Sprintf(`The current alt text for this image is: "%s"

Improve this alt text following accessibility best practices:
- Maximum 125 characters
- Don't start with "image of" or similar
- Focus on purpose and meaning
- Be specific and informative

Context: %s

Respond with ONLY the improved alt text, nothing else.`, existingAltText, truncateText(context, 300))

	visionReq := &VisionRequest{
		Images: []ImageInput{
			{Path: imagePath, MimeType: detectMimeType(imagePath)},
		},
		Prompt:      prompt,
		MaxTokens:   100,
		Temperature: 0.3,
	}

	resp, err := g.visionModel.Analyze(ctx, visionReq)
	if err != nil {
		return nil, err
	}

	cfg := DefaultAltTextConfig()
	return &AltTextResult{
		AltText:    g.cleanAltText(resp.Text, cfg),
		Confidence: 0.9,
	}, nil
}

// truncateText truncates text to a maximum length.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}
