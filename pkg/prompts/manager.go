package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/soypete/pedrocli/pkg/config"
)

// Manager handles prompt templates for different modes and job types
type Manager struct {
	config     *config.Config
	promptsDir string
	cache      map[string]string
}

// NewManager creates a new prompt manager
func NewManager(cfg *config.Config) *Manager {
	// Default prompts directory: ~/.pedrocli/prompts/
	home, _ := os.UserHomeDir()
	promptsDir := filepath.Join(home, ".pedrocli", "prompts")

	return &Manager{
		config:     cfg,
		promptsDir: promptsDir,
		cache:      make(map[string]string),
	}
}

// NewManagerWithDir creates a new prompt manager with a custom prompts directory
func NewManagerWithDir(cfg *config.Config, promptsDir string) *Manager {
	return &Manager{
		config:     cfg,
		promptsDir: promptsDir,
		cache:      make(map[string]string),
	}
}

// GetPodcastSystemPrompt returns the system prompt for podcast mode
func (m *Manager) GetPodcastSystemPrompt() string {
	// Try to load from file first
	if prompt, err := m.loadPromptFile("podcast/system.txt"); err == nil {
		return m.renderPrompt(prompt)
	}

	// Fall back to default embedded prompt
	return m.renderPrompt(defaultPodcastSystemPrompt)
}

// GetCodingSystemPrompt returns the system prompt for coding mode
func (m *Manager) GetCodingSystemPrompt() string {
	// Try to load from file first
	if prompt, err := m.loadPromptFile("coding/system.txt"); err == nil {
		return prompt
	}

	// Fall back to default embedded prompt
	return defaultCodingSystemPrompt
}

// GetPrompt returns a prompt for a specific job type
func (m *Manager) GetPrompt(mode, jobType string) string {
	// Try to load from file first
	filename := fmt.Sprintf("%s/%s.txt", mode, jobType)
	if prompt, err := m.loadPromptFile(filename); err == nil {
		return m.renderPrompt(prompt)
	}

	// Fall back to defaults
	if mode == "podcast" {
		if defaultPrompt, ok := defaultPodcastPrompts[jobType]; ok {
			return m.renderPrompt(defaultPrompt)
		}
	}

	if mode == "coding" {
		if defaultPrompt, ok := defaultCodingPrompts[jobType]; ok {
			return defaultPrompt
		}
	}

	return ""
}

// loadPromptFile loads a prompt from a file
func (m *Manager) loadPromptFile(filename string) (string, error) {
	// Check cache
	if cached, ok := m.cache[filename]; ok {
		return cached, nil
	}

	// Load from file
	path := filepath.Join(m.promptsDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	prompt := string(data)
	m.cache[filename] = prompt
	return prompt, nil
}

// renderPrompt renders a prompt template with config values
func (m *Manager) renderPrompt(promptTemplate string) string {
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return promptTemplate
	}

	// Build template data from config
	data := m.buildTemplateData()

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return promptTemplate
	}

	return buf.String()
}

// PromptData contains data for prompt templates
type PromptData struct {
	PodcastName        string
	PodcastDescription string
	PodcastFormat      string
	Cohosts            []config.Cohost
	CohostList         string
	NotionScriptsDB    string
	NotionArticlesDB   string
	NotionNewsDB       string
	NotionGuestsDB     string
	CalendarID         string
	RecordingPlatform  string
}

// buildTemplateData builds template data from config
func (m *Manager) buildTemplateData() PromptData {
	podcast := m.config.Podcast.Metadata

	// Build cohost list string
	var cohostList []string
	for _, cohost := range podcast.Cohosts {
		cohostList = append(cohostList, fmt.Sprintf("- %s (%s): %s", cohost.Name, cohost.Role, cohost.Bio))
	}

	return PromptData{
		PodcastName:        valueOrTODO(podcast.Name, "TODO: Add podcast name"),
		PodcastDescription: valueOrTODO(podcast.Description, "TODO: Add podcast description"),
		PodcastFormat:      valueOrTODO(podcast.Format, "TODO: Add podcast format"),
		Cohosts:            podcast.Cohosts,
		CohostList:         strings.Join(cohostList, "\n"),
		NotionScriptsDB:    valueOrTODO(m.config.Podcast.Notion.Databases.Scripts, "TODO: Add Notion Scripts database ID"),
		NotionArticlesDB:   valueOrTODO(m.config.Podcast.Notion.Databases.ArticlesReview, "TODO: Add Notion Articles database ID"),
		NotionNewsDB:       valueOrTODO(m.config.Podcast.Notion.Databases.NewsReview, "TODO: Add Notion News database ID"),
		NotionGuestsDB:     valueOrTODO(m.config.Podcast.Notion.Databases.Guests, "TODO: Add Notion Guests database ID"),
		CalendarID:         valueOrTODO(m.config.Podcast.Calendar.CalendarID, "TODO: Add Google Calendar ID"),
		RecordingPlatform:  valueOrTODO(podcast.RecordingPlatform, "TODO: Add recording platform"),
	}
}

func valueOrTODO(value, placeholder string) string {
	if value == "" {
		return placeholder
	}
	return value
}
