package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

// InputHandler manages user input with readline support
type InputHandler struct {
	rl      *readline.Instance
	session *Session
}

// NewInputHandler creates a new input handler
func NewInputHandler(session *Session) (*InputHandler, error) {
	// Get history file path
	historyFile := getHistoryFilePath()

	// Configure readline
	config := &readline.Config{
		Prompt:                 getPrompt(session),
		HistoryFile:            historyFile,
		HistoryLimit:           1000,
		DisableAutoSaveHistory: false,
		InterruptPrompt:        "^C",
		EOFPrompt:              "exit",
	}

	// Create readline instance
	rl, err := readline.NewEx(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create readline: %w", err)
	}

	return &InputHandler{
		rl:      rl,
		session: session,
	}, nil
}

// ReadLine reads a single line of input
func (h *InputHandler) ReadLine() (string, error) {
	return h.rl.Readline()
}

// ReadMultiLine reads multiple lines until Ctrl+D or empty line
func (h *InputHandler) ReadMultiLine() (string, error) {
	var lines []string

	// Set prompt for multi-line input
	h.rl.SetPrompt("...   ")

	for {
		line, err := h.rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C - cancel multi-line input
				h.UpdatePrompt()
				return "", nil
			}
			if err == io.EOF {
				// Ctrl+D - submit multi-line input
				break
			}
			return "", err
		}

		// Empty line also submits
		if strings.TrimSpace(line) == "" && len(lines) > 0 {
			break
		}

		lines = append(lines, line)
	}

	// Restore normal prompt
	h.UpdatePrompt()

	return strings.Join(lines, "\n"), nil
}

// UpdatePrompt updates the prompt based on current session state
func (h *InputHandler) UpdatePrompt() {
	h.rl.SetPrompt(getPrompt(h.session))
}

// SetPrompt sets a custom prompt temporarily
func (h *InputHandler) SetPrompt(prompt string) {
	h.rl.SetPrompt(prompt)
}

// Close closes the input handler
func (h *InputHandler) Close() error {
	return h.rl.Close()
}

// getPrompt generates the prompt string based on session state
func getPrompt(session *Session) string {
	agent := session.GetCurrentAgent()
	return fmt.Sprintf("pedro:%s> ", agent)
}

// getHistoryFilePath returns the path to the history file
func getHistoryFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/pedrocode_history"
	}

	return filepath.Join(homeDir, ".pedrocode_history")
}

// ClearScreen clears the terminal screen
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}
