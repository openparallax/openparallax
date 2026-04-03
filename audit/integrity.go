package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openparallax/openparallax/crypto"
)

// VerifyIntegrity checks the hash chain of an audit log file.
// Returns nil if the chain is valid, or an error describing the first violation.
func VerifyIntegrity(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read audit log: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}

	prevHash := ""
	for i, line := range lines {
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("line %d: invalid JSON: %w", i+1, err)
		}

		if entry.PreviousHash != prevHash {
			return fmt.Errorf("line %d: chain broken: previous_hash %q does not match expected %q",
				i+1, entry.PreviousHash, prevHash)
		}

		// Recompute the hash to verify the entry hasn't been modified.
		storedHash := entry.Hash
		entry.Hash = ""
		canonical, canonErr := crypto.Canonicalize(entry)
		if canonErr != nil {
			return fmt.Errorf("line %d: canonicalization failed: %w", i+1, canonErr)
		}
		computed := crypto.SHA256Hex(canonical)
		if computed != storedHash {
			return fmt.Errorf("line %d: hash mismatch: stored %q, computed %q",
				i+1, storedHash, computed)
		}

		prevHash = storedHash
	}

	return nil
}
