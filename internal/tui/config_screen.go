package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/config"
)

// ConfigScreen is the screen for configuring settings
type ConfigScreen struct {
	BaseScreen
	githubTokenInput       textinput.Model
	anthropicTokenInput    textinput.Model
	cliCommandInput        textinput.Model
	issueResponseBudgetInput textinput.Model
	prBudgetInput          textinput.Model
	testBudgetInput        textinput.Model
	defaultBudgetInput     textinput.Model
	inputs                 []textinput.Model
	focusedInput           int
	configurator           *config.Configurator
	executing              bool
	result                 string
	resultError            error
}

// NewConfigScreen creates a new configuration screen
func NewConfigScreen(app *App) *ConfigScreen {
	// GitHub token input
	githubTokenInput := textinput.New()
	githubTokenInput.Placeholder = "GitHub token"
	githubTokenInput.Width = 50
	if app.GetConfig() != nil {
		githubTokenInput.SetValue(app.GetConfig().GitHub.Token)
	}
	githubTokenInput.Focus()
	
	// Anthropic token input
	anthropicTokenInput := textinput.New()
	anthropicTokenInput.Placeholder = "Anthropic API key"
	anthropicTokenInput.Width = 50
	if app.GetConfig() != nil {
		anthropicTokenInput.SetValue(app.GetConfig().Anthropic.Token)
	}
	
	// CLI command input
	cliCommandInput := textinput.New()
	cliCommandInput.Placeholder = "CLI tool path"
	cliCommandInput.Width = 50
	if app.GetConfig() != nil {
		cliCommandInput.SetValue(app.GetConfig().CLI.Command)
	}
	
	// Budget inputs
	issueResponseBudgetInput := textinput.New()
	issueResponseBudgetInput.Placeholder = "Issue response budget"
	issueResponseBudgetInput.Width = 10
	if app.GetConfig() != nil {
		issueResponseBudgetInput.SetValue("10.0") // Default value
	}
	
	prBudgetInput := textinput.New()
	prBudgetInput.Placeholder = "PR creation budget"
	prBudgetInput.Width = 10
	if app.GetConfig() != nil {
		prBudgetInput.SetValue("15.0") // Default value
	}
	
	testBudgetInput := textinput.New()
	testBudgetInput.Placeholder = "Test run budget"
	testBudgetInput.Width = 10
	if app.GetConfig() != nil {
		testBudgetInput.SetValue("5.0") // Default value
	}
	
	defaultBudgetInput := textinput.New()
	defaultBudgetInput.Placeholder = "Default budget"
	defaultBudgetInput.Width = 10
	if app.GetConfig() != nil {
		defaultBudgetInput.SetValue("2.0") // Default value
	}
	
	inputs := []textinput.Model{
		githubTokenInput,
		anthropicTokenInput,
		cliCommandInput,
		issueResponseBudgetInput,
		prBudgetInput,
		testBudgetInput,
		defaultBudgetInput,
	}
	
	return &ConfigScreen{
		BaseScreen:             NewBaseScreen(app, "Configuration"),
		githubTokenInput:       githubTokenInput,
		anthropicTokenInput:    anthropicTokenInput,
		cliCommandInput:        cliCommandInput,
		issueResponseBudgetInput: issueResponseBudgetInput,
		prBudgetInput:          prBudgetInput,
		testBudgetInput:        testBudgetInput,
		defaultBudgetInput:     defaultBudgetInput,
		inputs:                 inputs,
		focusedInput:           0,
		configurator:           config.NewConfigurator(),
		executing:              false,
	}
}

// Init initializes the config screen
func (c *ConfigScreen) Init() tea.Cmd {
	c.executing = false
	c.result = ""
	c.resultError = nil
	return textinput.Blink
}

// Update handles UI updates for the config screen
func (c *ConfigScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if c.executing {
			// Allow going back if we're showing results
			if key.Matches(msg, c.app.keyMap.Back) {
				c.executing = false
				return c, nil
			}
			return c, nil
		}
		
		// Non-execution state key handling
		switch {
		case key.Matches(msg, c.app.keyMap.Back):
			return c, c.app.ChangeScreen(ScreenMainMenu)
		
		case key.Matches(msg, c.app.keyMap.Up, c.app.keyMap.Down):
			// Cycle through inputs
			if key.Matches(msg, c.app.keyMap.Up) {
				c.focusedInput--
				if c.focusedInput < 0 {
					c.focusedInput = len(c.inputs) - 1
				}
			} else {
				c.focusedInput++
				if c.focusedInput >= len(c.inputs) {
					c.focusedInput = 0
				}
			}
			
			// Focus the appropriate input
			for i := 0; i < len(c.inputs); i++ {
				if i == c.focusedInput {
					c.inputs[i].Focus()
				} else {
					c.inputs[i].Blur()
				}
			}
			
			// Update the reference inputs
			c.githubTokenInput = c.inputs[0]
			c.anthropicTokenInput = c.inputs[1]
			c.cliCommandInput = c.inputs[2]
			c.issueResponseBudgetInput = c.inputs[3]
			c.prBudgetInput = c.inputs[4]
			c.testBudgetInput = c.inputs[5]
			c.defaultBudgetInput = c.inputs[6]
			
			return c, nil
			
		case key.Matches(msg, c.app.keyMap.Execute):
			// Validate input
			if c.githubTokenInput.Value() == "" || c.anthropicTokenInput.Value() == "" || c.cliCommandInput.Value() == "" {
				c.result = "Please fill in all required fields"
				return c, nil
			}
			
			// Start execution
			c.executing = true
			return c, c.startExecution()
		}
	
	case executionResultMsg:
		c.result = msg.output
		c.resultError = msg.err
		return c, nil
	}
	
	// Handle input updates for the focused input
	if c.focusedInput >= 0 && c.focusedInput < len(c.inputs) {
		var cmd tea.Cmd
		c.inputs[c.focusedInput], cmd = c.inputs[c.focusedInput].Update(msg)
		
		// Update the reference inputs
		c.githubTokenInput = c.inputs[0]
		c.anthropicTokenInput = c.inputs[1]
		c.cliCommandInput = c.inputs[2]
		c.issueResponseBudgetInput = c.inputs[3]
		c.prBudgetInput = c.inputs[4]
		c.testBudgetInput = c.inputs[5]
		c.defaultBudgetInput = c.inputs[6]
		
		cmds = append(cmds, cmd)
	}
	
	return c, tea.Batch(cmds...)
}

// View renders the config screen
func (c *ConfigScreen) View() string {
	theme := c.app.GetTheme()
	
	if c.executing {
		return theme.Title.Render("Saving configuration...") + "\n\n" +
			theme.Text.Render("Please wait while we save your configuration...")
	}
	
	if c.result != "" {
		resultStyle := theme.Text
		if c.resultError != nil {
			resultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Error))
		}
		
		content := theme.Title.Render("Configuration Result") + "\n\n" +
			resultStyle.Render(c.result) + "\n\n" +
			theme.Faint.Render("Press ESC to go back")
		
		return lipgloss.NewStyle().Width(c.app.GetWidth()).Align(lipgloss.Left).Render(content)
	}
	
	// Normal input view
	content := c.RenderTitle() + "\n\n" +
		theme.Subtitle.Render("Enter configuration details:") + "\n\n"
	
	focusedStyle := theme.Bold.Copy().Foreground(lipgloss.Color(theme.Blue))
	normalStyle := theme.Text
	
	// Authentication section
	content += theme.Bold.Render("Authentication:") + "\n\n"
	
	// GitHub token
	githubLabel := normalStyle.Render("GitHub Token: ")
	if c.focusedInput == 0 {
		githubLabel = focusedStyle.Render("GitHub Token: ")
	}
	content += githubLabel + c.githubTokenInput.View() + "\n\n"
	
	// Anthropic token
	anthropicLabel := normalStyle.Render("Anthropic API Key: ")
	if c.focusedInput == 1 {
		anthropicLabel = focusedStyle.Render("Anthropic API Key: ")
	}
	content += anthropicLabel + c.anthropicTokenInput.View() + "\n\n"
	
	// CLI section
	content += theme.Bold.Render("CLI Tool:") + "\n\n"
	
	// CLI command
	cliLabel := normalStyle.Render("CLI Tool Path: ")
	if c.focusedInput == 2 {
		cliLabel = focusedStyle.Render("CLI Tool Path: ")
	}
	content += cliLabel + c.cliCommandInput.View() + "\n\n"
	
	// Budgets section
	content += theme.Bold.Render("Budgets:") + "\n\n"
	
	// Issue response budget
	issueLabel := normalStyle.Render("Issue Response Budget: ")
	if c.focusedInput == 3 {
		issueLabel = focusedStyle.Render("Issue Response Budget: ")
	}
	content += issueLabel + c.issueResponseBudgetInput.View() + "\n\n"
	
	// PR budget
	prLabel := normalStyle.Render("PR Creation Budget: ")
	if c.focusedInput == 4 {
		prLabel = focusedStyle.Render("PR Creation Budget: ")
	}
	content += prLabel + c.prBudgetInput.View() + "\n\n"
	
	// Test budget
	testLabel := normalStyle.Render("Test Run Budget: ")
	if c.focusedInput == 5 {
		testLabel = focusedStyle.Render("Test Run Budget: ")
	}
	content += testLabel + c.testBudgetInput.View() + "\n\n"
	
	// Default budget
	defaultLabel := normalStyle.Render("Default Budget: ")
	if c.focusedInput == 6 {
		defaultLabel = focusedStyle.Render("Default Budget: ")
	}
	content += defaultLabel + c.defaultBudgetInput.View() + "\n\n"
	
	// Instructions
	content += theme.Faint.Render("Use ↑/↓ to navigate, E to save configuration, ESC to go back") + "\n\n"
	
	// Footer
	content += c.RenderFooter()
	
	return lipgloss.NewStyle().MaxWidth(c.app.GetWidth()).Render(content)
}

// ShortHelp returns keybindings to be shown in the help menu
func (c *ConfigScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		c.app.keyMap.Up,
		c.app.keyMap.Down,
		c.app.keyMap.Execute,
		c.app.keyMap.Back,
		c.app.keyMap.Help,
		c.app.keyMap.Quit,
	}
}

// startExecution begins the execution process
func (c *ConfigScreen) startExecution() tea.Cmd {
	return func() tea.Msg {
		// Set configuration values
		c.configurator.SetGitHubToken(c.githubTokenInput.Value())
		c.configurator.SetGitHubUser("user") // Hardcoded for now, would be replaced with actual user in real implementation
		c.configurator.SetAnthropicToken(c.anthropicTokenInput.Value())
		c.configurator.SetCLIToolPath(c.cliCommandInput.Value())
		
		// Set budgets
		budgets := make(map[string]float64)
		budgets["issue_response"] = 10.0 // Parse from input in real implementation
		budgets["pr_creation"] = 15.0
		budgets["test_run"] = 5.0
		budgets["default"] = 2.0
		c.configurator.SetTaskBudgets(budgets)
		
		// Set default monitoring settings
		c.configurator.SetMonitoringSettings(30, true, []string{})
		
		// Save configuration
		err := c.configurator.Save()
		
		// Prepare result message
		result := "Configuration saved successfully"
		if err != nil {
			result = "Error saving configuration: " + err.Error()
		}
		
		return executionResultMsg{
			output: result,
			err:    err,
		}
	}
}