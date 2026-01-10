package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// DebugLogger handles debug logging for jobs
// In debug mode, logs go to both stderr and a job-specific file
// In async mode, logs only go to the file to avoid blocking the REPL
type DebugLogger struct {
	jobID      string
	logFile    *os.File
	mu         sync.Mutex
	asyncMode  bool
	debugMode  bool
	closedOnce sync.Once
}

// NewDebugLogger creates a new debug logger for a job
func NewDebugLogger(jobID string, asyncMode bool, debugMode bool) (*DebugLogger, error) {
	if !debugMode {
		// Debug logging disabled - return a no-op logger
		return &DebugLogger{
			jobID:     jobID,
			asyncMode: asyncMode,
			debugMode: false,
		}, nil
	}

	// Create log file in /tmp/pedrocode-jobs/<job-id>/debug.log
	jobDir := filepath.Join("/tmp/pedrocode-jobs", jobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create job directory: %w", err)
	}

	logPath := filepath.Join(jobDir, "debug.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create debug log: %w", err)
	}

	logger := &DebugLogger{
		jobID:     jobID,
		logFile:   logFile,
		asyncMode: asyncMode,
		debugMode: true,
	}

	// Write header
	logger.Printf("=== Debug Log for Job %s ===\n", jobID)
	logger.Printf("Async Mode: %v\n", asyncMode)
	logger.Printf("=====================================\n\n")

	return logger, nil
}

// Write implements io.Writer
func (d *DebugLogger) Write(p []byte) (n int, err error) {
	if !d.debugMode {
		return len(p), nil // No-op
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.logFile == nil {
		return 0, fmt.Errorf("debug logger closed")
	}

	// Always write to log file
	n, err = d.logFile.Write(p)
	if err != nil {
		return n, err
	}

	// In sync mode (not async), also write to stderr for real-time feedback
	if !d.asyncMode {
		os.Stderr.Write(p)
	}

	return n, nil
}

// Printf writes formatted debug output
func (d *DebugLogger) Printf(format string, args ...interface{}) {
	if !d.debugMode {
		return
	}

	msg := fmt.Sprintf(format, args...)
	_, _ = d.Write([]byte(msg))
}

// Fprintf writes formatted output to this logger
func (d *DebugLogger) Fprintf(format string, args ...interface{}) {
	d.Printf(format, args...)
}

// GetWriter returns an io.Writer for this logger
func (d *DebugLogger) GetWriter() io.Writer {
	if !d.debugMode {
		return io.Discard
	}
	return d
}

// GetLogPath returns the path to the debug log file
func (d *DebugLogger) GetLogPath() string {
	if !d.debugMode || d.logFile == nil {
		return ""
	}
	return d.logFile.Name()
}

// Close closes the debug log file
func (d *DebugLogger) Close() error {
	if !d.debugMode {
		return nil
	}

	var err error
	d.closedOnce.Do(func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		if d.logFile != nil {
			err = d.logFile.Close()
			d.logFile = nil
		}
	})
	return err
}
