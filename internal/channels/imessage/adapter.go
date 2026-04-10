//go:build darwin

// Package imessage implements the iMessage channel adapter for macOS.
// It uses AppleScript to communicate with Messages.app for sending and
// receiving iMessages. Requires macOS with a GUI session and Full Disk Access.
package imessage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

const (
	pollInterval = 2 * time.Second
	maxMsgLen    = 4096
)

// Adapter implements channels.ChannelAdapter and channels.ApprovalHandler
// for iMessage on macOS.
type Adapter struct {
	appleID       string
	manager       *channels.Manager
	log           *logging.Logger
	lastCheck     time.Time
	stopCh        chan struct{}
	activeSenders   map[string]bool
	activeSendersMu sync.Mutex
}

// New creates an iMessage adapter from config. Returns nil if not enabled.
func New(cfg *types.IMessageConfig, manager *channels.Manager, log *logging.Logger) *Adapter {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if cfg.AppleID == "" {
		return nil
	}
	return &Adapter{
		appleID:       cfg.AppleID,
		manager:       manager,
		log:           log,
		lastCheck:     time.Now(),
		stopCh:        make(chan struct{}),
		activeSenders: make(map[string]bool),
	}
}

// Name returns "imessage".
func (a *Adapter) Name() string { return "imessage" }

// IsConfigured returns true if the adapter has a valid Apple ID.
func (a *Adapter) IsConfigured() bool { return a.appleID != "" }

// Start begins polling Messages.app for new messages. Blocks until ctx is canceled.
func (a *Adapter) Start(ctx context.Context) error {
	a.log.Info("imessage_started", "apple_id", a.appleID)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-a.stopCh:
			return nil
		case <-ticker.C:
			a.poll(ctx)
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

// SendMessage sends a message via iMessage.
func (a *Adapter) SendMessage(chatID string, msg *channels.ChannelMessage) error {
	parts := channels.SplitMessage(msg.Text, maxMsgLen)
	for _, part := range parts {
		if err := sendMessage(chatID, part); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) poll(ctx context.Context) {
	msgs, err := getNewMessages(a.lastCheck)
	if err != nil {
		a.log.Warn("imessage_poll_error", "error", err)
		return
	}
	a.lastCheck = time.Now()

	for _, msg := range msgs {
		a.activeSendersMu.Lock()
		a.activeSenders[msg.Sender] = true
		a.activeSendersMu.Unlock()

		if strings.HasPrefix(msg.Text, "/") {
			if response, _, handled := a.manager.HandleCommand("imessage", msg.Sender, msg.Text, "imessage"); handled {
				if response != "" {
					_ = a.SendMessage(msg.Sender, &channels.ChannelMessage{Text: response})
				}
				continue
			}
		}
		go func(sender, text string) {
			response, err := a.manager.HandleMessage(ctx, "imessage", sender, text, types.SessionNormal)
			if err != nil {
				a.log.Error("imessage_error", "sender", sender, "error", err)
				return
			}
			if response != "" {
				_ = a.SendMessage(sender, &channels.ChannelMessage{Text: response})
			}
		}(msg.Sender, msg.Text)
	}
}

// RequestApproval sends a Tier 3 approval prompt via iMessage. iMessage does
// not support inline buttons, so the message includes text instructions.
func (a *Adapter) RequestApproval(actionID, toolName, reasoning string, timeoutSecs int) error {
	a.activeSendersMu.Lock()
	senders := make([]string, 0, len(a.activeSenders))
	for s := range a.activeSenders {
		senders = append(senders, s)
	}
	a.activeSendersMu.Unlock()

	if len(senders) == 0 {
		return fmt.Errorf("no active senders to send approval request")
	}

	text := fmt.Sprintf("Shield needs your approval\n\nTool: %s\nReason: %s\nAuto-denies in %ds\n\nApprove or deny on the web UI, Telegram, or Discord.", toolName, reasoning, timeoutSecs)
	for _, sender := range senders {
		_ = a.SendMessage(sender, &channels.ChannelMessage{Text: text})
	}
	return nil
}

// Compile-time check that Adapter satisfies ApprovalHandler.
var _ channels.ApprovalHandler = (*Adapter)(nil)
