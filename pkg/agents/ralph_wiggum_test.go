package agents

import (
	"strings"
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestNewRalphWiggumAgent(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			ModelName:   "test-model",
			ContextSize: 4096,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 10,
		},
	}

	prd := &PRD{
		ProjectName: "test-project",
		Mode:        PRDModeCode,
		UserStories: []UserStory{
			{ID: "US-001", Title: "Add feature", Priority: 1, Passes: false},
			{ID: "US-002", Title: "Add tests", Priority: 2, Passes: true},
		},
	}

	agent := NewRalphWiggumAgent(RalphWiggumConfig{
		Config:        cfg,
		Backend:       nil, // No LLM for unit tests
		JobManager:    nil,
		PRD:           prd,
		MaxIterations: 5,
	})

	if agent == nil {
		t.Fatal("expected non-nil agent")
	}

	if agent.Name() != "ralph_wiggum" {
		t.Errorf("expected name 'ralph_wiggum', got %q", agent.Name())
	}

	if agent.maxIterations != 5 {
		t.Errorf("expected max iterations 5, got %d", agent.maxIterations)
	}

	// Check progress tracker was initialized
	phases := agent.GetProgress().GetPhases()
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases in progress tracker, got %d", len(phases))
	}

	// US-002 should be done (Passes: true)
	us002Phase := agent.GetProgress().GetPhase("US-002")
	if us002Phase == nil {
		t.Fatal("expected US-002 phase in tracker")
	}
	if us002Phase.Status != PhaseStatusDone {
		t.Errorf("expected US-002 to be done, got %s", us002Phase.Status)
	}
}

func TestNewRalphWiggumAgentDefaultIterations(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			ModelName:   "test-model",
			ContextSize: 4096,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 10,
		},
	}

	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeBlog,
		UserStories: []UserStory{
			{ID: "B-001", Title: "Write", Priority: 1},
		},
	}

	agent := NewRalphWiggumAgent(RalphWiggumConfig{
		Config:        cfg,
		Backend:       nil,
		PRD:           prd,
		MaxIterations: 0, // Should default to 10
	})

	if agent.maxIterations != 10 {
		t.Errorf("expected default max iterations 10, got %d", agent.maxIterations)
	}
}

func TestRalphWiggumBuildSystemPrompt(t *testing.T) {
	tests := []struct {
		mode     PRDMode
		contains []string
	}{
		{
			mode: PRDModeCode,
			contains: []string{
				"Ralph Wiggum",
				"Code Mode",
				"Quality Checks",
				"Commit Standards",
			},
		},
		{
			mode: PRDModeBlog,
			contains: []string{
				"Ralph Wiggum",
				"Blog Mode",
				"Writing Standards",
				"practitioner-to-practitioner",
			},
		},
		{
			mode: PRDModePodcast,
			contains: []string{
				"Ralph Wiggum",
				"Podcast Mode",
				"Script Standards",
				"escape hatches",
			},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			cfg := &config.Config{
				Model: config.ModelConfig{
					ModelName:   "test-model",
					ContextSize: 4096,
				},
				Limits: config.LimitsConfig{
					MaxInferenceRuns: 10,
				},
			}

			prd := &PRD{
				ProjectName: "test",
				Mode:        tt.mode,
				UserStories: []UserStory{
					{ID: "T-001", Title: "Test", Priority: 1},
				},
			}

			agent := NewRalphWiggumAgent(RalphWiggumConfig{
				Config:  cfg,
				Backend: nil,
				PRD:     prd,
			})

			prompt := agent.buildSystemPrompt()

			for _, expected := range tt.contains {
				if !strings.Contains(prompt, expected) {
					t.Errorf("expected system prompt to contain %q for mode %s", expected, tt.mode)
				}
			}
		})
	}
}

func TestRalphWiggumBuildStoryPrompt(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			ModelName:   "test-model",
			ContextSize: 4096,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 10,
		},
	}

	prd := &PRD{
		ProjectName: "my-project",
		Mode:        PRDModeCode,
		OutputFile:  "output.txt",
		UserStories: []UserStory{
			{
				ID:                 "US-001",
				Title:              "Add health endpoint",
				Description:        "Add GET /health that returns 200",
				AcceptanceCriteria: []string{"Returns 200", "Has JSON body"},
				Priority:           1,
				Passes:             false,
			},
			{
				ID:       "US-002",
				Title:    "Add logging",
				Priority: 2,
				Passes:   true,
			},
		},
	}

	agent := NewRalphWiggumAgent(RalphWiggumConfig{
		Config:  cfg,
		Backend: nil,
		PRD:     prd,
	})

	story := &prd.UserStories[0]
	prompt := agent.buildStoryPrompt(story, 3)

	// Check essential elements are present
	checks := []string{
		"my-project",
		"output.txt",
		"Iteration 3",
		"US-001",
		"Add health endpoint",
		"Add GET /health that returns 200",
		"Returns 200",
		"Has JSON body",
		"[DONE] US-002",
		"[TODO] US-001",
		"STORY_COMPLETE",
		"feat(US-001)",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("expected story prompt to contain %q", check)
		}
	}
}

func TestRalphWiggumBuildStoryPromptWithLearnings(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			ModelName:   "test-model",
			ContextSize: 4096,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 10,
		},
	}

	prd := &PRD{
		ProjectName: "test",
		Mode:        PRDModeBlog,
		UserStories: []UserStory{
			{ID: "B-001", Title: "Write intro", Priority: 1},
		},
	}

	agent := NewRalphWiggumAgent(RalphWiggumConfig{
		Config:  cfg,
		Backend: nil,
		PRD:     prd,
	})

	// Add some learnings
	agent.learnings = []string{
		"--- Iteration 1: B-001 ---\n[Status: Incomplete]",
		"--- Iteration 2: B-001 ---\n[Status: Complete]",
	}

	prompt := agent.buildStoryPrompt(&prd.UserStories[0], 3)

	if !strings.Contains(prompt, "Previous Iterations") {
		t.Error("expected prompt to contain previous iterations section")
	}
	if !strings.Contains(prompt, "Iteration 1") {
		t.Error("expected prompt to contain learning from iteration 1")
	}
	if !strings.Contains(prompt, "Iteration 2") {
		t.Error("expected prompt to contain learning from iteration 2")
	}
}

func TestRalphWiggumBlogModePrompt(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			ModelName:   "test-model",
			ContextSize: 4096,
		},
		Limits: config.LimitsConfig{
			MaxInferenceRuns: 10,
		},
	}

	prd := &PRD{
		ProjectName: "blog-post",
		Mode:        PRDModeBlog,
		OutputFile:  "content/post.md",
		UserStories: []UserStory{
			{ID: "BLOG-001", Title: "Write intro", Priority: 1},
		},
	}

	agent := NewRalphWiggumAgent(RalphWiggumConfig{
		Config:  cfg,
		Backend: nil,
		PRD:     prd,
	})

	prompt := agent.buildStoryPrompt(&prd.UserStories[0], 1)

	// Blog mode should say "Save your work" not "Commit"
	if strings.Contains(prompt, "feat(BLOG-001)") {
		t.Error("blog mode should not have commit instructions")
	}
	if !strings.Contains(prompt, "Save your work") {
		t.Error("blog mode should have save instructions")
	}
}
