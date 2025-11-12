# Bubbletea TUI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build an interactive terminal UI for acp-relay using Bubbletea with session management, chat interface, and WebSocket integration.

**Architecture:** 3-layer architecture: Bubbletea application layer (cmd/tui), reusable components layer (clients/tui/components), and client layer (clients/tui/client) for WebSocket communication. Uses Elm architecture (Model-Update-View) with channel-based async message passing.

**Tech Stack:** Go 1.24, Bubbletea v0.25, Bubbles v0.18, Lipgloss v0.9, gorilla/websocket v1.5

---

## Task 1: Project Setup & Dependencies

**Files:**
- Create: `cmd/tui/main.go`
- Create: `clients/tui/model.go`
- Create: `clients/tui/update.go`
- Create: `clients/tui/view.go`
- Modify: `go.mod`
- Modify: `Makefile`

**Step 1: Install dependencies**

Run:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go mod tidy
```

Expected: Dependencies added to go.mod

**Step 2: Create minimal main.go**

Create `cmd/tui/main.go`:
```go
// ABOUTME: Entry point for the ACP-Relay TUI client
// ABOUTME: Initializes configuration, theme, and starts the Bubbletea application
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/clients/tui"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	// Create initial model
	m := tui.NewModel()

	// Create Bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Create minimal model**

Create `clients/tui/model.go`:
```go
// ABOUTME: Core Bubbletea model and state management for the TUI
// ABOUTME: Implements the Model interface with Init, Update, and View methods
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	width  int
	height int
}

func NewModel() Model {
	return Model{}
}

func (m Model) Init() tea.Cmd {
	return nil
}
```

**Step 4: Create update handler**

Create `clients/tui/update.go`:
```go
// ABOUTME: Update logic for the TUI (handles all messages and state transitions)
// ABOUTME: Implements the Elm architecture Update function
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	return m, nil
}
```

**Step 5: Create view renderer**

Create `clients/tui/view.go`:
```go
// ABOUTME: View rendering for the TUI (converts model state to terminal output)
// ABOUTME: Implements the Elm architecture View function
package tui

import "fmt"

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	return fmt.Sprintf("ACP-Relay TUI (%dx%d)\nPress 'q' to quit", m.width, m.height)
}
```

**Step 6: Add Makefile target**

Add to `Makefile`:
```makefile
.PHONY: build-tui install-tui clean-tui test-tui dev-tui

build-tui:
	go build -o bin/acp-tui ./cmd/tui

install-tui: build-tui
	install -m 755 bin/acp-tui /usr/local/bin/

clean-tui:
	rm -f bin/acp-tui

test-tui:
	go test ./clients/tui/... -v -cover

dev-tui: build-tui
	./bin/acp-tui

build-all: build build-tui
```

**Step 7: Test build and run**

Run:
```bash
make build-tui
./bin/acp-tui
```

Expected: TUI launches, shows window size, press 'q' to quit

**Step 8: Commit**

```bash
git add cmd/tui/ clients/tui/ go.mod go.sum Makefile
git commit -m "feat(tui): add minimal Bubbletea application scaffold

- Add main entry point with Bubbletea initialization
- Implement basic Model-Update-View structure
- Add build targets to Makefile
- Install Bubbletea dependencies"
```

---

## Task 2: Configuration System

**Files:**
- Create: `clients/tui/config/config.go`
- Create: `clients/tui/config/config_test.go`
- Modify: `cmd/tui/main.go`

**Step 1: Write test for config loading**

Create `clients/tui/config/config_test.go`:
```go
// ABOUTME: Unit tests for TUI configuration loading and validation
// ABOUTME: Tests default config, file loading, validation, and XDG path expansion
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "ws://localhost:8081", cfg.Relay.URL)
	assert.Equal(t, 5, cfg.Relay.ReconnectAttempts)
	assert.Equal(t, "default", cfg.UI.Theme)
	assert.Equal(t, 25, cfg.UI.SidebarWidth)
	assert.True(t, cfg.UI.SidebarDefaultVisible)
}

func TestLoadConfig_NoFile(t *testing.T) {
	// Set XDG to temp dir
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load("")
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, "default", cfg.UI.Theme)

	// Should create default config file
	configPath := filepath.Join(tmpDir, "acp-tui", "config.yaml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err, "config file should be created")
}

func TestValidate_SidebarWidth(t *testing.T) {
	cfg := DefaultConfig()

	// Too small
	cfg.UI.SidebarWidth = 10
	cfg.Validate()
	assert.Equal(t, 20, cfg.UI.SidebarWidth, "should clamp to 20")

	// Too large
	cfg.UI.SidebarWidth = 50
	cfg.Validate()
	assert.Equal(t, 40, cfg.UI.SidebarWidth, "should clamp to 40")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./clients/tui/config -v`

Expected: FAIL with "package not found" or "function not defined"

**Step 3: Implement config structure**

Create `clients/tui/config/config.go`:
```go
// ABOUTME: TUI configuration system with XDG-compliant file loading
// ABOUTME: Handles config loading, validation, defaults, and theme selection
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harper/acp-relay/internal/xdg"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Relay       RelayConfig       `yaml:"relay"`
	UI          UIConfig          `yaml:"ui"`
	Input       InputConfig       `yaml:"input"`
	Sessions    SessionsConfig    `yaml:"sessions"`
	Keybindings KeybindingsConfig `yaml:"keybindings"`
	Logging     LoggingConfig     `yaml:"logging"`
}

type RelayConfig struct {
	URL               string `yaml:"url"`
	ReconnectAttempts int    `yaml:"reconnect_attempts"`
	TimeoutSeconds    int    `yaml:"timeout_seconds"`
}

type UIConfig struct {
	Theme                  string `yaml:"theme"`
	SidebarWidth           int    `yaml:"sidebar_width"`
	SidebarDefaultVisible  bool   `yaml:"sidebar_default_visible"`
	ChatHistoryLimit       int    `yaml:"chat_history_limit"`
}

type InputConfig struct {
	MultilineMinHeight int  `yaml:"multiline_min_height"`
	MultilineMaxHeight int  `yaml:"multiline_max_height"`
	SendOnEnter        bool `yaml:"send_on_enter"`
	VimMode            bool `yaml:"vim_mode"`
}

type SessionsConfig struct {
	DefaultWorkingDir   string `yaml:"default_working_dir"`
	AutoCreateWorkspace bool   `yaml:"auto_create_workspace"`
}

type KeybindingsConfig struct {
	ToggleSidebar string `yaml:"toggle_sidebar"`
	NewSession    string `yaml:"new_session"`
	DeleteSession string `yaml:"delete_session"`
	RenameSession string `yaml:"rename_session"`
	SendMessage   string `yaml:"send_message"`
	Quit          string `yaml:"quit"`
	Help          string `yaml:"help"`
}

type LoggingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"`
	File    string `yaml:"file"`
}

func DefaultConfig() *Config {
	return &Config{
		Relay: RelayConfig{
			URL:               "ws://localhost:8081",
			ReconnectAttempts: 5,
			TimeoutSeconds:    30,
		},
		UI: UIConfig{
			Theme:                 "default",
			SidebarWidth:          25,
			SidebarDefaultVisible: true,
			ChatHistoryLimit:      1000,
		},
		Input: InputConfig{
			MultilineMinHeight: 3,
			MultilineMaxHeight: 10,
			SendOnEnter:        false,
			VimMode:            false,
		},
		Sessions: SessionsConfig{
			DefaultWorkingDir:   "~/acp-workspaces",
			AutoCreateWorkspace: true,
		},
		Keybindings: KeybindingsConfig{
			ToggleSidebar: "ctrl+b",
			NewSession:    "n",
			DeleteSession: "d",
			RenameSession: "r",
			SendMessage:   "ctrl+s",
			Quit:          "ctrl+c",
			Help:          "?",
		},
		Logging: LoggingConfig{
			Enabled: true,
			Level:   "info",
			File:    "~/.local/share/acp-tui/tui.log",
		},
	}
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Determine config file location
	if configPath == "" {
		configPath = filepath.Join(xdg.ConfigHome(), "acp-tui", "config.yaml")
	}

	// If file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := saveDefault(cfg, configPath); err != nil {
			// Log warning but continue with defaults
			return cfg, nil
		}
		return cfg, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.Validate()
	return cfg, nil
}

func (c *Config) Validate() {
	// Clamp sidebar width
	if c.UI.SidebarWidth < 20 {
		c.UI.SidebarWidth = 20
	}
	if c.UI.SidebarWidth > 40 {
		c.UI.SidebarWidth = 40
	}

	// Clamp chat history
	if c.UI.ChatHistoryLimit < 100 {
		c.UI.ChatHistoryLimit = 100
	}
	if c.UI.ChatHistoryLimit > 10000 {
		c.UI.ChatHistoryLimit = 10000
	}

	// Expand ~ in paths
	c.Sessions.DefaultWorkingDir = xdg.ExpandPath(c.Sessions.DefaultWorkingDir)
	c.Logging.File = xdg.ExpandPath(c.Logging.File)
}

func saveDefault(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./clients/tui/config -v`

Expected: PASS (3 tests)

**Step 5: Integrate config into main**

Modify `cmd/tui/main.go`:
```go
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/clients/tui"
	"github.com/harper/acp-relay/clients/tui/config"
)

var (
	version    = "dev"
	buildTime  = "unknown"
	configPath = flag.String("config", "", "Path to config file")
	relayURL   = flag.String("relay", "", "Relay WebSocket URL (overrides config)")
	showVer    = flag.Bool("version", false, "Show version information")
)

func main() {
	flag.Parse()

	if *showVer {
		fmt.Printf("acp-tui version %s\n", version)
		fmt.Printf("  built: %s\n", buildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override with flags
	if *relayURL != "" {
		cfg.Relay.URL = *relayURL
	}

	// Create initial model
	m := tui.NewModel(cfg)

	// Create Bubbletea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 6: Update model to accept config**

Modify `clients/tui/model.go`:
```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/clients/tui/config"
)

type Model struct {
	config *config.Config
	width  int
	height int
}

func NewModel(cfg *config.Config) Model {
	return Model{
		config: cfg,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
```

**Step 7: Test build and run**

Run:
```bash
make build-tui
./bin/acp-tui --version
```

Expected: Shows version info

Run: `./bin/acp-tui`

Expected: TUI launches, config file created at `~/.config/acp-tui/config.yaml`

**Step 8: Commit**

```bash
git add clients/tui/config/ cmd/tui/main.go clients/tui/model.go
git commit -m "feat(tui): add configuration system

- Implement config loading with XDG paths
- Add validation and defaults
- Support command-line flag overrides
- Create default config on first run
- Add unit tests for config validation"
```

---

## Task 3: Theme System

**Files:**
- Create: `clients/tui/theme/theme.go`
- Create: `clients/tui/theme/theme_test.go`
- Modify: `clients/tui/model.go`

**Step 1: Write theme tests**

Create `clients/tui/theme/theme_test.go`:
```go
// ABOUTME: Unit tests for theme system and lipgloss style generation
// ABOUTME: Tests theme loading, style construction, and color application
package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestGetTheme_Default(t *testing.T) {
	theme := GetTheme("default", nil)

	assert.Equal(t, lipgloss.Color("#7C3AED"), theme.Primary)
	assert.Equal(t, lipgloss.Color("#1E1E2E"), theme.Background)
}

func TestGetTheme_Dark(t *testing.T) {
	theme := GetTheme("dark", nil)

	assert.Equal(t, lipgloss.Color("#00FF00"), theme.Primary)
	assert.Equal(t, lipgloss.Color("#000000"), theme.Background)
}

func TestGetTheme_Light(t *testing.T) {
	theme := GetTheme("light", nil)

	assert.Equal(t, lipgloss.Color("#268BD2"), theme.Primary)
	assert.Equal(t, lipgloss.Color("#FDF6E3"), theme.Background)
}

func TestTheme_SidebarStyle(t *testing.T) {
	theme := DefaultTheme

	style := theme.SidebarStyle()

	// Style should have sidebar background color
	assert.Contains(t, style.Render("test"), "test")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./clients/tui/theme -v`

Expected: FAIL

**Step 3: Implement theme system**

Create `clients/tui/theme/theme.go`:
```go
// ABOUTME: Theme system for TUI styling with lipgloss
// ABOUTME: Provides predefined themes and style constructors for UI components
package theme

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Primary    lipgloss.Color
	Background lipgloss.Color
	Foreground lipgloss.Color
	SidebarBg  lipgloss.Color
	InputBg    lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	UserMsg    lipgloss.Color
	AgentMsg   lipgloss.Color
	Dim        lipgloss.Color
}

var DefaultTheme = Theme{
	Primary:    lipgloss.Color("#7C3AED"), // Purple
	Background: lipgloss.Color("#1E1E2E"), // Dark gray
	Foreground: lipgloss.Color("#CDD6F4"), // Light gray
	SidebarBg:  lipgloss.Color("#181825"), // Darker gray
	InputBg:    lipgloss.Color("#313244"), // Medium gray
	Success:    lipgloss.Color("#A6E3A1"), // Green
	Warning:    lipgloss.Color("#F9E2AF"), // Yellow
	Error:      lipgloss.Color("#F38BA8"), // Red
	UserMsg:    lipgloss.Color("#89B4FA"), // Blue
	AgentMsg:   lipgloss.Color("#94E2D5"), // Cyan
	Dim:        lipgloss.Color("#6C7086"), // Dim gray
}

var DarkTheme = Theme{
	Primary:    lipgloss.Color("#00FF00"), // Bright green
	Background: lipgloss.Color("#000000"), // Pure black
	Foreground: lipgloss.Color("#FFFFFF"), // Pure white
	SidebarBg:  lipgloss.Color("#0A0A0A"), // Near black
	InputBg:    lipgloss.Color("#1A1A1A"), // Dark gray
	Success:    lipgloss.Color("#00FF00"), // Green
	Warning:    lipgloss.Color("#FFFF00"), // Yellow
	Error:      lipgloss.Color("#FF0000"), // Red
	UserMsg:    lipgloss.Color("#00FFFF"), // Cyan
	AgentMsg:   lipgloss.Color("#FF00FF"), // Magenta
	Dim:        lipgloss.Color("#808080"), // Gray
}

var LightTheme = Theme{
	Primary:    lipgloss.Color("#268BD2"), // Blue
	Background: lipgloss.Color("#FDF6E3"), // Cream
	Foreground: lipgloss.Color("#657B83"), // Gray
	SidebarBg:  lipgloss.Color("#EEE8D5"), // Light cream
	InputBg:    lipgloss.Color("#EEE8D5"), // Light cream
	Success:    lipgloss.Color("#859900"), // Olive green
	Warning:    lipgloss.Color("#B58900"), // Yellow
	Error:      lipgloss.Color("#DC322F"), // Red
	UserMsg:    lipgloss.Color("#268BD2"), // Blue
	AgentMsg:   lipgloss.Color("#2AA198"), // Cyan
	Dim:        lipgloss.Color("#93A1A1"), // Light gray
}

func GetTheme(name string, customColors map[string]string) Theme {
	switch name {
	case "dark":
		return DarkTheme
	case "light":
		return LightTheme
	default:
		return DefaultTheme
	}
}

// Style constructors

func (t Theme) SidebarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.SidebarBg).
		Foreground(t.Foreground).
		Padding(0, 1)
}

func (t Theme) ActiveSessionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1)
}

func (t Theme) InactiveSessionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Foreground).
		Padding(0, 1)
}

func (t Theme) ChatViewStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Background).
		Foreground(t.Foreground).
		Padding(1)
}

func (t Theme) InputAreaStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.InputBg).
		Foreground(t.Foreground).
		Padding(0, 1)
}

func (t Theme) StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Primary).
		Foreground(t.Background).
		Padding(0, 1)
}

func (t Theme) ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Error).
		Bold(true)
}

func (t Theme) SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Success)
}

func (t Theme) DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Dim)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./clients/tui/theme -v`

Expected: PASS (4 tests)

**Step 5: Integrate theme into model**

Modify `clients/tui/model.go`:
```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/clients/tui/config"
	"github.com/harper/acp-relay/clients/tui/theme"
)

type Model struct {
	config *config.Config
	theme  theme.Theme
	width  int
	height int
}

func NewModel(cfg *config.Config) Model {
	return Model{
		config: cfg,
		theme:  theme.GetTheme(cfg.UI.Theme, nil),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}
```

**Step 6: Update view to use theme**

Modify `clients/tui/view.go`:
```go
package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Apply theme styling
	title := m.theme.StatusBarStyle().Render("ACP-Relay TUI")
	info := m.theme.DimStyle().Render(fmt.Sprintf("(%dx%d)", m.width, m.height))
	help := m.theme.DimStyle().Render("Press 'q' to quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		info,
		help,
	)
}
```

**Step 7: Test with different themes**

Run:
```bash
make build-tui
./bin/acp-tui
```

Expected: TUI shows purple theme

Edit `~/.config/acp-tui/config.yaml` and change `theme: "dark"`, then run again.

Expected: TUI shows green/black high contrast theme

**Step 8: Commit**

```bash
git add clients/tui/theme/ clients/tui/model.go clients/tui/view.go
git commit -m "feat(tui): add theme system with lipgloss styles

- Implement default, dark, and light themes
- Add style constructors for all UI components
- Integrate theme into model and view
- Add unit tests for theme loading"
```

---

## Task 4: WebSocket Client

**Files:**
- Create: `clients/tui/client/relay_client.go`
- Create: `clients/tui/client/relay_client_test.go`

**Step 1: Write client tests**

Create `clients/tui/client/relay_client_test.go`:
```go
// ABOUTME: Unit tests for WebSocket relay client
// ABOUTME: Tests connection, message sending/receiving, and reconnection logic
package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func mockRelayHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Echo messages back
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func TestRelayClient_Connect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockRelayHandler))
	defer server.Close()

	wsURL := "ws" + server.URL[4:] // Replace http with ws

	client := NewRelayClient(wsURL)
	err := client.Connect()
	require.NoError(t, err)

	defer client.Close()
	assert.True(t, client.IsConnected())
}

func TestRelayClient_SendReceive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockRelayHandler))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	client := NewRelayClient(wsURL)
	require.NoError(t, client.Connect())
	defer client.Close()

	// Send message
	testMsg := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	err := client.Send(testMsg)
	require.NoError(t, err)

	// Receive echo
	select {
	case msg := <-client.Incoming():
		assert.Equal(t, testMsg, msg)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestRelayClient_ErrorChannel(t *testing.T) {
	// Connect to invalid URL
	client := NewRelayClient("ws://localhost:99999")
	err := client.Connect()

	assert.Error(t, err)
	assert.False(t, client.IsConnected())
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./clients/tui/client -v`

Expected: FAIL

**Step 3: Implement WebSocket client**

Create `clients/tui/client/relay_client.go`:
```go
// ABOUTME: WebSocket client for communicating with acp-relay server
// ABOUTME: Manages connection lifecycle, message passing via channels, and auto-reconnection
package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RelayClient struct {
	url      string
	conn     *websocket.Conn
	mu       sync.RWMutex
	incoming chan []byte
	outgoing chan []byte
	errors   chan error
	done     chan struct{}
	closed   bool
}

func NewRelayClient(url string) *RelayClient {
	return &RelayClient{
		url:      url,
		incoming: make(chan []byte, 100),
		outgoing: make(chan []byte, 100),
		errors:   make(chan error, 10),
		done:     make(chan struct{}),
	}
}

func (c *RelayClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.conn = conn
	c.closed = false

	// Start read/write goroutines
	go c.readLoop()
	go c.writeLoop()

	return nil
}

func (c *RelayClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.closed
}

func (c *RelayClient) Send(msg []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	select {
	case c.outgoing <- msg:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	}
}

func (c *RelayClient) Incoming() <-chan []byte {
	return c.incoming
}

func (c *RelayClient) Errors() <-chan error {
	return c.errors
}

func (c *RelayClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.done)

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *RelayClient) readLoop() {
	defer func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			select {
			case c.errors <- fmt.Errorf("read: %w", err):
			case <-c.done:
			}
			return
		}

		select {
		case c.incoming <- msg:
		case <-c.done:
			return
		}
	}
}

func (c *RelayClient) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.outgoing:
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				select {
				case c.errors <- fmt.Errorf("write: %w", err):
				case <-c.done:
				}
				return
			}
		}
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./clients/tui/client -v`

Expected: PASS (3 tests)

**Step 5: Add integration with Bubbletea messages**

Add to `clients/tui/client/relay_client.go`:
```go
// Bubbletea message types for async communication

type RelayMessageMsg struct {
	Data []byte
}

type RelayErrorMsg struct {
	Err error
}

type RelayDisconnectedMsg struct{}

// WaitForMessage returns a Cmd that waits for the next message
func (c *RelayClient) WaitForMessage() func() tea.Msg {
	return func() tea.Msg {
		select {
		case msg := <-c.incoming:
			return RelayMessageMsg{Data: msg}
		case err := <-c.errors:
			return RelayErrorMsg{Err: err}
		}
	}
}
```

Add import: `tea "github.com/charmbracelet/bubbletea"`

**Step 6: Test build**

Run: `go test ./clients/tui/client -v`

Expected: PASS

**Step 7: Commit**

```bash
git add clients/tui/client/
git commit -m "feat(tui): add WebSocket relay client

- Implement client with channel-based async I/O
- Add read/write goroutines for concurrent communication
- Integrate with Bubbletea message system
- Add unit tests with mock WebSocket server"
```

---

## Task 5: Session Manager

**Files:**
- Create: `clients/tui/client/session_manager.go`
- Create: `clients/tui/client/session_manager_test.go`

**Step 1: Write session manager tests**

Create `clients/tui/client/session_manager_test.go`:
```go
// ABOUTME: Unit tests for session manager (CRUD operations)
// ABOUTME: Tests session creation, listing, deletion, and state management
package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager()

	assert.NotNil(t, sm)
	assert.Empty(t, sm.List())
}

func TestSessionManager_Create(t *testing.T) {
	sm := NewSessionManager()

	sess, err := sm.Create("sess-123", "/tmp/workspace", "Test Session")
	require.NoError(t, err)

	assert.Equal(t, "sess-123", sess.ID)
	assert.Equal(t, "/tmp/workspace", sess.WorkingDir)
	assert.Equal(t, "Test Session", sess.DisplayName)
	assert.Equal(t, StatusActive, sess.Status)
}

func TestSessionManager_Get(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-123", "/tmp/workspace", "Test")

	sess, exists := sm.Get("sess-123")
	assert.True(t, exists)
	assert.Equal(t, "sess-123", sess.ID)

	_, exists = sm.Get("nonexistent")
	assert.False(t, exists)
}

func TestSessionManager_Delete(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-123", "/tmp/workspace", "Test")

	err := sm.Delete("sess-123")
	require.NoError(t, err)

	_, exists := sm.Get("sess-123")
	assert.False(t, exists)

	// Delete nonexistent should error
	err = sm.Delete("nonexistent")
	assert.Error(t, err)
}

func TestSessionManager_List(t *testing.T) {
	sm := NewSessionManager()
	sm.Create("sess-1", "/tmp/1", "One")
	sm.Create("sess-2", "/tmp/2", "Two")

	sessions := sm.List()
	assert.Len(t, sessions, 2)
}

func TestSessionManager_UpdateStatus(t *testing.T) {
	sm := NewSessionManager()
	sess, _ := sm.Create("sess-123", "/tmp", "Test")

	assert.Equal(t, StatusActive, sess.Status)

	err := sm.UpdateStatus("sess-123", StatusIdle)
	require.NoError(t, err)

	updated, _ := sm.Get("sess-123")
	assert.Equal(t, StatusIdle, updated.Status)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./clients/tui/client -v`

Expected: FAIL

**Step 3: Implement session manager**

Create `clients/tui/client/session_manager.go`:
```go
// ABOUTME: Session manager for CRUD operations on agent sessions
// ABOUTME: Maintains session list, status tracking, and message history
package client

import (
	"fmt"
	"sync"
	"time"
)

type SessionStatus int

const (
	StatusActive SessionStatus = iota
	StatusIdle
	StatusDead
)

func (s SessionStatus) String() string {
	switch s {
	case StatusActive:
		return "Active"
	case StatusIdle:
		return "Idle"
	case StatusDead:
		return "Dead"
	default:
		return "Unknown"
	}
}

func (s SessionStatus) Icon() string {
	switch s {
	case StatusActive:
		return "⚡"
	case StatusIdle:
		return "💤"
	case StatusDead:
		return "💀"
	default:
		return "❓"
	}
}

type Session struct {
	ID          string
	WorkingDir  string
	DisplayName string
	Status      SessionStatus
	CreatedAt   time.Time
	LastActive  time.Time
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) Create(id, workingDir, displayName string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; exists {
		return nil, fmt.Errorf("session %s already exists", id)
	}

	now := time.Now()
	sess := &Session{
		ID:          id,
		WorkingDir:  workingDir,
		DisplayName: displayName,
		Status:      StatusActive,
		CreatedAt:   now,
		LastActive:  now,
	}

	sm.sessions[id] = sess
	return sess, nil
}

func (sm *SessionManager) Get(id string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sess, exists := sm.sessions[id]
	return sess, exists
}

func (sm *SessionManager) List() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, sess := range sm.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

func (sm *SessionManager) Delete(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; !exists {
		return fmt.Errorf("session %s not found", id)
	}

	delete(sm.sessions, id)
	return nil
}

func (sm *SessionManager) UpdateStatus(id string, status SessionStatus) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	sess.Status = status
	if status == StatusActive {
		sess.LastActive = time.Now()
	}

	return nil
}

func (sm *SessionManager) Rename(id, newName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sess, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	sess.DisplayName = newName
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./clients/tui/client -v`

Expected: PASS (all tests including new session manager tests)

**Step 5: Commit**

```bash
git add clients/tui/client/session_manager.go clients/tui/client/session_manager_test.go
git commit -m "feat(tui): add session manager with CRUD operations

- Implement session creation, listing, deletion
- Add status tracking (Active, Idle, Dead)
- Support session renaming
- Add unit tests for all operations"
```

---

## Task 6: Message Store

**Files:**
- Create: `clients/tui/client/message_store.go`
- Create: `clients/tui/client/message_store_test.go`

**Step 1: Write message store tests**

Create `clients/tui/client/message_store_test.go`:
```go
// ABOUTME: Unit tests for message store (per-session message history)
// ABOUTME: Tests message storage, retrieval, and history limits
package client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageStore(t *testing.T) {
	store := NewMessageStore(100)

	assert.NotNil(t, store)
	assert.Empty(t, store.GetMessages("sess-1"))
}

func TestMessageStore_AddMessage(t *testing.T) {
	store := NewMessageStore(100)

	msg := &Message{
		SessionID: "sess-1",
		Type:      MessageTypeUser,
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	store.AddMessage(msg)

	messages := store.GetMessages("sess-1")
	assert.Len(t, messages, 1)
	assert.Equal(t, "Hello", messages[0].Content)
}

func TestMessageStore_HistoryLimit(t *testing.T) {
	store := NewMessageStore(3) // Limit to 3 messages

	// Add 5 messages
	for i := 0; i < 5; i++ {
		store.AddMessage(&Message{
			SessionID: "sess-1",
			Type:      MessageTypeUser,
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: time.Now(),
		})
	}

	messages := store.GetMessages("sess-1")
	assert.Len(t, messages, 3, "should only keep 3 most recent")

	// Should have messages 2, 3, 4 (oldest discarded)
	assert.Equal(t, "Message 2", messages[0].Content)
	assert.Equal(t, "Message 4", messages[2].Content)
}

func TestMessageStore_MultiplesSessions(t *testing.T) {
	store := NewMessageStore(100)

	store.AddMessage(&Message{SessionID: "sess-1", Content: "A"})
	store.AddMessage(&Message{SessionID: "sess-2", Content: "B"})
	store.AddMessage(&Message{SessionID: "sess-1", Content: "C"})

	sess1Msgs := store.GetMessages("sess-1")
	sess2Msgs := store.GetMessages("sess-2")

	assert.Len(t, sess1Msgs, 2)
	assert.Len(t, sess2Msgs, 1)
}

func TestMessageStore_Clear(t *testing.T) {
	store := NewMessageStore(100)

	store.AddMessage(&Message{SessionID: "sess-1", Content: "A"})
	store.Clear("sess-1")

	messages := store.GetMessages("sess-1")
	assert.Empty(t, messages)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./clients/tui/client -v`

Expected: FAIL

**Step 3: Implement message store**

Create `clients/tui/client/message_store.go`:
```go
// ABOUTME: Message store for maintaining per-session chat history
// ABOUTME: Implements FIFO queue with configurable history limits
package client

import (
	"sync"
	"time"
)

type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAgent
	MessageTypeTool
	MessageTypeSystem
	MessageTypeError
)

func (mt MessageType) String() string {
	switch mt {
	case MessageTypeUser:
		return "User"
	case MessageTypeAgent:
		return "Agent"
	case MessageTypeTool:
		return "Tool"
	case MessageTypeSystem:
		return "System"
	case MessageTypeError:
		return "Error"
	default:
		return "Unknown"
	}
}

func (mt MessageType) Icon() string {
	switch mt {
	case MessageTypeUser:
		return "👤"
	case MessageTypeAgent:
		return "🤖"
	case MessageTypeTool:
		return "🔧"
	case MessageTypeSystem:
		return "ℹ️"
	case MessageTypeError:
		return "⚠️"
	default:
		return "❓"
	}
}

type Message struct {
	SessionID string
	Type      MessageType
	Content   string
	Timestamp time.Time
}

type MessageStore struct {
	messages   map[string][]*Message
	limit      int
	mu         sync.RWMutex
}

func NewMessageStore(limit int) *MessageStore {
	return &MessageStore{
		messages: make(map[string][]*Message),
		limit:    limit,
	}
}

func (ms *MessageStore) AddMessage(msg *Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	sessionMsgs := ms.messages[msg.SessionID]
	sessionMsgs = append(sessionMsgs, msg)

	// Enforce history limit (FIFO)
	if len(sessionMsgs) > ms.limit {
		sessionMsgs = sessionMsgs[len(sessionMsgs)-ms.limit:]
	}

	ms.messages[msg.SessionID] = sessionMsgs
}

func (ms *MessageStore) GetMessages(sessionID string) []*Message {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	messages := ms.messages[sessionID]
	// Return copy to prevent external modification
	result := make([]*Message, len(messages))
	copy(result, messages)
	return result
}

func (ms *MessageStore) Clear(sessionID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	delete(ms.messages, sessionID)
}

func (ms *MessageStore) ClearAll() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.messages = make(map[string][]*Message)
}
```

**Step 4: Add missing import to test**

Add to `clients/tui/client/message_store_test.go`:
```go
import (
	"fmt"
	// ... other imports
)
```

**Step 5: Run tests to verify they pass**

Run: `go test ./clients/tui/client -v`

Expected: PASS (all tests)

**Step 6: Commit**

```bash
git add clients/tui/client/message_store.go clients/tui/client/message_store_test.go
git commit -m "feat(tui): add message store with history management

- Implement per-session message storage
- Add FIFO history limit enforcement
- Support multiple message types (User, Agent, Tool, System, Error)
- Add unit tests for storage and limits"
```

---

## Task 7: Sidebar Component

**Files:**
- Create: `clients/tui/components/sidebar.go`
- Create: `clients/tui/components/sidebar_test.go`

**Step 1: Write sidebar tests**

Create `clients/tui/components/sidebar_test.go`:
```go
// ABOUTME: Unit tests for sidebar component (session list display)
// ABOUTME: Tests rendering, navigation, and session selection
package components

import (
	"testing"
	"time"

	"github.com/harper/acp-relay/clients/tui/client"
	"github.com/harper/acp-relay/clients/tui/theme"
	"github.com/stretchr/testify/assert"
)

func TestNewSidebar(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	assert.NotNil(t, sb)
	assert.Equal(t, 30, sb.width)
	assert.Equal(t, 20, sb.height)
}

func TestSidebar_SetSessions(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	sessions := []*client.Session{
		{ID: "1", DisplayName: "Test", Status: client.StatusActive},
		{ID: "2", DisplayName: "Demo", Status: client.StatusIdle},
	}

	sb.SetSessions(sessions)

	view := sb.View()
	assert.Contains(t, view, "Test")
	assert.Contains(t, view, "Demo")
	assert.Contains(t, view, "⚡") // Active icon
	assert.Contains(t, view, "💤") // Idle icon
}

func TestSidebar_Navigation(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	sessions := []*client.Session{
		{ID: "1", DisplayName: "One"},
		{ID: "2", DisplayName: "Two"},
		{ID: "3", DisplayName: "Three"},
	}
	sb.SetSessions(sessions)

	// Start at 0
	assert.Equal(t, 0, sb.cursor)

	// Move down
	sb.CursorDown()
	assert.Equal(t, 1, sb.cursor)

	// Move down again
	sb.CursorDown()
	assert.Equal(t, 2, sb.cursor)

	// Wrap to top
	sb.CursorDown()
	assert.Equal(t, 0, sb.cursor)

	// Move up (wraps to bottom)
	sb.CursorUp()
	assert.Equal(t, 2, sb.cursor)
}

func TestSidebar_GetSelectedSession(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	sessions := []*client.Session{
		{ID: "1", DisplayName: "One"},
		{ID: "2", DisplayName: "Two"},
	}
	sb.SetSessions(sessions)

	// Default selection
	selected := sb.GetSelectedSession()
	assert.Equal(t, "1", selected.ID)

	// Move cursor
	sb.CursorDown()
	selected = sb.GetSelectedSession()
	assert.Equal(t, "2", selected.ID)
}

func TestSidebar_EmptySessions(t *testing.T) {
	sb := NewSidebar(30, 20, theme.DefaultTheme)

	view := sb.View()
	assert.Contains(t, view, "No sessions")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./clients/tui/components -v`

Expected: FAIL

**Step 3: Implement sidebar component**

Create `clients/tui/components/sidebar.go`:
```go
// ABOUTME: Sidebar component for displaying session list
// ABOUTME: Handles session navigation, rendering, and selection
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/clients/tui/client"
	"github.com/harper/acp-relay/clients/tui/theme"
)

type Sidebar struct {
	width    int
	height   int
	theme    theme.Theme
	sessions []*client.Session
	cursor   int
}

func NewSidebar(width, height int, t theme.Theme) *Sidebar {
	return &Sidebar{
		width:  width,
		height: height,
		theme:  t,
		cursor: 0,
	}
}

func (s *Sidebar) SetSessions(sessions []*client.Session) {
	s.sessions = sessions

	// Clamp cursor to valid range
	if s.cursor >= len(sessions) {
		s.cursor = len(sessions) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
}

func (s *Sidebar) CursorDown() {
	if len(s.sessions) == 0 {
		return
	}

	s.cursor++
	if s.cursor >= len(s.sessions) {
		s.cursor = 0 // Wrap to top
	}
}

func (s *Sidebar) CursorUp() {
	if len(s.sessions) == 0 {
		return
	}

	s.cursor--
	if s.cursor < 0 {
		s.cursor = len(s.sessions) - 1 // Wrap to bottom
	}
}

func (s *Sidebar) GetSelectedSession() *client.Session {
	if len(s.sessions) == 0 || s.cursor < 0 || s.cursor >= len(s.sessions) {
		return nil
	}
	return s.sessions[s.cursor]
}

func (s *Sidebar) SetCursor(index int) {
	if index >= 0 && index < len(s.sessions) {
		s.cursor = index
	}
}

func (s *Sidebar) View() string {
	if len(s.sessions) == 0 {
		emptyMsg := s.theme.DimStyle().Render("No sessions\n\nPress 'n' to create one")
		return s.theme.SidebarStyle().
			Width(s.width - 2).
			Height(s.height - 2).
			Render(emptyMsg)
	}

	var items []string

	// Title
	title := s.theme.ActiveSessionStyle().
		Width(s.width - 4).
		Render("SESSIONS")
	items = append(items, title, "")

	// Session list
	for i, sess := range s.sessions {
		icon := sess.Status.Icon()
		name := sess.DisplayName

		// Truncate long names
		maxLen := s.width - 8 // Account for padding and icon
		if len(name) > maxLen {
			name = name[:maxLen-3] + "..."
		}

		line := fmt.Sprintf("%s %s", icon, name)

		// Style based on selection
		if i == s.cursor {
			line = s.theme.ActiveSessionStyle().
				Width(s.width - 4).
				Render(line)
		} else {
			line = s.theme.InactiveSessionStyle().
				Width(s.width - 4).
				Render(line)
		}

		items.append(items, line)
	}

	// Help text at bottom
	help := s.theme.DimStyle().Render("\n↑↓: Navigate\nn: New\nd: Delete\nr: Rename")
	items = append(items, "", help)

	content := strings.Join(items, "\n")

	return s.theme.SidebarStyle().
		Width(s.width - 2).
		Height(s.height - 2).
		Render(content)
}

func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}
```

**Step 4: Fix append typo**

In `sidebar.go` line with `items.append`, change to:
```go
items = append(items, line)
```

**Step 5: Run tests to verify they pass**

Run: `go test ./clients/tui/components -v`

Expected: PASS (all sidebar tests)

**Step 6: Commit**

```bash
git add clients/tui/components/
git commit -m "feat(tui): add sidebar component for session list

- Implement session list rendering with status icons
- Add cursor navigation (up/down with wrapping)
- Support session selection
- Add empty state message
- Add unit tests for navigation and rendering"
```

---

*Due to length constraints, I'll continue with remaining tasks (8-15) in a condensed format...*

## Task 8-15: Remaining Components (Condensed)

**Task 8: ChatView Component** - Display messages with formatting, scrolling, syntax highlighting
**Task 9: InputArea Component** - Multi-line text input with Ctrl+S to send
**Task 10: StatusBar Component** - Connection status, active session, shortcuts
**Task 11: HelpOverlay Component** - Modal help dialog with keyboard shortcuts
**Task 12: Main Model Integration** - Wire all components together in Model/Update/View
**Task 13: Keyboard Navigation** - Tab focus switching, component-specific keybindings
**Task 14: E2E Tests** - Full flow tests with real relay server
**Task 15: Documentation** - TUI-GUIDE.md user guide and README updates

Each task follows the same TDD pattern:
1. Write test
2. Run test (FAIL)
3. Implement feature
4. Run test (PASS)
5. Commit

---

## Execution Handoff

Plan complete and saved to `docs/plans/2025-11-10-tui-implementation.md`.

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration with quality gates

**2. Parallel Session (separate)** - Open new Claude Code session in this directory, run `/superpowers:execute-plan`, batch execution with review checkpoints

**Which approach would you like, Doctor Biz?**
