package fileio

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// LSPServerConfig represents configuration for an LSP server
type LSPServerConfig struct {
	Name        string                 `json:"name"`      // Display name
	Command     string                 `json:"command"`   // Command to start the server
	Args        []string               `json:"args"`      // Command arguments
	Languages   []string               `json:"languages"` // Supported language IDs
	RootURI     string                 `json:"root_uri"`  // Workspace root URI
	InitOptions map[string]interface{} `json:"init_options,omitempty"`
	Settings    map[string]interface{} `json:"settings,omitempty"`
	Enabled     bool                   `json:"enabled"`
}

// LSPConfig holds all LSP-related configuration
type LSPConfig struct {
	Enabled      bool                        `json:"enabled"`
	Servers      map[string]*LSPServerConfig `json:"servers"`       // Server name -> config
	AutoDiscover bool                        `json:"auto_discover"` // Auto-detect LSP servers
	Timeout      int                         `json:"timeout"`       // Connection timeout in seconds
}

// NewLSPConfig creates a new LSP configuration with defaults
func NewLSPConfig() *LSPConfig {
	return &LSPConfig{
		Enabled:      false,
		Servers:      make(map[string]*LSPServerConfig),
		AutoDiscover: true,
		Timeout:      30,
	}
}

// LSPServerRegistry manages available LSP servers
type LSPServerRegistry struct {
	mu       sync.RWMutex
	servers  map[string]*LSPServerConfig
	detected map[string]bool // Cache of detected server availability
}

// NewLSPServerRegistry creates a new LSP server registry
func NewLSPServerRegistry() *LSPServerRegistry {
	return &LSPServerRegistry{
		servers:  make(map[string]*LSPServerConfig),
		detected: make(map[string]bool),
	}
}

// RegisterServer registers an LSP server configuration
func (r *LSPServerRegistry) RegisterServer(config *LSPServerConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers[config.Name] = config
}

// GetServer returns the configuration for a server
func (r *LSPServerRegistry) GetServer(name string) *LSPServerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers[name]
}

// GetServersForLanguage returns all servers that support a language
func (r *LSPServerRegistry) GetServersForLanguage(languageID string) []*LSPServerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var servers []*LSPServerConfig
	for _, server := range r.servers {
		for _, lang := range server.Languages {
			if lang == languageID {
				servers = append(servers, server)
				break
			}
		}
	}
	return servers
}

// IsServerAvailable checks if an LSP server command is available
func (r *LSPServerRegistry) IsServerAvailable(name string) bool {
	r.mu.RLock()
	if detected, ok := r.detected[name]; ok {
		r.mu.RUnlock()
		return detected
	}
	r.mu.RUnlock()

	server := r.GetServer(name)
	if server == nil {
		return false
	}

	// Check if command exists
	_, err := exec.LookPath(server.Command)
	available := err == nil

	r.mu.Lock()
	r.detected[name] = available
	r.mu.Unlock()

	return available
}

// DetectServers checks which configured servers are available
func (r *LSPServerRegistry) DetectServers() map[string]bool {
	r.mu.RLock()
	serverNames := make([]string, 0, len(r.servers))
	for name := range r.servers {
		serverNames = append(serverNames, name)
	}
	r.mu.RUnlock()

	results := make(map[string]bool)
	for _, name := range serverNames {
		results[name] = r.IsServerAvailable(name)
	}
	return results
}

// DefaultLSPServers returns the default LSP server configurations
func DefaultLSPServers() []*LSPServerConfig {
	return []*LSPServerConfig{
		{
			Name:      "gopls",
			Command:   "gopls",
			Args:      []string{"serve"},
			Languages: []string{"go"},
			Enabled:   true,
		},
		{
			Name:      "typescript-language-server",
			Command:   "typescript-language-server",
			Args:      []string{"--stdio"},
			Languages: []string{"javascript", "javascriptreact", "typescript", "typescriptreact"},
			Enabled:   true,
		},
		{
			Name:      "pylsp",
			Command:   "pylsp",
			Args:      []string{},
			Languages: []string{"python"},
			Enabled:   true,
		},
		{
			Name:      "rust-analyzer",
			Command:   "rust-analyzer",
			Args:      []string{},
			Languages: []string{"rust"},
			Enabled:   true,
		},
		{
			Name:      "clangd",
			Command:   "clangd",
			Args:      []string{},
			Languages: []string{"c", "cpp"},
			Enabled:   true,
		},
		{
			Name:      "bash-language-server",
			Command:   "bash-language-server",
			Args:      []string{"start"},
			Languages: []string{"shellscript"},
			Enabled:   true,
		},
		{
			Name:      "yaml-language-server",
			Command:   "yaml-language-server",
			Args:      []string{"--stdio"},
			Languages: []string{"yaml"},
			Enabled:   true,
		},
		{
			Name:      "lua-language-server",
			Command:   "lua-language-server",
			Args:      []string{},
			Languages: []string{"lua"},
			Enabled:   true,
		},
	}
}

// LSPDiagnostic represents a diagnostic from an LSP server
type LSPDiagnostic struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	EndLine  int    `json:"end_line"`
	EndCol   int    `json:"end_column"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning", "info", "hint"
	Source   string `json:"source"`   // LSP server name
	Code     string `json:"code,omitempty"`
}

// LSPLocation represents a location in code
type LSPLocation struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_column"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_column"`
}

// LSPSymbol represents a symbol in code
type LSPSymbol struct {
	Name     string      `json:"name"`
	Kind     string      `json:"kind"` // "function", "class", "variable", etc.
	Location LSPLocation `json:"location"`
	Children []LSPSymbol `json:"children,omitempty"`
}

// LSPManager manages LSP servers and provides code intelligence
type LSPManager struct {
	registry    *LSPServerRegistry
	extRegistry *ExtensionRegistry
	config      *LSPConfig
	workspace   string
}

// NewLSPManager creates a new LSP manager
func NewLSPManager(config *LSPConfig, extRegistry *ExtensionRegistry, workspace string) *LSPManager {
	registry := NewLSPServerRegistry()

	// Register default servers
	for _, server := range DefaultLSPServers() {
		registry.RegisterServer(server)
	}

	// Register user-configured servers
	if config != nil && config.Servers != nil {
		for _, server := range config.Servers {
			registry.RegisterServer(server)
		}
	}

	return &LSPManager{
		registry:    registry,
		extRegistry: extRegistry,
		config:      config,
		workspace:   workspace,
	}
}

// GetAvailableServers returns a list of available LSP servers
func (m *LSPManager) GetAvailableServers() []string {
	detected := m.registry.DetectServers()
	available := make([]string, 0)
	for name, ok := range detected {
		if ok {
			available = append(available, name)
		}
	}
	return available
}

// GetServerForLanguage returns the best available server for a language
func (m *LSPManager) GetServerForLanguage(languageID string) *LSPServerConfig {
	servers := m.registry.GetServersForLanguage(languageID)
	for _, server := range servers {
		if m.registry.IsServerAvailable(server.Name) && server.Enabled {
			return server
		}
	}
	return nil
}

// GetDiagnosticsCommand returns a command to get diagnostics for a file
// This is a helper for tool integration - actual LSP communication would be more complex
func (m *LSPManager) GetDiagnosticsCommand(filePath string) (string, []string, error) {
	lang := m.extRegistry.GetLanguageForPath(filePath)
	if lang == nil {
		return "", nil, fmt.Errorf("unknown language for file: %s", filePath)
	}

	server := m.GetServerForLanguage(lang.ID)
	if server == nil {
		return "", nil, fmt.Errorf("no LSP server available for language: %s", lang.ID)
	}

	return server.Command, server.Args, nil
}

// FormatDiagnosticsForPrompt formats diagnostics for inclusion in LLM prompts
func FormatDiagnosticsForPrompt(diagnostics []LSPDiagnostic) string {
	if len(diagnostics) == 0 {
		return "No diagnostics found."
	}

	var sb strings.Builder
	sb.WriteString("## Code Diagnostics\n\n")

	// Group by file
	byFile := make(map[string][]LSPDiagnostic)
	for _, d := range diagnostics {
		byFile[d.FilePath] = append(byFile[d.FilePath], d)
	}

	for file, diags := range byFile {
		sb.WriteString(fmt.Sprintf("### %s\n", file))
		for _, d := range diags {
			sb.WriteString(fmt.Sprintf("- **%s** (line %d): %s\n",
				strings.ToUpper(d.Severity), d.Line, d.Message))
			if d.Code != "" {
				sb.WriteString(fmt.Sprintf("  Code: %s\n", d.Code))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// LSPCapabilities represents what capabilities are available
type LSPCapabilities struct {
	Diagnostics      bool `json:"diagnostics"`
	Completion       bool `json:"completion"`
	Hover            bool `json:"hover"`
	Definition       bool `json:"definition"`
	References       bool `json:"references"`
	DocumentSymbols  bool `json:"document_symbols"`
	WorkspaceSymbols bool `json:"workspace_symbols"`
	Rename           bool `json:"rename"`
	CodeActions      bool `json:"code_actions"`
	Formatting       bool `json:"formatting"`
}

// LSPStatus represents the status of an LSP server
type LSPStatus struct {
	Name         string          `json:"name"`
	Language     string          `json:"language"`
	Running      bool            `json:"running"`
	Capabilities LSPCapabilities `json:"capabilities"`
	LastActivity time.Time       `json:"last_activity"`
	Error        string          `json:"error,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for LSPConfig
func (c *LSPConfig) MarshalJSON() ([]byte, error) {
	type Alias LSPConfig
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for LSPConfig
func (c *LSPConfig) UnmarshalJSON(data []byte) error {
	type Alias LSPConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}

// ValidateConfig validates the LSP configuration
func (c *LSPConfig) ValidateConfig() error {
	if c.Timeout < 1 {
		return fmt.Errorf("LSP timeout must be at least 1 second")
	}
	if c.Timeout > 300 {
		return fmt.Errorf("LSP timeout cannot exceed 300 seconds")
	}

	for name, server := range c.Servers {
		if server.Command == "" {
			return fmt.Errorf("LSP server %s has no command specified", name)
		}
		if len(server.Languages) == 0 {
			return fmt.Errorf("LSP server %s has no languages specified", name)
		}
	}

	return nil
}

// GetDefaultConfig returns a default LSP configuration
func GetDefaultLSPConfig() *LSPConfig {
	config := NewLSPConfig()
	config.AutoDiscover = true
	config.Timeout = 30

	// Add default servers
	for _, server := range DefaultLSPServers() {
		config.Servers[server.Name] = server
	}

	return config
}

// LSPInitOptions contains common LSP initialization options
type LSPInitOptions struct {
	RootURI      string `json:"rootUri"`
	RootPath     string `json:"rootPath"`
	WorkspaceDir string `json:"workspaceFolder"`
}

// NewLSPInitOptions creates initialization options for a workspace
func NewLSPInitOptions(workspaceDir string) *LSPInitOptions {
	return &LSPInitOptions{
		RootURI:      "file://" + workspaceDir,
		RootPath:     workspaceDir,
		WorkspaceDir: workspaceDir,
	}
}

// ExecuteServerCommand executes an LSP server command and returns output
// This is a simplified helper - real LSP communication uses JSON-RPC
func ExecuteServerCommand(ctx context.Context, config *LSPServerConfig, input string) (string, error) {
	cmd := exec.CommandContext(ctx, config.Command, config.Args...)

	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("LSP server error: %w - output: %s", err, string(output))
	}

	return string(output), nil
}
