// Package permission provides granular permission management for tools.
package permission

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Permission represents a permission level
type Permission string

const (
	PermissionAllow Permission = "allow"
	PermissionDeny  Permission = "deny"
	PermissionAsk   Permission = "ask"
)

// PermissionManager handles tool permission checks
type PermissionManager struct {
	mu sync.RWMutex

	// Global tool permissions
	toolPermissions map[string]Permission

	// Pattern-based permissions (e.g., "mcp_*": "ask")
	patterns map[string]Permission

	// Command-specific bash permissions
	bashCommands map[string]Permission

	// Skill permissions
	skillPermissions map[string]Permission

	// Session-level overrides (temporary "allow for this session")
	sessionOverrides map[string]Permission

	// Interactive mode (can ask user for permission)
	interactive bool

	// Callbacks
	onAsk func(tool, description string) bool
}

// NewPermissionManager creates a new permission manager
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{
		toolPermissions:  make(map[string]Permission),
		patterns:         make(map[string]Permission),
		bashCommands:     make(map[string]Permission),
		skillPermissions: make(map[string]Permission),
		sessionOverrides: make(map[string]Permission),
		interactive:      true,
	}
}

// SetInteractive sets whether the manager can prompt for permissions
func (m *PermissionManager) SetInteractive(interactive bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.interactive = interactive
}

// SetOnAsk sets a custom callback for permission requests
func (m *PermissionManager) SetOnAsk(fn func(tool, description string) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAsk = fn
}

// SetToolPermission sets a permission for a specific tool
func (m *PermissionManager) SetToolPermission(tool string, perm Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolPermissions[tool] = perm
}

// SetPattern sets a pattern-based permission
func (m *PermissionManager) SetPattern(pattern string, perm Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patterns[pattern] = perm
}

// SetBashCommand sets a bash command-specific permission
func (m *PermissionManager) SetBashCommand(pattern string, perm Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bashCommands[pattern] = perm
}

// SetSkillPermission sets a skill-specific permission
func (m *PermissionManager) SetSkillPermission(pattern string, perm Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skillPermissions[pattern] = perm
}

// SetSessionOverride sets a temporary session-level override
func (m *PermissionManager) SetSessionOverride(tool string, perm Permission) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionOverrides[tool] = perm
}

// ClearSessionOverrides clears all session-level overrides
func (m *PermissionManager) ClearSessionOverrides() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionOverrides = make(map[string]Permission)
}

// Check checks if a tool is allowed to execute
func (m *PermissionManager) Check(tool string, args ...string) Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check session overrides first
	if perm, ok := m.sessionOverrides[tool]; ok {
		return perm
	}

	// For bash commands, check command-specific permissions
	if tool == "bash" && len(args) > 0 {
		cmd := args[0]
		if perm := m.checkBashCommand(cmd); perm != "" {
			return perm
		}
	}

	// Check direct tool permission
	if perm, ok := m.toolPermissions[tool]; ok {
		return perm
	}

	// Check pattern-based permissions
	for pattern, perm := range m.patterns {
		if matchGlob(pattern, tool) {
			return perm
		}
	}

	// Default to allow
	return PermissionAllow
}

// CheckSkill checks if a skill is allowed to be loaded
func (m *PermissionManager) CheckSkill(skillName string) Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check direct skill permission
	if perm, ok := m.skillPermissions[skillName]; ok {
		return perm
	}

	// Check pattern-based skill permissions
	for pattern, perm := range m.skillPermissions {
		if matchGlob(pattern, skillName) {
			return perm
		}
	}

	return PermissionAllow
}

// checkBashCommand checks permissions for a specific bash command
func (m *PermissionManager) checkBashCommand(cmd string) Permission {
	// Check exact match first
	if perm, ok := m.bashCommands[cmd]; ok {
		return perm
	}

	// Check patterns
	for pattern, perm := range m.bashCommands {
		if matchGlob(pattern, cmd) {
			return perm
		}
	}

	return ""
}

// RequestApproval requests permission from the user
func (m *PermissionManager) RequestApproval(tool string, description string) bool {
	m.mu.RLock()
	interactive := m.interactive
	onAsk := m.onAsk
	m.mu.RUnlock()

	if !interactive {
		return false
	}

	// Use custom callback if set
	if onAsk != nil {
		return onAsk(tool, description)
	}

	// Default terminal-based prompt
	return m.terminalPrompt(tool, description)
}

// terminalPrompt prompts the user via terminal
func (m *PermissionManager) terminalPrompt(tool string, description string) bool {
	fmt.Printf("\n\033[33m[Permission Required]\033[0m %s\n", tool)
	if description != "" {
		fmt.Printf("   %s\n", description)
	}
	fmt.Print("   Allow? [y/N/a(lways)]: ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "y", "yes":
		return true
	case "a", "always":
		// Set session override
		m.SetSessionOverride(tool, PermissionAllow)
		return true
	default:
		return false
	}
}

// CheckAndRequest checks permission and requests approval if needed
func (m *PermissionManager) CheckAndRequest(tool string, description string, args ...string) (bool, error) {
	perm := m.Check(tool, args...)

	switch perm {
	case PermissionAllow:
		return true, nil
	case PermissionDeny:
		return false, fmt.Errorf("permission denied for tool: %s", tool)
	case PermissionAsk:
		if m.RequestApproval(tool, description) {
			return true, nil
		}
		return false, fmt.Errorf("permission rejected by user for tool: %s", tool)
	default:
		return true, nil
	}
}

// LoadFromConfig loads permissions from configuration
func (m *PermissionManager) LoadFromConfig(cfg PermissionConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load tool permissions
	if cfg.Edit != "" {
		m.toolPermissions["edit"] = Permission(cfg.Edit)
		m.toolPermissions["code_edit"] = Permission(cfg.Edit)
	}
	if cfg.Write != "" {
		m.toolPermissions["write"] = Permission(cfg.Write)
		m.toolPermissions["file_write"] = Permission(cfg.Write)
	}
	if cfg.Bash != "" {
		m.toolPermissions["bash"] = Permission(cfg.Bash)
	}
	if cfg.WebFetch != "" {
		m.toolPermissions["webfetch"] = Permission(cfg.WebFetch)
		m.toolPermissions["webscrape"] = Permission(cfg.WebFetch)
	}
	if cfg.Git != "" {
		m.toolPermissions["git"] = Permission(cfg.Git)
	}

	// Load patterns
	for pattern, perm := range cfg.Patterns {
		m.patterns[pattern] = Permission(perm)
	}

	// Load bash commands
	for pattern, perm := range cfg.BashCommands {
		m.bashCommands[pattern] = Permission(perm)
	}

	// Load skill permissions
	for pattern, perm := range cfg.Skills {
		m.skillPermissions[pattern] = Permission(perm)
	}
}

// PermissionConfig mirrors config.PermissionConfig for loading
type PermissionConfig struct {
	Edit         string
	Write        string
	Bash         string
	WebFetch     string
	Git          string
	Patterns     map[string]string
	BashCommands map[string]string
	Skills       map[string]string
}

// matchGlob performs simple glob pattern matching
// Supports * for any characters and ? for single character
func matchGlob(pattern, str string) bool {
	// Simple implementation - can be enhanced with proper glob library
	if pattern == "*" {
		return true
	}

	// Convert glob pattern to components
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcards, exact match
		return pattern == str
	}

	// Handle prefix
	if !strings.HasPrefix(str, parts[0]) {
		return false
	}
	str = str[len(parts[0]):]

	// Handle suffix
	lastPart := parts[len(parts)-1]
	if !strings.HasSuffix(str, lastPart) {
		return false
	}
	str = str[:len(str)-len(lastPart)]

	// Handle middle parts
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(str, parts[i])
		if idx == -1 {
			return false
		}
		str = str[idx+len(parts[i]):]
	}

	return true
}

// GetAllPermissions returns a summary of all configured permissions
func (m *PermissionManager) GetAllPermissions() map[string]Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Permission)

	// Copy tool permissions
	for k, v := range m.toolPermissions {
		result[k] = v
	}

	// Copy patterns (with prefix for clarity)
	for k, v := range m.patterns {
		result["pattern:"+k] = v
	}

	// Copy bash commands
	for k, v := range m.bashCommands {
		result["bash:"+k] = v
	}

	return result
}

// ToolPermissionWrapper wraps a tool execution with permission checking
type ToolPermissionWrapper struct {
	manager *PermissionManager
}

// NewToolPermissionWrapper creates a new permission wrapper
func NewToolPermissionWrapper(manager *PermissionManager) *ToolPermissionWrapper {
	return &ToolPermissionWrapper{
		manager: manager,
	}
}

// Wrap wraps a tool execution function with permission checking
func (w *ToolPermissionWrapper) Wrap(toolName string, description string, execute func() error) error {
	allowed, err := w.manager.CheckAndRequest(toolName, description)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("permission denied for tool: %s", toolName)
	}
	return execute()
}

// SaveState saves the current permission state to a file
func (m *PermissionManager) SaveState(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write session overrides
	for tool, perm := range m.sessionOverrides {
		fmt.Fprintf(file, "session:%s=%s\n", tool, perm)
	}

	return nil
}

// LoadState loads permission state from a file
func (m *PermissionManager) LoadState(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "session:") {
			parts := strings.SplitN(strings.TrimPrefix(line, "session:"), "=", 2)
			if len(parts) == 2 {
				m.SetSessionOverride(parts[0], Permission(parts[1]))
			}
		}
	}

	return scanner.Err()
}
