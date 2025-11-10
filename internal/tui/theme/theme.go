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
