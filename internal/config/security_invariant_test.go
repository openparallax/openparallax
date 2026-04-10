package config

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestSettableKeysContainsNoSecurityNonNegotiables enforces the doc-as-test
// invariant: the list of forbidden keys is parsed from
// docs/security/non-negotiable.md, and any key that appears both there and
// in SettableKeys is a structural violation.
//
// To add a new non-negotiable: edit non-negotiable.md, add the key to the
// "Forbidden Config Keys" section. To remove one (the dangerous direction):
// the doc edit will appear in code review, where it can be challenged.
func TestSettableKeysContainsNoSecurityNonNegotiables(t *testing.T) {
	forbidden := parseForbiddenKeysFromDoc(t)
	if len(forbidden) == 0 {
		t.Fatal("non-negotiable.md parsed zero forbidden keys — doc shape changed?")
	}
	for _, key := range forbidden {
		if _, ok := SettableKeys[key]; ok {
			t.Fatalf("FORBIDDEN: %q is in SettableKeys but listed as non-negotiable in docs/security/non-negotiable.md", key)
		}
	}
	t.Logf("verified %d forbidden keys are absent from SettableKeys", len(forbidden))
}

func parseForbiddenKeysFromDoc(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../docs/security/non-negotiable.md")
	if err != nil {
		t.Fatalf("read non-negotiable.md: %v", err)
	}
	// Parse a fenced code block following the heading "Forbidden Config Keys".
	re := regexp.MustCompile("(?s)## Forbidden Config Keys.*?```\\n(.*?)```")
	m := re.FindSubmatch(raw)
	if len(m) != 2 {
		t.Fatal("non-negotiable.md missing 'Forbidden Config Keys' code block")
	}
	var keys []string
	for _, line := range strings.Split(string(m[1]), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			keys = append(keys, line)
		}
	}
	return keys
}
