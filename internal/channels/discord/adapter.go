// Package discord implements the Discord channel adapter using discordgo
// for WebSocket gateway + REST API communication.
package discord

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

const maxMsgLen = 2000

// Adapter implements channels.ChannelAdapter for Discord.
type Adapter struct {
	token             string
	allowedChannels   map[string]bool
	allowedUsers      map[string]bool
	respondToMentions bool
	manager           *channels.Manager
	log               *logging.Logger
	session           *discordgo.Session
	botID             string
	ctx               context.Context
	cancel            context.CancelFunc
}

// New creates a Discord adapter from config.
func New(cfg *types.DiscordConfig, manager *channels.Manager, log *logging.Logger) *Adapter {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	token := channels.ResolveEnv(cfg.TokenEnv)
	if token == "" {
		return nil
	}

	allowedChannels := make(map[string]bool)
	for _, ch := range cfg.AllowedChannels {
		allowedChannels[ch] = true
	}
	allowedUsers := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowedUsers[u] = true
	}

	return &Adapter{
		token:             token,
		allowedChannels:   allowedChannels,
		allowedUsers:      allowedUsers,
		respondToMentions: cfg.RespondToMentions,
		manager:           manager,
		log:               log,
	}
}

// Name returns "discord".
func (a *Adapter) Name() string { return "discord" }

// IsConfigured returns true if the adapter has a valid token.
func (a *Adapter) IsConfigured() bool { return a.token != "" }

// Start connects to Discord and begins listening for messages.
func (a *Adapter) Start(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	dg, err := discordgo.New("Bot " + a.token)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}
	a.session = dg

	dg.AddHandler(a.handleMessage)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	if err := dg.Open(); err != nil {
		return fmt.Errorf("connect to discord: %w", err)
	}

	a.botID = dg.State.User.ID
	a.log.Info("discord_started", "bot_id", a.botID)

	<-ctx.Done()
	return nil
}

// Stop disconnects from Discord.
func (a *Adapter) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.session != nil {
		return a.session.Close()
	}
	return nil
}

// SendMessage sends a text message to a Discord channel.
func (a *Adapter) SendMessage(chatID string, msg *channels.ChannelMessage) error {
	if a.session == nil {
		return fmt.Errorf("discord session not connected")
	}

	parts := channels.SplitMessage(msg.Text, maxMsgLen)
	for _, part := range parts {
		// Wrap code blocks for Discord.
		text := part
		if msg.Format == channels.FormatMarkdown {
			text = part // Discord natively supports markdown.
		}
		_, err := a.session.ChannelMessageSend(chatID, text)
		if err != nil {
			return err
		}
	}

	for _, att := range msg.Attachments {
		f, err := os.Open(att.Path)
		if err != nil {
			a.log.Warn("discord_attachment_failed", "file", att.Filename, "error", err)
			continue
		}
		_, sendErr := a.session.ChannelFileSend(chatID, att.Filename, f)
		_ = f.Close()
		if sendErr != nil {
			a.log.Warn("discord_attachment_failed", "file", att.Filename, "error", sendErr)
		}
	}
	return nil
}

func (a *Adapter) handleMessage(_ *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore own messages.
	if m.Author.ID == a.botID {
		return
	}

	// Channel filtering.
	if len(a.allowedChannels) > 0 && !a.allowedChannels[m.ChannelID] {
		return
	}

	// User filtering.
	if len(a.allowedUsers) > 0 && !a.allowedUsers[m.Author.ID] {
		return
	}

	// Mention filtering: in guild channels, only respond if @mentioned.
	if a.respondToMentions && m.GuildID != "" {
		mentioned := false
		for _, user := range m.Mentions {
			if user.ID == a.botID {
				mentioned = true
				break
			}
		}
		if !mentioned {
			return
		}
	}

	// Strip the mention from the message.
	text := m.Content
	text = strings.ReplaceAll(text, "<@"+a.botID+">", "")
	text = strings.ReplaceAll(text, "<@!"+a.botID+">", "")
	text = strings.TrimSpace(text)

	if text == "" {
		return
	}

	chatID := m.ChannelID

	if strings.HasPrefix(text, "/") {
		if response, action, handled := a.manager.HandleCommand("discord", chatID, text, "discord"); handled {
			if response != "" {
				_, _ = a.session.ChannelMessageSend(chatID, response)
			}
			_ = action
			return
		}
	}

	mode := types.SessionNormal
	go func() {
		response, err := a.manager.HandleMessage(a.ctx, "discord", chatID, text, mode)
		if err != nil {
			a.log.Error("discord_error", "channel", chatID, "error", err)
			return
		}
		if response != "" {
			_ = a.SendMessage(chatID, &channels.ChannelMessage{
				Text:   response,
				Format: channels.FormatMarkdown,
			})
		}
	}()
}
