package shield

// Logger is the logging interface Shield requires.
type Logger interface {
	Info(event string, keysAndValues ...any)
	Warn(event string, keysAndValues ...any)
	Debug(event string, keysAndValues ...any)
}

// nopLogger is a no-op Logger used when no logger is provided.
type nopLogger struct{}

func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Debug(string, ...any) {}
