package executors

import (
	"context"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockIMAPReader struct {
	listFn   func(ctx context.Context, folder string, limit int, unreadOnly bool) ([]EmailSummary, error)
	readFn   func(ctx context.Context, folder string, uid uint32) (*EmailMessage, error)
	searchFn func(ctx context.Context, folder string, query string, limit int) ([]EmailSummary, error)
	moveFn   func(ctx context.Context, uid uint32, fromFolder, toFolder string) error
	markFn   func(ctx context.Context, uid uint32, folder string, flag string, value bool) error
}

func (m *mockIMAPReader) ListMessages(ctx context.Context, folder string, limit int, unreadOnly bool) ([]EmailSummary, error) {
	return m.listFn(ctx, folder, limit, unreadOnly)
}

func (m *mockIMAPReader) ReadMessage(ctx context.Context, folder string, uid uint32) (*EmailMessage, error) {
	return m.readFn(ctx, folder, uid)
}

func (m *mockIMAPReader) SearchMessages(ctx context.Context, folder string, query string, limit int) ([]EmailSummary, error) {
	return m.searchFn(ctx, folder, query, limit)
}

func (m *mockIMAPReader) MoveMessage(ctx context.Context, uid uint32, fromFolder, toFolder string) error {
	return m.moveFn(ctx, uid, fromFolder, toFolder)
}

func (m *mockIMAPReader) MarkMessage(ctx context.Context, uid uint32, folder string, flag string, value bool) error {
	return m.markFn(ctx, uid, folder, flag, value)
}

func newEmailExecutorWithMockReader(reader IMAPReader) *EmailExecutor {
	return &EmailExecutor{
		provider: &mockMailProvider{},
		reader:   reader,
	}
}

func TestEmailExecutorWithIMAPSupportsReadActions(t *testing.T) {
	exec := newEmailExecutorWithMockReader(&mockIMAPReader{})
	actions := exec.SupportedActions()

	actionSet := make(map[types.ActionType]bool)
	for _, a := range actions {
		actionSet[a] = true
	}
	assert.True(t, actionSet[types.ActionSendEmail])
	assert.True(t, actionSet[types.ActionEmailList])
	assert.True(t, actionSet[types.ActionEmailRead])
	assert.True(t, actionSet[types.ActionEmailSearch])
	assert.True(t, actionSet[types.ActionEmailMove])
	assert.True(t, actionSet[types.ActionEmailMark])
	assert.Len(t, actions, 6)
}

func TestEmailExecutorSMTPOnlyNoReadActions(t *testing.T) {
	exec := &EmailExecutor{provider: &mockMailProvider{}}
	actions := exec.SupportedActions()
	assert.Len(t, actions, 1)
	assert.Equal(t, types.ActionSendEmail, actions[0])
}

func TestEmailExecutorIMAPOnlyNoSendAction(t *testing.T) {
	exec := &EmailExecutor{reader: &mockIMAPReader{}}
	actions := exec.SupportedActions()
	assert.Len(t, actions, 5)
	for _, a := range actions {
		assert.NotEqual(t, types.ActionSendEmail, a)
	}
}

func TestEmailListExecution(t *testing.T) {
	reader := &mockIMAPReader{
		listFn: func(_ context.Context, folder string, limit int, _ bool) ([]EmailSummary, error) {
			assert.Equal(t, "INBOX", folder)
			assert.Equal(t, 20, limit)
			return []EmailSummary{
				{UID: 1, From: "alice@example.com", Subject: "Hello", Date: "2026-04-01T10:00:00Z", Seen: false},
				{UID: 2, From: "bob@example.com", Subject: "Meeting", Date: "2026-04-01T11:00:00Z", Seen: true},
			}, nil
		},
	}
	exec := newEmailExecutorWithMockReader(reader)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionEmailList, Payload: map[string]any{},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "alice@example.com")
	assert.Contains(t, result.Output, "Hello")
	assert.Contains(t, result.Output, "bob@example.com")
	assert.Contains(t, result.Summary, "2 emails")
}

func TestEmailReadExecution(t *testing.T) {
	reader := &mockIMAPReader{
		readFn: func(_ context.Context, _ string, uid uint32) (*EmailMessage, error) {
			assert.Equal(t, uint32(42), uid)
			return &EmailMessage{
				EmailSummary: EmailSummary{UID: 42, From: "alice@example.com", Subject: "Report", Date: "2026-04-01"},
				Body:         "Here is the quarterly report.",
				Attachments:  []AttachmentInfo{{Filename: "report.pdf", MimeType: "application/pdf", Size: 1024}},
			}, nil
		},
	}
	exec := newEmailExecutorWithMockReader(reader)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionEmailRead, Payload: map[string]any{"uid": float64(42)},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "quarterly report")
	assert.Contains(t, result.Output, "report.pdf")
}

func TestEmailSearchExecution(t *testing.T) {
	reader := &mockIMAPReader{
		searchFn: func(_ context.Context, _ string, query string, _ int) ([]EmailSummary, error) {
			assert.Equal(t, "quarterly", query)
			return []EmailSummary{{UID: 10, From: "alice@example.com", Subject: "Q2 Report"}}, nil
		},
	}
	exec := newEmailExecutorWithMockReader(reader)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionEmailSearch, Payload: map[string]any{"query": "quarterly"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Q2 Report")
}

func TestEmailMoveExecution(t *testing.T) {
	moved := false
	reader := &mockIMAPReader{
		moveFn: func(_ context.Context, uid uint32, from, to string) error {
			assert.Equal(t, uint32(5), uid)
			assert.Equal(t, "INBOX", from)
			assert.Equal(t, "Archive", to)
			moved = true
			return nil
		},
	}
	exec := newEmailExecutorWithMockReader(reader)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionEmailMove, Payload: map[string]any{"uid": float64(5), "to_folder": "Archive"},
	})
	require.True(t, result.Success)
	assert.True(t, moved)
	assert.Contains(t, result.Output, "Archive")
}

func TestEmailMarkExecution(t *testing.T) {
	reader := &mockIMAPReader{
		markFn: func(_ context.Context, uid uint32, _ string, flag string, value bool) error {
			assert.Equal(t, uint32(3), uid)
			assert.Equal(t, "seen", flag)
			assert.True(t, value)
			return nil
		},
	}
	exec := newEmailExecutorWithMockReader(reader)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionEmailMark, Payload: map[string]any{"uid": float64(3), "action": "read"},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "read")
}

func TestStripHTML(t *testing.T) {
	html := `<html><body><h1>Hello</h1><p>This is a <b>test</b> email.</p></body></html>`
	text := StripHTML(html)
	assert.Contains(t, text, "Hello")
	assert.Contains(t, text, "test")
	assert.NotContains(t, text, "<")
}

func TestTruncateEmailBody(t *testing.T) {
	short := "Hello world"
	assert.Equal(t, short, TruncateEmailBody(short))

	long := ""
	for i := range 20000 {
		_ = i
		long += "x"
	}
	truncated := TruncateEmailBody(long)
	assert.True(t, len(truncated) < len(long))
	assert.Contains(t, truncated, "[Truncated")
}

func TestNewEmailExecutorNilWhenNotConfigured(t *testing.T) {
	exec := NewEmailExecutor(types.EmailConfig{}, nil)
	assert.Nil(t, exec)
}

func TestNewEmailExecutorSMTPOnly(t *testing.T) {
	exec := NewEmailExecutor(types.EmailConfig{
		SMTP: types.SMTPConfig{Host: "smtp.example.com", Port: 587},
	}, nil)
	require.NotNil(t, exec)
	assert.NotNil(t, exec.provider)
	assert.Nil(t, exec.reader)
}

func TestToolSchemasReflectConfig(t *testing.T) {
	// SMTP only — should have send_email only.
	exec := &EmailExecutor{provider: &mockMailProvider{}}
	schemas := exec.ToolSchemas()
	assert.Len(t, schemas, 1)
	assert.Equal(t, "send_email", schemas[0].Name)

	// SMTP + IMAP — should have all 6.
	exec2 := &EmailExecutor{provider: &mockMailProvider{}, reader: &mockIMAPReader{}}
	schemas2 := exec2.ToolSchemas()
	assert.Len(t, schemas2, 6)
}
