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
)
