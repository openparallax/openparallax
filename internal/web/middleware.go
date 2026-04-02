package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// withCORS wraps a handler with permissive CORS headers for localhost development.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Login endpoint is always accessible.
		if r.URL.Path == "/api/login" {
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
