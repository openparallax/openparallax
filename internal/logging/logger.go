// Package logging provides structured logging for the OpenParallax pipeline.
// Every significant operation is logged with structured key-value pairs.
// This is operational visibility — separate from the audit trail.
package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level is the log severity.
type Level int

const (
	// LevelDebug includes all events.
	LevelDebug Level = iota
	// LevelInfo includes informational events and above.
	LevelInfo
	// LevelWarn includes warnings and errors.
	LevelWarn
	// LevelError includes only errors.
	LevelError
)

// Logger provides structured logging with leveled output.
type Logger struct {
	writer io.Writer
	level  Level
	mu     sync.Mutex
}

// New creates a Logger writing to the given file path at the specified level.
func New(path string, level Level) (*Logger, error) {
	dir := path[:lastSlash(path)]
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{writer: f, level: level}, nil
}

// NewFromWriter creates a Logger writing to the given writer.
func NewFromWriter(w io.Writer, level Level) *Logger {
	return &Logger{writer: w, level: level}
}

// Nop returns a logger that discards all output.
func Nop() *Logger {
	return &Logger{writer: io.Discard, level: LevelError + 1}
}

// Debug logs a debug message with structured key-value pairs.
func (l *Logger) Debug(event string, kvs ...any) {
	if l.level > LevelDebug {
		return
	}
	l.log("DEBUG", event, kvs...)
}

// Info logs an informational message.
func (l *Logger) Info(event string, kvs ...any) {
	if l.level > LevelInfo {
		return
	}
	l.log("INFO", event, kvs...)
}

// Warn logs a warning.
func (l *Logger) Warn(event string, kvs ...any) {
	if l.level > LevelWarn {
		return
	}
	l.log("WARN", event, kvs...)
}

// Error logs an error.
func (l *Logger) Error(event string, kvs ...any) {
	l.log("ERROR", event, kvs...)
}

func (l *Logger) log(level, event string, kvs ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := time.Now().Format("2006-01-02T15:04:05.000")
	msg := fmt.Sprintf("%s [%s] %s", ts, level, event)

	for i := 0; i+1 < len(kvs); i += 2 {
		msg += fmt.Sprintf(" %s=%v", kvs[i], kvs[i+1])
	}

	_, _ = fmt.Fprintln(l.writer, msg)
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return i
		}
	}
	return -1
}
