package executors

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// BrowserExecutor handles all browser actions via chromedp (Chrome DevTools Protocol).
// Maintains a persistent headless browser session that is lazily started on first use
// and shut down after an idle timeout.
type BrowserExecutor struct {
	browserPath string
	log         *logging.Logger

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	lastUsed time.Time
	running  bool
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

// SupportedActions returns all browser action types.
func (b *BrowserExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionBrowserNav, types.ActionBrowserExtract,
		types.ActionBrowserClick, types.ActionBrowserType,
		types.ActionBrowserShot,
	}
}

// ToolSchemas returns tool definitions for the LLM.
func (b *BrowserExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{ActionType: types.ActionBrowserNav, Name: "browser_navigate", Description: "Navigate to a URL and return the page text content.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string", "description": "URL to navigate to."}}, "required": []string{"url"}}},
		{ActionType: types.ActionBrowserExtract, Name: "browser_extract", Description: "Extract content from the current page using a CSS selector.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"selector": map[string]any{"type": "string", "description": "CSS selector to extract. Omit for full page text."}, "format": map[string]any{"type": "string", "description": "Output format: text or html.", "enum": []string{"text", "html"}}}}},
		{ActionType: types.ActionBrowserClick, Name: "browser_click", Description: "Click an element on the current page by CSS selector.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"selector": map[string]any{"type": "string", "description": "CSS selector of the element to click."}}, "required": []string{"selector"}}},
		{ActionType: types.ActionBrowserType, Name: "browser_type", Description: "Type text into an input field on the current page.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"selector": map[string]any{"type": "string", "description": "CSS selector of the input element."}, "text": map[string]any{"type": "string", "description": "Text to type."}}, "required": []string{"selector", "text"}}},
		{ActionType: types.ActionBrowserShot, Name: "browser_screenshot", Description: "Take a screenshot of the current page and returns it as base64-encoded image data.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"full_page": map[string]any{"type": "boolean", "description": "Capture the full scrollable page. Default false."}}}},
	}
}

// Execute dispatches browser actions.
func (b *BrowserExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	bctx, err := b.ensureSession(ctx)
	if err != nil {
		return &types.ActionResult{
			RequestID: action.RequestID,
			Success:   false,
			Error:     "failed to start browser: " + err.Error(),
			Summary:   "browser session failed",
		}
	}

	switch action.Type {
	case types.ActionBrowserNav:
		return b.navigate(bctx, action)
	case types.ActionBrowserExtract:
		return b.extract(bctx, action)
	case types.ActionBrowserClick:
		return b.click(bctx, action)
	case types.ActionBrowserType:
		return b.typeText(bctx, action)
	case types.ActionBrowserShot:
		return b.screenshot(bctx, action)
	default:
		return ErrorResult(action.RequestID, "unknown browser action", "unknown action")
	}
}

// Shutdown closes the browser session.
func (b *BrowserExecutor) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
		b.running = false
	}
}

func (b *BrowserExecutor) navigate(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	url, _ := action.Payload["url"].(string)
	if url == "" {
		return ErrorResult(action.RequestID, "url is required", "missing url")
	}

	if err := validateURLNotPrivate(url); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "blocked: private/internal address")
	}

	var body string
	taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := chromedp.Run(taskCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.InnerHTML("body", &body),
	)
	if err != nil {
		return ErrorResult(action.RequestID, "navigation failed: "+err.Error(), "navigate failed")
	}

	// Extract text content for the LLM — raw HTML is too noisy.
	var text string
	_ = chromedp.Run(taskCtx, chromedp.Text("body", &text))
	if text == "" {
		text = body
	}

	content := truncateContent(text, 50000)

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  content,
		Summary: fmt.Sprintf("navigated to %s (%d chars)", url, len(content)),
	}
}

func (b *BrowserExecutor) extract(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	selector, _ := action.Payload["selector"].(string)
	format, _ := action.Payload["format"].(string)
	if selector == "" {
		selector = "body"
	}
	if format == "" {
		format = "text"
	}

	taskCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var content string
	var err error

	switch format {
	case "html":
		err = chromedp.Run(taskCtx, chromedp.InnerHTML(selector, &content))
	default:
		err = chromedp.Run(taskCtx, chromedp.Text(selector, &content))
	}
	if err != nil {
		return ErrorResult(action.RequestID, "extract failed: "+err.Error(), "extract failed")
	}

	content = truncateContent(content, 50000)

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  content,
		Summary: fmt.Sprintf("extracted %s from %s (%d chars)", format, selector, len(content)),
	}
}

func (b *BrowserExecutor) click(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	selector, _ := action.Payload["selector"].(string)
	if selector == "" {
		return ErrorResult(action.RequestID, "selector is required", "missing selector")
	}

	taskCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err := chromedp.Run(taskCtx,
		chromedp.WaitVisible(selector),
		chromedp.Click(selector),
	)
	if err != nil {
		return ErrorResult(action.RequestID, "click failed: "+err.Error(), "click failed")
	}

	// Brief wait for any navigation or rendering triggered by the click.
	_ = chromedp.Run(taskCtx, chromedp.Sleep(500*time.Millisecond))

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  "clicked",
		Summary: fmt.Sprintf("clicked %s", selector),
	}
}

func (b *BrowserExecutor) typeText(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	selector, _ := action.Payload["selector"].(string)
	text, _ := action.Payload["text"].(string)
	if selector == "" {
		return ErrorResult(action.RequestID, "selector is required", "missing selector")
	}
	if text == "" {
		return ErrorResult(action.RequestID, "text is required", "missing text")
	}

	taskCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	err := chromedp.Run(taskCtx,
		chromedp.WaitVisible(selector),
		chromedp.Clear(selector),
		chromedp.SendKeys(selector, text),
	)
	if err != nil {
		return ErrorResult(action.RequestID, "type failed: "+err.Error(), "type failed")
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  "typed",
		Summary: fmt.Sprintf("typed %d chars into %s", len(text), selector),
	}
}

func (b *BrowserExecutor) screenshot(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	fullPage, _ := action.Payload["full_page"].(bool)

	taskCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var buf []byte
	var err error

	if fullPage {
		err = chromedp.Run(taskCtx, chromedp.FullScreenshot(&buf, 90))
	} else {
		err = chromedp.Run(taskCtx, chromedp.CaptureScreenshot(&buf))
	}
	if err != nil {
		return ErrorResult(action.RequestID, "screenshot failed: "+err.Error(), "screenshot failed")
	}

	encoded := base64.StdEncoding.EncodeToString(buf)
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("data:image/png;base64,%s", encoded),
		Summary: fmt.Sprintf("screenshot taken (%d bytes)", len(buf)),
	}
}

// ensureSession lazily starts the headless browser via chromedp.
func (b *BrowserExecutor) ensureSession(ctx context.Context) (context.Context, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		b.lastUsed = time.Now()
		return b.ctx, nil
	}

	// Build chromedp allocator options.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	// Set the browser executable path.
	// For Flatpak browsers, we need the real binary path inside the Flatpak.
	// chromedp can only use a direct executable path, not "flatpak run ...".
	browserBin := b.resolveBrowserBinary()
	if browserBin != "" {
		opts = append(opts, chromedp.ExecPath(browserBin))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	bctx, bcancel := chromedp.NewContext(allocCtx)

	// Test the connection with a blank page.
	if err := chromedp.Run(bctx, chromedp.Navigate("about:blank")); err != nil {
		bcancel()
		allocCancel()
		return nil, fmt.Errorf("browser startup: %w", err)
	}

	b.ctx = bctx
	b.cancel = func() {
		bcancel()
		allocCancel()
	}
	b.running = true
	b.lastUsed = time.Now()

	go b.idleShutdown()

	if b.log != nil {
		b.log.Info("browser_session_started", "path", browserBin)
	}

	return bctx, nil
}

// resolveBrowserBinary converts the detected browser path into a direct
// executable path that chromedp can use. For Flatpak browsers, it creates
// a wrapper script since the binary can't run outside the Flatpak sandbox.
func (b *BrowserExecutor) resolveBrowserBinary() string {
	path := b.browserPath

	// Flatpak browsers need a wrapper script — the binary inside the
	// Flatpak can't run directly (missing runtime libraries like cobalt).
	if strings.HasPrefix(path, "flatpak run ") {
		wrapper, err := createFlatpakWrapper(path)
		if err != nil {
			return ""
		}
		return wrapper
	}

	// For multi-word paths, take the first token.
	parts := strings.Fields(path)
	if len(parts) > 1 {
		return parts[0]
	}

	if resolved, err := exec.LookPath(path); err == nil {
		return resolved
	}
	return path
}

// createFlatpakWrapper writes a temp shell script that delegates to flatpak run.
// chromedp needs a single executable path; this wrapper provides that.
func createFlatpakWrapper(flatpakCmd string) (string, error) {
	f, err := os.CreateTemp("", "openparallax-browser-*.sh")
	if err != nil {
		return "", err
	}
	script := fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", flatpakCmd)
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		return "", err
	}
	_ = f.Close()
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// idleShutdown closes the browser after 5 minutes of inactivity.
func (b *BrowserExecutor) idleShutdown() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		b.mu.Lock()
		if !b.running {
			b.mu.Unlock()
			return
		}
		if time.Since(b.lastUsed) > 5*time.Minute {
			if b.log != nil {
				b.log.Info("browser_session_idle_shutdown")
			}
			b.cancel()
			b.running = false
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()
	}
}

func truncateContent(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n\n[Content truncated]"
}

// DetectBrowser finds an installed Chromium-based browser.
// Checks system PATH, absolute paths, Flatpak, and Snap installations.
func DetectBrowser() string {
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
