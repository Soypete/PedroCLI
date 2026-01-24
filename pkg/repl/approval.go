package repl

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
)

// ApprovalPrompt represents an approval request
type ApprovalPrompt struct {
	Title    string   // e.g., "Apply code changes?"
	Details  string   // Diff or description
	Options  []string // e.g., ["y", "n", "e", "q"]
	Readline *readline.Instance
}

// ApprovalResponse represents the user's response
type ApprovalResponse struct {
	Approved bool
	Action   string // "yes", "no", "edit", "quit", "view"
	Input    string // Full input string
}

// NewApprovalPrompt creates a new approval prompt
func NewApprovalPrompt(title, details string, rl *readline.Instance) *ApprovalPrompt {
	return &ApprovalPrompt{
		Title:    title,
		Details:  details,
		Options:  []string{"y", "n", "e", "q"},
		Readline: rl,
	}
}

// Ask prompts the user for approval
func (a *ApprovalPrompt) Ask() (*ApprovalResponse, error) {
	// Print the title
	fmt.Printf("\n%s\n", a.Title)

	// Print the details (diff, description, etc.)
	if a.Details != "" {
		fmt.Printf("\n%s\n", a.Details)
	}

	// Build options string
	optionsStr := a.formatOptions()

	// Set prompt
	oldPrompt := a.Readline.Config.Prompt
	a.Readline.SetPrompt(fmt.Sprintf("\n%s: ", optionsStr))
	defer a.Readline.SetPrompt(oldPrompt)

	// Read response
	input, err := a.Readline.Readline()
	if err != nil {
		return nil, err
	}

	// Parse response
	return a.parseResponse(strings.TrimSpace(strings.ToLower(input)))
}

// formatOptions formats the options string
func (a *ApprovalPrompt) formatOptions() string {
	descriptions := map[string]string{
		"y": "yes",
		"n": "no",
		"e": "edit",
		"q": "quit",
		"v": "view",
		"d": "diff",
	}

	parts := make([]string, len(a.Options))
	for i, opt := range a.Options {
		if desc, ok := descriptions[opt]; ok {
			parts[i] = fmt.Sprintf("%s=%s", opt, desc)
		} else {
			parts[i] = opt
		}
	}

	return "[" + strings.Join(parts, "/") + "]"
}

// parseResponse parses the user's input
func (a *ApprovalPrompt) parseResponse(input string) (*ApprovalResponse, error) {
	switch input {
	case "y", "yes":
		return &ApprovalResponse{
			Approved: true,
			Action:   "yes",
			Input:    input,
		}, nil

	case "n", "no":
		return &ApprovalResponse{
			Approved: false,
			Action:   "no",
			Input:    input,
		}, nil

	case "e", "edit":
		return &ApprovalResponse{
			Approved: false,
			Action:   "edit",
			Input:    input,
		}, nil

	case "q", "quit":
		return &ApprovalResponse{
			Approved: false,
			Action:   "quit",
			Input:    input,
		}, nil

	case "v", "view", "d", "diff":
		return &ApprovalResponse{
			Approved: false,
			Action:   "view",
			Input:    input,
		}, nil

	default:
		// Treat unknown as "no"
		return &ApprovalResponse{
			Approved: false,
			Action:   "no",
			Input:    input,
		}, nil
	}
}

// FormatDiff formats a diff for display
func FormatDiff(filename string, oldContent, newContent string) string {
	// Simple diff formatter
	// TODO: Use a proper diff library for better output
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("╭─ %s ─────────────────────────────────╮\n", filename))

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines; i++ {
		if i < len(oldLines) && (i >= len(newLines) || oldLines[i] != newLines[i]) {
			sb.WriteString(fmt.Sprintf("│ - %s\n", oldLines[i]))
		}
		if i < len(newLines) && (i >= len(oldLines) || oldLines[i] != newLines[i]) {
			sb.WriteString(fmt.Sprintf("│ + %s\n", newLines[i]))
		}
	}

	sb.WriteString("╰────────────────────────────────────────────╯\n")

	return sb.String()
}

// ProposalSummary represents a code change proposal
type ProposalSummary struct {
	FilesChanged int
	LinesAdded   int
	LinesRemoved int
	Files        []string
}

// FormatProposal formats a proposal summary
func FormatProposal(summary ProposalSummary) string {
	var sb strings.Builder

	sb.WriteString("\n📝 Proposed Changes:\n")
	sb.WriteString(fmt.Sprintf("   %d files changed, +%d -%d lines\n\n",
		summary.FilesChanged, summary.LinesAdded, summary.LinesRemoved))

	for _, file := range summary.Files {
		sb.WriteString(fmt.Sprintf("   • %s\n", file))
	}

	return sb.String()
}
