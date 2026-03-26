package parser

import "regexp"

// KeywordDetector checks for destructive keywords using regex patterns.
type KeywordDetector struct {
	patterns []*regexp.Regexp
}

// NewKeywordDetector creates a KeywordDetector with 15 destructive patterns.
func NewKeywordDetector() *KeywordDetector {
	rawPatterns := []string{
		`(?i)\b(delete|remove|destroy|erase|wipe|purge)\b.*\b(all|every|entire)\b`,
		`(?i)\brm\s+-rf?\b`,
		`(?i)\bformat\s+(c:|d:|the\s+drive)\b`,
		`(?i)\bdrop\s+(table|database|schema)\b`,
		`(?i)\btruncate\s+table\b`,
		`(?i)\b(overwrite|replace)\s+(all|every)\b`,
		`(?i)\bfactory\s+reset\b`,
		`(?i)\bgit\s+push\s+(-f|--force)\b`,
		`(?i)\bgit\s+reset\s+--hard\b`,
		`(?i)\bchmod\s+777\b`,
		`(?i)\bsudo\s+rm\b`,
		`(?i)\bmkfs\b`,
		`(?i)\bdd\s+if=.*of=\b`,
		`(?i)\b(shutdown|reboot|halt)\s+(-h|now)\b`,
		`(?i)\bkill\s+-9\s+(-1|1)\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(rawPatterns))
	for _, p := range rawPatterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}

	return &KeywordDetector{patterns: compiled}
}

// IsDestructive returns true if the input matches any destructive keyword pattern.
func (k *KeywordDetector) IsDestructive(input string) bool {
	for _, p := range k.patterns {
		if p.MatchString(input) {
			return true
		}
	}
	return false
}
