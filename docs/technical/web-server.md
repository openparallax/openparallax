# Web Server

The web server (`internal/web/`) serves the embedded Svelte frontend, REST API, and WebSocket connections. It runs inside the Engine process and shares the Engine's data structures directly.

## Server Architecture

```go
type Server struct {
    engine   *engine.Engine
    log      *logging.Logger
    port     int
    host     string
    server   *http.Server
    listener net.Listener
    auth     *authConfig
    connsMu  sync.Mutex
    conns    map[*websocket.Conn]context.Context
}
```

### Configuration

```go
type ServerConfig struct {
    Host         string  // Bind host (default "127.0.0.1")
    Port         int     // HTTP port (default 3100 from config)
    PasswordHash string  // bcrypt hash for remote auth
}
```

## Startup: Listen and Serve

The server uses a two-phase startup to ensure the port is bound before the browser opens:

### Phase 1: Listen

`Listen()` binds the port, sets up routes, and returns. No connections are accepted yet.

```go
func (s *Server) Listen() error {
    mux := http.NewServeMux()
    s.registerAPIRoutes(mux)
    mux.HandleFunc("/api/ws", s.handleWebSocket)
    mux.HandleFunc("/", spaFallbackHandler)

    handler := withCORS(mux)
    if s.auth != nil {
        handler = withAuth(handler, s.auth)
    }

    s.server = &http.Server{
        Addr:              addr,
        Handler:           handler,
        ReadHeaderTimeout: 10 * time.Second,
        WriteTimeout:      0,    // WebSocket needs unlimited write time
        IdleTimeout:       120 * time.Second,
    }

    listener, err := net.Listen("tcp", addr)
    s.listener = listener
    return nil
}
```

### Phase 2: Serve

`Serve()` starts accepting connections. Called in a goroutine after `Listen()` succeeds.

```go
func (s *Server) Serve() error {
    return s.server.Serve(s.listener)
}
```

This split lets the Engine confirm the port is bound, write `WEB:<port>` to stdout, and optionally open the browser -- all before any HTTP requests are processed.

## Static File Serving

The Svelte frontend is embedded into the Go binary via `go:embed`:

```go
//go:embed dist
var distFS embed.FS
```

The `dist/` directory contains the Vite build output. The handler serves files directly from the embedded filesystem. For SPA routing, if a file is not found, the handler falls back to `index.html`:

```go
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if f, err := staticFS.Open(path[1:]); err == nil {
        f.Close()
        fileServer.ServeHTTP(w, r)  // Serve the actual file
        return
    }
    r.URL.Path = "/"
    fileServer.ServeHTTP(w, r)  // SPA fallback to index.html
})
```

## REST API Endpoints

All REST endpoints are registered in `registerAPIRoutes`:

| Method | Path | Handler | Description |
|---|---|---|---|
| GET | `/api/status` | `handleStatus` | Agent name, model, session count, sandbox status |
| GET | `/api/sessions` | `handleListSessions` | List all sessions |
| POST | `/api/sessions` | `handleCreateSession` | Create a new session |
| GET | `/api/sessions/{id}` | `handleGetSession` | Get session details |
| DELETE | `/api/sessions/{id}` | `handleDeleteSession` | Delete a session |
| PATCH | `/api/sessions/{id}` | `handleUpdateSession` | Update session title |
| GET | `/api/sessions/{id}/messages` | `handleGetMessages` | Get conversation history |
| GET | `/api/sessions/search` | `handleSearchSessions` | FTS5 search across sessions |
| GET | `/api/tools` | `handleListTools` | List available tools |
| GET | `/api/memory/search` | `handleMemorySearch` | FTS5 search across memory |
| GET | `/api/memory/{type}` | `handleReadMemory` | Read a memory file |
| GET | `/api/logs` | `handleLogs` | Tail engine.log (supports `?lines=`, `?level=`, `?event=`) |
| GET | `/api/audit` | `handleAudit` | Query audit log with chain verification |
| GET | `/api/settings` | `handleGetSettings` | Read configuration |
| PUT | `/api/settings` | `handlePutSettings` | Update configuration |
| POST | `/api/settings/test-mcp` | `handleTestMCP` | Test MCP server connection |
| POST | `/api/restart` | `handleRestart` | Restart engine (calls `os.Exit(75)`) |
| GET | `/api/sub-agents` | `handleListSubAgents` | List active sub-agents |
| POST | `/api/login` | `handleLogin` | Authenticate (when auth is enabled) |
| GET | `/api/channels` | `handleListChannels` | List active channel adapters and status |
| POST | `/api/channels/detach` | `handleDetachChannel` | Detach a channel adapter |

## WebSocket Handler

`handleWebSocket` upgrades HTTP connections to WebSocket for real-time chat:

```go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := websocket.Accept(w, r, &websocket.AcceptOptions{
        OriginPatterns: []string{"*"},
    })

    s.registerConn(conn, ctx)
    defer s.unregisterConn(conn)

    for {
        _, data, _ := conn.Read(ctx)
        var msg wsClientMessage
        json.Unmarshal(data, &msg)

        switch msg.Type {
        case "message":
            go s.handleWSMessage(ctx, conn, msg)
        case "tier3_decision":
            s.handleTier3Decision(msg)
        case "ping":
            s.writeWSJSON(ctx, conn, map[string]string{"type": "pong"})
        }
    }
}
```

### Client Message Format

```go
type wsClientMessage struct {
    Type      string `json:"type"`        // "message", "tier3_decision", "ping"
    SessionID string `json:"session_id"`
    Content   string `json:"content"`
    Mode      string `json:"mode"`        // "normal" or "otr"
    ActionID  string `json:"action_id"`   // For tier3_decision
    Decision  string `json:"decision"`    // "approve" or "deny"
}
```

### Message Handling

`handleWSMessage` runs in a goroutine (to keep the read loop responsive for pings and control frames):

1. Generate a message ID.
2. Create session if `session_id` is empty.
3. Create a `wsEventSender` for this connection.
4. Call `engine.ProcessMessageForWeb(ctx, sender, sid, mid, content, mode)`.

Events flow back through the `wsEventSender`, which serializes `PipelineEvent` structs as JSON and writes them to the WebSocket connection.

### Log Entry Broadcasting

The server registers a `LogHook` on the logger. Every structured log entry is broadcast to all connected WebSocket clients as a `log_entry` event:

```go
log.AddHook(func(entry logging.LogEntry) {
    s.broadcastLogEntry(entry)
})
```

Log entries are global events -- they are not filtered by session. They are delivered to all connected WebSocket clients regardless of which session they are viewing.

## Cookie-Based Authentication

Authentication is enabled when the server binds to a non-localhost address and a password hash is configured:

```go
if !isLocalhost(host) && cfg.PasswordHash != "" {
    s.auth = &authConfig{
        passwordHash: cfg.PasswordHash,
        sessionToken: GenerateSessionToken(),
        isRemote:     true,
    }
}
```

### Login Flow

1. Client sends `POST /api/login` with `{"password": "..."}`.
2. Server compares the password against the bcrypt hash.
3. On success, sets a cookie:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "op_session",
    Value:    cfg.sessionToken,  // Random 64-char hex token
    Path:     "/",
    HttpOnly: true,              // Not accessible from JavaScript
    Secure:   true,              // Only sent over HTTPS
    SameSite: http.SameSiteStrictMode, // No cross-site requests
    MaxAge:   86400,             // 24 hours
})
```

### Request Verification

The `withAuth` middleware checks the `op_session` cookie on every request (except `/api/login`):

```go
func withAuth(next http.Handler, cfg *authConfig) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/login" {
            handleLogin(w, r, cfg)
            return
        }
        cookie, err := r.Cookie("op_session")
        if err != nil || cookie.Value != cfg.sessionToken {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

The session token is regenerated on every server start, so all existing sessions are invalidated on restart.

### Login Rate Limiting

The `/api/login` endpoint is rate-limited to 5 attempts per minute per IP address. Requests exceeding this limit receive a `429 Too Many Requests` response. This prevents brute-force password guessing.

### WebSocket Authentication

When authentication is enabled, the WebSocket upgrade request at `/api/ws` is subject to the same cookie-based authentication as REST endpoints. The `op_session` cookie must be present and valid for the upgrade to succeed. Unauthenticated WebSocket connections are rejected with `401 Unauthorized` before the protocol upgrade occurs.

### WebSocket Message Size Limit

WebSocket messages are limited to 10MB. Messages exceeding this size cause the connection to be closed with a protocol error. This prevents memory exhaustion from oversized payloads.

### Localhost Bypass

When the server binds to `127.0.0.1`, `localhost`, or `::1`, authentication is not applied. This is the default configuration. Auth only activates for remote access scenarios (e.g., binding to `0.0.0.0`).

## CORS

The `withCORS` middleware validates request origins against a configured allowlist. When no origins are configured, only localhost origins (`http://localhost:*`, `http://127.0.0.1:*`) are permitted. This prevents cross-origin requests from arbitrary websites.

```go
// Validates Origin header against configured allowed origins.
// Localhost-only when the allowlist is empty.
```

OPTIONS preflight requests return 204 immediately.

## Setup Mode

`SetupServer` serves a special setup wizard when no workspace exists. It provides:

- `GET /api/status`: Returns `{"setup_required": true}`.
- `POST /api/setup/test-provider`: Tests an LLM provider connection.
- `POST /api/setup/test-embedding`: Tests an embedding provider connection.
- `POST /api/setup/complete`: Creates the workspace, writes `config.yaml`, and signals completion via the `doneCh` channel.

The setup server transitions to the full server after completion.

## Shutdown

```go
func (s *Server) Stop() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    s.server.Shutdown(ctx)
}
```

Graceful shutdown with a 5-second timeout for in-flight requests.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/web/server.go` | Server struct, Listen, Serve, Stop, log broadcasting |
| `internal/web/handlers.go` | REST API handlers |
| `internal/web/websocket.go` | WebSocket handler, message processing |
| `internal/web/ws_sender.go` | wsEventSender implementation |
| `internal/web/middleware.go` | CORS, auth middleware, cookie handling |
| `internal/web/setup.go` | SetupServer for onboarding wizard |
| `internal/web/embed.go` | go:embed directive for Svelte build |
| `internal/web/settings.go` | Settings read/write handlers |
