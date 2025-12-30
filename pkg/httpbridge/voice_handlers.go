package httpbridge

import (
	"fmt"
	"io"
	"net/http"

	"github.com/soypete/pedrocli/pkg/voice"
)

// handleVoiceTranscribe handles POST /api/voice/transcribe
func (s *Server) handleVoiceTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if voice is enabled
	if !s.config.Voice.Enabled {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Voice transcription is not enabled",
		})
		return
	}

	// Parse multipart form (max 10MB audio file)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Failed to parse form: %v", err),
		})
		return
	}

	// Get audio file
	file, header, err := r.FormFile("audio")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Failed to get audio file: %v", err),
		})
		return
	}
	defer file.Close()

	// Read audio data
	audioData, err := io.ReadAll(file)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to read audio data: %v", err),
		})
		return
	}

	// Get audio format from content type or filename
	format := "webm" // Default for browser MediaRecorder
	if contentType := header.Header.Get("Content-Type"); contentType != "" {
		switch contentType {
		case "audio/wav", "audio/wave", "audio/x-wav":
			format = "wav"
		case "audio/mpeg", "audio/mp3":
			format = "mp3"
		case "audio/webm":
			format = "webm"
		case "audio/ogg":
			format = "ogg"
		}
	}

	// Get optional language hint
	language := r.FormValue("language")
	if language == "" {
		language = s.config.Voice.Language
	}

	// Convert audio to WAV if needed (whisper.cpp only accepts WAV)
	if voice.NeedsConversion(format) {
		wavData, err := voice.ConvertToWav(r.Context(), audioData, format)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to convert audio: %v", err),
			})
			return
		}
		audioData = wavData
		format = "wav"
	}

	// Create voice client
	voiceClient := voice.NewClient(s.config.Voice.WhisperURL)

	// Transcribe
	req := voice.TranscribeRequest{
		Audio:    audioData,
		Format:   format,
		Language: language,
	}

	resp, err := voiceClient.Transcribe(r.Context(), req)
	if err != nil || !resp.Success {
		status := http.StatusInternalServerError
		if err != nil {
			respondJSON(w, status, map[string]string{
				"error": resp.Error,
			})
		} else {
			respondJSON(w, status, resp)
		}
		return
	}

	// Return transcription
	respondJSON(w, http.StatusOK, resp)
}

// handleVoiceStatus handles GET /api/voice/status
func (s *Server) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if voice is enabled
	if !s.config.Voice.Enabled {
		respondJSON(w, http.StatusOK, voice.StatusResponse{
			Running: false,
			Error:   "Voice transcription is not enabled in config",
		})
		return
	}

	// Create voice client
	voiceClient := voice.NewClient(s.config.Voice.WhisperURL)

	// Check status
	status, err := voiceClient.Status(r.Context())
	if err != nil {
		respondJSON(w, http.StatusOK, status) // Return status even if error (status contains error message)
		return
	}

	respondJSON(w, http.StatusOK, status)
}
