package repl

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Spinner displays an animated loading indicator
type Spinner struct {
	mu      sync.Mutex
	running bool
	stop    chan struct{}
	writer  io.Writer
	message string
}

// NewSpinner creates a new spinner
func NewSpinner(writer io.Writer, message string) *Spinner {
	return &Spinner{
		writer:  writer,
		message: message,
		stop:    make(chan struct{}),
	}
}

// Dancing Pedro frames - simple ASCII animation
// TODO: Reserved for future Pedro animation feature
//
//nolint:unused
var pedroFrames = []string{
	// Frame 1: Arms up
	`
    🤖
   \|/
    |
   / \
`,
	// Frame 2: Arms down
	`
    🤖
    |
    |
   / \
`,
	// Frame 3: Arms to side
	`
    🤖
   -|-
    |
   / \
`,
	// Frame 4: Arms down
	`
    🤖
    |
    |
   / \
`,
}

// Simple spinner frames (fallback)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.animate()
}

// Stop stops the spinner animation
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stop)
	s.running = false

	// Clear the spinner line
	fmt.Fprint(s.writer, "\r\033[K")
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}

// animate runs the animation loop
func (s *Spinner) animate() {
	frameIndex := 0
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.message
			s.mu.Unlock()

			// Use simple spinner for now (Pedro animation takes too much space)
			frame := spinnerFrames[frameIndex%len(spinnerFrames)]
			fmt.Fprintf(s.writer, "\r%s %s", frame, msg)

			frameIndex++
		}
	}
}

// ShowPedro displays a single frame of dancing Pedro
func ShowPedro(writer io.Writer) {
	pedro := `
   🤖 Pedro is working...
   \|/
    |
   / \
`
	fmt.Fprint(writer, pedro)
}

// ShowCompletePedro displays Pedro's completion animation
func ShowCompletePedro(writer io.Writer) {
	pedro := `
   🎉 Done!
    🤖
   \|/
    |
   / \
`
	fmt.Fprint(writer, pedro)
}

// ProgressSpinner is a spinner that shows progress events
type ProgressSpinner struct {
	spinner       *Spinner
	writer        io.Writer
	mu            sync.Mutex
	events        []string
	maxEvents     int
	currentAction string
}

// NewProgressSpinner creates a new progress spinner
func NewProgressSpinner(writer io.Writer) *ProgressSpinner {
	return &ProgressSpinner{
		writer:    writer,
		maxEvents: 5, // Show last 5 events
		events:    make([]string, 0, 5),
	}
}

// Start starts the progress spinner
func (ps *ProgressSpinner) Start(message string) {
	ps.mu.Lock()
	ps.currentAction = message
	ps.mu.Unlock()

	ps.spinner = NewSpinner(ps.writer, message)
	ps.spinner.Start()
}

// Stop stops the progress spinner
func (ps *ProgressSpinner) Stop() {
	if ps.spinner != nil {
		ps.spinner.Stop()
		fmt.Fprintln(ps.writer) // Newline after spinner
	}
}

// AddEvent adds a progress event
func (ps *ProgressSpinner) AddEvent(event string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Add to events list
	ps.events = append(ps.events, event)
	if len(ps.events) > ps.maxEvents {
		ps.events = ps.events[1:]
	}

	// Update spinner message with latest event
	if ps.spinner != nil {
		ps.spinner.UpdateMessage(fmt.Sprintf("%s - %s", ps.currentAction, event))
	}
}

// UpdateAction updates the current action being performed
func (ps *ProgressSpinner) UpdateAction(action string) {
	ps.mu.Lock()
	ps.currentAction = action
	ps.mu.Unlock()

	if ps.spinner != nil {
		ps.spinner.UpdateMessage(action)
	}
}

// ShowProgress displays a more detailed progress view
func (ps *ProgressSpinner) ShowProgress() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.spinner != nil {
		ps.spinner.Stop()
	}

	// Clear screen and show progress
	fmt.Fprintf(ps.writer, "\033[2J\033[H") // Clear screen, move cursor to top

	// Show Pedro
	fmt.Fprintf(ps.writer, "   🤖 Pedro is working...\n")
	fmt.Fprintf(ps.writer, "   %-30s\n\n", ps.currentAction)

	// Show recent events
	fmt.Fprintf(ps.writer, "Recent activity:\n")
	for i, event := range ps.events {
		prefix := "  "
		if i == len(ps.events)-1 {
			prefix = "▶ "
		}
		fmt.Fprintf(ps.writer, "%s%s\n", prefix, event)
	}

	// Restart spinner
	if ps.spinner != nil {
		ps.spinner = NewSpinner(ps.writer, ps.currentAction)
		ps.spinner.Start()
	}
}

// CompactProgressDisplay shows progress without clearing screen
type CompactProgressDisplay struct {
	writer       io.Writer
	mu           sync.Mutex
	lastEvent    string
	toolCount    int
	llmCallCount int
	currentRound int
	startTime    time.Time
}

// NewCompactProgressDisplay creates a compact progress display
func NewCompactProgressDisplay(writer io.Writer) *CompactProgressDisplay {
	return &CompactProgressDisplay{
		writer:    writer,
		startTime: time.Now(),
	}
}

// OnRoundStart is called when a new inference round starts
func (cpd *CompactProgressDisplay) OnRoundStart(round int) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	cpd.currentRound = round
	elapsed := time.Since(cpd.startTime).Round(time.Second)

	fmt.Fprintf(cpd.writer, "\r\033[K") // Clear line
	fmt.Fprintf(cpd.writer, "\n🔄 Round %d (elapsed: %v)\n", round, elapsed)
}

// OnToolCall is called when a tool is called
func (cpd *CompactProgressDisplay) OnToolCall(toolName string, args string) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	cpd.toolCount++

	// Truncate args if too long
	if len(args) > 50 {
		args = args[:47] + "..."
	}

	fmt.Fprintf(cpd.writer, "\r\033[K") // Clear line
	fmt.Fprintf(cpd.writer, "  🔧 %s(%s)\n", toolName, args)
}

// OnToolResult is called when a tool returns a result
func (cpd *CompactProgressDisplay) OnToolResult(success bool, output string) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	status := "✅"
	if !success {
		status = "❌"
	}

	// Truncate output
	if len(output) > 60 {
		output = output[:57] + "..."
	}

	fmt.Fprintf(cpd.writer, "     %s %s\n", status, output)
}

// OnLLMResponse is called when the LLM responds
func (cpd *CompactProgressDisplay) OnLLMResponse(message string) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	cpd.llmCallCount++

	// Just show that we got a response, not the full content
	fmt.Fprintf(cpd.writer, "  💭 LLM response received\n")
}

// OnComplete is called when execution completes
func (cpd *CompactProgressDisplay) OnComplete(success bool, message string) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	elapsed := time.Since(cpd.startTime).Round(time.Second)

	fmt.Fprintf(cpd.writer, "\r\033[K") // Clear line
	if success {
		fmt.Fprintf(cpd.writer, "\n✅ Complete! (%v, %d tools, %d LLM calls)\n", elapsed, cpd.toolCount, cpd.llmCallCount)
		ShowCompletePedro(cpd.writer)
	} else {
		fmt.Fprintf(cpd.writer, "\n❌ Failed: %s (%v)\n", message, elapsed)
	}
}

// OnMessage is called for general messages
func (cpd *CompactProgressDisplay) OnMessage(message string) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	fmt.Fprintf(cpd.writer, "  ℹ️  %s\n", message)
}

// AddStatusLine adds a status line with spinner
func (cpd *CompactProgressDisplay) AddStatusLine(message string) {
	cpd.mu.Lock()
	defer cpd.mu.Unlock()

	cpd.lastEvent = message

	// Simple inline status
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[cpd.toolCount%len(frames)]

	fmt.Fprintf(cpd.writer, "\r%s %s", frame, message)
}

// ClearStatusLine clears the status line
func (cpd *CompactProgressDisplay) ClearStatusLine() {
	fmt.Fprintf(cpd.writer, "\r\033[K")
}
