package engine

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// StreamingRedactor buffers LLM output tokens and redacts secrets before
// flushing to the client. Secrets never reach the client, even during streaming.
type StreamingRedactor struct {
	buffer   strings.Builder
	flush    func(string)
	patterns []*regexp.Regexp
	prefixes []string
	interval time.Duration
	timer    *time.Timer
	mu       sync.Mutex
}

// NewStreamingRedactor creates a redactor with the given flush callback.
func NewStreamingRedactor(flush func(string)) *StreamingRedactor {
	return &StreamingRedactor{
		flush:    flush,
		patterns: compileSecretPatterns(),
		prefixes: secretPrefixes(),
		interval: 50 * time.Millisecond,
	}
}

// Write adds text to the buffer and flushes safe content.
func (r *StreamingRedactor) Write(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buffer.WriteString(text)

	if r.hasPrefixMatch() {
		r.resetTimer()
		return
	}

	r.flushSafe()
}

// Flush forces all buffered text to be flushed with redaction applied.
func (r *StreamingRedactor) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}

	if r.buffer.Len() > 0 {
		clean := r.redact(r.buffer.String())
		r.buffer.Reset()
		if clean != "" {
			r.flush(clean)
		}
	}
}

func (r *StreamingRedactor) flushSafe() {
	text := r.buffer.String()

	safeEnd := r.findSafeFlushPoint(text)
	if safeEnd <= 0 {
		r.resetTimer()
		return
	}

	safe := text[:safeEnd]
	remainder := text[safeEnd:]

	clean := r.redact(safe)
	r.buffer.Reset()
	r.buffer.WriteString(remainder)

	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}

	if clean != "" {
		r.flush(clean)
	}
}

func (r *StreamingRedactor) redact(text string) string {
	for _, p := range r.patterns {
		text = p.ReplaceAllStringFunc(text, func(match string) string {
			if strings.Contains(match, "BEGIN") && strings.Contains(match, "KEY") {
				return "[REDACTED: private key]"
			}
			if len(match) > 4 {
				return match[:4] + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return text
}

func (r *StreamingRedactor) hasPrefixMatch() bool {
	text := r.buffer.String()
	tail := text
	if len(tail) > 60 {
		tail = tail[len(tail)-60:]
	}
	for _, prefix := range r.prefixes {
		if strings.Contains(tail, prefix) {
			return true
		}
	}
	return false
}

func (r *StreamingRedactor) findSafeFlushPoint(text string) int {
	earliest := len(text)
	for _, prefix := range r.prefixes {
		idx := strings.LastIndex(text, prefix)
		if idx >= 0 && idx < earliest {
			earliest = idx
		}
	}
	return earliest
}

func (r *StreamingRedactor) resetTimer() {
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(r.interval, func() {
		r.Flush()
	})
}

func compileSecretPatterns() []*regexp.Regexp {
	raw := []string{
		`-----BEGIN\s+(RSA|EC|OPENSSH|PGP)\s+PRIVATE\s+KEY-----`,
		`AKIA[0-9A-Z]{16}`,
		`ghp_[a-zA-Z0-9]{36}`,
		`(sk|pk)_(live|test)_[a-zA-Z0-9]{24,}`,
		`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]{20,}`,
		`(mongodb|postgres|mysql|redis)://[^\s]+`,
		`https?://[^:]+:[^@]+@[^\s]+`,
		`(?i)(api[_-]?key|secret[_-]?key|access[_-]?token)\s*[:=]\s*['"][^\s'"]{8,}`,
		`AIZA[a-zA-Z0-9\-_]{35}`,
		`xox[bporas]-[a-zA-Z0-9-]+`,
	}
	compiled := make([]*regexp.Regexp, 0, len(raw))
	for _, p := range raw {
		compiled = append(compiled, regexp.MustCompile(p))
	}
	return compiled
}

func secretPrefixes() []string {
	return []string{
		"-----BEGIN", "AKIA", "ghp_", "sk_live_", "sk_test_", "pk_live_", "pk_test_",
		"eyJ", "mongodb://", "postgres://", "mysql://", "redis://",
		"AIZA", "xoxb-", "xoxp-", "xoxa-",
	}
}
