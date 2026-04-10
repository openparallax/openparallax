//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func browserCandidatesPlatform() []string {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	localAppData := os.Getenv("LocalAppData")

	join := func(base string, parts ...string) string {
		if base == "" {
			return ""
		}
		return filepath.Join(append([]string{base}, parts...)...)
	}

	return []string{
		join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
		join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
		join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
		join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
		join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
		join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		join(localAppData, "Programs", "Opera", "opera.exe"),
		join(localAppData, "Vivaldi", "Application", "vivaldi.exe"),
	}
}
