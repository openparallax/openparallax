// Package channels defines the ChannelAdapter interface shared by all messaging
// platform adapters (Telegram, WhatsApp, Discord, Signal).
package channels

import "context"

// ChannelAdapter receives messages from an external platform and routes them
// through the engine pipeline.
type ChannelAdapter interface {
	// Name returns the adapter name (e.g., "telegram", "whatsapp").
	Name() string

	// Start begins listening for incoming messages. Blocks until ctx is canceled.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the adapter.
	Stop() error

	// SendMessage sends a response back to the platform.
	SendMessage(chatID string, message *ChannelMessage) error

	// IsConfigured returns true if the adapter has valid configuration.
	IsConfigured() bool
}

// ChannelMessage is a message to send on a messaging platform.
type ChannelMessage struct {
	// Text is the message text content.
	Text string

	// Attachments are files or images to include.
	Attachments []ChannelAttachment

	// ReplyToID is the platform-specific message ID to reply to (for threading).
	ReplyToID string

	// Format specifies how the text should be rendered.
	Format MessageFormat
}

// ChannelAttachment is a file attached to a channel message.
type ChannelAttachment struct {
	// Filename is the display name of the file.
	Filename string

	// Path is the local filesystem path to the file.
	Path string

	// MimeType is the MIME type of the file.
	MimeType string
}

// MessageFormat specifies text rendering format.
type MessageFormat int

const (
	// FormatPlain sends plain text.
	FormatPlain MessageFormat = iota
	// FormatMarkdown sends Markdown-formatted text.
	FormatMarkdown
	// FormatHTML sends HTML-formatted text.
	FormatHTML
)

// MaxMessageLen returns the maximum message length for a platform.
func MaxMessageLen(platform string) int {
	switch platform {
	case "telegram":
		return 4096
	case "whatsapp":
		return 4096
	case "discord":
		return 2000
	default:
		return 4096
	}
}

// SplitMessage splits a long message at paragraph boundaries respecting maxLen.
func SplitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			parts = append(parts, text)
			break
		}

		// Find a good split point: newline, then space.
		splitAt := maxLen
		for i := maxLen - 1; i > maxLen/2; i-- {
			if text[i] == '\n' {
				splitAt = i + 1
				break
			}
		}
		if splitAt == maxLen {
			for i := maxLen - 1; i > maxLen/2; i-- {
				if text[i] == ' ' {
					splitAt = i + 1
					break
				}
			}
		}

		parts = append(parts, text[:splitAt])
		text = text[splitAt:]
	}
	return parts
}
