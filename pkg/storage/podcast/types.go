package podcast

import (
	"time"
)

// EpisodeStatus represents the current workflow state of an episode.
type EpisodeStatus string

const (
	StatusUploaded      EpisodeStatus = "uploaded"
	StatusTranscribing  EpisodeStatus = "transcribing"
	StatusTranscribed   EpisodeStatus = "transcribed"
	StatusFactChecking  EpisodeStatus = "fact_checking"
	StatusFactChecked   EpisodeStatus = "fact_checked"
	StatusShowNotes     EpisodeStatus = "show_notes"
	StatusShowNotesDone EpisodeStatus = "show_notes_done"
	StatusPublished     EpisodeStatus = "published"
)

// Episode represents a podcast episode in the workflow.
type Episode struct {
	ID            string        `json:"id"`
	EpisodeNumber string        `json:"episode_number"` // e.g., "S01E03"
	Title         string        `json:"title"`
	RecordDate    time.Time     `json:"record_date"`
	Status        EpisodeStatus `json:"status"`

	// S3 storage keys
	RecordingKey  string `json:"recording_key"`
	TranscriptKey string `json:"transcript_key"`

	// Content (loaded on demand)
	Transcript string       `json:"transcript,omitempty"`
	FactChecks []FactCheck  `json:"fact_checks,omitempty"`
	ShowNotes  *ShowNotes   `json:"show_notes,omitempty"`
	Template   *NoteTemplate `json:"template,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FactCheck represents a single fact-check annotation on the transcript.
type FactCheck struct {
	ID          string `json:"id"`
	Timestamp   string `json:"timestamp"`    // e.g., "12:34"
	Claim       string `json:"claim"`        // The claim text from the transcript
	Status      string `json:"status"`       // unchecked, verified, incorrect, needs_edit
	Notes       string `json:"notes"`        // Reviewer notes
	CorrectedTo string `json:"corrected_to,omitempty"`
}

// ShowNotes holds extracted show notes for an episode.
type ShowNotes struct {
	Summary  string    `json:"summary"`
	Links    []Link    `json:"links"`
	Chapters []Chapter `json:"chapters"`
}

// Link represents a URL mentioned or referenced in the episode.
type Link struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Context   string `json:"context"`   // Where in the episode it was mentioned
	Timestamp string `json:"timestamp"` // Approximate timestamp
}

// Chapter represents a timestamped section of the episode.
type Chapter struct {
	Timestamp string `json:"timestamp"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
}

// NoteTemplate is the final show notes template with all metadata.
type NoteTemplate struct {
	EpisodeNumber string    `json:"episode_number"`
	Title         string    `json:"title"`
	RecordDate    time.Time `json:"record_date"`
	PublishDate   time.Time `json:"publish_date,omitempty"`

	// People
	Hosts  []HostBio `json:"hosts"`
	Guests []HostBio `json:"guests,omitempty"`

	// Content
	Description string    `json:"description"`
	Summary     string    `json:"summary"`
	Chapters    []Chapter `json:"chapters"`
	Links       []Link    `json:"links"`

	// Platform links
	SpotifyURL  string `json:"spotify_url,omitempty"`
	ApplePodURL string `json:"apple_pod_url,omitempty"`
	YouTubeURL  string `json:"youtube_url,omitempty"`

	// Sponsorship
	Sponsors []Sponsor `json:"sponsors,omitempty"`
}

// HostBio holds info about a host or guest.
type HostBio struct {
	Name    string `json:"name"`
	Bio     string `json:"bio"`
	Twitter string `json:"twitter,omitempty"`
	GitHub  string `json:"github,omitempty"`
	Website string `json:"website,omitempty"`
}

// Sponsor holds sponsor info.
type Sponsor struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Message string `json:"message"`
}
