package vision

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/tools"
	"github.com/soypete/pedrocli/pkg/vision"
)

// GenerateTool provides image generation capabilities.
type GenerateTool struct {
	comfyUI      *vision.ComfyUIClient
	imageStorage *storage.ImageStorage
}

// NewGenerateTool creates a new generate tool.
func NewGenerateTool(comfyUI *vision.ComfyUIClient, imageStorage *storage.ImageStorage) *GenerateTool {
	return &GenerateTool{
		comfyUI:      comfyUI,
		imageStorage: imageStorage,
	}
}

// Name returns the tool name.
func (t *GenerateTool) Name() string {
	return "generate_image"
}

// Description returns the tool description.
func (t *GenerateTool) Description() string {
	return `Generate images using ComfyUI and Stable Diffusion/Flux models.

Actions:
- generate: Generate a new image from a text prompt
- list_workflows: List available workflow templates
- queue_status: Check ComfyUI queue status
- cancel: Cancel a running generation

Arguments for generate:
  action (required): "generate"
  prompt (required): Text description of the image to generate
  negative_prompt (optional): What to avoid in the image
  workflow (optional): Workflow template name (default: "sdxl_base")
  width (optional): Image width in pixels (default: 1024)
  height (optional): Image height in pixels (default: 1024)
  steps (optional): Number of inference steps
  cfg_scale (optional): Classifier-free guidance scale
  seed (optional): Random seed for reproducibility
  model (optional): Model name to use
  job_id (optional): Job ID for storing results

Available workflows: sdxl_base, flux_schnell, flux_dev, blog_hero

Example:
  {"action": "generate", "prompt": "A beautiful mountain landscape at sunset", "workflow": "flux_schnell"}
  {"action": "list_workflows"}`
}

// Execute executes the tool.
func (t *GenerateTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	action, _ := args["action"].(string)
	if action == "" {
		action = "generate"
	}

	switch action {
	case "generate":
		return t.generate(ctx, args)
	case "list_workflows":
		return t.listWorkflows(ctx)
	case "queue_status":
		return t.queueStatus(ctx)
	case "cancel":
		return t.cancel(ctx, args)
	default:
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// generate creates a new image.
func (t *GenerateTool) generate(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return &tools.Result{
			Success: false,
			Error:   "prompt is required",
		}, nil
	}

	// Build generation request
	req := &vision.GenerationRequest{
		PositivePrompt: prompt,
		Workflow:       "sdxl_base",
	}

	if v, ok := args["negative_prompt"].(string); ok {
		req.NegativePrompt = v
	}
	if v, ok := args["workflow"].(string); ok {
		req.Workflow = v
	}
	if v, ok := args["width"].(float64); ok {
		req.Width = int(v)
	}
	if v, ok := args["height"].(float64); ok {
		req.Height = int(v)
	}
	if v, ok := args["steps"].(float64); ok {
		req.Steps = int(v)
	}
	if v, ok := args["cfg_scale"].(float64); ok {
		req.CFGScale = v
	}
	if v, ok := args["seed"].(float64); ok {
		req.Seed = int64(v)
	}
	if v, ok := args["model"].(string); ok {
		req.Model = v
	}
	if v, ok := args["output_prefix"].(string); ok {
		req.OutputPrefix = v
	}

	// Generate the image
	result, err := t.comfyUI.Generate(ctx, req)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("generation failed: %v", err),
		}, nil
	}

	// Download and save generated images if job_id provided
	var savedImages []string
	jobID, _ := args["job_id"].(string)
	if jobID != "" && t.imageStorage != nil {
		for _, img := range result.Images {
			data, err := t.comfyUI.GetImage(ctx, img.Filename, img.Subfolder, img.Type)
			if err != nil {
				continue
			}
			info, err := t.imageStorage.UploadGenerated(jobID, data, img.Filename)
			if err != nil {
				continue
			}
			savedImages = append(savedImages, info.Path)
		}
	}

	output := map[string]interface{}{
		"prompt_id":    result.PromptID,
		"status":       result.Status,
		"seed":         result.Seed,
		"duration_ms":  result.Duration.Milliseconds(),
		"images":       result.Images,
		"saved_images": savedImages,
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success:       true,
		Output:        string(outputJSON),
		ModifiedFiles: savedImages,
	}, nil
}

// listWorkflows returns available workflow templates.
func (t *GenerateTool) listWorkflows(ctx context.Context) (*tools.Result, error) {
	workflows := t.comfyUI.ListWorkflows()

	output := map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// queueStatus returns the ComfyUI queue status.
func (t *GenerateTool) queueStatus(ctx context.Context) (*tools.Result, error) {
	status, err := t.comfyUI.GetQueueStatus(ctx)
	if err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("failed to get queue status: %v", err),
		}, nil
	}

	output := map[string]interface{}{
		"queue_running": status.QueueRunning,
		"queue_pending": status.QueuePending,
	}

	outputJSON, _ := json.MarshalIndent(output, "", "  ")
	return &tools.Result{
		Success: true,
		Output:  string(outputJSON),
	}, nil
}

// cancel cancels a running generation.
func (t *GenerateTool) cancel(ctx context.Context, args map[string]interface{}) (*tools.Result, error) {
	promptID, _ := args["prompt_id"].(string)
	if promptID == "" {
		// Cancel current generation
		if err := t.comfyUI.InterruptGeneration(ctx); err != nil {
			return &tools.Result{
				Success: false,
				Error:   fmt.Sprintf("failed to interrupt: %v", err),
			}, nil
		}
		return &tools.Result{
			Success: true,
			Output:  "Current generation interrupted",
		}, nil
	}

	if err := t.comfyUI.CancelPrompt(ctx, promptID); err != nil {
		return &tools.Result{
			Success: false,
			Error:   fmt.Sprintf("failed to cancel: %v", err),
		}, nil
	}

	return &tools.Result{
		Success: true,
		Output:  fmt.Sprintf("Cancelled prompt %s", promptID),
	}, nil
}

// Ensure GenerateTool implements Tool interface
var _ tools.Tool = (*GenerateTool)(nil)
