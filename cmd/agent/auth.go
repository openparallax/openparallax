package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"os"

	"github.com/openparallax/openparallax/internal/config"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/oauth"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var authAccount string

var authCmd = &cobra.Command{
	Use:   "auth <provider>",
	Short: "Authenticate with an OAuth2 provider (google, microsoft)",
	Long: `Run the OAuth2 authorization flow for a provider.
This opens your browser to authorize OpenParallax, then stores the tokens securely.

Supported providers: google, microsoft

Example:
  openparallax auth google --account user@gmail.com
  openparallax auth microsoft --account user@outlook.com`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runAuth,
}

func init() {
	authCmd.Flags().StringVar(&authAccount, "account", "", "email address for the OAuth account (required)")
	rootCmd.AddCommand(authCmd)
}

// providerOAuthConfig holds OAuth2 endpoints and default scopes per provider.
var providerOAuthConfig = map[string]struct {
	authURL  string
	tokenURL string
	scopes   []string
}{
	"google": {
		authURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL: "https://oauth2.googleapis.com/token",
		scopes:   []string{"https://mail.google.com/", "https://www.googleapis.com/auth/calendar"},
	},
	"microsoft": {
		authURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		tokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		scopes:   []string{"offline_access", "https://outlook.office365.com/IMAP.AccessAsUser.All", "https://outlook.office365.com/SMTP.Send", "https://graph.microsoft.com/Calendars.ReadWrite"},
	},
}

func runAuth(_ *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])
	if _, ok := providerOAuthConfig[provider]; !ok {
		return fmt.Errorf("unsupported provider %q: use 'google' or 'microsoft'", provider)
	}
	if authAccount == "" {
		return fmt.Errorf("--account flag is required (e.g. --account user@gmail.com)")
	}

	cfgPath := findConfig()
	if cfgPath == "" {
		return fmt.Errorf("workspace not found: run 'openparallax init' first")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Get OAuth client credentials from config.
	clientID, clientSecret, tenantID, err := getOAuthCredentials(cfg, provider)
	if err != nil {
		return err
	}

	provCfg := providerOAuthConfig[provider]

	// For Microsoft, use tenant-specific URLs if configured.
	tokenURL := provCfg.tokenURL
	authURL := provCfg.authURL
	if provider == "microsoft" && tenantID != "" && tenantID != "common" {
		tokenURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
		authURL = fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantID)
	}

	// Start local callback server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start callback server: %w", err)
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("failed to get callback port")
	}
	callbackPort := tcpAddr.Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", callbackPort)

	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes:      provCfg.scopes,
		RedirectURL: redirectURL,
	}

	state, _ := crypto.RandomHex(16)
	authorizationURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)

	fmt.Printf("\nOpen this URL in your browser to authorize OpenParallax:\n\n  %s\n\n", authorizationURL)
	openBrowser(authorizationURL)
	fmt.Println("Waiting for authorization...")

	// Wait for the callback.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("invalid state parameter")
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("authorization denied: %s", errMsg)
			_, _ = fmt.Fprintf(w, "<html><body><h2>Authorization denied.</h2><p>%s</p><p>You can close this tab.</p></body></html>", errMsg)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}
		codeCh <- code
		_, _ = fmt.Fprint(w, "<html><body><h2>Authorized!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
	})

	server := &http.Server{Handler: mux}
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case authErr := <-errCh:
		_ = server.Close()
		return authErr
	case <-time.After(5 * time.Minute):
		_ = server.Close()
		return fmt.Errorf("authorization timed out after 5 minutes")
	}

	// Shut down callback server.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)

	// Exchange code for tokens.
	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// Store tokens.
	dbPath := filepath.Join(cfg.Workspace, ".openparallax", "openparallax.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	canaryPath := filepath.Join(cfg.Workspace, ".openparallax", "canary.token")
	canaryHex := readCanaryFile(canaryPath)

	providers := buildProviderConfigs(cfg)
	mgr, err := oauth.NewManager(db, canaryHex, providers, nil)
	if err != nil {
		return fmt.Errorf("create OAuth manager: %w", err)
	}

	oauthTokens := &oauth.OAuthTokens{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	}
	if err := mgr.StoreTokens(ctx, provider, authAccount, oauthTokens); err != nil {
		return fmt.Errorf("store tokens: %w", err)
	}

	fmt.Printf("\n  ✓ Connected! Account %s authorized for %s.\n\n", authAccount, provider)
	return nil
}

func getOAuthCredentials(cfg *types.AgentConfig, provider string) (clientID, clientSecret, tenantID string, err error) {
	switch provider {
	case "google":
		if cfg.OAuth.Google == nil || cfg.OAuth.Google.ClientID == "" {
			return "", "", "", fmt.Errorf("google OAuth not configured: add oauth.google.client_id and oauth.google.client_secret to config.yaml")
		}
		return cfg.OAuth.Google.ClientID, cfg.OAuth.Google.ClientSecret, "", nil
	case "microsoft":
		if cfg.OAuth.Microsoft == nil || cfg.OAuth.Microsoft.ClientID == "" {
			return "", "", "", fmt.Errorf("microsoft OAuth not configured: add oauth.microsoft.client_id and oauth.microsoft.client_secret to config.yaml")
		}
		return cfg.OAuth.Microsoft.ClientID, cfg.OAuth.Microsoft.ClientSecret, cfg.OAuth.Microsoft.TenantID, nil
	default:
		return "", "", "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func readCanaryFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		c, _ := crypto.GenerateCanary()
		return c
	}
	return strings.TrimSpace(string(data))
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
	default:
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command(cmd, "url.dll,FileProtocolHandler", url).Start()
	} else {
		_ = exec.Command(cmd, url).Start()
	}
}

// buildProviderConfigs creates OAuth provider configs from AgentConfig.
func buildProviderConfigs(cfg *types.AgentConfig) map[string]oauth.ProviderConfig {
	providers := make(map[string]oauth.ProviderConfig)
	if cfg.OAuth.Google != nil && cfg.OAuth.Google.ClientID != "" {
		providers["google"] = oauth.ProviderConfig{
			ClientID:     cfg.OAuth.Google.ClientID,
			ClientSecret: cfg.OAuth.Google.ClientSecret,
		}
	}
	if cfg.OAuth.Microsoft != nil && cfg.OAuth.Microsoft.ClientID != "" {
		providers["microsoft"] = oauth.ProviderConfig{
			ClientID:     cfg.OAuth.Microsoft.ClientID,
			ClientSecret: cfg.OAuth.Microsoft.ClientSecret,
			TenantID:     cfg.OAuth.Microsoft.TenantID,
		}
	}
	return providers
}
