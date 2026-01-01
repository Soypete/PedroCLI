// Package vision provides vision and image generation tools for PedroCLI.
package vision

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/soypete/pedrocli/pkg/tools"
	"github.com/soypete/pedrocli/pkg/vision"
)

// AnalyzeTool provides image analysis capabilities.
type AnalyzeTool struct {
	visionModel *vision.VisionModel
}

// NewAnalyzeTool creates a new analyze tool.
func NewAnalyzeTool(vm *vision.VisionModel) *AnalyzeTool {
	return &AnalyzeTool{
		visionModel: vm,
	}
}

// Name returns the tool name.
func (t *AnalyzeTool) Name() string {
	return "analyze_image"
}

// Description returns the tool description.
func (t *AnalyzeTool) Description() string {
	return `Analyze an image using a vision model.

Actions:
- analyze: Analyze an image and return a description
- extract_text: Extract text from an image (OCR)
- suggest_style: Analyze image style for generation
- compare: Compare two images

Arguments:
  action (required): The action to perform (analyze, extract_text, suggest_style, compare)
  image_path (required): Path to the image file
  image_path_2 (optional): Second image path for comparison
  prompt (optional): Custom prompt for analysis
  context (optional): Surrounding context for better analysis

Example:
  {"action": "analyze", "image_path": "/path/to/image.png", "prompt": "Describe this image"}
  {"action": "extract_text", "image_path": "/path/to/screenshot.png"}
  {"action": "suggest_style", "image_path": "/path/to/reference.jpg"}
  {"action": "compare", "image_path": "/path/to/image1.png", "image_path_2": "/path/to/image2.png"}`
}

// Execute executes the tool.
func (t *AnalyzeTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	action, _ := args["action"].(string)
	if action == "" {
		return &tools.Result{
			Success: false,
			Error:   "action is required",
		}, nil
	}

	imagePath, _ := args["image_path"].(string)
	if imagePath == "" {
		return &tools.Result{
			Success: false,
			Error:   "image_path is required",
		}, nil
	}

	switch action {
	case "analyze":
		return t.analyze(ctx, args)
	case "extract_text":
		return t.extractText(ctx, imagePath)
	case "suggest_style":
		return t.suggestStyle(ctx, imagePath)
	case "compare":
		return t.compare(ctx, args)
	default:
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// analyze performs general image analysis.
func (t *AnalyzeTool) analyze(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	imagePath := args["image_path"].(string)
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		prompt = "Describe this image in detail. Include information about the subject, composition, colors, and any notable elements."
	}

	context, _ := args["context"].(string)
	if context != "" {
		prompt = fmt.Sprintf("%s\n\nContext: %s", prompt, context)
	}

	result, err := t.visionModel.AnalyzeImage(ctx, imagePath, prompt)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("analysis failed: %v", err),
		}, nil
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// extractText performs OCR on an image.
func (t *AnalyzeTool) extractText(ctx context.Context, imagePath string) (*tools.Result, error) {
	text, err := t.visionModel.ExtractText(ctx, imagePath)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("text extraction failed: %v", err),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  text,
	}, nil
}

// suggestStyle analyzes image style for generation.
func (t *AnalyzeTool) suggestStyle(ctx context.Context, imagePath string) (*tools.Result, error) {
	suggestion, err := t.visionModel.SuggestImageStyle(ctx, imagePath)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("style analysis failed: %v", err),
		}, nil
	}

	output, _ := json.MarshalIndent(suggestion, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(output),
	}, nil
}

// compare compares two images.
func (t *AnalyzeTool) compare(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	imagePath1 := args["image_path"].(string)
	imagePath2, _ := args["image_path_2"].(string)
	if imagePath2 == "" {
		return &tools.Result{
			Success: false,
			Error:   "image_path_2 is required for comparison",
		}, nil
	}

	prompt, _ := args["prompt"].(string)

	comparison, err := t.visionModel.CompareImages(ctx, imagePath1, imagePath2, prompt)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("comparison failed: %v", err),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  comparison,
	}, nil
}

// Ensure AnalyzeTool implements Tool interface
var _ tools.Tool = (*AnalyzeTool)(nil)
