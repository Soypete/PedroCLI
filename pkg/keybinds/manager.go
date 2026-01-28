// Package keybinds provides configurable keyboard shortcut management with leader key support.
package keybinds

import (
	"strings"
	"sync"
)

// Action represents a keybind action
type Action string

const (
	ActionAppExit            Action = "app_exit"
	ActionSessionNew         Action = "session_new"
	ActionSessionList        Action = "session_list"
	ActionAgentCycle         Action = "agent_cycle"
	ActionAgentCycleReverse  Action = "agent_cycle_reverse"
	ActionModelList          Action = "model_list"
	ActionCommandList        Action = "command_list"
	ActionInputSubmit        Action = "input_submit"
	ActionInputNewline       Action = "input_newline"
	ActionMessagesUndo       Action = "messages_undo"
	ActionMessagesRedo       Action = "messages_redo"
	ActionInputClear         Action = "input_clear"
	ActionHistoryPrev        Action = "history_prev"
	ActionHistoryNext        Action = "history_next"
	ActionAutocompleteToggle Action = "autocomplete_toggle"
	ActionHelpToggle         Action = "help_toggle"
)

// KeybindManager manages keyboard shortcuts
type KeybindManager struct {
	mu sync.RWMutex

	// Leader key (default: ctrl+x)
	leader string

	// Action to key bindings (action -> list of keys)
	bindings map[Action][]string

	// Reverse lookup: key -> action
	keyToAction map[string]Action

	// Action handlers
	handlers map[Action]func()

	// Leader key state
	leaderActive   bool
	leaderSequence []string
}

// NewKeybindManager creates a new keybind manager with defaults
func NewKeybindManager() *KeybindManager {
	m := &KeybindManager{
		leader:      "ctrl+x",
		bindings:    make(map[Action][]string),
		keyToAction: make(map[string]Action),
		handlers:    make(map[Action]func()),
	}
	m.loadDefaults()
	return m
}

// loadDefaults sets the default key bindings
func (m *KeybindManager) loadDefaults() {
	defaults := map[Action]string{
		ActionAppExit:            "ctrl+c,ctrl+d,<leader>q",
		ActionSessionNew:         "<leader>n",
		ActionSessionList:        "<leader>l",
		ActionAgentCycle:         "tab",
		ActionAgentCycleReverse:  "shift+tab",
		ActionModelList:          "<leader>m",
		ActionCommandList:        "ctrl+p",
		ActionInputSubmit:        "return",
		ActionInputNewline:       "shift+return,ctrl+return",
		ActionMessagesUndo:       "<leader>u",
		ActionMessagesRedo:       "<leader>r",
		ActionInputClear:         "ctrl+c",
		ActionHistoryPrev:        "up",
		ActionHistoryNext:        "down",
		ActionAutocompleteToggle: "ctrl+space",
		ActionHelpToggle:         "<leader>?",
	}

	for action, keys := range defaults {
		m.SetBinding(action, keys)
	}
}

// SetLeader sets the leader key
func (m *KeybindManager) SetLeader(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leader = key
	m.rebuildKeyToAction()
}

// GetLeader returns the current leader key
func (m *KeybindManager) GetLeader() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.leader
}

// SetBinding sets the key binding for an action
func (m *KeybindManager) SetBinding(action Action, keys string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse keys (comma-separated)
	keyList := parseKeys(keys)
	m.bindings[action] = keyList

	// Rebuild reverse lookup
	m.rebuildKeyToAction()
}

// GetBinding returns the key binding for an action
func (m *KeybindManager) GetBinding(action Action) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bindings[action]
}

// GetBindingString returns the key binding as a display string
func (m *KeybindManager) GetBindingString(action Action) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := m.bindings[action]
	if len(keys) == 0 {
		return ""
	}
	// Return the first binding, expanded
	return m.expandLeader(keys[0])
}

// RegisterHandler registers a handler for an action
func (m *KeybindManager) RegisterHandler(action Action, handler func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[action] = handler
}

// HandleKey processes a key press and returns true if handled
func (m *KeybindManager) HandleKey(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedKey := normalizeKey(key)

	// Check if this is the leader key
	if normalizedKey == m.leader {
		m.leaderActive = true
		m.leaderSequence = []string{}
		return true
	}

	// If leader is active, build the sequence
	if m.leaderActive {
		m.leaderSequence = append(m.leaderSequence, normalizedKey)
		fullKey := m.leader + " " + strings.Join(m.leaderSequence, " ")

		// Check if this matches any binding
		if action, ok := m.keyToAction[fullKey]; ok {
			m.leaderActive = false
			m.leaderSequence = nil
			if handler, ok := m.handlers[action]; ok {
				handler()
				return true
			}
		}

		// Check if this could still match something (prefix match)
		possibleMatch := false
		for boundKey := range m.keyToAction {
			if strings.HasPrefix(boundKey, fullKey) {
				possibleMatch = true
				break
			}
		}

		if !possibleMatch {
			// No possible match, reset leader state
			m.leaderActive = false
			m.leaderSequence = nil
		}

		return possibleMatch
	}

	// Check direct key bindings
	if action, ok := m.keyToAction[normalizedKey]; ok {
		if handler, ok := m.handlers[action]; ok {
			handler()
			return true
		}
	}

	return false
}

// IsLeaderActive returns whether the leader key is currently active
func (m *KeybindManager) IsLeaderActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.leaderActive
}

// CancelLeader cancels the current leader key sequence
func (m *KeybindManager) CancelLeader() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaderActive = false
	m.leaderSequence = nil
}

// GetAllBindings returns all current bindings
func (m *KeybindManager) GetAllBindings() map[Action][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[Action][]string)
	for action, keys := range m.bindings {
		keysCopy := make([]string, len(keys))
		copy(keysCopy, keys)
		result[action] = keysCopy
	}
	return result
}

// LoadFromConfig loads keybindings from configuration
func (m *KeybindManager) LoadFromConfig(cfg KeybindConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.Leader != "" {
		m.leader = cfg.Leader
	}

	if cfg.AppExit != "" {
		m.bindings[ActionAppExit] = parseKeys(cfg.AppExit)
	}
	if cfg.SessionNew != "" {
		m.bindings[ActionSessionNew] = parseKeys(cfg.SessionNew)
	}
	if cfg.SessionList != "" {
		m.bindings[ActionSessionList] = parseKeys(cfg.SessionList)
	}
	if cfg.AgentCycle != "" {
		m.bindings[ActionAgentCycle] = parseKeys(cfg.AgentCycle)
	}
	if cfg.AgentCycleReverse != "" {
		m.bindings[ActionAgentCycleReverse] = parseKeys(cfg.AgentCycleReverse)
	}
	if cfg.ModelList != "" {
		m.bindings[ActionModelList] = parseKeys(cfg.ModelList)
	}
	if cfg.CommandList != "" {
		m.bindings[ActionCommandList] = parseKeys(cfg.CommandList)
	}
	if cfg.InputSubmit != "" {
		m.bindings[ActionInputSubmit] = parseKeys(cfg.InputSubmit)
	}
	if cfg.InputNewline != "" {
		m.bindings[ActionInputNewline] = parseKeys(cfg.InputNewline)
	}
	if cfg.MessagesUndo != "" {
		m.bindings[ActionMessagesUndo] = parseKeys(cfg.MessagesUndo)
	}
	if cfg.MessagesRedo != "" {
		m.bindings[ActionMessagesRedo] = parseKeys(cfg.MessagesRedo)
	}
	if cfg.InputClear != "" {
		m.bindings[ActionInputClear] = parseKeys(cfg.InputClear)
	}
	if cfg.HistoryPrev != "" {
		m.bindings[ActionHistoryPrev] = parseKeys(cfg.HistoryPrev)
	}
	if cfg.HistoryNext != "" {
		m.bindings[ActionHistoryNext] = parseKeys(cfg.HistoryNext)
	}
	if cfg.AutocompleteToggle != "" {
		m.bindings[ActionAutocompleteToggle] = parseKeys(cfg.AutocompleteToggle)
	}
	if cfg.HelpToggle != "" {
		m.bindings[ActionHelpToggle] = parseKeys(cfg.HelpToggle)
	}

	// Handle custom bindings
	for name, keys := range cfg.Custom {
		m.bindings[Action(name)] = parseKeys(keys)
	}

	m.rebuildKeyToAction()
}

// rebuildKeyToAction rebuilds the reverse lookup map
func (m *KeybindManager) rebuildKeyToAction() {
	m.keyToAction = make(map[string]Action)
	for action, keys := range m.bindings {
		for _, key := range keys {
			// Expand <leader> placeholder
			expandedKey := m.expandLeader(key)
			m.keyToAction[expandedKey] = action
		}
	}
}

// expandLeader expands <leader> placeholder with actual leader key
func (m *KeybindManager) expandLeader(key string) string {
	return strings.ReplaceAll(key, "<leader>", m.leader+" ")
}

// parseKeys parses a comma-separated list of key bindings
func parseKeys(keys string) []string {
	if keys == "" {
		return nil
	}
	parts := strings.Split(keys, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized := normalizeKey(strings.TrimSpace(part))
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

// normalizeKey normalizes a key string for consistent matching
func normalizeKey(key string) string {
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, " ", "")

	// Normalize common aliases
	replacements := map[string]string{
		"enter":     "return",
		"esc":       "escape",
		"del":       "delete",
		"backspace": "backspace",
		"space":     "space",
		"ctrl":      "ctrl",
		"alt":       "alt",
		"shift":     "shift",
		"cmd":       "cmd",
		"super":     "super",
		"meta":      "meta",
	}

	for old, new := range replacements {
		key = strings.ReplaceAll(key, old, new)
	}

	return key
}

// KeybindConfig mirrors config.KeybindConfig for loading
type KeybindConfig struct {
	Leader             string
	AppExit            string
	SessionNew         string
	SessionList        string
	AgentCycle         string
	AgentCycleReverse  string
	ModelList          string
	CommandList        string
	InputSubmit        string
	InputNewline       string
	MessagesUndo       string
	MessagesRedo       string
	InputClear         string
	HistoryPrev        string
	HistoryNext        string
	AutocompleteToggle string
	HelpToggle         string
	Custom             map[string]string
}

// GetHelp returns help text for all keybindings
func (m *KeybindManager) GetHelp() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Keyboard Shortcuts:\n\n")
	sb.WriteString("  Leader key: " + m.leader + "\n\n")

	actions := []struct {
		action Action
		name   string
	}{
		{ActionAppExit, "Exit"},
		{ActionAgentCycle, "Next agent"},
		{ActionAgentCycleReverse, "Previous agent"},
		{ActionCommandList, "Command palette"},
		{ActionSessionNew, "New session"},
		{ActionSessionList, "List sessions"},
		{ActionModelList, "Select model"},
		{ActionInputSubmit, "Send message"},
		{ActionInputNewline, "New line"},
		{ActionInputClear, "Clear input"},
		{ActionMessagesUndo, "Undo"},
		{ActionMessagesRedo, "Redo"},
		{ActionHistoryPrev, "Previous history"},
		{ActionHistoryNext, "Next history"},
		{ActionHelpToggle, "Toggle help"},
	}

	for _, a := range actions {
		keys := m.bindings[a.action]
		if len(keys) > 0 {
			expanded := m.expandLeader(keys[0])
			sb.WriteString("  " + expanded + " - " + a.name + "\n")
		}
	}

	return sb.String()
}
