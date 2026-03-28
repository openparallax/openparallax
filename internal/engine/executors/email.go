package executors

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strings"

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

// EmailExecutor sends emails through a configured provider.
type EmailExecutor struct {
	provider MailProvider
}

// NewEmailExecutor creates an email executor from config.
// Returns nil if email is not configured.
func NewEmailExecutor(cfg types.EmailConfig) *EmailExecutor {
	if cfg.SMTP.Host == "" {
		return nil
	}
	return &EmailExecutor{
		provider: &smtpProvider{cfg: cfg.SMTP},
	}
}

func (e *EmailExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionSendEmail}
}

func (e *EmailExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionSendEmail,
			Name:        "send_email",
			Description: "Send an email to one or more recipients. Requires SMTP configuration.",
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
		},
	}
}

func (e *EmailExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
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
