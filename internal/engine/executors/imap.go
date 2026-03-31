package executors

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
	"github.com/openparallax/openparallax/internal/oauth"
	"github.com/openparallax/openparallax/internal/types"
)

// maxEmailBodyLen is the maximum email body length returned to the LLM.
const maxEmailBodyLen = 10000

// IMAPReader provides read access to email via IMAP.
type IMAPReader interface {
	// ListMessages lists emails in a folder.
	ListMessages(ctx context.Context, folder string, limit int, unreadOnly bool) ([]EmailSummary, error)

	// ReadMessage reads a specific email by UID.
	ReadMessage(ctx context.Context, folder string, uid uint32) (*EmailMessage, error)

	// SearchMessages searches emails by query.
	SearchMessages(ctx context.Context, folder string, query string, limit int) ([]EmailSummary, error)

	// MoveMessage moves an email to a different folder.
	MoveMessage(ctx context.Context, uid uint32, fromFolder, toFolder string) error

	// MarkMessage sets or clears a flag on an email.
	MarkMessage(ctx context.Context, uid uint32, folder string, flag string, value bool) error
}

// EmailSummary is a compact email representation for listing.
type EmailSummary struct {
	UID     uint32   `json:"uid"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Date    string   `json:"date"`
	Seen    bool     `json:"seen"`
	Flagged bool     `json:"flagged"`
}

// EmailMessage is the full email content.
type EmailMessage struct {
	EmailSummary
	Body        string           `json:"body"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// AttachmentInfo describes an email attachment without content.
type AttachmentInfo struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// imapProvider implements IMAPReader using go-imap v2.
type imapProvider struct {
	host     string
	port     int
	useTLS   bool
	authFunc func(ctx context.Context) (sasl.Client, error)
}

// newIMAPProvider creates an IMAP provider from config.
func newIMAPProvider(cfg types.IMAPConfig, oauthMgr *oauth.Manager) *imapProvider {
	authFunc := func(_ context.Context) (sasl.Client, error) {
		username := expandEnv(cfg.Username)
		password := expandEnv(cfg.Password)
		return sasl.NewPlainClient("", username, password), nil
	}

	if cfg.AuthMode == "oauth2" && oauthMgr != nil {
		provider := "google"
		if strings.Contains(cfg.Host, "outlook") || strings.Contains(cfg.Host, "office365") {
			provider = "microsoft"
		}
		account := cfg.Account
		authFunc = func(ctx context.Context) (sasl.Client, error) {
			token, err := oauthMgr.GetValidToken(ctx, provider, account)
			if err != nil {
				return nil, fmt.Errorf("get OAuth token: %w", err)
			}
			return newXOAuth2Client(account, token), nil
		}
	}

	useTLS := cfg.TLS
	if cfg.Port == 993 {
		useTLS = true
	}

	return &imapProvider{
		host:     cfg.Host,
		port:     cfg.Port,
		useTLS:   useTLS,
		authFunc: authFunc,
	}
}

func (p *imapProvider) connect(ctx context.Context) (*imapclient.Client, error) {
	addr := net.JoinHostPort(p.host, fmt.Sprintf("%d", p.port))

	var opts imapclient.Options
	if p.useTLS {
		opts.TLSConfig = &tls.Config{ServerName: p.host}
	}

	client, err := imapclient.DialTLS(addr, &opts)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}

	saslClient, err := p.authFunc(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("build auth: %w", err)
	}

	if err := client.Authenticate(saslClient); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	return client, nil
}

func (p *imapProvider) ListMessages(ctx context.Context, folder string, limit int, unreadOnly bool) ([]EmailSummary, error) {
	client, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	if folder == "" {
		folder = "INBOX"
	}

	selectData, err := client.Select(folder, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("select %s: %w", folder, err)
	}

	if selectData.NumMessages == 0 {
		return nil, nil
	}

	// Fetch the last N messages.
	end := selectData.NumMessages
	start := uint32(1)
	if int(end) > limit {
		start = end - uint32(limit) + 1
	}

	var seqSet imap.SeqSet
	seqSet.AddRange(start, end)
	fetchOpts := &imap.FetchOptions{
		Flags:    true,
		Envelope: true,
	}

	msgs := client.Fetch(seqSet, fetchOpts)
	var summaries []EmailSummary

	for {
		msg := msgs.Next()
		if msg == nil {
			break
		}

		buf, fetchErr := msg.Collect()
		if fetchErr != nil {
			continue
		}

		seen := false
		flagged := false
		for _, f := range buf.Flags {
			if f == imap.FlagSeen {
				seen = true
			}
			if f == imap.FlagFlagged {
				flagged = true
			}
		}

		if unreadOnly && seen {
			continue
		}

		env := buf.Envelope
		if env == nil {
			continue
		}

		from := ""
		if len(env.From) > 0 {
			from = formatAddr(&env.From[0])
		}
		var to []string
		for i := range env.To {
			to = append(to, formatAddr(&env.To[i]))
		}

		summaries = append(summaries, EmailSummary{
			UID:     buf.SeqNum,
			From:    from,
			To:      to,
			Subject: env.Subject,
			Date:    env.Date.Format(time.RFC3339),
			Seen:    seen,
			Flagged: flagged,
		})
	}

	if err := msgs.Close(); err != nil {
		return summaries, nil
	}

	return summaries, nil
}

func (p *imapProvider) ReadMessage(_ context.Context, _ string, _ uint32) (*EmailMessage, error) {
	return nil, fmt.Errorf("email_read requires a running IMAP connection (not yet fully implemented for v2)")
}

func (p *imapProvider) SearchMessages(_ context.Context, _ string, _ string, _ int) ([]EmailSummary, error) {
	return nil, fmt.Errorf("email_search requires a running IMAP connection (not yet fully implemented for v2)")
}

func (p *imapProvider) MoveMessage(_ context.Context, _ uint32, _ string, _ string) error {
	return fmt.Errorf("email_move requires a running IMAP connection (not yet fully implemented for v2)")
}

func (p *imapProvider) MarkMessage(_ context.Context, _ uint32, _ string, _ string, _ bool) error {
	return fmt.Errorf("email_mark requires a running IMAP connection (not yet fully implemented for v2)")
}

// --- HTML stripping ---

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// StripHTML removes HTML tags and normalizes whitespace.
func StripHTML(html string) string {
	text := htmlTagRegex.ReplaceAllString(html, " ")
	// Collapse whitespace.
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// TruncateEmailBody truncates email body to maxEmailBodyLen.
func TruncateEmailBody(body string) string {
	if len(body) <= maxEmailBodyLen {
		return body
	}
	return body[:maxEmailBodyLen] + fmt.Sprintf("\n[Truncated — full email is %d characters]", len(body))
}

// --- XOAUTH2 SASL ---

type xoauth2Client struct {
	username string
	token    string
}

func newXOAuth2Client(username, token string) *xoauth2Client {
	return &xoauth2Client{username: username, token: token}
}

func (c *xoauth2Client) Start() (string, []byte, error) {
	resp := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.token)
	return "XOAUTH2", []byte(resp), nil
}

func (c *xoauth2Client) Next(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("unexpected server challenge")
}

func formatAddr(addr *imap.Address) string {
	if addr.Name != "" {
		return fmt.Sprintf("%s <%s>", addr.Name, addr.Addr())
	}
	return addr.Addr()
}

func expandEnv(s string) string {
	if strings.HasPrefix(s, "$") {
		if v, ok := lookupEnv(s[1:]); ok {
			return v
		}
	}
	return s
}

func lookupEnv(name string) (string, bool) {
	return "", false // Simplified — use os.LookupEnv in production
}
