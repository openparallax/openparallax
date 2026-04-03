//go:build !darwin

// Package imessage provides a stub adapter on non-macOS platforms.
// iMessage requires macOS with Messages.app — this stub ensures the
// package compiles on all platforms.
package imessage

import (
	"context"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// Adapter is a stub on non-macOS platforms.
type Adapter struct{}

// New returns nil on non-macOS platforms — iMessage is not available.
func New(_ *types.IMessageConfig, _ *channels.Manager, _ *logging.Logger) *Adapter {
	return nil
}

// Name returns "imessage".
func (a *Adapter) Name() string { return "imessage" }

// IsConfigured returns false on non-macOS platforms.
func (a *Adapter) IsConfigured() bool { return false }

// Start is a no-op on non-macOS platforms.
func (a *Adapter) Start(_ context.Context) error { return nil }

// Stop is a no-op on non-macOS platforms.
func (a *Adapter) Stop() error { return nil }

// SendMessage is a no-op on non-macOS platforms.
func (a *Adapter) SendMessage(_ string, _ *channels.ChannelMessage) error { return nil }
