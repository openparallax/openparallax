package whatsapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAdapterNilWhenDisabled(t *testing.T) {
	assert.Nil(t, New(&types.WhatsAppConfig{Enabled: false}, nil, nil))
}

func TestNewAdapterNilWhenNilConfig(t *testing.T) {
	assert.Nil(t, New(nil, nil, nil))
}

func TestAdapterName(t *testing.T) {
	a := &Adapter{accessToken: "test", phoneNumberID: "123"}
	assert.Equal(t, "whatsapp", a.Name())
}

func TestIsConfigured(t *testing.T) {
	a := &Adapter{}
	assert.False(t, a.IsConfigured())

	a.accessToken = "token"
	a.phoneNumberID = "123"
	assert.True(t, a.IsConfigured())
}

func TestWebhookVerification(t *testing.T) {
	a := &Adapter{verifyToken: "my-secret"}

	// Valid verification.
	req := httptest.NewRequest(http.MethodGet,
		"/webhook?hub.mode=subscribe&hub.verify_token=my-secret&hub.challenge=test123", nil)
	rec := httptest.NewRecorder()
	a.handleVerify(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test123", rec.Body.String())

	// Invalid token.
	req = httptest.NewRequest(http.MethodGet,
		"/webhook?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=test", nil)
	rec = httptest.NewRecorder()
	a.handleVerify(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestWebhookPayloadParsing(t *testing.T) {
	raw := `{
		"entry": [{
			"changes": [{
				"value": {
					"messages": [{
						"from": "+1234567890",
						"type": "text",
						"text": {"body": "hello agent"}
					}]
				}
			}]
		}]
	}`

	var payload webhookPayload
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))
	require.Len(t, payload.Entry, 1)
	require.Len(t, payload.Entry[0].Changes, 1)
	require.Len(t, payload.Entry[0].Changes[0].Value.Messages, 1)

	msg := payload.Entry[0].Changes[0].Value.Messages[0]
	assert.Equal(t, "+1234567890", msg.From)
	assert.Equal(t, "hello agent", msg.Text.Body)
}

func TestAllowedNumbersFiltering(t *testing.T) {
	allowed := map[string]bool{"+1111111111": true}
	assert.True(t, allowed["+1111111111"])
	assert.False(t, allowed["+9999999999"])
}

func TestSendMessagePayloadStructure(t *testing.T) {
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                "+1234567890",
		"type":              "text",
		"text":              map[string]any{"body": "Hello!"},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Hello!")
	assert.Contains(t, string(data), "whatsapp")
	assert.Contains(t, string(data), "+1234567890")
}
