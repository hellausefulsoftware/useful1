package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// KeyMap defines keybindings
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Select  key.Binding
	Back    key.Binding
	Quit    key.Binding
	Help    key.Binding
	Execute key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "right"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c/q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Execute: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "execute"),
		),
	}
}

// App is the main TUI application
type App struct {
	config   *config.Config
	theme    *ColorblindFriendlyTheme
	keyMap   KeyMap
	help     help.Model
	screen   Screen
	screens  map[ScreenType]Screen
	width    int
	height   int
	ready    bool
	showHelp bool
}

// NewApp creates a new TUI application
func NewApp(cfg *config.Config) *App {
	theme := NewTheme()
	keyMap := DefaultKeyMap()
	helpModel := help.New()
	helpModel.Styles.ShortKey = theme.Bold
	helpModel.Styles.ShortDesc = theme.Text
	helpModel.Styles.ShortSeparator = theme.Faint
	helpModel.Styles.FullKey = theme.Bold
	helpModel.Styles.FullDesc = theme.Text
	helpModel.Styles.FullSeparator = theme.Faint

	// Always set logging output to stderr in TUI mode to prevent breaking the interface
	if cfg != nil {
		// Always direct logs to stderr in TUI mode
		writer := os.Stderr
		cfg.Logging.Output = writer
		cfg.Logging.Level = "info"
		cfg.Logging.JSONFormat = false

		// Initialize logger
		logConfig := &logging.Config{
			Level:      logging.LogLevel(cfg.Logging.Level),
			Output:     cfg.Logging.Output,
			JSONFormat: cfg.Logging.JSONFormat,
		}
		logging.Initialize(logConfig)
	}

	app := &App{
		config:   cfg,
		theme:    theme,
		keyMap:   keyMap,
		help:     helpModel,
		screens:  make(map[ScreenType]Screen),
		showHelp: false,
	}

	// Initialize screens
	app.screens[ScreenMainMenu] = NewMainMenuScreen(app)

	// Only initialize screens that need config if config exists
	app.screens[ScreenConfig] = NewConfigScreen(app)
	app.screens[ScreenMonitor] = NewMonitorScreen(app)
	app.screens[ScreenExecute] = NewExecuteScreen(app)

	// Set initial screen
	app.screen = app.screens[ScreenMainMenu]

	return app
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	// If config is nil, start with the config screen
	if a.config == nil && a.screen != a.screens[ScreenMainMenu] {
		// Show a message about missing config
		return func() tea.Msg {
			return ConfigMissingMsg{}
		}
	}
	return nil
}

// ConfigMissingMsg is sent when the configuration is missing
type ConfigMissingMsg struct{}

// Update handles UI updates
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, a.keyMap.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keyMap.Help):
			a.showHelp = !a.showHelp
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width
		a.ready = true

	case ChangeScreenMsg:
		if screen, ok := a.screens[msg.Screen]; ok {
			a.screen = screen
			return a, screen.Init()
		}

	case ConfigMissingMsg:
		// If config is missing, switch to main menu or config screen
		if a.config == nil {
			a.screen = a.screens[ScreenMainMenu]
			return a, func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
			}
		}
	}

	// Update the current screen
	newScreen, cmd := a.screen.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// If the screen was changed, update the reference
	if s, ok := newScreen.(Screen); ok && s != a.screen {
		a.screen = s
	}

	return a, tea.Batch(cmds...)
}

// View renders the UI
func (a *App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	content := a.screen.View()

	// Show help at the bottom if enabled
	if a.showHelp {
		// Convert shortHelp to key.Map for help.View
		helpKeys := a.screen.ShortHelp()
		helpView := a.help.ShortHelpView(helpKeys)
		return lipgloss.JoinVertical(lipgloss.Left, content, "\n", helpView)
	}

	return content
}

// GetTheme returns the theme
func (a *App) GetTheme() *ColorblindFriendlyTheme {
	return a.theme
}

// GetKeyMap returns the keymap
func (a *App) GetKeyMap() KeyMap {
	return a.keyMap
}

// GetConfig returns the config
func (a *App) GetConfig() *config.Config {
	return a.config
}

// GetWidth returns the terminal width
func (a *App) GetWidth() int {
	return a.width
}

// GetHeight returns the terminal height
func (a *App) GetHeight() int {
	return a.height
}

// ChangeScreen changes the current screen
func (a *App) ChangeScreen(screenType ScreenType) tea.Cmd {
	return func() tea.Msg {
		return ChangeScreenMsg{Screen: screenType}
	}
}

// CreateCLIExecutor creates a CLI executor
func (a *App) CreateCLIExecutor() *cli.Executor {
	if a.config == nil {
		return nil
	}
	return cli.NewExecutor(a.config)
}

// ChangeScreenMsg is a message to change the current screen
type ChangeScreenMsg struct {
	Screen ScreenType
}

// Run runs the TUI application
func Run(cfg *config.Config) error {
	return RunWithScreen(cfg, ScreenMainMenu)
}

// RunWithScreen runs the TUI application with a specific initial screen
func RunWithScreen(cfg *config.Config, initialScreen ScreenType) error {
	app := NewApp(cfg)

	// Set the initial screen if it exists
	if screen, ok := app.screens[initialScreen]; ok {
		app.screen = screen
	}

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	// This type assertion is not needed since we're not using the result
	// But we keep it as a comment for future reference in case we need to use it
	// _, ok := model.(*App)
	return nil
}
