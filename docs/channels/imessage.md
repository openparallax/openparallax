# iMessage

Connect your AI agent to iMessage via AppleScript and Messages.app on macOS.

::: warning macOS Only
iMessage integration is available on macOS only. This is a platform constraint from Apple, not an OpenParallax limitation. Messages.app and its AppleScript bridge are not available on Linux or Windows.
:::

## How It Works

The iMessage adapter uses AppleScript to communicate with Messages.app:

1. **Polling**: The adapter periodically queries Messages.app for new incoming messages using AppleScript (`osascript`)
2. **Receiving**: New messages are normalized and routed through the Engine pipeline like any other channel
3. **Sending**: Responses are sent back via `osascript` commands that instruct Messages.app to deliver the message through iMessage

All messages go through the same Shield evaluation, audit logging, and memory persistence as every other channel.

## Prerequisites

- **macOS** with Messages.app configured and signed in with your Apple ID
- **Full Disk Access** granted to the OpenParallax process (System Settings > Privacy & Security > Full Disk Access)
- **GUI session** -- the adapter requires an active user session. It does not work on headless servers or over SSH without a GUI

## Configuration

Add to your `config.yaml`:

```yaml
channels:
  imessage:
    enabled: true
    apple_id: "you@icloud.com"  # Apple ID email used in Messages.app
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable the iMessage adapter |
| `apple_id` | string | `""` | The Apple ID email address configured in Messages.app. Used to identify the correct account when multiple accounts are signed in. |

## Setup Steps

1. Open Messages.app and sign in with your Apple ID (if not already signed in)
2. Grant Full Disk Access to the OpenParallax process:
   - Open **System Settings > Privacy & Security > Full Disk Access**
   - Add the `openparallax` binary or your terminal application
3. Add the iMessage configuration to your `config.yaml` with your Apple ID email
4. Restart the agent: `openparallax restart`

On first run, macOS may prompt you to allow OpenParallax to control Messages.app via AppleScript. Click **OK** to grant access.

## Message Types

The initial implementation supports **text messages only**. Attachments (images, files, audio) are not processed. If a message contains an attachment alongside text, only the text content is extracted and passed to the agent.

## Access Control

Unlike cloud-based channels, iMessage does not have a webhook or bot API with user IDs. The adapter processes messages from all conversations in Messages.app. To restrict access, configure your Messages.app contacts and iMessage settings at the macOS level.

## Limitations

- **macOS only** -- requires Messages.app, which is exclusive to macOS
- **GUI session required** -- the AppleScript bridge requires an active desktop session. It will not work on headless macOS servers, remote SSH sessions without a GUI, or CI environments
- **iCloud account required** -- Messages.app must be signed in with an Apple ID that has iMessage enabled
- **No read receipts** -- the adapter does not send or process read receipts
- **Text only** -- attachments are not supported in the initial implementation
- **Polling-based** -- new messages are detected by polling, not push notifications. There is a small delay between receiving a message and the agent processing it
- **Single account** -- the adapter works with one Apple ID at a time

## Troubleshooting

### "Not permitted to send Apple events"

macOS is blocking AppleScript access to Messages.app. Go to **System Settings > Privacy & Security > Automation** and ensure OpenParallax (or your terminal) has permission to control Messages.app.

### Messages not being received

1. Verify Messages.app is open and signed in
2. Check that Full Disk Access is granted
3. Ensure you are running in a GUI session (not headless)
4. Check the engine log for errors: `openparallax logs --event imessage`

### Agent responds but messages don't appear in iMessage

The Apple ID in `config.yaml` may not match the account configured in Messages.app. Open Messages.app > Settings > iMessage and verify the email address.
