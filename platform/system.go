package platform

// SystemTools exposes the cross-platform command builders used by the
// SystemExecutor (clipboard read/write, open in default app, OS
// notifications, desktop screenshot). Per-platform implementations
// live in system_{linux,darwin,windows}.go and select the
// appropriate native binary for the host OS at build time.
//
// Each builder returns a command-line argv slice ready to feed into
// exec.Command, or nil when the operation is not supported on the
// current platform.

// ClipboardReadCmd returns the command that reads the system
// clipboard as text on stdout. Returns nil + error when the platform
// has no available clipboard provider.
func ClipboardReadCmd() ([]string, error) { return clipboardReadCmd() }

// ClipboardWriteCmd returns the command that writes stdin to the
// system clipboard. Returns nil + error when the platform has no
// available clipboard provider.
func ClipboardWriteCmd() ([]string, error) { return clipboardWriteCmd() }

// OpenCmd returns the command that opens the given target (file path
// or URL) in the system's default application. Returns nil when the
// platform has no recognized open command.
func OpenCmd(target string) []string { return openCmd(target) }

// NotifyCmd returns the command that emits an OS notification with
// the given title and body. Returns nil when the platform has no
// recognized notification provider.
func NotifyCmd(title, message string) []string { return notifyCmd(title, message) }

// ScreenshotCmd returns the command that captures the desktop and
// writes the result to outputPath as a PNG. Returns nil when the
// platform has no recognized screenshot provider.
func ScreenshotCmd(outputPath string) []string { return screenshotCmd(outputPath) }

// SystemToolCapabilities reports which individual system tools are
// available on this host. Each capability is checked independently —
// system_info and open are always available, while clipboard, notify, and
// screenshot depend on platform-specific binaries (xclip, notify-send,
// screencapture, etc.) that may or may not be installed.
//
// The map uses internal/types ActionType keys directly so the
// SystemExecutor can index it without an extra translation layer. The
// caller treats absence as "not available" — only present-and-true entries
// expose the corresponding tool to the LLM.
func SystemToolCapabilities() map[SystemAction]bool {
	return systemToolCapabilities()
}

// SystemAction is the per-tool action identifier the system executor uses
// to gate availability. Mirrors internal/types ActionType values to keep
// the platform package free of an internal/types dependency.
type SystemAction string

const (
	SystemActionClipboardRead  SystemAction = "clipboard_read"
	SystemActionClipboardWrite SystemAction = "clipboard_write"
	SystemActionOpen           SystemAction = "open"
	SystemActionNotify         SystemAction = "notify"
	SystemActionSystemInfo     SystemAction = "system_info"
	SystemActionScreenshot     SystemAction = "screenshot"
)
