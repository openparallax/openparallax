// Package cli implements the interactive terminal channel adapter using bubbletea.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/storage"
	pb "github.com/openparallax/openparallax/internal/types/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Adapter is the CLI channel adapter that provides an interactive terminal.
type Adapter struct {
	grpcAddr  string
	agentName string
	workspace string
}

// New creates a CLI adapter connected to the engine at the given gRPC address.
func New(grpcAddr, agentName, workspace string) *Adapter {
	return &Adapter{
		grpcAddr:  grpcAddr,
		agentName: agentName,
		workspace: workspace,
	}
}

// Run starts the interactive terminal UI. Blocks until the user exits.
func (a *Adapter) Run(ctx context.Context) error {
	conn, err := grpc.NewClient(a.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to engine: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewPipelineServiceClient(conn)

	// Read-only DB connection for session listing and history loading.
	// The engine handles all writes. This avoids SQLite write lock conflicts.
	dbPath := fmt.Sprintf("%s/.openparallax/openparallax.db", a.workspace)
	db, _ := storage.Open(dbPath)

	sessionID := crypto.NewID()

	m := newModel(ctx, client, db, sessionID, a.agentName)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p

	_, err = p.Run()
	return err
}

// model is the bubbletea model for the CLI.
type model struct {
	client    pb.PipelineServiceClient
	db        *storage.DB
	sessionID string
	otrMode   bool
	agentName string
	ctx       context.Context
	program   *tea.Program

	spinner       spinner.Model
	viewport      viewport.Model
	lines         []styledLine
	thoughts      []string
	input         textarea.Model
	stream        string
	thinking      bool
	quitting      bool
	tabCycleIndex int
	ready         bool
	width         int
	height        int
}

// styledLine is a pre-rendered line in the chat history.
type styledLine struct {
	text string
}

// Bubbletea messages for the update loop.
type (
	tokenMsg   string
	doneMsg    string
	thoughtMsg string
	errMsg     struct{ err error }
	newSession struct{}
)

func newModel(ctx context.Context, client pb.PipelineServiceClient, db *storage.DB, sessionID, agentName string) *model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(1)
	ta.CharLimit = 0

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	return &model{
		client:    client,
		db:        db,
		sessionID: sessionID,
		agentName: agentName,
		ctx:       ctx,
		input:     ta,
		spinner:   s,
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.triggerShutdown()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyTab:
			if !m.thinking {
				text := strings.TrimSpace(m.input.Value())
				if strings.HasPrefix(text, "/") && !strings.Contains(text, " ") {
					completed := m.tabComplete(text)
					if completed != text {
						m.input.SetValue(completed)
						m.input.CursorEnd()
						return m, nil
					}
				}
			}
		case tea.KeyEnter:
			if m.thinking {
				break
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				break
			}
			m.input.Reset()

			lower := strings.ToLower(text)
			switch {
			case lower == "/quit" || lower == "/exit":
				m.triggerShutdown()
				m.quitting = true
				return m, tea.Quit
			case lower == "/new":
				return m, m.handleNewSession()
			case lower == "/otr":
				return m, m.handleOTRToggle()
			case lower == "/sessions":
				m.handleListSessions()
				m.syncViewport()
				return m, nil
			case strings.HasPrefix(lower, "/switch "):
				id := strings.TrimSpace(text[8:])
				m.handleSwitchSession(id)
				m.syncViewport()
				return m, nil
			case lower == "/help":
				m.handleHelp()
				m.syncViewport()
				return m, nil
			case lower == "/clear":
				m.lines = nil
				m.addLine(m.dimStyle("Chat cleared. History is preserved."))
				m.syncViewport()
				return m, nil
			case lower == "/status":
				m.handleStatus()
				m.syncViewport()
				return m, nil
			case lower == "/delete":
				m.handleDeleteSession()
				m.syncViewport()
				return m, nil
			case lower == "/restart":
				m.addLine(m.dimStyle("Restarting engine..."))
				m.syncViewport()
				return m, nil
			case lower == "/export":
				m.handleExport()
				m.syncViewport()
				return m, nil
			case strings.HasPrefix(lower, "/"):
				m.addLine(m.errStyle(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", lower)))
				m.syncViewport()
				return m, nil
			}

			prompt := m.userStyle("You: ")
			if m.otrMode {
				prompt = m.otrStyle("[OTR] You: ")
			}
			m.addLine(prompt + text)
			m.syncViewport()
			m.thinking = true
			m.stream = ""
			m.thoughts = nil
			m.startStreaming(text)
			return m, nil
		}

	case tokenMsg:
		m.stream += string(msg)
		m.syncViewport()
		return m, nil

	case thoughtMsg:
		m.thoughts = append(m.thoughts, string(msg))
		m.syncViewport()
		return m, nil

	case doneMsg:
		m.thinking = false
		if len(m.thoughts) > 0 {
			for _, t := range m.thoughts {
				m.addLine(m.dimStyle("  " + t))
			}
		}
		content := string(msg)
		if content == "" {
			content = m.stream
		}
		m.addLine(m.assistantStyle(m.agentName+": ") + content)
		m.stream = ""
		m.thoughts = nil
		m.syncViewport()
		return m, nil

	case errMsg:
		m.thinking = false
		m.addLine(m.errStyle("Error: " + msg.err.Error()))
		m.syncViewport()
		return m, nil

	case newSession:
		m.sessionID = crypto.NewID()
		label := "--- New session ---"
		if m.otrMode {
			label = "--- New OTR session (read-only, nothing persists) ---"
		}
		m.lines = nil
		m.stream = ""
		m.thoughts = nil
		m.addLine(m.dimStyle(label))
		m.syncViewport()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(m.width - 4)

		// Header (2 lines) + input area (3 lines) = 5 reserved.
		vpHeight := m.height - 5
		if vpHeight < 1 {
			vpHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.viewport.MouseWheelEnabled = true
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
		}
		m.syncViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Forward to viewport for scrolling (arrow keys, page up/down, mouse wheel).
	if m.ready {
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	if !m.thinking {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	// Header.
	sb.WriteString(m.titleStyle(fmt.Sprintf(" %s ", m.agentName)))
	if m.otrMode {
		sb.WriteString(m.otrStyle(" [OTR] "))
	}
	sb.WriteString(m.dimStyle("  /help for commands"))
	sb.WriteString("\n")

	// Scrollable viewport with chat content.
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n")

	// Input line or spinner — fixed below viewport.
	if m.thinking {
		if m.stream == "" && len(m.thoughts) == 0 {
			sb.WriteString(m.spinner.View())
			sb.WriteString(" Thinking...")
		} else {
			sb.WriteString(m.dimStyle("  (streaming...)"))
		}
	} else {
		sb.WriteString(m.input.View())
	}

	return sb.String()
}

// syncViewport rebuilds the viewport content from chat history + active stream + input.
func (m *model) syncViewport() {
	if !m.ready {
		return
	}

	var sb strings.Builder

	for _, l := range m.lines {
		sb.WriteString(l.text)
		sb.WriteString("\n\n")
	}

	if m.thinking && len(m.thoughts) > 0 {
		for _, t := range m.thoughts {
			sb.WriteString(m.dimStyle("  " + t))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if m.thinking && m.stream != "" {
		sb.WriteString(m.assistantStyle(m.agentName+": ") + m.stream)
		sb.WriteString("\n\n")
	}

	content := sb.String()
	m.viewport.SetContent(content)
	contentLines := strings.Count(content, "\n")
	if contentLines >= m.viewport.Height {
		m.viewport.GotoBottom()
	}
}

func (m *model) addLine(text string) {
	m.lines = append(m.lines, styledLine{text: text})
}

// Styles.
func (m *model) titleStyle(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render(s)
}

func (m *model) userStyle(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4")).Render(s)
}

func (m *model) assistantStyle(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")).Render(s)
}

func (m *model) dimStyle(s string) string {
	return lipgloss.NewStyle().Faint(true).Render(s)
}

func (m *model) errStyle(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(s)
}

func (m *model) otrStyle(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).Render(s)
}

// startStreaming launches a goroutine that reads gRPC events and pushes them
// into the bubbletea update loop via p.Send().
func (m *model) startStreaming(text string) {
	go func() {
		msgID := crypto.NewID()

		mode := pb.SessionMode_NORMAL
		if m.otrMode {
			mode = pb.SessionMode_OTR
		}

		stream, err := m.client.ProcessMessage(m.ctx, &pb.ProcessMessageRequest{
			Content:   text,
			SessionId: m.sessionID,
			MessageId: msgID,
			Mode:      mode,
			Source:    "cli",
		})
		if err != nil {
			m.program.Send(errMsg{err: fmt.Errorf("gRPC call failed: %w", err)})
			return
		}

		var fullText string
		for {
			event, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				m.program.Send(errMsg{err: fmt.Errorf("stream error: %w", recvErr)})
				return
			}

			switch event.EventType {
			case pb.PipelineEventType_LLM_TOKEN:
				if event.LlmToken != nil {
					fullText += event.LlmToken.Text
					m.program.Send(tokenMsg(event.LlmToken.Text))
				}
			case pb.PipelineEventType_RESPONSE_COMPLETE:
				if event.ResponseComplete != nil {
					fullText = event.ResponseComplete.Content
				}
			case pb.PipelineEventType_ACTION_STARTED:
				if event.ActionStarted != nil {
					m.program.Send(thoughtMsg(fmt.Sprintf("\U0001f527 %s", event.ActionStarted.Summary)))
				}
			case pb.PipelineEventType_SHIELD_VERDICT:
				if event.ShieldVerdict != nil {
					decision := event.ShieldVerdict.Decision.String()
					mark := "\u2192"
					if decision == "BLOCK" {
						mark = "\u2717"
					}
					m.program.Send(thoughtMsg(fmt.Sprintf("%s Shield: %s (Tier %d)", mark, decision, event.ShieldVerdict.Tier)))
				}
			case pb.PipelineEventType_ACTION_COMPLETED:
				if event.ActionCompleted != nil {
					mark := "\u2713"
					if !event.ActionCompleted.Success {
						mark = "\u2717"
					}
					m.program.Send(thoughtMsg(fmt.Sprintf("%s %s", mark, event.ActionCompleted.Summary)))
				}
			case pb.PipelineEventType_OTR_BLOCKED:
				if event.OtrBlocked != nil {
					m.program.Send(thoughtMsg(fmt.Sprintf("\u2717 OTR: %s", event.OtrBlocked.Reason)))
				}
			case pb.PipelineEventType_ERROR:
				if event.PipelineError != nil {
					m.program.Send(errMsg{err: fmt.Errorf("%s: %s", event.PipelineError.Code, event.PipelineError.Message)})
					return
				}
			}
		}

		m.program.Send(doneMsg(fullText))
	}()
}

// handleOTRToggle switches between Normal and OTR mode by creating a new session.
func (m *model) handleOTRToggle() tea.Cmd {
	return func() tea.Msg {
		m.otrMode = !m.otrMode
		return newSession{}
	}
}

// handleListSessions shows available Normal sessions.
func (m *model) handleListSessions() {
	if m.db == nil {
		m.addLine(m.dimStyle("Session listing unavailable."))
		return
	}
	sessions, err := m.db.ListSessions()
	if err != nil || len(sessions) == 0 {
		m.addLine(m.dimStyle("No sessions found."))
		return
	}
	m.addLine(m.dimStyle("--- Sessions ---"))
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		line := fmt.Sprintf("  %s  %s", s.ID[:8], title)
		if s.ID == m.sessionID {
			line += " (current)"
		}
		m.addLine(m.dimStyle(line))
	}
}

// handleSwitchSession switches to a different session by ID prefix.
func (m *model) handleSwitchSession(id string) {
	if m.db == nil {
		m.addLine(m.errStyle("Session switching unavailable."))
		return
	}
	sessions, err := m.db.ListSessions()
	if err != nil {
		m.addLine(m.errStyle("Error: " + err.Error()))
		return
	}
	var fullID string
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, id) {
			fullID = s.ID
			break
		}
	}
	if fullID == "" {
		m.addLine(m.errStyle("Session not found: " + id))
		return
	}

	m.sessionID = fullID
	m.otrMode = false
	m.lines = nil
	m.stream = ""
	m.thoughts = nil

	messages, _ := m.db.GetMessages(fullID)
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			m.addLine(m.userStyle("You: ") + msg.Content)
		case "assistant":
			m.addLine(m.assistantStyle(m.agentName+": ") + msg.Content)
		}
	}
	m.addLine(m.dimStyle(fmt.Sprintf("--- Switched to session %s ---", fullID[:8])))
}

// handleHelp displays all available slash commands.
func (m *model) handleHelp() {
	m.addLine(m.dimStyle("Available commands:"))
	cmds := []struct{ cmd, desc string }{
		{"/new", "Start a new session"},
		{"/otr", "Start a new Off The Record session"},
		{"/quit", "End current session and exit"},
		{"/clear", "Clear the chat view (history preserved)"},
		{"/status", "Show system health and session stats"},
		{"/export", "Export this session as markdown"},
		{"/delete", "Delete the current session"},
		{"/sessions", "List sessions"},
		{"/switch <id>", "Switch to a session by ID prefix"},
		{"/restart", "Restart the engine"},
		{"/help", "Show this help message"},
	}
	for _, c := range cmds {
		m.addLine(m.dimStyle(fmt.Sprintf("  %-16s %s", c.cmd, c.desc)))
	}
}

// handleStatus shows system health from the engine's gRPC status.
func (m *model) handleStatus() {
	ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
	defer cancel()

	resp, err := m.client.GetStatus(ctx, &pb.StatusRequest{})
	if err != nil {
		m.addLine(m.errStyle("Failed to fetch status: " + err.Error()))
		return
	}

	m.addLine(m.dimStyle("--- System Status ---"))
	m.addLine(m.dimStyle(fmt.Sprintf("  Agent:     %s", resp.AgentName)))
	m.addLine(m.dimStyle(fmt.Sprintf("  Model:     %s", resp.Model)))
	m.addLine(m.dimStyle(fmt.Sprintf("  Sessions:  %d", resp.SessionCount)))
	mode := "Normal"
	if m.otrMode {
		mode = "OTR (read-only)"
	}
	m.addLine(m.dimStyle(fmt.Sprintf("  Mode:      %s", mode)))
}

// handleDeleteSession removes the current session.
func (m *model) handleDeleteSession() {
	if m.db == nil {
		m.addLine(m.errStyle("Delete unavailable: no database connection."))
		return
	}
	if err := m.db.DeleteSession(m.sessionID); err != nil {
		m.addLine(m.errStyle("Failed to delete session: " + err.Error()))
		return
	}
	m.addLine(m.dimStyle("Session deleted."))
	m.sessionID = crypto.NewID()
	m.lines = nil
	m.addLine(m.dimStyle("--- New session ---"))
}

// handleExport writes the current session messages to a markdown file.
func (m *model) handleExport() {
	if m.db == nil {
		m.addLine(m.errStyle("Export unavailable: no database connection."))
		return
	}
	messages, err := m.db.GetMessages(m.sessionID)
	if err != nil || len(messages) == 0 {
		m.addLine(m.errStyle("No messages to export."))
		return
	}

	now := time.Now()
	filename := fmt.Sprintf("session-export-%s.md", now.Format("2006-01-02"))

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Session Export\n*Exported on %s*\n\n---\n\n", now.Format("2006-01-02 15:04"))

	for _, msg := range messages {
		who := "**You**"
		if msg.Role == "assistant" {
			who = fmt.Sprintf("**%s**", m.agentName)
		}
		ts := msg.Timestamp.Format("15:04")
		fmt.Fprintf(&sb, "%s (%s):\n%s\n\n---\n\n", who, ts, msg.Content)
	}

	path := filename
	if writeErr := writeExportFile(path, sb.String()); writeErr != nil {
		m.addLine(m.errStyle("Failed to write export: " + writeErr.Error()))
		return
	}
	m.addLine(m.dimStyle(fmt.Sprintf("Session exported to %s", path)))
}

// triggerShutdown calls the engine's Shutdown RPC which summarizes active
// sessions and persists memory. Waits up to 5 seconds.
func (m *model) triggerShutdown() {
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()
	_, _ = m.client.Shutdown(ctx, &pb.ShutdownRequest{})
}

// handleNewSession starts a fresh session.
func (m *model) handleNewSession() tea.Cmd {
	return func() tea.Msg {
		return newSession{}
	}
}

var slashCommands = []string{
	"/help", "/new", "/otr", "/quit", "/clear", "/status",
	"/export", "/delete", "/restart", "/sessions", "/switch",
}

func (m *model) tabComplete(prefix string) string {
	lower := strings.ToLower(prefix)
	var matches []string
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd, lower) {
			matches = append(matches, cmd)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		idx := (m.tabCycleIndex) % len(matches)
		m.tabCycleIndex++
		return matches[idx]
	}
	return prefix
}

func writeExportFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
