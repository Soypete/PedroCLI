package voice

// TranscribeRequest represents a request to transcribe audio
type TranscribeRequest struct {
	// Audio data (base64 encoded or raw bytes)
	Audio []byte `json:"audio"`

	// Audio format (e.g., "wav", "mp3", "webm")
	Format string `json:"format,omitempty"`

	// Language hint (e.g., "en", "es", "auto")
	Language string `json:"language,omitempty"`

	// Optional prompt to guide transcription
	Prompt string `json:"prompt,omitempty"`
}

// TranscribeResponse represents the response from transcription
type TranscribeResponse struct {
	// Transcribed text
	Text string `json:"text"`

	// Language detected
	Language string `json:"language,omitempty"`

	// Processing time in milliseconds
	ProcessingTimeMs int64 `json:"processing_time_ms,omitempty"`

	// Confidence score (0-1)
	Confidence float64 `json:"confidence,omitempty"`

	// Error message if transcription failed
	Error string `json:"error,omitempty"`

	// Success flag
	Success bool `json:"success"`
}

// StatusResponse represents whisper.cpp server health status
type StatusResponse struct {
	// Whether the server is running
	Running bool `json:"running"`

	// Server version
	Version string `json:"version,omitempty"`

	// Model loaded
	Model string `json:"model,omitempty"`

	// Error message if any
	Error string `json:"error,omitempty"`
}
