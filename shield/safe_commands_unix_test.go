//go:build !windows

package shield

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSafeCommand_UnixAllowed(t *testing.T) {
	cases := []string{
		"git status",
		"git log --oneline -10",
		"git diff",
		"npm install",
		"npm run build",
		"make build",
		"make test",
		"go test ./...",
		"go build ./cmd/agent",
		"cargo build --release",
		"docker ps",
		"kubectl get pods",
		"pwd",
		"whoami",
		"date",
		"hostname",
		"echo hello",
		"python --version",
		"node -v",
	}
	for _, c := range cases {
		assert.True(t, IsSafeCommand(c), "expected safe: %q", c)
	}
}

func TestIsSafeCommand_UnixCDPrefixAllowed(t *testing.T) {
	cases := []string{
		"cd /home/user/project && git status",
		"cd /home/user/project && npm install",
		"cd /home/user/project && make build",
		`cd "/home/user/my project" && git status`,
		`cd '/home/user/my project' && npm test`,
		"cd ~/projects && git status",
	}
	for _, c := range cases {
		assert.True(t, IsSafeCommand(c), "expected safe: %q", c)
	}
}

func TestIsSafeCommand_UnixFullPathBinary(t *testing.T) {
	// Full paths to known-safe binaries should still match.
	assert.True(t, IsSafeCommand("/usr/bin/git status"))
	assert.True(t, IsSafeCommand("/usr/local/bin/npm install"))
}
