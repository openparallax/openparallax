// Command shield-bridge is a JSON-RPC 2.0 server over stdin/stdout that
// exposes the Shield security pipeline for cross-language wrappers.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/openparallax/openparallax/internal/jsonrpc"
	"github.com/openparallax/openparallax/shield"
)

var pipeline *shield.Pipeline

func main() {
	server := jsonrpc.NewServer()

	server.Handle("configure", func(params json.RawMessage) (any, error) {
		var cfg shield.Config
		if err := json.Unmarshal(params, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		p, err := shield.NewPipeline(cfg)
		if err != nil {
			return nil, err
		}
		pipeline = p
		return map[string]bool{"ok": true}, nil
	})

	server.Handle("evaluate", func(params json.RawMessage) (any, error) {
		if pipeline == nil {
			return nil, fmt.Errorf("not configured: call configure first")
		}
		var action shield.ActionRequest
		if err := json.Unmarshal(params, &action); err != nil {
			return nil, fmt.Errorf("invalid action: %w", err)
		}
		if action.Timestamp.IsZero() {
			action.Timestamp = time.Now()
		}
		verdict := pipeline.Evaluate(context.Background(), &action)
		return verdict, nil
	})

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "shield-bridge: %v\n", err)
		os.Exit(1)
	}
}
