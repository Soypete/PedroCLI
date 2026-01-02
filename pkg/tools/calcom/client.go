package calcom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	// DefaultBaseURL is the default Cal.com API base URL
	DefaultBaseURL = "https://api.cal.com/v1"

	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
)

// Client handles HTTP communication with the Cal.com API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Cal.com API client
func NewClient(apiKey string, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// doRequest performs an HTTP request to the Cal.com API
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Build full URL
	fullURL := c.baseURL + path

	// Marshal request body if provided
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Unmarshal response
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// GetBookings retrieves a list of bookings
func (c *Client) GetBookings(ctx context.Context, status string, eventTypeID int, limit int) ([]Booking, error) {
	params := url.Values{}
	if status != "" {
		params.Set("status", status)
	}
	if eventTypeID > 0 {
		params.Set("eventTypeId", strconv.Itoa(eventTypeID))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	path := "/bookings"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp BookingsResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	return resp.Bookings, nil
}

// GetBooking retrieves a specific booking by UID
func (c *Client) GetBooking(ctx context.Context, bookingUID string) (*Booking, error) {
	path := "/bookings/" + bookingUID

	var booking Booking
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &booking); err != nil {
		return nil, err
	}

	return &booking, nil
}

// CreateBookingRequest represents the request to create a booking
type CreateBookingRequest struct {
	EventTypeID int                    `json:"eventTypeId"`
	Start       string                 `json:"start"` // ISO 8601 datetime
	Attendee    Attendee               `json:"responses"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Notes       string                 `json:"notes,omitempty"`
	TimeZone    string                 `json:"timeZone,omitempty"`
	Language    string                 `json:"language,omitempty"`
}

// CreateBooking creates a new booking
func (c *Client) CreateBooking(ctx context.Context, req CreateBookingRequest) (*Booking, error) {
	var booking Booking
	if err := c.doRequest(ctx, http.MethodPost, "/bookings", req, &booking); err != nil {
		return nil, err
	}

	return &booking, nil
}

// RescheduleBookingRequest represents the request to reschedule a booking
type RescheduleBookingRequest struct {
	NewStart string `json:"start"` // ISO 8601 datetime
	Reason   string `json:"rescheduleReason,omitempty"`
}

// RescheduleBooking reschedules an existing booking
func (c *Client) RescheduleBooking(ctx context.Context, bookingUID string, req RescheduleBookingRequest) (*Booking, error) {
	path := "/bookings/" + bookingUID

	var booking Booking
	if err := c.doRequest(ctx, http.MethodPatch, path, req, &booking); err != nil {
		return nil, err
	}

	return &booking, nil
}

// CancelBookingRequest represents the request to cancel a booking
type CancelBookingRequest struct {
	Reason string `json:"cancellationReason,omitempty"`
}

// CancelBooking cancels an existing booking
func (c *Client) CancelBooking(ctx context.Context, bookingUID string, reason string) error {
	path := "/bookings/" + bookingUID + "/cancel"

	req := CancelBookingRequest{
		Reason: reason,
	}

	if err := c.doRequest(ctx, http.MethodDelete, path, req, nil); err != nil {
		return err
	}

	return nil
}

// GetEventTypes retrieves a list of event types
func (c *Client) GetEventTypes(ctx context.Context, limit int) ([]EventType, error) {
	path := "/event-types"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}

	var resp EventTypesResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	return resp.EventTypes, nil
}

// GetEventType retrieves a specific event type by ID
func (c *Client) GetEventType(ctx context.Context, eventTypeID int) (*EventType, error) {
	path := fmt.Sprintf("/event-types/%d", eventTypeID)

	var eventType EventType
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &eventType); err != nil {
		return nil, err
	}

	return &eventType, nil
}

// UpdateEventTypeRequest represents fields that can be updated on an event type
type UpdateEventTypeRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Length      *int    `json:"length,omitempty"`
	Hidden      *bool   `json:"hidden,omitempty"`
}

// UpdateEventType updates an existing event type
func (c *Client) UpdateEventType(ctx context.Context, eventTypeID int, req UpdateEventTypeRequest) (*EventType, error) {
	path := fmt.Sprintf("/event-types/%d", eventTypeID)

	var eventType EventType
	if err := c.doRequest(ctx, http.MethodPatch, path, req, &eventType); err != nil {
		return nil, err
	}

	return &eventType, nil
}

// DeleteEventType deletes an event type
func (c *Client) DeleteEventType(ctx context.Context, eventTypeID int) error {
	path := fmt.Sprintf("/event-types/%d", eventTypeID)

	if err := c.doRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return err
	}

	return nil
}

// CreateEventTypeRequest represents the request to create an event type
type CreateEventTypeRequest struct {
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Length      int        `json:"length"`
	Description string     `json:"description,omitempty"`
	Locations   []Location `json:"locations,omitempty"`
}

// CreateEventType creates a new event type
func (c *Client) CreateEventType(ctx context.Context, req CreateEventTypeRequest) (*EventType, error) {
	var eventType EventType
	if err := c.doRequest(ctx, http.MethodPost, "/event-types", req, &eventType); err != nil {
		return nil, err
	}

	return &eventType, nil
}

// GetSchedules retrieves a list of availability schedules
func (c *Client) GetSchedules(ctx context.Context) ([]Schedule, error) {
	var resp SchedulesResponse
	if err := c.doRequest(ctx, http.MethodGet, "/schedules", nil, &resp); err != nil {
		return nil, err
	}

	return resp.Schedules, nil
}

// GetAvailability retrieves available time slots for an event type
func (c *Client) GetAvailability(ctx context.Context, eventTypeID int, startTime, endTime, timeZone string) ([]AvailableSlot, error) {
	params := url.Values{}
	params.Set("eventTypeId", strconv.Itoa(eventTypeID))
	params.Set("startTime", startTime)
	params.Set("endTime", endTime)
	if timeZone != "" {
		params.Set("timeZone", timeZone)
	}

	path := "/availability?" + params.Encode()

	var resp AvailabilityResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	return resp.Slots, nil
}

// GetBusyTimes retrieves busy/unavailable time periods
func (c *Client) GetBusyTimes(ctx context.Context, startTime, endTime string) ([]BusyTime, error) {
	params := url.Values{}
	params.Set("dateFrom", startTime)
	params.Set("dateTo", endTime)

	path := "/busy-times?" + params.Encode()

	var resp BusyTimesResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	return resp.Busy, nil
}

// GetMe retrieves the authenticated user's profile
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var user User
	if err := c.doRequest(ctx, http.MethodGet, "/me", nil, &user); err != nil {
		return nil, err
	}

	return &user, nil
}
