// Package database provides SQLite-based persistent storage for repository management.
//
// # Database Schema
//
// The database schema is managed by goose migrations located in the migrations/ directory.
// Migrations are automatically applied when the store is initialized.
//
// Current tables:
//   - managed_repos: Managed repository information
//   - repo_operations: Repository operation history
//   - tracked_prs: Tracked pull requests
//   - hook_runs: Hook execution records
//   - repo_jobs: Agent job records
//   - git_credentials: Git credential storage
//   - oauth_tokens: OAuth tokens for external services
//
// See migrations/*.sql for the full schema definition.
// See docs/migrations.md for migration documentation.
package database
