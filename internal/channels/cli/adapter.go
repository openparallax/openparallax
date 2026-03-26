// Package cli implements the interactive terminal channel adapter using bubbletea.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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

	// Create initial session.
	sessionID := crypto.NewID()
	_ = db.InsertSession(&types.Session{
		ID:        sessionID,
		Mode:      types.SessionNormal,
		CreatedAt: time.Now(),
	})

	m := newModel(client, db, sessionID, a.agentName, ctx)
	p := tea.NewProgram(m, tea.WithAltScreen())
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

	input    textarea.Model
	spinner  spinner.Model
	messages []chatMessage
	stream   string
	thinking bool
	err      error
	quitting bool
	width    int
	height   int
}

type chatMessage struct {
	role    string
	content string
}

// Bubbletea messages.
type (
	tokenMsg   string
	doneMsg    string
	errMsg     struct{ err error }
	newSession struct{}
)

func newModel(client pb.PipelineServiceClient, db *storage.DB, sessionID, agentName string, ctx context.Context) *model {
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
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.thinking {
				break
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				break
			}
			m.input.Reset()

			// Handle commands.
			switch strings.ToLower(text) {
			case "/quit", "/exit":
				m.quitting = true
				return m, tea.Quit
			case "/new":
				return m, m.handleNewSession()
			}

			m.messages = append(m.messages, chatMessage{role: "user", content: text})
			m.thinking = true
			m.stream = ""
			return m, m.sendMessage(text)
		}

	case tokenMsg:
		m.stream += string(msg)
		return m, nil

	case doneMsg:
		m.thinking = false
		m.messages = append(m.messages, chatMessage{role: "assistant", content: string(msg)})
		m.stream = ""
		return m, nil

	case errMsg:
		m.thinking = false
		m.err = msg.err
		return m, nil

	case newSession:
		m.sessionID = crypto.NewID()
		_ = m.db.InsertSession(&types.Session{
			ID:        m.sessionID,
			Mode:      types.SessionNormal,
			CreatedAt: time.Now(),
		})
		m.messages = nil
		m.stream = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
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
		return "Goodbye!\n"
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	userStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	assistantStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	dimStyle := lipgloss.NewStyle().Faint(true)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("  %s", m.agentName)))
	sb.WriteString(dimStyle.Render("  /quit to exit, /new for new session"))
	sb.WriteString("\n\n")

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			sb.WriteString(userStyle.Render("You: "))
			sb.WriteString(msg.content)
		case "assistant":
			sb.WriteString(assistantStyle.Render(m.agentName + ": "))
			sb.WriteString(msg.content)
		}
		sb.WriteString("\n\n")
	}

	if m.thinking {
		if m.stream != "" {
			sb.WriteString(assistantStyle.Render(m.agentName + ": "))
			sb.WriteString(m.stream)
			sb.WriteString("\n\n")
		} else {
			sb.WriteString(m.spinner.View())
			sb.WriteString(" Thinking...\n\n")
		}
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
		sb.WriteString(errStyle.Render(fmt.Sprintf("Error: %s", m.err)))
		sb.WriteString("\n\n")
		m.err = nil
	}

	sb.WriteString(m.input.View())

	return sb.String()
}

// sendMessage sends a user message to the engine via gRPC and streams the response.
func (m *model) sendMessage(text string) tea.Cmd {
	return func() tea.Msg {
		msgID := crypto.NewID()

		stream, err := m.client.ProcessMessage(m.ctx, &pb.ProcessMessageRequest{
			Content:   text,
			SessionId: m.sessionID,
			MessageId: msgID,
			Mode:      pb.SessionMode_NORMAL,
			Source:    "cli",
		})
		if err != nil {
			return errMsg{err: fmt.Errorf("gRPC call failed: %w", err)}
		}

		var fullText string
		for {
			event, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				return errMsg{err: fmt.Errorf("stream error: %w", recvErr)}
			}

			switch event.EventType {
			case pb.PipelineEventType_LLM_TOKEN:
				if event.LlmToken != nil {
					fullText += event.LlmToken.Text
					// We can't send tea.Msg from inside a Cmd easily for
					// streaming tokens. Instead, accumulate and return the
					// full response at the end.
				}
			case pb.PipelineEventType_RESPONSE_COMPLETE:
				if event.ResponseComplete != nil {
					fullText = event.ResponseComplete.Content
				}
			case pb.PipelineEventType_ERROR:
				if event.PipelineError != nil {
					return errMsg{err: fmt.Errorf("%s: %s", event.PipelineError.Code, event.PipelineError.Message)}
				}
			}
		}

		return doneMsg(fullText)
	}
}

// handleNewSession starts a fresh session.
func (m *model) handleNewSession() tea.Cmd {
	return func() tea.Msg {
		return newSession{}
	}
}
