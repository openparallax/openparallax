// Package signal implements the Signal channel adapter using signal-cli
// as an external subprocess for sending and receiving messages.
package signal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// Adapter implements channels.ChannelAdapter for Signal via signal-cli.
type Adapter struct {
	cliPath        string
	account        string
	allowedNumbers map[string]bool
	manager        *channels.Manager
	log            *logging.Logger
	cmd            *exec.Cmd
	cancel         context.CancelFunc
}

// New creates a Signal adapter from config.
func New(cfg *types.SignalConfig, manager *channels.Manager, log *logging.Logger) *Adapter {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	cliPath := cfg.CLIPath
	if cliPath == "" {
		// Try to find signal-cli in PATH.
		if p, err := exec.LookPath("signal-cli"); err == nil {
			cliPath = p
		}
	}
	if cliPath == "" {
		return nil
	}

	// Verify signal-cli exists.
	if _, err := os.Stat(cliPath); err != nil {
		return nil
	}

	allowed := make(map[string]bool)
	for _, num := range cfg.AllowedNumbers {
		allowed[num] = true
	}

	return &Adapter{
		cliPath:        cliPath,
		account:        cfg.Account,
		allowedNumbers: allowed,
		manager:        manager,
		log:            log,
	}
}

// Name returns "signal".
func (a *Adapter) Name() string { return "signal" }

// IsConfigured returns true if signal-cli is available and an account is set.
func (a *Adapter) IsConfigured() bool { return a.cliPath != "" && a.account != "" }

// Start begins listening for Signal messages via signal-cli JSON-RPC mode.
func (a *Adapter) Start(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	a.cmd = exec.CommandContext(childCtx, a.cliPath,
		"-a", a.account, "jsonRpc")
	a.cmd.Stderr = os.Stderr

	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("signal-cli stdout pipe: %w", err)
	}

	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("signal-cli stdin pipe: %w", err)
	}

	if err := a.cmd.Start(); err != nil {
		return fmt.Errorf("start signal-cli: %w", err)
	}

	a.log.Info("signal_started", "account", a.account)

	// Read JSON-RPC messages from stdout.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var rpcMsg signalRPCMessage
		if parseErr := json.Unmarshal([]byte(line), &rpcMsg); parseErr != nil {
			continue
		}

		if rpcMsg.Method == "receive" && rpcMsg.Params.Envelope.DataMessage.Message != "" {
			go a.handleMessage(ctx, stdin, rpcMsg.Params.Envelope)
		}
	}

	_ = stdin.Close()
	return a.cmd.Wait()
}

// Stop terminates the signal-cli process.
func (a *Adapter) Stop() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		return a.cmd.Process.Kill()
	}
	return nil
}

// SendMessage sends a text message via signal-cli.
func (a *Adapter) SendMessage(chatID string, msg *channels.ChannelMessage) error {
	parts := channels.SplitMessage(msg.Text, 4096)
	for _, part := range parts {
		cmd := exec.Command(a.cliPath,
			"-a", a.account,
			"send", "-m", part, chatID)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("signal send: %w", err)
		}
	}
	return nil
}

func (a *Adapter) handleMessage(ctx context.Context, _ interface{}, envelope signalEnvelope) {
	from := envelope.Source
	text := envelope.DataMessage.Message

	if len(a.allowedNumbers) > 0 && !a.allowedNumbers[from] {
		a.log.Info("signal_unauthorized", "from", from)
		return
	}

	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		if response, action, handled := a.manager.HandleCommand("signal", from, text, "signal"); handled {
			if response != "" {
				_ = a.SendMessage(from, &channels.ChannelMessage{Text: response})
			}
			_ = action
			return
		}
	}

	mode := types.SessionNormal
	response, err := a.manager.HandleMessage(ctx, "signal", from, text, mode)
	if err != nil {
		a.log.Error("signal_error", "from", from, "error", err)
		return
	}
	if response != "" {
		_ = a.SendMessage(from, &channels.ChannelMessage{Text: response})
	}
}

// --- Signal JSON-RPC types ---

type signalRPCMessage struct {
	Method string       `json:"method"`
	Params signalParams `json:"params"`
}

type signalParams struct {
	Envelope signalEnvelope `json:"envelope"`
}

type signalEnvelope struct {
	Source      string            `json:"source"`
	DataMessage signalDataMessage `json:"dataMessage"`
}

type signalDataMessage struct {
	Message string `json:"message"`
}
