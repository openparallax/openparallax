//go:build darwin

// Package imessage implements the iMessage channel adapter for macOS.
// It uses AppleScript to communicate with Messages.app for sending and
// receiving iMessages. Requires macOS with a GUI session and Full Disk Access.
package imessage

import (
	"context"
	"time"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

const (
	pollInterval = 2 * time.Second
	maxMsgLen    = 4096
)

// Adapter implements channels.ChannelAdapter for iMessage on macOS.
type Adapter struct {
	appleID   string
	manager   *channels.Manager
	log       *logging.Logger
	lastCheck time.Time
	stopCh    chan struct{}
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
		appleID:   cfg.AppleID,
		manager:   manager,
		log:       log,
		lastCheck: time.Now(),
		stopCh:    make(chan struct{}),
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
		a.manager.HandleMessage(ctx, "imessage", msg.Sender, msg.Text, types.SessionNormal)
	}
}
