package executors

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
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

	var (
		client *imapclient.Client
		err    error
	)
	if p.useTLS {
		opts := &imapclient.Options{TLSConfig: &tls.Config{ServerName: p.host}}
		client, err = imapclient.DialTLS(addr, opts)
	} else {
		// Plain dial — only used in test setups and against loopback
		// relays. Production configs should always set TLS=true or use
		// port 993, which the constructor force-enables TLS for.
		client, err = imapclient.DialInsecure(addr, nil)
	}
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

// summaryFromBuffer projects a fetched message buffer onto the compact
// EmailSummary the LLM consumes. Returns ok=false when the message lacks an
// envelope so the caller can skip it.
func summaryFromBuffer(buf *imapclient.FetchMessageBuffer) (EmailSummary, bool) {
	if buf == nil || buf.Envelope == nil {
		return EmailSummary{}, false
	}
	seen, flagged := false, false
	for _, f := range buf.Flags {
		switch f {
		case imap.FlagSeen:
			seen = true
		case imap.FlagFlagged:
			flagged = true
		}
	}
	from := ""
	if len(buf.Envelope.From) > 0 {
		from = formatAddr(&buf.Envelope.From[0])
	}
	var to []string
	for i := range buf.Envelope.To {
		to = append(to, formatAddr(&buf.Envelope.To[i]))
	}
	return EmailSummary{
		UID:     uint32(buf.UID),
		From:    from,
		To:      to,
		Subject: buf.Envelope.Subject,
		Date:    buf.Envelope.Date.Format(time.RFC3339),
		Seen:    seen,
		Flagged: flagged,
	}, true
}

func summaryFetchOptions() *imap.FetchOptions {
	return &imap.FetchOptions{
		Flags:    true,
		Envelope: true,
		UID:      true,
	}
}

func selectFolder(client *imapclient.Client, folder string) (*imap.SelectData, error) {
	if folder == "" {
		folder = "INBOX"
	}
	data, err := client.Select(folder, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("select %s: %w", folder, err)
	}
	return data, nil
}

func (p *imapProvider) ListMessages(ctx context.Context, folder string, limit int, unreadOnly bool) ([]EmailSummary, error) {
	client, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	selectData, err := selectFolder(client, folder)
	if err != nil {
		return nil, err
	}
	if selectData.NumMessages == 0 {
		return nil, nil
	}

	// Fetch the last N messages by sequence number, then read the real UID
	// out of each fetched buffer so the caller has a stable identifier
	// across subsequent operations.
	end := selectData.NumMessages
	start := uint32(1)
	if int(end) > limit {
		start = end - uint32(limit) + 1
	}
	var seqSet imap.SeqSet
	seqSet.AddRange(start, end)

	msgs := client.Fetch(seqSet, summaryFetchOptions())
	defer func() { _ = msgs.Close() }()

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
		summary, ok := summaryFromBuffer(buf)
		if !ok {
			continue
		}
		if unreadOnly && summary.Seen {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (p *imapProvider) ReadMessage(ctx context.Context, folder string, uid uint32) (*EmailMessage, error) {
	client, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	if _, err := selectFolder(client, folder); err != nil {
		return nil, err
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	fetchOpts := &imap.FetchOptions{
		Flags:         true,
		Envelope:      true,
		UID:           true,
		BodyStructure: &imap.FetchItemBodyStructure{Extended: true},
		BodySection:   []*imap.FetchItemBodySection{{Peek: true}},
	}

	msgs := client.Fetch(uidSet, fetchOpts)
	defer func() { _ = msgs.Close() }()

	msg := msgs.Next()
	if msg == nil {
		return nil, fmt.Errorf("no message with UID %d in %s", uid, folder)
	}
	buf, err := msg.Collect()
	if err != nil {
		return nil, fmt.Errorf("collect message %d: %w", uid, err)
	}
	summary, ok := summaryFromBuffer(buf)
	if !ok {
		return nil, fmt.Errorf("message %d has no envelope", uid)
	}

	body := ""
	if len(buf.BodySection) > 0 {
		body = extractTextBody(buf.BodySection[0].Bytes)
	}

	out := &EmailMessage{
		EmailSummary: summary,
		Body:         body,
		Attachments:  extractAttachments(buf.BodyStructure),
	}
	return out, nil
}

func (p *imapProvider) SearchMessages(ctx context.Context, folder, query string, limit int) ([]EmailSummary, error) {
	client, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	if _, err := selectFolder(client, folder); err != nil {
		return nil, err
	}

	criteria := &imap.SearchCriteria{
		Body: []string{query},
	}
	searchData, err := client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search %s for %q: %w", folder, query, err)
	}

	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}
	if limit > 0 && len(uids) > limit {
		// Keep the most recent matches.
		uids = uids[len(uids)-limit:]
	}

	var uidSet imap.UIDSet
	uidSet.AddNum(uids...)

	msgs := client.Fetch(uidSet, summaryFetchOptions())
	defer func() { _ = msgs.Close() }()

	var results []EmailSummary
	for {
		msg := msgs.Next()
		if msg == nil {
			break
		}
		buf, fetchErr := msg.Collect()
		if fetchErr != nil {
			continue
		}
		if summary, ok := summaryFromBuffer(buf); ok {
			results = append(results, summary)
		}
	}
	return results, nil
}

func (p *imapProvider) MoveMessage(ctx context.Context, uid uint32, fromFolder, toFolder string) error {
	if toFolder == "" {
		return fmt.Errorf("destination folder is required")
	}
	client, err := p.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if _, err := selectFolder(client, fromFolder); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	if _, err := client.Move(uidSet, toFolder).Wait(); err != nil {
		return fmt.Errorf("move %d from %s to %s: %w", uid, fromFolder, toFolder, err)
	}
	return nil
}

func (p *imapProvider) MarkMessage(ctx context.Context, uid uint32, folder, flag string, value bool) error {
	client, err := p.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if _, err := selectFolder(client, folder); err != nil {
		return err
	}

	var imapFlag imap.Flag
	switch flag {
	case "seen":
		imapFlag = imap.FlagSeen
	case "flagged":
		imapFlag = imap.FlagFlagged
	default:
		return fmt.Errorf("unknown flag %q", flag)
	}

	op := imap.StoreFlagsAdd
	if !value {
		op = imap.StoreFlagsDel
	}
	store := &imap.StoreFlags{
		Op:     op,
		Silent: true,
		Flags:  []imap.Flag{imapFlag},
	}
	uidSet := imap.UIDSetNum(imap.UID(uid))
	cmd := client.Store(uidSet, store, nil)
	if err := cmd.Close(); err != nil {
		return fmt.Errorf("store flag %s on %d: %w", flag, uid, err)
	}
	return nil
}

// extractTextBody pulls a plain-text body out of the raw RFC822 message bytes
// returned by FETCH BODY[]. The full MIME parser lives in
// internal/agent/compaction; this is a deliberately small inline parser that
// strips headers and decodes HTML to text only when no plain-text part is
// present.
func extractTextBody(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	// Strip headers — the body starts after the first blank line.
	if idx := strings.Index(s, "\r\n\r\n"); idx >= 0 {
		s = s[idx+4:]
	} else if idx := strings.Index(s, "\n\n"); idx >= 0 {
		s = s[idx+2:]
	}
	// If the body looks like HTML, fall back to the tag stripper.
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, ">") {
		return StripHTML(trimmed)
	}
	return trimmed
}

// extractAttachments walks the body structure and returns metadata for every
// non-inline part with a filename. The body bytes themselves are not pulled
// over the wire — the LLM only needs to know what is attached.
func extractAttachments(bs imap.BodyStructure) []AttachmentInfo {
	if bs == nil {
		return nil
	}
	var out []AttachmentInfo
	bs.Walk(func(_ []int, part imap.BodyStructure) bool {
		single, ok := part.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		filename := ""
		if single.Extended != nil && single.Extended.Disposition != nil {
			filename = single.Extended.Disposition.Params["filename"]
		}
		if filename == "" {
			return true
		}
		out = append(out, AttachmentInfo{
			Filename: filename,
			MimeType: single.Type + "/" + single.Subtype,
			Size:     int64(single.Size),
		})
		return true
	})
	return out
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

// expandEnv treats a leading "$" as an environment-variable reference and
// resolves it via os.LookupEnv. Strings without a "$" prefix are returned
// verbatim so a literal username/password in the config still works.
func expandEnv(s string) string {
	if strings.HasPrefix(s, "$") {
		if v, ok := os.LookupEnv(s[1:]); ok {
			return v
		}
	}
	return s
}
