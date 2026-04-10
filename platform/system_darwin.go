//go:build darwin

package platform

import "fmt"

// systemToolCapabilities on macOS returns every tool as available:
// pbpaste, pbcopy, open, osascript, and screencapture all ship with
// the OS.
func systemToolCapabilities() map[SystemAction]bool {
	return map[SystemAction]bool{
		SystemActionClipboardRead:  true,
		SystemActionClipboardWrite: true,
		SystemActionOpen:           true,
		SystemActionNotify:         true,
		SystemActionSystemInfo:     true,
		SystemActionScreenshot:     true,
	}
}

func clipboardReadCmd() ([]string, error) {
	return []string{"pbpaste"}, nil
}

func clipboardWriteCmd() ([]string, error) {
	return []string{"pbcopy"}, nil
}

func openCmd(target string) []string {
	return []string{"open", target}
}

func notifyCmd(title, message string) []string {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	return []string{"osascript", "-e", script}
}

func screenshotCmd(outputPath string) []string {
	return []string{"screencapture", "-x", outputPath}
}
