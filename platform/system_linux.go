//go:build linux

package platform

import (
	"fmt"
	"os/exec"
)

// systemToolCapabilities reports per-tool availability on Linux. Each
// capability is checked independently against the actual binaries on
// PATH. system_info uses Go stdlib so it is always available; open
// requires xdg-open. Clipboard, notify, and screenshot each have their
// own dependency set.
func systemToolCapabilities() map[SystemAction]bool {
	caps := map[SystemAction]bool{
		SystemActionSystemInfo: true,
	}
	if hasAny("xdg-open") {
		caps[SystemActionOpen] = true
	}
	if hasAny("wl-paste", "xclip", "xsel") {
		caps[SystemActionClipboardRead] = true
	}
	if hasAny("wl-copy", "xclip", "xsel") {
		caps[SystemActionClipboardWrite] = true
	}
	if hasAny("notify-send") {
		caps[SystemActionNotify] = true
	}
	if hasAny("grim", "scrot", "gnome-screenshot", "import") {
		caps[SystemActionScreenshot] = true
	}
	return caps
}

func hasAny(bins ...string) bool {
	for _, b := range bins {
		if _, err := exec.LookPath(b); err == nil {
			return true
		}
	}
	return false
}

func clipboardReadCmd() ([]string, error) {
	if _, err := exec.LookPath("wl-paste"); err == nil {
		return []string{"wl-paste", "--no-newline"}, nil
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return []string{"xclip", "-selection", "clipboard", "-o"}, nil
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return []string{"xsel", "--clipboard", "--output"}, nil
	}
	return nil, fmt.Errorf("clipboard not available — no display server detected (install xclip, xsel, or wl-paste)")
}

func clipboardWriteCmd() ([]string, error) {
	if _, err := exec.LookPath("wl-copy"); err == nil {
		return []string{"wl-copy"}, nil
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return []string{"xclip", "-selection", "clipboard", "-i"}, nil
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return []string{"xsel", "--clipboard", "--input"}, nil
	}
	return nil, fmt.Errorf("clipboard not available — no display server detected (install xclip, xsel, or wl-copy)")
}

func openCmd(target string) []string {
	return []string{"xdg-open", target}
}

func notifyCmd(title, message string) []string {
	if _, err := exec.LookPath("notify-send"); err == nil {
		return []string{"notify-send", title, message}
	}
	return nil
}

func screenshotCmd(outputPath string) []string {
	if _, err := exec.LookPath("grim"); err == nil {
		return []string{"grim", outputPath}
	}
	if _, err := exec.LookPath("scrot"); err == nil {
		return []string{"scrot", outputPath}
	}
	if _, err := exec.LookPath("gnome-screenshot"); err == nil {
		return []string{"gnome-screenshot", "-f", outputPath}
	}
	if _, err := exec.LookPath("import"); err == nil {
		return []string{"import", "-window", "root", outputPath}
	}
	return nil
}
