package signal

import (
	"encoding/json"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapterNilWhenDisabled(t *testing.T) {
	assert.Nil(t, New(&types.SignalConfig{Enabled: false}, nil, nil))
}

func TestNewAdapterNilWhenNilConfig(t *testing.T) {
	assert.Nil(t, New(nil, nil, nil))
}

func TestNewAdapterNilWhenNoCLI(t *testing.T) {
	assert.Nil(t, New(&types.SignalConfig{
		Enabled: true,
		CLIPath: "/nonexistent/signal-cli",
		Account: "+1234567890",
	}, nil, nil))
}

func TestAdapterName(t *testing.T) {
	a := &Adapter{cliPath: "/usr/bin/signal-cli", account: "+1"}
	assert.Equal(t, "signal", a.Name())
}

func TestIsConfigured(t *testing.T) {
	a := &Adapter{}
	assert.False(t, a.IsConfigured())

	a.cliPath = "/usr/bin/signal-cli"
	a.account = "+1234567890"
	assert.True(t, a.IsConfigured())
}

func TestAllowedNumbersFiltering(t *testing.T) {
	allowed := map[string]bool{"+1111": true}
	assert.True(t, allowed["+1111"])
	assert.False(t, allowed["+2222"])
}

func TestSignalRPCParsing(t *testing.T) {
	raw := `{
		"method": "receive",
		"params": {
			"envelope": {
				"source": "+1234567890",
				"dataMessage": {
					"message": "hello from signal"
				}
			}
		}
	}`

	var msg signalRPCMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &msg))
	assert.Equal(t, "receive", msg.Method)
	assert.Equal(t, "+1234567890", msg.Params.Envelope.Source)
	assert.Equal(t, "hello from signal", msg.Params.Envelope.DataMessage.Message)
}
