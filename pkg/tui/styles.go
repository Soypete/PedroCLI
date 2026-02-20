package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the TUI.
var Theme = struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Muted     lipgloss.Color
	Accent    lipgloss.Color
}{
	Primary:   lipgloss.Color("#7C3AED"), // Purple
	Secondary: lipgloss.Color("#06B6D4"), // Cyan
	Success:   lipgloss.Color("#10B981"), // Green
	Error:     lipgloss.Color("#EF4444"), // Red
	Warning:   lipgloss.Color("#F59E0B"), // Amber
	Muted:     lipgloss.Color("#6B7280"), // Gray
	Accent:    lipgloss.Color("#8B5CF6"), // Light purple
}

// Shared styles used across components.
var (
	// HeaderStyle styles the top bar.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Theme.Primary).
			PaddingLeft(1)

	// PromptStyle styles the input prompt.
	PromptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Theme.Accent)

	// DimStyle styles muted/secondary text.
	DimStyle = lipgloss.NewStyle().
			Foreground(Theme.Muted)

	// SuccessStyle styles success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Theme.Success)

	// ErrorStyle styles error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Theme.Error)

	// WarningStyle styles warning messages.
	WarningStyle = lipgloss.NewStyle().
			Foreground(Theme.Warning)

	// ToolStyle styles tool call information.
	ToolStyle = lipgloss.NewStyle().
			Foreground(Theme.Secondary)

	// LLMStyle styles LLM response text.
	LLMStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	// BorderStyle creates a bordered box.
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Theme.Muted)

	// SpinnerFrames for animated progress indicators.
	SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	// PedroFrames is a dancing Pedro ASCII animation rendered beside
	// the progress tree while the agent is working. Each frame is
	// exactly 5 lines tall and 11 characters wide so the layout
	// stays stable across frames (no jitter).
	PedroFrames = []string{
		// Frame 0: neutral
		"   ┌───┐  \n" +
			"   │ ◕◕│  \n" +
			"  ─┤   ├─ \n" +
			"   │ ▽ │  \n" +
			"   └┬─┬┘  ",
		// Frame 1: arms up, lean right
		"   ┌───┐  \n" +
			"  \\│ ◕◕│  \n" +
			"   ┤   ├\\ \n" +
			"   │ ▽ │  \n" +
			"   └┬─┬┘  ",
		// Frame 2: arms down, lean left
		"   ┌───┐  \n" +
			"   │◕◕ │  \n" +
			"  /┤   ├─ \n" +
			"   │ ▽ │  \n" +
			"   └┬─┬┘  ",
		// Frame 3: hands up!
		"  \\┌───┐/ \n" +
			"   │ ◕◕│  \n" +
			"   ┤   ├  \n" +
			"   │ ▽ │  \n" +
			"   └┬─┬┘  ",
		// Frame 4: shimmy right
		"   ┌───┐  \n" +
			"   │◕ ◕│  \n" +
			"   ┤   ├─ \n" +
			"   │ ◡ │  \n" +
			"    └┬─┬┘ ",
		// Frame 5: shimmy left
		"   ┌───┐  \n" +
			"   │◕ ◕│  \n" +
			"  ─┤   ├  \n" +
			"   │ ◡ │  \n" +
			"  └┬─┬┘   ",
		// Frame 6: big wave
		"   ┌───┐/ \n" +
			"   │ ◕◕│  \n" +
			"  ─┤   ├  \n" +
			"   │ ▽ │  \n" +
			"   └┬─┬┘  ",
		// Frame 7: dip
		"           \n" +
			"   ┌───┐  \n" +
			"  ─┤◕ ◕├─ \n" +
			"   │ ◡ │  \n" +
			"   └┬─┬┘  ",
	}

	// PedroIdleFrames is a slower idle animation when nothing is running.
	PedroIdleFrames = []string{
		"   ┌───┐  \n" +
			"   │ ◕◕│  \n" +
			"   ┤   ├  \n" +
			"   │ ─ │  \n" +
			"   └┬─┬┘  ",
		"   ┌───┐  \n" +
			"   │◕◕ │  \n" +
			"   ┤   ├  \n" +
			"   │ ─ │  \n" +
			"   └┬─┬┘  ",
	}

	// PedroDoneFrame is shown when the agent finishes successfully.
	PedroDoneFrame = "" +
		"  \\┌───┐/ \n" +
		"   │ ◕◕│  \n" +
		"   ┤   ├  \n" +
		"   │ ◡ │  \n" +
		"   └┬─┬┘  "

	// PedroStyle colors the Pedro animation.
	PedroStyle = lipgloss.NewStyle().
			Foreground(Theme.Accent)
)
