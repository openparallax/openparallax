package types

// Artifact is a viewable output produced by an action.
type Artifact struct {
	// ID is a unique identifier.
	ID string `json:"id"`

	// Type is "file", "command_output", "diff", or "error".
	Type string `json:"type"`

	// Title is the display name (filename, command summary, etc.).
	Title string `json:"title"`

	// Path is the filesystem path (for file artifacts).
	Path string `json:"path,omitempty"`

	// Content is the artifact content.
	Content string `json:"content"`

	// Language is the programming language (for syntax highlighting).
	Language string `json:"language,omitempty"`

	// SizeBytes is the content size.
	SizeBytes int64 `json:"size_bytes"`

	// PreviewType determines how the frontend renders this artifact.
	// Values: "code", "markdown", "html", "image", "text", "terminal", "diff"
	PreviewType string `json:"preview_type"`
}
