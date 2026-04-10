//go:build windows

package shield

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSafeCommand_WindowsAllowed(t *testing.T) {
	cases := []string{
		"git status",
		"npm install",
		"go build .\\cmd\\agent",
		"cargo build",
		"docker ps",
		"hostname",
		"whoami",
		"echo hello",
		"tasklist",
		"ipconfig",
	}
	for _, c := range cases {
		assert.True(t, IsSafeCommand(c), "expected safe: %q", c)
	}
}

func TestIsSafeCommand_WindowsExeSuffix(t *testing.T) {
	// .exe suffix is stripped before lookup so the bare cmd matches.
	cases := []string{
		"git.exe status",
		"GIT.EXE status",
		"npm.cmd install",
	}
	for _, c := range cases {
		assert.True(t, IsSafeCommand(c), "expected safe: %q", c)
	}
}

func TestIsSafeCommand_WindowsCDPrefixAllowed(t *testing.T) {
	cases := []string{
		`cd C:\Users\me\project && git status`,
		`cd "C:\Users\me\my project" && npm install`,
	}
	for _, c := range cases {
		assert.True(t, IsSafeCommand(c), "expected safe: %q", c)
	}
}
