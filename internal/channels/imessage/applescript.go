//go:build darwin

package imessage

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// imsg represents a received iMessage.
type imsg struct {
	Sender   string
	Text     string
	RecvTime time.Time
}

// getNewMessages queries Messages.app for messages received after since.
func getNewMessages(since time.Time) ([]imsg, error) {
	script := fmt.Sprintf(`
tell application "Messages"
	set output to ""
	repeat with c in text chats
		set cid to id of c
		repeat with m in messages of c
			if date received of m > date "%s" then
				set sndr to handle of sender of m
				set txt to text of m
				set output to output & sndr & "	" & txt & "
"
			end if
		end repeat
	end repeat
	return output
end tell`, since.Format("Monday, January 2, 2006 at 3:04:05 PM"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if err != nil {
		return nil, fmt.Errorf("osascript: %w", err)
	}

	var msgs []imsg
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		msgs = append(msgs, imsg{
			Sender:   parts[0],
			Text:     parts[1],
			RecvTime: time.Now(),
		})
	}
	return msgs, nil
}

// sendMessage sends a text message via Messages.app to the given recipient.
func sendMessage(recipient, text string) error {
	escapeAS := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	script := fmt.Sprintf(`
tell application "Messages"
	set targetService to 1st service whose service type = iMessage
	set targetBuddy to buddy "%s" of targetService
	send "%s" to targetBuddy
end tell`, escapeAS(recipient), escapeAS(text))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := exec.CommandContext(ctx, "osascript", "-e", script).Run(); err != nil {
		return fmt.Errorf("send imessage to %s: %w", recipient, err)
	}
	return nil
}
