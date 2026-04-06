package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// isAllowedOrigin checks whether the given origin is permitted by the allowed
// origins list. When the list is empty, only localhost origins are accepted.
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}
	if len(allowedOrigins) == 0 {
		return strings.HasPrefix(origin, "http://localhost:") ||
			strings.HasPrefix(origin, "http://127.0.0.1:")
	}
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

// withCORS wraps a handler with CORS headers restricted to allowed origins.
// When allowedOrigins is empty, only localhost origins are permitted.
func withCORS(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimiter tracks per-IP request rates using a token bucket algorithm.
type rateLimiter struct {
	buckets sync.Map // IP string → *bucket
}

// bucket holds the token bucket state for a single IP address.
type bucket struct {
	mu     sync.Mutex
	tokens float64
	limit  float64
	last   time.Time
}

// allow checks whether a request from the given IP is permitted.
func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	// Refill tokens based on elapsed time (limit tokens per 60 seconds).
	b.tokens += elapsed * (b.limit / 60.0)
	if b.tokens > b.limit {
		b.tokens = b.limit
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// newRateLimiter creates a rate limiter.
func newRateLimiter() *rateLimiter {
	return &rateLimiter{}
}

// allow checks whether the IP is within its rate limit. The limit parameter
// is the maximum requests per minute.
func (rl *rateLimiter) allow(ip string, limit float64) bool {
	val, loaded := rl.buckets.LoadOrStore(ip, &bucket{
		tokens: limit,
		limit:  limit,
		last:   time.Now(),
	})
	b, ok := val.(*bucket)
	if !ok {
		return false
	}
	if !loaded {
		// Newly created bucket: consume one token for this request.
		b.mu.Lock()
		b.tokens--
		b.mu.Unlock()
		return true
	}
	return b.allow()
}

// withRateLimit applies per-IP rate limiting to API endpoints.
// Authenticated requests (valid op_session cookie) get authLimit req/min;
// unauthenticated requests get unauthLimit req/min.
func withRateLimit(next http.Handler, rl *rateLimiter, auth *authConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractIP(r)
		limit := float64(10) // unauthenticated: 10 req/min

		if auth != nil {
			cookie, err := r.Cookie("op_session")
			if err == nil && cookie.Value == auth.sessionToken {
				limit = 60 // authenticated: 60 req/min
			}
		} else {
			// No auth configured (localhost) — use the higher limit.
			limit = 60
		}

		if !rl.allow(ip, limit) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractIP returns the client IP from the request, stripping the port.
func extractIP(r *http.Request) string {
	addr := r.RemoteAddr
	// RemoteAddr is "IP:port"; strip the port.
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// authConfig holds the web authentication settings.
type authConfig struct {
	passwordHash string // bcrypt hash of the password
	sessionToken string // random token for the current server session
	isRemote     bool   // true when binding to non-localhost
}

// withAuth wraps a handler with cookie-based authentication. Only active
// when the server binds to a non-localhost address.
func withAuth(next http.Handler, cfg *authConfig) http.Handler {
	if cfg == nil || !cfg.isRemote {
		return next
	}

	loginRL := newRateLimiter()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Login endpoint is always accessible but rate-limited strictly.
		if r.URL.Path == "/api/login" {
			ip := extractIP(r)
			if !loginRL.allow(ip, 5) { // 5 attempts per minute per IP
				http.Error(w, "too many login attempts", http.StatusTooManyRequests)
				return
			}
			handleLogin(w, r, cfg)
			return
		}

		// Check session cookie.
		cookie, err := r.Cookie("op_session")
		if err != nil || cookie.Value != cfg.sessionToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleLogin validates the password and sets a session cookie.
func handleLogin(w http.ResponseWriter, r *http.Request, cfg *authConfig) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cfg.passwordHash), []byte(body.Password)); err != nil {
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "op_session",
		Value:    cfg.sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(24 * time.Hour / time.Second),
	})

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// GenerateSessionToken creates a random session token.
func GenerateSessionToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashPassword creates a bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// isLocalhost returns true if the host is a localhost address.
func isLocalhost(host string) bool {
	h := strings.Split(host, ":")[0]
	return h == "" || h == "127.0.0.1" || h == "localhost" || h == "::1"
}
