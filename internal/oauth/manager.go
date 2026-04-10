// Package oauth manages OAuth2 token storage, retrieval, and auto-refresh
// for Google and Microsoft providers. Tokens are encrypted at rest using
// AES-256-GCM with a key derived from the workspace canary token.
package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/storage"
)

// ErrTokenRevoked indicates the refresh token has been revoked or expired.
var ErrTokenRevoked = errors.New("oauth token revoked: re-authorize in Settings")

// refreshBuffer is how far before expiry to proactively refresh.
const refreshBuffer = 5 * time.Minute

// OAuthTokens holds the token pair for an OAuth2 account.
type OAuthTokens struct {
	// AccessToken is the short-lived token used for API calls.
	AccessToken string
	// RefreshToken is the long-lived token used to obtain new access tokens.
	RefreshToken string
	// Expiry is when the access token expires.
	Expiry time.Time
	// Scopes are the granted OAuth2 scopes.
	Scopes []string
}

// ProviderConfig holds OAuth2 client credentials for a provider.
type ProviderConfig struct {
	// ClientID is the OAuth2 application client ID.
	ClientID string
	// ClientSecret is the OAuth2 application client secret.
	ClientSecret string
	// TenantID is the Microsoft Azure tenant ID (Microsoft only).
	TenantID string
	// TokenURL is the provider's token endpoint.
	TokenURL string
}

// Manager handles OAuth2 token lifecycle: storage, retrieval, and auto-refresh.
type Manager struct {
	db        *storage.DB
	encKey    []byte
	providers map[string]ProviderConfig
	mu        sync.Mutex
	log       *logging.Logger
	client    *http.Client
}

// NewManager creates a new OAuth2 token manager. The encryption key is derived
// from the canary token using HKDF-SHA256.
func NewManager(db *storage.DB, canaryHex string, providers map[string]ProviderConfig, log *logging.Logger) (*Manager, error) {
	key, err := crypto.DeriveKey(canaryHex, "openparallax-oauth-encryption")
	if err != nil {
		return nil, fmt.Errorf("derive encryption key: %w", err)
	}
	if log == nil {
		log = logging.Nop()
	}
	return &Manager{
		db:        db,
		encKey:    key,
		providers: providers,
		log:       log,
		client:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// GetValidToken returns a valid access token for the given provider and account.
// If the stored token is expired or about to expire, it is auto-refreshed using
// the refresh token. Returns ErrTokenRevoked if the refresh token is invalid.
func (m *Manager) GetValidToken(ctx context.Context, provider, account string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tokens, err := m.loadTokens(ctx, provider, account)
	if err != nil {
		return "", fmt.Errorf("load tokens: %w", err)
	}

	// Check if access token is still valid (with buffer).
	if time.Until(tokens.Expiry) > refreshBuffer {
		return tokens.AccessToken, nil
	}

	// Refresh the token.
	m.log.Info("oauth_refresh", "provider", provider, "account", account)
	refreshed, err := m.refreshToken(ctx, provider, account, tokens.RefreshToken)
	if err != nil {
		return "", err
	}

	if err := m.storeTokensLocked(ctx, provider, account, refreshed); err != nil {
		return "", fmt.Errorf("store refreshed tokens: %w", err)
	}

	return refreshed.AccessToken, nil
}

// StoreTokens encrypts and stores OAuth2 tokens for the given provider and account.
func (m *Manager) StoreTokens(ctx context.Context, provider, account string, tokens *OAuthTokens) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.storeTokensLocked(ctx, provider, account, tokens)
}

func (m *Manager) storeTokensLocked(ctx context.Context, provider, account string, tokens *OAuthTokens) error {
	accessEnc, err := crypto.Encrypt(m.encKey, []byte(tokens.AccessToken))
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	refreshEnc, err := crypto.Encrypt(m.encKey, []byte(tokens.RefreshToken))
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	scopes := strings.Join(tokens.Scopes, ",")
	expiry := tokens.Expiry.Format(time.RFC3339)

	_, err = m.db.Conn().ExecContext(ctx,
		`INSERT OR REPLACE INTO oauth_tokens (provider, account, access_token_enc, refresh_token_enc, expiry, scopes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		provider, account, accessEnc, refreshEnc, expiry, scopes)
	return err
}

// InvalidateAccessToken marks the stored access token as expired so the next
// GetValidToken call is forced to use the refresh token. The refresh token
// itself is left intact. Used by API clients that observe a 401 response and
// need to retry with a freshly minted access token.
func (m *Manager) InvalidateAccessToken(ctx context.Context, provider, account string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.db.Conn().ExecContext(ctx,
		`UPDATE oauth_tokens SET expiry = ?, updated_at = datetime('now') WHERE provider = ? AND account = ?`,
		time.Unix(0, 0).Format(time.RFC3339), provider, account)
	return err
}

// RevokeTokens deletes stored tokens for the given provider and account.
func (m *Manager) RevokeTokens(ctx context.Context, provider, account string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Conn().ExecContext(ctx,
		`DELETE FROM oauth_tokens WHERE provider = ? AND account = ?`,
		provider, account)
	return err
}

// HasTokens checks if tokens exist for the given provider and account.
func (m *Manager) HasTokens(ctx context.Context, provider, account string) (bool, error) {
	var count int
	err := m.db.Conn().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM oauth_tokens WHERE provider = ? AND account = ?`,
		provider, account).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListAccounts returns all accounts with stored tokens for a given provider.
func (m *Manager) ListAccounts(ctx context.Context, provider string) ([]string, error) {
	rows, err := m.db.Conn().QueryContext(ctx,
		`SELECT account FROM oauth_tokens WHERE provider = ?`, provider)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var accounts []string
	for rows.Next() {
		var account string
		if err := rows.Scan(&account); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (m *Manager) loadTokens(ctx context.Context, provider, account string) (*OAuthTokens, error) {
	var accessEnc, refreshEnc []byte
	var expiryStr, scopesStr string

	err := m.db.Conn().QueryRowContext(ctx,
		`SELECT access_token_enc, refresh_token_enc, expiry, scopes FROM oauth_tokens WHERE provider = ? AND account = ?`,
		provider, account).Scan(&accessEnc, &refreshEnc, &expiryStr, &scopesStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no tokens stored for %s/%s", provider, account)
		}
		return nil, err
	}

	accessToken, err := crypto.Decrypt(m.encKey, accessEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt access token: %w", err)
	}
	refreshToken, err := crypto.Decrypt(m.encKey, refreshEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return nil, fmt.Errorf("parse expiry: %w", err)
	}

	var scopes []string
	if scopesStr != "" {
		scopes = strings.Split(scopesStr, ",")
	}

	return &OAuthTokens{
		AccessToken:  string(accessToken),
		RefreshToken: string(refreshToken),
		Expiry:       expiry,
		Scopes:       scopes,
	}, nil
}

// refreshToken exchanges a refresh token for new access + refresh tokens.
func (m *Manager) refreshToken(ctx context.Context, provider, account, refreshTok string) (*OAuthTokens, error) {
	pcfg, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("no OAuth config for provider %q", provider)
	}

	tokenURL := pcfg.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL(provider, pcfg.TenantID)
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshTok},
		"client_id":     {pcfg.ClientID},
		"client_secret": {pcfg.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		m.log.Warn("oauth_refresh_failed", "provider", provider, "account", account,
			"status", resp.StatusCode, "body", truncate(string(body), 200))
		// Delete revoked tokens.
		_, _ = m.db.Conn().ExecContext(ctx,
			`DELETE FROM oauth_tokens WHERE provider = ? AND account = ?`,
			provider, account)
		return nil, ErrTokenRevoked
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	// Microsoft may rotate refresh tokens.
	newRefresh := refreshTok
	if tokenResp.RefreshToken != "" {
		newRefresh = tokenResp.RefreshToken
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	var scopes []string
	if tokenResp.Scope != "" {
		scopes = strings.Split(tokenResp.Scope, " ")
	}

	return &OAuthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: newRefresh,
		Expiry:       expiry,
		Scopes:       scopes,
	}, nil
}

func defaultTokenURL(provider, tenantID string) string {
	switch provider {
	case "google":
		return "https://oauth2.googleapis.com/token"
	case "microsoft":
		tenant := tenantID
		if tenant == "" {
			tenant = "common"
		}
		return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)
	default:
		return ""
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
