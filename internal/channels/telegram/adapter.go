// Package telegram implements the Telegram channel adapter using long-polling
// against the Bot API. No webhook server is needed.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

const (
	apiBase         = "https://api.telegram.org/bot"
	maxMsgLen       = 4096
	rateLimit       = 30 // messages per minute per user
	rateLimitWindow = 60 * time.Second
)

// Adapter implements channels.ChannelAdapter for Telegram.
type Adapter struct {
	token         string
	allowedUsers  map[int64]bool
	allowedGroups map[int64]bool
	privateOnly   bool
	pollInterval  time.Duration
	manager       *channels.Manager
	log           *logging.Logger
	client        *http.Client
	offset        int64
	rateLimits    sync.Map // userID → []time.Time
	stopCh        chan struct{}
}

// New creates a Telegram adapter from config.
func New(cfg *types.TelegramConfig, manager *channels.Manager, log *logging.Logger) *Adapter {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	token := channels.ResolveEnv(cfg.TokenEnv)
	if token == "" {
		return nil
	}

	allowed := make(map[int64]bool)
	for _, uid := range cfg.AllowedUsers {
		allowed[uid] = true
	}

	allowedGroups := make(map[int64]bool)
	for _, gid := range cfg.AllowedGroups {
		allowedGroups[gid] = true
	}

	// Default to private-only when PrivateOnly is unset (nil).
	privateOnly := true
	if cfg.PrivateOnly != nil {
		privateOnly = *cfg.PrivateOnly
	}

	if !privateOnly && len(allowedGroups) == 0 {
		log.Warn("channel_security_warning", "msg", "Telegram adapter has private_only disabled with no group restrictions — responding to all groups")
	}

	interval := time.Duration(cfg.PollingInterval) * time.Second
	if interval <= 0 {
		interval = time.Second
	}

	return &Adapter{
		token:         token,
		allowedUsers:  allowed,
		allowedGroups: allowedGroups,
		privateOnly:   privateOnly,
		pollInterval:  interval,
		manager:       manager,
		log:           log,
		client:        &http.Client{Timeout: 60 * time.Second},
		stopCh:        make(chan struct{}),
	}
}

// Name returns "telegram".
func (a *Adapter) Name() string { return "telegram" }

// IsConfigured returns true if the adapter has a valid token.
func (a *Adapter) IsConfigured() bool { return a.token != "" }

// Start begins long-polling for Telegram updates. Blocks until ctx is canceled.
func (a *Adapter) Start(ctx context.Context) error {
	a.log.Info("telegram_started")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-a.stopCh:
			return nil
		default:
		}

		updates, err := a.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			a.log.Warn("telegram_poll_error", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			a.offset = update.UpdateID + 1
			if update.Message == nil {
				continue
			}
			go a.handleUpdate(ctx, update)
		}
	}
}

// Stop signals the adapter to shut down.
func (a *Adapter) Stop() error {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
	return nil
}

// SendMessage sends a text message to a Telegram chat.
func (a *Adapter) SendMessage(chatID string, msg *channels.ChannelMessage) error {
	parts := channels.SplitMessage(msg.Text, maxMsgLen)
	for _, part := range parts {
		text := part
		parseMode := ""
		if msg.Format == channels.FormatMarkdown {
			text = EscapeMarkdownV2(part)
			parseMode = "MarkdownV2"
		}

		payload := map[string]any{
			"chat_id": chatID,
			"text":    text,
		}
		if parseMode != "" {
			payload["parse_mode"] = parseMode
		}
		if msg.ReplyToID != "" {
			payload["reply_to_message_id"] = msg.ReplyToID
		}

		if err := a.apiCall("sendMessage", payload); err != nil {
			return err
		}
	}

	// Send attachments.
	for _, att := range msg.Attachments {
		if err := a.sendDocument(chatID, att); err != nil {
			a.log.Warn("telegram_attachment_failed", "file", att.Filename, "error", err)
		}
	}
	return nil
}

func (a *Adapter) handleUpdate(ctx context.Context, update telegramUpdate) {
	msg := update.Message
	userID := msg.From.ID
	chatID := fmt.Sprintf("%d", msg.Chat.ID)

	// Chat type access control: restrict non-private chats.
	if msg.Chat.Type != "private" {
		if a.privateOnly {
			a.log.Info("telegram_group_rejected", "chat_id", chatID, "chat_type", msg.Chat.Type)
			return
		}
		if len(a.allowedGroups) > 0 && !a.allowedGroups[msg.Chat.ID] {
			a.log.Info("telegram_group_rejected", "chat_id", chatID, "chat_type", msg.Chat.Type)
			return
		}
	}

	// Access control.
	if len(a.allowedUsers) > 0 && !a.allowedUsers[userID] {
		a.log.Info("telegram_unauthorized", "user_id", userID)
		_ = a.apiCall("sendMessage", map[string]any{
			"chat_id": chatID,
			"text":    "This agent is private.",
		})
		return
	}

	// Rate limiting.
	if !a.checkRateLimit(userID) {
		_ = a.apiCall("sendMessage", map[string]any{
			"chat_id": chatID,
			"text":    "Rate limit exceeded. Please wait a moment.",
		})
		return
	}

	text := msg.Text
	if text == "" {
		return
	}

	// Handle slash commands via centralized registry.
	if strings.HasPrefix(text, "/") {
		if response, action, handled := a.manager.HandleCommand("telegram", chatID, text, "telegram"); handled {
			if response != "" {
				_ = a.apiCall("sendMessage", map[string]any{"chat_id": chatID, "text": response})
			}
			_ = action
			return
		}
	}

	// Route to engine.
	mode := types.SessionNormal
	response, err := a.manager.HandleMessage(ctx, "telegram", chatID, text, mode)
	if err != nil {
		a.log.Error("telegram_pipeline_error", "chat_id", chatID, "error", err)
		_ = a.apiCall("sendMessage", map[string]any{
			"chat_id": chatID,
			"text":    "An error occurred. Please try again.",
		})
		return
	}

	if response != "" {
		_ = a.SendMessage(chatID, &channels.ChannelMessage{
			Text:   response,
			Format: channels.FormatPlain,
		})
	}
}

func (a *Adapter) checkRateLimit(userID int64) bool {
	key := userID
	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)

	val, _ := a.rateLimits.Load(key)
	var times []time.Time
	if val != nil {
		if t, ok := val.([]time.Time); ok {
			times = t
		}
	}

	var recent []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= rateLimit {
		return false
	}
	recent = append(recent, now)
	a.rateLimits.Store(key, recent)
	return true
}

// --- Telegram Bot API ---

func (a *Adapter) getUpdates(ctx context.Context) ([]telegramUpdate, error) {
	url := fmt.Sprintf("%s%s/getUpdates?offset=%d&timeout=30", apiBase, a.token, a.offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OK     bool             `json:"ok"`
		Result []telegramUpdate `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse updates: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(body))
	}
	return result.Result, nil
}

func (a *Adapter) apiCall(method string, payload map[string]any) error {
	url := fmt.Sprintf("%s%s/%s", apiBase, a.token, method)
	data, _ := json.Marshal(payload)

	resp, err := a.client.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API %s: status %d", method, resp.StatusCode)
	}
	return nil
}

func (a *Adapter) sendDocument(chatID string, att channels.ChannelAttachment) error {
	// Send file reference via sendDocument API (text fallback since multipart
	// file upload requires more complex HTTP handling).
	return a.apiCall("sendDocument", map[string]any{
		"chat_id": chatID,
		"caption": fmt.Sprintf("Attachment: %s", att.Filename),
	})
}

// --- Telegram types ---

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID int64        `json:"message_id"`
	From      telegramUser `json:"from"`
	Chat      telegramChat `json:"chat"`
	Text      string       `json:"text"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// EscapeMarkdownV2 escapes special characters for Telegram MarkdownV2 format.
func EscapeMarkdownV2(text string) string {
	// Characters that must be escaped in MarkdownV2.
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, ch := range special {
		text = strings.ReplaceAll(text, ch, "\\"+ch)
	}
	return text
}
