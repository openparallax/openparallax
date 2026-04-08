package executors

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/openparallax/openparallax/internal/oauth"
	"github.com/openparallax/openparallax/internal/types"
)

// MailProvider is the interface for sending emails. SMTP is the initial
// implementation. Gmail and Outlook OAuth2 providers implement this interface.
type MailProvider interface {
	Send(ctx context.Context, msg *Email) error
}

// Email represents an outgoing email message.
type Email struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	ReplyTo     string
	Attachments []string
}

// EmailExecutor sends and reads emails through configured providers.
type EmailExecutor struct {
	provider MailProvider // SMTP send (may be nil)
	reader   IMAPReader   // IMAP read (may be nil)
}

// NewEmailExecutor creates an email executor from config.
// Returns nil if neither SMTP nor IMAP is configured.
func NewEmailExecutor(cfg types.EmailConfig, oauthMgr *oauth.Manager) *EmailExecutor {
	var provider MailProvider
	if cfg.SMTP.Host != "" {
		provider = &smtpProvider{cfg: cfg.SMTP}
	}

	var reader IMAPReader
	if cfg.IMAP.Host != "" {
		reader = newIMAPProvider(cfg.IMAP, oauthMgr)
	}

	if provider == nil && reader == nil {
		return nil
	}
	return &EmailExecutor{provider: provider, reader: reader}
}

// WorkspaceScope reports that the email executor does not write to the filesystem.
func (e *EmailExecutor) WorkspaceScope() WorkspaceScope { return ScopeNoFilesystem }

// SupportedActions returns actions based on what is configured.
func (e *EmailExecutor) SupportedActions() []types.ActionType {
	var actions []types.ActionType
	if e.provider != nil {
		actions = append(actions, types.ActionSendEmail)
	}
	if e.reader != nil {
		actions = append(actions,
			types.ActionEmailList, types.ActionEmailRead,
			types.ActionEmailSearch, types.ActionEmailMove, types.ActionEmailMark)
	}
	return actions
}

func (e *EmailExecutor) ToolSchemas() []ToolSchema {
	var schemas []ToolSchema

	if e.provider != nil {
		schemas = append(schemas, ToolSchema{
			ActionType:  types.ActionSendEmail,
			Name:        "send_email",
			Description: "Send an email to one or more recipients.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"to":       map[string]any{"type": "string", "description": "Recipient email address(es), comma-separated for multiple."},
					"subject":  map[string]any{"type": "string", "description": "Email subject line."},
					"body":     map[string]any{"type": "string", "description": "Email body text."},
					"cc":       map[string]any{"type": "string", "description": "CC recipients, comma-separated."},
					"reply_to": map[string]any{"type": "string", "description": "Reply-to address."},
				},
				"required": []string{"to", "subject", "body"},
			},
		})
	}

	if e.reader != nil {
		schemas = append(schemas, imapToolSchemas()...)
	}

	return schemas
}

func imapToolSchemas() []ToolSchema {
	return []ToolSchema{
		{ActionType: types.ActionEmailList, Name: "email_list", Description: "List emails from a mailbox folder. Returns subject, from, date, and flags.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"folder": map[string]any{"type": "string", "description": "Mailbox folder. Default: INBOX"}, "limit": map[string]any{"type": "integer", "description": "Max emails to return. Default: 20, max: 50"}, "unread_only": map[string]any{"type": "boolean", "description": "Only return unread emails. Default: false"}}}},
		{ActionType: types.ActionEmailRead, Name: "email_read", Description: "Read the full content of an email by its UID. Returns headers, body (plain text), and attachment names.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"uid": map[string]any{"type": "integer", "description": "Email UID from email_list results."}, "folder": map[string]any{"type": "string", "description": "Mailbox folder. Default: INBOX"}}, "required": []string{"uid"}}},
		{ActionType: types.ActionEmailSearch, Name: "email_search", Description: "Search emails using IMAP search criteria.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "Search query. Searches subject, from, and body."}, "folder": map[string]any{"type": "string", "description": "Folder to search. Default: INBOX"}, "limit": map[string]any{"type": "integer", "description": "Max results. Default: 20"}}, "required": []string{"query"}}},
		{ActionType: types.ActionEmailMove, Name: "email_move", Description: "Move an email to a different folder (e.g., Archive, Trash).", Parameters: map[string]any{"type": "object", "properties": map[string]any{"uid": map[string]any{"type": "integer", "description": "Email UID."}, "from_folder": map[string]any{"type": "string", "description": "Current folder. Default: INBOX"}, "to_folder": map[string]any{"type": "string", "description": "Destination folder."}}, "required": []string{"uid", "to_folder"}}},
		{ActionType: types.ActionEmailMark, Name: "email_mark", Description: "Mark an email as read, unread, or flagged.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"uid": map[string]any{"type": "integer", "description": "Email UID."}, "folder": map[string]any{"type": "string", "description": "Folder. Default: INBOX"}, "action": map[string]any{"type": "string", "enum": []string{"read", "unread", "flag", "unflag"}, "description": "Action to perform."}}, "required": []string{"uid", "action"}}},
	}
}

func (e *EmailExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionSendEmail:
		return e.executeSend(ctx, action)
	case types.ActionEmailList:
		return e.executeList(ctx, action)
	case types.ActionEmailRead:
		return e.executeRead(ctx, action)
	case types.ActionEmailSearch:
		return e.executeSearch(ctx, action)
	case types.ActionEmailMove:
		return e.executeMove(ctx, action)
	case types.ActionEmailMark:
		return e.executeMark(ctx, action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown email action"}
	}
}

func (e *EmailExecutor) executeSend(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if e.provider == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "SMTP not configured", Summary: "email send failed"}
	}

	to, _ := action.Payload["to"].(string)
	subject, _ := action.Payload["subject"].(string)
	body, _ := action.Payload["body"].(string)

	if to == "" || subject == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "to and subject are required", Summary: "email send failed"}
	}

	recipients := parseRecipients(to)
	var cc []string
	if ccStr, ok := action.Payload["cc"].(string); ok && ccStr != "" {
		cc = parseRecipients(ccStr)
	}

	replyTo, _ := action.Payload["reply_to"].(string)

	msg := &Email{
		To:      recipients,
		CC:      cc,
		Subject: subject,
		Body:    body,
		ReplyTo: replyTo,
	}

	if err := e.provider.Send(ctx, msg); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "email send failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Email sent to %s: %s", to, subject),
		Summary: fmt.Sprintf("sent email to %s", to),
	}
}

func (e *EmailExecutor) executeList(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if e.reader == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "IMAP not configured"}
	}

	folder, _ := action.Payload["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}
	limit := 20
	if l, ok := action.Payload["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 50 {
			limit = 50
		}
	}
	unreadOnly, _ := action.Payload["unread_only"].(bool)

	messages, err := e.reader.ListMessages(ctx, folder, limit, unreadOnly)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "email list failed"}
	}

	if len(messages) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "No emails found.", Summary: "0 emails"}
	}

	var sb strings.Builder
	for _, m := range messages {
		flag := " "
		if !m.Seen {
			flag = "*"
		}
		fmt.Fprintf(&sb, "%s [%d] %s — %s (%s)\n", flag, m.UID, m.From, m.Subject, m.Date)
	}
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: sb.String(), Summary: fmt.Sprintf("%d emails in %s", len(messages), folder),
	}
}

func (e *EmailExecutor) executeRead(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if e.reader == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "IMAP not configured"}
	}

	uid, _ := action.Payload["uid"].(float64)
	if uid == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "uid is required"}
	}
	folder, _ := action.Payload["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}

	msg, err := e.reader.ReadMessage(ctx, folder, uint32(uid))
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "email read failed"}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "From: %s\n", msg.From)
	fmt.Fprintf(&sb, "To: %s\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(&sb, "Subject: %s\n", msg.Subject)
	fmt.Fprintf(&sb, "Date: %s\n\n", msg.Date)
	sb.WriteString(TruncateEmailBody(msg.Body))

	if len(msg.Attachments) > 0 {
		sb.WriteString("\n\nAttachments:\n")
		for _, a := range msg.Attachments {
			fmt.Fprintf(&sb, "  - %s (%s, %d bytes)\n", a.Filename, a.MimeType, a.Size)
		}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: sb.String(), Summary: fmt.Sprintf("read email: %s", msg.Subject),
	}
}

func (e *EmailExecutor) executeSearch(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if e.reader == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "IMAP not configured"}
	}

	query, _ := action.Payload["query"].(string)
	if query == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "query is required"}
	}
	folder, _ := action.Payload["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}
	limit := 20
	if l, ok := action.Payload["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	results, err := e.reader.SearchMessages(ctx, folder, query, limit)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "email search failed"}
	}

	if len(results) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "No matching emails found.", Summary: "0 results"}
	}

	var sb strings.Builder
	for _, m := range results {
		fmt.Fprintf(&sb, "[%d] %s — %s (%s)\n", m.UID, m.From, m.Subject, m.Date)
	}
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: sb.String(), Summary: fmt.Sprintf("%d results for %q", len(results), query),
	}
}

func (e *EmailExecutor) executeMove(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if e.reader == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "IMAP not configured"}
	}

	uid, _ := action.Payload["uid"].(float64)
	if uid == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "uid is required"}
	}
	fromFolder, _ := action.Payload["from_folder"].(string)
	if fromFolder == "" {
		fromFolder = "INBOX"
	}
	toFolder, _ := action.Payload["to_folder"].(string)
	if toFolder == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "to_folder is required"}
	}

	if err := e.reader.MoveMessage(ctx, uint32(uid), fromFolder, toFolder); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "email move failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Moved email %d from %s to %s", uint32(uid), fromFolder, toFolder),
		Summary: fmt.Sprintf("moved to %s", toFolder),
	}
}

func (e *EmailExecutor) executeMark(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	if e.reader == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "IMAP not configured"}
	}

	uid, _ := action.Payload["uid"].(float64)
	if uid == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "uid is required"}
	}
	folder, _ := action.Payload["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}
	markAction, _ := action.Payload["action"].(string)

	var flag string
	var value bool
	switch markAction {
	case "read":
		flag, value = "seen", true
	case "unread":
		flag, value = "seen", false
	case "flag":
		flag, value = "flagged", true
	case "unflag":
		flag, value = "flagged", false
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "action must be read, unread, flag, or unflag"}
	}

	if err := e.reader.MarkMessage(ctx, uint32(uid), folder, flag, value); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "email mark failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Marked email %d as %s", uint32(uid), markAction),
		Summary: fmt.Sprintf("marked %s", markAction),
	}
}

func parseRecipients(s string) []string {
	var out []string
	for _, r := range strings.Split(s, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

// --- SMTP provider ---

type smtpProvider struct {
	cfg types.SMTPConfig
}

func (p *smtpProvider) Send(_ context.Context, msg *Email) error {
	username := os.ExpandEnv(p.cfg.Username)
	password := os.ExpandEnv(p.cfg.Password)
	from := os.ExpandEnv(p.cfg.From)

	allRecipients := append([]string{}, msg.To...)
	allRecipients = append(allRecipients, msg.CC...)
	allRecipients = append(allRecipients, msg.BCC...)

	var body strings.Builder
	fmt.Fprintf(&body, "From: %s\r\n", from)
	fmt.Fprintf(&body, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.CC) > 0 {
		fmt.Fprintf(&body, "Cc: %s\r\n", strings.Join(msg.CC, ", "))
	}
	if msg.ReplyTo != "" {
		fmt.Fprintf(&body, "Reply-To: %s\r\n", msg.ReplyTo)
	}
	fmt.Fprintf(&body, "Subject: %s\r\n", msg.Subject)
	fmt.Fprintf(&body, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&body, "Content-Type: text/plain; charset=utf-8\r\n")
	fmt.Fprintf(&body, "\r\n%s", msg.Body)

	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	auth := smtp.PlainAuth("", username, password, p.cfg.Host)

	return smtp.SendMail(addr, auth, from, allRecipients, []byte(body.String()))
}
