# Channels Node.js API

The Node.js wrapper provides native JavaScript/TypeScript access to the channels module for building messaging platform integrations.

## Installation

```bash
npm install @openparallax/channels
```

Requires Node.js 18+.

## ChannelAdapter

The base interface for all platform adapters:

```typescript
import { ChannelAdapter, ChannelMessage } from '@openparallax/channels';

class MyAdapter implements ChannelAdapter {
  name(): string {
    return 'mychannel';
  }

  isConfigured(): boolean {
    return true;
  }

  async start(signal: AbortSignal): Promise<void> {
    // Listen for incoming messages. Return when signal is aborted.
  }

  async stop(): Promise<void> {
    // Gracefully shut down.
  }

  async sendMessage(chatId: string, message: ChannelMessage): Promise<void> {
    // Send a response back to the platform.
  }
}
```

## ChannelMessage

```typescript
import { ChannelMessage, MessageFormat } from '@openparallax/channels';

const message: ChannelMessage = {
  text: 'Hello from OpenParallax!',
  format: MessageFormat.Markdown,
  attachments: [
    { filename: 'report.pdf', path: '/tmp/report.pdf', mimeType: 'application/pdf' },
  ],
  replyToId: 'msg-123',
};
```

## Manager

```typescript
import { Manager } from '@openparallax/channels';

const manager = new Manager({
  messageHandler: async (adapterName, chatId, content, mode) => {
    // Process the message through your AI pipeline.
    return 'Response from the agent.';
  },
});

manager.register(new MyAdapter());
await manager.startAll();
```

## Built-in Adapters

### Telegram

```typescript
import { TelegramAdapter } from '@openparallax/channels/telegram';

const adapter = new TelegramAdapter({
  token: process.env.TELEGRAM_BOT_TOKEN!,
  allowedUsers: [123456789],
  pollingInterval: 1,
});
```

### WhatsApp

```typescript
import { WhatsAppAdapter } from '@openparallax/channels/whatsapp';

const adapter = new WhatsAppAdapter({
  phoneNumberId: '1234567890',
  accessToken: process.env.WHATSAPP_ACCESS_TOKEN!,
  verifyToken: 'my-verify-token',
  webhookPort: 9443,
  allowedNumbers: ['+1234567890'],
});
```

### Discord

```typescript
import { DiscordAdapter } from '@openparallax/channels/discord';

const adapter = new DiscordAdapter({
  token: process.env.DISCORD_BOT_TOKEN!,
  allowedChannels: ['channel-id-1'],
  respondToMentions: true,
});
```

## Utilities

### splitMessage

```typescript
import { splitMessage } from '@openparallax/channels';

const parts = splitMessage('Very long text...', 2000);
for (const part of parts) {
  await adapter.sendMessage(chatId, { text: part });
}
```

### maxMessageLen

```typescript
import { maxMessageLen } from '@openparallax/channels';

const limit = maxMessageLen('discord');  // 2000
const limit2 = maxMessageLen('telegram'); // 4096
```

## TypeScript Support

Full TypeScript declarations are included:

```typescript
import type {
  ChannelAdapter,
  ChannelMessage,
  ChannelAttachment,
  MessageFormat,
} from '@openparallax/channels';
```
