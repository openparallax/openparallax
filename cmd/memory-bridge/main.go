// Command memory-bridge is a JSON-RPC 2.0 server over stdin/stdout that
// exposes the semantic memory system for cross-language wrappers.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openparallax/openparallax/internal/jsonrpc"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/memory"
)

var mgr *memory.Manager

func main() {
	server := jsonrpc.NewServer()

	server.Handle("configure", func(params json.RawMessage) (any, error) {
		var cfg struct {
			Workspace string `json:"workspace"`
			DBPath    string `json:"db_path"`
		}
		if err := json.Unmarshal(params, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		db, err := storage.Open(cfg.DBPath)
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		mgr = memory.NewManager(cfg.Workspace, db, nil)
		return map[string]bool{"ok": true}, nil
	})

	server.Handle("search", func(params json.RawMessage) (any, error) {
		if mgr == nil {
			return nil, fmt.Errorf("not configured: call configure first")
		}
		var q struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(params, &q); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		results, err := mgr.Search(q.Query, q.Limit)
		if err != nil {
			return nil, err
		}
		return results, nil
	})

	server.Handle("read", func(params json.RawMessage) (any, error) {
		if mgr == nil {
			return nil, fmt.Errorf("not configured: call configure first")
		}
		var p struct {
			FileType string `json:"file_type"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		content, err := mgr.Read(types.MemoryFileType(p.FileType))
		if err != nil {
			return nil, err
		}
		return map[string]string{"content": content}, nil
	})

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "memory-bridge: %v\n", err)
		os.Exit(1)
	}
}
