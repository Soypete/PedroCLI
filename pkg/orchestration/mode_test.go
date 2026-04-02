package orchestration

import (
	"testing"
)

func TestDefaultModeEngine_IsToolAllowed(t *testing.T) {
	tests := []struct {
		name        string
		mode        Mode
		tool        string
		wantAllowed bool
	}{
		{
			name:        "chat mode denies code_edit",
			mode:        ModeChat,
			tool:        "code_edit",
			wantAllowed: false,
		},
		{
			name:        "chat mode allows search",
			mode:        ModeChat,
			tool:        "search",
			wantAllowed: true,
		},
		{
			name:        "plan mode denies code_edit",
			mode:        ModePlan,
			tool:        "code_edit",
			wantAllowed: false,
		},
		{
			name:        "plan mode allows git",
			mode:        ModePlan,
			tool:        "git",
			wantAllowed: true,
		},
		{
			name:        "build mode allows all tools",
			mode:        ModeBuild,
			tool:        "code_edit",
			wantAllowed: true,
		},
		{
			name:        "build mode allows bash",
			mode:        ModeBuild,
			tool:        "bash",
			wantAllowed: true,
		},
		{
			name:        "review mode denies code_edit",
			mode:        ModeReview,
			tool:        "code_edit",
			wantAllowed: false,
		},
		{
			name:        "review mode denies bash",
			mode:        ModeReview,
			tool:        "bash",
			wantAllowed: false,
		},
		{
			name:        "review mode allows search",
			mode:        ModeReview,
			tool:        "search",
			wantAllowed: true,
		},
		{
			name:        "code mode allows all tools",
			mode:        ModeCode,
			tool:        "code_edit",
			wantAllowed: true,
		},
		{
			name:        "unknown tool in restricted mode denied",
			mode:        ModeChat,
			tool:        "unknown_tool",
			wantAllowed: false,
		},
		{
			name:        "unknown tool in build mode allowed",
			mode:        ModeBuild,
			tool:        "unknown_tool",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewModeEngine()
			engine.current = tt.mode

			gotAllowed := engine.IsToolAllowed(tt.tool)
			if gotAllowed != tt.wantAllowed {
				t.Errorf("IsToolAllowed(%q) in mode %q = %v, want %v",
					tt.tool, tt.mode, gotAllowed, tt.wantAllowed)
			}
		})
	}
}

func TestDefaultModeEngine_AllowWrites(t *testing.T) {
	tests := []struct {
		name       string
		mode       Mode
		wantWrites bool
	}{
		{
			name:       "chat mode denies writes",
			mode:       ModeChat,
			wantWrites: false,
		},
		{
			name:       "plan mode denies writes",
			mode:       ModePlan,
			wantWrites: false,
		},
		{
			name:       "review mode denies writes",
			mode:       ModeReview,
			wantWrites: false,
		},
		{
			name:       "build mode allows writes",
			mode:       ModeBuild,
			wantWrites: true,
		},
		{
			name:       "code mode allows writes",
			mode:       ModeCode,
			wantWrites: true,
		},
		{
			name:       "blog mode allows writes",
			mode:       ModeBlog,
			wantWrites: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewModeEngine()
			engine.current = tt.mode

			gotWrites := engine.AllowWrites()
			if gotWrites != tt.wantWrites {
				t.Errorf("AllowWrites() in mode %q = %v, want %v",
					tt.mode, gotWrites, tt.wantWrites)
			}
		})
	}
}

func TestDefaultModeEngine_SetMode(t *testing.T) {
	engine := NewModeEngine()

	tests := []struct {
		name     string
		modeName string
		wantErr  bool
		wantMode Mode
	}{
		{
			name:     "set to chat",
			modeName: "chat",
			wantErr:  false,
			wantMode: ModeChat,
		},
		{
			name:     "set to build",
			modeName: "build",
			wantErr:  false,
			wantMode: ModeBuild,
		},
		{
			name:     "set to plan",
			modeName: "plan",
			wantErr:  false,
			wantMode: ModePlan,
		},
		{
			name:     "invalid mode returns default",
			modeName: "invalid",
			wantErr:  false,
			wantMode: ModeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.SetMode(tt.modeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetMode(%q) error = %v, wantErr %v", tt.modeName, err, tt.wantErr)
				return
			}
			if !tt.wantErr && engine.Current() != tt.wantMode {
				t.Errorf("SetMode(%q) = %v, want %v", tt.modeName, engine.Current(), tt.wantMode)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		wantMode Mode
	}{
		{"code", ModeCode},
		{"blog", ModeBlog},
		{"podcast", ModePodcast},
		{"chat", ModeChat},
		{"plan", ModePlan},
		{"build", ModeBuild},
		{"review", ModeReview},
		{"technical_writer", ModeTechnicalWriter},
		{"unknown", ModeCode},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseMode(tt.input)
			if got != tt.wantMode {
				t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.wantMode)
			}
		})
	}
}

func TestGetModeForIntent(t *testing.T) {
	tests := []struct {
		intent   IntentType
		wantMode Mode
	}{
		{IntentBuild, ModeCode},
		{IntentDebug, ModeCode},
		{IntentReview, ModeCode},
		{IntentTriage, ModeCode},
		{IntentChat, ModeChat},
		{IntentPlan, ModePlan},
		{IntentBlog, ModeBlog},
		{IntentPodcast, ModePodcast},
		{IntentTechnicalWriter, ModeTechnicalWriter},
	}

	for _, tt := range tests {
		t.Run(string(tt.intent), func(t *testing.T) {
			got := GetModeForIntent(tt.intent)
			if got != tt.wantMode {
				t.Errorf("GetModeForIntent(%q) = %v, want %v", tt.intent, got, tt.wantMode)
			}
		})
	}
}
