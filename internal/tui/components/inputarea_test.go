// ABOUTME: Tests for InputArea component multi-line text input
// ABOUTME: Verifies input operations, focus handling, and sizing behavior
package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/harper/acp-relay/internal/tui/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInputArea(t *testing.T) {
	width, height := 80, 5
	th := theme.DefaultTheme
	ia := NewInputArea(width, height, th)

	require.NotNil(t, ia)
	assert.Equal(t, width, ia.width)
	assert.Equal(t, height, ia.height)
	assert.Equal(t, th, ia.theme)
	assert.NotNil(t, ia.textarea)
	assert.False(t, ia.focused)
}

func TestInputArea_SetValue(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	testValue := "Hello, world!"
	ia.SetValue(testValue)

	assert.Equal(t, testValue, ia.GetValue())
}

func TestInputArea_GetValue(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Initially empty
	assert.Equal(t, "", ia.GetValue())

	// After setting value
	testValue := "Test message"
	ia.SetValue(testValue)
	assert.Equal(t, testValue, ia.GetValue())
}

func TestInputArea_Clear(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Set a value
	ia.SetValue("Some text here")
	assert.NotEqual(t, "", ia.GetValue())

	// Clear it
	ia.Clear()
	assert.Equal(t, "", ia.GetValue())
}

func TestInputArea_Focus(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Initially not focused
	assert.False(t, ia.focused)

	// Focus it
	ia.Focus()
	assert.True(t, ia.focused)
}

func TestInputArea_Blur(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Focus first
	ia.Focus()
	assert.True(t, ia.focused)

	// Then blur
	ia.Blur()
	assert.False(t, ia.focused)
}

func TestInputArea_SetSize(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	newWidth, newHeight := 100, 8
	ia.SetSize(newWidth, newHeight)

	assert.Equal(t, newWidth, ia.width)
	assert.Equal(t, newHeight, ia.height)
}

func TestInputArea_Init(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	cmd := ia.Init()
	// Init should return textarea.Blink command
	assert.NotNil(t, cmd)
}

func TestInputArea_Update(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Test with a key message
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	updatedModel, cmd := ia.Update(msg)

	// Should return an InputArea
	assert.NotNil(t, updatedModel)
	updatedIA, ok := updatedModel.(*InputArea)
	assert.True(t, ok)
	assert.NotNil(t, updatedIA)

	// Command might be nil or not, depending on textarea behavior
	_ = cmd
}

func TestInputArea_View(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// View should render
	view := ia.View()
	assert.NotEmpty(t, view)
}

func TestInputArea_ViewWithPlaceholder(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// When empty, should show placeholder
	view := ia.View()
	assert.Contains(t, view, "Type your message")
	assert.Contains(t, view, "Ctrl+S to send")
}

func TestInputArea_ViewWithContent(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Set content
	testContent := "This is my message"
	ia.SetValue(testContent)

	view := ia.View()
	assert.NotEmpty(t, view)
	// Note: The actual content visibility depends on textarea rendering
}

func TestInputArea_MultilineContent(t *testing.T) {
	ia := NewInputArea(80, 5, theme.DefaultTheme)

	// Test multiline input
	multilineText := "Line 1\nLine 2\nLine 3"
	ia.SetValue(multilineText)

	assert.Equal(t, multilineText, ia.GetValue())
}

func TestInputArea_ThemeApplication(t *testing.T) {
	tests := []struct {
		name  string
		theme theme.Theme
	}{
		{"Default theme", theme.DefaultTheme},
		{"Dark theme", theme.DarkTheme},
		{"Light theme", theme.LightTheme},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ia := NewInputArea(80, 5, tt.theme)
			assert.Equal(t, tt.theme, ia.theme)

			// View should render without panic
			view := ia.View()
			assert.NotEmpty(t, view)
		})
	}
}
