package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/openparallax/openparallax/internal/engine"
	"github.com/openparallax/openparallax/internal/logging"
)

// Server is the HTTP server for the Web UI.
// It serves the embedded Svelte application, REST API endpoints,
// and a WebSocket connection for real-time chat.
// Server is the HTTP server for the Web UI.
// It serves the embedded Svelte application, REST API endpoints,
// and a WebSocket connection for real-time chat.
type Server struct {
	engine   *engine.Engine
	log      *logging.Logger
	port     int
	host     string
	server   *http.Server
	listener net.Listener
	auth     *authConfig

	connsMu sync.Mutex
	conns   map[*websocket.Conn]context.Context
	senders map[*websocket.Conn]engine.EventSender
}

// ServerConfig holds web server configuration.
type ServerConfig struct {
	Host         string // bind host (default "127.0.0.1")
	Port         int
	PasswordHash string // bcrypt hash for remote auth (empty = no auth)
}

// NewServer creates a web server.
func NewServer(eng *engine.Engine, log *logging.Logger, cfg ServerConfig) *Server {
	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}

	s := &Server{
		engine:  eng,
		log:     log,
		port:    cfg.Port,
		host:    host,
		conns:   make(map[*websocket.Conn]context.Context),
		senders: make(map[*websocket.Conn]engine.EventSender),
	}

	// Enable auth for non-localhost bindings.
	if !isLocalhost(host) && cfg.PasswordHash != "" {
		s.auth = &authConfig{
			passwordHash: cfg.PasswordHash,
			sessionToken: GenerateSessionToken(),
			isRemote:     true,
		}
	}

	log.AddHook(func(entry logging.LogEntry) {
		s.broadcastLogEntry(entry)
	})

	return s
}

// registerConn adds a WebSocket connection for log broadcasting.
func (s *Server) registerConn(conn *websocket.Conn, ctx context.Context) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	s.conns[conn] = ctx
}

// unregisterConn removes a WebSocket connection and its event sender.
func (s *Server) unregisterConn(conn *websocket.Conn) {
	s.connsMu.Lock()
	sender := s.senders[conn]
	delete(s.conns, conn)
	delete(s.senders, conn)
	s.connsMu.Unlock()

	if sender != nil {
		clientID := fmt.Sprintf("ws-%p", sender)
		s.engine.Broadcaster().Unsubscribe(clientID)
	}
}

// getOrCreateSender returns the stable EventSender for a WebSocket connection.
func (s *Server) getOrCreateSender(conn *websocket.Conn, ctx context.Context) engine.EventSender {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if sender, ok := s.senders[conn]; ok {
		return sender
	}
	sender := newWSEventSender(ctx, conn)
	s.senders[conn] = sender
	return sender
}

// broadcastLogEntry sends a log entry to all connected WebSocket clients.
func (s *Server) broadcastLogEntry(entry logging.LogEntry) {
	msg := map[string]any{
		"type":  "log_entry",
		"entry": entry,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	s.connsMu.Lock()
	snapshot := make(map[*websocket.Conn]context.Context, len(s.conns))
	for conn, ctx := range s.conns {
		snapshot[conn] = ctx
	}
	s.connsMu.Unlock()

	for conn, ctx := range snapshot {
		_ = conn.Write(ctx, websocket.MessageText, data)
	}
}

// BroadcastEvent sends a pipeline event to all connected WebSocket clients.
// Used for sub-agent lifecycle events that are not tied to a specific message stream.
func (s *Server) BroadcastEvent(event *engine.PipelineEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.connsMu.Lock()
	snapshot := make(map[*websocket.Conn]context.Context, len(s.conns))
	for conn, ctx := range s.conns {
		snapshot[conn] = ctx
	}
	s.connsMu.Unlock()

	for conn, ctx := range snapshot {
		_ = conn.Write(ctx, websocket.MessageText, data)
	}
}

// Listen binds the web server port and prepares routes. Call Serve to
// start accepting connections. Splitting listen from serve lets callers
// confirm the port is bound before opening a browser.
func (s *Server) Listen() error {
	mux := http.NewServeMux()

	// Static files — embedded Svelte build.
	staticFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("failed to create sub filesystem: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	// REST API.
	s.registerAPIRoutes(mux)

	// WebSocket.
	mux.HandleFunc("/api/ws", s.handleWebSocket)

	// Static files — fallback to index.html for SPA routing.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try serving the exact file first.
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		// Check if the file exists in the embedded FS.
		if f, err := staticFS.Open(path[1:]); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	handler := withCORS(mux)
	if s.auth != nil {
		handler = withAuth(handler, s.auth)
	}
	s.server = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0, // WebSocket needs unlimited write time.
		IdleTimeout:       120 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("web server listen on %s: %w", addr, err)
	}
	s.listener = listener
	s.log.Info("web_server_listening", "addr", addr)
	return nil
}

// Serve starts accepting connections. Must be called after Listen.
func (s *Server) Serve() error {
	if s.listener == nil {
		return fmt.Errorf("web server: Listen must be called before Serve")
	}
	return s.server.Serve(s.listener)
}

// Stop gracefully shuts down the web server.
func (s *Server) Stop() {
	if s.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.server.Shutdown(ctx)
}

// Port returns the configured port.
func (s *Server) Port() int { return s.port }
