package calcom

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/soypete/pedrocli/pkg/config"
)

// TokenManager defines the interface for retrieving tokens
type TokenManager interface {
	GetToken(ctx context.Context, provider, service string) (accessToken string, err error)
}

// Result represents a tool execution result
type Result struct {
	Success       bool                   `json:"success"`
	Output        string                 `json:"output"`
	Error         string                 `json:"error,omitempty"`
	ModifiedFiles []string               `json:"modified_files,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
}

// ErrorResult creates an error result with the given message
func ErrorResult(msg string) *Result {
	return &Result{
		Success: false,
		Error:   msg,
		Output:  "Error: " + msg,
	}
}

// CalComTool provides Cal.com scheduling capabilities
type CalComTool struct {
	config       *config.Config
	tokenManager TokenManager
	client       *Client
}

// NewCalComTool creates a new Cal.com tool
func NewCalComTool(cfg *config.Config, tokenManager TokenManager) *CalComTool {
	return &CalComTool{
		config:       cfg,
		tokenManager: tokenManager,
	}
}

// Name returns the tool name
func (t *CalComTool) Name() string {
	return "cal_com"
}

// Description returns the tool description
func (t *CalComTool) Description() string {
	return "Interact with Cal.com for scheduling, bookings, and availability management. Supports creating booking links, managing event types, checking availability, and handling bookings."
}

// getClient returns an initialized Cal.com client with API key
func (t *CalComTool) getClient(ctx context.Context) (*Client, error) {
	if t.client != nil {
		return t.client, nil
	}

	// Get API key using 3-tier fallback
	apiKey, err := t.getAPIKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("cal.com API key not configured: %w", err)
	}

	// Get base URL from config or use default
	baseURL := DefaultBaseURL
	if t.config != nil && t.config.CalCom.BaseURL != "" {
		baseURL = t.config.CalCom.BaseURL
	}

	// Create and cache client
	t.client = NewClient(apiKey, baseURL)
	return t.client, nil
}

// getAPIKey retrieves the Cal.com API key using 3-tier fallback
func (t *CalComTool) getAPIKey(ctx context.Context) (string, error) {
	// Tier 1: TokenManager (database - secure)
	if t.tokenManager != nil {
		token, err := t.tokenManager.GetToken(ctx, "calcom", "api")
		if err == nil && token != "" {
			return token, nil
		}
	}

	// Tier 2: Config file
	if t.config != nil && t.config.CalCom.APIKey != "" {
		return t.config.CalCom.APIKey, nil
	}

	// Tier 3: Environment variable (supports `op run`)
	if apiKey := os.Getenv("CAL_API_KEY"); apiKey != "" {
		return apiKey, nil
	}

	return "", fmt.Errorf("no API key found in TokenManager, config, or CAL_API_KEY env var")
}

// Execute executes the Cal.com tool action
func (t *CalComTool) Execute(ctx context.Context, args map[string]interface{}) (*Result, error) {
	// Get action
	action, ok := args["action"].(string)
	if !ok {
		return ErrorResult("missing or invalid 'action' parameter"), nil
	}

	// Get client
	client, err := t.getClient(ctx)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	// Dispatch to action handler
	switch action {
	case "get_bookings":
		return t.getBookings(ctx, client, args)
	case "get_booking":
		return t.getBooking(ctx, client, args)
	case "create_booking":
		return t.createBooking(ctx, client, args)
	case "reschedule_booking":
		return t.rescheduleBooking(ctx, client, args)
	case "cancel_booking":
		return t.cancelBooking(ctx, client, args)
	case "get_event_types":
		return t.getEventTypes(ctx, client, args)
	case "get_event_type":
		return t.getEventType(ctx, client, args)
	case "create_event_type":
		return t.createEventType(ctx, client, args)
	case "update_event_type":
		return t.updateEventType(ctx, client, args)
	case "delete_event_type":
		return t.deleteEventType(ctx, client, args)
	case "get_schedules":
		return t.getSchedules(ctx, client, args)
	case "get_availability":
		return t.getAvailability(ctx, client, args)
	case "get_busy_times":
		return t.getBusyTimes(ctx, client, args)
	case "get_me":
		return t.getMe(ctx, client, args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action)), nil
	}
}

// getBookings handles the get_bookings action
func (t *CalComTool) getBookings(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	status, _ := args["status"].(string)
	eventTypeID, _ := args["eventTypeId"].(int)
	limit, _ := args["limit"].(int)
	if limit == 0 {
		limit = 10 // Default
	}

	bookings, err := client.GetBookings(ctx, status, eventTypeID, limit)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(bookings, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Found %d bookings:\n%s", len(bookings), string(output)),
		Data:    map[string]interface{}{"bookings": bookings},
	}, nil
}

// getBooking handles the get_booking action
func (t *CalComTool) getBooking(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	bookingUID, ok := args["bookingUid"].(string)
	if !ok {
		return ErrorResult("missing required parameter: bookingUid"), nil
	}

	booking, err := client.GetBooking(ctx, bookingUID)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(booking, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Booking details:\n%s", string(output)),
		Data:    map[string]interface{}{"booking": booking},
	}, nil
}

// createBooking handles the create_booking action
func (t *CalComTool) createBooking(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	eventTypeID, ok := args["eventTypeId"].(int)
	if !ok {
		return ErrorResult("missing required parameter: eventTypeId"), nil
	}

	start, ok := args["start"].(string)
	if !ok {
		return ErrorResult("missing required parameter: start (ISO 8601 datetime)"), nil
	}

	attendeeData, ok := args["attendee"].(map[string]interface{})
	if !ok {
		return ErrorResult("missing required parameter: attendee {name, email, timeZone}"), nil
	}

	attendee := Attendee{
		Name:     attendeeData["name"].(string),
		Email:    attendeeData["email"].(string),
		TimeZone: attendeeData["timeZone"].(string),
	}

	req := CreateBookingRequest{
		EventTypeID: eventTypeID,
		Start:       start,
		Attendee:    attendee,
	}

	if metadata, ok := args["metadata"].(map[string]interface{}); ok {
		req.Metadata = metadata
	}
	if notes, ok := args["notes"].(string); ok {
		req.Notes = notes
	}

	booking, err := client.CreateBooking(ctx, req)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(booking, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("✓ Booking created successfully:\n%s", string(output)),
		Data:    map[string]interface{}{"booking": booking},
	}, nil
}

// rescheduleBooking handles the reschedule_booking action
func (t *CalComTool) rescheduleBooking(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	bookingUID, ok := args["bookingUid"].(string)
	if !ok {
		return ErrorResult("missing required parameter: bookingUid"), nil
	}

	newStart, ok := args["newStart"].(string)
	if !ok {
		return ErrorResult("missing required parameter: newStart (ISO 8601 datetime)"), nil
	}

	reason, _ := args["reason"].(string)

	req := RescheduleBookingRequest{
		NewStart: newStart,
		Reason:   reason,
	}

	booking, err := client.RescheduleBooking(ctx, bookingUID, req)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(booking, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("✓ Booking rescheduled to %s:\n%s", newStart, string(output)),
		Data:    map[string]interface{}{"booking": booking},
	}, nil
}

// cancelBooking handles the cancel_booking action
func (t *CalComTool) cancelBooking(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	bookingUID, ok := args["bookingUid"].(string)
	if !ok {
		return ErrorResult("missing required parameter: bookingUid"), nil
	}

	reason, _ := args["reason"].(string)

	err := client.CancelBooking(ctx, bookingUID, reason)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("✓ Booking %s cancelled successfully", bookingUID),
	}, nil
}

// getEventTypes handles the get_event_types action
func (t *CalComTool) getEventTypes(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	limit, _ := args["limit"].(int)

	eventTypes, err := client.GetEventTypes(ctx, limit)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(eventTypes, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Found %d event types:\n%s", len(eventTypes), string(output)),
		Data:    map[string]interface{}{"event_types": eventTypes},
	}, nil
}

// getEventType handles the get_event_type action
func (t *CalComTool) getEventType(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	eventTypeID, ok := args["eventTypeId"].(int)
	if !ok {
		return ErrorResult("missing required parameter: eventTypeId"), nil
	}

	eventType, err := client.GetEventType(ctx, eventTypeID)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(eventType, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Event type details:\n%s", string(output)),
		Data:    map[string]interface{}{"event_type": eventType},
	}, nil
}

// createEventType handles the create_event_type action
func (t *CalComTool) createEventType(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	title, ok := args["title"].(string)
	if !ok {
		return ErrorResult("missing required parameter: title"), nil
	}

	slug, ok := args["slug"].(string)
	if !ok {
		return ErrorResult("missing required parameter: slug"), nil
	}

	length, ok := args["length"].(int)
	if !ok {
		return ErrorResult("missing required parameter: length (duration in minutes)"), nil
	}

	req := CreateEventTypeRequest{
		Title:  title,
		Slug:   slug,
		Length: length,
	}

	if description, ok := args["description"].(string); ok {
		req.Description = description
	}

	eventType, err := client.CreateEventType(ctx, req)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(eventType, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("✓ Event type created:\n%s\n\nBooking URL: %s", string(output), eventType.BookingURL),
		Data:    map[string]interface{}{"event_type": eventType},
	}, nil
}

// updateEventType handles the update_event_type action
func (t *CalComTool) updateEventType(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	eventTypeID, ok := args["eventTypeId"].(int)
	if !ok {
		return ErrorResult("missing required parameter: eventTypeId"), nil
	}

	req := UpdateEventTypeRequest{}
	if title, ok := args["title"].(string); ok {
		req.Title = &title
	}
	if description, ok := args["description"].(string); ok {
		req.Description = &description
	}
	if length, ok := args["length"].(int); ok {
		req.Length = &length
	}
	if hidden, ok := args["hidden"].(bool); ok {
		req.Hidden = &hidden
	}

	eventType, err := client.UpdateEventType(ctx, eventTypeID, req)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(eventType, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("✓ Event type updated:\n%s", string(output)),
		Data:    map[string]interface{}{"event_type": eventType},
	}, nil
}

// deleteEventType handles the delete_event_type action
func (t *CalComTool) deleteEventType(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	eventTypeID, ok := args["eventTypeId"].(int)
	if !ok {
		return ErrorResult("missing required parameter: eventTypeId"), nil
	}

	err := client.DeleteEventType(ctx, eventTypeID)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	return &Result{
		Success: true,
		Output:  fmt.Sprintf("✓ Event type %d deleted successfully", eventTypeID),
	}, nil
}

// getSchedules handles the get_schedules action
func (t *CalComTool) getSchedules(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	schedules, err := client.GetSchedules(ctx)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(schedules, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Found %d schedules:\n%s", len(schedules), string(output)),
		Data:    map[string]interface{}{"schedules": schedules},
	}, nil
}

// getAvailability handles the get_availability action
func (t *CalComTool) getAvailability(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	eventTypeID, ok := args["eventTypeId"].(int)
	if !ok {
		return ErrorResult("missing required parameter: eventTypeId"), nil
	}

	startTime, ok := args["startTime"].(string)
	if !ok {
		return ErrorResult("missing required parameter: startTime (ISO 8601)"), nil
	}

	endTime, ok := args["endTime"].(string)
	if !ok {
		return ErrorResult("missing required parameter: endTime (ISO 8601)"), nil
	}

	timeZone, _ := args["timeZone"].(string)

	slots, err := client.GetAvailability(ctx, eventTypeID, startTime, endTime, timeZone)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(slots, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Found %d available slots:\n%s", len(slots), string(output)),
		Data:    map[string]interface{}{"slots": slots},
	}, nil
}

// getBusyTimes handles the get_busy_times action
func (t *CalComTool) getBusyTimes(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	startTime, ok := args["startTime"].(string)
	if !ok {
		return ErrorResult("missing required parameter: startTime (ISO 8601)"), nil
	}

	endTime, ok := args["endTime"].(string)
	if !ok {
		return ErrorResult("missing required parameter: endTime (ISO 8601)"), nil
	}

	busyTimes, err := client.GetBusyTimes(ctx, startTime, endTime)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(busyTimes, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("Found %d busy time periods:\n%s", len(busyTimes), string(output)),
		Data:    map[string]interface{}{"busy_times": busyTimes},
	}, nil
}

// getMe handles the get_me action
func (t *CalComTool) getMe(ctx context.Context, client *Client, args map[string]interface{}) (*Result, error) {
	user, err := client.GetMe(ctx)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	output, _ := json.MarshalIndent(user, "", "  ")
	return &Result{
		Success: true,
		Output:  fmt.Sprintf("User profile:\n%s", string(output)),
		Data:    map[string]interface{}{"user": user},
	}, nil
}
