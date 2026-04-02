package orchestration

import (
	"context"
	"fmt"

	"github.com/soypete/pedrocli/pkg/config"
)

type Mode string

const (
	ModeCode            Mode = "code"
	ModeBlog            Mode = "blog"
	ModePodcast         Mode = "podcast"
	ModeChat            Mode = "chat"
	ModePlan            Mode = "plan"
	ModeBuild           Mode = "build"
	ModeReview          Mode = "review"
	ModeTechnicalWriter Mode = "technical_writer"
)

func (m Mode) String() string {
	return string(m)
}

func ParseMode(s string) Mode {
	switch s {
	case "code":
		return ModeCode
	case "blog":
		return ModeBlog
	case "podcast":
		return ModePodcast
	case "chat":
		return ModeChat
	case "plan":
		return ModePlan
	case "build":
		return ModeBuild
	case "review":
		return ModeReview
	case "technical_writer":
		return ModeTechnicalWriter
	default:
		return ModeCode
	}
}

type ModeEngine interface {
	Execute(ctx context.Context, input string, mode Mode) (*QueryResult, error)
	GetDefaultMode() Mode
	GetModeForIntent(intent IntentType) Mode
}

type DefaultModeEngine struct {
	modes   map[Mode]config.ModeConfig
	current Mode
}

func NewModeEngine() *DefaultModeEngine {
	modes := make(map[Mode]config.ModeConfig)
	for mode := range defaultModeConstraints {
		modes[mode] = GetModeConfig(mode)
	}
	return &DefaultModeEngine{
		modes:   modes,
		current: ModeCode,
	}
}

func (m *DefaultModeEngine) SetMode(name string) error {
	mode := ParseMode(name)
	if _, ok := m.modes[mode]; !ok {
		return fmt.Errorf("unknown mode: %s", name)
	}
	m.current = mode
	return nil
}

func (m *DefaultModeEngine) Current() Mode {
	return m.current
}

func (m *DefaultModeEngine) IsToolAllowed(toolName string) bool {
	cfg, ok := m.modes[m.current]
	if !ok {
		return true
	}

	for _, denied := range cfg.DeniedTools {
		if denied == toolName {
			return false
		}
	}

	if len(cfg.AllowedTools) > 0 {
		for _, allowed := range cfg.AllowedTools {
			if allowed == toolName {
				return true
			}
		}
		return false
	}

	return true
}

func (m *DefaultModeEngine) AllowWrites() bool {
	cfg, ok := m.modes[m.current]
	if !ok {
		return true
	}
	return cfg.AllowWrites
}

func (m *DefaultModeEngine) Execute(ctx context.Context, input string, mode Mode) (*QueryResult, error) {
	oldMode := m.current
	m.current = mode
	defer func() { m.current = oldMode }()

	return nil, fmt.Errorf("not implemented: use QueryEngine.ExecuteWithMode instead")
}

func (m *DefaultModeEngine) GetDefaultMode() Mode {
	return ModeCode
}

func (m *DefaultModeEngine) GetModeForIntent(intent IntentType) Mode {
	return GetModeForIntent(intent)
}

type intentToModeMap map[IntentType]Mode

var defaultIntentToMode = intentToModeMap{
	IntentBuild:           ModeCode,
	IntentDebug:           ModeCode,
	IntentReview:          ModeCode,
	IntentTriage:          ModeCode,
	IntentChat:            ModeChat,
	IntentPlan:            ModePlan,
	IntentBlog:            ModeBlog,
	IntentPodcast:         ModePodcast,
	IntentTechnicalWriter: ModeTechnicalWriter,
}

func GetModeForIntent(intent IntentType) Mode {
	if mode, ok := defaultIntentToMode[intent]; ok {
		return mode
	}
	return ModeCode
}

func GetSupportedModes() []Mode {
	return []Mode{ModeCode, ModeBlog, ModePodcast, ModeChat, ModePlan, ModeBuild, ModeReview, ModeTechnicalWriter}
}

func GetSupportedIntents() []IntentType {
	return []IntentType{
		IntentChat,
		IntentPlan,
		IntentBuild,
		IntentDebug,
		IntentReview,
		IntentTriage,
		IntentBlog,
		IntentPodcast,
		IntentTechnicalWriter,
	}
}
