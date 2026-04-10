//go:build darwin

package platform

func browserCandidatesPlatform() []string {
	return []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Arc.app/Contents/MacOS/Arc",
		"/Applications/Opera.app/Contents/MacOS/Opera",
		"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
	}
}
