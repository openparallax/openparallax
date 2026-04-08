package platform

// BrowserCandidates returns the ordered list of executable names or absolute
// paths to try when locating a Chromium-based browser on this platform. The
// list is platform-specific and supplied by build-tagged files. Empty entries
// (e.g. paths built from a missing env var) are filtered out so callers never
// see partially-formed paths.
func BrowserCandidates() []string {
	raw := browserCandidatesPlatform()
	out := make([]string, 0, len(raw))
	for _, c := range raw {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}
