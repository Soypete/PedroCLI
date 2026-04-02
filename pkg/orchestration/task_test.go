package orchestration

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTaskEnvelope_Serialization(t *testing.T) {
	envelope := &TaskEnvelope{
		ID:    "test-123",
		Agent: "builder",
		Goal:  "Add a new feature",
		Mode:  ModeCode,
		Context: TaskContext{
			Workspace:  "/workspace/project",
			WorkingDir: "/workspace/project",
			Files:      []string{"main.go", "utils.go"},
			Metadata:   map[string]interface{}{"priority": "high"},
		},
		ToolsAllowed: []string{"file", "code_edit", "search"},
		MaxSteps:     20,
		ReturnSchema: map[string]string{
			"files_modified": "[]string",
			"success":        "bool",
		},
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	decoded := &TaskEnvelope{}
	if err := json.Unmarshal(data, decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Agent != envelope.Agent {
		t.Errorf("expected Agent %q, got %q", envelope.Agent, decoded.Agent)
	}
	if decoded.Goal != envelope.Goal {
		t.Errorf("expected Goal %q, got %q", envelope.Goal, decoded.Goal)
	}
	if decoded.Mode != envelope.Mode {
		t.Errorf("expected Mode %v, got %v", envelope.Mode, decoded.Mode)
	}
	if len(decoded.Context.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(decoded.Context.Files))
	}
}

func TestTaskEnvelope_Validation(t *testing.T) {
	tests := []struct {
		name     string
		envelope *TaskEnvelope
		wantErr  bool
	}{
		{
			name: "valid envelope",
			envelope: &TaskEnvelope{
				Agent: "builder",
				Goal:  "Add feature",
				Context: TaskContext{
					Workspace: "/workspace",
				},
			},
			wantErr: false,
		},
		{
			name: "missing agent",
			envelope: &TaskEnvelope{
				Goal: "Add feature",
				Context: TaskContext{
					Workspace: "/workspace",
				},
			},
			wantErr: true,
		},
		{
			name: "missing goal",
			envelope: &TaskEnvelope{
				Agent: "builder",
				Context: TaskContext{
					Workspace: "/workspace",
				},
			},
			wantErr: true,
		},
		{
			name: "missing workspace",
			envelope: &TaskEnvelope{
				Agent: "builder",
				Goal:  "Add feature",
				Context: TaskContext{
					Workspace: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.envelope.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTaskContext_Workspace(t *testing.T) {
	ctx := TaskContext{
		Workspace:  "/Users/dev/project",
		WorkingDir: "/Users/dev/project/src",
		Files:      []string{"main.go", "handler.go"},
		Metadata: map[string]interface{}{
			"branch": "feature/new-feature",
			"author": "developer",
		},
	}

	if ctx.Workspace == "" {
		t.Error("expected workspace to be set")
	}
	if ctx.WorkingDir == "" {
		t.Error("expected working dir to be set")
	}
	if len(ctx.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(ctx.Files))
	}
	if ctx.Metadata["branch"] != "feature/new-feature" {
		t.Errorf("expected branch metadata, got %v", ctx.Metadata["branch"])
	}
}

func TestTaskResult_ParsedOutput(t *testing.T) {
	result := &TaskResult{
		ID:         "task-123",
		Success:    true,
		Output:     "Feature added successfully",
		RoundsUsed: 5,
		Finished:   true,
		StartedAt:  time.Now(),
	}

	result.SetParsed("files_modified", []string{"main.go", "handler.go"})
	result.SetParsed("success", true)

	if result.Parsed == nil {
		t.Error("expected Parsed to be set")
	}
	if result.Parsed["files_modified"] == nil {
		t.Error("expected files_modified to be set")
	}
}

func TestTaskResult_MarkComplete(t *testing.T) {
	result := &TaskResult{
		ID:         "task-123",
		Success:    false,
		Output:     "",
		RoundsUsed: 0,
		Finished:   false,
		StartedAt:  time.Now(),
	}

	result.MarkComplete(true, "Task completed successfully")

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.Output != "Task completed successfully" {
		t.Errorf("expected Output %q, got %q", "Task completed successfully", result.Output)
	}
	if !result.Finished {
		t.Error("expected Finished to be true")
	}
	if result.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}

func TestNewTaskEnvelope(t *testing.T) {
	envelope := NewTaskEnvelope("builder", "Add a login feature", ModeCode, "/workspace/project")

	if envelope.Agent != "builder" {
		t.Errorf("expected Agent 'builder', got %q", envelope.Agent)
	}
	if envelope.Goal != "Add a login feature" {
		t.Errorf("expected Goal 'Add a login feature', got %q", envelope.Goal)
	}
	if envelope.Mode != ModeCode {
		t.Errorf("expected Mode ModeCode, got %v", envelope.Mode)
	}
	if envelope.Context.Workspace != "/workspace/project" {
		t.Errorf("expected Workspace '/workspace/project', got %q", envelope.Context.Workspace)
	}
	if envelope.MaxSteps != 20 {
		t.Errorf("expected MaxSteps 20, got %d", envelope.MaxSteps)
	}
	if envelope.ID == "" {
		t.Error("expected ID to be generated")
	}
}

func TestTaskEnvelope_SetToolsAllowed(t *testing.T) {
	envelope := NewTaskEnvelope("builder", "Add feature", ModeCode, "/workspace")
	envelope.SetToolsAllowed([]string{"file", "code_edit", "search"})

	if len(envelope.ToolsAllowed) != 3 {
		t.Errorf("expected 3 tools allowed, got %d", len(envelope.ToolsAllowed))
	}
	if envelope.ToolsAllowed[0] != "file" {
		t.Errorf("expected first tool 'file', got %q", envelope.ToolsAllowed[0])
	}
}

func TestTaskEnvelope_SetReturnSchema(t *testing.T) {
	envelope := NewTaskEnvelope("builder", "Add feature", ModeCode, "/workspace")
	envelope.SetReturnSchema(map[string]string{
		"files_modified": "[]string",
		"success":        "bool",
		"error":          "string",
	})

	if envelope.ReturnSchema == nil {
		t.Error("expected ReturnSchema to be set")
	}
	if envelope.ReturnSchema["files_modified"] != "[]string" {
		t.Errorf("expected files_modified type '[]string', got %q", envelope.ReturnSchema["files_modified"])
	}
}

func TestTaskEnvelope_AddFile(t *testing.T) {
	envelope := NewTaskEnvelope("builder", "Add feature", ModeCode, "/workspace")
	envelope.AddFile("main.go")
	envelope.AddFile("handler.go")

	if len(envelope.Context.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(envelope.Context.Files))
	}
}

func TestTaskEnvelope_SetMetadata(t *testing.T) {
	envelope := NewTaskEnvelope("builder", "Add feature", ModeCode, "/workspace")
	envelope.SetMetadata("priority", "high")
	envelope.SetMetadata("branch", "main")

	if envelope.Context.Metadata["priority"] != "high" {
		t.Errorf("expected priority 'high', got %v", envelope.Context.Metadata["priority"])
	}
	if envelope.Context.Metadata["branch"] != "main" {
		t.Errorf("expected branch 'main', got %v", envelope.Context.Metadata["branch"])
	}
}

func TestTaskEnvelopeValidationError(t *testing.T) {
	err := ErrInvalidTaskEnvelope("agent is required")
	if err.Error() != "invalid task envelope: agent is required" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestMode_JSON(t *testing.T) {
	tests := []struct {
		input  string
		expect Mode
	}{
		{input: "code", expect: ModeCode},
		{input: "blog", expect: ModeBlog},
		{input: "podcast", expect: ModePodcast},
		{input: "chat", expect: ModeChat},
		{input: "plan", expect: ModePlan},
		{input: "build", expect: ModeBuild},
		{input: "review", expect: ModeReview},
		{input: "technical_writer", expect: ModeTechnicalWriter},
		{input: "unknown_mode", expect: ModeCode},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode := ParseMode(tt.input)
			if mode != tt.expect {
				t.Errorf("ParseMode(%q) = %v, want %v", tt.input, mode, tt.expect)
			}
		})
	}
}
