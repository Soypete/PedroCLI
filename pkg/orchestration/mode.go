package orchestration

import "context"

type Mode string

const (
	ModeCode    Mode = "code"
	ModeBlog    Mode = "blog"
	ModePodcast Mode = "podcast"
	ModeChat    Mode = "chat"
	ModePlan    Mode = "plan"
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
	default:
		return ModeCode
	}
}

type ModeEngine interface {
	Execute(ctx context.Context, input string, mode Mode) (*QueryResult, error)
	GetDefaultMode() Mode
	GetModeForIntent(intent IntentType) Mode
}

type intentToModeMap map[IntentType]Mode

var defaultIntentToMode = intentToModeMap{
	IntentBuild:   ModeCode,
	IntentDebug:   ModeCode,
	IntentReview:  ModeCode,
	IntentTriage:  ModeCode,
	IntentChat:    ModeChat,
	IntentPlan:    ModePlan,
	IntentBlog:    ModeBlog,
	IntentPodcast: ModePodcast,
}

func GetModeForIntent(intent IntentType) Mode {
	if mode, ok := defaultIntentToMode[intent]; ok {
		return mode
	}
	return ModeCode
}

func GetSupportedModes() []Mode {
	return []Mode{ModeCode, ModeBlog, ModePodcast, ModeChat, ModePlan}
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
	}
}
