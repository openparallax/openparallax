package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCanary = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"

func newTestManager(t *testing.T, tokenServer *httptest.Server) *Manager {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	providers := map[string]ProviderConfig{
		"google": {
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			TokenURL:     "",
		},
	}
	if tokenServer != nil {
		providers["google"] = ProviderConfig{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			TokenURL:     tokenServer.URL + "/token",
		}
	}

	mgr, err := NewManager(db, testCanary, providers, nil)
	require.NoError(t, err)
	return mgr
}

func TestStoreAndGetValidToken(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	tokens := &OAuthTokens{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		Expiry:       time.Now().Add(1 * time.Hour),
		Scopes:       []string{"email", "calendar"},
	}
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", tokens))

	got, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "access-123", got)
}

func TestGetValidTokenRefreshesExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "refreshed-token",
			"expires_in":   3600,
			"scope":        "email calendar",
		})
	}))
	defer server.Close()

	mgr := newTestManager(t, server)
	ctx := context.Background()

	// Store expired token.
	tokens := &OAuthTokens{
		AccessToken:  "old-token",
		RefreshToken: "refresh-456",
		Expiry:       time.Now().Add(-1 * time.Hour), // expired
	}
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", tokens))

	got, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "refreshed-token", got)
}

func TestGetValidTokenProactiveRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "proactive-refresh",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	mgr := newTestManager(t, server)
	ctx := context.Background()

	// Store token expiring within refresh buffer (5 min).
	tokens := &OAuthTokens{
		AccessToken:  "almost-expired",
		RefreshToken: "refresh-456",
		Expiry:       time.Now().Add(2 * time.Minute), // within 5-min buffer
	}
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", tokens))

	got, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "proactive-refresh", got)
}

func TestGetValidTokenRefreshFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	mgr := newTestManager(t, server)
	ctx := context.Background()

	tokens := &OAuthTokens{
		AccessToken:  "old-token",
		RefreshToken: "bad-refresh",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", tokens))

	_, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
	assert.ErrorIs(t, err, ErrTokenRevoked)

	// Tokens should be deleted.
	has, _ := mgr.HasTokens(ctx, "google", "user@gmail.com")
	assert.False(t, has)
}

func TestRefreshWithRotatedRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "rotated-refresh",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	mgr := newTestManager(t, server)
	ctx := context.Background()

	tokens := &OAuthTokens{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		Expiry:       time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", tokens))

	got, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "new-access", got)
}

func TestRevokeTokens(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	tokens := &OAuthTokens{
		AccessToken: "x", RefreshToken: "y",
		Expiry: time.Now().Add(1 * time.Hour),
	}
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", tokens))

	has, err := mgr.HasTokens(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.True(t, has)

	require.NoError(t, mgr.RevokeTokens(ctx, "google", "user@gmail.com"))

	has, err = mgr.HasTokens(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.False(t, has)
}

func TestHasTokens(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	has, err := mgr.HasTokens(ctx, "google", "nobody@gmail.com")
	require.NoError(t, err)
	assert.False(t, has)

	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", &OAuthTokens{
		AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))

	has, err = mgr.HasTokens(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.True(t, has)
}

func TestListAccounts(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	accounts, err := mgr.ListAccounts(ctx, "google")
	require.NoError(t, err)
	assert.Empty(t, accounts)

	require.NoError(t, mgr.StoreTokens(ctx, "google", "a@gmail.com", &OAuthTokens{
		AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))
	require.NoError(t, mgr.StoreTokens(ctx, "google", "b@gmail.com", &OAuthTokens{
		AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))

	accounts, err = mgr.ListAccounts(ctx, "google")
	require.NoError(t, err)
	assert.Len(t, accounts, 2)
	assert.Contains(t, accounts, "a@gmail.com")
	assert.Contains(t, accounts, "b@gmail.com")
}

func TestEncryptedStorageNotPlaintext(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	secret := "super-secret-access-token-value"
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", &OAuthTokens{
		AccessToken: secret, RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))

	// Read raw from DB — should NOT contain the plaintext.
	var accessEnc []byte
	err := mgr.db.Conn().QueryRowContext(ctx,
		`SELECT access_token_enc FROM oauth_tokens WHERE provider = ? AND account = ?`,
		"google", "user@gmail.com").Scan(&accessEnc)
	require.NoError(t, err)
	assert.NotContains(t, string(accessEnc), secret)
}

func TestConcurrentAccess(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", &OAuthTokens{
		AccessToken: "token", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
			assert.NoError(t, err)
			assert.Equal(t, "token", tok)
		}()
	}
	wg.Wait()
}

func TestStoreOverwrite(t *testing.T) {
	mgr := newTestManager(t, nil)
	ctx := context.Background()

	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", &OAuthTokens{
		AccessToken: "v1", RefreshToken: "r", Expiry: time.Now().Add(time.Hour),
	}))
	require.NoError(t, mgr.StoreTokens(ctx, "google", "user@gmail.com", &OAuthTokens{
		AccessToken: "v2", RefreshToken: "r2", Expiry: time.Now().Add(time.Hour),
	}))

	got, err := mgr.GetValidToken(ctx, "google", "user@gmail.com")
	require.NoError(t, err)
	assert.Equal(t, "v2", got)
}
