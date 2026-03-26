// Package cli implements the interactive terminal channel adapter using bubbletea.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
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

	dbPath := fmt.Sprintf("%s/.openparallax/openparallax.db", a.workspace)
	db, err := storage.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	sessionID := crypto.NewID()
	_ = db.InsertSession(&types.Session{
		ID:        sessionID,
		Mode:      types.SessionNormal,
		CreatedAt: time.Now(),
	})

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
	agentName string
	ctx       context.Context
	program   *tea.Program

	viewport viewport.Model
	spinner  spinner.Model
	lines    []styledLine
	thoughts []string
	stream   string
	thinking bool
	inputBuf string
	quitting bool
	ready    bool
	width    int
	height   int
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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	return &model{
		client:    client,
		db:        db,
		sessionID: sessionID,
		agentName: agentName,
		ctx:       ctx,
		spinner:   s,
	}
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.thinking {
			if msg.Type == tea.KeyCtrlC {
				m.quitting = true
				return m, tea.Quit
			}
			break
		}
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(m.inputBuf)
			m.inputBuf = ""
			if text == "" {
				break
			}

			switch strings.ToLower(text) {
			case "/quit", "/exit":
				m.quitting = true
				return m, tea.Quit
			case "/new":
				return m, m.handleNewSession()
			}

			m.addLine(m.userStyle("You: ") + text)
			m.thinking = true
			m.stream = ""
			m.thoughts = nil
			m.startStreaming(text)
			return m, nil
		case tea.KeyBackspace:
			if len(m.inputBuf) > 0 {
				m.inputBuf = m.inputBuf[:len(m.inputBuf)-1]
			}
		case tea.KeyRunes:
			m.inputBuf += string(msg.Runes)
		case tea.KeySpace:
			m.inputBuf += " "
		}

	case tokenMsg:
		m.stream += string(msg)
		m.updateViewport()
		return m, nil

	case thoughtMsg:
		m.thoughts = append(m.thoughts, string(msg))
		m.updateViewport()
		return m, nil

	case doneMsg:
		m.thinking = false
		content := string(msg)
		if content == "" {
			content = m.stream
		}
		m.addLine(m.assistantStyle(m.agentName+": ") + content)
		m.stream = ""
		m.thoughts = nil
		m.updateViewport()
		return m, nil

	case errMsg:
		m.thinking = false
		m.addLine(m.errStyle("Error: " + msg.err.Error()))
		m.updateViewport()
		return m, nil

	case newSession:
		m.sessionID = crypto.NewID()
		_ = m.db.InsertSession(&types.Session{
			ID:        m.sessionID,
			Mode:      types.SessionNormal,
			CreatedAt: time.Now(),
		})
		m.lines = nil
		m.stream = ""
		m.thoughts = nil
		m.addLine(m.dimStyle("--- New session ---"))
		m.updateViewport()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 2
		inputHeight := 2
		vpHeight := m.height - headerHeight - inputHeight
		if vpHeight < 1 {
			vpHeight = 1
		}
		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
		}
		m.updateViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "Initializing...\n"
	}

	var sb strings.Builder

	// Header.
	sb.WriteString(m.titleStyle(fmt.Sprintf(" %s ", m.agentName)))
	sb.WriteString(m.dimStyle("  /quit to exit, /new for new session"))
	sb.WriteString("\n")

	// Viewport with chat history.
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n")

	// Input line.
	if m.thinking {
		sb.WriteString(m.spinner.View())
		sb.WriteString(" ")
	} else {
		sb.WriteString("> ")
	}
	sb.WriteString(m.inputBuf)

	return sb.String()
}

func (m *model) addLine(text string) {
	m.lines = append(m.lines, styledLine{text: text})
}

func (m *model) updateViewport() {
	if !m.ready {
		return
	}
	var sb strings.Builder
	for _, l := range m.lines {
		sb.WriteString(l.text)
		sb.WriteString("\n\n")
	}

	// Active thinking steps.
	if m.thinking && len(m.thoughts) > 0 {
		for _, t := range m.thoughts {
			sb.WriteString(m.dimStyle("  " + t))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Active stream.
	if m.thinking && m.stream != "" {
		sb.WriteString(m.assistantStyle(m.agentName+": ") + m.stream)
		sb.WriteString("\n")
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
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

// startStreaming launches a goroutine that reads gRPC events and pushes them
// into the bubbletea update loop via p.Send().
func (m *model) startStreaming(text string) {
	go func() {
		msgID := crypto.NewID()

		stream, err := m.client.ProcessMessage(m.ctx, &pb.ProcessMessageRequest{
			Content:   text,
			SessionId: m.sessionID,
			MessageId: msgID,
			Mode:      pb.SessionMode_NORMAL,
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
			case pb.PipelineEventType_INTENT_PARSED:
				if event.IntentParsed != nil {
					m.program.Send(thoughtMsg(fmt.Sprintf("Understanding: %s", event.IntentParsed.Goal)))
				}
			case pb.PipelineEventType_ACTIONS_PLANNED:
				if event.ActionsPlanned != nil && event.ActionsPlanned.Count > 0 {
					m.program.Send(thoughtMsg(fmt.Sprintf("Planning %d action(s)", event.ActionsPlanned.Count)))
				}
			case pb.PipelineEventType_SELF_EVAL_PASSED:
				if event.SelfEvalResult != nil && event.SelfEvalResult.Passed {
					m.program.Send(thoughtMsg("Safety check: passed"))
				}
			case pb.PipelineEventType_SHIELD_VERDICT:
				if event.ShieldVerdict != nil {
					decision := event.ShieldVerdict.Decision.String()
					m.program.Send(thoughtMsg(fmt.Sprintf("Shield: %s (Tier %d)", decision, event.ShieldVerdict.Tier)))
				}
			case pb.PipelineEventType_ACTION_STARTED:
				if event.ActionStarted != nil {
					m.program.Send(thoughtMsg(fmt.Sprintf("\u25b8 %s", event.ActionStarted.Summary)))
				}
			case pb.PipelineEventType_ACTION_COMPLETED:
				if event.ActionCompleted != nil {
					mark := "\u2713"
					if !event.ActionCompleted.Success {
						mark = "\u2717"
					}
					m.program.Send(thoughtMsg(fmt.Sprintf("%s %s", mark, event.ActionCompleted.Summary)))
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

// handleNewSession starts a fresh session.
func (m *model) handleNewSession() tea.Cmd {
	return func() tea.Msg {
		return newSession{}
	}
}
