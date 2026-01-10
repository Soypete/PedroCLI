package permission

import (
	"testing"
)

func TestNewPermissionManager(t *testing.T) {
	manager := NewPermissionManager()

	if manager == nil {
		t.Fatal("expected manager to be created")
	}
}

func TestPermissionManager_Check(t *testing.T) {
	manager := NewPermissionManager()

	// Default should be allow
	if manager.Check("unknown_tool") != PermissionAllow {
		t.Error("expected default permission to be allow")
	}

	// Set specific tool permission
	manager.SetToolPermission("bash", PermissionAsk)
	if manager.Check("bash") != PermissionAsk {
		t.Error("expected bash permission to be ask")
	}

	manager.SetToolPermission("edit", PermissionAllow)
	if manager.Check("edit") != PermissionAllow {
		t.Error("expected edit permission to be allow")
	}
}

func TestPermissionManager_Patterns(t *testing.T) {
	manager := NewPermissionManager()

	// Set pattern-based permission
	manager.SetPattern("mcp_*", PermissionAsk)

	if manager.Check("mcp_github") != PermissionAsk {
		t.Error("expected mcp_github to match pattern")
	}
	if manager.Check("mcp_notion") != PermissionAsk {
		t.Error("expected mcp_notion to match pattern")
	}
	if manager.Check("bash") != PermissionAllow {
		t.Error("expected bash to not match pattern")
	}
}

func TestPermissionManager_BashCommands(t *testing.T) {
	manager := NewPermissionManager()

	// Set bash command permissions
	manager.SetBashCommand("git *", PermissionAllow)
	manager.SetBashCommand("rm -rf*", PermissionDeny)
	manager.SetBashCommand("sudo *", PermissionDeny)

	// Check bash command permissions
	if manager.Check("bash", "git status") != PermissionAllow {
		t.Error("expected 'git status' to be allowed")
	}
	if manager.Check("bash", "rm -rf /") != PermissionDeny {
		t.Error("expected 'rm -rf /' to be denied")
	}
	if manager.Check("bash", "sudo apt install") != PermissionDeny {
		t.Error("expected sudo command to be denied")
	}
}

func TestPermissionManager_SessionOverrides(t *testing.T) {
	manager := NewPermissionManager()

	// Set base permission to ask
	manager.SetToolPermission("bash", PermissionAsk)

	// Verify it's ask
	if manager.Check("bash") != PermissionAsk {
		t.Error("expected bash to be ask")
	}

	// Set session override
	manager.SetSessionOverride("bash", PermissionAllow)

	// Session override should take precedence
	if manager.Check("bash") != PermissionAllow {
		t.Error("expected session override to take precedence")
	}

	// Clear overrides
	manager.ClearSessionOverrides()

	// Should be back to ask
	if manager.Check("bash") != PermissionAsk {
		t.Error("expected bash to be ask after clearing overrides")
	}
}

func TestPermissionManager_CheckSkill(t *testing.T) {
	manager := NewPermissionManager()

	// Default should be allow
	if manager.CheckSkill("git-release") != PermissionAllow {
		t.Error("expected default skill permission to be allow")
	}

	// Set skill permission
	manager.SetSkillPermission("internal-*", PermissionDeny)

	if manager.CheckSkill("internal-debug") != PermissionDeny {
		t.Error("expected internal-debug to be denied")
	}
	if manager.CheckSkill("git-release") != PermissionAllow {
		t.Error("expected git-release to be allowed")
	}
}

func TestPermissionManager_LoadFromConfig(t *testing.T) {
	manager := NewPermissionManager()

	cfg := PermissionConfig{
		Edit: "allow",
		Bash: "ask",
		Git:  "allow",
		Patterns: map[string]string{
			"mcp_*": "ask",
		},
		BashCommands: map[string]string{
			"rm -rf*": "deny",
		},
	}

	manager.LoadFromConfig(cfg)

	if manager.Check("edit") != PermissionAllow {
		t.Error("expected edit to be allow from config")
	}
	if manager.Check("bash") != PermissionAsk {
		t.Error("expected bash to be ask from config")
	}
	if manager.Check("mcp_github") != PermissionAsk {
		t.Error("expected mcp_github to be ask from pattern")
	}
	if manager.Check("bash", "rm -rf /tmp") != PermissionDeny {
		t.Error("expected rm -rf to be denied from config")
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		str     string
		want    bool
	}{
		{"*", "anything", true},
		{"mcp_*", "mcp_github", true},
		{"mcp_*", "bash", false},
		{"*_tool", "my_tool", true},
		{"*_tool", "my_tools", false},
		{"prefix_*_suffix", "prefix_middle_suffix", true},
		{"prefix_*_suffix", "prefix_suffix", false},
		{"exact", "exact", true},
		{"exact", "different", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.str, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.str)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.str, got, tt.want)
			}
		})
	}
}

func TestPermissionManager_GetAllPermissions(t *testing.T) {
	manager := NewPermissionManager()

	manager.SetToolPermission("bash", PermissionAsk)
	manager.SetToolPermission("edit", PermissionAllow)
	manager.SetPattern("mcp_*", PermissionAsk)
	manager.SetBashCommand("rm -rf*", PermissionDeny)

	allPerms := manager.GetAllPermissions()

	if allPerms["bash"] != PermissionAsk {
		t.Error("expected bash in permissions")
	}
	if allPerms["edit"] != PermissionAllow {
		t.Error("expected edit in permissions")
	}
	if allPerms["pattern:mcp_*"] != PermissionAsk {
		t.Error("expected pattern in permissions")
	}
	if allPerms["bash:rm -rf*"] != PermissionDeny {
		t.Error("expected bash command in permissions")
	}
}
