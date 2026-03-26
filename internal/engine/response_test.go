package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactPrivateKey(t *testing.T) {
	c := NewResponseChecker()
	text := "Here is the key: -----BEGIN RSA PRIVATE KEY----- and more"
	result := c.Redact(text)
	assert.Contains(t, result, "[REDACTED: private key detected]")
	assert.NotContains(t, result, "-----BEGIN RSA PRIVATE KEY-----")
}

func TestRedactAWSKey(t *testing.T) {
	c := NewResponseChecker()
	text := "The access key is AKIAIOSFODNN7EXAMPLE and the secret is..."
	result := c.Redact(text)
	assert.Contains(t, result, "AKIA[REDACTED]")
	assert.NotContains(t, result, "AKIAIOSFODNN7EXAMPLE")
}

func TestRedactGitHubToken(t *testing.T) {
	c := NewResponseChecker()
	text := "Use ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij as your token"
	result := c.Redact(text)
	assert.Contains(t, result, "ghp_[REDACTED]")
}

func TestRedactJWT(t *testing.T) {
	c := NewResponseChecker()
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	text := "Token: " + jwt
	result := c.Redact(text)
	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "dozjgNryP4J3jVmNHl0w5N")
}

func TestRedactSlackToken(t *testing.T) {
	c := NewResponseChecker()
	text := "Slack token: xoxb-123456789012-1234567890123-AbCdEfGhIjKlMnOpQrStUvWx"
	result := c.Redact(text)
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedactCleanTextUnchanged(t *testing.T) {
	c := NewResponseChecker()
	text := "This is a normal response with no secrets. The weather is nice today."
	result := c.Redact(text)
	assert.Equal(t, text, result)
}

func TestRedactConnectionString(t *testing.T) {
	c := NewResponseChecker()
	text := "Connect to postgres://admin:password123@db.example.com:5432/mydb"
	result := c.Redact(text)
	assert.Contains(t, result, "[REDACTED]")
	assert.NotContains(t, result, "password123")
}
