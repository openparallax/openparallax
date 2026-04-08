//go:build darwin

package platform

import "fmt"

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
