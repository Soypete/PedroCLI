package tui

import (
	"strings"
)

// OutputLine represents a styled line in the scrolling output area.
type OutputLine struct {
	Text  string
	Style string // info, success, error, warning, tool, llm
}

// OutputPanel manages a scrolling buffer of styled output lines.
type OutputPanel struct {
	lines     []OutputLine
	maxLines  int
	width     int
	scrollPos int // 0 = bottom (most recent), positive = scrolled up
}

// NewOutputPanel creates a new output panel.
func NewOutputPanel(maxLines int) *OutputPanel {
	return &OutputPanel{
		lines:    make([]OutputLine, 0, maxLines),
		maxLines: maxLines,
	}
}

// SetWidth sets the panel width.
func (o *OutputPanel) SetWidth(w int) {
	o.width = w
}

// Append adds a line to the output buffer.
func (o *OutputPanel) Append(text, style string) {
	// Split multi-line text
	for _, line := range strings.Split(text, "\n") {
		o.lines = append(o.lines, OutputLine{Text: line, Style: style})
	}

	// Trim to max
	if len(o.lines) > o.maxLines {
		o.lines = o.lines[len(o.lines)-o.maxLines:]
	}

	// Auto-scroll to bottom on new content
	o.scrollPos = 0
}

// ScrollUp scrolls the output up by n lines.
func (o *OutputPanel) ScrollUp(n int) {
	o.scrollPos += n
	max := len(o.lines)
	if o.scrollPos > max {
		o.scrollPos = max
	}
}

// ScrollDown scrolls the output down by n lines.
func (o *OutputPanel) ScrollDown(n int) {
	o.scrollPos -= n
	if o.scrollPos < 0 {
		o.scrollPos = 0
	}
}

// Clear empties the output buffer.
func (o *OutputPanel) Clear() {
	o.lines = o.lines[:0]
	o.scrollPos = 0
}

// View renders the visible portion of the output, fitting into viewHeight lines.
func (o *OutputPanel) View(viewHeight int) string {
	if viewHeight <= 0 || len(o.lines) == 0 {
		return ""
	}

	// Calculate visible window
	total := len(o.lines)
	end := total - o.scrollPos
	if end < 0 {
		end = 0
	}
	start := end - viewHeight
	if start < 0 {
		start = 0
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		line := o.lines[i]
		styled := o.styleLine(line)
		b.WriteString(styled)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// styleLine applies the appropriate style to an output line.
func (o *OutputPanel) styleLine(line OutputLine) string {
	text := line.Text

	switch line.Style {
	case "success":
		return SuccessStyle.Render(text)
	case "error":
		return ErrorStyle.Render(text)
	case "warning":
		return WarningStyle.Render(text)
	case "tool":
		return ToolStyle.Render(text)
	case "llm":
		return LLMStyle.Render(text)
	default:
		return text
	}
}

// LineCount returns the total number of lines in the buffer.
func (o *OutputPanel) LineCount() int {
	return len(o.lines)
}
