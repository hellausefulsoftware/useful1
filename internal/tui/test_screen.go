package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/cli"
)

// TestScreen is the screen for running tests
type TestScreen struct {
	BaseScreen
	suiteInput   textinput.Model
	executor     *cli.Executor
	executing    bool
	result       string
	resultError  error
}

// NewTestScreen creates a new test screen
func NewTestScreen(app *App) *TestScreen {
	suiteInput := textinput.New()
	suiteInput.Placeholder = "Test suite name (optional)"
	suiteInput.Focus()
	suiteInput.Width = 40
	
	var executor *cli.Executor
	if app.GetConfig() != nil {
		executor = cli.NewExecutor(app.GetConfig())
	}
	
	return &TestScreen{
		BaseScreen:    NewBaseScreen(app, "Run Tests"),
		suiteInput:    suiteInput,
		executor:      executor,
		executing:     false,
	}
}

// Init initializes the test screen
func (t *TestScreen) Init() tea.Cmd {
	t.executing = false
	t.result = ""
	t.resultError = nil
	return textinput.Blink
}

// Update handles UI updates for the test screen
func (t *TestScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if t.executing {
			// Allow going back if we're showing results
			if key.Matches(msg, t.app.keyMap.Back) {
				t.executing = false
				return t, nil
			}
			return t, nil
		}
		
		// Non-execution state key handling
		switch {
		case key.Matches(msg, t.app.keyMap.Back):
			return t, t.app.ChangeScreen(ScreenMainMenu)
			
		case key.Matches(msg, t.app.keyMap.Execute):
			// Start execution
			t.executing = true
			return t, t.startExecution()
		}
	
	case executionResultMsg:
		t.result = msg.output
		t.resultError = msg.err
		return t, nil
	}
	
	// Handle input updates
	t.suiteInput, cmd = t.suiteInput.Update(msg)
	return t, cmd
}

// View renders the test screen
func (t *TestScreen) View() string {
	theme := t.app.GetTheme()
	
	if t.executing && t.result == "" {
		return theme.Title.Render("Running tests...") + "\n\n" +
			theme.Text.Render("Please wait while tests are running...")
	}
	
	if t.result != "" {
		resultStyle := theme.Text
		if t.resultError != nil {
			resultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Error))
		}
		
		content := theme.Title.Render("Test Results") + "\n\n" +
			resultStyle.Render(t.result) + "\n\n" +
			theme.Faint.Render("Press ESC to go back")
		
		return lipgloss.NewStyle().Width(t.app.GetWidth()).Align(lipgloss.Left).Render(content)
	}
	
	// Normal input view
	content := t.RenderTitle() + "\n\n" +
		theme.Subtitle.Render("Enter test suite details:") + "\n\n"
	
	// Test suite input
	focusedStyle := theme.Bold.Copy().Foreground(lipgloss.Color(theme.Blue))
	suiteLabel := focusedStyle.Render("Test Suite: ")
	content += suiteLabel + t.suiteInput.View() + "\n\n"
	
	// Help text
	content += theme.Faint.Render("Leave blank to run all tests") + "\n\n"
	
	// Instructions
	content += theme.Faint.Render("Press E to execute, ESC to go back") + "\n\n"
	
	// Footer
	content += t.RenderFooter()
	
	return lipgloss.NewStyle().Width(t.app.GetWidth()).Align(lipgloss.Left).Render(content)
}

// ShortHelp returns keybindings to be shown in the help menu
func (t *TestScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		t.app.keyMap.Execute,
		t.app.keyMap.Back,
		t.app.keyMap.Help,
		t.app.keyMap.Quit,
	}
}

// startExecution begins the execution process
func (t *TestScreen) startExecution() tea.Cmd {
	return func() tea.Msg {
		// Get the input values
		testSuite := t.suiteInput.Value()
		
		// Check if executor is available
		if t.executor == nil {
			return executionResultMsg{
				output: "Error: Configuration not found. Please run 'useful1 config' first",
				err:    fmt.Errorf("configuration not found"),
			}
		}
		
		// Execute the command
		err := t.executor.RunTests(testSuite)
		
		// Prepare result message
		result := "Tests completed successfully"
		if err != nil {
			result = "Error running tests: " + err.Error()
		}
		
		return executionResultMsg{
			output: result,
			err:    err,
		}
	}
}