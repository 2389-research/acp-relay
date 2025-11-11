// ABOUTME: Entry point for the ACP-Relay TUI client
// ABOUTME: Initializes configuration, theme, and starts the Bubbletea application
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui"
	"github.com/harper/acp-relay/internal/tui/config"
)

var (
	version    = "dev"
	buildTime  = "unknown"
	configPath = flag.String("config", "", "Path to config file")
	relayURL   = flag.String("relay", "", "Relay WebSocket URL (overrides config)")
	showVer    = flag.Bool("version", false, "Show version information")
	debug      = flag.Bool("debug", false, "Enable debug logging to ~/.local/share/acp-tui/debug.log")
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

	// Setup debug logging if enabled
	if *debug {
		logFile := os.ExpandEnv("$HOME/.local/share/acp-tui/debug.log")
		os.MkdirAll(os.ExpandEnv("$HOME/.local/share/acp-tui"), 0755)
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open debug log: %v\n", err)
		} else {
			tui.EnableDebug(f)
			defer f.Close()
			tui.DebugLog("TUI starting with relay URL: %s", cfg.Relay.URL)
		}
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
