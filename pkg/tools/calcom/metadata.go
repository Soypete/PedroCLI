package calcom

import (
	"github.com/soypete/pedrocli/pkg/logits"
)

// ToolMetadata represents rich metadata for a tool (local copy to avoid import cycle)
type ToolMetadata struct {
	Schema               *logits.JSONSchema
	Category             string
	Optionality          string
	UsageHint            string
	Examples             []ToolExample
	RequiresCapabilities []string
	Consumes             []string
	Produces             []string
}

// ToolExample represents an example invocation
type ToolExample struct {
	Description string
	Arguments   map[string]interface{}
}

// Metadata returns the tool metadata for Cal.com
func (t *CalComTool) Metadata() *ToolMetadata {
	return &ToolMetadata{
		Schema: &logits.JSONSchema{
			Type: "object",
			Properties: map[string]*logits.JSONSchema{
				"action": {
					Type: "string",
					Enum: []interface{}{
						"get_bookings",
						"get_booking",
						"create_booking",
						"reschedule_booking",
						"cancel_booking",
						"get_event_types",
						"get_event_type",
						"create_event_type",
						"update_event_type",
						"delete_event_type",
						"get_schedules",
						"get_availability",
						"get_busy_times",
						"get_me",
					},
					Description: "The Cal.com action to perform",
				},
				// Booking parameters
				"bookingUid": {
					Type:        "string",
					Description: "Unique ID of the booking (for get/reschedule/cancel actions)",
				},
				"status": {
					Type:        "string",
					Enum:        []interface{}{"upcoming", "recurring", "past", "cancelled", "unconfirmed"},
					Description: "Filter bookings by status",
				},
				"eventTypeId": {
					Type:        "integer",
					Description: "ID of the event type to filter by or book",
				},
				"start": {
					Type:        "string",
					Description: "ISO 8601 datetime for booking start (e.g., 2024-03-15T10:00:00Z)",
				},
				"newStart": {
					Type:        "string",
					Description: "New ISO 8601 datetime for rescheduling",
				},
				"attendee": {
					Type: "object",
					Properties: map[string]*logits.JSONSchema{
						"name": {
							Type:        "string",
							Description: "Attendee's full name",
						},
						"email": {
							Type:        "string",
							Description: "Attendee's email address",
						},
						"timeZone": {
							Type:        "string",
							Description: "Attendee's timezone (e.g., America/New_York)",
						},
					},
					Required:    []string{"name", "email", "timeZone"},
					Description: "Attendee information for booking",
				},
				"metadata": {
					Type:        "object",
					Description: "Custom metadata to attach to booking",
				},
				"notes": {
					Type:        "string",
					Description: "Notes to attach to the booking",
				},
				"reason": {
					Type:        "string",
					Description: "Reason for rescheduling or cancellation",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return (default 10)",
				},
				// Event type parameters
				"title": {
					Type:        "string",
					Description: "Event type title/name",
				},
				"slug": {
					Type:        "string",
					Description: "URL slug for booking page (e.g., '30min' for cal.com/user/30min)",
				},
				"length": {
					Type:        "integer",
					Description: "Duration of the event in minutes",
				},
				"description": {
					Type:        "string",
					Description: "Description shown on the booking page",
				},
				"hidden": {
					Type:        "boolean",
					Description: "Whether to hide the event type from public listing",
				},
				"locations": {
					Type: "array",
					Items: &logits.JSONSchema{
						Type: "object",
						Properties: map[string]*logits.JSONSchema{
							"type": {
								Type:        "string",
								Description: "Location type (integration, phone, link)",
							},
							"address": {
								Type:        "string",
								Description: "Physical address or phone number",
							},
							"link": {
								Type:        "string",
								Description: "Meeting link URL",
							},
						},
					},
					Description: "Meeting location options",
				},
				// Availability parameters
				"startTime": {
					Type:        "string",
					Description: "ISO 8601 datetime for availability range start",
				},
				"endTime": {
					Type:        "string",
					Description: "ISO 8601 datetime for availability range end",
				},
				"timeZone": {
					Type:        "string",
					Description: "Timezone for availability results (e.g., America/New_York)",
				},
			},
			Required: []string{"action"},
		},
		Category:    "utility",
		Optionality: "optional",
		UsageHint: `Use this tool to manage Cal.com scheduling and bookings. Common workflows:
- Create a booking link: get_event_types to list available types
- Check availability: get_availability with event type ID and date range
- Create booking: create_booking with event type, time, and attendee info
- Manage bookings: get_bookings, reschedule_booking, cancel_booking
- User profile: get_me to see your Cal.com profile and settings`,
		Examples: []ToolExample{
			{
				Description: "List all event types (booking pages)",
				Arguments: map[string]interface{}{
					"action": "get_event_types",
				},
			},
			{
				Description: "Get availability for an event type",
				Arguments: map[string]interface{}{
					"action":      "get_availability",
					"eventTypeId": 12345,
					"startTime":   "2024-03-15T00:00:00Z",
					"endTime":     "2024-03-22T00:00:00Z",
					"timeZone":    "America/New_York",
				},
			},
			{
				Description: "Create a booking",
				Arguments: map[string]interface{}{
					"action":      "create_booking",
					"eventTypeId": 12345,
					"start":       "2024-03-15T10:00:00Z",
					"attendee": map[string]interface{}{
						"name":     "John Doe",
						"email":    "john@example.com",
						"timeZone": "America/New_York",
					},
					"notes": "Looking forward to our meeting!",
				},
			},
			{
				Description: "List upcoming bookings",
				Arguments: map[string]interface{}{
					"action": "get_bookings",
					"status": "upcoming",
					"limit":  10,
				},
			},
			{
				Description: "Create a new event type (booking page)",
				Arguments: map[string]interface{}{
					"action":      "create_event_type",
					"title":       "Podcast Interview",
					"slug":        "podcast-60min",
					"length":      60,
					"description": "60-minute podcast interview for SoypeteTech",
				},
			},
			{
				Description: "Get user profile",
				Arguments: map[string]interface{}{
					"action": "get_me",
				},
			},
		},
		RequiresCapabilities: []string{"internet", "calendar"},
		Consumes:             []string{"datetime", "email", "timezone"},
		Produces:             []string{"booking_url", "calendar_event", "availability_slots"},
	}
}
