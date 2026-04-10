package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstToken(t *testing.T) {
	cases := map[string]string{
		"":                         "",
		"git":                      "git",
		"git status":               "git",
		"git\tstatus":              "git",
		"  git status":             "",
		"npm install --save react": "npm",
	}
	for in, want := range cases {
		assert.Equal(t, want, FirstToken(in), "FirstToken(%q)", in)
	}
}

func TestStripLeadingCD_Basic(t *testing.T) {
	target, rest, ok := StripLeadingCD("cd /home/user/project && git status")
	assert.True(t, ok)
	assert.Equal(t, "/home/user/project", target)
	assert.Equal(t, "git status", rest)
}

func TestStripLeadingCD_DoubleQuoted(t *testing.T) {
	target, rest, ok := StripLeadingCD(`cd "/home/user/my project" && git status`)
	assert.True(t, ok)
	assert.Equal(t, "/home/user/my project", target)
	assert.Equal(t, "git status", rest)
}

func TestStripLeadingCD_SingleQuoted(t *testing.T) {
	target, rest, ok := StripLeadingCD(`cd '/home/user/my project' && git status`)
	assert.True(t, ok)
	assert.Equal(t, "/home/user/my project", target)
	assert.Equal(t, "git status", rest)
}

func TestStripLeadingCD_RelativeStillReturned(t *testing.T) {
	// StripLeadingCD does not validate absoluteness — it returns the
	// target as-is and lets the caller decide.
	target, rest, ok := StripLeadingCD("cd backend && rm main.go")
	assert.True(t, ok)
	assert.Equal(t, "backend", target)
	assert.Equal(t, "rm main.go", rest)
}

func TestStripLeadingCD_NoCDPrefix(t *testing.T) {
	cases := []string{
		"git status",
		"echo cd /tmp && rm",
		"",
		"  ",
	}
	for _, in := range cases {
		_, _, ok := StripLeadingCD(in)
		assert.False(t, ok, "StripLeadingCD(%q)", in)
	}
}

func TestStripLeadingCD_EmptyCDTarget(t *testing.T) {
	// "cd " followed by nothing must not panic and must return ok=false.
	cases := []string{
		"cd ",
		"cd \t",
		"cd  \t  ",
	}
	for _, in := range cases {
		_, _, ok := StripLeadingCD(in)
		assert.False(t, ok, "StripLeadingCD(%q)", in)
	}
}

func TestStripLeadingCD_UnterminatedQuote(t *testing.T) {
	cases := []string{
		`cd "/home/user/proj && git status`,
		`cd '/home/user/proj && git status`,
	}
	for _, in := range cases {
		_, _, ok := StripLeadingCD(in)
		assert.False(t, ok, "StripLeadingCD(%q)", in)
	}
}

func TestStripLeadingCD_NoAndAnd(t *testing.T) {
	_, _, ok := StripLeadingCD("cd /tmp; rm foo")
	assert.False(t, ok)
}

func TestStripLeadingCD_TabSeparator(t *testing.T) {
	target, rest, ok := StripLeadingCD("cd\t/home/user\t&&\tgit status")
	assert.True(t, ok)
	assert.Equal(t, "/home/user", target)
	assert.Equal(t, "git status", rest)
}

func TestIsAbsolutePathSpec(t *testing.T) {
	assert.False(t, IsAbsolutePathSpec(""))
	assert.False(t, IsAbsolutePathSpec("foo"))
	assert.False(t, IsAbsolutePathSpec("./foo"))
	assert.False(t, IsAbsolutePathSpec("../foo"))
	assert.True(t, IsAbsolutePathSpec("/foo"))
	assert.True(t, IsAbsolutePathSpec("~"))
	assert.True(t, IsAbsolutePathSpec("~/foo"))
}

func TestPathHasPrefix(t *testing.T) {
	assert.True(t, PathHasPrefix("/home/user/.ssh", "/home/user/.ssh"))
	assert.True(t, PathHasPrefix("/home/user/.ssh/id_rsa", "/home/user/.ssh"))
	assert.True(t, PathHasPrefix("/home/user/.ssh/sub/key", "/home/user/.ssh"))
	assert.False(t, PathHasPrefix("/home/user/.ssh-backup", "/home/user/.ssh"))
	assert.False(t, PathHasPrefix("/home/user", "/home/user/.ssh"))
	assert.True(t, PathHasPrefix("/etc/hosts", "/etc/hosts/"))
}
