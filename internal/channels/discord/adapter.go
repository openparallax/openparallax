// Package discord implements the Discord channel adapter using discordgo
// for WebSocket gateway + REST API communication.
package discord

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

const maxMsgLen = 2000

// Adapter implements channels.ChannelAdapter and channels.ApprovalHandler
// for Discord.
type Adapter struct {
	token             string
	allowedGuilds     map[string]bool
	allowedChannels   map[string]bool
	allowedUsers      map[string]bool
	respondToMentions bool
	manager           *channels.Manager
	log               *logging.Logger
	session           *discordgo.Session
	botID             string
	ctx               context.Context
	cancel            context.CancelFunc
	activeChansMu     sync.Mutex
	activeChans       map[string]bool // channelID → true for channels that have sent messages
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

	allowedGuilds := make(map[string]bool)
	for _, g := range cfg.AllowedGuilds {
		allowedGuilds[g] = true
	}
	allowedChannels := make(map[string]bool)
	for _, ch := range cfg.AllowedChannels {
		allowedChannels[ch] = true
	}
	allowedUsers := make(map[string]bool)
	for _, u := range cfg.AllowedUsers {
		allowedUsers[u] = true
	}

	if len(allowedGuilds) == 0 && len(allowedChannels) == 0 {
		log.Warn("channel_security_warning", "msg", "Discord adapter has no guild or channel restrictions — responding only to DMs")
	}

	return &Adapter{
		token:             token,
		allowedGuilds:     allowedGuilds,
		allowedChannels:   allowedChannels,
		allowedUsers:      allowedUsers,
		respondToMentions: cfg.RespondToMentions,
		manager:           manager,
		log:               log,
		activeChans:       make(map[string]bool),
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
	dg.AddHandler(a.handleInteraction)
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

	// Guild access control: if no guild or channel allowlist is configured,
	// only respond to DMs (GuildID is empty for direct messages).
	if len(a.allowedGuilds) == 0 && len(a.allowedChannels) == 0 {
		if m.GuildID != "" {
			return
		}
	}

	// Guild filtering.
	if len(a.allowedGuilds) > 0 && m.GuildID != "" && !a.allowedGuilds[m.GuildID] {
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

	a.activeChansMu.Lock()
	a.activeChans[chatID] = true
	a.activeChansMu.Unlock()

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

// RequestApproval sends a Tier 3 approval prompt with Approve/Deny buttons.
func (a *Adapter) RequestApproval(actionID, toolName, reasoning string, timeoutSecs int) error {
	if a.session == nil {
		return fmt.Errorf("discord session not connected")
	}

	a.activeChansMu.Lock()
	chans := make([]string, 0, len(a.activeChans))
	for ch := range a.activeChans {
		chans = append(chans, ch)
	}
	a.activeChansMu.Unlock()

	if len(chans) == 0 {
		return fmt.Errorf("no active channels to send approval request")
	}

	content := fmt.Sprintf("**Shield needs your approval**\n\nTool: `%s`\nReason: %s\nAuto-denies in %ds", toolName, reasoning, timeoutSecs)
	for _, ch := range chans {
		_, err := a.session.ChannelMessageSendComplex(ch, &discordgo.MessageSend{
			Content: content,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Approve",
							Style:    discordgo.SuccessButton,
							CustomID: "tier3:approve:" + actionID,
						},
						discordgo.Button{
							Label:    "Deny",
							Style:    discordgo.DangerButton,
							CustomID: "tier3:deny:" + actionID,
						},
					},
				},
			},
		})
		if err != nil {
			a.log.Warn("tier3_discord_send_failed", "channel", ch, "error", err)
		}
	}
	return nil
}

func (a *Adapter) handleInteraction(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := i.MessageComponentData()
	parts := strings.SplitN(data.CustomID, ":", 3)
	if len(parts) != 3 || parts[0] != "tier3" {
		return
	}

	approved := parts[1] == "approve"
	actionID := parts[2]

	if err := a.manager.HandleApprovalResponse(actionID, approved); err != nil {
		a.log.Warn("tier3_discord_decide_failed", "action_id", actionID, "error", err)
		_ = a.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Approval failed: " + err.Error(), Flags: discordgo.MessageFlagsEphemeral},
		})
		return
	}

	decision := "Denied"
	if approved {
		decision = "Approved"
	}
	a.log.Info("tier3_discord_decided", "action_id", actionID, "decision", decision)

	user := "unknown"
	if i.Member != nil && i.Member.User != nil {
		user = i.Member.User.Username
	} else if i.User != nil {
		user = i.User.Username
	}

	_ = a.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    fmt.Sprintf("**Shield approval: %s** — %s by %s", actionID[:8], decision, user),
			Components: []discordgo.MessageComponent{},
		},
	})
}

// Compile-time check that Adapter satisfies ApprovalHandler.
var _ channels.ApprovalHandler = (*Adapter)(nil)
