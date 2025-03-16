package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MenuItem represents a menu item
type MenuItem struct {
	title       string
	description string
	screen      ScreenType
}

// MainMenuScreen is the main menu of the application
type MainMenuScreen struct {
	BaseScreen
	items  []MenuItem
	cursor int
}

// NewMainMenuScreen creates a new main menu screen
func NewMainMenuScreen(app *App) *MainMenuScreen {
	m := &MainMenuScreen{
		BaseScreen: NewBaseScreen(app, "Useful1 CLI"),
		items: []MenuItem{
			{title: "Respond", description: "Respond to GitHub issues", screen: ScreenRespond},
			{title: "PR", description: "Create pull requests", screen: ScreenPR},
			{title: "Config", description: "Configure settings", screen: ScreenConfig},
			{title: "Monitor", description: "Monitor GitHub issues", screen: ScreenMonitor},
			{title: "Execute", description: "Run the CLI tool directly", screen: ScreenExecute},
			{title: "Quit", description: "Exit the application", screen: 0},
		},
		cursor: 0,
	}
	return m
}

// Init initializes the main menu screen
func (m *MainMenuScreen) Init() tea.Cmd {
	return nil
}

// Update handles UI updates for the main menu
func (m *MainMenuScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.app.keyMap.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.app.keyMap.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.app.keyMap.Select):
			if m.cursor == len(m.items)-1 {
				// Last item is Quit
				return m, tea.Quit
			}
			return m, m.app.ChangeScreen(m.items[m.cursor].screen)
		}
	}

	return m, nil
}

// View renders the main menu
func (m *MainMenuScreen) View() string {
	theme := m.app.GetTheme()

	// Render title
	s := m.RenderTitle() + "\n\n"

	// Header with app name and version
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.LightBlue)).Bold(true)
	s += headerStyle.Render("Useful1 Automation Tool") + "\n\n"

	// Check if config exists
	if m.app.GetConfig() == nil {
		warning := theme.Bold.Foreground(lipgloss.Color(theme.Warning)).Render("Configuration not found! Please run the Config operation first.")
		s += warning + "\n\n"
	}

	// Render subtitle
	s += theme.Subtitle.Render("Select an operation:") + "\n\n"

	// Render menu items
	for i, item := range m.items {
		cursor := " "
		style := theme.UnselectedItem

		if m.cursor == i {
			cursor = ">"
			style = theme.SelectedItem
		}

		s += cursor + " " + style.Render(item.title) + " - " + theme.Text.Render(item.description) + "\n"
	}

	// Add empty line and footer
	s += "\n" + m.RenderFooter()

	// Left-align in terminal
	return lipgloss.NewStyle().Width(m.app.GetWidth()).Align(lipgloss.Left).Render(s)
}

// ShortHelp returns keybindings to be shown in the help menu
func (m *MainMenuScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		m.app.keyMap.Up,
		m.app.keyMap.Down,
		m.app.keyMap.Select,
		m.app.keyMap.Help,
		m.app.keyMap.Quit,
	}
}
