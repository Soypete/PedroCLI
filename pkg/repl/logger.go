package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Logger handles REPL session logging
type Logger struct {
	sessionDir   string
	sessionFile  *os.File
	agentFile    *os.File
	toolFile     *os.File
	llmFile      *os.File
	debugEnabled bool
	keepLogs     bool
}

// NewLogger creates a new logger for a REPL session
func NewLogger(sessionID string, debugEnabled bool, keepLogs bool) (*Logger, error) {
	// Create session directory in /tmp
	sessionDir := filepath.Join("/tmp", "pedrocode-sessions", sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create log files
	sessionFile, err := os.Create(filepath.Join(sessionDir, "session.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create session log: %w", err)
	}

	agentFile, err := os.Create(filepath.Join(sessionDir, "agent-calls.log"))
	if err != nil {
		sessionFile.Close()
		return nil, fmt.Errorf("failed to create agent log: %w", err)
	}

	toolFile, err := os.Create(filepath.Join(sessionDir, "tool-calls.log"))
	if err != nil {
		sessionFile.Close()
		agentFile.Close()
		return nil, fmt.Errorf("failed to create tool log: %w", err)
	}

	var llmFile *os.File
	if debugEnabled {
		llmFile, err = os.Create(filepath.Join(sessionDir, "llm-requests.log"))
		if err != nil {
			sessionFile.Close()
			agentFile.Close()
			toolFile.Close()
			return nil, fmt.Errorf("failed to create LLM log: %w", err)
		}
	}

	logger := &Logger{
		sessionDir:   sessionDir,
		sessionFile:  sessionFile,
		agentFile:    agentFile,
		toolFile:     toolFile,
		llmFile:      llmFile,
		debugEnabled: debugEnabled,
		keepLogs:     keepLogs,
	}

	// Write header
	logger.logSession("Session started: %s\n", sessionID)
	logger.logSession("Debug mode: %v\n", debugEnabled)
	logger.logSession("Keep logs: %v\n", keepLogs)
	logger.logSession("Log directory: %s\n\n", sessionDir)

	return logger, nil
}

// GetSessionDir returns the session log directory path
func (l *Logger) GetSessionDir() string {
	return l.sessionDir
}

// LogSession logs to the session transcript
func (l *Logger) LogSession(format string, args ...interface{}) {
	l.logWithTimestamp(l.sessionFile, format, args...)
}

// LogAgent logs agent execution details
func (l *Logger) LogAgent(format string, args ...interface{}) {
	l.logWithTimestamp(l.agentFile, format, args...)
}

// LogTool logs tool execution details
func (l *Logger) LogTool(format string, args ...interface{}) {
	l.logWithTimestamp(l.toolFile, format, args...)
}

// LogLLM logs LLM API calls (only if debug enabled)
func (l *Logger) LogLLM(format string, args ...interface{}) {
	if l.llmFile != nil {
		l.logWithTimestamp(l.llmFile, format, args...)
	}
}

// LogInput logs user input
func (l *Logger) LogInput(input string) {
	l.logSession(">>> %s\n", input)
}

// LogOutput logs system output
func (l *Logger) LogOutput(output string) {
	l.logSession("<<< %s\n", output)
}

// LogError logs an error
func (l *Logger) LogError(err error) {
	l.logSession("ERROR: %v\n", err)
}

// logWithTimestamp writes a timestamped log entry
func (l *Logger) logWithTimestamp(w io.Writer, format string, args ...interface{}) {
	if w == nil {
		return
	}

	timestamp := time.Now().Format("15:04:05.000")
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(w, "[%s] %s", timestamp, message)
}

// logSession logs without timestamp (internal helper)
func (l *Logger) logSession(format string, args ...interface{}) {
	if l.sessionFile != nil {
		fmt.Fprintf(l.sessionFile, format, args...)
	}
}

// Flush flushes all log files
func (l *Logger) Flush() error {
	var errs []error

	if l.sessionFile != nil {
		if err := l.sessionFile.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	if l.agentFile != nil {
		if err := l.agentFile.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	if l.toolFile != nil {
		if err := l.toolFile.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	if l.llmFile != nil {
		if err := l.llmFile.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to flush logs: %v", errs)
	}

	return nil
}

// Close closes all log files and optionally cleans up
func (l *Logger) Close() error {
	// Close all files
	if l.sessionFile != nil {
		l.sessionFile.Close()
	}
	if l.agentFile != nil {
		l.agentFile.Close()
	}
	if l.toolFile != nil {
		l.toolFile.Close()
	}
	if l.llmFile != nil {
		l.llmFile.Close()
	}

	// Clean up if not keeping logs
	if !l.keepLogs {
		if err := os.RemoveAll(l.sessionDir); err != nil {
			return fmt.Errorf("failed to clean up logs: %w", err)
		}
	}

	return nil
}

// CleanupOldSessions removes old session directories (older than 24 hours)
func CleanupOldSessions() error {
	sessionsDir := filepath.Join("/tmp", "pedrocode-sessions")

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No sessions directory yet
		}
		return err
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Remove if older than 24 hours
		if info.ModTime().Before(cutoff) {
			dirPath := filepath.Join(sessionsDir, entry.Name())
			os.RemoveAll(dirPath)
		}
	}

	return nil
}
