package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/soypete/pedrocli/pkg/config"
)

// CalendarTool provides access to Google Calendar via MCP server
type CalendarTool struct {
	config       *config.Config
	tokenManager TokenManager
	mu           sync.Mutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       *bufio.Reader
	msgID        int
	started      bool
}

// NewCalendarTool creates a new Calendar tool
func NewCalendarTool(cfg *config.Config, tokenMgr TokenManager) *CalendarTool {
	return &CalendarTool{
		config:       cfg,
		tokenManager: tokenMgr,
		msgID:        0,
	}
}

// Name returns the tool name
func (t *CalendarTool) Name() string {
	return "calendar"
}

// Description returns the tool description
func (t *CalendarTool) Description() string {
	return `Google Calendar management via MCP server.

Actions:
- list_events: List upcoming events
  Args: calendar_id (optional, uses default), time_min (optional), time_max (optional), max_results (optional int)

- create_event: Create a new calendar event
  Args: summary (string), start_time (ISO8601), end_time (ISO8601), description (optional), location (optional), attendees (optional array of emails)

- update_event: Update an existing event
  Args: event_id (string), summary (optional), start_time (optional), end_time (optional), description (optional), location (optional)

- delete_event: Delete an event
  Args: event_id (string)

- get_event: Get event details
  Args: event_id (string)

- check_availability: Check free/busy times
  Args: time_min (ISO8601), time_max (ISO8601), calendar_ids (optional array)

Example:
{"tool": "calendar", "args": {"action": "create_event", "summary": "[Recording] Episode 42", "start_time": "2024-01-15T14:00:00Z", "end_time": "2024-01-15T15:30:00Z", "description": "Recording session for episode 42"}}`
}

// Execute executes a Calendar action
func (t *CalendarTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Check if Calendar is enabled
	if !t.config.Podcast.Calendar.Enabled {
		return &Result{
			Success: false,
			Error:   "Google Calendar integration is not enabled. Set podcast.calendar.enabled=true in config.",
		}, nil
	}

	// Note: Credentials validation happens in ensureStarted() via TokenManager
	// Don't check config.Podcast.Calendar.CredentialsPath here - tokens may be retrieved from token storage

	action, ok := args["action"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "action is required",
		}, nil
	}

	switch action {
	case "list_events":
		return t.listEvents(ctx, args)
	case "create_event":
		return t.createEvent(ctx, args)
	case "update_event":
		return t.updateEvent(ctx, args)
	case "delete_event":
		return t.deleteEvent(ctx, args)
	case "get_event":
		return t.getEvent(ctx, args)
	case "check_availability":
		return t.checkAvailability(ctx, args)
	default:
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", action),
		}, nil
	}
}

// ensureStarted starts the MCP server if not already running
func (t *CalendarTool) ensureStarted(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return nil
	}

	// Parse command and args
	cmdParts := strings.Fields(t.config.Podcast.Calendar.Command)
	if len(cmdParts) == 0 {
		return fmt.Errorf("no Calendar MCP command configured")
	}

	// Set up environment with credentials
	// Try TokenManager first for OAuth tokens, fall back to credentials file
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	if t.tokenManager != nil {
		// Try to get OAuth access token (NEVER exposed to LLM)
		accessToken, err := t.tokenManager.GetToken(ctx, "google", "calendar")
		if err == nil && accessToken != "" {
			// Use OAuth token if available
			cmd.Env = append(cmd.Environ(), fmt.Sprintf("GOOGLE_OAUTH_TOKEN=%s", accessToken))
		} else {
			// Fall back to credentials file
			cmd.Env = append(cmd.Environ(),
				fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", t.config.Podcast.Calendar.CredentialsPath),
			)
		}
	} else {
		// No token manager, use credentials file
		cmd.Env = append(cmd.Environ(),
			fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", t.config.Podcast.Calendar.CredentialsPath),
		)
	}

	// Set up pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Calendar MCP server: %w", err)
	}

	t.cmd = cmd
	t.stdin = stdin
	t.stdout = bufio.NewReader(stdout)
	t.started = true

	// Initialize the MCP server
	if err := t.initialize(ctx); err != nil {
		t.stop()
		return fmt.Errorf("failed to initialize Calendar MCP server: %w", err)
	}

	return nil
}

// stop stops the MCP server
func (t *CalendarTool) stop() {
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	t.started = false
}

// initialize sends the initialize request to the MCP server
func (t *CalendarTool) initialize(ctx context.Context) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      t.nextID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "pedrocli",
				"version": "1.0.0",
			},
		},
	}

	_, err := t.sendRequest(ctx, req)
	return err
}

// sendRequest sends a JSON-RPC request and waits for response
func (t *CalendarTool) sendRequest(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	// Marshal request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response
	line, err := t.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for error
	if errObj, ok := resp["error"]; ok {
		return nil, fmt.Errorf("MCP error: %v", errObj)
	}

	return resp, nil
}

// nextID returns the next message ID
func (t *CalendarTool) nextID() int {
	t.msgID++
	return t.msgID
}

// callTool calls an MCP tool
func (t *CalendarTool) callTool(ctx context.Context, toolName string, toolArgs map[string]interface{}) (*Result, error) {
	if err := t.ensureStarted(ctx); err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      t.nextID(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": toolArgs,
		},
	}

	resp, err := t.sendRequest(ctx, req)
	if err != nil {
		return &Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Extract result
	result, ok := resp["result"]
	if !ok {
		return &Result{
			Success: false,
			Error:   "no result in response",
		}, nil
	}

	// Format result as JSON for output
	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return &Result{
		Success: true,
		Output:  string(resultJSON),
	}, nil
}

// getDefaultCalendarID returns the default calendar ID from config
func (t *CalendarTool) getDefaultCalendarID() string {
	if t.config.Podcast.Calendar.CalendarID != "" {
		return t.config.Podcast.Calendar.CalendarID
	}
	return "primary"
}

// listEvents lists upcoming events
func (t *CalendarTool) listEvents(ctx context.Context, args map[string]interface{}) (*Result, error) {
	calendarID, ok := args["calendar_id"].(string)
	if !ok || calendarID == "" {
		calendarID = t.getDefaultCalendarID()
	}

	toolArgs := map[string]interface{}{
		"calendar_id": calendarID,
	}

	if timeMin, ok := args["time_min"].(string); ok {
		toolArgs["time_min"] = timeMin
	}
	if timeMax, ok := args["time_max"].(string); ok {
		toolArgs["time_max"] = timeMax
	}
	if maxResults, ok := args["max_results"].(float64); ok {
		toolArgs["max_results"] = int(maxResults)
	}

	return t.callTool(ctx, "calendar_list_events", toolArgs)
}

// createEvent creates a new calendar event
func (t *CalendarTool) createEvent(ctx context.Context, args map[string]interface{}) (*Result, error) {
	summary, ok := args["summary"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "summary is required",
		}, nil
	}

	startTime, ok := args["start_time"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "start_time is required (ISO8601 format)",
		}, nil
	}

	endTime, ok := args["end_time"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "end_time is required (ISO8601 format)",
		}, nil
	}

	calendarID, ok := args["calendar_id"].(string)
	if !ok || calendarID == "" {
		calendarID = t.getDefaultCalendarID()
	}

	toolArgs := map[string]interface{}{
		"calendar_id": calendarID,
		"summary":     summary,
		"start_time":  startTime,
		"end_time":    endTime,
	}

	if description, ok := args["description"].(string); ok {
		toolArgs["description"] = description
	}
	if location, ok := args["location"].(string); ok {
		toolArgs["location"] = location
	}
	if attendees, ok := args["attendees"].([]interface{}); ok {
		toolArgs["attendees"] = attendees
	}

	return t.callTool(ctx, "calendar_create_event", toolArgs)
}

// updateEvent updates an existing event
func (t *CalendarTool) updateEvent(ctx context.Context, args map[string]interface{}) (*Result, error) {
	eventID, ok := args["event_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "event_id is required",
		}, nil
	}

	calendarID, ok := args["calendar_id"].(string)
	if !ok || calendarID == "" {
		calendarID = t.getDefaultCalendarID()
	}

	toolArgs := map[string]interface{}{
		"calendar_id": calendarID,
		"event_id":    eventID,
	}

	// Add optional fields
	if summary, ok := args["summary"].(string); ok {
		toolArgs["summary"] = summary
	}
	if startTime, ok := args["start_time"].(string); ok {
		toolArgs["start_time"] = startTime
	}
	if endTime, ok := args["end_time"].(string); ok {
		toolArgs["end_time"] = endTime
	}
	if description, ok := args["description"].(string); ok {
		toolArgs["description"] = description
	}
	if location, ok := args["location"].(string); ok {
		toolArgs["location"] = location
	}

	return t.callTool(ctx, "calendar_update_event", toolArgs)
}

// deleteEvent deletes an event
func (t *CalendarTool) deleteEvent(ctx context.Context, args map[string]interface{}) (*Result, error) {
	eventID, ok := args["event_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "event_id is required",
		}, nil
	}

	calendarID, ok := args["calendar_id"].(string)
	if !ok || calendarID == "" {
		calendarID = t.getDefaultCalendarID()
	}

	toolArgs := map[string]interface{}{
		"calendar_id": calendarID,
		"event_id":    eventID,
	}

	return t.callTool(ctx, "calendar_delete_event", toolArgs)
}

// getEvent gets event details
func (t *CalendarTool) getEvent(ctx context.Context, args map[string]interface{}) (*Result, error) {
	eventID, ok := args["event_id"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "event_id is required",
		}, nil
	}

	calendarID, ok := args["calendar_id"].(string)
	if !ok || calendarID == "" {
		calendarID = t.getDefaultCalendarID()
	}

	toolArgs := map[string]interface{}{
		"calendar_id": calendarID,
		"event_id":    eventID,
	}

	return t.callTool(ctx, "calendar_get_event", toolArgs)
}

// checkAvailability checks free/busy times
func (t *CalendarTool) checkAvailability(ctx context.Context, args map[string]interface{}) (*Result, error) {
	timeMin, ok := args["time_min"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "time_min is required (ISO8601 format)",
		}, nil
	}

	timeMax, ok := args["time_max"].(string)
	if !ok {
		return &Result{
			Success: false,
			Error:   "time_max is required (ISO8601 format)",
		}, nil
	}

	toolArgs := map[string]interface{}{
		"time_min": timeMin,
		"time_max": timeMax,
	}

	if calendarIDs, ok := args["calendar_ids"].([]interface{}); ok {
		toolArgs["calendar_ids"] = calendarIDs
	} else {
		// Use default calendar
		toolArgs["calendar_ids"] = []string{t.getDefaultCalendarID()}
	}

	return t.callTool(ctx, "calendar_check_availability", toolArgs)
}
