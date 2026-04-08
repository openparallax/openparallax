package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultProtection_RestrictedCredentialDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only credential paths")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home dir")
	}

	cases := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".aws", "credentials"),
		filepath.Join(home, ".gnupg", "private-keys-v1.d", "key"),
		filepath.Join(home, ".kube", "config"),
	}
	for _, p := range cases {
		assert.Equal(t, FullBlock, defaultProtection(p), "%s should be FullBlock", p)
	}
}

func TestDefaultProtection_ProtectedShellRC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only rc files")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home dir")
	}

	cases := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".vimrc"),
	}
	for _, p := range cases {
		assert.Equal(t, ReadOnly, defaultProtection(p), "%s should be ReadOnly", p)
	}
}

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

func TestDefaultProtection_RestrictedBasenamePatterns_CaseInsensitive(t *testing.T) {
	cases := []string{
		"/tmp/SERVER.PEM",
		"/tmp/MyApp.KEY",
		"/tmp/.ENV",
	}
	for _, p := range cases {
		assert.Equal(t, FullBlock, defaultProtection(p), "%s should be FullBlock", p)
	}
}

func TestDefaultProtection_PublicKeyAllowed(t *testing.T) {
	// .pub files are public keys and not sensitive.
	cases := []string{
		"/home/user/.ssh/id_rsa.pub",
	}
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only ssh paths")
	}
	for _, p := range cases {
		// .ssh dir is restricted as a prefix even for .pub files —
		// the user can copy the .pub out if they need to expose it.
		assert.Equal(t, FullBlock, defaultProtection(p))
	}
}

func TestDefaultProtection_LinuxSystemFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only")
	}
	assert.Equal(t, FullBlock, defaultProtection("/etc/shadow"))
	assert.Equal(t, FullBlock, defaultProtection("/etc/sudoers"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/hosts"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/passwd"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/cron.d/some-job"))
}

func TestDefaultProtection_BenignPath(t *testing.T) {
	cases := []string{
		"/home/user/Desktop/notes.txt",
		"/tmp/scratch.go",
		"/home/user/projects/myapp/main.go",
		"/var/log/myapp.log",
	}
	for _, p := range cases {
		assert.Equal(t, Unprotected, defaultProtection(p), "%s should be Unprotected", p)
	}
}

func TestDefaultProtection_EmptyPath(t *testing.T) {
	assert.Equal(t, Unprotected, defaultProtection(""))
}

func TestPathHasPrefix(t *testing.T) {
	assert.True(t, pathHasPrefix("/home/user/.ssh", "/home/user/.ssh"))
	assert.True(t, pathHasPrefix("/home/user/.ssh/id_rsa", "/home/user/.ssh"))
	assert.True(t, pathHasPrefix("/home/user/.ssh/sub/key", "/home/user/.ssh"))
	assert.False(t, pathHasPrefix("/home/user/.ssh-backup", "/home/user/.ssh"))
	assert.False(t, pathHasPrefix("/home/user", "/home/user/.ssh"))
}
