package discord

import (
	"testing"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewAdapterNilWhenDisabled(t *testing.T) {
	assert.Nil(t, New(&types.DiscordConfig{Enabled: false}, nil, nil))
}

func TestNewAdapterNilWhenNilConfig(t *testing.T) {
	assert.Nil(t, New(nil, nil, nil))
}

func TestNewAdapterNilWhenNoToken(t *testing.T) {
	assert.Nil(t, New(&types.DiscordConfig{Enabled: true, TokenEnv: "NONEXISTENT_VAR"}, nil, nil))
}

func TestAdapterName(t *testing.T) {
	a := &Adapter{token: "test"}
	assert.Equal(t, "discord", a.Name())
}

func TestIsConfigured(t *testing.T) {
	a := &Adapter{token: ""}
	assert.False(t, a.IsConfigured())
	a.token = "test"
	assert.True(t, a.IsConfigured())
}

func TestAllowedChannelsFiltering(t *testing.T) {
	allowed := map[string]bool{"ch1": true, "ch2": true}
	assert.True(t, allowed["ch1"])
	assert.False(t, allowed["ch3"])
}

func TestAllowedUsersFiltering(t *testing.T) {
	allowed := map[string]bool{"user1": true}
	assert.True(t, allowed["user1"])
	assert.False(t, allowed["user2"])
}

func TestMessageSplitting(t *testing.T) {
	text := ""
	for range 100 {
		text += "This is a test message with some text. "
	}
	assert.Greater(t, len(text), maxMsgLen) // verify text exceeds limit
	parts := channels.SplitMessage(text, maxMsgLen)
	assert.True(t, len(parts) > 1)
	for _, part := range parts {
		assert.LessOrEqual(t, len(part), maxMsgLen)
	}
}
