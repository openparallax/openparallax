package shield

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferActionType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ActionType
	}{
		{"read_file maps to ActionReadFile", "read_file", ActionReadFile},
		{"get_file_contents maps to ActionReadFile", "get_file_contents", ActionReadFile},
		{"cat maps to ActionReadFile", "cat", ActionReadFile},
		{"write_file maps to ActionWriteFile", "write_file", ActionWriteFile},
		{"create_file maps to ActionWriteFile", "create_file", ActionWriteFile},
		{"put_file maps to ActionWriteFile", "put_file", ActionWriteFile},
		{"delete_file maps to ActionDeleteFile", "delete_file", ActionDeleteFile},
		{"remove_file maps to ActionDeleteFile", "remove_file", ActionDeleteFile},
		{"rm maps to ActionDeleteFile", "rm", ActionDeleteFile},
		{"move_file maps to ActionMoveFile", "move_file", ActionMoveFile},
		{"rename_file maps to ActionMoveFile", "rename_file", ActionMoveFile},
		{"mv maps to ActionMoveFile", "mv", ActionMoveFile},
		{"copy_file maps to ActionCopyFile", "copy_file", ActionCopyFile},
		{"cp maps to ActionCopyFile", "cp", ActionCopyFile},
		{"create_directory maps to ActionCreateDir", "create_directory", ActionCreateDir},
		{"mkdir maps to ActionCreateDir", "mkdir", ActionCreateDir},
		{"list_directory maps to ActionListDir", "list_directory", ActionListDir},
		{"ls maps to ActionListDir", "ls", ActionListDir},
		{"list_dir maps to ActionListDir", "list_dir", ActionListDir},
		{"search maps to ActionSearchFiles", "search", ActionSearchFiles},
		{"grep maps to ActionSearchFiles", "grep", ActionSearchFiles},
		{"find maps to ActionSearchFiles", "find", ActionSearchFiles},
		{"glob maps to ActionSearchFiles", "glob", ActionSearchFiles},
		{"run_command maps to ActionExecCommand", "run_command", ActionExecCommand},
		{"execute maps to ActionExecCommand", "execute", ActionExecCommand},
		{"bash maps to ActionExecCommand", "bash", ActionExecCommand},
		{"shell maps to ActionExecCommand", "shell", ActionExecCommand},
		{"exec maps to ActionExecCommand", "exec", ActionExecCommand},
		{"send_message maps to ActionSendMessage", "send_message", ActionSendMessage},
		{"post_message maps to ActionSendMessage", "post_message", ActionSendMessage},
		{"send_email maps to ActionSendEmail", "send_email", ActionSendEmail},
		{"http maps to ActionHTTPRequest", "http", ActionHTTPRequest},
		{"fetch maps to ActionHTTPRequest", "fetch", ActionHTTPRequest},
		{"request maps to ActionHTTPRequest", "request", ActionHTTPRequest},
		{"curl maps to ActionHTTPRequest", "curl", ActionHTTPRequest},
		{"api maps to ActionHTTPRequest", "api", ActionHTTPRequest},
		{"unknown tool returns lowercased name", "SomeCustomTool", ActionType("somecustomtool")},
		{"already lowercase unknown", "my_special_tool", ActionType("my_special_tool")},
		{"case insensitive matching", "READ_FILE", ActionReadFile},
		{"mixed case matching", "Write_File", ActionWriteFile},
		{"substring match works", "my_read_file_tool", ActionReadFile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, inferActionType(tc.input))
		})
	}
}

func TestServerFromToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"namespaced tool extracts server", "mcp:filesystem:read_file", "filesystem"},
		{"two-part mcp prefix returns server", "mcp:github", "github"},
		{"no prefix returns empty", "read_file", ""},
		{"non-mcp prefix returns empty", "other:server:tool", ""},
		{"empty string returns empty", "", ""},
		{"mcp with colons in tool name", "mcp:myserver:tool:with:colons", "myserver"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, serverFromToolName(tc.input))
		})
	}
}

func TestOriginalToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"namespaced tool extracts original", "mcp:filesystem:read_file", "read_file"},
		{"non-namespaced returns as-is", "read_file", "read_file"},
		{"two-part returns as-is", "mcp:server", "mcp:server"},
		{"empty string returns as-is", "", ""},
		{"colons in tool name preserved", "mcp:server:tool:sub:part", "tool:sub:part"},
		{"non-mcp prefix returns as-is", "other:server:tool", "other:server:tool"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, originalToolName(tc.input))
		})
	}
}

func TestResolveActionType(t *testing.T) {
	t.Run("custom mapping takes priority", func(t *testing.T) {
		gw := &MCPGateway{
			toolMapping: map[string]ActionType{
				"mcp:fs:read": ActionWriteFile,
			},
		}
		assert.Equal(t, ActionWriteFile, gw.resolveActionType("mcp:fs:read"))
	})

	t.Run("original name checked in mapping", func(t *testing.T) {
		gw := &MCPGateway{
			toolMapping: map[string]ActionType{
				"read_file": ActionReadFile,
			},
		}
		assert.Equal(t, ActionReadFile, gw.resolveActionType("mcp:server:read_file"))
	})

	t.Run("falls back to inference", func(t *testing.T) {
		gw := &MCPGateway{
			toolMapping: map[string]ActionType{},
		}
		assert.Equal(t, ActionReadFile, gw.resolveActionType("mcp:fs:read_file"))
	})

	t.Run("inference on plain tool name", func(t *testing.T) {
		gw := &MCPGateway{
			toolMapping: map[string]ActionType{},
		}
		assert.Equal(t, ActionExecCommand, gw.resolveActionType("bash"))
	})

	t.Run("full name mapping beats original name mapping", func(t *testing.T) {
		gw := &MCPGateway{
			toolMapping: map[string]ActionType{
				"mcp:server:write_file": ActionDeleteFile,
				"write_file":            ActionWriteFile,
			},
		}
		assert.Equal(t, ActionDeleteFile, gw.resolveActionType("mcp:server:write_file"))
	})
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		patterns []string
		expected bool
	}{
		{"exact match", "read_file", []string{"read_file"}, true},
		{"substring match", "my_read_file_tool", []string{"read_file"}, true},
		{"no match", "write_file", []string{"read_file", "cat"}, false},
		{"first pattern matches", "cat", []string{"cat", "dog"}, true},
		{"second pattern matches", "dog", []string{"cat", "dog"}, true},
		{"empty patterns", "anything", []string{}, false},
		{"empty string no match", "", []string{"something"}, false},
		{"empty string empty pattern matches", "", []string{""}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, containsAny(tc.s, tc.patterns...))
		})
	}
}
