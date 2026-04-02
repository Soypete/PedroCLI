package phases

type PhaseDefinition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
	MaxRounds   int      `json:"max_rounds"`
	ExpectsJSON bool     `json:"expects_json"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

type Registry interface {
	GetPhase(name string) (*PhaseDefinition, error)
	GetPhases(names []string) ([]PhaseDefinition, error)
	ListPhases() []string
	RegisterPhase(def PhaseDefinition)
}

type registry struct {
	phases map[string]PhaseDefinition
}

func NewRegistry() Registry {
	r := &registry{
		phases: make(map[string]PhaseDefinition),
	}
	for _, def := range getStandardPhases() {
		r.phases[def.Name] = def
	}
	return r
}

var defaultRegistry Registry

func DefaultRegistry() Registry {
	if defaultRegistry == nil {
		defaultRegistry = NewRegistry()
	}
	return defaultRegistry
}

func SetDefaultRegistry(reg Registry) {
	defaultRegistry = reg
}

func (r *registry) GetPhase(name string) (*PhaseDefinition, error) {
	phase, ok := r.phases[name]
	if !ok {
		return nil, nil
	}
	return &phase, nil
}

func (r *registry) GetPhases(names []string) ([]PhaseDefinition, error) {
	result := make([]PhaseDefinition, 0, len(names))
	for _, name := range names {
		phase, ok := r.phases[name]
		if !ok {
			return nil, nil
		}
		result = append(result, phase)
	}
	return result, nil
}

func (r *registry) ListPhases() []string {
	names := make([]string, 0, len(r.phases))
	for name := range r.phases {
		names = append(names, name)
	}
	return names
}

func (r *registry) RegisterPhase(def PhaseDefinition) {
	r.phases[def.Name] = def
}

func getStandardPhases() []PhaseDefinition {
	return []PhaseDefinition{
		{
			Name:        "analyze",
			Description: "Analyze the request, evaluate repo state, gather requirements",
			Tools:       []string{"search", "navigate", "file", "git", "lsp"},
			MaxRounds:   10,
			ExpectsJSON: true,
		},
		{
			Name:        "plan",
			Description: "Create a detailed implementation plan with numbered steps",
			Tools:       []string{"search", "navigate", "file", "context"},
			MaxRounds:   5,
			ExpectsJSON: true,
			DependsOn:   []string{"analyze"},
		},
		{
			Name:        "implement",
			Description: "Write code following the plan, chunk by chunk",
			Tools:       []string{"file", "code_edit", "search", "git", "bash", "lsp", "context"},
			MaxRounds:   30,
			ExpectsJSON: false,
			DependsOn:   []string{"plan"},
		},
		{
			Name:        "validate",
			Description: "Run tests, linter, verify the implementation works",
			Tools:       []string{"test", "bash", "file", "code_edit", "lsp", "search", "navigate"},
			MaxRounds:   15,
			ExpectsJSON: false,
			DependsOn:   []string{"implement"},
		},
		{
			Name:        "deliver",
			Description: "Commit changes and create draft PR",
			Tools:       []string{"git", "github"},
			MaxRounds:   5,
			ExpectsJSON: false,
			DependsOn:   []string{"validate"},
		},
		{
			Name:        "review",
			Description: "Code review analysis",
			Tools:       []string{"search", "navigate", "file", "git", "github", "lsp"},
			MaxRounds:   10,
			ExpectsJSON: true,
		},
		{
			Name:        "reproduce",
			Description: "Reproduce the issue consistently",
			Tools:       []string{"test", "bash", "file", "search"},
			MaxRounds:   8,
			ExpectsJSON: true,
		},
		{
			Name:        "investigate",
			Description: "Gather evidence about the root cause",
			Tools:       []string{"search", "file", "lsp", "git", "navigate", "context"},
			MaxRounds:   12,
			ExpectsJSON: false,
			DependsOn:   []string{"reproduce"},
		},
		{
			Name:        "isolate",
			Description: "Narrow down to the exact root cause",
			Tools:       []string{"file", "lsp", "search", "bash", "context"},
			MaxRounds:   10,
			ExpectsJSON: true,
			DependsOn:   []string{"investigate"},
		},
		{
			Name:        "fix",
			Description: "Implement a targeted fix",
			Tools:       []string{"file", "code_edit", "search", "lsp"},
			MaxRounds:   10,
			ExpectsJSON: false,
			DependsOn:   []string{"isolate"},
		},
		{
			Name:        "verify",
			Description: "Verify the fix works and doesn't break anything",
			Tools:       []string{"test", "bash", "lsp", "file", "code_edit"},
			MaxRounds:   15,
			ExpectsJSON: false,
			DependsOn:   []string{"fix"},
		},
		{
			Name:        "commit",
			Description: "Commit the fix with a clear message",
			Tools:       []string{"git"},
			MaxRounds:   3,
			ExpectsJSON: false,
			DependsOn:   []string{"verify"},
		},
		{
			Name:        "gather",
			Description: "Fetch PR details, checkout branch, get diff and LSP diagnostics",
			Tools:       []string{"github", "git", "lsp", "search", "navigate", "file"},
			MaxRounds:   10,
			ExpectsJSON: false,
		},
		{
			Name:        "security",
			Description: "Analyze for security vulnerabilities and issues",
			Tools:       []string{"search", "file", "lsp", "context"},
			MaxRounds:   8,
			ExpectsJSON: true,
			DependsOn:   []string{"gather"},
		},
		{
			Name:        "quality",
			Description: "Review code quality, performance, and maintainability",
			Tools:       []string{"search", "file", "lsp", "navigate", "context"},
			MaxRounds:   10,
			ExpectsJSON: true,
			DependsOn:   []string{"gather"},
		},
		{
			Name:        "compile",
			Description: "Compile all findings into structured review",
			Tools:       []string{"context"},
			MaxRounds:   5,
			ExpectsJSON: true,
			DependsOn:   []string{"security", "quality"},
		},
		{
			Name:        "publish",
			Description: "Post review to GitHub (optional)",
			Tools:       []string{"github"},
			MaxRounds:   3,
			ExpectsJSON: false,
			DependsOn:   []string{"compile"},
		},
	}
}
