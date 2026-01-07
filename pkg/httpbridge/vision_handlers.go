// Package httpbridge provides HTTP handlers for vision tools.
// TODO: Vision features are currently disabled - these handlers will be used when vision is re-enabled
//
//nolint:unused // Vision handlers are not yet connected to routes
package httpbridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/soypete/pedrocli/pkg/storage"
	"github.com/soypete/pedrocli/pkg/vision"
)

// VisionRequest represents a vision API request
type VisionRequest struct {
	Action         string `json:"action"`
	ImagePath      string `json:"image_path,omitempty"`
	ImagePath2     string `json:"image_path_2,omitempty"`
	ReferenceImage string `json:"reference_image,omitempty"`
	BlogContent    string `json:"blog_content,omitempty"`
	StylePreset    string `json:"style_preset,omitempty"`
	AspectRatio    string `json:"aspect_ratio,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
	Instructions   string `json:"instructions,omitempty"`
	Context        string `json:"context,omitempty"`
}

// VisionResponse represents a vision API response
type VisionResponse struct {
	Success     bool                   `json:"success"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	ImagePath   string                 `json:"image_path,omitempty"`
	AltText     string                 `json:"alt_text,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// handleVisionPage serves the vision UI page
func (s *Server) handleVisionPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"title": "Vision Tools - PedroCLI",
	}

	if err := s.templates.ExecuteTemplate(w, "vision.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleVision handles vision API requests
func (s *Server) handleVision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVisionError(w, "Invalid request body: "+err.Error())
		return
	}

	// Initialize vision model
	visionCfg := &vision.VisionModelConfig{
		Backend:     "ollama",
		ServerURL:   "http://localhost:11434",
		ModelPath:   "llama3.2-vision",
		Temperature: 0.1,
		MaxTokens:   1024,
	}

	vm, err := vision.NewVisionModel(visionCfg)
	if err != nil {
		writeVisionError(w, "Failed to initialize vision model: "+err.Error())
		return
	}

	// Initialize storage
	storageCfg := &storage.ImageStorageConfig{
		BasePath: s.config.Vision.StoragePath,
	}
	imageStorage, err := storage.NewImageStorage(storageCfg)
	if err != nil {
		writeVisionError(w, "Failed to initialize storage: "+err.Error())
		return
	}

	switch req.Action {
	case "analyze":
		s.handleAnalyzeAction(w, r, vm, &req)
	case "generate_blog_image":
		s.handleGenerateBlogImage(w, r, vm, imageStorage, &req)
	case "generate_with_reference":
		s.handleGenerateWithReference(w, r, vm, imageStorage, &req)
	case "extract_text":
		s.handleExtractText(w, r, vm, &req)
	case "suggest_style":
		s.handleSuggestStyle(w, r, vm, &req)
	default:
		writeVisionError(w, fmt.Sprintf("Unknown action: %s", req.Action))
	}
}

// handleVisionAnalyze handles image analysis requests
func (s *Server) handleVisionAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVisionError(w, "Invalid request body: "+err.Error())
		return
	}

	req.Action = "analyze"

	// Initialize vision model
	visionCfg := &vision.VisionModelConfig{
		Backend:     "ollama",
		ServerURL:   "http://localhost:11434",
		ModelPath:   "llama3.2-vision",
		Temperature: 0.1,
		MaxTokens:   1024,
	}

	vm, err := vision.NewVisionModel(visionCfg)
	if err != nil {
		writeVisionError(w, "Failed to initialize vision model: "+err.Error())
		return
	}

	s.handleAnalyzeAction(w, r, vm, &req)
}

// handleAnalyzeAction analyzes an image
func (s *Server) handleAnalyzeAction(w http.ResponseWriter, r *http.Request, vm *vision.VisionModel, req *VisionRequest) {
	if req.ImagePath == "" {
		writeVisionError(w, "image_path is required")
		return
	}

	// Check if file exists
	if _, err := os.Stat(req.ImagePath); os.IsNotExist(err) {
		// Try relative to images path
		altPath := filepath.Join(s.config.Vision.ImagesPath, filepath.Base(req.ImagePath))
		if _, err := os.Stat(altPath); os.IsNotExist(err) {
			writeVisionError(w, "Image file not found: "+req.ImagePath)
			return
		}
		req.ImagePath = altPath
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	result, err := vm.AnalyzeImage(s.ctx, req.ImagePath, prompt)
	if err != nil {
		writeVisionError(w, "Analysis failed: "+err.Error())
		return
	}

	writeVisionSuccess(w, &VisionResponse{
		Success:     true,
		Description: result.Description,
		Data: map[string]interface{}{
			"objects": result.Objects,
			"colors":  result.Colors,
			"style":   result.Style,
			"mood":    result.Mood,
		},
	})
}

// handleGenerateBlogImage generates a blog image
func (s *Server) handleGenerateBlogImage(w http.ResponseWriter, r *http.Request, vm *vision.VisionModel, imageStorage *storage.ImageStorage, req *VisionRequest) {
	if req.BlogContent == "" {
		writeVisionError(w, "blog_content is required")
		return
	}

	// For now, we'll use the vision model to suggest an image prompt
	// In a full implementation, this would connect to ComfyUI for generation
	suggestPrompt := fmt.Sprintf(`Based on this blog post content, suggest an image prompt for a professional blog hero image:

Blog Content:
%s

Style: %s
Aspect Ratio: %s

Respond with a detailed image generation prompt.`, req.BlogContent, req.StylePreset, req.AspectRatio)

	visionReq := &vision.VisionRequest{
		Prompt:    suggestPrompt,
		MaxTokens: 512,
	}

	// Note: This is a text-only request to the vision model for prompt generation
	// A full implementation would then send this to ComfyUI
	writeVisionSuccess(w, &VisionResponse{
		Success: true,
		Message: "Image generation requires ComfyUI. Generated prompt suggestion:",
		Data: map[string]interface{}{
			"suggested_prompt": visionReq.Prompt,
			"style_preset":     req.StylePreset,
			"aspect_ratio":     req.AspectRatio,
			"note":             "Connect ComfyUI at localhost:8188 for actual image generation",
		},
	})
}

// handleGenerateWithReference generates an image using a reference
func (s *Server) handleGenerateWithReference(w http.ResponseWriter, r *http.Request, vm *vision.VisionModel, imageStorage *storage.ImageStorage, req *VisionRequest) {
	if req.BlogContent == "" {
		writeVisionError(w, "blog_content is required")
		return
	}
	if req.ReferenceImage == "" {
		writeVisionError(w, "reference_image is required")
		return
	}

	// Check if reference image exists
	refPath := req.ReferenceImage
	if _, err := os.Stat(refPath); os.IsNotExist(err) {
		// Try relative to images path
		altPath := filepath.Join(s.config.Vision.ImagesPath, filepath.Base(refPath))
		if _, err := os.Stat(altPath); os.IsNotExist(err) {
			writeVisionError(w, "Reference image not found: "+req.ReferenceImage)
			return
		}
		refPath = altPath
	}

	// Analyze the reference image for style
	styleResult, err := vm.SuggestImageStyle(s.ctx, refPath)
	if err != nil {
		writeVisionError(w, "Failed to analyze reference image: "+err.Error())
		return
	}

	writeVisionSuccess(w, &VisionResponse{
		Success: true,
		Message: "Reference image analyzed. Style suggestions ready for ComfyUI.",
		Data: map[string]interface{}{
			"reference_analysis": styleResult,
			"blog_content":       req.BlogContent[:min(200, len(req.BlogContent))] + "...",
			"instructions":       req.Instructions,
			"note":               "Connect ComfyUI at localhost:8188 for actual image generation",
		},
	})
}

// handleExtractText extracts text from an image (OCR)
func (s *Server) handleExtractText(w http.ResponseWriter, r *http.Request, vm *vision.VisionModel, req *VisionRequest) {
	if req.ImagePath == "" {
		writeVisionError(w, "image_path is required")
		return
	}

	text, err := vm.ExtractText(s.ctx, req.ImagePath)
	if err != nil {
		writeVisionError(w, "Text extraction failed: "+err.Error())
		return
	}

	writeVisionSuccess(w, &VisionResponse{
		Success: true,
		Data: map[string]interface{}{
			"extracted_text": text,
		},
	})
}

// handleSuggestStyle analyzes an image and suggests style parameters
func (s *Server) handleSuggestStyle(w http.ResponseWriter, r *http.Request, vm *vision.VisionModel, req *VisionRequest) {
	if req.ImagePath == "" {
		writeVisionError(w, "image_path is required")
		return
	}

	suggestion, err := vm.SuggestImageStyle(s.ctx, req.ImagePath)
	if err != nil {
		writeVisionError(w, "Style analysis failed: "+err.Error())
		return
	}

	writeVisionSuccess(w, &VisionResponse{
		Success: true,
		Data: map[string]interface{}{
			"style":       suggestion.Style,
			"colors":      suggestion.Colors,
			"lighting":    suggestion.Lighting,
			"composition": suggestion.Composition,
			"texture":     suggestion.Texture,
			"raw":         suggestion.RawDescription,
		},
	})
}

// writeVisionError writes an error response
func writeVisionError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(&VisionResponse{
		Success: false,
		Error:   message,
	})
}

// writeVisionSuccess writes a success response
func writeVisionSuccess(w http.ResponseWriter, resp *VisionResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Unused but may be needed for multipart form handling
var _ = io.ReadAll
