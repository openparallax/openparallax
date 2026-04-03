// Command sandbox-bridge is a JSON-RPC 2.0 server over stdin/stdout that
// exposes kernel sandbox verification for cross-language wrappers.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openparallax/openparallax/internal/jsonrpc"
	"github.com/openparallax/openparallax/sandbox"
)

func main() {
	server := jsonrpc.NewServer()

	server.Handle("verify_canary", func(_ json.RawMessage) (any, error) {
		result := sandbox.VerifyCanary()
		return result, nil
	})

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox-bridge: %v\n", err)
		os.Exit(1)
	}
}
