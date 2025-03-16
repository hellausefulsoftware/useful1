package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
)

// ScreenType defines the different screens in the application
type ScreenType int

// Screen type constants
const (
	ScreenMainMenu ScreenType = iota // Main menu screen
	ScreenRespond                    // Response screen
	ScreenPR                         // Pull request screen
	ScreenTest                       // Test screen
	ScreenConfig                     // Configuration screen
	ScreenMonitor                    // Monitoring screen
)

// Screen is the interface for all screens in the application
type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (tea.Model, tea.Cmd)
	View() string
	ShortHelp() []key.Binding
}

// BaseScreen provides common functionality for all screens
type BaseScreen struct {
	app   *App
	title string
}

// ShortHelp returns keybindings to be shown in the help menu
func (b *BaseScreen) ShortHelp() []key.Binding {
	return []key.Binding{b.app.keyMap.Help, b.app.keyMap.Quit}
}

// NewBaseScreen creates a new base screen
func NewBaseScreen(app *App, title string) BaseScreen {
	return BaseScreen{
		app:   app,
		title: title,
	}
}

// RenderTitle renders the screen title
func (b *BaseScreen) RenderTitle() string {
	return b.app.theme.Title.Render(b.title)
}

// RenderFooter renders the screen footer
func (b *BaseScreen) RenderFooter() string {
	help := "? for help"
	quit := "q to quit"
	return b.app.theme.Faint.Render(help + " • " + quit)
}
