package tui

import (
	"strings"
	"testing"
)

func TestOutputPanel_Empty(t *testing.T) {
	o := NewOutputPanel(100)
	view := o.View(10)
	if view != "" {
		t.Errorf("expected empty view, got: %q", view)
	}
}

func TestOutputPanel_AppendAndView(t *testing.T) {
	o := NewOutputPanel(100)
	o.Append("hello world", "info")

	view := o.View(10)
	if !strings.Contains(view, "hello world") {
		t.Errorf("expected 'hello world' in view, got: %q", view)
	}
}

func TestOutputPanel_MultiLineAppend(t *testing.T) {
	o := NewOutputPanel(100)
	o.Append("line1\nline2\nline3", "info")

	if o.LineCount() != 3 {
		t.Errorf("expected 3 lines, got %d", o.LineCount())
	}
}

func TestOutputPanel_MaxLines(t *testing.T) {
	o := NewOutputPanel(5)
	for i := 0; i < 10; i++ {
		o.Append("line", "info")
	}

	if o.LineCount() != 5 {
		t.Errorf("expected max 5 lines, got %d", o.LineCount())
	}
}

func TestOutputPanel_ScrollUp(t *testing.T) {
	o := NewOutputPanel(100)
	for i := 0; i < 20; i++ {
		o.Append("line", "info")
	}

	// Auto-scroll should be at bottom (scrollPos=0)
	if o.scrollPos != 0 {
		t.Errorf("expected scrollPos 0 after appending, got %d", o.scrollPos)
	}

	o.ScrollUp(5)
	if o.scrollPos != 5 {
		t.Errorf("expected scrollPos 5, got %d", o.scrollPos)
	}
}

func TestOutputPanel_ScrollDown(t *testing.T) {
	o := NewOutputPanel(100)
	for i := 0; i < 20; i++ {
		o.Append("line", "info")
	}

	o.ScrollUp(10)
	o.ScrollDown(3)
	if o.scrollPos != 7 {
		t.Errorf("expected scrollPos 7, got %d", o.scrollPos)
	}
}

func TestOutputPanel_ScrollDown_Floor(t *testing.T) {
	o := NewOutputPanel(100)
	o.Append("line", "info")

	o.ScrollDown(100)
	if o.scrollPos != 0 {
		t.Errorf("expected scrollPos floor at 0, got %d", o.scrollPos)
	}
}

func TestOutputPanel_Clear(t *testing.T) {
	o := NewOutputPanel(100)
	o.Append("test", "info")
	o.Clear()

	if o.LineCount() != 0 {
		t.Errorf("expected 0 lines after clear, got %d", o.LineCount())
	}
}

func TestOutputPanel_ViewHeight(t *testing.T) {
	o := NewOutputPanel(100)
	for i := 0; i < 20; i++ {
		o.Append("line", "info")
	}

	// View with height 5 should only show 5 lines
	view := o.View(5)
	lines := strings.Split(view, "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 visible lines, got %d", len(lines))
	}
}

func TestOutputPanel_ZeroHeight(t *testing.T) {
	o := NewOutputPanel(100)
	o.Append("test", "info")

	view := o.View(0)
	if view != "" {
		t.Errorf("expected empty view for zero height, got: %q", view)
	}
}

func TestOutputPanel_AutoScrollOnAppend(t *testing.T) {
	o := NewOutputPanel(100)
	o.Append("line1", "info")
	o.ScrollUp(5)

	// After appending, should auto-scroll back to bottom
	o.Append("line2", "info")
	if o.scrollPos != 0 {
		t.Errorf("expected auto-scroll to 0 on append, got %d", o.scrollPos)
	}
}
