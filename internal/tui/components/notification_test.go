// ABOUTME: Tests for the toast notification system component
// ABOUTME: Verifies notification creation, dismissal, auto-dismiss, and rendering
package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/theme"
)

func TestNotificationCreation(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Initially should have no notifications
	if len(nc.notifications) != 0 {
		t.Errorf("Expected 0 notifications initially, got %d", len(nc.notifications))
	}

	// Show a notification
	cmd := nc.Show("Test message", "info")
	if cmd == nil {
		t.Error("Expected Show to return a command for auto-dismiss")
	}

	// Should now have 1 notification
	if len(nc.notifications) != 1 {
		t.Errorf("Expected 1 notification after Show, got %d", len(nc.notifications))
	}

	// Verify notification properties
	notif := nc.notifications[0]
	if notif.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", notif.Message)
	}
	if notif.Severity != "info" {
		t.Errorf("Expected severity 'info', got '%s'", notif.Severity)
	}
	if !notif.Visible {
		t.Error("Expected notification to be visible")
	}
	if notif.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestNotificationMaxLimit(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Add 5 notifications (max is 3)
	for i := 0; i < 5; i++ {
		_ = nc.Show("Message", "info")
	}

	// Should only have 3 notifications (most recent)
	if len(nc.notifications) != 3 {
		t.Errorf("Expected max 3 notifications, got %d", len(nc.notifications))
	}
}

func TestNotificationDismiss(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Add 3 notifications
	_ = nc.Show("Message 1", "info")
	_ = nc.Show("Message 2", "warning")
	_ = nc.Show("Message 3", "error")

	if len(nc.notifications) != 3 {
		t.Fatalf("Expected 3 notifications, got %d", len(nc.notifications))
	}

	// Dismiss the middle one
	nc.Dismiss(1)

	// Should have 2 notifications left
	if len(nc.notifications) != 2 {
		t.Errorf("Expected 2 notifications after dismiss, got %d", len(nc.notifications))
	}

	// Verify the correct one was removed
	if nc.notifications[0].Message != "Message 1" {
		t.Errorf("Expected first notification to be 'Message 1', got '%s'", nc.notifications[0].Message)
	}
	if nc.notifications[1].Message != "Message 3" {
		t.Errorf("Expected second notification to be 'Message 3', got '%s'", nc.notifications[1].Message)
	}
}

func TestNotificationDismissOutOfBounds(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	_ = nc.Show("Message", "info")

	// Try to dismiss invalid indices
	nc.Dismiss(-1) // Should not panic
	nc.Dismiss(10) // Should not panic

	// Should still have 1 notification
	if len(nc.notifications) != 1 {
		t.Errorf("Expected 1 notification after invalid dismiss, got %d", len(nc.notifications))
	}
}

func TestNotificationAutoDismiss(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Create a notification
	cmd := nc.Show("Test", "info")
	if cmd == nil {
		t.Fatal("Expected Show to return auto-dismiss command")
	}

	// Execute the command to get the dismiss message
	msg := cmd()

	// Should be a DismissNotificationMsg
	dismissMsg, ok := msg.(DismissNotificationMsg)
	if !ok {
		t.Fatalf("Expected DismissNotificationMsg, got %T", msg)
	}

	// Verify the notification index
	if dismissMsg.Index != 0 {
		t.Errorf("Expected dismiss index 0, got %d", dismissMsg.Index)
	}

	// Now update with the dismiss message
	_ = nc.Update(dismissMsg)

	// Notification should be removed
	if len(nc.notifications) != 0 {
		t.Errorf("Expected 0 notifications after auto-dismiss, got %d", len(nc.notifications))
	}
}

func TestNotificationUpdateHandlesDismissMsg(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Add 2 notifications
	_ = nc.Show("Message 1", "info")
	_ = nc.Show("Message 2", "warning")

	// Send dismiss message for first notification
	dismissMsg := DismissNotificationMsg{Index: 0}
	_ = nc.Update(dismissMsg)

	// Should have 1 notification left
	if len(nc.notifications) != 1 {
		t.Errorf("Expected 1 notification after dismiss, got %d", len(nc.notifications))
	}

	// Remaining notification should be Message 2
	if nc.notifications[0].Message != "Message 2" {
		t.Errorf("Expected remaining notification to be 'Message 2', got '%s'", nc.notifications[0].Message)
	}
}

func TestNotificationRenderingSeverities(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	testCases := []struct {
		severity     string
		expectedIcon string
	}{
		{"info", "ℹ️"},
		{"warning", "⚠️"},
		{"error", "❌"},
		{"success", "✅"},
	}

	for _, tc := range testCases {
		t.Run(tc.severity, func(t *testing.T) {
			// Clear notifications
			nc.notifications = []*Notification{}

			// Add notification with specific severity
			_ = nc.Show("Test message", tc.severity)

			// Render
			output := nc.View()

			// Check for icon
			if !strings.Contains(output, tc.expectedIcon) {
				t.Errorf("Expected output to contain icon '%s' for severity '%s', got: %s", tc.expectedIcon, tc.severity, output)
			}

			// Check for message
			if !strings.Contains(output, "Test message") {
				t.Errorf("Expected output to contain 'Test message', got: %s", output)
			}
		})
	}
}

func TestNotificationRenderingEmpty(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// No notifications
	output := nc.View()

	// Should be empty
	if output != "" {
		t.Errorf("Expected empty output with no notifications, got: %s", output)
	}
}

func TestNotificationRenderingMultiple(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Add 3 notifications
	_ = nc.Show("Info message", "info")
	_ = nc.Show("Warning message", "warning")
	_ = nc.Show("Error message", "error")

	// Render
	output := nc.View()

	// Check all messages are present
	if !strings.Contains(output, "Info message") {
		t.Error("Expected output to contain 'Info message'")
	}
	if !strings.Contains(output, "Warning message") {
		t.Error("Expected output to contain 'Warning message'")
	}
	if !strings.Contains(output, "Error message") {
		t.Error("Expected output to contain 'Error message'")
	}

	// Check all icons are present
	if !strings.Contains(output, "ℹ️") {
		t.Error("Expected output to contain info icon")
	}
	if !strings.Contains(output, "⚠️") {
		t.Error("Expected output to contain warning icon")
	}
	if !strings.Contains(output, "❌") {
		t.Error("Expected output to contain error icon")
	}
}

func TestNotificationUpdateWithUnrelatedMessage(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	_ = nc.Show("Test", "info")

	// Send an unrelated message
	cmd := nc.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	// Should return nil (not handled)
	if cmd != nil {
		t.Error("Expected nil command for unrelated message")
	}

	// Notification should still exist
	if len(nc.notifications) != 1 {
		t.Errorf("Expected 1 notification after unrelated message, got %d", len(nc.notifications))
	}
}

func TestNotificationMessageWrapping(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(40, th) // Narrow width

	// Long message that should wrap
	longMessage := "This is a very long message that should wrap across multiple lines when rendered"
	_ = nc.Show(longMessage, "info")

	// Render
	output := nc.View()

	// Should contain the message (possibly wrapped)
	if !strings.Contains(output, "This is a very long message") {
		t.Errorf("Expected output to contain wrapped message, got: %s", output)
	}
}

func TestNotificationAutoDismissTimer(t *testing.T) {
	th := theme.DefaultTheme
	nc := NewNotificationComponent(80, th)

	// Show notification and get the auto-dismiss command
	cmd := nc.Show("Test", "info")
	if cmd == nil {
		t.Fatal("Expected auto-dismiss command")
	}

	// The command should return a tick message that eventually triggers dismiss
	// We can't easily test the 3-second delay in unit tests, but we can verify
	// that the command exists and is of the right type

	// Execute the command
	msg := cmd()

	// Should be a DismissNotificationMsg after the delay
	if _, ok := msg.(DismissNotificationMsg); !ok {
		t.Errorf("Expected DismissNotificationMsg after auto-dismiss, got %T", msg)
	}
}
