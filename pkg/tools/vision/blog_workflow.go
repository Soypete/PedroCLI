package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/tools"
	"github.com/soypete/pedrocli/pkg/vision"
	"github.com/google/uuid"
)

// BlogWorkflowTool provides the blog post image generation workflow.
type BlogWorkflowTool struct {
	visionModel   *vision.VisionModel
	comfyUI       *vision.ComfyUIClient
	altTextGen    *vision.AltTextGenerator
	imageStorage  *storage.ImageStorage
}

// NewBlogWorkflowTool creates a new blog workflow tool.
func NewBlogWorkflowTool(
	vm *vision.VisionModel,
	comfyUI *vision.ComfyUIClient,
	imageStorage *storage.ImageStorage,
) *BlogWorkflowTool {
	return &BlogWorkflowTool{
		visionModel:  vm,
		comfyUI:      comfyUI,
		altTextGen:   vision.NewAltTextGenerator(vm),
		imageStorage: imageStorage,
	}
}

// Name returns the tool name.
func (t *BlogWorkflowTool) Name() string {
	return "blog_image"
}

// Description returns the tool description.
func (t *BlogWorkflowTool) Description() string {
	return `Generate images for blog posts with automatic alt text.

Actions:
- generate_blog_image: Generate a hero image for a blog post
- generate_with_reference: Generate an image using a reference image for style
- regenerate: Regenerate an image with modified parameters
- generate_alt_text: Generate alt text for an existing image
- batch_generate: Generate multiple images for a blog post

Arguments for generate_blog_image:
  action (required): "generate_blog_image"
  blog_content (required): The blog post text content
  style_preset (optional): Style preset (blog_hero, technical_diagram, code_visualization, social_preview, newsletter_header)
  instructions (optional): Additional generation instructions
  aspect_ratio (optional): Aspect ratio (16:9, 21:9, 4:3, 1:1)

Arguments for generate_with_reference:
  action (required): "generate_with_reference"
  blog_content (required): The blog post text content
  reference_image (required): Path to reference image
  instructions (optional): Additional generation instructions

Arguments for generate_alt_text:
  action (required): "generate_alt_text"
  image_path (required): Path to the image
  context (optional): Context for alt text generation

Example:
  {"action": "generate_blog_image", "blog_content": "This article explores...", "style_preset": "blog_hero"}
  {"action": "generate_with_reference", "blog_content": "...", "reference_image": "/path/to/ref.jpg"}`
}

// Execute executes the tool.
func (t *BlogWorkflowTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return &tools.Result{
			Success: false,
			Error:   "action is required",
		}, nil
	}

	switch action {
	case "generate_blog_image":
		return t.generateBlogImage(ctx, args)
	case "generate_with_reference":
		return t.generateWithReference(ctx, args)
	case "regenerate":
		return t.regenerate(ctx, args)
	case "generate_alt_text":
		return t.generateAltText(ctx, args)
	case "batch_generate":
		return t.batchGenerate(ctx, args)
	default:
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// generateBlogImage creates a hero image for a blog post.
func (t *BlogWorkflowTool) generateBlogImage(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	blogContent, _ := args["blog_content"].(string)
	if blogContent == "" {
		return &tools.Result{
			Success: false,
			Error:   "blog_content is required",
		}, nil
	}

	stylePreset, _ := args["style_preset"].(string)
	if stylePreset == "" {
		stylePreset = "blog_hero"
	}

	instructions, _ := args["instructions"].(string)
	aspectRatio, _ := args["aspect_ratio"].(string)

	// Generate prompt from blog content
	imagePrompt := t.generatePromptFromBlog(ctx, blogContent, stylePreset, instructions)

	// Get dimensions for aspect ratio
	width, height := t.getDimensionsForAspectRatio(aspectRatio, stylePreset)

	// Create job ID
	jobID := uuid.New().String()

	// Generate the image
	genReq := &vision.GenerationRequest{
		PositivePrompt: imagePrompt.Prompt,
		NegativePrompt: imagePrompt.NegativePrompt,
		Workflow:       t.getWorkflowForPreset(stylePreset),
		Width:          width,
		Height:         height,
		OutputPrefix:   "blog_" + jobID[:8],
	}

	result, err := t.comfyUI.Generate(ctx, genReq)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("image generation failed: %v", err),
		}, nil
	}

	// Download and save generated images
	var savedImages []storage.ImageInfo
	for _, img := range result.Images {
		data, err := t.comfyUI.GetImage(ctx, img.Filename, img.Subfolder, img.Type)
		if err != nil {
			continue
		}
		info, err := t.imageStorage.UploadGenerated(jobID, data, img.Filename)
		if err != nil {
			continue
		}
		savedImages = append(savedImages, *info)
	}

	// Generate alt text for the first image
	var altText string
	if len(savedImages) > 0 {
		altResult, err := t.altTextGen.Generate(ctx, &vision.AltTextRequest{
			ImagePath: savedImages[0].Path,
			Context:   truncateBlogContent(blogContent, 500),
			Purpose:   stylePreset,
		})
		if err == nil {
			altText = altResult.AltText
		}
	}

	output := map[string]interface{}{
		"job_id":        jobID,
		"prompt_id":     result.PromptID,
		"prompt_used":   imagePrompt.Prompt,
		"status":        result.Status,
		"seed":          result.Seed,
		"images":        savedImages,
		"alt_text":      altText,
		"style_preset":  stylePreset,
		"duration_ms":   result.Duration.Milliseconds(),
	}

	var modifiedFiles []string
	for _, img := range savedImages {
		modifiedFiles = append(modifiedFiles, img.Path)
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success:       true,
		Output:        string(outputJSON),
		ModifiedFiles: modifiedFiles,
	}, nil
}

// generateWithReference generates an image using a reference for style.
func (t *BlogWorkflowTool) generateWithReference(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	blogContent, _ := args["blog_content"].(string)
	if blogContent == "" {
		return &tools.Result{
			Success: false,
			Error:   "blog_content is required",
		}, nil
	}

	referencePath, _ := args["reference_image"].(string)
	if referencePath == "" {
		return &tools.Result{
			Success: false,
			Error:   "reference_image is required",
		}, nil
	}

	instructions, _ := args["instructions"].(string)

	// Analyze reference image for style
	styleSuggestion, err := t.visionModel.SuggestImageStyle(ctx, referencePath)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("failed to analyze reference image: %v", err),
		}, nil
	}

	// Build style-informed prompt
	styleDesc := styleSuggestion.RawDescription
	if styleDesc == "" {
		styleDesc = fmt.Sprintf("Style: %s, Lighting: %s, Colors: %v",
			styleSuggestion.Style, styleSuggestion.Lighting, styleSuggestion.Colors)
	}

	// Generate prompt incorporating style
	combinedInstructions := fmt.Sprintf("Match this visual style: %s\n\nAdditional instructions: %s",
		styleDesc, instructions)

	imagePrompt := t.generatePromptFromBlog(ctx, blogContent, "blog_hero", combinedInstructions)

	// Create job ID
	jobID := uuid.New().String()

	// Upload reference image to ComfyUI
	refData, _, err := t.imageStorage.PreprocessImage(referencePath, 1024, 1024)
	if err == nil {
		t.comfyUI.UploadImage(ctx, refData, "reference_"+jobID[:8]+".png")
	}

	// Generate the image
	genReq := &vision.GenerationRequest{
		PositivePrompt: imagePrompt.Prompt,
		NegativePrompt: imagePrompt.NegativePrompt,
		Workflow:       "sdxl_base",
		Width:          1536,
		Height:         864,
		OutputPrefix:   "blog_ref_" + jobID[:8],
	}

	result, err := t.comfyUI.Generate(ctx, genReq)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("image generation failed: %v", err),
		}, nil
	}

	// Save generated images
	var savedImages []storage.ImageInfo
	for _, img := range result.Images {
		data, err := t.comfyUI.GetImage(ctx, img.Filename, img.Subfolder, img.Type)
		if err != nil {
			continue
		}
		info, err := t.imageStorage.UploadGenerated(jobID, data, img.Filename)
		if err != nil {
			continue
		}
		savedImages = append(savedImages, *info)
	}

	// Generate alt text
	var altText string
	if len(savedImages) > 0 {
		altResult, err := t.altTextGen.Generate(ctx, &vision.AltTextRequest{
			ImagePath: savedImages[0].Path,
			Context:   truncateBlogContent(blogContent, 500),
			Purpose:   "blog_hero",
		})
		if err == nil {
			altText = altResult.AltText
		}
	}

	output := map[string]interface{}{
		"job_id":           jobID,
		"prompt_id":        result.PromptID,
		"prompt_used":      imagePrompt.Prompt,
		"reference_style":  styleSuggestion,
		"status":           result.Status,
		"seed":             result.Seed,
		"images":           savedImages,
		"alt_text":         altText,
		"duration_ms":      result.Duration.Milliseconds(),
	}

	var modifiedFiles []string
	for _, img := range savedImages {
		modifiedFiles = append(modifiedFiles, img.Path)
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success:       true,
		Output:        string(outputJSON),
		ModifiedFiles: modifiedFiles,
	}, nil
}

// regenerate regenerates an image with modified parameters.
func (t *BlogWorkflowTool) regenerate(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return &tools.Result{
			Success: false,
			Error:   "prompt is required for regeneration",
		}, nil
	}

	jobID := uuid.New().String()

	genReq := &vision.GenerationRequest{
		PositivePrompt: prompt,
		Workflow:       "sdxl_base",
		Width:          1536,
		Height:         864,
		OutputPrefix:   "regen_" + jobID[:8],
	}

	if v, _ := args["negative_prompt"].(string); v != "" {
		genReq.NegativePrompt = v
	}
	if v, _ := args["seed"].(float64); v > 0 {
		genReq.Seed = int64(v)
	}
	if v, _ := args["workflow"].(string); v != "" {
		genReq.Workflow = v
	}

	result, err := t.comfyUI.Generate(ctx, genReq)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("regeneration failed: %v", err),
		}, nil
	}

	output := map[string]interface{}{
		"job_id":      jobID,
		"prompt_id":   result.PromptID,
		"status":      result.Status,
		"seed":        result.Seed,
		"images":      result.Images,
		"duration_ms": result.Duration.Milliseconds(),
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// generateAltText generates alt text for an existing image.
func (t *BlogWorkflowTool) generateAltText(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	imagePath, _ := args["image_path"].(string)
	if imagePath == "" {
		return &tools.Result{
			Success: false,
			Error:   "image_path is required",
		}, nil
	}

	context, _ := args["context"].(string)
	purpose, _ := args["purpose"].(string)

	result, err := t.altTextGen.Generate(ctx, &vision.AltTextRequest{
		ImagePath: imagePath,
		Context:   context,
		Purpose:   purpose,
	})
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("alt text generation failed: %v", err),
		}, nil
	}

	// Validate the alt text
	issues := t.altTextGen.ValidateAltText(result.AltText)

	output := map[string]interface{}{
		"alt_text":   result.AltText,
		"image_type": result.ImageType,
		"confidence": result.Confidence,
		"issues":     issues,
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// batchGenerate generates multiple images for a blog post.
func (t *BlogWorkflowTool) batchGenerate(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	blogContent, _ := args["blog_content"].(string)
	if blogContent == "" {
		return &tools.Result{
			Success: false,
			Error:   "blog_content is required",
		}, nil
	}

	countF, _ := args["count"].(float64)
	count := int(countF)
	if count <= 0 {
		count = 3 // Default to 3 variations
	}
	if count > 5 {
		count = 5 // Max 5 at a time
	}

	var results []map[string]interface{}
	for i := 0; i < count; i++ {
		result, err := t.generateBlogImage(ctx, args)
		if err != nil {
			continue
		}
		var parsed map[string]interface{}
		json.Unmarshal([]byte(result.Output), &parsed)
		results = append(results, parsed)
	}

	output := map[string]interface{}{
		"count":   len(results),
		"results": results,
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// ImagePrompt contains the generated prompt for image generation.
type ImagePrompt struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
}

// generatePromptFromBlog creates an image generation prompt from blog content.
func (t *BlogWorkflowTool) generatePromptFromBlog(ctx context.Context, blogContent, stylePreset, instructions string) *ImagePrompt {
	// Use vision model to generate a prompt from blog content
	analysisPrompt := fmt.Sprintf(`Based on this blog post content, generate a single image generation prompt for a %s image.

Blog content:
%s

%s

Generate a concise, descriptive prompt (max 200 words) that captures the main theme and mood.
The prompt should be suitable for Stable Diffusion/Flux image generation.
Include style, composition, and quality keywords.

Respond with ONLY the image prompt, nothing else.`, stylePreset, truncateBlogContent(blogContent, 1000), instructions)

	req := &vision.VisionRequest{
		Prompt:      analysisPrompt,
		MaxTokens:   300,
		Temperature: 0.7,
	}

	// If no vision model available, generate a simple prompt
	if t.visionModel == nil {
		return t.generateSimplePrompt(blogContent, stylePreset)
	}

	resp, err := t.visionModel.Analyze(ctx, req)
	if err != nil {
		return t.generateSimplePrompt(blogContent, stylePreset)
	}

	return &ImagePrompt{
		Prompt:         t.enhancePrompt(resp.Text, stylePreset),
		NegativePrompt: t.getNegativePrompt(stylePreset),
	}
}

// generateSimplePrompt creates a basic prompt without LLM.
func (t *BlogWorkflowTool) generateSimplePrompt(blogContent, stylePreset string) *ImagePrompt {
	// Extract first sentence or title-like content
	lines := strings.Split(blogContent, "\n")
	var title string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 && len(line) < 200 {
			title = line
			break
		}
	}

	styleKeywords := map[string]string{
		"blog_hero":           "professional blog header image, clean modern design, high quality",
		"technical_diagram":   "technical diagram, clean lines, professional illustration",
		"code_visualization":  "code concept art, digital technology, abstract visualization",
		"social_preview":      "social media preview image, bold colors, eye-catching design",
		"newsletter_header":   "newsletter header image, minimal design, professional",
	}

	style := styleKeywords[stylePreset]
	if style == "" {
		style = styleKeywords["blog_hero"]
	}

	return &ImagePrompt{
		Prompt:         fmt.Sprintf("%s, %s", title, style),
		NegativePrompt: t.getNegativePrompt(stylePreset),
	}
}

// enhancePrompt adds style keywords to a prompt.
func (t *BlogWorkflowTool) enhancePrompt(prompt, stylePreset string) string {
	suffixes := map[string]string{
		"blog_hero":           ", professional photography, high quality, sharp focus, clean composition",
		"technical_diagram":   ", clean vector style, professional diagram, clear layout",
		"code_visualization":  ", digital art, technology theme, abstract patterns",
		"social_preview":      ", bold and vibrant, high contrast, eye-catching",
		"newsletter_header":   ", minimal and clean, professional design, modern aesthetic",
	}

	suffix := suffixes[stylePreset]
	if suffix == "" {
		suffix = suffixes["blog_hero"]
	}

	return strings.TrimSpace(prompt) + suffix
}

// getNegativePrompt returns the negative prompt for a style preset.
func (t *BlogWorkflowTool) getNegativePrompt(stylePreset string) string {
	return "low quality, blurry, distorted, text, watermark, logo, amateur, bad composition, ugly, deformed"
}

// getWorkflowForPreset returns the workflow template for a style preset.
func (t *BlogWorkflowTool) getWorkflowForPreset(stylePreset string) string {
	workflows := map[string]string{
		"blog_hero":           "blog_hero",
		"technical_diagram":   "sdxl_base",
		"code_visualization":  "sdxl_base",
		"social_preview":      "sdxl_base",
		"newsletter_header":   "sdxl_base",
	}

	workflow := workflows[stylePreset]
	if workflow == "" {
		return "sdxl_base"
	}
	return workflow
}

// getDimensionsForAspectRatio returns width and height for an aspect ratio.
func (t *BlogWorkflowTool) getDimensionsForAspectRatio(aspectRatio, stylePreset string) (int, int) {
	ratios := map[string][2]int{
		"16:9": {1536, 864},
		"21:9": {1792, 768},
		"4:3":  {1280, 960},
		"1:1":  {1024, 1024},
		"3:2":  {1344, 896},
	}

	if dims, ok := ratios[aspectRatio]; ok {
		return dims[0], dims[1]
	}

	// Default dimensions based on preset
	switch stylePreset {
	case "social_preview":
		return 1200, 630 // OG image size
	case "newsletter_header":
		return 1200, 400
	default:
		return 1536, 864 // 16:9
	}
}

// truncateBlogContent truncates blog content to a maximum length.
func truncateBlogContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen-3] + "..."
}

// Ensure BlogWorkflowTool implements Tool interface
var _ tools.Tool = (*BlogWorkflowTool)(nil)
