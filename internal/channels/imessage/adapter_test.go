//go:build !darwin

package imessage

import (
	"context"
	"testing"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewReturnsNilOnNonDarwin(t *testing.T) {
	cfg := &types.IMessageConfig{Enabled: true, AppleID: "test@icloud.com"}

	adapter := New(cfg, nil, nil)

	assert.Nil(t, adapter, "New must return nil on non-darwin platforms")
}

func TestNewReturnsNilWithNilConfig(t *testing.T) {
	adapter := New(nil, nil, nil)

	assert.Nil(t, adapter, "New must return nil when config is nil")
}

func TestNewReturnsNilWithDisabledConfig(t *testing.T) {
	cfg := &types.IMessageConfig{Enabled: false}

	adapter := New(cfg, nil, nil)

	assert.Nil(t, adapter, "New must return nil when config is disabled")
}

func TestStubName(t *testing.T) {
	var a Adapter

	assert.Equal(t, "imessage", a.Name())
}

func TestStubIsConfiguredReturnsFalse(t *testing.T) {
	var a Adapter

	assert.False(t, a.IsConfigured(), "stub must report not configured")
}

func TestStubStartIsNoop(t *testing.T) {
	var a Adapter

	err := a.Start(context.Background())

	assert.NoError(t, err, "stub Start must succeed")
}

func TestStubStartRespectsContext(t *testing.T) {
	var a Adapter
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Start(ctx)

	assert.NoError(t, err, "stub Start must succeed with canceled context")
}

func TestStubStopIsNoop(t *testing.T) {
	var a Adapter

	err := a.Stop()

	assert.NoError(t, err, "stub Stop must succeed")
}

func TestStubSendMessageIsNoop(t *testing.T) {
	var a Adapter
	msg := &channels.ChannelMessage{Text: "hello"}

	err := a.SendMessage("+1234567890", msg)

	assert.NoError(t, err, "stub SendMessage must succeed")
}

func TestStubSendMessageWithNilMessage(t *testing.T) {
	var a Adapter

	err := a.SendMessage("chat-id", nil)

	assert.NoError(t, err, "stub SendMessage must succeed with nil message")
}

func TestStubImplementsChannelAdapter(t *testing.T) {
	var a Adapter
	var iface channels.ChannelAdapter = &a

	assert.Equal(t, "imessage", iface.Name())
	assert.False(t, iface.IsConfigured())
	assert.NoError(t, iface.Start(context.Background()))
	assert.NoError(t, iface.Stop())
	assert.NoError(t, iface.SendMessage("id", &channels.ChannelMessage{Text: "test"}))
}
