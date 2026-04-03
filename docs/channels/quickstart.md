# Channels Quick Start

Connect your AI agent to a messaging platform in minutes.

## Go: Telegram Bot

This example sets up a Telegram adapter that routes messages to the OpenParallax engine.

### 1. Configure

Add to your `config.yaml`:

```yaml
channels:
  telegram:
    enabled: true
    token_env: TELEGRAM_BOT_TOKEN
    allowed_users: [123456789]  # Your Telegram user ID
```

Set the environment variable:

```bash
export TELEGRAM_BOT_TOKEN="1234567890:ABCDefGHIJKlmnOPQRSTuvwxyz"
```

### 2. Register the Adapter

```go
package main

import (
    "context"

    "github.com/openparallax/openparallax/internal/channels"
    "github.com/openparallax/openparallax/internal/channels/telegram"
    "github.com/openparallax/openparallax/internal/engine"
    "github.com/openparallax/openparallax/internal/logging"
    "github.com/openparallax/openparallax/internal/types"
)

func main() {
    // Assume engine and logger are initialized.
    var eng *engine.Engine
    var log *logging.Logger

    // Create the channel manager.
    manager := channels.NewManager(eng, log)

    // Create and register the Telegram adapter.
    cfg := &types.TelegramConfig{
        Enabled:  true,
        TokenEnv: "TELEGRAM_BOT_TOKEN",
        AllowedUsers: []int64{123456789},
    }
    adapter := telegram.New(cfg, manager, log)
    if adapter != nil {
        manager.Register(adapter)
    }

    // Start all registered adapters.
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    manager.StartAll(ctx)

    // Block until shutdown.
    <-ctx.Done()
    manager.StopAll()
}
```

### 3. Handle Messages

The Manager automatically routes incoming messages through the engine pipeline and sends responses back. You do not need to write any message handling code.

When a user sends a message on Telegram:

1. The adapter receives it via long-polling
2. The Manager maps the chat to a session (creating one if needed)
3. The Manager calls `engine.ProcessMessageForWeb()` with a response collector
4. The collector aggregates streaming LLM tokens into a complete response
5. The response is sent back to the Telegram chat via the Bot API

## Go: Custom Adapter

To connect a new messaging platform, implement the `ChannelAdapter` interface:

```go
package mychannel

import (
    "context"

    "github.com/openparallax/openparallax/internal/channels"
)

type MyAdapter struct {
    manager *channels.Manager
}

func (a *MyAdapter) Name() string { return "mychannel" }

func (a *MyAdapter) IsConfigured() bool { return true }

func (a *MyAdapter) Start(ctx context.Context) error {
    // Listen for incoming messages from your platform.
    // When you receive a message:
    //   response, err := a.manager.HandleMessage(ctx, "mychannel", chatID, text, mode)
    //   if err == nil && response != "" {
    //       a.SendMessage(chatID, &channels.ChannelMessage{Text: response})
    //   }
    <-ctx.Done()
    return nil
}

func (a *MyAdapter) Stop() error {
    return nil
}

func (a *MyAdapter) SendMessage(chatID string, msg *channels.ChannelMessage) error {
    // Send the message to your platform.
    // Use channels.SplitMessage(msg.Text, maxLen) for long messages.
    parts := channels.SplitMessage(msg.Text, 4096)
    for _, part := range parts {
        _ = part // send to your platform
    }
    return nil
}
```

Register your adapter with the Manager:

```go
manager.Register(&MyAdapter{manager: manager})
```

## Multiple Platforms

You can register multiple adapters simultaneously. Each runs in its own goroutine:

```go
manager := channels.NewManager(eng, log)

if tgAdapter := telegram.New(tgCfg, manager, log); tgAdapter != nil {
    manager.Register(tgAdapter)
}
if waAdapter := whatsapp.New(waCfg, manager, log); waAdapter != nil {
    manager.Register(waAdapter)
}
if dcAdapter := discord.New(dcCfg, manager, log); dcAdapter != nil {
    manager.Register(dcAdapter)
}

manager.StartAll(ctx)
// All three adapters are now running concurrently.
```

Each platform gets independent session management. A Telegram user and a WhatsApp user talking to the same agent have separate sessions.
