// Package templates embeds the workspace template files for use by the init command.
package templates

import "embed"

// WorkspaceFS contains the embedded workspace template files.
// The path prefix "files/" must be stripped when reading entries.
//
//go:embed files
var WorkspaceFS embed.FS
