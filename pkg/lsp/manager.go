package lsp

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/soypete/pedrocli/pkg/config"
)

// Manager manages multiple LSP clients for different languages.
type Manager struct {
	config    *config.LSPConfig
	workspace string
	clients   map[string]*Client // keyed by language name
	mu        sync.RWMutex
	timeout   time.Duration
}

// NewManager creates a new LSP manager.
func NewManager(cfg *config.LSPConfig, workspace string) *Manager {
	timeout := 30 * time.Second
	if cfg != nil && cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &Manager{
		config:    cfg,
		workspace: workspace,
		clients:   make(map[string]*Client),
		timeout:   timeout,
	}
}

// GetClient returns or starts the appropriate client for a file.
func (m *Manager) GetClient(ctx context.Context, filePath string) (*Client, error) {
	language := m.detectLanguage(filePath)
	if language == "" {
		return nil, fmt.Errorf("unable to detect language for file: %s", filePath)
	}

	return m.GetClientForLanguage(ctx, language)
}

// GetClientForLanguage returns or starts a client for a specific language.
func (m *Manager) GetClientForLanguage(ctx context.Context, language string) (*Client, error) {
	m.mu.RLock()
	client, exists := m.clients[language]
	m.mu.RUnlock()

	if exists && client.IsReady() {
		return client, nil
	}

	// Create new client
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists = m.clients[language]; exists && client.IsReady() {
		return client, nil
	}

	// Get server config
	serverCfg := m.getServerConfig(language)
	if serverCfg == nil {
		return nil, fmt.Errorf("no LSP server configured for language: %s", language)
	}

	if !serverCfg.Enabled {
		return nil, fmt.Errorf("LSP server for %s is disabled", language)
	}

	// Check if server command is available
	if _, err := exec.LookPath(serverCfg.Command); err != nil {
		return nil, fmt.Errorf("LSP server command not found: %s (install %s)", serverCfg.Command, serverCfg.Command)
	}

	// Create client with timeout context
	clientCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	newClient, err := NewClient(clientCtx, serverCfg.Command, serverCfg.Args, m.workspace, language)
	if err != nil {
		return nil, fmt.Errorf("failed to start LSP server for %s: %w", language, err)
	}

	m.clients[language] = newClient
	return newClient, nil
}

// Definition returns the definition location for a symbol.
func (m *Manager) Definition(ctx context.Context, filePath string, line, col int) ([]LocationResult, error) {
	client, err := m.GetClient(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return client.Definition(ctx, filePath, line, col)
}

// References returns all references to a symbol.
func (m *Manager) References(ctx context.Context, filePath string, line, col int) ([]LocationResult, error) {
	client, err := m.GetClient(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return client.References(ctx, filePath, line, col, true)
}

// Hover returns hover information for a symbol.
func (m *Manager) Hover(ctx context.Context, filePath string, line, col int) (string, error) {
	client, err := m.GetClient(ctx, filePath)
	if err != nil {
		return "", err
	}
	return client.Hover(ctx, filePath, line, col)
}

// Diagnostics returns diagnostics for a file.
func (m *Manager) Diagnostics(ctx context.Context, filePath string) ([]DiagnosticResult, error) {
	client, err := m.GetClient(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return client.GetDiagnostics(ctx, filePath)
}

// Symbols returns symbols for a file or workspace.
func (m *Manager) Symbols(ctx context.Context, filePath string, scope string) ([]SymbolResult, error) {
	client, err := m.GetClient(ctx, filePath)
	if err != nil {
		return nil, err
	}

	if scope == "workspace" {
		// For workspace symbols, use empty query to get all
		return client.WorkspaceSymbols(ctx, "")
	}

	return client.DocumentSymbols(ctx, filePath)
}

// Shutdown gracefully stops all language servers.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for lang, client := range m.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", lang, err))
		}
	}
	m.clients = make(map[string]*Client)

	if len(errs) > 0 {
		return fmt.Errorf("errors shutting down LSP servers: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetAvailableServers returns a list of available LSP servers.
func (m *Manager) GetAvailableServers() []string {
	available := []string{}

	for _, serverCfg := range m.getDefaultServers() {
		if _, err := exec.LookPath(serverCfg.Command); err == nil {
			available = append(available, serverCfg.Name)
		}
	}

	// Add configured servers
	if m.config != nil && m.config.Servers != nil {
		for name, serverCfg := range m.config.Servers {
			if serverCfg.Enabled {
				if _, err := exec.LookPath(serverCfg.Command); err == nil {
					available = append(available, name)
				}
			}
		}
	}

	return available
}

// detectLanguage detects the language for a file based on extension.
func (m *Manager) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".jsx":   "javascriptreact",
		".ts":    "typescript",
		".tsx":   "typescriptreact",
		".rs":    "rust",
		".c":     "c",
		".h":     "c",
		".cpp":   "cpp",
		".hpp":   "cpp",
		".cc":    "cpp",
		".java":  "java",
		".rb":    "ruby",
		".php":   "php",
		".sh":    "shellscript",
		".bash":  "shellscript",
		".yaml":  "yaml",
		".yml":   "yaml",
		".json":  "json",
		".lua":   "lua",
		".zig":   "zig",
		".swift": "swift",
		".kt":    "kotlin",
		".kts":   "kotlin",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	return ""
}

// getServerConfig gets the server configuration for a language.
func (m *Manager) getServerConfig(language string) *config.LSPServerDef {
	// Check user-configured servers first
	if m.config != nil && m.config.Servers != nil {
		for _, serverCfg := range m.config.Servers {
			for _, lang := range serverCfg.Languages {
				if lang == language {
					return &serverCfg
				}
			}
		}
	}

	// Fall back to default servers
	defaults := m.getDefaultServers()
	for _, serverCfg := range defaults {
		for _, lang := range serverCfg.Languages {
			if lang == language {
				return &config.LSPServerDef{
					Command:   serverCfg.Command,
					Args:      serverCfg.Args,
					Languages: serverCfg.Languages,
					Enabled:   true,
				}
			}
		}
	}

	return nil
}

// ServerConfig represents a default server configuration.
type ServerConfig struct {
	Name      string
	Command   string
	Args      []string
	Languages []string
}

// getDefaultServers returns default LSP server configurations.
func (m *Manager) getDefaultServers() []ServerConfig {
	return []ServerConfig{
		{
			Name:      "gopls",
			Command:   "gopls",
			Args:      []string{"serve"},
			Languages: []string{"go"},
		},
		{
			Name:      "typescript-language-server",
			Command:   "typescript-language-server",
			Args:      []string{"--stdio"},
			Languages: []string{"javascript", "javascriptreact", "typescript", "typescriptreact"},
		},
		{
			Name:      "pylsp",
			Command:   "pylsp",
			Args:      []string{},
			Languages: []string{"python"},
		},
		{
			Name:      "rust-analyzer",
			Command:   "rust-analyzer",
			Args:      []string{},
			Languages: []string{"rust"},
		},
		{
			Name:      "clangd",
			Command:   "clangd",
			Args:      []string{},
			Languages: []string{"c", "cpp"},
		},
		{
			Name:      "bash-language-server",
			Command:   "bash-language-server",
			Args:      []string{"start"},
			Languages: []string{"shellscript"},
		},
		{
			Name:      "yaml-language-server",
			Command:   "yaml-language-server",
			Args:      []string{"--stdio"},
			Languages: []string{"yaml"},
		},
		{
			Name:      "lua-language-server",
			Command:   "lua-language-server",
			Args:      []string{},
			Languages: []string{"lua"},
		},
		{
			Name:      "zls",
			Command:   "zls",
			Args:      []string{},
			Languages: []string{"zig"},
		},
		{
			Name:      "jdtls",
			Command:   "jdtls",
			Args:      []string{},
			Languages: []string{"java"},
		},
	}
}
