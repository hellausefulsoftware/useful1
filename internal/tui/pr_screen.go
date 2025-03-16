package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/cli"
)

// PRScreen is the screen for creating pull requests
type PRScreen struct {
	BaseScreen
	branchInput  textinput.Model
	baseInput    textinput.Model
	titleInput   textinput.Model
	focusedInput int
	executor     *cli.Executor
	executing    bool
	result       string
	resultError  error
}

// NewPRScreen creates a new PR screen
func NewPRScreen(app *App) *PRScreen {
	branchInput := textinput.New()
	branchInput.Placeholder = "Branch name"
	branchInput.Focus()
	branchInput.Width = 30
	
	baseInput := textinput.New()
	baseInput.Placeholder = "Base branch (default: main)"
	baseInput.Width = 30
	baseInput.SetValue("main")
	
	titleInput := textinput.New()
	titleInput.Placeholder = "PR title (optional)"
	titleInput.Width = 50
	
	var executor *cli.Executor
	if app.GetConfig() != nil {
		executor = cli.NewExecutor(app.GetConfig())
	}
	
	return &PRScreen{
		BaseScreen:   NewBaseScreen(app, "Create Pull Request"),
		branchInput:  branchInput,
		baseInput:    baseInput,
		titleInput:   titleInput,
		focusedInput: 0,
		executor:     executor,
		executing:    false,
	}
}

// Init initializes the PR screen
func (p *PRScreen) Init() tea.Cmd {
	p.executing = false
	p.result = ""
	p.resultError = nil
	return textinput.Blink
}

// Update handles UI updates for the PR screen
func (p *PRScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if p.executing {
			// Allow going back if we're showing results
			if key.Matches(msg, p.app.keyMap.Back) {
				p.executing = false
				return p, nil
			}
			return p, nil
		}
		
		// Non-execution state key handling
		switch {
		case key.Matches(msg, p.app.keyMap.Back):
			return p, p.app.ChangeScreen(ScreenMainMenu)
		
		case key.Matches(msg, p.app.keyMap.Up, p.app.keyMap.Down):
			// Cycle through inputs
			if key.Matches(msg, p.app.keyMap.Up) {
				p.focusedInput--
				if p.focusedInput < 0 {
					p.focusedInput = 2
				}
			} else {
				p.focusedInput++
				if p.focusedInput > 2 {
					p.focusedInput = 0
				}
			}
			
			// Focus the appropriate input
			p.branchInput.Blur()
			p.baseInput.Blur()
			p.titleInput.Blur()
			
			switch p.focusedInput {
			case 0:
				p.branchInput.Focus()
			case 1: 
				p.baseInput.Focus()
			case 2:
				p.titleInput.Focus()
			}
			
			return p, nil
			
		case key.Matches(msg, p.app.keyMap.Execute):
			// Validate input
			if p.branchInput.Value() == "" {
				p.result = "Please enter a branch name"
				return p, nil
			}
			
			// Start execution
			p.executing = true
			return p, p.startExecution()
		}
	
	case executionResultMsg:
		p.result = msg.output
		p.resultError = msg.err
		return p, nil
	}
	
	// Handle input updates
	switch p.focusedInput {
	case 0:
		p.branchInput, cmd = p.branchInput.Update(msg)
	case 1:
		p.baseInput, cmd = p.baseInput.Update(msg)
	case 2:
		p.titleInput, cmd = p.titleInput.Update(msg)
	}
	cmds = append(cmds, cmd)
	
	return p, tea.Batch(cmds...)
}

// View renders the PR screen
func (p *PRScreen) View() string {
	theme := p.app.GetTheme()
	
	if p.executing && p.result == "" {
		return theme.Title.Render("Creating pull request...") + "\n\n" +
			theme.Text.Render("Please wait while we process your request...")
	}
	
	if p.result != "" {
		resultStyle := theme.Text
		if p.resultError != nil {
			resultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Error))
		}
		
		content := theme.Title.Render("Pull Request Result") + "\n\n" +
			resultStyle.Render(p.result) + "\n\n" +
			theme.Faint.Render("Press ESC to go back")
		
		return lipgloss.NewStyle().Width(p.app.GetWidth()).Align(lipgloss.Center).Render(content)
	}
	
	// Normal input view
	content := p.RenderTitle() + "\n\n" +
		theme.Subtitle.Render("Enter the pull request details:") + "\n\n"
	
	// Branch input
	focusedStyle := theme.Bold.Copy().Foreground(lipgloss.Color(theme.Blue))
	
	branchLabel := theme.Text.Render("Branch: ")
	if p.focusedInput == 0 {
		branchLabel = focusedStyle.Render("Branch: ")
	}
	content += branchLabel + p.branchInput.View() + "\n\n"
	
	// Base input
	baseLabel := theme.Text.Render("Base Branch: ")
	if p.focusedInput == 1 {
		baseLabel = focusedStyle.Render("Base Branch: ")
	}
	content += baseLabel + p.baseInput.View() + "\n\n"
	
	// Title input
	titleLabel := theme.Text.Render("Title (optional): ")
	if p.focusedInput == 2 {
		titleLabel = focusedStyle.Render("Title (optional): ")
	}
	content += titleLabel + p.titleInput.View() + "\n\n"
	
	// Instructions
	content += theme.Faint.Render("Use ↑/↓ to navigate, E to execute, ESC to go back") + "\n\n"
	
	// Footer
	content += p.RenderFooter()
	
	return lipgloss.NewStyle().Width(p.app.GetWidth()).Align(lipgloss.Center).Render(content)
}

// ShortHelp returns keybindings to be shown in the help menu
func (p *PRScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		p.app.keyMap.Up,
		p.app.keyMap.Down,
		p.app.keyMap.Execute,
		p.app.keyMap.Back,
		p.app.keyMap.Help,
		p.app.keyMap.Quit,
	}
}

// startExecution begins the execution process
func (p *PRScreen) startExecution() tea.Cmd {
	return func() tea.Msg {
		// Get the input values
		branch := p.branchInput.Value()
		base := p.baseInput.Value()
		title := p.titleInput.Value()
		
		// Check if executor is available
		if p.executor == nil {
			return executionResultMsg{
				output: "Error: Configuration not found. Please run 'useful1 config' first",
				err:    fmt.Errorf("configuration not found"),
			}
		}
		
		// Execute the command
		err := p.executor.CreatePullRequest(branch, base, title)
		
		// Prepare result message
		result := "Successfully created pull request"
		if err != nil {
			result = "Error: " + err.Error()
		}
		
		return executionResultMsg{
			output: result,
			err:    err,
		}
	}
}