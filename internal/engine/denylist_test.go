package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Cross-platform tests for the denylist evaluator. Per-OS tests live
// in denylist_linux_test.go etc.

func TestDefaultProtection_RestrictedBasenamePatterns(t *testing.T) {
	cases := []string{
		"/tmp/anywhere/id_rsa",
		"/home/user/projects/secret.pem",
		"/home/user/server.key",
		"/var/data/backup.p12",
		"/home/user/project/.env",
		"/home/user/myapp/credentials.json",
		"/tmp/secrets.yaml",
	}
	for _, p := range cases {
		assert.Equal(t, FullBlock, defaultProtection(p), "%s should be FullBlock", p)
	}
}

func TestDefaultProtection_RestrictedBasenamePatternsCaseInsensitive(t *testing.T) {
	cases := []string{
		"/tmp/SERVER.PEM",
		"/tmp/MyApp.KEY",
		"/tmp/.ENV",
	}
	for _, p := range cases {
		assert.Equal(t, FullBlock, defaultProtection(p), "%s should be FullBlock", p)
	}
}

func TestDefaultProtection_BenignPath(t *testing.T) {
	cases := []string{
		"/tmp/scratch.go",
		"/var/log/myapp.log",
		"/data/notes.txt",
	}
	for _, p := range cases {
		assert.Equal(t, Unprotected, defaultProtection(p), "%s should be Unprotected", p)
	}
}

func TestDefaultProtection_EmptyPath(t *testing.T) {
	assert.Equal(t, Unprotected, defaultProtection(""))
}
