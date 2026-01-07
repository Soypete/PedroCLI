package calcom

import "time"

// Booking represents a Cal.com booking/appointment
type Booking struct {
	ID          int       `json:"id"`
	UID         string    `json:"uid"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Status      string    `json:"status"` // upcoming, cancelled, past, etc.

	// Attendee information
	Attendees []Attendee `json:"attendees"`

	// Event type information
	EventTypeID int    `json:"eventTypeId"`
	EventType   string `json:"eventType,omitempty"`

	// Metadata
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CustomInputs map[string]interface{} `json:"customInputs,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Meeting details
	Location   string `json:"location,omitempty"`
	MeetingURL string `json:"meetingUrl,omitempty"`

	// Cancellation
	CancellationReason string `json:"cancellationReason,omitempty"`
	RescheduledFrom    string `json:"rescheduledFromUid,omitempty"`
	RescheduledTo      string `json:"rescheduledToUid,omitempty"`
}

// Attendee represents a booking attendee
type Attendee struct {
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	TimeZone string   `json:"timeZone"`
	Locale   string   `json:"locale,omitempty"`
	Guests   []string `json:"guests,omitempty"`
}

// EventType represents a Cal.com event type (booking page)
type EventType struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Length      int    `json:"length"` // Duration in minutes

	// Booking URL
	BookingURL string `json:"link,omitempty"` // Full booking URL

	// Availability
	ScheduleID int  `json:"scheduleId,omitempty"`
	Hidden     bool `json:"hidden"`

	// Location settings
	Locations []Location `json:"locations,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`

	// Advanced settings
	MinimumBookingNotice int  `json:"minimumBookingNotice,omitempty"` // Minutes
	BeforeEventBuffer    int  `json:"beforeEventBuffer,omitempty"`    // Minutes
	AfterEventBuffer     int  `json:"afterEventBuffer,omitempty"`     // Minutes
	RequiresConfirmation bool `json:"requiresConfirmation"`

	// User info
	UserID   int    `json:"userId,omitempty"`
	Username string `json:"username,omitempty"`
}

// Location represents a meeting location option
type Location struct {
	Type        string `json:"type"` // integration, phone, link, etc.
	Address     string `json:"address,omitempty"`
	Link        string `json:"link,omitempty"`
	DisplayName string `json:"displayLocationLabel,omitempty"`
}

// Schedule represents an availability schedule
type Schedule struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	TimeZone      string         `json:"timeZone"`
	IsDefault     bool           `json:"isDefault"`
	Availability  []TimeSlot     `json:"availability"`
	DateOverrides []DateOverride `json:"dateOverrides,omitempty"`
}

// TimeSlot represents a recurring availability slot
type TimeSlot struct {
	Days      []string  `json:"days"` // Monday, Tuesday, etc.
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

// DateOverride represents a specific date override
type DateOverride struct {
	Date      time.Time `json:"date"`
	StartTime time.Time `json:"startTime,omitempty"`
	EndTime   time.Time `json:"endTime,omitempty"`
}

// AvailableSlot represents an available booking slot
type AvailableSlot struct {
	Time      time.Time `json:"time"`
	Users     []int     `json:"users,omitempty"`
	Attendees int       `json:"attendees,omitempty"`
}

// BusyTime represents a busy/unavailable time period
type BusyTime struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Source string    `json:"source,omitempty"` // calendar name/source
}

// User represents the Cal.com user profile
type User struct {
	ID          int       `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	Username    string    `json:"username"`
	TimeZone    string    `json:"timeZone"`
	WeekStart   string    `json:"weekStart"` // Sunday, Monday, etc.
	CreatedDate time.Time `json:"createdDate"`

	// Default settings
	DefaultScheduleID int `json:"defaultScheduleId,omitempty"`

	// Avatar/branding
	Avatar         string `json:"avatar,omitempty"`
	BrandColor     string `json:"brandColor,omitempty"`
	DarkBrandColor string `json:"darkBrandColor,omitempty"`
}

// API Response wrappers

// BookingsResponse wraps the bookings list endpoint
type BookingsResponse struct {
	Bookings []Booking `json:"bookings"`
}

// EventTypesResponse wraps the event types list endpoint
type EventTypesResponse struct {
	EventTypes []EventType `json:"event_types"`
}

// SchedulesResponse wraps the schedules list endpoint
type SchedulesResponse struct {
	Schedules []Schedule `json:"schedules"`
}

// AvailabilityResponse wraps the availability endpoint
type AvailabilityResponse struct {
	Slots []AvailableSlot `json:"slots"`
}

// BusyTimesResponse wraps the busy times endpoint
type BusyTimesResponse struct {
	Busy []BusyTime `json:"busy"`
}

// ErrorResponse represents a Cal.com API error
type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}
