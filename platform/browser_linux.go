//go:build linux

package platform

func browserCandidatesPlatform() []string {
	return []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"microsoft-edge",
		"microsoft-edge-stable",
		"brave-browser",
		"opera",
		"vivaldi",
	}
}
