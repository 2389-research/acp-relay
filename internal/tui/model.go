// ABOUTME: Core Bubbletea model and state management for the TUI
// ABOUTME: Implements the Model interface with Init, Update, and View methods
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/components"
	"github.com/harper/acp-relay/internal/tui/config"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// FocusArea represents which component currently has focus
type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusChatView
	FocusInputArea
)

type Model struct {
	config *config.Config
	theme  theme.Theme
	width  int
	height int

	// Components
	sidebar     *components.Sidebar
	chatView    *components.ChatView
	inputArea   *components.InputArea
	statusBar   *components.StatusBar
	helpOverlay *components.HelpOverlay

	// Data managers
	relayClient    *client.RelayClient
	sessionManager *client.SessionManager
	messageStore   *client.MessageStore

	// UI state
	focusedArea     FocusArea
	activeSessionID string
	sidebarVisible  bool
}

func NewModel(cfg *config.Config) Model {
	th := theme.GetTheme(cfg.UI.Theme, nil)

	// Initialize components with default dimensions (will be resized on first WindowSizeMsg)
	sidebar := components.NewSidebar(30, 24, th)
	chatView := components.NewChatView(80, 20, th)
	inputArea := components.NewInputArea(80, 4, th)
	statusBar := components.NewStatusBar(80, th)
	helpOverlay := components.NewHelpOverlay(80, 24, th)

	// Initialize data managers
	relayClient := client.NewRelayClient(cfg.Relay.URL)
	sessionManager := client.NewSessionManager()
	messageStore := client.NewMessageStore(1000) // 1000 message history limit

	return Model{
		config:          cfg,
		theme:           th,
		sidebar:         sidebar,
		chatView:        chatView,
		inputArea:       inputArea,
		statusBar:       statusBar,
		helpOverlay:     helpOverlay,
		relayClient:     relayClient,
		sessionManager:  sessionManager,
		messageStore:    messageStore,
		focusedArea:     FocusSidebar,
		activeSessionID: "",
		sidebarVisible:  true,
	}
}

func (m Model) Init() tea.Cmd {
	// Initialize input area blinking cursor and connect to relay
	return tea.Batch(
		m.inputArea.Init(),
		m.connectToRelay(),
	)
}

// connectToRelay returns a command that connects to the relay server
func (m Model) connectToRelay() tea.Cmd {
	return func() tea.Msg {
		// Update status to connecting
		m.statusBar.SetConnectionStatus("connecting")

		if err := m.relayClient.Connect(); err != nil {
			return RelayErrorMsg{Err: err}
		}

		return RelayConnectedMsg{}
	}
}

// waitForRelayMessage returns a command that waits for the next relay message
func (m Model) waitForRelayMessage() tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-m.relayClient.Incoming():
			return RelayMessageMsg{Data: msg}
		case err := <-m.relayClient.Errors():
			return RelayErrorMsg{Err: err}
		}
	}
}
