package repl

import (
	"io"
	"log"
	"os"
)

// SuppressStderr redirects stderr to /dev/null
func SuppressStderr() func() {
	// Save original stderr
	originalStderr := os.Stderr

	// Open /dev/null
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		// If we can't open /dev/null, just return a no-op
		return func() {}
	}

	// Redirect stderr to /dev/null
	os.Stderr = devNull

	// Also redirect Go's log package
	log.SetOutput(devNull)

	// Return cleanup function
	return func() {
		os.Stderr = originalStderr
		log.SetOutput(originalStderr)
		devNull.Close()
	}
}

// EnableStderr restores stderr output
func EnableStderr() {
	// Ensure log output goes to stderr
	log.SetOutput(os.Stderr)
}

// ConditionalStderr suppresses stderr if not in debug mode
func ConditionalStderr(debugMode bool) func() {
	if debugMode {
		// Debug mode: keep stderr enabled
		return func() {}
	}

	// Normal mode: suppress stderr
	return SuppressStderr()
}

// RedirectStderrToWriter redirects stderr to a custom writer
func RedirectStderrToWriter(w io.Writer) func() {
	originalStderr := os.Stderr

	// Create a pipe
	r, w2, err := os.Pipe()
	if err != nil {
		return func() {}
	}

	// Redirect stderr to the write end of the pipe
	os.Stderr = w2

	// Copy from pipe to custom writer
	go io.Copy(w, r)

	// Return cleanup function
	return func() {
		w2.Close()
		r.Close()
		os.Stderr = originalStderr
	}
}
