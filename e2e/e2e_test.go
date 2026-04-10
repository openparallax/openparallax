//go:build e2e

// Package e2e provides end-to-end integration tests for OpenParallax.
//
// A single engine boots in TestMain and is shared across all tests. Each
// test creates its own session for isolation. Restart tests (config persist,
// memory persist) stop and restart the shared engine mid-test.
//
// Run with: go test -tags e2e -v -timeout 300s ./e2e/...
//
// LLM modes (E2E_LLM env var):
//   - mock   (default) — deterministic mock LLM, no API key needed
//   - ollama           — real Ollama model (set OLLAMA_MODEL)
//   - cloud            — real provider (set E2E_PROVIDER + API key + optional E2E_BASE_URL, E2E_MODEL)
package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func parseJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse JSON: %v (data: %s)", err, string(data))
	}
	return m
}

func TestMain(m *testing.M) {
	SharedEngine = SetupSharedEngine()

	if err := SharedEngine.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: engine start failed: %v\n", err)
		SharedEngine.Cleanup()
		os.Exit(1)
	}

	code := m.Run()

	SharedEngine.Cleanup()
	os.Exit(code)
}
