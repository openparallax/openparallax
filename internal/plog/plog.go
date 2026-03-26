// Package plog provides structured pipeline logging for diagnostic output.
// When verbose mode is enabled, each pipeline stage logs its activity to stderr.
// When disabled, all logging is silently discarded.
package plog

import (
	"fmt"
	"os"
)

// Logger writes structured pipeline diagnostic output.
type Logger struct {
	enabled bool
}

// New creates a Logger. When enabled is false, all output is discarded.
func New(enabled bool) *Logger {
	return &Logger{enabled: enabled}
}

// Log writes a formatted pipeline log line to stderr.
// The stage identifies the pipeline component (e.g., "parser", "shield", "executor").
func (l *Logger) Log(stage, format string, args ...any) {
	if !l.enabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", stage, msg)
}

// Enabled returns whether verbose logging is active.
func (l *Logger) Enabled() bool {
	return l.enabled
}
