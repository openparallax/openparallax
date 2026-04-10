# Signal

The Signal adapter connects your agent to Signal using [signal-cli](https://github.com/AsamK/signal-cli) as the transport layer. Signal provides end-to-end encrypted messaging, meaning all messages between you and the agent are encrypted in transit and at rest on Signal's servers. The adapter communicates with signal-cli via its JSON-RPC interface for real-time message delivery, or via D-Bus when running in daemon mode.

## Prerequisites

1. **Linux or macOS** -- signal-cli runs on Linux and macOS (Windows support is experimental)
2. **Java 21+** -- signal-cli is a Java application
3. **signal-cli 0.13+** -- installed and configured with a registered phone number
4. **A phone number** -- either a dedicated number for the bot or a linked device

## Setup

### 1. Install signal-cli

**From release archive (recommended):**

```bash
# Download the latest release
SIGNAL_CLI_VERSION="0.13.12"
curl -L -o signal-cli.tar.gz \
  "https://github.com/AsamK/signal-cli/releases/download/v${SIGNAL_CLI_VERSION}/signal-cli-${SIGNAL_CLI_VERSION}.tar.gz"

# Extract
tar xf signal-cli.tar.gz
sudo mv signal-cli-${SIGNAL_CLI_VERSION} /opt/signal-cli
sudo ln -s /opt/signal-cli/bin/signal-cli /usr/local/bin/signal-cli

# Verify
signal-cli --version
```

**Via package manager:**

```bash
# Arch Linux (AUR)
yay -S signal-cli

# Homebrew (macOS)
brew install signal-cli
```

signal-cli requires Java 21 or later. On Debian/Ubuntu, install it with `apt install openjdk-21-jre`. On macOS, Homebrew handles the dependency automatically.

### 2. Register a Phone Number

You have two options: register a new number or link to an existing Signal account.

**Option A: Register a new number (recommended for bots)**

Use a dedicated phone number that can receive SMS or voice calls for verification:

```bash
# Request a verification code via SMS
signal-cli -a +1234567890 register

# Or request via voice call
signal-cli -a +1234567890 register --voice

# Enter the verification code
signal-cli -a +1234567890 verify 123-456
```

After verification, the number is registered as a Signal account controlled by signal-cli. This number cannot be used with the Signal mobile app simultaneously -- registering on the app deregisters signal-cli, and vice versa. For a dedicated bot, use a separate phone number (a prepaid SIM or a VoIP number that can receive SMS).

**Option B: Link as a secondary device**

Link signal-cli to an existing Signal account as a secondary device. This lets you keep using the Signal app on your phone while signal-cli operates alongside it:

```bash
# Generate a linking URI and display as QR code
# (install qrencode: apt install qrencode / brew install qrencode)
signal-cli link -n "OpenParallax Agent" | tee /dev/stderr | head -1 | qrencode -t ANSI

# Scan the QR code with Signal on your phone:
# Signal > Settings > Linked Devices > Link New Device
```

Linked devices receive all messages sent to the account. The adapter filters messages based on `allowed_numbers` to process only conversations intended for the bot. Messages you send from your phone are also visible to signal-cli, but the adapter ignores messages sent by the registered account itself.

### 3. Trust Known Contacts

After registration, you may need to trust the safety numbers of contacts you will communicate with:

```bash
# List known identities
signal-cli -a +1234567890 listIdentities

# Trust a contact (accept their current key)
signal-cli -a +1234567890 trust +0987654321 --trust-all-known-keys
```

By default, signal-cli trusts contacts on first use. The `--trust-all-known-keys` flag accepts key changes without manual verification, which is practical for automated use. For high-security deployments, verify safety numbers manually.

### 4. Configure OpenParallax

```yaml
channels:
  signal:
    enabled: true
    cli_path: /usr/local/bin/signal-cli
    account: "+1234567890"
    allowed_numbers:
      - "+0987654321"
```

| Field | Required | Description |
|---|---|---|
| `enabled` | Yes | Enable the Signal adapter |
| `cli_path` | No | Absolute path to the signal-cli binary. If empty, searches `$PATH`. |
| `account` | Yes | The phone number registered with signal-cli (international format with country code) |
| `allowed_numbers` | No | Phone numbers allowed to message the bot. Empty list allows all numbers. |

## How It Works

### JSON-RPC Interface

The adapter spawns signal-cli as a long-lived subprocess in JSON-RPC mode:

```bash
signal-cli -a +1234567890 jsonRpc
```

In this mode, signal-cli reads JSON-RPC requests from stdin and writes responses and notifications to stdout. When a Signal message arrives, signal-cli emits a `receive` notification containing the message envelope. The adapter parses these notifications in real-time, providing low-latency message delivery without polling.

The JSON-RPC mode keeps signal-cli running continuously, maintaining the Signal protocol session and handling encryption key exchanges in the background. This is more efficient than spawning a new signal-cli process for each receive cycle, which would incur JVM startup overhead (5-10 seconds per invocation).

### D-Bus Mode (Linux alternative)

On Linux systems with a D-Bus session bus, signal-cli can run as a daemon that exposes a D-Bus interface:

```bash
signal-cli -a +1234567890 daemon
```

In daemon mode, signal-cli emits D-Bus signals for incoming messages. The adapter can subscribe to these signals for instant delivery. D-Bus mode is useful when signal-cli is managed as a system service independently of OpenParallax.

The adapter prefers JSON-RPC mode (managed subprocess) by default. D-Bus mode is used when signal-cli is already running as a daemon and the adapter detects its D-Bus service.

### Message Flow

```
Signal user sends message
       |
       v
  Signal servers (end-to-end encrypted)
       |
       v
  signal-cli subprocess (JSON-RPC / D-Bus)
       |
       v
  Decrypt and parse envelope
       |
       v
  Extract: sender number, message text, group ID, timestamp
       |
       v
  Access control: check sender against allowed_numbers
       |                                    |
       | (allowed)                          | (denied)
       v                                    v
  Check for slash command prefix        Silently ignored (logged)
       |                     |
       | (not a command)     | (/new, /otr)
       v                     v
  engine.ProcessMessage   Handle command locally
  ForWeb()
       |
       v
  Collect streaming events
       |
       v
  Send response via signal-cli JSON-RPC send method
```

### Session Mapping

Sessions are identified by:
- **Private chats:** `signal:<phone_number>` (e.g., `signal:+0987654321`)
- **Group chats:** `signal:group:<group_id>` (e.g., `signal:group:abc123def456`)

Each private conversation gets its own persistent session. Group chats share one session among all group members. Sessions persist across restarts because signal-cli maintains stable identifiers.

### Outgoing Messages

Responses are sent via the JSON-RPC interface to the running signal-cli subprocess:

```json
{"jsonrpc":"2.0","method":"send","params":{"recipients":["+0987654321"],"message":"Response from agent"},"id":1}
```

For group messages, the `groupId` parameter is used instead of `recipients`.

Long messages (over 4,096 characters) are automatically split at paragraph boundaries. Each chunk is sent as a separate Signal message.

## End-to-End Encryption

Signal provides end-to-end encryption for all messages using the Signal Protocol (Double Ratchet). This has specific implications for the OpenParallax deployment:

### What is encrypted

- All messages between you and the agent are encrypted end-to-end in transit
- Messages are encrypted on Signal's servers -- Signal cannot read them
- The Signal Protocol provides forward secrecy: compromising a key does not compromise past messages
- Each message uses a unique encryption key derived from the ratchet

### What is not encrypted

- Once a message reaches signal-cli, it is decrypted and passed to the engine as plaintext
- The engine stores messages in the SQLite database in plaintext
- Audit logs, memory entries, and session history are stored unencrypted on disk
- The LLM provider receives message content in plaintext over HTTPS

### Practical implication

Signal encryption protects messages in transit between Signal clients. Once the message arrives at your OpenParallax instance, standard application-level security applies. The security of the agent's storage depends on your disk encryption, file permissions, and host security -- not on Signal's encryption.

If you need full end-to-end protection including at-rest data, enable full-disk encryption on the host running OpenParallax and restrict filesystem access to the workspace directory.

### Safety number verification

When a contact reinstalls Signal or changes devices, their safety number changes. signal-cli logs a warning about the change. By default, the adapter continues to accept messages from the contact (trust on first use). For high-security deployments:

1. Disable trust on first use: `signal-cli -a +NUMBER updateConfiguration --trust-new-identities false`
2. Manually verify each contact: `signal-cli -a +NUMBER trust +CONTACT`

When trust is disabled and a contact's key changes, their messages are silently dropped until you explicitly trust the new key.

## Group Chat Support

The adapter supports Signal group chats (both legacy groups and groups v2):

- When the bot's number is added to a group, it receives all messages from the group
- All group members share one session (identified by the group ID)
- The bot responds in the group, visible to all members
- Access control applies per-sender: if `allowed_numbers` is set, only messages from listed numbers are processed, even in groups
- Messages from non-allowed numbers in a group are silently ignored (no error or rejection message)

To add the bot to a group, any group member adds the bot's phone number to the group. The bot starts receiving messages immediately.

### Group vs. Private Chat Behavior

| Behavior | Private Chat | Group Chat |
|---|---|---|
| Session scope | Per sender | Per group (shared among all members) |
| Access control | Per sender number | Per sender number within group |
| Response visibility | Only sender sees it | All group members see it |
| OTR mode | Per sender | Per group |
| `/new` command | Resets sender's session | Resets the group session (affects all members) |

## Supported Message Types

| Type | Incoming | Outgoing |
|---|---|---|
| Text | Yes | Yes |
| Images | No | No |
| Attachments | No | No |
| Reactions | No | No |
| Stickers | No | No |
| Voice messages | No | No |

The adapter handles text messages only. Non-text messages (images, attachments, reactions) are silently ignored. The message text is extracted from the envelope and passed to the engine as-is, with no formatting conversion.

## Slash Commands

| Command | Action |
|---|---|
| `/new` | Reset the session for this chat and start fresh |
| `/otr` | Toggle Off-The-Record mode for this session |

Commands are detected by the `/` prefix. The adapter intercepts them before the message reaches the engine pipeline.

## Access Control

When `allowed_numbers` is configured, only listed phone numbers (in international format with country code) can interact with the bot. Messages from other numbers are silently ignored -- no response is sent, but the attempt is logged with the sender's number and timestamp.

When the list is empty, all numbers are accepted. For personal use, always configure `allowed_numbers` to prevent unauthorized access to your agent.

Phone numbers must include the `+` prefix and country code (e.g., `+1` for US, `+44` for UK, `+49` for Germany). The format must match what signal-cli reports as the sender number. If unsure, check with `signal-cli -a +YOUR_NUMBER listContacts`.

## Resource Usage

signal-cli is a Java application and consumes more memory than other adapter transports:

| Metric | Value |
|---|---|
| Idle memory (JVM) | 200-400 MB |
| Active memory | 400-600 MB during message processing |
| JVM startup time | 5-10 seconds |
| Disk (signal-cli + deps) | ~80 MB |
| Disk (account data) | 10-50 MB per account |

In JSON-RPC mode, signal-cli runs continuously and holds its memory allocation. The JVM startup cost is paid once. For servers with limited memory, signal-cli is the most resource-intensive adapter transport.

## Troubleshooting

| Problem | Solution |
|---|---|
| "No such account" error | The phone number is not registered. Run `signal-cli -a +NUMBER register` and `verify`. |
| Messages not arriving | Check that signal-cli is running in JSON-RPC mode. Try `signal-cli -a +NUMBER receive` manually to verify connectivity. |
| "Untrusted identity" warning | A contact's safety number changed. Run `signal-cli -a +NUMBER trust +CONTACT --trust-all-known-keys`. |
| High memory usage (400+ MB) | This is normal for the JVM. Ensure the host has at least 1 GB free memory for signal-cli. |
| signal-cli subprocess crashes | Check Java version (`java -version`, must be 21+). Check signal-cli logs in stderr. |
| Group messages not received | The bot's number must be a member of the group. Have a group member add the number. |
| "Unregistered user" when sending | The recipient is not on Signal or has deregistered. Verify the number is correct. |
| Linked device not receiving messages | The primary device must be online periodically for linked devices to sync. Check the phone's Signal app. |

## Next Steps

- [Channels Overview](/channels/) -- architecture and shared concepts
- [Configuration](/guide/configuration) -- full config.yaml reference
- [Security](/guide/security) -- how Shield protects across channels
