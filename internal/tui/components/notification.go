// ABOUTME: Toast notification system for displaying temporary messages
// ABOUTME: Supports info, warning, error, and success severities with auto-dismiss
package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harper/acp-relay/internal/tui/theme"
)

// Notification represents a single toast notification.
type Notification struct {
	Message   string
	Severity  string // info, warning, error, success
	Visible   bool
	CreatedAt time.Time
}

// NotificationComponent manages toast notifications.
type NotificationComponent struct {
	notifications []*Notification
	maxVisible    int
	width         int
	theme         theme.Theme
}

// DismissNotificationMsg is sent to dismiss a notification.
type DismissNotificationMsg struct {
	Index int
}

const (
	maxNotifications     = 3
	notificationWidth    = 40
	autoDismissDelay     = 3 * time.Second
	severityInfo         = "info"
	severityWarning      = "warning"
	severityError        = "error"
	severitySuccess      = "success"
	severityDefaultValue = "info"
)

// NewNotificationComponent creates a new notification component.
func NewNotificationComponent(width int, th theme.Theme) *NotificationComponent {
	return &NotificationComponent{
		notifications: make([]*Notification, 0, maxNotifications),
		maxVisible:    maxNotifications,
		width:         width,
		theme:         th,
	}
}

// Show creates and displays a new notification, returns auto-dismiss command.
func (nc *NotificationComponent) Show(message string, severity string) tea.Cmd {
	// Create new notification
	notif := &Notification{
		Message:   message,
		Severity:  severity,
		Visible:   true,
		CreatedAt: time.Now(),
	}

	// Add to notifications slice
	nc.notifications = append(nc.notifications, notif)

	// Get the index of the new notification
	index := len(nc.notifications) - 1

	// Enforce max visible limit (keep most recent)
	if len(nc.notifications) > nc.maxVisible {
		nc.notifications = nc.notifications[len(nc.notifications)-nc.maxVisible:]
		// Adjust index after truncation
		index = len(nc.notifications) - 1
	}

	// Return auto-dismiss command
	return nc.autoDismissCmd(index)
}

// Dismiss removes a notification by index.
func (nc *NotificationComponent) Dismiss(index int) {
	if index < 0 || index >= len(nc.notifications) {
		return
	}

	// Remove notification at index
	nc.notifications = append(nc.notifications[:index], nc.notifications[index+1:]...)
}

// Update handles messages for the notification component.
func (nc *NotificationComponent) Update(msg tea.Msg) tea.Cmd {
	if dismissMsg, ok := msg.(DismissNotificationMsg); ok {
		nc.Dismiss(dismissMsg.Index)
		return nil
	}

	return nil
}

// View renders the notifications as a vertical stack.
func (nc *NotificationComponent) View() string {
	if len(nc.notifications) == 0 {
		return ""
	}

	notificationViews := make([]string, 0, len(nc.notifications))

	for _, notif := range nc.notifications {
		if !notif.Visible {
			continue
		}

		// Get icon and color based on severity
		icon := nc.getIcon(notif.Severity)
		borderColor := nc.getBorderColor(notif.Severity)

		// Create notification style
		notifStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			Width(notificationWidth).
			MarginBottom(1)

		// Format notification content
		content := icon + " " + notif.Message

		// Render notification
		notificationViews = append(notificationViews, notifStyle.Render(content))
	}

	// Stack notifications vertically
	return lipgloss.JoinVertical(lipgloss.Left, notificationViews...)
}

// getIcon returns the icon for a given severity.
func (nc *NotificationComponent) getIcon(severity string) string {
	switch severity {
	case severityInfo:
		return "ℹ️"
	case severityWarning:
		return "⚠️"
	case severityError:
		return "❌"
	case severitySuccess:
		return "✅"
	default:
		return "ℹ️"
	}
}

// getBorderColor returns the border color for a given severity.
func (nc *NotificationComponent) getBorderColor(severity string) lipgloss.Color {
	switch severity {
	case severityInfo:
		return nc.theme.UserMsg // Blue
	case severityWarning:
		return nc.theme.Warning // Yellow
	case severityError:
		return nc.theme.Error // Red
	case severitySuccess:
		return nc.theme.Success // Green
	default:
		return nc.theme.UserMsg // Default to blue
	}
}

// autoDismissCmd returns a command that dismisses a notification after delay.
func (nc *NotificationComponent) autoDismissCmd(index int) tea.Cmd {
	return tea.Tick(autoDismissDelay, func(time.Time) tea.Msg {
		return DismissNotificationMsg{Index: index}
	})
}
