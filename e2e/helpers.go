//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// writeTestFile creates a file in the workspace with the given content.
func writeTestFile(workspace, name, content string) error {
	return os.WriteFile(filepath.Join(workspace, name), []byte(content), 0o644)
}

// json helper for harness (non-test context).
func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
