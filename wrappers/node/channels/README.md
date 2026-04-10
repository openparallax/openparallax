# @openparallax/channels

Multi-channel messaging adapters (WhatsApp, Telegram, Discord, Signal, iMessage)

TypeScript wrapper for the [OpenParallax](https://docs.openparallax.dev) channels module. Communicates with a pre-built Go binary over JSON-RPC (stdin/stdout).

## Installation

```bash
npm install @openparallax/channels
```

## Usage

```typescript
import { Channels } from "@openparallax/channels";

const ch = new Channels();

// Split a long message into platform-safe chunks
const parts = await ch.splitMessage(longText, 2000);

// Format a message (0=plain, 1=markdown, 2=HTML)
const msg = await ch.formatMessage("Hello!", 1);

ch.close();
```

## Passing Channel Credentials

Channel credentials (bot tokens, API keys, webhook URLs) are passed through OpenParallax's workspace `config.yaml`, not through this wrapper directly. The wrapper communicates with the Go channels-bridge binary, which reads credentials from the workspace config.

Configure channels in your workspace `config.yaml`:

```yaml
channels:
  telegram:
    enabled: true
    bot_token_env: TELEGRAM_BOT_TOKEN
  discord:
    enabled: true
    bot_token_env: DISCORD_BOT_TOKEN
  slack:
    enabled: true
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN
```

### Telegram Example

```bash
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF..."
```

```yaml
# config.yaml
channels:
  telegram:
    enabled: true
    bot_token_env: TELEGRAM_BOT_TOKEN
```

```typescript
import { Channels } from "@openparallax/channels";

const ch = new Channels();
const parts = await ch.splitMessage(responseText, 4096);
for (const part of parts) {
  await sendToTelegram(chatId, part);
}
ch.close();
```

## Documentation

See the [channels documentation](https://docs.openparallax.dev/channels/) for all supported platforms and configuration options.

## License

Apache License 2.0
