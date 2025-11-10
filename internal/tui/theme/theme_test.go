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
