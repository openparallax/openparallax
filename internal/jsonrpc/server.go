// Package jsonrpc provides a minimal JSON-RPC 2.0 server over stdin/stdout.
// Designed for cross-language bridge binaries.
package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error is a JSON-RPC 2.0 error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Handler processes a JSON-RPC method call.
type Handler func(params json.RawMessage) (any, error)

// Server is a JSON-RPC 2.0 server that reads from stdin and writes to stdout.
type Server struct {
	handlers map[string]Handler
	mu       sync.Mutex
}

// NewServer creates a new JSON-RPC server.
func NewServer() *Server {
	return &Server{handlers: make(map[string]Handler)}
}

// Handle registers a handler for a method name.
func (s *Server) Handle(method string, h Handler) {
	s.handlers[method] = h
}

// Serve reads requests from stdin and writes responses to stdout.
// It blocks until stdin is closed or an unrecoverable error occurs.
func (s *Server) Serve() error {
	return s.ServeIO(os.Stdin, os.Stdout)
}

// ServeIO reads requests from r and writes responses to w.
func (s *Server) ServeIO(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(w, Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &Error{Code: -32700, Message: "parse error"},
			})
			continue
		}

		s.mu.Lock()
		handler, ok := s.handlers[req.Method]
		s.mu.Unlock()

		if !ok {
			s.writeResponse(w, Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
			})
			continue
		}

		result, err := handler(req.Params)
		if err != nil {
			s.writeResponse(w, Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: -32000, Message: err.Error()},
			})
			continue
		}

		s.writeResponse(w, Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		})
	}

	return scanner.Err()
}

func (s *Server) writeResponse(w io.Writer, resp Response) {
	data, _ := json.Marshal(resp)
	_, _ = fmt.Fprintf(w, "%s\n", data)
}
