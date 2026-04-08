package executors

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/platform"
)

const (
	clipboardMaxRead  = 50000
	clipboardMaxWrite = 1 << 20 // 1MB
	notifyRateLimit   = 5
	notifyRateWindow  = 60 * time.Second
)

var schemeRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// SystemExecutor provides clipboard, open, notify, system_info, and screenshot tools.
type SystemExecutor struct {
	workspacePath string
	notifyMu      sync.Mutex
	notifyTimes   []time.Time
}

// NewSystemExecutor creates a system executor.
func NewSystemExecutor(workspacePath string) *SystemExecutor {
	return &SystemExecutor{workspacePath: workspacePath}
}

// WorkspaceScope reports that system tools (clipboard, open, screenshot,
// notify) intentionally operate outside the workspace boundary.
func (s *SystemExecutor) WorkspaceScope() WorkspaceScope { return ScopeUnscoped }

// SupportedActions returns system tool action types.
func (s *SystemExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionClipboardRead, types.ActionClipboardWrite,
		types.ActionOpen, types.ActionNotify,
		types.ActionSystemInfo, types.ActionScreenshot,
	}
}

// ToolSchemas returns tool definitions for system tools.
func (s *SystemExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{ActionType: types.ActionClipboardRead, Name: "clipboard_read", Description: "Read the current contents of the system clipboard. Returns text content only.", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
		{ActionType: types.ActionClipboardWrite, Name: "clipboard_write", Description: "Write text to the system clipboard. Overwrites current clipboard contents.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"content": map[string]any{"type": "string", "description": "Text to write to the clipboard."}}, "required": []string{"content"}}},
		{ActionType: types.ActionOpen, Name: "open", Description: "Open a file or URL in the system's default application.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"target": map[string]any{"type": "string", "description": "Absolute file path or http(s) URL to open. Relative paths are rejected — Shield evaluates the literal target."}}, "required": []string{"target"}}},
		{ActionType: types.ActionNotify, Name: "notify", Description: "Send an OS notification with a title and message.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string", "description": "Notification title."}, "message": map[string]any{"type": "string", "description": "Notification body text."}}, "required": []string{"title", "message"}}},
		{ActionType: types.ActionSystemInfo, Name: "system_info", Description: "Get system information: disk usage, memory, CPU, processes, or network.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"category": map[string]any{"type": "string", "enum": []string{"disk", "memory", "cpu", "processes", "network", "all"}, "description": "Category of information. Default: all"}}}},
		{ActionType: types.ActionScreenshot, Name: "screenshot", Description: "Capture a screenshot of the desktop for visual analysis.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"display": map[string]any{"type": "integer", "description": "Display number for multi-monitor. Default: 0 (primary)."}}}},
	}
}

// Execute dispatches to the appropriate system tool.
func (s *SystemExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionClipboardRead:
		return s.clipboardRead(ctx, action)
	case types.ActionClipboardWrite:
		return s.clipboardWrite(ctx, action)
	case types.ActionOpen:
		return s.open(ctx, action)
	case types.ActionNotify:
		return s.notify(ctx, action)
	case types.ActionSystemInfo:
		return s.systemInfo(action)
	case types.ActionScreenshot:
		return s.screenshot(ctx, action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown system action"}
	}
}

// --- Clipboard ---

func (s *SystemExecutor) clipboardRead(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	cmd, err := platform.ClipboardReadCmd()
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
	}

	var out bytes.Buffer
	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	execCmd.Stdout = &out
	if runErr := execCmd.Run(); runErr != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("clipboard read failed: %s", runErr)}
	}

	content := out.String()
	if content == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "Clipboard is empty.", Summary: "empty clipboard"}
	}
	if len(content) > clipboardMaxRead {
		content = content[:clipboardMaxRead] + fmt.Sprintf("\n[Truncated — clipboard has %d characters]", len(out.String()))
	}

	return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: content, Summary: fmt.Sprintf("clipboard: %d chars", len(content))}
}

func (s *SystemExecutor) clipboardWrite(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	content, _ := action.Payload["content"].(string)
	if content == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "content is required"}
	}
	if len(content) > clipboardMaxWrite {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("content too large: %d bytes (max %d)", len(content), clipboardMaxWrite)}
	}

	cmd, err := platform.ClipboardWriteCmd()
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
	}

	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	execCmd.Stdin = strings.NewReader(content)
	if runErr := execCmd.Run(); runErr != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("clipboard write failed: %s", runErr)}
	}

	return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: fmt.Sprintf("Copied %d characters to clipboard.", len(content)), Summary: "copied to clipboard"}
}

// --- Open ---

func (s *SystemExecutor) open(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	target, _ := action.Payload["target"].(string)
	if target == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "target is required"}
	}

	// Validate target. URLs must be http(s); file paths must be
	// absolute. The cross-platform denylist enforced upstream by
	// CheckProtection blocks any path on the restricted set, so this
	// executor only needs to confirm the target is well-formed and
	// not a foreign URL scheme.
	switch {
	case strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://"):
		// URL — allowed.
	case schemeRegexp.MatchString(target):
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("unsupported scheme in %q — only http:// and https:// URLs are allowed", target)}
	default:
		if !platform.IsAbsolutePathSpec(target) {
			return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("path %q is relative — open requires an absolute path or an http(s) URL", target)}
		}
		target = platform.NormalizePath(target)
	}

	cmd := platform.OpenCmd(target)
	if cmd == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("open not supported on %s", runtime.GOOS)}
	}

	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	_ = execCmd.Start() // fire-and-forget

	name := target
	if len(name) > 60 {
		name = "..." + name[len(name)-57:]
	}
	return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: fmt.Sprintf("Opened %s", name), Summary: "opened"}
}

// --- Notify ---

func (s *SystemExecutor) notify(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	title, _ := action.Payload["title"].(string)
	message, _ := action.Payload["message"].(string)
	if title == "" || message == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "title and message are required"}
	}

	// Rate limit.
	s.notifyMu.Lock()
	now := time.Now()
	cutoff := now.Add(-notifyRateWindow)
	var recent []time.Time
	for _, t := range s.notifyTimes {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= notifyRateLimit {
		s.notifyMu.Unlock()
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("notification rate limit exceeded (%d per minute)", notifyRateLimit)}
	}
	recent = append(recent, now)
	s.notifyTimes = recent
	s.notifyMu.Unlock()

	cmd := platform.NotifyCmd(title, message)
	if cmd == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("notifications not supported on %s", runtime.GOOS)}
	}

	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	if err := execCmd.Run(); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("notification failed: %s", err)}
	}

	return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: fmt.Sprintf("Notification sent: %s", title), Summary: "notified"}
}

// --- System Info ---

func (s *SystemExecutor) systemInfo(action *types.ActionRequest) *types.ActionResult {
	category, _ := action.Payload["category"].(string)
	if category == "" {
		category = "all"
	}

	var sb strings.Builder
	sb.WriteString("System Information:\n\n")

	switch category {
	case "disk":
		sb.WriteString(diskInfo())
	case "memory":
		sb.WriteString(memoryInfo())
	case "cpu":
		sb.WriteString(cpuInfo())
	case "network":
		sb.WriteString(networkInfo())
	case "all":
		sb.WriteString(diskInfo())
		sb.WriteString("\n")
		sb.WriteString(memoryInfo())
		sb.WriteString("\n")
		sb.WriteString(cpuInfo())
		sb.WriteString("\n")
		sb.WriteString(networkInfo())
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("unknown category: %s", category)}
	}

	return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: sb.String(), Summary: fmt.Sprintf("system info: %s", category)}
}

func diskInfo() string {
	var sb strings.Builder
	sb.WriteString("Disk:\n")
	out, err := exec.Command("df", "-h").Output()
	if err != nil {
		sb.WriteString("  (unavailable)\n")
		return sb.String()
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fmt.Fprintf(&sb, "  %s\n", line)
		}
	}
	return sb.String()
}

func memoryInfo() string {
	var sb strings.Builder
	sb.WriteString("Memory:\n")
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(&sb, "  Go Heap: %.1f MB alloc, %.1f MB sys\n",
		float64(m.Alloc)/(1<<20), float64(m.Sys)/(1<<20))

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[:min(5, len(lines))] {
			if strings.TrimSpace(line) != "" {
				fmt.Fprintf(&sb, "  %s\n", strings.TrimSpace(line))
			}
		}
	}
	return sb.String()
}

func cpuInfo() string {
	var sb strings.Builder
	sb.WriteString("CPU:\n")
	fmt.Fprintf(&sb, "  Cores: %d\n", runtime.NumCPU())
	fmt.Fprintf(&sb, "  GOARCH: %s\n", runtime.GOARCH)

	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") {
				fmt.Fprintf(&sb, "  %s\n", strings.TrimSpace(line))
				break
			}
		}
	}
	return sb.String()
}

func networkInfo() string {
	var sb strings.Builder
	sb.WriteString("Network:\n")
	ifaces, err := net.Interfaces()
	if err != nil {
		sb.WriteString("  (unavailable)\n")
		return sb.String()
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		var addrStrs []string
		for _, addr := range addrs {
			addrStrs = append(addrStrs, addr.String())
		}
		status := "UP"
		if iface.Flags&net.FlagLoopback != 0 {
			status = "LOOPBACK"
		}
		fmt.Fprintf(&sb, "  %s: %s (%s)\n", iface.Name, strings.Join(addrStrs, ", "), status)
	}
	return sb.String()
}

// --- Screenshot ---

func (s *SystemExecutor) screenshot(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	tmpDir := filepath.Join(s.workspacePath, ".openparallax", "tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	screenshotPath := filepath.Join(tmpDir, fmt.Sprintf("screenshot-%d.png", time.Now().UnixMilli()))

	cmd := platform.ScreenshotCmd(screenshotPath)
	if cmd == nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "screenshot not available — no display server detected"}
	}

	execCmd := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	if err := execCmd.Run(); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("screenshot failed: %s", err)}
	}

	info, err := os.Stat(screenshotPath)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "screenshot file not created"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Screenshot saved to %s (%s)", screenshotPath, formatFileSize(info.Size())),
		Summary: "screenshot captured",
	}
}

func formatFileSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
