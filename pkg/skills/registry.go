// Package skills provides OpenCode-inspired skill management for on-demand loading.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Skill represents a loadable skill with instructions
type Skill struct {
	Name        string
	Description string
	Content     string // Full skill content
	Path        string // Path to SKILL.md file
	Category    string // Optional category
	Tags        []string
}

// SkillRegistry manages skill discovery and loading
type SkillRegistry struct {
	mu          sync.RWMutex
	skills      map[string]*Skill
	searchPaths []string
}

// NewSkillRegistry creates a new skill registry
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills:      make(map[string]*Skill),
		searchPaths: make([]string, 0),
	}
}

// AddSearchPath adds a path to search for skills
func (r *SkillRegistry) AddSearchPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.searchPaths = append(r.searchPaths, path)
}

// Discover scans all search paths for skills
func (r *SkillRegistry) Discover() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, basePath := range r.searchPaths {
		if err := r.discoverInPath(basePath); err != nil {
			// Continue discovering in other paths even if one fails
			continue
		}
	}

	return nil
}

// discoverInPath discovers skills in a single path
func (r *SkillRegistry) discoverInPath(basePath string) error {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return err // Path doesn't exist or is not readable
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(basePath, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue // No SKILL.md in this directory
		}

		skill, err := r.loadSkill(skillPath, entry.Name())
		if err != nil {
			continue // Skip invalid skill files
		}

		r.skills[skill.Name] = skill
	}

	return nil
}

// loadSkill loads a skill from a SKILL.md file
func (r *SkillRegistry) loadSkill(path, defaultName string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	frontmatter, body := parseFrontmatter(string(content))

	var fm SkillFrontmatter
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
			return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	// Default name from directory
	if fm.Name == "" {
		fm.Name = defaultName
	}

	return &Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Content:     body,
		Path:        path,
		Category:    fm.Category,
		Tags:        fm.Tags,
	}, nil
}

// SkillFrontmatter represents the YAML frontmatter in SKILL.md files
type SkillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Category    string   `yaml:"category"`
	Tags        []string `yaml:"tags"`
}

// parseFrontmatter extracts YAML frontmatter and body from markdown content
func parseFrontmatter(content string) (frontmatter, body string) {
	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\n(.*)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	return "", content
}

// Get returns a skill by name
func (r *SkillRegistry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[name]
	return skill, ok
}

// Load loads a skill's content (re-reads from file if needed)
func (r *SkillRegistry) Load(name string) (string, error) {
	r.mu.RLock()
	skill, ok := r.skills[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	// Optionally re-read from file to get latest content
	if skill.Path != "" {
		content, err := os.ReadFile(skill.Path)
		if err != nil {
			// Fall back to cached content
			return skill.Content, nil
		}
		_, body := parseFrontmatter(string(content))
		return body, nil
	}

	return skill.Content, nil
}

// List returns all discovered skills
func (r *SkillRegistry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		result = append(result, skill)
	}
	return result
}

// ListByCategory returns skills filtered by category
func (r *SkillRegistry) ListByCategory(category string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Skill, 0)
	for _, skill := range r.skills {
		if skill.Category == category {
			result = append(result, skill)
		}
	}
	return result
}

// Search returns skills matching a query (name, description, or tags)
func (r *SkillRegistry) Search(query string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	result := make([]*Skill, 0)

	for _, skill := range r.skills {
		// Check name
		if strings.Contains(strings.ToLower(skill.Name), query) {
			result = append(result, skill)
			continue
		}
		// Check description
		if strings.Contains(strings.ToLower(skill.Description), query) {
			result = append(result, skill)
			continue
		}
		// Check tags
		for _, tag := range skill.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				result = append(result, skill)
				break
			}
		}
	}

	return result
}

// Register manually registers a skill
func (r *SkillRegistry) Register(skill *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name] = skill
}

// GetToolDescription generates a tool description listing available skills
func (r *SkillRegistry) GetToolDescription() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for name, skill := range r.skills {
		sb.WriteString("<skill>\n")
		sb.WriteString(fmt.Sprintf("<name>%s</name>\n", name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("<description>%s</description>\n", skill.Description))
		}
		if skill.Category != "" {
			sb.WriteString(fmt.Sprintf("<category>%s</category>\n", skill.Category))
		}
		sb.WriteString("</skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

// DiscoverWithDefaults sets up default search paths and discovers skills
func (r *SkillRegistry) DiscoverWithDefaults(workDir string) error {
	// Add standard search paths
	paths := []string{
		filepath.Join(workDir, ".pedro", "skill"),
		filepath.Join(workDir, ".claude", "skills"), // Claude compatibility
	}

	// Add global user path
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".config", "pedrocli", "skill"))
	}

	for _, path := range paths {
		r.AddSearchPath(path)
	}

	return r.Discover()
}
