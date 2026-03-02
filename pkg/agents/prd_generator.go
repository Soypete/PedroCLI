package agents

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/llm"
)

const prdGenerationSystemPrompt = `You are a technical project planner. Your job is to take a natural language description
and produce a structured PRD (Product Requirements Document) in JSON format.

The PRD must follow this exact JSON schema:

{
  "projectName": "short-kebab-case-name",
  "mode": "<mode>",
  "outputFile": "path/to/output.md",
  "userStories": [
    {
      "id": "<PREFIX>-001",
      "title": "Short title",
      "description": "Detailed description of what needs to be done",
      "acceptanceCriteria": ["criterion 1", "criterion 2"],
      "priority": 1,
      "passes": false
    }
  ]
}

Rules:
- mode must be one of: code, blog, podcast
- For code mode: use "US-" prefix for IDs, omit outputFile
- For blog mode: use "BLOG-" prefix for IDs, set outputFile to a .md path
- For podcast mode: use "POD-" prefix for IDs, set outputFile to a .md path
- Create 3-8 user stories, ordered by priority (1 = highest)
- Each story should be independently completable
- Each story should have 2-4 acceptance criteria
- All stories start with "passes": false
- Stories should flow logically: earlier stories build foundations for later ones
- For blog/podcast: include a revision pass as the final story
- Keep descriptions actionable and specific

Output ONLY the JSON. No markdown fences, no explanation, just valid JSON.`

// GeneratePRD uses the LLM to generate a PRD from a natural language description.
func GeneratePRD(ctx context.Context, backend llm.Backend, prompt string, mode PRDMode) (*PRD, error) {
	if backend == nil {
		return nil, fmt.Errorf("LLM backend is required for PRD generation")
	}

	userPrompt := fmt.Sprintf("Generate a PRD in %s mode for the following:\n\n%s", mode, prompt)

	resp, err := backend.Infer(ctx, &llm.InferenceRequest{
		SystemPrompt: prdGenerationSystemPrompt,
		UserPrompt:   userPrompt,
		Temperature:  0.3,
		MaxTokens:    4096,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM inference failed: %w", err)
	}

	// Extract JSON from response (strip markdown fences if the model wraps them)
	jsonStr := extractJSONFromCodeBlock(resp.Text)

	prd, err := ParsePRD([]byte(jsonStr))
	if err != nil {
		return nil, fmt.Errorf("LLM produced invalid PRD: %w\nRaw output:\n%s", err, jsonStr)
	}

	// Override mode in case the LLM got it wrong
	prd.Mode = mode

	return prd, nil
}
