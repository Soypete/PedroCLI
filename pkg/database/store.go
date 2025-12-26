package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/soypete/pedrocli/pkg/hooks"
	"github.com/soypete/pedrocli/pkg/repos"
)

// SQLiteStore implements repos.Store using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	if _, err := db.Exec(Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SaveRepo saves or updates a repository record
func (s *SQLiteStore) SaveRepo(ctx context.Context, repo *repos.LocalRepo) error {
	if repo.ID == "" {
		repo.ID = uuid.New().String()
	}

	hooksConfigJSON, _ := json.Marshal(nil) // TODO: Store hooks config

	query := `
		INSERT INTO managed_repos (id, provider, owner, repo_name, local_path, default_branch, project_type, hooks_config, last_fetched, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, owner, repo_name) DO UPDATE SET
			local_path = excluded.local_path,
			default_branch = excluded.default_branch,
			project_type = excluded.project_type,
			hooks_config = excluded.hooks_config,
			last_fetched = excluded.last_fetched,
			updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		repo.ID,
		repo.Provider,
		repo.Owner,
		repo.Name,
		repo.LocalPath,
		repo.DefaultBranch,
		repo.ProjectType,
		string(hooksConfigJSON),
		repo.LastFetched,
		repo.CreatedAt,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to save repo: %w", err)
	}

	return nil
}

// GetRepo retrieves a repository by provider/owner/name
func (s *SQLiteStore) GetRepo(ctx context.Context, provider, owner, name string) (*repos.LocalRepo, error) {
	query := `
		SELECT id, provider, owner, repo_name, local_path, default_branch, project_type, last_fetched, created_at, updated_at
		FROM managed_repos
		WHERE provider = ? AND owner = ? AND repo_name = ?
	`

	row := s.db.QueryRowContext(ctx, query, provider, owner, name)
	return s.scanRepo(row)
}

// GetRepoByID retrieves a repository by ID
func (s *SQLiteStore) GetRepoByID(ctx context.Context, id string) (*repos.LocalRepo, error) {
	query := `
		SELECT id, provider, owner, repo_name, local_path, default_branch, project_type, last_fetched, created_at, updated_at
		FROM managed_repos
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	return s.scanRepo(row)
}

// ListRepos lists all repositories
func (s *SQLiteStore) ListRepos(ctx context.Context) ([]repos.LocalRepo, error) {
	query := `
		SELECT id, provider, owner, repo_name, local_path, default_branch, project_type, last_fetched, created_at, updated_at
		FROM managed_repos
		ORDER BY updated_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}
	defer rows.Close()

	var repoList []repos.LocalRepo
	for rows.Next() {
		repo, err := s.scanRepoFromRows(rows)
		if err != nil {
			return nil, err
		}
		repoList = append(repoList, *repo)
	}

	return repoList, rows.Err()
}

// DeleteRepo deletes a repository record
func (s *SQLiteStore) DeleteRepo(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM managed_repos WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete repo: %w", err)
	}
	return nil
}

// SaveOperation saves a repository operation record
func (s *SQLiteStore) SaveOperation(ctx context.Context, op *repos.RepoOperation) error {
	if op.ID == "" {
		op.ID = uuid.New().String()
	}

	detailsJSON, _ := json.Marshal(op.Details)

	query := `
		INSERT INTO repo_operations (id, repo_id, operation_type, ref_before, ref_after, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		op.ID,
		op.RepoID,
		op.OperationType,
		op.RefBefore,
		op.RefAfter,
		string(detailsJSON),
		op.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save operation: %w", err)
	}

	return nil
}

// ListOperations lists operations for a repository
func (s *SQLiteStore) ListOperations(ctx context.Context, repoID string, limit int) ([]repos.RepoOperation, error) {
	query := `
		SELECT id, repo_id, operation_type, ref_before, ref_after, details, created_at
		FROM repo_operations
		WHERE repo_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list operations: %w", err)
	}
	defer rows.Close()

	var operations []repos.RepoOperation
	for rows.Next() {
		var op repos.RepoOperation
		var detailsJSON string

		err := rows.Scan(&op.ID, &op.RepoID, &op.OperationType, &op.RefBefore, &op.RefAfter, &detailsJSON, &op.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %w", err)
		}

		if detailsJSON != "" {
			json.Unmarshal([]byte(detailsJSON), &op.Details)
		}

		operations = append(operations, op)
	}

	return operations, rows.Err()
}

// SavePR saves or updates a tracked PR
func (s *SQLiteStore) SavePR(ctx context.Context, pr *repos.TrackedPR) error {
	if pr.ID == "" {
		pr.ID = uuid.New().String()
	}

	query := `
		INSERT INTO tracked_prs (id, repo_id, pr_number, branch_name, base_branch, title, body, status, local_commit_hash, remote_commit_hash, html_url, created_at, updated_at, merged_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, pr_number) DO UPDATE SET
			branch_name = excluded.branch_name,
			title = excluded.title,
			body = excluded.body,
			status = excluded.status,
			local_commit_hash = excluded.local_commit_hash,
			remote_commit_hash = excluded.remote_commit_hash,
			html_url = excluded.html_url,
			updated_at = excluded.updated_at,
			merged_at = excluded.merged_at
	`

	_, err := s.db.ExecContext(ctx, query,
		pr.ID,
		pr.RepoID,
		pr.PRNumber,
		pr.BranchName,
		pr.BaseBranch,
		pr.Title,
		pr.Body,
		string(pr.Status),
		pr.LocalCommitHash,
		pr.RemoteCommitHash,
		pr.HTMLURL,
		pr.CreatedAt,
		time.Now(),
		pr.MergedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save PR: %w", err)
	}

	return nil
}

// GetPR retrieves a PR by repo ID and number
func (s *SQLiteStore) GetPR(ctx context.Context, repoID string, prNumber int) (*repos.TrackedPR, error) {
	query := `
		SELECT id, repo_id, pr_number, branch_name, base_branch, title, body, status, local_commit_hash, remote_commit_hash, html_url, created_at, updated_at, merged_at
		FROM tracked_prs
		WHERE repo_id = ? AND pr_number = ?
	`

	row := s.db.QueryRowContext(ctx, query, repoID, prNumber)
	return s.scanPR(row)
}

// ListPRs lists PRs for a repository
func (s *SQLiteStore) ListPRs(ctx context.Context, repoID string) ([]repos.TrackedPR, error) {
	query := `
		SELECT id, repo_id, pr_number, branch_name, base_branch, title, body, status, local_commit_hash, remote_commit_hash, html_url, created_at, updated_at, merged_at
		FROM tracked_prs
		WHERE repo_id = ?
		ORDER BY updated_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list PRs: %w", err)
	}
	defer rows.Close()

	var prs []repos.TrackedPR
	for rows.Next() {
		pr, err := s.scanPRFromRows(rows)
		if err != nil {
			return nil, err
		}
		prs = append(prs, *pr)
	}

	return prs, rows.Err()
}

// SaveJob saves or updates a repo job
func (s *SQLiteStore) SaveJob(ctx context.Context, job *repos.RepoJob) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	inputJSON, _ := json.Marshal(job.InputPayload)
	outputJSON, _ := json.Marshal(job.OutputPayload)
	validationJSON, _ := json.Marshal(job.LastValidationResult)

	query := `
		INSERT INTO repo_jobs (id, repo_id, job_type, branch_name, status, input_payload, output_payload, validation_attempts, last_validation_result, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			output_payload = excluded.output_payload,
			validation_attempts = excluded.validation_attempts,
			last_validation_result = excluded.last_validation_result,
			completed_at = excluded.completed_at
	`

	_, err := s.db.ExecContext(ctx, query,
		job.ID,
		job.RepoID,
		job.JobType,
		job.BranchName,
		job.Status,
		string(inputJSON),
		string(outputJSON),
		job.ValidationAttempts,
		string(validationJSON),
		job.CreatedAt,
		job.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save job: %w", err)
	}

	return nil
}

// GetJob retrieves a job by ID
func (s *SQLiteStore) GetJob(ctx context.Context, id string) (*repos.RepoJob, error) {
	query := `
		SELECT id, repo_id, job_type, branch_name, status, input_payload, output_payload, validation_attempts, last_validation_result, created_at, completed_at
		FROM repo_jobs
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, id)
	return s.scanJob(row)
}

// ListJobs lists jobs for a repository
func (s *SQLiteStore) ListJobs(ctx context.Context, repoID string) ([]repos.RepoJob, error) {
	query := `
		SELECT id, repo_id, job_type, branch_name, status, input_payload, output_payload, validation_attempts, last_validation_result, created_at, completed_at
		FROM repo_jobs
		WHERE repo_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []repos.RepoJob
	for rows.Next() {
		job, err := s.scanJobFromRows(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}

	return jobs, rows.Err()
}

// SaveHookRun saves a hook run record
func (s *SQLiteStore) SaveHookRun(ctx context.Context, run *hooks.HookRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}

	resultsJSON, _ := json.Marshal(run.Results)

	query := `
		INSERT INTO hook_runs (id, repo_id, hook_type, triggered_by, passed, results, agent_feedback, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	passed := 0
	if run.Passed {
		passed = 1
	}

	_, err := s.db.ExecContext(ctx, query,
		run.ID,
		run.RepoID,
		string(run.HookType),
		run.TriggeredBy,
		passed,
		string(resultsJSON),
		run.AgentFeedback,
		run.DurationMs,
		run.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save hook run: %w", err)
	}

	return nil
}

// ListHookRuns lists hook runs for a repository
func (s *SQLiteStore) ListHookRuns(ctx context.Context, repoID string, limit int) ([]hooks.HookRun, error) {
	query := `
		SELECT id, repo_id, hook_type, triggered_by, passed, results, agent_feedback, duration_ms, created_at
		FROM hook_runs
		WHERE repo_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, repoID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list hook runs: %w", err)
	}
	defer rows.Close()

	var runs []hooks.HookRun
	for rows.Next() {
		var run hooks.HookRun
		var resultsJSON string
		var passed int

		err := rows.Scan(&run.ID, &run.RepoID, &run.HookType, &run.TriggeredBy, &passed, &resultsJSON, &run.AgentFeedback, &run.DurationMs, &run.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hook run: %w", err)
		}

		run.Passed = passed == 1
		if resultsJSON != "" {
			json.Unmarshal([]byte(resultsJSON), &run.Results)
		}

		runs = append(runs, run)
	}

	return runs, rows.Err()
}

// Helper methods for scanning rows

func (s *SQLiteStore) scanRepo(row *sql.Row) (*repos.LocalRepo, error) {
	var repo repos.LocalRepo
	var lastFetched sql.NullTime

	err := row.Scan(
		&repo.ID,
		&repo.Provider,
		&repo.Owner,
		&repo.Name,
		&repo.LocalPath,
		&repo.DefaultBranch,
		&repo.ProjectType,
		&lastFetched,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan repo: %w", err)
	}

	if lastFetched.Valid {
		repo.LastFetched = lastFetched.Time
	}

	return &repo, nil
}

func (s *SQLiteStore) scanRepoFromRows(rows *sql.Rows) (*repos.LocalRepo, error) {
	var repo repos.LocalRepo
	var lastFetched sql.NullTime

	err := rows.Scan(
		&repo.ID,
		&repo.Provider,
		&repo.Owner,
		&repo.Name,
		&repo.LocalPath,
		&repo.DefaultBranch,
		&repo.ProjectType,
		&lastFetched,
		&repo.CreatedAt,
		&repo.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan repo: %w", err)
	}

	if lastFetched.Valid {
		repo.LastFetched = lastFetched.Time
	}

	return &repo, nil
}

func (s *SQLiteStore) scanPR(row *sql.Row) (*repos.TrackedPR, error) {
	var pr repos.TrackedPR
	var status string
	var mergedAt sql.NullTime
	var body sql.NullString

	err := row.Scan(
		&pr.ID,
		&pr.RepoID,
		&pr.PRNumber,
		&pr.BranchName,
		&pr.BaseBranch,
		&pr.Title,
		&body,
		&status,
		&pr.LocalCommitHash,
		&pr.RemoteCommitHash,
		&pr.HTMLURL,
		&pr.CreatedAt,
		&pr.UpdatedAt,
		&mergedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan PR: %w", err)
	}

	pr.Status = repos.PRStatus(status)
	if body.Valid {
		pr.Body = body.String
	}
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}

	return &pr, nil
}

func (s *SQLiteStore) scanPRFromRows(rows *sql.Rows) (*repos.TrackedPR, error) {
	var pr repos.TrackedPR
	var status string
	var mergedAt sql.NullTime
	var body sql.NullString

	err := rows.Scan(
		&pr.ID,
		&pr.RepoID,
		&pr.PRNumber,
		&pr.BranchName,
		&pr.BaseBranch,
		&pr.Title,
		&body,
		&status,
		&pr.LocalCommitHash,
		&pr.RemoteCommitHash,
		&pr.HTMLURL,
		&pr.CreatedAt,
		&pr.UpdatedAt,
		&mergedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan PR: %w", err)
	}

	pr.Status = repos.PRStatus(status)
	if body.Valid {
		pr.Body = body.String
	}
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}

	return &pr, nil
}

func (s *SQLiteStore) scanJob(row *sql.Row) (*repos.RepoJob, error) {
	var job repos.RepoJob
	var inputJSON, outputJSON, validationJSON string
	var completedAt sql.NullTime
	var branchName sql.NullString

	err := row.Scan(
		&job.ID,
		&job.RepoID,
		&job.JobType,
		&branchName,
		&job.Status,
		&inputJSON,
		&outputJSON,
		&job.ValidationAttempts,
		&validationJSON,
		&job.CreatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	if branchName.Valid {
		job.BranchName = branchName.String
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	if inputJSON != "" {
		json.Unmarshal([]byte(inputJSON), &job.InputPayload)
	}
	if outputJSON != "" {
		json.Unmarshal([]byte(outputJSON), &job.OutputPayload)
	}
	if validationJSON != "" {
		json.Unmarshal([]byte(validationJSON), &job.LastValidationResult)
	}

	return &job, nil
}

func (s *SQLiteStore) scanJobFromRows(rows *sql.Rows) (*repos.RepoJob, error) {
	var job repos.RepoJob
	var inputJSON, outputJSON, validationJSON string
	var completedAt sql.NullTime
	var branchName sql.NullString

	err := rows.Scan(
		&job.ID,
		&job.RepoID,
		&job.JobType,
		&branchName,
		&job.Status,
		&inputJSON,
		&outputJSON,
		&job.ValidationAttempts,
		&validationJSON,
		&job.CreatedAt,
		&completedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	if branchName.Valid {
		job.BranchName = branchName.String
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	if inputJSON != "" {
		json.Unmarshal([]byte(inputJSON), &job.InputPayload)
	}
	if outputJSON != "" {
		json.Unmarshal([]byte(outputJSON), &job.OutputPayload)
	}
	if validationJSON != "" {
		json.Unmarshal([]byte(validationJSON), &job.LastValidationResult)
	}

	return &job, nil
}

// Ensure SQLiteStore implements repos.Store
var _ repos.Store = (*SQLiteStore)(nil)
