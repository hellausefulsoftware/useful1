package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/cli"
)

// RespondScreen is the screen for responding to GitHub issues
type RespondScreen struct {
	BaseScreen
	issueInput   textinput.Model
	templateInput textinput.Model
	focusedInput int
	executor     *cli.Executor
	executing    bool
	result       string
	resultError  error
}

// NewRespondScreen creates a new respond screen
func NewRespondScreen(app *App) *RespondScreen {
	issueInput := textinput.New()
	issueInput.Placeholder = "Issue number"
	issueInput.Focus()
	issueInput.Width = 30
	
	templateInput := textinput.New()
	templateInput.Placeholder = "Template name (default: default)"
	templateInput.Width = 30
	templateInput.SetValue("default")
	
	var executor *cli.Executor
	if app.GetConfig() != nil {
		executor = cli.NewExecutor(app.GetConfig())
	}
	
	return &RespondScreen{
		BaseScreen:    NewBaseScreen(app, "Respond to GitHub Issues"),
		issueInput:    issueInput,
		templateInput: templateInput,
		focusedInput:  0,
		executor:      executor,
		executing:     false,
	}
}

// Init initializes the respond screen
func (r *RespondScreen) Init() tea.Cmd {
	r.executing = false
	r.result = ""
	r.resultError = nil
	return textinput.Blink
}

// Update handles UI updates for the respond screen
func (r *RespondScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if r.executing {
			// Allow going back if we're showing results
			if key.Matches(msg, r.app.keyMap.Back) {
				r.executing = false
				return r, nil
			}
			return r, nil
		}
		
		// Non-execution state key handling
		switch {
		case key.Matches(msg, r.app.keyMap.Back):
			return r, r.app.ChangeScreen(ScreenMainMenu)
		
		case key.Matches(msg, r.app.keyMap.Up, r.app.keyMap.Down):
			// Cycle through inputs
			if key.Matches(msg, r.app.keyMap.Up) {
				r.focusedInput--
				if r.focusedInput < 0 {
					r.focusedInput = 1
				}
			} else {
				r.focusedInput++
				if r.focusedInput > 1 {
					r.focusedInput = 0
				}
			}
			
			// Focus the appropriate input
			if r.focusedInput == 0 {
				r.issueInput.Focus()
				r.templateInput.Blur()
			} else {
				r.issueInput.Blur()
				r.templateInput.Focus()
			}
			
			return r, nil
			
		case key.Matches(msg, r.app.keyMap.Execute):
			// Validate input
			if r.issueInput.Value() == "" {
				r.result = "Please enter an issue number"
				return r, nil
			}
			
			// Start execution
			r.executing = true
			return r, r.startExecution()
		}
	
	case executionResultMsg:
		r.result = msg.output
		r.resultError = msg.err
		return r, nil
	}
	
	// Handle input updates
	if r.focusedInput == 0 {
		r.issueInput, cmd = r.issueInput.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		r.templateInput, cmd = r.templateInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return r, tea.Batch(cmds...)
}

// View renders the respond screen
func (r *RespondScreen) View() string {
	theme := r.app.GetTheme()
	
	if r.executing && r.result == "" {
		return theme.Title.Render("Responding to issue...") + "\n\n" +
			theme.Text.Render("Please wait while we process your request...")
	}
	
	if r.result != "" {
		resultStyle := theme.Text
		if r.resultError != nil {
			resultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Error))
		}
		
		content := theme.Title.Render("Response Result") + "\n\n" +
			resultStyle.Render(r.result) + "\n\n" +
			theme.Faint.Render("Press ESC to go back")
		
		return lipgloss.NewStyle().Width(r.app.GetWidth()).Align(lipgloss.Left).Render(content)
	}
	
	// Normal input view
	content := r.RenderTitle() + "\n\n" +
		theme.Subtitle.Render("Enter the issue details:") + "\n\n"
	
	// Issue input
	focusedStyle := theme.Bold.Copy().Foreground(lipgloss.Color(theme.Blue))
	issueLabel := theme.Text.Render("Issue Number: ")
	if r.focusedInput == 0 {
		issueLabel = focusedStyle.Render("Issue Number: ")
	}
	content += issueLabel + r.issueInput.View() + "\n\n"
	
	// Template input
	templateLabel := theme.Text.Render("Template: ")
	if r.focusedInput == 1 {
		templateLabel = focusedStyle.Render("Template: ")
	}
	content += templateLabel + r.templateInput.View() + "\n\n"
	
	// Instructions
	content += theme.Faint.Render("Use ↑/↓ to navigate, E to execute, ESC to go back") + "\n\n"
	
	// Footer
	content += r.RenderFooter()
	
	return lipgloss.NewStyle().Width(r.app.GetWidth()).Align(lipgloss.Left).Render(content)
}

// ShortHelp returns keybindings to be shown in the help menu
func (r *RespondScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		r.app.keyMap.Up,
		r.app.keyMap.Down,
		r.app.keyMap.Execute,
		r.app.keyMap.Back,
		r.app.keyMap.Help,
		r.app.keyMap.Quit,
	}
}

// startExecution begins the execution process
func (r *RespondScreen) startExecution() tea.Cmd {
	return func() tea.Msg {
		// Get the input values
		issueNumber := r.issueInput.Value()
		templateName := r.templateInput.Value()
		
		// Check if executor is available
		if r.executor == nil {
			return executionResultMsg{
				output: "Error: Configuration not found. Please run 'useful1 config' first",
				err:    fmt.Errorf("configuration not found"),
			}
		}
		
		// Execute the command
		err := r.executor.RespondToIssue(issueNumber, templateName)
		
		// Prepare result message
		result := "Successfully responded to issue #" + issueNumber
		if err != nil {
			result = "Error: " + err.Error()
		}
		
		return executionResultMsg{
			output: result,
			err:    err,
		}
	}
}

// executionResultMsg is a message containing the result of command execution
type executionResultMsg struct {
	output string
	err    error
}