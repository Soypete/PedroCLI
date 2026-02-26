package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// PRDMode represents the type of work being done
type PRDMode string

const (
	PRDModeCode    PRDMode = "code"
	PRDModeBlog    PRDMode = "blog"
	PRDModePodcast PRDMode = "podcast"
)

// PRD represents a Product Requirements Document with user stories
type PRD struct {
	ProjectName string      `json:"projectName"`
	BranchName  string      `json:"branchName,omitempty"`
	Mode        PRDMode     `json:"mode"`
	OutputFile  string      `json:"outputFile,omitempty"`
	UserStories []UserStory `json:"userStories"`
}

// UserStory represents a single unit of work in the PRD
type UserStory struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	Priority           int      `json:"priority"`
	Passes             bool     `json:"passes"`
}

// LoadPRD loads a PRD from a JSON file
func LoadPRD(path string) (*PRD, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRD file: %w", err)
	}

	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, fmt.Errorf("failed to parse PRD: %w", err)
	}

	if err := prd.Validate(); err != nil {
		return nil, err
	}

	return &prd, nil
}

// ParsePRD parses a PRD from JSON bytes
func ParsePRD(data []byte) (*PRD, error) {
	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, fmt.Errorf("failed to parse PRD: %w", err)
	}

	if err := prd.Validate(); err != nil {
		return nil, err
	}

	return &prd, nil
}

// Validate checks the PRD for correctness
func (p *PRD) Validate() error {
	if p.ProjectName == "" {
		return fmt.Errorf("PRD projectName is required")
	}
	if p.Mode == "" {
		return fmt.Errorf("PRD mode is required")
	}
	if p.Mode != PRDModeCode && p.Mode != PRDModeBlog && p.Mode != PRDModePodcast {
		return fmt.Errorf("PRD mode must be one of: code, blog, podcast (got %q)", p.Mode)
	}
	if len(p.UserStories) == 0 {
		return fmt.Errorf("PRD must have at least one user story")
	}

	ids := make(map[string]bool)
	for _, story := range p.UserStories {
		if story.ID == "" {
			return fmt.Errorf("user story must have an ID")
		}
		if ids[story.ID] {
			return fmt.Errorf("duplicate story ID: %s", story.ID)
		}
		ids[story.ID] = true

		if story.Title == "" {
			return fmt.Errorf("story %s must have a title", story.ID)
		}
	}

	return nil
}

// IncompleteStories returns stories that haven't passed yet, sorted by priority
func (p *PRD) IncompleteStories() []UserStory {
	var incomplete []UserStory
	for _, s := range p.UserStories {
		if !s.Passes {
			incomplete = append(incomplete, s)
		}
	}
	sort.Slice(incomplete, func(i, j int) bool {
		return incomplete[i].Priority < incomplete[j].Priority
	})
	return incomplete
}

// NextStory returns the next incomplete story by priority, or nil if all complete
func (p *PRD) NextStory() *UserStory {
	incomplete := p.IncompleteStories()
	if len(incomplete) == 0 {
		return nil
	}
	return &incomplete[0]
}

// MarkStoryComplete marks a story as passing
func (p *PRD) MarkStoryComplete(storyID string) bool {
	for i := range p.UserStories {
		if p.UserStories[i].ID == storyID {
			p.UserStories[i].Passes = true
			return true
		}
	}
	return false
}

// AllComplete returns true if all stories pass
func (p *PRD) AllComplete() bool {
	for _, s := range p.UserStories {
		if !s.Passes {
			return false
		}
	}
	return true
}

// CompletedCount returns the number of completed stories
func (p *PRD) CompletedCount() int {
	count := 0
	for _, s := range p.UserStories {
		if s.Passes {
			count++
		}
	}
	return count
}

// StatusSummary returns a human-readable status of all stories
func (p *PRD) StatusSummary() string {
	var summary string
	for _, s := range p.UserStories {
		status := "TODO"
		if s.Passes {
			status = "DONE"
		}
		summary += fmt.Sprintf("  [%s] %s: %s\n", status, s.ID, s.Title)
	}
	return summary
}

// SavePRD writes the PRD to a JSON file
func (p *PRD) SavePRD(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRD: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ToJSON serializes the PRD to JSON bytes
func (p *PRD) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
