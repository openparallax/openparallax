// Package logging provides structured JSON logging for the OpenParallax pipeline.
// Every significant operation is logged with structured key-value pairs.
// This is operational visibility — separate from the audit trail.
package logging

import (
	"encoding/json"
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

// LogEntry is a single structured log entry written as JSON.
type LogEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Event     string         `json:"event"`
	Data      map[string]any `json:"data,omitempty"`
}

// LogHook is called for every log entry, enabling live broadcasting.
type LogHook func(entry LogEntry)

// Logger provides structured JSON logging with leveled output.
type Logger struct {
	writer io.Writer
	level  Level
	hooks  []LogHook
	mu     sync.Mutex
}

// New creates a Logger writing JSON to the given file path at the specified level.
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

// AddHook registers a hook that is called for every log entry.
func (l *Logger) AddHook(hook LogHook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// Debug logs a debug message with structured key-value pairs.
func (l *Logger) Debug(event string, kvs ...any) {
	if l.level > LevelDebug {
		return
	}
	l.log("debug", event, kvs...)
}

// Info logs an informational message.
func (l *Logger) Info(event string, kvs ...any) {
	if l.level > LevelInfo {
		return
	}
	l.log("info", event, kvs...)
}

// Warn logs a warning.
func (l *Logger) Warn(event string, kvs ...any) {
	if l.level > LevelWarn {
		return
	}
	l.log("warn", event, kvs...)
}

// Error logs an error.
func (l *Logger) Error(event string, kvs ...any) {
	l.log("error", event, kvs...)
}

func (l *Logger) log(level, event string, kvs ...any) {
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     level,
		Event:     event,
	}

	if len(kvs) >= 2 {
		entry.Data = make(map[string]any, len(kvs)/2)
		for i := 0; i+1 < len(kvs); i += 2 {
			key, ok := kvs[i].(string)
			if !ok {
				key = fmt.Sprintf("%v", kvs[i])
			}
			val := kvs[i+1]
			if e, ok := val.(error); ok {
				val = e.Error()
			}
			entry.Data[key] = val
		}
	}

	l.mu.Lock()
	data, err := json.Marshal(entry)
	if err == nil {
		_, _ = fmt.Fprintln(l.writer, string(data))
	}

	hooks := l.hooks
	l.mu.Unlock()

	for _, hook := range hooks {
		hook(entry)
	}
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return i
		}
	}
	return -1
}
