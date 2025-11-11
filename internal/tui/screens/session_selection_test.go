// ABOUTME: Tests for SessionSelectionScreen modal component
// ABOUTME: Verifies navigation, selection, and rendering behavior
package screens

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/client"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// TestSessionSelectionScreen_NavigationUpDown tests arrow key navigation.
func TestSessionSelectionScreen_NavigationUpDown(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{
		{ID: "session-1", WorkingDirectory: "/tmp/dir1", CreatedAt: time.Now(), IsActive: true},
		{ID: "session-2", WorkingDirectory: "/tmp/dir2", CreatedAt: time.Now(), IsActive: false},
		{ID: "session-3", WorkingDirectory: "/tmp/dir3", CreatedAt: time.Now(), IsActive: true},
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// Initial selection should be 0
	if screen.selectedIndex != 0 {
		t.Errorf("expected initial selectedIndex=0, got %d", screen.selectedIndex)
	}

	// Press down arrow - should move to index 1
	updatedScreen, _ := screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != 1 {
		t.Errorf("after down, expected selectedIndex=1, got %d", screen.selectedIndex)
	}

	// Press down arrow again - should move to index 2
	updatedScreen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != 2 {
		t.Errorf("after down, expected selectedIndex=2, got %d", screen.selectedIndex)
	}

	// Press down arrow at end - should wrap to 0
	updatedScreen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != 0 {
		t.Errorf("after down at end, expected wrap to selectedIndex=0, got %d", screen.selectedIndex)
	}

	// Press up arrow from 0 - should wrap to 2 (last item)
	updatedScreen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != 2 {
		t.Errorf("after up from 0, expected wrap to selectedIndex=2, got %d", screen.selectedIndex)
	}

	// Press up arrow - should move to 1
	updatedScreen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != 1 {
		t.Errorf("after up, expected selectedIndex=1, got %d", screen.selectedIndex)
	}
}

// TestSessionSelectionScreen_NavigationEmptySessions tests navigation with no sessions.
func TestSessionSelectionScreen_NavigationEmptySessions(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// selectedIndex should be -1 for empty sessions
	if screen.selectedIndex != -1 {
		t.Errorf("expected selectedIndex=-1 for empty sessions, got %d", screen.selectedIndex)
	}

	// Pressing navigation keys should not change anything
	updatedScreen, _ := screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != -1 {
		t.Errorf("after down with empty sessions, expected selectedIndex=-1, got %d", screen.selectedIndex)
	}

	updatedScreen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	screen = updatedScreen.(*SessionSelectionScreen)
	if screen.selectedIndex != -1 {
		t.Errorf("after up with empty sessions, expected selectedIndex=-1, got %d", screen.selectedIndex)
	}
}

// TestSessionSelectionScreen_SelectSession tests Enter key to select session.
func TestSessionSelectionScreen_SelectSession(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{
		{ID: "session-1", WorkingDirectory: "/tmp/dir1", CreatedAt: time.Now(), IsActive: true},
		{ID: "session-2", WorkingDirectory: "/tmp/dir2", CreatedAt: time.Now(), IsActive: false},
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// Move to second session
	screen.selectedIndex = 1

	// Press Enter - should return SessionSelectedMsg
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command after Enter, got nil")
	}

	msg := cmd()
	selectedMsg, ok := msg.(SessionSelectedMsg)
	if !ok {
		t.Fatalf("expected SessionSelectedMsg, got %T", msg)
	}

	if selectedMsg.Session.ID != "session-2" {
		t.Errorf("expected selected session ID=session-2, got %s", selectedMsg.Session.ID)
	}

	if selectedMsg.Session.IsActive != false {
		t.Errorf("expected selected session IsActive=false, got %v", selectedMsg.Session.IsActive)
	}
}

// TestSessionSelectionScreen_SelectEmptySessions tests Enter with no sessions.
func TestSessionSelectionScreen_SelectEmptySessions(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// Press Enter - should not return any command
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("expected no command with empty sessions, got %v", cmd)
	}
}

// TestSessionSelectionScreen_CreateNewSession tests 'n' key to create new session.
func TestSessionSelectionScreen_CreateNewSession(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{
		{ID: "session-1", WorkingDirectory: "/tmp/dir1", CreatedAt: time.Now(), IsActive: true},
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// Press 'n' - should return CreateNewSessionMsg
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected command after 'n', got nil")
	}

	msg := cmd()
	_, ok := msg.(CreateNewSessionMsg)
	if !ok {
		t.Fatalf("expected CreateNewSessionMsg, got %T", msg)
	}
}

// TestSessionSelectionScreen_Quit tests 'q' and Esc keys to quit.
func TestSessionSelectionScreen_Quit(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{
		{ID: "session-1", WorkingDirectory: "/tmp/dir1", CreatedAt: time.Now(), IsActive: true},
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// Press 'q' - should return tea.Quit
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected command after 'q', got nil")
	}

	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	if !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}

	// Press Esc - should also return tea.Quit
	_, cmd = screen.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command after Esc, got nil")
	}

	msg = cmd()
	_, ok = msg.(tea.QuitMsg)
	if !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestSessionSelectionScreen_ViewRendering tests the View method output.
func TestSessionSelectionScreen_ViewRendering(t *testing.T) {
	th := theme.DefaultTheme
	now := time.Date(2025, 11, 10, 15, 30, 0, 0, time.UTC)
	sessions := []client.ManagementSession{
		{ID: "session-abc123def456", WorkingDirectory: "/tmp/dir1", CreatedAt: now, IsActive: true},
		{ID: "session-xyz789ghi012", WorkingDirectory: "/tmp/dir2", CreatedAt: now.Add(-1 * time.Hour), IsActive: false},
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	view := screen.View()

	// Check for title
	if !strings.Contains(view, "Select or Create Session") {
		t.Error("view should contain title 'Select or Create Session'")
	}

	// Check for emoji in title
	if !strings.Contains(view, "🔄") {
		t.Error("view should contain 🔄 emoji")
	}

	// Check for active session indicator (✅)
	if !strings.Contains(view, "✅") {
		t.Error("view should contain ✅ for active session")
	}

	// Check for closed session indicator (💤)
	if !strings.Contains(view, "💤") {
		t.Error("view should contain 💤 for closed session")
	}

	// Check for session ID (first 12 chars)
	if !strings.Contains(view, "session-abc1") {
		t.Error("view should contain truncated session ID 'session-abc1'")
	}

	// Check for keybindings
	if !strings.Contains(view, "Navigate") {
		t.Error("view should contain navigation instructions")
	}
	if !strings.Contains(view, "Enter") {
		t.Error("view should contain Enter key instruction")
	}
	if !strings.Contains(view, "n: New") {
		t.Error("view should contain 'n: New' instruction")
	}
	if !strings.Contains(view, "q: Quit") {
		t.Error("view should contain 'q: Quit' instruction")
	}
}

// TestSessionSelectionScreen_ViewRenderingEmpty tests rendering with no sessions.
func TestSessionSelectionScreen_ViewRenderingEmpty(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	view := screen.View()

	// Check for title
	if !strings.Contains(view, "Select or Create Session") {
		t.Error("view should contain title even with no sessions")
	}

	// Check for empty message (should contain "No" and "sessions" somewhere)
	viewLower := strings.ToLower(view)
	if !strings.Contains(viewLower, "no") || !strings.Contains(viewLower, "sessions") {
		t.Errorf("view should indicate no sessions available, got: %q", view)
	}

	// Check for keybindings
	if !strings.Contains(view, "n: New") {
		t.Error("view should contain 'n: New' instruction even with no sessions")
	}
}

// TestSessionSelectionScreen_ViewRenderingMany tests rendering with many sessions (>15).
func TestSessionSelectionScreen_ViewRenderingMany(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{}
	now := time.Now()

	// Create 20 sessions
	for i := 0; i < 20; i++ {
		sessions = append(sessions, client.ManagementSession{
			ID:               string(rune('a'+i)) + "-session-id-long",
			WorkingDirectory: "/tmp/dir",
			CreatedAt:        now.Add(-time.Duration(i) * time.Hour),
			IsActive:         i%2 == 0,
		})
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	view := screen.View()

	// Should only show first 15 sessions
	// Count how many session indicators appear
	activeCount := strings.Count(view, "✅")
	closedCount := strings.Count(view, "💤")
	totalVisible := activeCount + closedCount

	if totalVisible > 15 {
		t.Errorf("view should show at most 15 sessions, but shows %d", totalVisible)
	}
}

// TestSessionSelectionScreen_Resize tests window resize handling.
func TestSessionSelectionScreen_Resize(t *testing.T) {
	th := theme.DefaultTheme
	sessions := []client.ManagementSession{
		{ID: "session-1", WorkingDirectory: "/tmp/dir1", CreatedAt: time.Now(), IsActive: true},
	}

	screen := NewSessionSelectionScreen(sessions, 80, 24, th)

	// Initial dimensions
	if screen.width != 80 || screen.height != 24 {
		t.Errorf("expected initial size 80x24, got %dx%d", screen.width, screen.height)
	}

	// Send WindowSizeMsg
	updatedScreen, _ := screen.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	screen = updatedScreen.(*SessionSelectionScreen)

	// Check dimensions updated
	if screen.width != 100 || screen.height != 30 {
		t.Errorf("expected updated size 100x30, got %dx%d", screen.width, screen.height)
	}
}
