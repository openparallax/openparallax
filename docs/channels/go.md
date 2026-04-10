# Channels Go API

The channels package provides the adapter interface, message types, and channel manager for multi-platform messaging.

Package: `github.com/openparallax/openparallax/internal/channels`

## ChannelAdapter Interface

```go
type ChannelAdapter interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    SendMessage(chatID string, message *ChannelMessage) error
    IsConfigured() bool
}
```

### Name

```go
func (a *Adapter) Name() string
```

Returns the adapter identifier: `"telegram"`, `"whatsapp"`, `"discord"`, `"slack"`, `"signal"`, or `"teams"`.

### Start

```go
func (a *Adapter) Start(ctx context.Context) error
```

Begins listening for incoming messages. This method blocks until `ctx` is canceled or a fatal error occurs. On Telegram, this runs a long-polling loop. On WhatsApp, this starts a webhook HTTP server. On Discord, this opens a WebSocket connection to the Gateway.

### Stop

```go
func (a *Adapter) Stop() error
```

Gracefully shuts down the adapter. Closes connections, stops HTTP servers, and releases resources.

### SendMessage

```go
func (a *Adapter) SendMessage(chatID string, message *ChannelMessage) error
```

Sends a response back to the platform. The `chatID` is platform-specific: a phone number for WhatsApp/Signal, a chat ID for Telegram, a channel ID for Discord.

### IsConfigured

```go
func (a *Adapter) IsConfigured() bool
```

Returns `true` if the adapter has valid configuration (API tokens present, required fields set). The Manager only registers adapters that are configured.

## ChannelMessage

```go
type ChannelMessage struct {
    Text        string
    Attachments []ChannelAttachment
    ReplyToID   string
    Format      MessageFormat
}
```

| Field | Description |
|---|---|
| `Text` | The message text content |
| `Attachments` | Files or images to include |
| `ReplyToID` | Platform-specific message ID for threading |
| `Format` | How the text should be rendered: `FormatPlain`, `FormatMarkdown`, or `FormatHTML` |

## ChannelAttachment

```go
type ChannelAttachment struct {
    Filename string
    Path     string
    MimeType string
}
```

| Field | Description |
|---|---|
| `Filename` | Display name of the file |
| `Path` | Local filesystem path to the file |
| `MimeType` | MIME type of the file |

## MessageFormat

```go
const (
    FormatPlain    MessageFormat = iota  // Plain text
    FormatMarkdown                       // Markdown
    FormatHTML                           // HTML
)
```

## Manager

```go
type Manager struct { ... }
```

Manages adapter lifecycle and routes messages to the engine pipeline.

### NewManager

```go
func NewManager(eng *engine.Engine, log *logging.Logger) *Manager
```

Creates a channel manager connected to the engine.

### Register

```go
func (m *Manager) Register(adapter ChannelAdapter)
```

Adds an adapter to the manager. Only registers adapters where `IsConfigured()` returns `true`. Logs a warning for unconfigured adapters.

### StartAll

```go
func (m *Manager) StartAll(ctx context.Context)
```

Starts all registered adapters in separate goroutines. Each adapter runs with retry logic: up to 5 attempts with 30-second delays between retries. If an adapter fails all retry attempts, it is logged and abandoned (other adapters continue running).

### StopAll

```go
func (m *Manager) StopAll()
```

Gracefully stops all registered adapters.

### HandleMessage

```go
func (m *Manager) HandleMessage(ctx context.Context, adapterName, chatID, content string, mode types.SessionMode) (string, error)
```

Routes an incoming message to the engine pipeline and returns the full response text. This is the primary method called by adapters when they receive a message.

The method:

1. Gets or creates a session for the `adapterName:chatID` pair
2. Generates a unique message ID
3. Creates a `responseCollector` (an `EventSender` that aggregates streaming events)
4. Calls `engine.ProcessMessageForWeb()` with the collector
5. Returns the complete response text

### ResetSession

```go
func (m *Manager) ResetSession(adapterName, chatID string)
```

Creates a new session for a chat. Called when the user sends `/new` on any platform.

### AdapterCount

```go
func (m *Manager) AdapterCount() int
```

Returns the number of registered adapters.

## SplitMessage

```go
func SplitMessage(text string, maxLen int) []string
```

Splits a long message at natural boundaries (paragraph breaks, then word boundaries) to respect platform message length limits. If the text is within `maxLen`, it is returned as a single-element slice.

```go
parts := channels.SplitMessage(response, 2000) // Discord limit
for _, part := range parts {
    _ = adapter.SendMessage(chatID, &channels.ChannelMessage{Text: part})
}
```

## MaxMessageLen

```go
func MaxMessageLen(platform string) int
```

Returns the maximum message length for a platform:

| Platform | Max Length |
|---|---|
| `"telegram"` | 4,096 |
| `"whatsapp"` | 4,096 |
| `"discord"` | 2,000 |
| default | 4,096 |

## EventSender Implementation

The `responseCollector` is an internal `EventSender` that aggregates pipeline events into a response string:

```go
type responseCollector struct {
    mu       sync.Mutex
    text     strings.Builder
    response string
    done     bool
}

func (r *responseCollector) SendEvent(event *engine.PipelineEvent) error {
    switch event.Type {
    case engine.EventLLMToken:
        r.text.WriteString(event.LLMToken.Text)
    case engine.EventResponseComplete:
        r.response = event.ResponseComplete.Content
        r.done = true
    }
    return nil
}
```

The collector accumulates streaming text tokens and extracts the final response from the `response_complete` event. The `fullResponse()` method returns the `ResponseComplete` content if available, falling back to the accumulated text.
