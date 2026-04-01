// Package whatsapp implements the WhatsApp channel adapter using the
// WhatsApp Business Cloud API (Meta). Requires a Meta Business account,
// a public webhook URL, and approved message templates.
package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

const (
	graphAPIBase = "https://graph.facebook.com/v21.0"
	maxMsgLen    = 4096
)

// Adapter implements channels.ChannelAdapter for WhatsApp.
type Adapter struct {
	phoneNumberID  string
	accessToken    string
	verifyToken    string
	webhookPort    int
	allowedNumbers map[string]bool
	manager        *channels.Manager
	log            *logging.Logger
	client         *http.Client
	server         *http.Server
}

// New creates a WhatsApp adapter from config.
func New(cfg *types.WhatsAppConfig, manager *channels.Manager, log *logging.Logger) *Adapter {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	token := os.Getenv(cfg.AccessTokenEnv)
	if token == "" || cfg.PhoneNumberID == "" {
		return nil
	}

	allowed := make(map[string]bool)
	for _, num := range cfg.AllowedNumbers {
		allowed[num] = true
	}

	port := cfg.WebhookPort
	if port == 0 {
		port = 9443
	}

	return &Adapter{
		phoneNumberID:  cfg.PhoneNumberID,
		accessToken:    token,
		verifyToken:    cfg.VerifyToken,
		webhookPort:    port,
		allowedNumbers: allowed,
		manager:        manager,
		log:            log,
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns "whatsapp".
func (a *Adapter) Name() string { return "whatsapp" }

// IsConfigured returns true if the adapter has valid credentials.
func (a *Adapter) IsConfigured() bool { return a.accessToken != "" && a.phoneNumberID != "" }

// Start begins the webhook server for incoming messages.
func (a *Adapter) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /webhook", a.handleVerify)
	mux.HandleFunc("POST /webhook", a.handleWebhook(ctx))

	a.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.webhookPort),
		Handler: mux,
	}

	a.log.Info("whatsapp_started", "port", a.webhookPort)

	errCh := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// Stop shuts down the webhook server.
func (a *Adapter) Stop() error {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(ctx)
	}
	return nil
}

// SendMessage sends a text message via the WhatsApp Cloud API.
func (a *Adapter) SendMessage(chatID string, msg *channels.ChannelMessage) error {
	parts := channels.SplitMessage(msg.Text, maxMsgLen)
	for _, part := range parts {
		payload := map[string]any{
			"messaging_product": "whatsapp",
			"to":                chatID,
			"type":              "text",
			"text":              map[string]any{"body": part},
		}
		data, _ := json.Marshal(payload)
		url := fmt.Sprintf("%s/%s/messages", graphAPIBase, a.phoneNumberID)

		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(data)))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+a.accessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("WhatsApp API error: %d", resp.StatusCode)
		}
	}
	return nil
}

// handleVerify handles the webhook verification challenge from Meta.
func (a *Adapter) handleVerify(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == a.verifyToken {
		_, _ = fmt.Fprint(w, challenge)
		return
	}
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// handleWebhook processes incoming WhatsApp messages.
func (a *Adapter) handleWebhook(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		var payload webhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)

		for _, entry := range payload.Entry {
			for _, change := range entry.Changes {
				for _, msg := range change.Value.Messages {
					go a.processMessage(ctx, msg)
				}
			}
		}
	}
}

func (a *Adapter) processMessage(ctx context.Context, msg whatsappMessage) {
	from := msg.From
	if len(a.allowedNumbers) > 0 && !a.allowedNumbers[from] {
		a.log.Info("whatsapp_unauthorized", "from", from)
		return
	}

	text := msg.Text.Body
	if text == "" {
		return
	}

	mode := types.SessionNormal
	if text == "/new" {
		a.manager.ResetSession("whatsapp", from)
		_ = a.SendMessage(from, &channels.ChannelMessage{Text: "New session started."})
		return
	}
	if text == "/otr" {
		mode = types.SessionOTR
	}

	response, err := a.manager.HandleMessage(ctx, "whatsapp", from, text, mode)
	if err != nil {
		a.log.Error("whatsapp_error", "from", from, "error", err)
		return
	}
	if response != "" {
		_ = a.SendMessage(from, &channels.ChannelMessage{Text: response})
	}
}

// --- WhatsApp webhook types ---

type webhookPayload struct {
	Entry []webhookEntry `json:"entry"`
}

type webhookEntry struct {
	Changes []webhookChange `json:"changes"`
}

type webhookChange struct {
	Value webhookValue `json:"value"`
}

type webhookValue struct {
	Messages []whatsappMessage `json:"messages"`
}

type whatsappMessage struct {
	From string `json:"from"`
	Type string `json:"type"`
	Text struct {
		Body string `json:"body"`
	} `json:"text"`
}
