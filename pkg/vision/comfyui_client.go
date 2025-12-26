// Package vision provides image generation and vision model integration.
package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ComfyUIClient provides an interface to the ComfyUI API.
type ComfyUIClient struct {
	baseURL     string
	httpClient  *http.Client
	clientID    string
	workflows   map[string]*Workflow
	mu          sync.RWMutex
	workflowDir string
}

// ComfyUIConfig configures the ComfyUI client.
type ComfyUIConfig struct {
	BaseURL     string `json:"base_url"`
	Timeout     int    `json:"timeout_seconds"`
	WorkflowDir string `json:"workflow_dir"`
}

// DefaultComfyUIConfig returns default ComfyUI configuration.
func DefaultComfyUIConfig() *ComfyUIConfig {
	return &ComfyUIConfig{
		BaseURL:     "http://localhost:8188",
		Timeout:     300,
		WorkflowDir: "./comfyui/workflows",
	}
}

// Workflow represents a ComfyUI workflow template.
type Workflow struct {
	Name        string                 `json:"name"`
	Description string                 `json:"_description"`
	Version     string                 `json:"version"`
	Nodes       map[string]interface{} `json:"nodes"`
	Defaults    map[string]interface{} `json:"defaults"`
	Extends     string                 `json:"extends,omitempty"`
}

// GenerationRequest represents an image generation request.
type GenerationRequest struct {
	Workflow       string                 `json:"workflow"`
	PositivePrompt string                 `json:"positive_prompt"`
	NegativePrompt string                 `json:"negative_prompt,omitempty"`
	Width          int                    `json:"width,omitempty"`
	Height         int                    `json:"height,omitempty"`
	Seed           int64                  `json:"seed,omitempty"`
	Steps          int                    `json:"steps,omitempty"`
	CFGScale       float64                `json:"cfg_scale,omitempty"`
	Sampler        string                 `json:"sampler,omitempty"`
	Scheduler      string                 `json:"scheduler,omitempty"`
	Model          string                 `json:"model,omitempty"`
	OutputPrefix   string                 `json:"output_prefix,omitempty"`
	ExtraParams    map[string]interface{} `json:"extra_params,omitempty"`
}

// GenerationResult represents the result of image generation.
type GenerationResult struct {
	PromptID   string            `json:"prompt_id"`
	Images     []GeneratedImage  `json:"images"`
	Seed       int64             `json:"seed"`
	Duration   time.Duration     `json:"duration"`
	Status     string            `json:"status"`
	Error      string            `json:"error,omitempty"`
}

// GeneratedImage represents a generated image.
type GeneratedImage struct {
	Filename string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

// QueueStatus represents the ComfyUI queue status.
type QueueStatus struct {
	QueueRunning  int `json:"queue_running"`
	QueuePending  int `json:"queue_pending"`
}

// SystemStats represents ComfyUI system statistics.
type SystemStats struct {
	Devices []DeviceStats `json:"devices"`
}

// DeviceStats represents GPU device statistics.
type DeviceStats struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	VRAMTotal  int64   `json:"vram_total"`
	VRAMFree   int64   `json:"vram_free"`
	TorchVRAM  int64   `json:"torch_vram_total"`
}

// NewComfyUIClient creates a new ComfyUI client.
func NewComfyUIClient(cfg *ComfyUIConfig) (*ComfyUIClient, error) {
	if cfg == nil {
		cfg = DefaultComfyUIConfig()
	}

	client := &ComfyUIClient{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		clientID:    uuid.New().String(),
		workflows:   make(map[string]*Workflow),
		workflowDir: cfg.WorkflowDir,
	}

	// Load workflows from directory
	if err := client.loadWorkflows(); err != nil {
		return nil, fmt.Errorf("failed to load workflows: %w", err)
	}

	return client, nil
}

// loadWorkflows loads all workflow templates from the workflow directory.
func (c *ComfyUIClient) loadWorkflows() error {
	if c.workflowDir == "" {
		return nil
	}

	entries, err := os.ReadDir(c.workflowDir)
	if os.IsNotExist(err) {
		return nil // No workflows directory is okay
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(c.workflowDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var workflow Workflow
		if err := json.Unmarshal(data, &workflow); err != nil {
			continue // Skip invalid JSON
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		if workflow.Name == "" {
			workflow.Name = name
		}
		c.workflows[name] = &workflow
	}

	return nil
}

// GetWorkflow returns a workflow by name.
func (c *ComfyUIClient) GetWorkflow(name string) (*Workflow, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	workflow, ok := c.workflows[name]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", name)
	}
	return workflow, nil
}

// ListWorkflows returns all available workflows.
func (c *ComfyUIClient) ListWorkflows() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var names []string
	for name := range c.workflows {
		names = append(names, name)
	}
	return names
}

// Generate generates an image using the specified workflow.
func (c *ComfyUIClient) Generate(ctx context.Context, req *GenerationRequest) (*GenerationResult, error) {
	start := time.Now()

	// Get workflow
	workflow, err := c.GetWorkflow(req.Workflow)
	if err != nil {
		return nil, err
	}

	// Apply defaults and request parameters
	params := c.buildParams(workflow, req)

	// Build the prompt API payload
	prompt := c.buildPrompt(workflow, params)

	// Queue the prompt
	promptID, err := c.queuePrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to queue prompt: %w", err)
	}

	// Wait for completion
	images, err := c.waitForCompletion(ctx, promptID)
	if err != nil {
		return &GenerationResult{
			PromptID: promptID,
			Status:   "failed",
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	return &GenerationResult{
		PromptID: promptID,
		Images:   images,
		Seed:     params["seed"].(int64),
		Duration: time.Since(start),
		Status:   "completed",
	}, nil
}

// buildParams merges workflow defaults with request parameters.
func (c *ComfyUIClient) buildParams(workflow *Workflow, req *GenerationRequest) map[string]interface{} {
	params := make(map[string]interface{})

	// Start with defaults
	for k, v := range workflow.Defaults {
		params[k] = v
	}

	// Override with request values
	params["positive_prompt"] = req.PositivePrompt
	if req.NegativePrompt != "" {
		params["negative_prompt"] = req.NegativePrompt
	}
	if req.Width > 0 {
		params["width"] = req.Width
	}
	if req.Height > 0 {
		params["height"] = req.Height
	}
	if req.Steps > 0 {
		params["steps"] = req.Steps
	}
	if req.CFGScale > 0 {
		params["cfg_scale"] = req.CFGScale
	}
	if req.Sampler != "" {
		params["sampler"] = req.Sampler
	}
	if req.Scheduler != "" {
		params["scheduler"] = req.Scheduler
	}
	if req.Model != "" {
		params["model_name"] = req.Model
	}
	if req.OutputPrefix != "" {
		params["output_prefix"] = req.OutputPrefix
	}

	// Generate seed if not provided
	if req.Seed <= 0 {
		params["seed"] = rand.Int63()
	} else {
		params["seed"] = req.Seed
	}

	// Merge extra params
	for k, v := range req.ExtraParams {
		params[k] = v
	}

	return params
}

// buildPrompt builds the ComfyUI prompt API payload from a workflow and params.
func (c *ComfyUIClient) buildPrompt(workflow *Workflow, params map[string]interface{}) map[string]interface{} {
	// Deep copy nodes and substitute parameters
	nodes := make(map[string]interface{})
	nodesJSON, _ := json.Marshal(workflow.Nodes)
	nodesStr := string(nodesJSON)

	// Replace template placeholders
	for key, value := range params {
		placeholder := fmt.Sprintf("{{%s}}", key)
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = v
		case int, int64, float64:
			valueStr = fmt.Sprintf("%v", v)
		default:
			jsonValue, _ := json.Marshal(v)
			valueStr = string(jsonValue)
		}
		nodesStr = strings.ReplaceAll(nodesStr, placeholder, valueStr)
	}

	json.Unmarshal([]byte(nodesStr), &nodes)

	return map[string]interface{}{
		"prompt":    nodes,
		"client_id": c.clientID,
	}
}

// queuePrompt sends a prompt to the ComfyUI queue.
func (c *ComfyUIClient) queuePrompt(ctx context.Context, prompt map[string]interface{}) (string, error) {
	data, err := json.Marshal(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to marshal prompt: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/prompt", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to queue prompt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("queue prompt failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.PromptID, nil
}

// waitForCompletion waits for a prompt to complete using WebSocket.
func (c *ComfyUIClient) waitForCompletion(ctx context.Context, promptID string) ([]GeneratedImage, error) {
	wsURL := strings.Replace(c.baseURL, "http", "ws", 1) + "/ws?clientId=" + c.clientID

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		// Fall back to polling if WebSocket fails
		return c.pollForCompletion(ctx, promptID)
	}
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("websocket read error: %w", err)
		}

		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "executing":
			var data struct {
				PromptID string `json:"prompt_id"`
				Node     string `json:"node"`
			}
			json.Unmarshal(msg.Data, &data)
			if data.PromptID == promptID && data.Node == "" {
				// Execution complete, fetch results
				return c.getHistoryImages(ctx, promptID)
			}
		case "execution_error":
			var data struct {
				PromptID string `json:"prompt_id"`
				Message  string `json:"message"`
			}
			json.Unmarshal(msg.Data, &data)
			if data.PromptID == promptID {
				return nil, fmt.Errorf("execution error: %s", data.Message)
			}
		}
	}
}

// pollForCompletion polls the history API for completion.
func (c *ComfyUIClient) pollForCompletion(ctx context.Context, promptID string) ([]GeneratedImage, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			images, completed, err := c.checkHistory(ctx, promptID)
			if err != nil {
				continue // Retry on error
			}
			if completed {
				return images, nil
			}
		}
	}
}

// checkHistory checks the history API for a completed prompt.
func (c *ComfyUIClient) checkHistory(ctx context.Context, promptID string) ([]GeneratedImage, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/history/"+promptID, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, nil // Not ready yet
	}

	var history map[string]struct {
		Outputs map[string]struct {
			Images []GeneratedImage `json:"images"`
		} `json:"outputs"`
		Status struct {
			Completed   bool   `json:"completed"`
			StatusStr   string `json:"status_str"`
		} `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, false, err
	}

	entry, ok := history[promptID]
	if !ok {
		return nil, false, nil
	}

	if !entry.Status.Completed {
		return nil, false, nil
	}

	var images []GeneratedImage
	for _, output := range entry.Outputs {
		images = append(images, output.Images...)
	}

	return images, true, nil
}

// getHistoryImages fetches images from history for a completed prompt.
func (c *ComfyUIClient) getHistoryImages(ctx context.Context, promptID string) ([]GeneratedImage, error) {
	images, completed, err := c.checkHistory(ctx, promptID)
	if err != nil {
		return nil, err
	}
	if !completed {
		return nil, fmt.Errorf("prompt not completed")
	}
	return images, nil
}

// GetImage downloads an image from ComfyUI.
func (c *ComfyUIClient) GetImage(ctx context.Context, filename, subfolder, imageType string) ([]byte, error) {
	url := fmt.Sprintf("%s/view?filename=%s&subfolder=%s&type=%s", c.baseURL, filename, subfolder, imageType)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get image: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// UploadImage uploads an image to ComfyUI input folder.
func (c *ComfyUIClient) UploadImage(ctx context.Context, data []byte, filename string) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}

	// Add subfolder and type fields
	writer.WriteField("subfolder", "")
	writer.WriteField("type", "input")
	writer.WriteField("overwrite", "true")

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/upload/image", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Name, nil
}

// GetQueueStatus returns the current queue status.
func (c *ComfyUIClient) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/queue", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		QueueRunning []interface{} `json:"queue_running"`
		QueuePending []interface{} `json:"queue_pending"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &QueueStatus{
		QueueRunning: len(result.QueueRunning),
		QueuePending: len(result.QueuePending),
	}, nil
}

// GetSystemStats returns system statistics including GPU info.
func (c *ComfyUIClient) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/system_stats", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		System struct {
			Devices []DeviceStats `json:"devices"`
		} `json:"system"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &SystemStats{
		Devices: result.System.Devices,
	}, nil
}

// CancelPrompt cancels a queued or running prompt.
func (c *ComfyUIClient) CancelPrompt(ctx context.Context, promptID string) error {
	data := map[string]interface{}{
		"delete": []string{promptID},
	}
	body, _ := json.Marshal(data)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/queue", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// InterruptGeneration interrupts the current generation.
func (c *ComfyUIClient) InterruptGeneration(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/interrupt", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Ping checks if ComfyUI is reachable.
func (c *ComfyUIClient) Ping(ctx context.Context) error {
	_, err := c.GetSystemStats(ctx)
	return err
}
