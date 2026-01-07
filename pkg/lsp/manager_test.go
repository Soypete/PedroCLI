package lsp

import (
	"testing"

	"github.com/soypete/pedrocli/pkg/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: true,
		Timeout: 30,
		Servers: map[string]config.LSPServerDef{
			"go": {
				Command:   "gopls",
				Args:      []string{"serve"},
				Languages: []string{"go"},
				Enabled:   true,
			},
		},
	}

	manager := NewManager(cfg, "/tmp/workspace")

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.workspace != "/tmp/workspace" {
		t.Errorf("workspace = %q, want %q", manager.workspace, "/tmp/workspace")
	}

	if manager.timeout.Seconds() != 30 {
		t.Errorf("timeout = %v, want 30s", manager.timeout)
	}
}

func TestNewManagerWithNilConfig(t *testing.T) {
	manager := NewManager(nil, "/tmp/workspace")

	if manager == nil {
		t.Fatal("NewManager returned nil with nil config")
	}

	// Should use default timeout
	if manager.timeout.Seconds() != 30 {
		t.Errorf("default timeout = %v, want 30s", manager.timeout)
	}
}

func TestDetectLanguage(t *testing.T) {
	manager := NewManager(nil, "/tmp/workspace")

	tests := []struct {
		file     string
		expected string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.js", "javascript"},
		{"app.jsx", "javascriptreact"},
		{"index.ts", "typescript"},
		{"app.tsx", "typescriptreact"},
		{"lib.rs", "rust"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"Main.java", "java"},
		{"script.sh", "shellscript"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"data.json", "json"},
		{"plugin.lua", "lua"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := manager.detectLanguage(tt.file)
			if result != tt.expected {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.file, result, tt.expected)
			}
		})
	}
}

func TestDetectLanguageCaseInsensitive(t *testing.T) {
	manager := NewManager(nil, "/tmp/workspace")

	// Extensions should be case-insensitive
	tests := []struct {
		file     string
		expected string
	}{
		{"main.GO", "go"},
		{"main.Go", "go"},
		{"app.PY", "python"},
		{"index.JS", "javascript"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := manager.detectLanguage(tt.file)
			if result != tt.expected {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.file, result, tt.expected)
			}
		})
	}
}

func TestGetDefaultServers(t *testing.T) {
	manager := NewManager(nil, "/tmp/workspace")

	servers := manager.getDefaultServers()

	if len(servers) == 0 {
		t.Fatal("getDefaultServers returned empty slice")
	}

	// Check that gopls is in the list
	foundGopls := false
	for _, s := range servers {
		if s.Name == "gopls" {
			foundGopls = true
			if s.Command != "gopls" {
				t.Errorf("gopls command = %q, want %q", s.Command, "gopls")
			}
			if len(s.Languages) == 0 || s.Languages[0] != "go" {
				t.Errorf("gopls languages = %v, want [go]", s.Languages)
			}
			break
		}
	}

	if !foundGopls {
		t.Error("gopls not found in default servers")
	}
}

func TestGetServerConfig(t *testing.T) {
	cfg := &config.LSPConfig{
		Enabled: true,
		Servers: map[string]config.LSPServerDef{
			"custom-go": {
				Command:   "/custom/gopls",
				Args:      []string{"serve", "-v"},
				Languages: []string{"go"},
				Enabled:   true,
			},
		},
	}

	manager := NewManager(cfg, "/tmp/workspace")

	// User-configured server should take precedence
	serverCfg := manager.getServerConfig("go")
	if serverCfg == nil {
		t.Fatal("getServerConfig returned nil for go")
	}

	if serverCfg.Command != "/custom/gopls" {
		t.Errorf("command = %q, want %q", serverCfg.Command, "/custom/gopls")
	}
}

func TestGetServerConfigFallbackToDefault(t *testing.T) {
	// Empty config should fall back to defaults
	cfg := &config.LSPConfig{
		Enabled: true,
		Servers: map[string]config.LSPServerDef{},
	}

	manager := NewManager(cfg, "/tmp/workspace")

	serverCfg := manager.getServerConfig("go")
	if serverCfg == nil {
		t.Fatal("getServerConfig returned nil for go")
	}

	if serverCfg.Command != "gopls" {
		t.Errorf("command = %q, want %q", serverCfg.Command, "gopls")
	}
}

func TestGetServerConfigUnknownLanguage(t *testing.T) {
	manager := NewManager(nil, "/tmp/workspace")

	serverCfg := manager.getServerConfig("unknown-language")
	if serverCfg != nil {
		t.Errorf("getServerConfig returned non-nil for unknown language: %+v", serverCfg)
	}
}

func TestPathToURI(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/home/user/project/main.go", "file:///home/user/project/main.go"},
		{"/tmp/test.go", "file:///tmp/test.go"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := pathToURI(tt.path)
			if result != tt.expected {
				t.Errorf("pathToURI(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"file:///home/user/project/main.go", "/home/user/project/main.go"},
		{"file:///tmp/test.go", "/tmp/test.go"},
		{"/already/a/path", "/already/a/path"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := uriToPath(tt.uri)
			if result != tt.expected {
				t.Errorf("uriToPath(%q) = %q, want %q", tt.uri, result, tt.expected)
			}
		})
	}
}
