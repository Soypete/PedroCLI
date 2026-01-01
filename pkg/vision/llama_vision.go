package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// VisionModel represents a vision-capable LLM.
type VisionModel struct {
	config     *VisionModelConfig
	httpClient *http.Client
	serverCmd  *exec.Cmd
	serverURL  string
	mu         sync.RWMutex
	ready      bool
}

// VisionModelConfig configures the vision model.
type VisionModelConfig struct {
	// Model files
	ModelPath  string `json:"model_path"`  // Path to the GGUF model file
	MMProjPath string `json:"mmproj_path"` // Path to the multimodal projector file

	// Server configuration
	Backend      string `json:"backend"`       // "llamacpp" or "ollama"
	ServerURL    string `json:"server_url"`    // For remote server or Ollama
	LlamaCppPath string `json:"llamacpp_path"` // Path to llama.cpp directory
	Port         int    `json:"port"`          // Server port (default: 8081)

	// Model parameters
	ContextSize int     `json:"context_size"` // Context window size
	NGPULayers  int     `json:"n_gpu_layers"` // Number of layers to offload to GPU
	Threads     int     `json:"threads"`      // Number of CPU threads
	Temperature float64 `json:"temperature"`  // Generation temperature
	MaxTokens   int     `json:"max_tokens"`   // Maximum tokens to generate

	// Hardware target
	HardwareTarget string `json:"hardware_target"` // "rtx5090", "mac64", etc.
}

// DefaultVisionModelConfig returns default configuration.
func DefaultVisionModelConfig() *VisionModelConfig {
	return &VisionModelConfig{
		Backend:        "llamacpp",
		Port:           8081,
		ContextSize:    4096,
		NGPULayers:     -1, // Offload all layers
		Threads:        8,
		Temperature:    0.1,
		MaxTokens:      512,
		HardwareTarget: "rtx5090",
	}
}

// VisionRequest represents a vision model request.
type VisionRequest struct {
	Images       []ImageInput `json:"images"`
	Prompt       string       `json:"prompt"`
	SystemPrompt string       `json:"system_prompt,omitempty"`
	MaxTokens    int          `json:"max_tokens,omitempty"`
	Temperature  float64      `json:"temperature,omitempty"`
}

// ImageInput represents an input image for vision models.
type ImageInput struct {
	Data     []byte `json:"-"`         // Raw image data
	Base64   string `json:"base64"`    // Base64-encoded image
	Path     string `json:"path"`      // Path to image file
	URL      string `json:"url"`       // URL to image
	MimeType string `json:"mime_type"` // MIME type
}

// VisionResponse represents a vision model response.
type VisionResponse struct {
	Text       string        `json:"text"`
	TokensUsed int           `json:"tokens_used"`
	Duration   time.Duration `json:"duration"`
}

// AnalysisResult represents the result of image analysis.
type AnalysisResult struct {
	Description string                 `json:"description"`
	Objects     []string               `json:"objects,omitempty"`
	Colors      []string               `json:"colors,omitempty"`
	Style       string                 `json:"style,omitempty"`
	Mood        string                 `json:"mood,omitempty"`
	Text        string                 `json:"text,omitempty"` // OCR text if present
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewVisionModel creates a new vision model instance.
func NewVisionModel(cfg *VisionModelConfig) (*VisionModel, error) {
	if cfg == nil {
		cfg = DefaultVisionModelConfig()
	}

	vm := &VisionModel{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}

	// Set server URL based on backend
	switch cfg.Backend {
	case "ollama":
		if cfg.ServerURL == "" {
			cfg.ServerURL = "http://localhost:11434"
		}
		vm.serverURL = cfg.ServerURL
		vm.ready = true // Ollama should already be running
	case "llamacpp":
		if cfg.ServerURL != "" {
			vm.serverURL = cfg.ServerURL
			vm.ready = true
		} else {
			vm.serverURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
		}
	default:
		return nil, fmt.Errorf("unsupported backend: %s", cfg.Backend)
	}

	return vm, nil
}

// Start starts the llama.cpp server if needed.
func (vm *VisionModel) Start(ctx context.Context) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.ready {
		return nil
	}

	if vm.config.Backend != "llamacpp" {
		return nil
	}

	// Build llama-server command
	serverPath := vm.config.LlamaCppPath + "/llama-server"
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		serverPath = vm.config.LlamaCppPath + "/server" // Try older name
	}

	args := []string{
		"-m", vm.config.ModelPath,
		"--mmproj", vm.config.MMProjPath,
		"-c", fmt.Sprintf("%d", vm.config.ContextSize),
		"-ngl", fmt.Sprintf("%d", vm.config.NGPULayers),
		"-t", fmt.Sprintf("%d", vm.config.Threads),
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", vm.config.Port),
	}

	vm.serverCmd = exec.CommandContext(ctx, serverPath, args...)
	vm.serverCmd.Stdout = os.Stdout
	vm.serverCmd.Stderr = os.Stderr

	if err := vm.serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	// Wait for server to be ready
	if err := vm.waitForServer(ctx); err != nil {
		vm.Stop()
		return err
	}

	vm.ready = true
	return nil
}

// waitForServer waits for the server to be ready.
func (vm *VisionModel) waitForServer(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for server to start")
		case <-ticker.C:
			if err := vm.Ping(ctx); err == nil {
				return nil
			}
		}
	}
}

// Stop stops the llama.cpp server.
func (vm *VisionModel) Stop() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.serverCmd != nil && vm.serverCmd.Process != nil {
		_ = vm.serverCmd.Process.Kill()
		_ = vm.serverCmd.Wait() // Error ignored: process was killed
		vm.serverCmd = nil
	}

	vm.ready = false
	return nil
}

// Ping checks if the server is reachable.
func (vm *VisionModel) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", vm.serverURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := vm.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server not healthy: %s", resp.Status)
	}

	return nil
}

// Analyze analyzes an image and returns a description.
func (vm *VisionModel) Analyze(ctx context.Context, req *VisionRequest) (*VisionResponse, error) {
	switch vm.config.Backend {
	case "ollama":
		return vm.analyzeOllama(ctx, req)
	case "llamacpp":
		return vm.analyzeLlamaCpp(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported backend: %s", vm.config.Backend)
	}
}

// analyzeOllama uses Ollama API for vision inference.
func (vm *VisionModel) analyzeOllama(ctx context.Context, req *VisionRequest) (*VisionResponse, error) {
	start := time.Now()

	// Prepare images as base64
	var images []string
	for _, img := range req.Images {
		b64, err := vm.imageToBase64(img)
		if err != nil {
			return nil, fmt.Errorf("failed to encode image: %w", err)
		}
		images = append(images, b64)
	}

	// Build Ollama request
	ollamaReq := map[string]interface{}{
		"model":  vm.config.ModelPath, // For Ollama, this is the model name
		"prompt": req.Prompt,
		"images": images,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": vm.config.Temperature,
			"num_predict": vm.config.MaxTokens,
		},
	}

	if req.SystemPrompt != "" {
		ollamaReq["system"] = req.SystemPrompt
	}
	if req.MaxTokens > 0 {
		ollamaReq["options"].(map[string]interface{})["num_predict"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		ollamaReq["options"].(map[string]interface{})["temperature"] = req.Temperature
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", vm.serverURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := vm.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error: %s - %s", resp.Status, string(respBody))
	}

	var result struct {
		Response  string `json:"response"`
		Context   []int  `json:"context"`
		EvalCount int    `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &VisionResponse{
		Text:       result.Response,
		TokensUsed: result.EvalCount,
		Duration:   time.Since(start),
	}, nil
}

// analyzeLlamaCpp uses llama.cpp server API for vision inference.
func (vm *VisionModel) analyzeLlamaCpp(ctx context.Context, req *VisionRequest) (*VisionResponse, error) {
	start := time.Now()

	// Build content array with images and text
	var content []map[string]interface{}

	for _, img := range req.Images {
		b64, err := vm.imageToBase64(img)
		if err != nil {
			return nil, fmt.Errorf("failed to encode image: %w", err)
		}
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]string{
				"url": "data:" + img.MimeType + ";base64," + b64,
			},
		})
	}

	content = append(content, map[string]interface{}{
		"type": "text",
		"text": req.Prompt,
	})

	// Build OpenAI-compatible request
	messages := []map[string]interface{}{
		{
			"role":    "user",
			"content": content,
		},
	}

	if req.SystemPrompt != "" {
		messages = append([]map[string]interface{}{
			{"role": "system", "content": req.SystemPrompt},
		}, messages...)
	}

	llamaReq := map[string]interface{}{
		"messages":    messages,
		"max_tokens":  vm.config.MaxTokens,
		"temperature": vm.config.Temperature,
	}

	if req.MaxTokens > 0 {
		llamaReq["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		llamaReq["temperature"] = req.Temperature
	}

	body, err := json.Marshal(llamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", vm.serverURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := vm.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llamacpp request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llamacpp error: %s - %s", resp.Status, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from model")
	}

	return &VisionResponse{
		Text:       result.Choices[0].Message.Content,
		TokensUsed: result.Usage.TotalTokens,
		Duration:   time.Since(start),
	}, nil
}

// imageToBase64 converts an ImageInput to base64.
func (vm *VisionModel) imageToBase64(img ImageInput) (string, error) {
	if img.Base64 != "" {
		return img.Base64, nil
	}

	var data []byte
	var err error

	if len(img.Data) > 0 {
		data = img.Data
	} else if img.Path != "" {
		data, err = os.ReadFile(img.Path)
		if err != nil {
			return "", fmt.Errorf("failed to read image file: %w", err)
		}
	} else if img.URL != "" {
		// TODO: Implement URL fetching
		return "", fmt.Errorf("URL fetching not implemented")
	} else {
		return "", fmt.Errorf("no image data provided")
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// AnalyzeImage is a convenience method for analyzing a single image.
func (vm *VisionModel) AnalyzeImage(ctx context.Context, imagePath string, prompt string) (*AnalysisResult, error) {
	req := &VisionRequest{
		Images: []ImageInput{
			{Path: imagePath, MimeType: detectMimeType(imagePath)},
		},
		Prompt: prompt,
	}

	resp, err := vm.Analyze(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse the response into structured format
	return &AnalysisResult{
		Description: resp.Text,
	}, nil
}

// ExtractText performs OCR on an image.
func (vm *VisionModel) ExtractText(ctx context.Context, imagePath string) (string, error) {
	prompt := "Extract and transcribe all text visible in this image. If there is no text, respond with 'No text found.'"

	req := &VisionRequest{
		Images: []ImageInput{
			{Path: imagePath, MimeType: detectMimeType(imagePath)},
		},
		Prompt: prompt,
	}

	resp, err := vm.Analyze(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

// SuggestImageStyle analyzes an image and suggests style parameters for generation.
func (vm *VisionModel) SuggestImageStyle(ctx context.Context, imagePath string) (*StyleSuggestion, error) {
	prompt := `Analyze this image and describe its visual style for use in image generation prompts.
Include:
1. Art style (e.g., photorealistic, illustrated, minimalist, etc.)
2. Color palette (dominant colors and mood)
3. Lighting (soft, dramatic, natural, etc.)
4. Composition elements
5. Texture and detail level

Format as JSON with fields: style, colors, lighting, composition, texture`

	req := &VisionRequest{
		Images: []ImageInput{
			{Path: imagePath, MimeType: detectMimeType(imagePath)},
		},
		Prompt: prompt,
	}

	resp, err := vm.Analyze(ctx, req)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON, fall back to raw text
	var suggestion StyleSuggestion
	if err := json.Unmarshal([]byte(resp.Text), &suggestion); err != nil {
		// Parse from raw text
		suggestion.RawDescription = resp.Text
	}

	return &suggestion, nil
}

// StyleSuggestion contains style analysis results.
type StyleSuggestion struct {
	Style          string   `json:"style"`
	Colors         []string `json:"colors"`
	Lighting       string   `json:"lighting"`
	Composition    string   `json:"composition"`
	Texture        string   `json:"texture"`
	RawDescription string   `json:"raw_description,omitempty"`
}

// CompareImages compares two images and describes differences.
func (vm *VisionModel) CompareImages(ctx context.Context, image1Path, image2Path, prompt string) (string, error) {
	if prompt == "" {
		prompt = "Compare these two images and describe the key differences and similarities."
	}

	req := &VisionRequest{
		Images: []ImageInput{
			{Path: image1Path, MimeType: detectMimeType(image1Path)},
			{Path: image2Path, MimeType: detectMimeType(image2Path)},
		},
		Prompt: prompt,
	}

	resp, err := vm.Analyze(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

// detectMimeType detects MIME type from file extension.
func detectMimeType(path string) string {
	ext := strings.ToLower(path)
	switch {
	case strings.HasSuffix(ext, ".jpg"), strings.HasSuffix(ext, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(ext, ".png"):
		return "image/png"
	case strings.HasSuffix(ext, ".gif"):
		return "image/gif"
	case strings.HasSuffix(ext, ".webp"):
		return "image/webp"
	case strings.HasSuffix(ext, ".bmp"):
		return "image/bmp"
	default:
		return "image/png"
	}
}
