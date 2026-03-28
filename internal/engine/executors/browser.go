package executors

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// BrowserExecutor handles browser navigation and content extraction.
// Uses a detected Chromium-based browser. Returns nil if no browser is found.
type BrowserExecutor struct {
	browserPath string
	log         *logging.Logger
}

// NewBrowserExecutor detects a Chromium-based browser and creates the executor.
// Returns nil if no browser is found — browser tools won't appear in the LLM's tool set.
func NewBrowserExecutor(log *logging.Logger) *BrowserExecutor {
	browserPath := DetectBrowser()
	if browserPath == "" {
		if log != nil {
			log.Info("browser_not_detected", "message", "No Chromium-based browser found. Browser tools will not be available.")
		}
		return nil
	}
	if log != nil {
		log.Info("browser_detected", "path", browserPath)
	}
	return &BrowserExecutor{browserPath: browserPath, log: log}
}

func (b *BrowserExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionBrowserNav, types.ActionBrowserExtract,
		types.ActionBrowserClick, types.ActionBrowserType,
		types.ActionBrowserShot,
	}
}

func (b *BrowserExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{ActionType: types.ActionBrowserNav, Name: "browser_navigate", Description: "Navigate to a URL and return the page content as an accessibility tree — structured headings, links, buttons, and text that represent the page layout.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string", "description": "URL to navigate to."}}, "required": []string{"url"}}},
		{ActionType: types.ActionBrowserExtract, Name: "browser_extract", Description: "Extract content from the current page using a CSS selector or as full accessibility tree.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"selector": map[string]any{"type": "string", "description": "CSS selector to extract. Omit for full page."}, "format": map[string]any{"type": "string", "description": "Output format: text, html, or accessibility_tree.", "enum": []string{"text", "html", "accessibility_tree"}}}}},
		{ActionType: types.ActionBrowserClick, Name: "browser_click", Description: "Click an element on the current page.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"selector": map[string]any{"type": "string", "description": "CSS selector of the element to click."}}, "required": []string{"selector"}}},
		{ActionType: types.ActionBrowserType, Name: "browser_type", Description: "Type text into an input field on the current page.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"selector": map[string]any{"type": "string", "description": "CSS selector of the input element."}, "text": map[string]any{"type": "string", "description": "Text to type."}}, "required": []string{"selector", "text"}}},
		{ActionType: types.ActionBrowserShot, Name: "browser_screenshot", Description: "Take a screenshot of the current page.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"full_page": map[string]any{"type": "boolean", "description": "Capture the full page including scroll. Default false."}}}},
	}
}

func (b *BrowserExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	// Browser operations require chromedp which is an optional dependency.
	// For now, use a simple curl-based fallback for navigate.
	switch action.Type {
	case types.ActionBrowserNav:
		return b.navigate(action)
	default:
		return &types.ActionResult{
			RequestID: action.RequestID, Success: false,
			Error:   "browser interaction requires the chromedp dependency (coming soon). Use http_request for simple page fetching.",
			Summary: "browser action not yet available",
		}
	}
}

func (b *BrowserExecutor) navigate(action *types.ActionRequest) *types.ActionResult {
	url, _ := action.Payload["url"].(string)
	if url == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "url is required"}
	}

	// Build the command — handle flatpak paths which contain spaces.
	args := []string{"--headless=new", "--dump-dom", "--disable-gpu", "--no-sandbox", url}
	cmd := buildBrowserCommand(b.browserPath, args)
	output, err := cmd.Output()
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "browser navigation failed: " + err.Error(), Summary: "browser navigate failed"}
	}

	// Truncate large output.
	content := string(output)
	if len(content) > 50000 {
		content = content[:50000] + "\n\n[Content truncated at 50KB]"
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  content,
		Summary: fmt.Sprintf("navigated to %s", url),
	}
}

// DetectBrowser finds an installed Chromium-based browser.
// Checks system PATH, absolute paths, Flatpak, and Snap installations.
func DetectBrowser() string {
	// System PATH and absolute paths.
	for _, path := range browserCandidates() {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Flatpak browsers.
	if _, err := exec.LookPath("flatpak"); err == nil {
		flatpakBrowsers := []string{
			"com.brave.Browser",
			"com.google.Chrome",
			"org.chromium.Chromium",
			"com.microsoft.Edge",
			"com.opera.Opera",
			"com.vivaldi.Vivaldi",
		}
		for _, appID := range flatpakBrowsers {
			if exec.Command("flatpak", "info", appID).Run() == nil {
				return "flatpak run " + appID
			}
		}
	}

	// Snap browsers.
	if _, err := exec.LookPath("snap"); err == nil {
		snapBrowsers := []string{"chromium", "brave", "google-chrome"}
		for _, name := range snapBrowsers {
			snapBin := filepath.Join("/snap/bin", name)
			if _, err := os.Stat(snapBin); err == nil {
				return snapBin
			}
		}
	}

	return ""
}

// buildBrowserCommand creates an exec.Cmd for the browser path.
// Handles flatpak paths like "flatpak run com.brave.Browser" by splitting.
func buildBrowserCommand(browserPath string, args []string) *exec.Cmd {
	parts := strings.Fields(browserPath)
	if len(parts) > 1 {
		return exec.Command(parts[0], append(parts[1:], args...)...)
	}
	return exec.Command(browserPath, args...)
}

func browserCandidates() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
			"microsoft-edge", "microsoft-edge-stable", "brave-browser", "opera", "vivaldi",
		}
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Arc.app/Contents/MacOS/Arc",
			"/Applications/Opera.app/Contents/MacOS/Opera",
			"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi",
		}
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LocalAppData")
		return []string{
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(localAppData, "Programs", "Opera", "opera.exe"),
			filepath.Join(localAppData, "Vivaldi", "Application", "vivaldi.exe"),
		}
	default:
		return nil
	}
}
