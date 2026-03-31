package executors

import (
	"context"
	"errors"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

// mockMailProvider records sent emails for verification.
type mockMailProvider struct {
	sent []*Email
	err  error
}

func (m *mockMailProvider) Send(_ context.Context, msg *Email) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func newTestEmailExecutor(provider MailProvider) *EmailExecutor {
	return &EmailExecutor{provider: provider}
}

func TestEmailSendSingle(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"to": "user@example.com", "subject": "Hello", "body": "Hi there"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "user@example.com")
	assert.Len(t, mock.sent, 1)
	assert.Equal(t, []string{"user@example.com"}, mock.sent[0].To)
	assert.Equal(t, "Hello", mock.sent[0].Subject)
	assert.Equal(t, "Hi there", mock.sent[0].Body)
}

func TestEmailSendMultipleRecipients(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"to": "a@test.com, b@test.com, c@test.com", "subject": "Group", "body": "Hi all"},
	})

	assert.True(t, result.Success)
	assert.Len(t, mock.sent, 1)
	assert.Equal(t, []string{"a@test.com", "b@test.com", "c@test.com"}, mock.sent[0].To)
}

func TestEmailSendWithCC(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"to": "main@test.com", "cc": "cc1@test.com, cc2@test.com", "subject": "CC Test", "body": "body"},
	})

	assert.True(t, result.Success)
	assert.Equal(t, []string{"cc1@test.com", "cc2@test.com"}, mock.sent[0].CC)
}

func TestEmailSendWithReplyTo(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"to": "user@test.com", "subject": "Reply", "body": "body", "reply_to": "noreply@test.com"},
	})

	assert.True(t, result.Success)
	assert.Equal(t, "noreply@test.com", mock.sent[0].ReplyTo)
}

func TestEmailSendMissingTo(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"subject": "No recipient", "body": "body"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "to and subject are required")
}

func TestEmailSendMissingSubject(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"to": "user@test.com", "body": "body"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "to and subject are required")
}

func TestEmailSendProviderError(t *testing.T) {
	mock := &mockMailProvider{err: errors.New("SMTP connection refused")}
	e := newTestEmailExecutor(mock)

	result := e.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{"to": "user@test.com", "subject": "Test", "body": "body"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "SMTP connection refused")
}

func TestNewEmailExecutorNilWhenUnconfigured(t *testing.T) {
	exec := NewEmailExecutor(types.EmailConfig{}, nil)
	assert.Nil(t, exec)
}

func TestNewEmailExecutorNilWhenNoHost(t *testing.T) {
	exec := NewEmailExecutor(types.EmailConfig{SMTP: types.SMTPConfig{Port: 587}}, nil)
	assert.Nil(t, exec)
}

func TestEmailSupportedActions(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)
	assert.Equal(t, []types.ActionType{types.ActionSendEmail}, e.SupportedActions())
}

func TestEmailToolSchemas(t *testing.T) {
	mock := &mockMailProvider{}
	e := newTestEmailExecutor(mock)
	schemas := e.ToolSchemas()
	assert.Len(t, schemas, 1)
	assert.Equal(t, "send_email", schemas[0].Name)
}

func TestParseRecipients(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a@b.com", []string{"a@b.com"}},
		{"a@b.com, c@d.com", []string{"a@b.com", "c@d.com"}},
		{" a@b.com , c@d.com , ", []string{"a@b.com", "c@d.com"}},
		{"", nil},
	}

	for _, tt := range tests {
		result := parseRecipients(tt.input)
		assert.Equal(t, tt.expected, result, "input: %q", tt.input)
	}
}
