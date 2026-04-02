package artifacts

import "time"

type ArtifactType string

const (
	ArtifactRepoMap     ArtifactType = "repo_map"
	ArtifactTask        ArtifactType = "task"
	ArtifactPlan        ArtifactType = "plan"
	ArtifactDiff        ArtifactType = "diff"
	ArtifactTestResults ArtifactType = "test_results"
	ArtifactReview      ArtifactType = "review"
	ArtifactOutput      ArtifactType = "output"
	ArtifactContext     ArtifactType = "context"
	ArtifactError       ArtifactType = "error"
)

type Artifact struct {
	ID        string       `json:"id"`
	Type      ArtifactType `json:"type"`
	Name      string       `json:"name"`
	Content   string       `json:"content,omitempty"`
	Size      int64        `json:"size"`
	CreatedBy string       `json:"created_by"`
	CreatedAt time.Time    `json:"created_at"`
}

type ArtifactFilter struct {
	Type      ArtifactType `json:"type,omitempty"`
	Name      string       `json:"name,omitempty"`
	CreatedBy string       `json:"created_by,omitempty"`
	Since     time.Time    `json:"since,omitempty"`
}

func NewArtifact(id string, artifactType ArtifactType, name, content, createdBy string) *Artifact {
	return &Artifact{
		ID:        id,
		Type:      artifactType,
		Name:      name,
		Content:   content,
		Size:      int64(len(content)),
		CreatedBy: createdBy,
		CreatedAt: time.Now(),
	}
}

func (a *Artifact) IsValid() bool {
	return a.ID != "" && a.Type != "" && a.Name != ""
}
