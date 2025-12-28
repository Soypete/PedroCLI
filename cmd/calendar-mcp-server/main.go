package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in the response
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CalendarServer is the MCP server for Google Calendar
type CalendarServer struct {
	service    *calendar.Service
	calendarID string
}

func main() {
	// Read credentials path from environment or args
	credsPath := os.Getenv("GOOGLE_CALENDAR_CREDENTIALS")
	if credsPath == "" && len(os.Args) > 1 {
		credsPath = os.Args[1]
	}
	if credsPath == "" {
		log.Fatal("GOOGLE_CALENDAR_CREDENTIALS environment variable or credentials path required")
	}

	calendarID := os.Getenv("GOOGLE_CALENDAR_ID")
	if calendarID == "" {
		calendarID = "primary"
	}

	// Initialize calendar service
	srv, err := initCalendarService(credsPath)
	if err != nil {
		log.Fatalf("Failed to initialize Calendar service: %v", err)
	}

	server := &CalendarServer{
		service:    srv,
		calendarID: calendarID,
	}

	// Start JSON-RPC server over stdio
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(req.ID, -32700, "Parse error", nil)
			continue
		}

		server.handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

func initCalendarService(credsPath string) (*calendar.Service, error) {
	ctx := context.Background()

	b, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %w", err)
	}

	// Get token from file or start OAuth flow
	token, err := getToken(config)
	if err != nil {
		return nil, fmt.Errorf("unable to get token: %w", err)
	}

	client := config.Client(ctx, token)
	service, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create Calendar service: %w", err)
	}

	return service, nil
}

func getToken(config *oauth2.Config) (*oauth2.Token, error) {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		saveToken(tokFile, tok)
	}
	return tok, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Fprintf(os.Stderr, "Go to the following link in your browser:\n%v\n", authURL)
	fmt.Fprintf(os.Stderr, "Enter authorization code: ")

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	return tok, nil
}

func saveToken(path string, token *oauth2.Token) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("Unable to cache oauth token: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (s *CalendarServer) handleRequest(req *JSONRPCRequest) {
	switch req.Method {
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolCall(req)
	default:
		sendError(req.ID, -32601, "Method not found", nil)
	}
}

func (s *CalendarServer) handleToolsList(req *JSONRPCRequest) {
	tools := []map[string]interface{}{
		{
			"name":        "list_events",
			"description": "List upcoming calendar events",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"time_min":    map[string]string{"type": "string", "description": "Start time (ISO8601)"},
					"time_max":    map[string]string{"type": "string", "description": "End time (ISO8601)"},
					"max_results": map[string]string{"type": "integer", "description": "Maximum number of events"},
				},
			},
		},
		{
			"name":        "create_event",
			"description": "Create a new calendar event",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"summary":     map[string]string{"type": "string", "description": "Event title"},
					"start_time":  map[string]string{"type": "string", "description": "Start time (ISO8601)"},
					"end_time":    map[string]string{"type": "string", "description": "End time (ISO8601)"},
					"description": map[string]string{"type": "string", "description": "Event description"},
					"location":    map[string]string{"type": "string", "description": "Event location"},
				},
				"required": []string{"summary", "start_time", "end_time"},
			},
		},
		{
			"name":        "update_event",
			"description": "Update an existing event",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id":    map[string]string{"type": "string", "description": "Event ID"},
					"summary":     map[string]string{"type": "string", "description": "Event title"},
					"start_time":  map[string]string{"type": "string", "description": "Start time (ISO8601)"},
					"end_time":    map[string]string{"type": "string", "description": "End time (ISO8601)"},
					"description": map[string]string{"type": "string", "description": "Event description"},
				},
				"required": []string{"event_id"},
			},
		},
		{
			"name":        "delete_event",
			"description": "Delete a calendar event",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_id": map[string]string{"type": "string", "description": "Event ID"},
				},
				"required": []string{"event_id"},
			},
		},
	}

	sendResponse(req.ID, map[string]interface{}{"tools": tools})
}

func (s *CalendarServer) handleToolCall(req *JSONRPCRequest) {
	params, ok := req.Params["arguments"].(map[string]interface{})
	if !ok {
		params = make(map[string]interface{})
	}

	toolName, ok := req.Params["name"].(string)
	if !ok {
		sendError(req.ID, -32602, "Invalid params: name required", nil)
		return
	}

	var result *ToolResult
	var err error

	switch toolName {
	case "list_events":
		result, err = s.listEvents(params)
	case "create_event":
		result, err = s.createEvent(params)
	case "update_event":
		result, err = s.updateEvent(params)
	case "delete_event":
		result, err = s.deleteEvent(params)
	default:
		sendError(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", toolName), nil)
		return
	}

	if err != nil {
		result = &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		}
	}

	sendResponse(req.ID, result)
}

func (s *CalendarServer) listEvents(args map[string]interface{}) (*ToolResult, error) {
	timeMin := time.Now().Format(time.RFC3339)
	if tm, ok := args["time_min"].(string); ok {
		timeMin = tm
	}

	timeMax := time.Now().AddDate(0, 1, 0).Format(time.RFC3339)
	if tm, ok := args["time_max"].(string); ok {
		timeMax = tm
	}

	maxResults := int64(10)
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int64(mr)
	}

	events, err := s.service.Events.List(s.calendarID).
		TimeMin(timeMin).
		TimeMax(timeMax).
		MaxResults(maxResults).
		SingleEvents(true).
		OrderBy("startTime").
		Do()

	if err != nil {
		return nil, err
	}

	var output string
	if len(events.Items) == 0 {
		output = "No upcoming events found."
	} else {
		output = fmt.Sprintf("Found %d events:\n\n", len(events.Items))
		for _, item := range events.Items {
			start := item.Start.DateTime
			if start == "" {
				start = item.Start.Date
			}
			output += fmt.Sprintf("- %s (%s) [ID: %s]\n", item.Summary, start, item.Id)
			if item.Description != "" {
				output += fmt.Sprintf("  %s\n", item.Description)
			}
		}
	}

	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: output}},
	}, nil
}

func (s *CalendarServer) createEvent(args map[string]interface{}) (*ToolResult, error) {
	summary, ok := args["summary"].(string)
	if !ok {
		return nil, fmt.Errorf("summary is required")
	}

	startTime, ok := args["start_time"].(string)
	if !ok {
		return nil, fmt.Errorf("start_time is required")
	}

	endTime, ok := args["end_time"].(string)
	if !ok {
		return nil, fmt.Errorf("end_time is required")
	}

	event := &calendar.Event{
		Summary: summary,
		Start:   &calendar.EventDateTime{DateTime: startTime},
		End:     &calendar.EventDateTime{DateTime: endTime},
	}

	if desc, ok := args["description"].(string); ok {
		event.Description = desc
	}

	if loc, ok := args["location"].(string); ok {
		event.Location = loc
	}

	created, err := s.service.Events.Insert(s.calendarID, event).Do()
	if err != nil {
		return nil, err
	}

	output := fmt.Sprintf("Event created successfully!\nID: %s\nLink: %s", created.Id, created.HtmlLink)
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: output}},
	}, nil
}

func (s *CalendarServer) updateEvent(args map[string]interface{}) (*ToolResult, error) {
	eventID, ok := args["event_id"].(string)
	if !ok {
		return nil, fmt.Errorf("event_id is required")
	}

	event, err := s.service.Events.Get(s.calendarID, eventID).Do()
	if err != nil {
		return nil, err
	}

	if summary, ok := args["summary"].(string); ok {
		event.Summary = summary
	}

	if startTime, ok := args["start_time"].(string); ok {
		event.Start.DateTime = startTime
	}

	if endTime, ok := args["end_time"].(string); ok {
		event.End.DateTime = endTime
	}

	if desc, ok := args["description"].(string); ok {
		event.Description = desc
	}

	updated, err := s.service.Events.Update(s.calendarID, eventID, event).Do()
	if err != nil {
		return nil, err
	}

	output := fmt.Sprintf("Event updated successfully!\nID: %s\nLink: %s", updated.Id, updated.HtmlLink)
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: output}},
	}, nil
}

func (s *CalendarServer) deleteEvent(args map[string]interface{}) (*ToolResult, error) {
	eventID, ok := args["event_id"].(string)
	if !ok {
		return nil, fmt.Errorf("event_id is required")
	}

	err := s.service.Events.Delete(s.calendarID, eventID).Do()
	if err != nil {
		return nil, err
	}

	output := fmt.Sprintf("Event %s deleted successfully!", eventID)
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: output}},
	}, nil
}

func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	d, _ := json.Marshal(resp)
	fmt.Println(string(d))
}
