//go:build linux

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultProtection_LinuxCredentialDirs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	cases := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".aws", "credentials"),
		filepath.Join(home, ".gnupg", "private-keys-v1.d", "key"),
		filepath.Join(home, ".kube", "config"),
		filepath.Join(home, ".docker", "config.json"),
		filepath.Join(home, ".password-store", "vault.gpg"),
	}
	for _, p := range cases {
		assert.Equal(t, FullBlock, defaultProtection(p), "%s should be FullBlock", p)
	}
}

func TestDefaultProtection_LinuxShellRC(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	cases := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".vimrc"),
		filepath.Join(home, ".profile"),
	}
	for _, p := range cases {
		assert.Equal(t, ReadOnly, defaultProtection(p), "%s should be ReadOnly", p)
	}
}

func TestDefaultProtection_LinuxSystemFiles(t *testing.T) {
	assert.Equal(t, FullBlock, defaultProtection("/etc/shadow"))
	assert.Equal(t, FullBlock, defaultProtection("/etc/sudoers"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/hosts"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/passwd"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/cron.d/some-job"))
	assert.Equal(t, ReadOnly, defaultProtection("/etc/systemd/system/foo.service"))
}

func TestDefaultProtection_LinuxRoot(t *testing.T) {
	assert.Equal(t, FullBlock, defaultProtection("/root/.ssh/authorized_keys"))
}

func TestDefaultProtection_LinuxBenign(t *testing.T) {
	cases := []string{
		"/home/user/Desktop/notes.txt",
		"/home/user/projects/myapp/main.go",
	}
	for _, p := range cases {
		assert.Equal(t, Unprotected, defaultProtection(p), "%s should be Unprotected", p)
	}
}
