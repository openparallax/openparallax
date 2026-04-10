package llm

import "testing"

func TestSanitizeToolCallID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"toolu_01ABC", "toolu_01ABC"},
		{"call_read_file_0", "call_read_file_0"},
		{"call-with-dash", "call-with-dash"},
		{"has.dot.in.it", "has_dot_in_it"},
		{"has:colon", "has_colon"},
		{"has/slash", "has_slash"},
		{"has space", "has_space"},
		{"mcp:fs.read_0", "mcp_fs_read_0"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeToolCallID(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeToolCallID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
