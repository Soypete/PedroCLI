package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}

	if mgr.stateDir != tmpDir {
		t.Errorf("stateDir = %v, want %v", mgr.stateDir, tmpDir)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("State directory was not created")
	}
}

func TestNewManagerDefaultDir(t *testing.T) {
	mgr, err := NewManager("")
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	expectedDir := "/tmp/pedroceli-jobs"
	if mgr.stateDir != expectedDir {
		t.Errorf("stateDir = %v, want %v", mgr.stateDir, expectedDir)
	}
}

func TestNewManagerLoadExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a job file
	job := &Job{
		ID:          "job-12345",
		Type:        "build",
		Status:      StatusCompleted,
		Description: "Test job",
		Input:       map[string]interface{}{"test": "value"},
		CreatedAt:   time.Now(),
	}

	data, _ := json.MarshalIndent(job, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "job-12345.json"), data, 0644)

	// Create manager - should load existing job
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if len(mgr.jobs) != 1 {
		t.Errorf("Expected 1 loaded job, got %d", len(mgr.jobs))
	}

	loadedJob, ok := mgr.jobs["job-12345"]
	if !ok {
		t.Error("Job was not loaded")
	}

	if loadedJob.Type != "build" {
		t.Errorf("Loaded job type = %v, want build", loadedJob.Type)
	}
}

func TestCreate(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	input := map[string]interface{}{
		"description": "Add new feature",
		"branch":      "feature/test",
	}

	job, err := mgr.Create(context.Background(), "build", "Build new feature", input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if job == nil {
		t.Fatal("Create() returned nil job")
	}

	if job.ID == "" {
		t.Error("Job ID should not be empty")
	}

	if job.Type != "build" {
		t.Errorf("Type = %v, want build", job.Type)
	}

	if job.Status != StatusPending {
		t.Errorf("Status = %v, want %v", job.Status, StatusPending)
	}

	if job.Description != "Build new feature" {
		t.Errorf("Description = %v, want 'Build new feature'", job.Description)
	}

	// Verify job was saved to disk
	filename := filepath.Join(tmpDir, job.ID+".json")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Job was not saved to disk")
	}

	// Verify job is in memory
	if _, ok := mgr.jobs[job.ID]; !ok {
		t.Error("Job was not added to memory")
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a job
	createdJob, _ := mgr.Create(context.Background(), "review", "Review PR", map[string]interface{}{})

	// Get the job
	job, err := mgr.Get(context.Background(), createdJob.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if job.ID != createdJob.ID {
		t.Errorf("Got job ID = %v, want %v", job.ID, createdJob.ID)
	}

	// Try to get non-existent job
	_, err = mgr.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Get() should error for non-existent job")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create multiple jobs with sufficient time between them
	mgr.Create(context.Background(), "build", "Job 1", map[string]interface{}{})
	time.Sleep(1 * time.Second) // Ensure different Unix timestamps
	mgr.Create(context.Background(), "review", "Job 2", map[string]interface{}{})
	time.Sleep(1 * time.Second)
	mgr.Create(context.Background(), "debug", "Job 3", map[string]interface{}{})

	// List all jobs
	jobs, _ := mgr.List(context.Background())

	if len(jobs) != 3 {
		t.Errorf("List() returned %d jobs, want 3", len(jobs))
	}

	// Verify all jobs are present
	types := make(map[string]bool)
	for _, job := range jobs {
		types[job.Type] = true
	}

	if !types["build"] || !types["review"] || !types["debug"] {
		t.Error("Not all job types are present in the list")
	}
}

func TestListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	jobs, _ := mgr.List(context.Background())

	if len(jobs) != 0 {
		t.Errorf("List() returned %d jobs for empty manager, want 0", len(jobs))
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a job
	job, _ := mgr.Create(context.Background(), "build", "Test job", map[string]interface{}{})

	// Update to running
	output := map[string]interface{}{"step": "compiling"}
	err = mgr.Update(context.Background(), job.ID, StatusRunning, output, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	updatedJob, _ := mgr.Get(context.Background(), job.ID)
	if updatedJob.Status != StatusRunning {
		t.Errorf("Status = %v, want %v", updatedJob.Status, StatusRunning)
	}

	if updatedJob.StartedAt == nil {
		t.Error("StartedAt should be set when status changes to running")
	}

	if updatedJob.Output["step"] != "compiling" {
		t.Error("Output was not updated")
	}

	// Update to completed
	finalOutput := map[string]interface{}{"result": "success"}
	err = mgr.Update(context.Background(), job.ID, StatusCompleted, finalOutput, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	completedJob, _ := mgr.Get(context.Background(), job.ID)
	if completedJob.Status != StatusCompleted {
		t.Errorf("Status = %v, want %v", completedJob.Status, StatusCompleted)
	}

	if completedJob.CompletedAt == nil {
		t.Error("CompletedAt should be set when status changes to completed")
	}
}

func TestUpdateWithError(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a job
	job, _ := mgr.Create(context.Background(), "build", "Test job", map[string]interface{}{})

	// Update to failed with error
	testError := errors.New("build failed: compilation error")
	err = mgr.Update(context.Background(), job.ID, StatusFailed, nil, testError)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify error was saved
	failedJob, _ := mgr.Get(context.Background(), job.ID)
	if failedJob.Status != StatusFailed {
		t.Errorf("Status = %v, want %v", failedJob.Status, StatusFailed)
	}

	if failedJob.Error != testError.Error() {
		t.Errorf("Error = %v, want %v", failedJob.Error, testError.Error())
	}

	if failedJob.CompletedAt == nil {
		t.Error("CompletedAt should be set when status changes to failed")
	}
}

func TestUpdateNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	err = mgr.Update(context.Background(), "nonexistent", StatusCompleted, nil, nil)
	if err == nil {
		t.Error("Update() should error for non-existent job")
	}
}

func TestCancel(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a running job
	job, _ := mgr.Create(context.Background(), "build", "Test job", map[string]interface{}{})
	mgr.Update(context.Background(), job.ID, StatusRunning, nil, nil)

	// Cancel the job
	err = mgr.Cancel(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	// Verify it was cancelled
	cancelledJob, _ := mgr.Get(context.Background(), job.ID)
	if cancelledJob.Status != StatusCancelled {
		t.Errorf("Status = %v, want %v", cancelledJob.Status, StatusCancelled)
	}

	if cancelledJob.CompletedAt == nil {
		t.Error("CompletedAt should be set when job is cancelled")
	}
}

func TestCleanupOldJobs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create old completed job
	oldJob, _ := mgr.Create(context.Background(), "build", "Old job", map[string]interface{}{})
	time.Sleep(1 * time.Second) // Ensure different timestamp
	oldTime := time.Now().Add(-2 * time.Hour)
	mgr.Update(context.Background(), oldJob.ID, StatusCompleted, nil, nil)
	// Manually set old completion time and save
	mgr.mu.Lock()
	mgr.jobs[oldJob.ID].CompletedAt = &oldTime
	mgr.saveJob(mgr.jobs[oldJob.ID])
	mgr.mu.Unlock()

	// Create recent completed job
	time.Sleep(1 * time.Second)
	recentJob, _ := mgr.Create(context.Background(), "review", "Recent job", map[string]interface{}{})
	mgr.Update(context.Background(), recentJob.ID, StatusCompleted, nil, nil)

	// Create pending job
	time.Sleep(1 * time.Second)
	pendingJob, _ := mgr.Create(context.Background(), "debug", "Pending job", map[string]interface{}{})

	// Cleanup jobs older than 1 hour
	err = mgr.CleanupOldJobs(context.Background(), 1*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOldJobs() error = %v", err)
	}

	// Old job should be removed
	if _, ok := mgr.jobs[oldJob.ID]; ok {
		t.Error("Old job should have been removed")
	}

	// Recent job should still exist
	if _, ok := mgr.jobs[recentJob.ID]; !ok {
		t.Error("Recent job should not be removed")
	}

	// Pending job should still exist
	if _, ok := mgr.jobs[pendingJob.ID]; !ok {
		t.Error("Pending job should not be removed")
	}

	// Verify old job file was deleted
	oldFilename := filepath.Join(tmpDir, oldJob.ID+".json")
	if _, err := os.Stat(oldFilename); !os.IsNotExist(err) {
		t.Error("Old job file should be deleted")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create some jobs first
	jobIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		job, _ := mgr.Create(context.Background(), "build", "Concurrent test job", map[string]interface{}{})
		jobIDs[i] = job.ID
		time.Sleep(1 * time.Second) // Ensure unique timestamps
	}

	// Test concurrent reads and updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		idx := i
		go func() {
			// Concurrent reads
			jobID := jobIDs[idx%5]
			mgr.Get(context.Background(), jobID)
			mgr.List(context.Background())

			// Concurrent updates
			mgr.Update(context.Background(), jobID, StatusRunning, map[string]interface{}{"iteration": idx}, nil)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify jobs are still accessible
	jobs, _ := mgr.List(context.Background())
	if len(jobs) != 5 {
		t.Errorf("Expected 5 jobs, got %d", len(jobs))
	}
}

func TestJobPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager and add job
	mgr1, _ := NewManager(tmpDir)
	job, _ := mgr1.Create(context.Background(), "build", "Persistent job", map[string]interface{}{
		"test": "value",
	})
	mgr1.Update(context.Background(), job.ID, StatusRunning, map[string]interface{}{"progress": 50}, nil)

	// Create new manager instance - should load persisted job
	mgr2, _ := NewManager(tmpDir)

	loadedJob, err := mgr2.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Failed to load persisted job: %v", err)
	}

	if loadedJob.Status != StatusRunning {
		t.Errorf("Loaded job status = %v, want %v", loadedJob.Status, StatusRunning)
	}

	if loadedJob.Output["progress"] != float64(50) {
		t.Errorf("Loaded job output = %v, want progress: 50", loadedJob.Output)
	}

	if loadedJob.Input["test"] != "value" {
		t.Error("Loaded job input was not preserved")
	}
}

func TestJobStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status Status
	}{
		{"pending", StatusPending},
		{"running", StatusRunning},
		{"completed", StatusCompleted},
		{"failed", StatusFailed},
		{"cancelled", StatusCancelled},
	}

	tmpDir := t.TempDir()
	mgr, _ := NewManager(tmpDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, _ := mgr.Create(context.Background(), "test", "Test job", map[string]interface{}{})

			err := mgr.Update(context.Background(), job.ID, tt.status, nil, nil)
			if err != nil {
				t.Errorf("Update() error = %v", err)
			}

			updatedJob, _ := mgr.Get(context.Background(), job.ID)
			if updatedJob.Status != tt.status {
				t.Errorf("Status = %v, want %v", updatedJob.Status, tt.status)
			}
		})
	}
}
