package engine

import (
	"regexp"
	"strings"
)

// ResponseChecker scans agent responses for secrets before sending to the user.
type ResponseChecker struct {
	patterns []*regexp.Regexp
}

// NewResponseChecker creates a ResponseChecker with 10 secret detection patterns.
func NewResponseChecker() *ResponseChecker {
	rawPatterns := []string{
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

	compiled := make([]*regexp.Regexp, 0, len(rawPatterns))
	for _, p := range rawPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}

	return &ResponseChecker{patterns: compiled}
}

// Redact replaces detected secrets with [REDACTED] markers.
func (c *ResponseChecker) Redact(text string) string {
	for _, p := range c.patterns {
		text = p.ReplaceAllStringFunc(text, func(match string) string {
			if strings.Contains(match, "BEGIN") && strings.Contains(match, "KEY") {
				return "[REDACTED: private key detected]"
			}
			if len(match) > 8 {
				return match[:4] + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return text
}
