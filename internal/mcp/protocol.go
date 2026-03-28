package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/openparallax/openparallax/internal/llm"
)

// MCP JSON-RPC protocol implementation over stdio.
// This is a minimal implementation sufficient for tool discovery and execution.

var requestID atomic.Int64

type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type stdinWriter struct {
	w  io.WriteCloser
	mu sync.Mutex
}

func newStdinWriter(w io.WriteCloser) *stdinWriter {
	return &stdinWriter{w: w}
}

func (sw *stdinWriter) send(req jsonrpcRequest) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	// MCP uses newline-delimited JSON.
	_, err = fmt.Fprintf(sw.w, "%s\n", data)
	return err
}

type stdoutReader struct {
	scanner *bufio.Scanner
	mu      sync.Mutex
}

func newStdoutReader(r io.ReadCloser) *stdoutReader {
	return &stdoutReader{scanner: bufio.NewScanner(r)}
}

func (sr *stdoutReader) recv() (jsonrpcResponse, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if !sr.scanner.Scan() {
		if err := sr.scanner.Err(); err != nil {
			return jsonrpcResponse{}, err
		}
		return jsonrpcResponse{}, io.EOF
	}
	var resp jsonrpcResponse
	if err := json.Unmarshal(sr.scanner.Bytes(), &resp); err != nil {
		return jsonrpcResponse{}, err
	}
	return resp, nil
}

// initializeLocked sends the MCP initialize handshake and discovers tools.
func (c *Client) initializeLocked(_ context.Context) ([]llm.ToolDefinition, error) {
	// Send initialize request.
	initID := requestID.Add(1)
	if err := c.stdin.send(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      initID,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "openparallax",
				"version": "1.0.0",
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("send initialize: %w", err)
	}

	// Read initialize response.
	resp, err := c.stdout.recv()
	if err != nil {
		return nil, fmt.Errorf("recv initialize: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// Send initialized notification.
	_ = c.stdin.send(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "notifications/initialized",
	})

	// Discover tools.
	toolsID := requestID.Add(1)
	if err := c.stdin.send(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      toolsID,
		Method:  "tools/list",
	}); err != nil {
		return nil, fmt.Errorf("send tools/list: %w", err)
	}

	toolsResp, err := c.stdout.recv()
	if err != nil {
		return nil, fmt.Errorf("recv tools/list: %w", err)
	}
	if toolsResp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", toolsResp.Error.Message)
	}

	var toolsResult struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(toolsResp.Result, &toolsResult); err != nil {
		return nil, fmt.Errorf("parse tools: %w", err)
	}

	var tools []llm.ToolDefinition
	for _, t := range toolsResult.Tools {
		tools = append(tools, llm.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return tools, nil
}

// callToolLocked sends a tool call to the MCP server and returns the result.
func (c *Client) callToolLocked(_ context.Context, name string, args map[string]any) (string, error) {
	callID := requestID.Add(1)
	if err := c.stdin.send(jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      callID,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}); err != nil {
		return "", fmt.Errorf("send tools/call: %w", err)
	}

	resp, err := c.stdout.recv()
	if err != nil {
		return "", fmt.Errorf("recv tools/call: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("tool call error: %s", resp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return string(resp.Result), nil
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	if len(texts) == 0 {
		return string(resp.Result), nil
	}
	return texts[0], nil
}
