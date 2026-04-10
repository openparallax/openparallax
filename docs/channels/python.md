# Channels Python API

::: warning Planned — Not Yet Implemented
The API described below is planned but not yet implemented. The documented interface shows the intended design.
:::

The Python wrapper provides native Python access to the channels module for building messaging platform integrations.

## Installation

```bash
pip install openparallax-channels
```

Requires Python 3.9+.

## ChannelAdapter

The base class for all platform adapters:

```python
from openparallax_channels import ChannelAdapter, ChannelMessage

class MyAdapter(ChannelAdapter):
    def name(self) -> str:
        return "mychannel"

    def is_configured(self) -> bool:
        return True

    async def start(self) -> None:
        """Listen for incoming messages. Blocks until cancelled."""
        pass

    async def stop(self) -> None:
        """Gracefully shut down."""
        pass

    async def send_message(self, chat_id: str, message: ChannelMessage) -> None:
        """Send a response back to the platform."""
        pass
```

## ChannelMessage

```python
from openparallax_channels import ChannelMessage, MessageFormat, Attachment

message = ChannelMessage(
    text="Hello from OpenParallax!",
    format=MessageFormat.MARKDOWN,
    attachments=[
        Attachment(filename="report.pdf", path="/tmp/report.pdf", mime_type="application/pdf"),
    ],
    reply_to_id="msg-123",
)
```

## Manager

```python
from openparallax_channels import Manager

manager = Manager(message_handler=my_handler)
manager.register(my_adapter)
await manager.start_all()
```

The `message_handler` is a callable that receives `(adapter_name, chat_id, content, mode)` and returns a response string:

```python
async def my_handler(adapter_name: str, chat_id: str, content: str, mode: str) -> str:
    # Process the message through your AI pipeline.
    return "Response from the agent."
```

## Built-in Adapters

### Telegram

```python
from openparallax_channels.telegram import TelegramAdapter

adapter = TelegramAdapter(
    token="1234567890:ABCDefGHIJKlmnOPQRSTuvwxyz",
    allowed_users=[123456789],
    polling_interval=1,
)
```

### WhatsApp

```python
from openparallax_channels.whatsapp import WhatsAppAdapter

adapter = WhatsAppAdapter(
    phone_number_id="1234567890",
    access_token="EAAx...",
    verify_token="my-verify-token",
    webhook_port=9443,
    allowed_numbers=["+1234567890"],
)
```

### Discord

```python
from openparallax_channels.discord import DiscordAdapter

adapter = DiscordAdapter(
    token="ODk...",
    allowed_channels=["channel-id-1"],
    respond_to_mentions=True,
)
```

## Utilities

### split_message

```python
from openparallax_channels import split_message

parts = split_message("Very long text...", max_len=2000)
for part in parts:
    await adapter.send_message(chat_id, ChannelMessage(text=part))
```

### max_message_len

```python
from openparallax_channels import max_message_len

limit = max_message_len("discord")  # 2000
limit = max_message_len("telegram") # 4096
```
