package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the Pedroceli configuration
type Config struct {
	Model       ModelConfig       `json:"model"`
	Execution   ExecutionConfig   `json:"execution"`
	Git         GitConfig         `json:"git"`
	Tools       ToolsConfig       `json:"tools"`
	Project     ProjectConfig     `json:"project"`
	Limits      LimitsConfig      `json:"limits"`
	Debug       DebugConfig       `json:"debug"`
	Platform    PlatformConfig    `json:"platform"`
	Init        InitConfig        `json:"init"`
	LSP         LSPConfig         `json:"lsp"`
	FileIO      FileIOConfig      `json:"fileio"`
	Web         WebConfig         `json:"web"`
	Voice       VoiceConfig       `json:"voice"`
	RepoStorage RepoStorageConfig `json:"repo_storage"`
	Hooks       HooksConfig       `json:"hooks"`
	// Model profiles for different use cases (coding vs content)
	ModelProfiles map[string]ModelConfig `json:"model_profiles,omitempty"`
	// Podcast tools configuration
	Podcast PodcastConfig `json:"podcast,omitempty"`
	// OAuth configuration for external services
	OAuth OAuthConfig `json:"oauth,omitempty"`
	// Web scraping configuration
	WebScraping WebScrapingConfig `json:"web_scraping,omitempty"`
	// Blog writing tools configuration
	Blog     BlogConfig     `json:"blog"`
	Database DatabaseConfig `json:"database"`
}

// ModelConfig contains model configuration
type ModelConfig struct {
	Type          string  `json:"type"` // "llamacpp" or "ollama"
	ModelPath     string  `json:"model_path,omitempty"`
	LlamaCppPath  string  `json:"llamacpp_path,omitempty"`
	ModelName     string  `json:"model_name,omitempty"` // for Ollama
	OllamaURL     string  `json:"ollama_url,omitempty"` // Ollama API URL
	ContextSize   int     `json:"context_size"`
	UsableContext int     `json:"usable_context,omitempty"`
	NGpuLayers    int     `json:"n_gpu_layers,omitempty"`
	Temperature   float64 `json:"temperature"`
	Threads       int     `json:"threads,omitempty"`
}

// ExecutionConfig contains execution settings
type ExecutionConfig struct {
	RunOnSpark bool   `json:"run_on_spark"`
	SparkSSH   string `json:"spark_ssh,omitempty"`
}

// GitConfig contains git settings
type GitConfig struct {
	AlwaysDraftPR bool   `json:"always_draft_pr"`
	BranchPrefix  string `json:"branch_prefix"`
	Remote        string `json:"remote"`
}

// ToolsConfig contains tool settings
type ToolsConfig struct {
	AllowedBashCommands []string `json:"allowed_bash_commands"`
	ForbiddenCommands   []string `json:"forbidden_commands"`
}

// ProjectConfig contains project settings
type ProjectConfig struct {
	Name      string   `json:"name"`
	Workdir   string   `json:"workdir"`
	TechStack []string `json:"tech_stack"`
}

// LimitsConfig contains execution limits
type LimitsConfig struct {
	MaxTaskDurationMinutes int `json:"max_task_duration_minutes"`
	MaxInferenceRuns       int `json:"max_inference_runs"`
}

// DebugConfig contains debug settings
type DebugConfig struct {
	Enabled       bool   `json:"enabled"`
	KeepTempFiles bool   `json:"keep_temp_files"`
	LogLevel      string `json:"log_level"`
}

// PlatformConfig contains platform settings
type PlatformConfig struct {
	OS    string `json:"os"`    // "auto", "darwin", "linux", "windows"
	Shell string `json:"shell"` // "/bin/sh", "/bin/bash", etc.
}

// InitConfig contains initialization settings
type InitConfig struct {
	SkipChecks bool `json:"skip_checks"`
	Verbose    bool `json:"verbose"`
}

// LSPConfig contains LSP (Language Server Protocol) settings
type LSPConfig struct {
	Enabled      bool                    `json:"enabled"`
	AutoDiscover bool                    `json:"auto_discover"`
	Timeout      int                     `json:"timeout"`
	Servers      map[string]LSPServerDef `json:"servers,omitempty"`
}

// LSPServerDef defines an LSP server configuration
type LSPServerDef struct {
	Command     string                 `json:"command"`
	Args        []string               `json:"args,omitempty"`
	Languages   []string               `json:"languages"`
	RootURI     string                 `json:"root_uri,omitempty"`
	InitOptions map[string]interface{} `json:"init_options,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
	Enabled     bool                   `json:"enabled"`
}

// FileIOConfig contains file I/O settings
type FileIOConfig struct {
	MaxFileSize   int64  `json:"max_file_size"`
	EnableBackup  bool   `json:"enable_backup"`
	BackupDir     string `json:"backup_dir,omitempty"`
	AtomicWrites  bool   `json:"atomic_writes"`
	PreservePerms bool   `json:"preserve_permissions"`
}

// WebConfig contains web server settings
type WebConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Host    string `json:"host"`
}

// VoiceConfig contains voice/whisper.cpp settings
type VoiceConfig struct {
	Enabled    bool   `json:"enabled"`
	WhisperURL string `json:"whisper_url"`
	Language   string `json:"language,omitempty"` // Default language hint (e.g., "en", "auto")
}

// RepoStorageConfig contains repository storage settings
type RepoStorageConfig struct {
	BasePath       string                      `json:"base_path"`
	DatabasePath   string                      `json:"database_path,omitempty"`
	GitCredentials map[string]GitCredentialDef `json:"git_credentials,omitempty"`
	AutoPruneDays  int                         `json:"auto_prune_days,omitempty"`
	DefaultBranch  string                      `json:"default_branch,omitempty"`
	FetchOnAccess  bool                        `json:"fetch_on_access"`
	SSHKeyPath     string                      `json:"ssh_key_path,omitempty"`
}

// GitCredentialDef defines credentials for a git provider
type GitCredentialDef struct {
	Type       string `json:"type"` // "ssh", "https", "token"
	SSHKeyPath string `json:"ssh_key_path,omitempty"`
	Username   string `json:"username,omitempty"`
	// Token should be stored in environment variable, not config file
	TokenEnvVar string `json:"token_env_var,omitempty"`
}

// HooksConfig contains git hooks settings
type HooksConfig struct {
	AutoInstall      bool          `json:"auto_install"`
	ParseCIConfig    bool          `json:"parse_ci_config"`
	CustomChecks     []CustomCheck `json:"custom_checks,omitempty"`
	PreCommitTimeout string        `json:"pre_commit_timeout,omitempty"`
	PrePushTimeout   string        `json:"pre_push_timeout,omitempty"`
}

// CustomCheck defines a custom hook check
type CustomCheck struct {
	Name     string   `json:"name"`
	Command  string   `json:"command"`
	Args     []string `json:"args,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

// PodcastConfig contains podcast tools configuration
type PodcastConfig struct {
	Enabled bool `json:"enabled"`
	// Model profile to use for podcast tasks (references ModelProfiles key)
	ModelProfile string `json:"model_profile,omitempty"`
	// Notion MCP server configuration
	Notion NotionMCPConfig `json:"notion,omitempty"`
	// Google Calendar MCP server configuration
	Calendar CalendarMCPConfig `json:"calendar,omitempty"`
	// Podcast metadata
	Metadata PodcastMetadata `json:"metadata,omitempty"`
}

// NotionMCPConfig contains Notion MCP server configuration
type NotionMCPConfig struct {
	Enabled bool   `json:"enabled"`
	Command string `json:"command,omitempty"` // e.g., "npx @notionhq/notion-mcp-server" (stdio transport)
	URL     string `json:"url,omitempty"`     // e.g., "https://mcp.notion.com/mcp" (HTTP transport)
	// TODO: Add your Notion API key here
	APIKey string `json:"api_key,omitempty"`
	// Database IDs for different content types
	Databases NotionDatabases `json:"databases,omitempty"`
}

// NotionDatabases contains Notion database IDs
type NotionDatabases struct {
	// TODO: Add your Notion database IDs here
	Scripts          string `json:"scripts,omitempty"`            // Episode scripts and drafts
	PotentialArticle string `json:"potential_articles,omitempty"` // Links to review for episodes
	ArticlesReview   string `json:"articles_review,omitempty"`    // Curated article summaries
	NewsReview       string `json:"news_review,omitempty"`        // Current news items to discuss
	Guests           string `json:"guests,omitempty"`             // Guest information and scheduling
}

// CalendarMCPConfig contains Google Calendar MCP server configuration
type CalendarMCPConfig struct {
	Enabled bool   `json:"enabled"`
	Command string `json:"command,omitempty"` // e.g., "npx google-calendar-mcp"
	// TODO: Add your Google Calendar ID here
	CalendarID string `json:"calendar_id,omitempty"`
	// TODO: Path to OAuth credentials file
	CredentialsPath string `json:"credentials_path,omitempty"`
}

// PodcastMetadata contains podcast information for prompts
type PodcastMetadata struct {
	// TODO: Fill in your podcast details
	Name        string   `json:"name,omitempty"`        // e.g., "Domesticating AI"
	Description string   `json:"description,omitempty"` // Podcast description
	Format      string   `json:"format,omitempty"`      // e.g., "weekly discussion with cohost"
	Cohosts     []Cohost `json:"cohosts,omitempty"`     // Cohost information
	// TODO: Add recording platform details if needed
	RecordingPlatform string `json:"recording_platform,omitempty"` // e.g., "Riverside"
	// TODO: Google Drive folder for assets
	DriveFolder string `json:"drive_folder,omitempty"`
}

// Cohost contains cohost information
type Cohost struct {
	Name string `json:"name,omitempty"`
	Bio  string `json:"bio,omitempty"`
	Role string `json:"role,omitempty"` // e.g., "host", "cohost", "producer"
}

// OAuthConfig contains OAuth client credentials for external services
type OAuthConfig struct {
	// Google OAuth credentials for Calendar API
	Google GoogleOAuthConfig `json:"google,omitempty"`
}

// GoogleOAuthConfig contains Google OAuth client credentials
type GoogleOAuthConfig struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	// Optional: override default redirect URI
	RedirectURI string `json:"redirect_uri,omitempty"`
}

// WebScrapingConfig contains web scraping configuration
type WebScrapingConfig struct {
	Enabled bool `json:"enabled"`
	// HTTP client settings
	UserAgent      string `json:"user_agent,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	MaxSizeMB      int    `json:"max_size_mb,omitempty"`
	// Rate limiting (requests per second per domain)
	RateLimits map[string]float64 `json:"rate_limits,omitempty"`
	// Caching
	CacheEnabled   bool   `json:"cache_enabled"`
	CacheType      string `json:"cache_type,omitempty"` // "sqlite", "memory"
	CachePath      string `json:"cache_path,omitempty"`
	CacheTTLHours  int    `json:"cache_ttl_hours,omitempty"`
	CacheMaxSizeMB int64  `json:"cache_max_size_mb,omitempty"`
	// API tokens (optional, for higher rate limits)
	// These can also be set via environment variables:
	// GITHUB_TOKEN, GITLAB_TOKEN, SO_API_KEY
	GitHubToken      string `json:"github_token,omitempty"`
	GitLabToken      string `json:"gitlab_token,omitempty"`
	StackOverflowKey string `json:"stackoverflow_key,omitempty"`
	GoogleSearchKey  string `json:"google_search_key,omitempty"`
	GoogleSearchCX   string `json:"google_search_cx,omitempty"`
	// Search engine preference
	SearchEngine string `json:"search_engine,omitempty"` // "duckduckgo", "google", "searxng"
	SearXNGURL   string `json:"searxng_url,omitempty"`   // For self-hosted SearXNG
	// GitLab instance URL (for self-hosted)
	GitLabURL string `json:"gitlab_url,omitempty"`
}

// BlogConfig contains blog writing tools configuration
type BlogConfig struct {
	Enabled           bool   `json:"enabled"`
	DefaultModel      string `json:"default_model"`       // Model to use for blog writing (e.g., "qwen3:7b")
	WriterAutoRevise  bool   `json:"writer_auto_revise"`  // Auto-revise after writing
	EditorMode        string `json:"editor_mode"`         // "review" or "auto_revise"
	NotionAPIKey      string `json:"notion_api_key"`      // TODO: Move to secure storage
	NotionDraftsDB    string `json:"notion_drafts_db"`    // Database ID for drafts
	NotionPublishedDB string `json:"notion_published_db"` // Database ID for published posts
	NotionAssetsDB    string `json:"notion_assets_db"`    // Database ID for assets
	NotionIdeasDB     string `json:"notion_ideas_db"`     // Database ID for post ideas
	SubstackEnabled   bool   `json:"substack_enabled"`    // Enable Substack integration
	PaywallDays       int    `json:"paywall_days"`        // Days before removing paywall (default: 7)
	WhisperURL        string `json:"whisper_url"`         // Whisper server URL for transcription (e.g., "http://localhost:8081")
	WhisperModel      string `json:"whisper_model"`       // Whisper model name (e.g., "base.en", "medium.en")
	// Research tools configuration
	RSSFeedURL  string             `json:"rss_feed_url,omitempty"` // RSS feed URL for previous blog posts
	Research    BlogResearchConfig `json:"research,omitempty"`     // Research tools settings
	StaticLinks BlogStaticLinks    `json:"static_links,omitempty"` // Static links for newsletter
}

// BlogResearchConfig contains settings for blog research tools
type BlogResearchConfig struct {
	Enabled         bool `json:"enabled"`                     // Enable research tools
	CalendarEnabled bool `json:"calendar_enabled"`            // Enable Google Calendar integration
	RSSEnabled      bool `json:"rss_enabled"`                 // Enable RSS feed parsing
	MaxRSSPosts     int  `json:"max_rss_posts,omitempty"`     // Max RSS posts to fetch (default: 5)
	MaxCalendarDays int  `json:"max_calendar_days,omitempty"` // Max days ahead for calendar (default: 30)
}

// BlogStaticLinks contains static links for newsletter sections
type BlogStaticLinks struct {
	Discord            string       `json:"discord,omitempty"`
	LinkTree           string       `json:"linktree,omitempty"`
	YouTube            string       `json:"youtube,omitempty"`
	Twitter            string       `json:"twitter,omitempty"`
	Bluesky            string       `json:"bluesky,omitempty"`
	LinkedIn           string       `json:"linkedin,omitempty"`
	Newsletter         string       `json:"newsletter,omitempty"`
	YouTubePlaceholder string       `json:"youtube_placeholder,omitempty"` // Placeholder for latest video link
	CustomLinks        []CustomLink `json:"custom_links,omitempty"`
}

// CustomLink defines a custom link for newsletter sections
type CustomLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon,omitempty"` // emoji or icon name
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"` // TODO: Move to secure storage
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"` // disable, require, verify-ca, verify-full
}

// GetModelConfig returns the model configuration for a given profile name.
// If the profile is empty or not found, returns the default Model config.
func (c *Config) GetModelConfig(profile string) ModelConfig {
	if profile == "" {
		return c.Model
	}
	if c.ModelProfiles != nil {
		if cfg, ok := c.ModelProfiles[profile]; ok {
			return cfg
		}
	}
	return c.Model
}

// GetPodcastModelConfig returns the model configuration for podcast tasks.
// Uses the podcast.model_profile if set, otherwise falls back to default.
func (c *Config) GetPodcastModelConfig() ModelConfig {
	if c.Podcast.ModelProfile != "" {
		return c.GetModelConfig(c.Podcast.ModelProfile)
	}
	// If no podcast profile specified, try "content" profile, then default
	if c.ModelProfiles != nil {
		if cfg, ok := c.ModelProfiles["content"]; ok {
			return cfg
		}
	}
	return c.Model
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	config.setDefaults()

	// Validate
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadDefault attempts to load .pedrocli.json from current directory or home
func LoadDefault() (*Config, error) {
	// Try current directory
	if _, err := os.Stat(".pedrocli.json"); err == nil {
		return Load(".pedrocli.json")
	}

	// Try home directory
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(home, ".pedrocli.json")
		if _, err := os.Stat(homePath); err == nil {
			return Load(homePath)
		}
	}

	return nil, fmt.Errorf("no .pedrocli.json found in current directory or home")
}

// setDefaults sets default values for configuration
func (c *Config) setDefaults() {
	// Model defaults
	if c.Model.Temperature == 0 {
		c.Model.Temperature = 0.2
	}
	if c.Model.UsableContext == 0 && c.Model.ContextSize > 0 {
		c.Model.UsableContext = c.Model.ContextSize * 3 / 4
	}
	if c.Model.Threads == 0 {
		c.Model.Threads = 8
	}

	// Git defaults
	if c.Git.Remote == "" {
		c.Git.Remote = "origin"
	}
	if c.Git.BranchPrefix == "" {
		c.Git.BranchPrefix = "pedroceli/"
	}

	// Limits defaults
	if c.Limits.MaxTaskDurationMinutes == 0 {
		c.Limits.MaxTaskDurationMinutes = 30
	}
	if c.Limits.MaxInferenceRuns == 0 {
		c.Limits.MaxInferenceRuns = 20
	}

	// Debug defaults
	if c.Debug.LogLevel == "" {
		c.Debug.LogLevel = "info"
	}

	// Platform defaults
	if c.Platform.OS == "" {
		c.Platform.OS = "auto"
	}
	if c.Platform.Shell == "" {
		c.Platform.Shell = "/bin/sh"
	}

	// Tools defaults
	if len(c.Tools.AllowedBashCommands) == 0 {
		c.Tools.AllowedBashCommands = []string{
			"git", "gh", "go", "cat", "ls", "head", "tail", "wc", "sort", "uniq",
		}
	}
	if len(c.Tools.ForbiddenCommands) == 0 {
		c.Tools.ForbiddenCommands = []string{
			"sed", "grep", "find", "xargs", "rm", "mv", "dd", "sudo",
		}
	}

	// LSP defaults
	if c.LSP.Timeout == 0 {
		c.LSP.Timeout = 30
	}
	if c.LSP.Servers == nil {
		c.LSP.Servers = make(map[string]LSPServerDef)
	}

	// FileIO defaults
	if c.FileIO.MaxFileSize == 0 {
		c.FileIO.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}
	if !c.FileIO.AtomicWrites {
		c.FileIO.AtomicWrites = true // Enable by default
	}

	// Web defaults
	if c.Web.Port == 0 {
		c.Web.Port = 8080
	}
	if c.Web.Host == "" {
		c.Web.Host = "0.0.0.0" // Bind to all interfaces for Tailscale/remote access
	}

	// Voice defaults
	if c.Voice.WhisperURL == "" {
		c.Voice.WhisperURL = "http://localhost:9090" // Default whisper.cpp server
	}
	if c.Voice.Language == "" {
		c.Voice.Language = "auto" // Auto-detect language
	}

	// RepoStorage defaults
	if c.RepoStorage.BasePath == "" {
		c.RepoStorage.BasePath = "/var/pedro/repos"
	}
	if c.RepoStorage.DatabasePath == "" {
		c.RepoStorage.DatabasePath = filepath.Join(c.RepoStorage.BasePath, "pedro.db")
	}
	if c.RepoStorage.DefaultBranch == "" {
		c.RepoStorage.DefaultBranch = "main"
	}
	if c.RepoStorage.AutoPruneDays == 0 {
		c.RepoStorage.AutoPruneDays = 30
	}
	if c.RepoStorage.GitCredentials == nil {
		c.RepoStorage.GitCredentials = make(map[string]GitCredentialDef)
	}

	// Hooks defaults
	if c.Hooks.PreCommitTimeout == "" {
		c.Hooks.PreCommitTimeout = "30s"
	}
	if c.Hooks.PrePushTimeout == "" {
		c.Hooks.PrePushTimeout = "5m"
	}

	// Model profiles defaults - apply same defaults to each profile
	for name, profile := range c.ModelProfiles {
		if profile.Temperature == 0 {
			profile.Temperature = 0.2
		}
		if profile.UsableContext == 0 && profile.ContextSize > 0 {
			profile.UsableContext = profile.ContextSize * 3 / 4
		}
		if profile.Threads == 0 {
			profile.Threads = 8
		}
		c.ModelProfiles[name] = profile
	}

	// Podcast defaults
	if c.Podcast.Notion.Command == "" {
		c.Podcast.Notion.Command = "npx -y @notionhq/notion-mcp-server"
	}
	if c.Podcast.Calendar.Command == "" {
		// Use our built-in calendar MCP server
		c.Podcast.Calendar.Command = "./pedrocli-calendar-mcp"
	}

	// Web scraping defaults
	if c.WebScraping.UserAgent == "" {
		c.WebScraping.UserAgent = "PedroCLI/1.0 (Web Scraping Tool)"
	}
	if c.WebScraping.TimeoutSeconds == 0 {
		c.WebScraping.TimeoutSeconds = 30
	}
	if c.WebScraping.MaxSizeMB == 0 {
		c.WebScraping.MaxSizeMB = 10
	}
	if c.WebScraping.CacheType == "" {
		c.WebScraping.CacheType = "memory"
	}
	if c.WebScraping.CacheTTLHours == 0 {
		c.WebScraping.CacheTTLHours = 1
	}
	if c.WebScraping.CacheMaxSizeMB == 0 {
		c.WebScraping.CacheMaxSizeMB = 100
	}
	if c.WebScraping.SearchEngine == "" {
		c.WebScraping.SearchEngine = "duckduckgo"
	}
	if c.WebScraping.GitLabURL == "" {
		c.WebScraping.GitLabURL = "https://gitlab.com"
	}

	// Blog defaults
	if c.Blog.DefaultModel == "" {
		c.Blog.DefaultModel = "qwen3:7b" // Default to Qwen 3 (not Coder)
	}
	if c.Blog.EditorMode == "" {
		c.Blog.EditorMode = "review" // Default to review mode
	}
	if c.Blog.PaywallDays == 0 {
		c.Blog.PaywallDays = 7 // Default 7-day paywall
	}
	// Blog research defaults
	if c.Blog.Research.MaxRSSPosts == 0 {
		c.Blog.Research.MaxRSSPosts = 5 // Default 5 RSS posts
	}
	if c.Blog.Research.MaxCalendarDays == 0 {
		c.Blog.Research.MaxCalendarDays = 30 // Default 30 days ahead
	}
	if c.Blog.StaticLinks.YouTubePlaceholder == "" {
		c.Blog.StaticLinks.YouTubePlaceholder = "🎥 **Latest Video**: [ADD LINK BEFORE SUBSTACK PUBLISH]"
	}

	// Database defaults
	if c.Database.Host == "" {
		c.Database.Host = "localhost"
	}
	if c.Database.Port == 0 {
		c.Database.Port = 5432
	}
	if c.Database.User == "" {
		c.Database.User = "pedrocli"
	}
	if c.Database.Database == "" {
		c.Database.Database = "pedrocli_blog"
	}
	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable" // TODO: Enable SSL for production
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate model type
	if c.Model.Type != "llamacpp" && c.Model.Type != "ollama" {
		return fmt.Errorf("invalid model type: %s (must be 'llamacpp' or 'ollama')", c.Model.Type)
	}

	// Validate llama.cpp config
	if c.Model.Type == "llamacpp" {
		if c.Model.ModelPath == "" {
			return fmt.Errorf("model_path is required for llamacpp backend")
		}
		if c.Model.LlamaCppPath == "" {
			return fmt.Errorf("llamacpp_path is required for llamacpp backend")
		}
		if c.Model.ContextSize < 2048 {
			return fmt.Errorf("context_size too small: %d (minimum 2048)", c.Model.ContextSize)
		}
		if c.Model.ContextSize > 200000 {
			return fmt.Errorf("context_size suspiciously large: %d", c.Model.ContextSize)
		}
	}

	// Validate Ollama config
	if c.Model.Type == "ollama" {
		if c.Model.ModelName == "" {
			return fmt.Errorf("model_name is required for ollama backend")
		}
	}

	// Validate LSP config
	if c.LSP.Enabled {
		if c.LSP.Timeout < 1 || c.LSP.Timeout > 300 {
			return fmt.Errorf("LSP timeout must be between 1 and 300 seconds")
		}
		for name, server := range c.LSP.Servers {
			if server.Command == "" {
				return fmt.Errorf("LSP server %s has no command specified", name)
			}
			if len(server.Languages) == 0 {
				return fmt.Errorf("LSP server %s has no languages specified", name)
			}
		}
	}

	// Validate FileIO config (only if explicitly set to a non-default value)
	if c.FileIO.MaxFileSize != 0 && c.FileIO.MaxFileSize < 1024 {
		return fmt.Errorf("max_file_size must be at least 1KB")
	}
	if c.FileIO.MaxFileSize > 100*1024*1024 {
		return fmt.Errorf("max_file_size cannot exceed 100MB")
	}

	return nil
}
