package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/soypete/pedrocli/pkg/db"
)

// Generator orchestrates artifact generation for documents.
type Generator struct {
	llm *LlamaClient
	db  *db.DB
}

// NewGenerator creates a Generator.
func NewGenerator(llm *LlamaClient, database *db.DB) *Generator {
	return &Generator{llm: llm, db: database}
}

// perItemTypes are the artifact types generated for each individual document.
var perItemTypes = []struct {
	artType    db.ArtifactType
	promptFunc func(string) (string, string)
}{
	{db.ArtifactTypeSummary, SummaryPrompts},
	{db.ArtifactTypeFaq, FAQPrompts},
	{db.ArtifactTypeStudyGuide, StudyGuidePrompts},
	{db.ArtifactTypeTimeline, TimelinePrompts},
}

// GenerateAll runs all four per-item artifact types for the given doc,
// parses the JSON responses, and upserts them to the artifacts table.
func (g *Generator) GenerateAll(ctx context.Context, docID uuid.UUID) error {
	doc, err := g.db.GetDoc(ctx, docID)
	if err != nil {
		return fmt.Errorf("get doc: %w", err)
	}

	sourceText := doc.RawContent
	if sourceText == "" {
		return fmt.Errorf("doc %s has empty content", docID)
	}

	for _, pt := range perItemTypes {
		log.Printf("artifacts: generating %s for doc %s", pt.artType, docID)
		sysPr, userPr := pt.promptFunc(sourceText)

		resp, usage, err := g.llm.Complete(ctx, sysPr, userPr)
		if err != nil {
			return fmt.Errorf("generate %s: %w", pt.artType, err)
		}

		content := json.RawMessage(resp)
		// Validate JSON — if the model returned non-JSON, wrap it.
		if !json.Valid(content) {
			wrapped, _ := json.Marshal(map[string]string{"text": resp})
			content = wrapped
		}

		_, upsertErr := g.db.UpsertArtifact(ctx, db.UpsertArtifactParams{
			DocID:         &docID,
			ArtifactType:  pt.artType,
			PromptVersion: "v1",
			Content:       content,
			Model:         g.llm.Model,
			InputTokens:   int32(usage.PromptTokens),
			OutputTokens:  int32(usage.CompletionTokens),
		})
		if upsertErr != nil {
			return fmt.Errorf("upsert %s: %w", pt.artType, upsertErr)
		}
	}

	return nil
}
