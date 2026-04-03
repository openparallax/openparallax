// Command audit-bridge is a JSON-RPC 2.0 server over stdin/stdout that
// exposes the audit logging system for cross-language wrappers.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/internal/jsonrpc"
)

var logger *audit.Logger

func main() {
	server := jsonrpc.NewServer()

	server.Handle("configure", func(params json.RawMessage) (any, error) {
		var cfg struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		l, err := audit.NewLogger(cfg.Path)
		if err != nil {
			return nil, err
		}
		logger = l
		return map[string]bool{"ok": true}, nil
	})

	server.Handle("log", func(params json.RawMessage) (any, error) {
		if logger == nil {
			return nil, fmt.Errorf("not configured: call configure first")
		}
		var entry audit.Entry
		if err := json.Unmarshal(params, &entry); err != nil {
			return nil, fmt.Errorf("invalid entry: %w", err)
		}
		if err := logger.Log(entry); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	server.Handle("verify", func(params json.RawMessage) (any, error) {
		var cfg struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(params, &cfg); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		err := audit.VerifyIntegrity(cfg.Path)
		return map[string]any{
			"valid": err == nil,
			"error": fmt.Sprintf("%v", err),
		}, nil
	})

	server.Handle("query", func(params json.RawMessage) (any, error) {
		var q struct {
			Path  string      `json:"path"`
			Query audit.Query `json:"query"`
		}
		if err := json.Unmarshal(params, &q); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		entries, err := audit.ReadEntries(q.Path, q.Query)
		if err != nil {
			return nil, err
		}
		return entries, nil
	})

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "audit-bridge: %v\n", err)
		os.Exit(1)
	}
}
