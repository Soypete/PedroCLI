package httpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/storage/podcast"
	"github.com/soypete/pedrocli/pkg/voice"
)

// PodcastEpisodeResponse represents an episode in API responses.
type PodcastEpisodeResponse struct {
	ID            string                `json:"id"`
	EpisodeNumber string                `json:"episode_number"`
	Title         string                `json:"title"`
	RecordDate    string                `json:"record_date"`
	Status        podcast.EpisodeStatus `json:"status"`
	Transcript    string                `json:"transcript,omitempty"`
	FactChecks    []podcast.FactCheck   `json:"fact_checks,omitempty"`
	ShowNotes     *podcast.ShowNotes    `json:"show_notes,omitempty"`
	Template      *podcast.NoteTemplate `json:"template,omitempty"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// handlePodcastUpload handles POST /api/podcast/upload
// Accepts multipart form with MP4 file + episode metadata.
// Uploads to S3, transcribes via whisper.cpp, saves transcript to S3.
func (s *Server) handlePodcastUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 500MB for video files)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to parse form: %v", err),
		})
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "File is required",
		})
		return
	}
	defer file.Close()

	// Get metadata from form
	episodeNumber := r.FormValue("episode_number")
	title := r.FormValue("title")
	recordDateStr := r.FormValue("record_date")

	if title == "" {
		title = "Untitled Episode"
	}

	var recordDate time.Time
	if recordDateStr != "" {
		recordDate, err = time.Parse("2006-01-02", recordDateStr)
		if err != nil {
			recordDate = time.Now()
		}
	} else {
		recordDate = time.Now()
	}

	// Create episode record
	episodeID := uuid.New().String()
	ep := &podcast.Episode{
		ID:            episodeID,
		EpisodeNumber: episodeNumber,
		Title:         title,
		RecordDate:    recordDate,
		Status:        podcast.StatusUploaded,
	}

	if s.appCtx.PodcastStore == nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Podcast store not configured",
		})
		return
	}

	if err := s.appCtx.PodcastStore.CreateEpisode(r.Context(), ep); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to create episode: %v", err),
		})
		return
	}

	// Upload to S3 if configured
	recordingKey := fmt.Sprintf("episodes/%s/recording%s", episodeID, fileExtension(header.Filename))
	if s.appCtx.S3Client != nil {
		_, err = s.appCtx.S3Client.Upload(r.Context(), recordingKey, file, header.Header.Get("Content-Type"))
		if err != nil {
			log.Printf("Warning: S3 upload failed: %v (continuing with transcription)", err)
		} else {
			ep.RecordingKey = recordingKey
		}
	}

	// Reset file reader for transcription
	file.Seek(0, io.SeekStart)

	// Start transcription in background
	ep.Status = podcast.StatusTranscribing
	_ = s.appCtx.PodcastStore.UpdateEpisode(r.Context(), ep)

	go s.transcribeEpisode(episodeID, file, header.Filename)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"episode_id": episodeID,
		"message":    "Episode uploaded, transcription started",
	})
}

// transcribeEpisode runs whisper.cpp transcription in the background.
func (s *Server) transcribeEpisode(episodeID string, audioReader io.ReadSeeker, filename string) {
	ctx := context.Background()

	ep, err := s.appCtx.PodcastStore.GetEpisode(ctx, episodeID)
	if err != nil {
		log.Printf("Transcription error: episode not found: %v", err)
		return
	}

	// Read audio data
	audioData, err := io.ReadAll(audioReader)
	if err != nil {
		log.Printf("Transcription error: failed to read audio: %v", err)
		ep.Status = podcast.StatusUploaded
		_ = s.appCtx.PodcastStore.UpdateEpisode(ctx, ep)
		return
	}

	// Determine format from filename
	format := "mp4"
	if ext := fileExtension(filename); ext != "" {
		format = strings.TrimPrefix(ext, ".")
	}

	// Convert to WAV if needed (whisper.cpp needs WAV)
	if voice.NeedsConversion(format) {
		audioData, err = voice.ConvertToWav(ctx, audioData, format)
		if err != nil {
			log.Printf("Transcription error: audio conversion failed: %v", err)
			ep.Status = podcast.StatusUploaded
			_ = s.appCtx.PodcastStore.UpdateEpisode(ctx, ep)
			return
		}
		format = "wav"
	}

	// Send to whisper.cpp
	if s.appCtx.VoiceClient == nil {
		log.Printf("Transcription error: whisper client not configured")
		ep.Status = podcast.StatusUploaded
		_ = s.appCtx.PodcastStore.UpdateEpisode(ctx, ep)
		return
	}

	resp, err := s.appCtx.VoiceClient.Transcribe(ctx, voice.TranscribeRequest{
		Audio:    audioData,
		Format:   format,
		Language: s.config.Voice.Language,
	})
	if err != nil || !resp.Success {
		errMsg := "unknown error"
		if err != nil {
			errMsg = err.Error()
		} else if resp != nil {
			errMsg = resp.Error
		}
		log.Printf("Transcription error: %s", errMsg)
		ep.Status = podcast.StatusUploaded
		_ = s.appCtx.PodcastStore.UpdateEpisode(ctx, ep)
		return
	}

	// Save transcript
	ep.Transcript = resp.Text
	ep.Status = podcast.StatusTranscribed
	_ = s.appCtx.PodcastStore.UpdateEpisode(ctx, ep)

	// Persist to S3
	if memStore, ok := s.appCtx.PodcastStore.(*podcast.MemoryStore); ok {
		if err := memStore.SaveTranscriptToS3(ctx, episodeID, resp.Text); err != nil {
			log.Printf("Warning: failed to save transcript to S3: %v", err)
		}
	}

	log.Printf("Transcription complete for episode %s (%d chars, %dms)", episodeID, len(resp.Text), resp.ProcessingTimeMs)
}

// handlePodcastEpisodes handles GET /api/podcast/episodes
func (s *Server) handlePodcastEpisodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.appCtx.PodcastStore == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"episodes": []interface{}{},
			"total":    0,
		})
		return
	}

	episodes, err := s.appCtx.PodcastStore.ListEpisodes(r.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": fmt.Sprintf("Failed to list episodes: %v", err),
		})
		return
	}

	responses := make([]PodcastEpisodeResponse, len(episodes))
	for i, ep := range episodes {
		responses[i] = episodeToResponse(ep)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"episodes": responses,
		"total":    len(responses),
	})
}

// handlePodcastEpisodeByID handles GET /api/podcast/episodes/:id
func (s *Server) handlePodcastEpisodeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/podcast/episodes/")
	id = strings.Split(id, "/")[0]
	if id == "" {
		http.Error(w, "Episode ID required", http.StatusBadRequest)
		return
	}

	if s.appCtx.PodcastStore == nil {
		http.Error(w, "Podcast store not configured", http.StatusInternalServerError)
		return
	}

	ep, err := s.appCtx.PodcastStore.GetEpisode(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Episode not found: %v", err), http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, episodeToResponse(ep))
}

// handlePodcastEpisodeFactChecks handles PUT /api/podcast/episodes/:id/fact-checks
func (s *Server) handlePodcastEpisodeFactChecks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractEpisodeID(r.URL.Path)
	if id == "" {
		http.Error(w, "Episode ID required", http.StatusBadRequest)
		return
	}

	var factChecks []podcast.FactCheck
	if err := json.NewDecoder(r.Body).Decode(&factChecks); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	ep, err := s.appCtx.PodcastStore.GetEpisode(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Episode not found: %v", err), http.StatusNotFound)
		return
	}

	ep.FactChecks = factChecks
	if ep.Status == podcast.StatusTranscribed || ep.Status == podcast.StatusFactChecking {
		ep.Status = podcast.StatusFactChecked
	}

	if err := s.appCtx.PodcastStore.UpdateEpisode(r.Context(), ep); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to update episode: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Fact checks saved",
	})
}

// handlePodcastEpisodeShowNotes handles PUT /api/podcast/episodes/:id/show-notes
func (s *Server) handlePodcastEpisodeShowNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractEpisodeID(r.URL.Path)
	if id == "" {
		http.Error(w, "Episode ID required", http.StatusBadRequest)
		return
	}

	var showNotes podcast.ShowNotes
	if err := json.NewDecoder(r.Body).Decode(&showNotes); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	ep, err := s.appCtx.PodcastStore.GetEpisode(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Episode not found: %v", err), http.StatusNotFound)
		return
	}

	ep.ShowNotes = &showNotes
	if ep.Status == podcast.StatusFactChecked || ep.Status == podcast.StatusShowNotes {
		ep.Status = podcast.StatusShowNotesDone
	}

	if err := s.appCtx.PodcastStore.UpdateEpisode(r.Context(), ep); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to update episode: %v", err),
		})
		return
	}

	// Persist to S3
	if memStore, ok := s.appCtx.PodcastStore.(*podcast.MemoryStore); ok {
		if err := memStore.SaveShowNotesToS3(r.Context(), id, &showNotes); err != nil {
			log.Printf("Warning: failed to save show notes to S3: %v", err)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Show notes saved",
	})
}

// handlePodcastEpisodeTemplate handles GET /api/podcast/episodes/:id/template
func (s *Server) handlePodcastEpisodeTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractEpisodeID(r.URL.Path)
	if id == "" {
		http.Error(w, "Episode ID required", http.StatusBadRequest)
		return
	}

	ep, err := s.appCtx.PodcastStore.GetEpisode(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Episode not found: %v", err), http.StatusNotFound)
		return
	}

	// Build template from episode data + podcast metadata config
	meta := s.config.Podcast.Metadata
	tmpl := podcast.NoteTemplate{
		EpisodeNumber: ep.EpisodeNumber,
		Title:         ep.Title,
		RecordDate:    ep.RecordDate,
	}

	// Populate hosts from config
	for _, cohost := range meta.Cohosts {
		tmpl.Hosts = append(tmpl.Hosts, podcast.HostBio{
			Name:    cohost.Name,
			Bio:     cohost.Bio,
			Twitter: findSocialLink(cohost.SocialLinks, "twitter"),
			GitHub:  findSocialLink(cohost.SocialLinks, "github"),
			Website: findSocialLink(cohost.SocialLinks, "website"),
		})
	}

	// Populate from show notes if available
	if ep.ShowNotes != nil {
		tmpl.Summary = ep.ShowNotes.Summary
		tmpl.Chapters = ep.ShowNotes.Chapters
		tmpl.Links = ep.ShowNotes.Links
		tmpl.Description = ep.ShowNotes.Summary
	}

	// Populate sponsors from config
	if meta.SponsorInfo != "" {
		tmpl.Sponsors = append(tmpl.Sponsors, podcast.Sponsor{
			Name:    "Sponsor",
			Message: meta.SponsorInfo,
			URL:     meta.SponsorLinks,
		})
	}

	respondJSON(w, http.StatusOK, tmpl)
}

// handlePodcastEpisodeStatus handles PUT /api/podcast/episodes/:id/status
func (s *Server) handlePodcastEpisodeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractEpisodeID(r.URL.Path)
	if id == "" {
		http.Error(w, "Episode ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Status podcast.EpisodeStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	ep, err := s.appCtx.PodcastStore.GetEpisode(r.Context(), id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Episode not found: %v", err), http.StatusNotFound)
		return
	}

	ep.Status = req.Status
	if err := s.appCtx.PodcastStore.UpdateEpisode(r.Context(), ep); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to update status: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  ep.Status,
	})
}

// handlePodcastEpisodePage serves the episode review page
func (s *Server) handlePodcastEpisodePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data := map[string]interface{}{
		"title": "Podcast Episode - PedroCLI",
	}

	if err := s.templates.ExecuteTemplate(w, "podcast_episode.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Helper functions

func episodeToResponse(ep *podcast.Episode) PodcastEpisodeResponse {
	return PodcastEpisodeResponse{
		ID:            ep.ID,
		EpisodeNumber: ep.EpisodeNumber,
		Title:         ep.Title,
		RecordDate:    ep.RecordDate.Format("2006-01-02"),
		Status:        ep.Status,
		Transcript:    ep.Transcript,
		FactChecks:    ep.FactChecks,
		ShowNotes:     ep.ShowNotes,
		Template:      ep.Template,
		CreatedAt:     ep.CreatedAt,
		UpdatedAt:     ep.UpdatedAt,
	}
}

func extractEpisodeID(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/api/podcast/episodes/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func fileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		return "." + parts[len(parts)-1]
	}
	return ""
}

func findSocialLink(links []string, platform string) string {
	for _, link := range links {
		if strings.Contains(strings.ToLower(link), platform) {
			return link
		}
	}
	return ""
}

