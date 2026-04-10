// Command channels-bridge is a JSON-RPC 2.0 server over stdin/stdout that
// exposes the channel adapter utilities for cross-language wrappers.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/openparallax/openparallax/channels"
	"github.com/openparallax/openparallax/internal/jsonrpc"
)

func main() {
	server := jsonrpc.NewServer()

	server.Handle("split_message", func(params json.RawMessage) (any, error) {
		var p struct {
			Content   string `json:"content"`
			MaxLength int    `json:"max_length"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		if p.MaxLength <= 0 {
			p.MaxLength = 4096
		}
		return channels.SplitMessage(p.Content, p.MaxLength), nil
	})

	server.Handle("format_message", func(params json.RawMessage) (any, error) {
		var p struct {
			Text   string `json:"text"`
			Format int    `json:"format"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		msg := channels.ChannelMessage{
			Text:   p.Text,
			Format: channels.MessageFormat(p.Format),
		}
		return msg, nil
	})

	if err := server.Serve(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "channels-bridge: %v\n", err)
		os.Exit(1)
	}
}
