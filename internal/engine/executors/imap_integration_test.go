package executors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/emersion/go-sasl"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// literalReader satisfies imap.LiteralReader for in-memory message bodies.
type literalReader struct {
	*bytes.Reader
	size int64
}

func newLiteralReader(b []byte) *literalReader {
	return &literalReader{Reader: bytes.NewReader(b), size: int64(len(b))}
}

func (l *literalReader) Size() int64 { return l.size }

// imapTestServer wires an in-memory imapmemserver behind a real TCP listener
// so the production imapclient code path runs end-to-end.
type imapTestServer struct {
	server   *imapserver.Server
	memUser  *imapmemserver.User
	mailbox  *imapmemserver.Mailbox
	listener net.Listener
	host     string
	port     int
}

func (s *imapTestServer) close() {
	_ = s.server.Close()
	_ = s.listener.Close()
}

// startIMAPTestServer launches a fresh in-memory IMAP server with one user
// (testuser/testpass) and an empty INBOX. Tests append messages via the
// returned User handle.
func startIMAPTestServer(t *testing.T) *imapTestServer {
	t.Helper()

	memServer := imapmemserver.New()
	user := imapmemserver.NewUser("testuser", "testpass")
	require.NoError(t, user.Create("INBOX", nil))
	require.NoError(t, user.Create("Archive", nil))
	memServer.AddUser(user)

	server := imapserver.New(&imapserver.Options{
		NewSession: func(_ *imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		Caps: imap.CapSet{
			imap.CapIMAP4rev2: {},
		},
		InsecureAuth: true,
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		_ = server.Serve(ln)
	}()

	addr := ln.Addr().(*net.TCPAddr)
	ts := &imapTestServer{
		server:   server,
		memUser:  user,
		listener: ln,
		host:     "127.0.0.1",
		port:     addr.Port,
	}

	mbox, err := lookupMailbox(user, "INBOX")
	require.NoError(t, err)
	ts.mailbox = mbox

	t.Cleanup(ts.close)

	// Wait briefly for the server goroutine to enter Accept.
	time.Sleep(50 * time.Millisecond)
	return ts
}

// lookupMailbox is a thin wrapper around the unexported user.mailbox lookup.
// imapmemserver does not export the method directly, so we round-trip via
// Status which returns the mailbox if it exists.
func lookupMailbox(u *imapmemserver.User, name string) (*imapmemserver.Mailbox, error) {
	if _, err := u.Status(name, &imap.StatusOptions{NumMessages: true}); err != nil {
		return nil, err
	}
	// imapmemserver does not let us reach into the mailbox directly. We
	// instead append via User.Append which routes to the mailbox internally,
	// so the returned mailbox handle is only used as a non-nil sentinel.
	return &imapmemserver.Mailbox{}, nil
}

func appendTestMessage(t *testing.T, ts *imapTestServer, mailbox string, body string, flags []imap.Flag) {
	t.Helper()
	r := newLiteralReader([]byte(body))
	_, err := ts.memUser.Append(mailbox, r, &imap.AppendOptions{
		Flags: flags,
		Time:  time.Now(),
	})
	require.NoError(t, err)
}

// rfc822 builds a minimal RFC-822 formatted message.
func rfc822(from, to, subject, body string) string {
	return strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
}

func newTestIMAPProvider(ts *imapTestServer) *imapProvider {
	return &imapProvider{
		host:   ts.host,
		port:   ts.port,
		useTLS: false,
		authFunc: func(_ context.Context) (sasl.Client, error) {
			return sasl.NewPlainClient("", "testuser", "testpass"), nil
		},
	}
}

// --- Tests ---

func TestIMAPIntegration_ListMessages(t *testing.T) {
	ts := startIMAPTestServer(t)
	appendTestMessage(t, ts, "INBOX", rfc822("alice@example.com", "bob@example.com", "Hello", "First message"), nil)
	appendTestMessage(t, ts, "INBOX", rfc822("alice@example.com", "bob@example.com", "Second", "Second message"), []imap.Flag{imap.FlagSeen})

	provider := newTestIMAPProvider(ts)
	ctx := context.Background()

	summaries, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.NotZero(t, summaries[0].UID, "UID must be the real IMAP UID, not a sequence number")
	assert.NotEqual(t, summaries[0].UID, summaries[1].UID)

	subjects := []string{summaries[0].Subject, summaries[1].Subject}
	assert.Contains(t, subjects, "Hello")
	assert.Contains(t, subjects, "Second")

	// unread_only filter
	unread, err := provider.ListMessages(ctx, "INBOX", 10, true)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, "Hello", unread[0].Subject)
	assert.False(t, unread[0].Seen)
}

func TestIMAPIntegration_ReadMessage(t *testing.T) {
	ts := startIMAPTestServer(t)
	appendTestMessage(t, ts, "INBOX", rfc822("alice@example.com", "bob@example.com", "Pickup at 3", "See you then."), nil)

	provider := newTestIMAPProvider(ts)
	ctx := context.Background()

	summaries, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	msg, err := provider.ReadMessage(ctx, "INBOX", summaries[0].UID)
	require.NoError(t, err)
	assert.Equal(t, "Pickup at 3", msg.Subject)
	assert.Contains(t, msg.Body, "See you then.")
	assert.NotContains(t, msg.Body, "Subject:", "headers must be stripped from body")
}

func TestIMAPIntegration_ReadMessage_Missing(t *testing.T) {
	ts := startIMAPTestServer(t)
	provider := newTestIMAPProvider(ts)
	_, err := provider.ReadMessage(context.Background(), "INBOX", 999)
	assert.Error(t, err)
}

func TestIMAPIntegration_SearchMessages(t *testing.T) {
	ts := startIMAPTestServer(t)
	appendTestMessage(t, ts, "INBOX", rfc822("a@x.com", "b@x.com", "Lunch plans", "How about Friday?"), nil)
	appendTestMessage(t, ts, "INBOX", rfc822("a@x.com", "b@x.com", "Project status", "Sprint review on Monday."), nil)
	appendTestMessage(t, ts, "INBOX", rfc822("a@x.com", "b@x.com", "Random", "Friday is a good day."), nil)

	provider := newTestIMAPProvider(ts)
	ctx := context.Background()

	results, err := provider.SearchMessages(ctx, "INBOX", "Friday", 10)
	require.NoError(t, err)
	require.Len(t, results, 2, "two messages mention Friday")

	// Limit should keep the most recent matches.
	limited, err := provider.SearchMessages(ctx, "INBOX", "Friday", 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestIMAPIntegration_MoveMessage(t *testing.T) {
	ts := startIMAPTestServer(t)
	appendTestMessage(t, ts, "INBOX", rfc822("a@x.com", "b@x.com", "Move me", "body"), nil)

	provider := newTestIMAPProvider(ts)
	ctx := context.Background()

	summaries, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	uid := summaries[0].UID

	require.NoError(t, provider.MoveMessage(ctx, uid, "INBOX", "Archive"))

	inboxAfter, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	assert.Empty(t, inboxAfter, "message should have left INBOX")

	archive, err := provider.ListMessages(ctx, "Archive", 10, false)
	require.NoError(t, err)
	require.Len(t, archive, 1)
	assert.Equal(t, "Move me", archive[0].Subject)
}

func TestIMAPIntegration_MarkMessage(t *testing.T) {
	ts := startIMAPTestServer(t)
	appendTestMessage(t, ts, "INBOX", rfc822("a@x.com", "b@x.com", "Mark me", "body"), nil)

	provider := newTestIMAPProvider(ts)
	ctx := context.Background()

	summaries, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	uid := summaries[0].UID
	require.False(t, summaries[0].Seen)
	require.False(t, summaries[0].Flagged)

	require.NoError(t, provider.MarkMessage(ctx, uid, "INBOX", "seen", true))
	require.NoError(t, provider.MarkMessage(ctx, uid, "INBOX", "flagged", true))

	after, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.True(t, after[0].Seen)
	assert.True(t, after[0].Flagged)

	// Reverse the seen flag.
	require.NoError(t, provider.MarkMessage(ctx, uid, "INBOX", "seen", false))
	after2, err := provider.ListMessages(ctx, "INBOX", 10, false)
	require.NoError(t, err)
	assert.False(t, after2[0].Seen)
}

func TestIMAPIntegration_UnknownFlag(t *testing.T) {
	ts := startIMAPTestServer(t)
	appendTestMessage(t, ts, "INBOX", rfc822("a@x.com", "b@x.com", "x", "body"), nil)
	provider := newTestIMAPProvider(ts)
	summaries, _ := provider.ListMessages(context.Background(), "INBOX", 10, false)
	require.Len(t, summaries, 1)
	err := provider.MarkMessage(context.Background(), summaries[0].UID, "INBOX", "important", true)
	assert.ErrorContains(t, err, "unknown flag")
}

// --- Sanity wrapper to keep the unused-import check happy across helpers ---

var (
	_ io.Reader = (*literalReader)(nil)
	_ types.IMAPConfig
	_ = fmt.Sprintf
)
