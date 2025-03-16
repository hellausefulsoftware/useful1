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
	githubTokenInput         textinput.Model
	anthropicTokenInput      textinput.Model
	cliCommandInput          textinput.Model
	issueResponseBudgetInput textinput.Model
	prBudgetInput            textinput.Model
	testBudgetInput          textinput.Model
	defaultBudgetInput       textinput.Model
	inputs                   []textinput.Model
	focusedInput             int
	menuItems                []string
	selectedMenuItem         int
	inMenuSelection          bool
	configurator             *config.Configurator
	executing                bool
	result                   string
	resultError              error
	showGithubHelp           bool
	showAnthropicHelp        bool
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
	cliCommandInput.Placeholder = "CLI tool path (default: claude --dangerously-skip-permissions)"
	cliCommandInput.Width = 70
	if app.GetConfig() != nil && app.GetConfig().CLI.Command != "" {
		cliCommandInput.SetValue(app.GetConfig().CLI.Command)
	} else {
		cliCommandInput.SetValue("claude --dangerously-skip-permissions")
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

	// Define menu items
	menuItems := []string{
		"GitHub Token Help - Get help generating a GitHub token",
		"Anthropic API Key Help - Get help generating an Anthropic API key",
		"Save Configuration - Save your settings",
	}

	return &ConfigScreen{
		BaseScreen:               NewBaseScreen(app, "Configuration"),
		githubTokenInput:         githubTokenInput,
		anthropicTokenInput:      anthropicTokenInput,
		cliCommandInput:          cliCommandInput,
		issueResponseBudgetInput: issueResponseBudgetInput,
		prBudgetInput:            prBudgetInput,
		testBudgetInput:          testBudgetInput,
		defaultBudgetInput:       defaultBudgetInput,
		inputs:                   inputs,
		focusedInput:             0,
		menuItems:                menuItems,
		selectedMenuItem:         0,
		inMenuSelection:          false,
		configurator:             config.NewConfigurator(),
		executing:                false,
		showGithubHelp:           false,
		showAnthropicHelp:        false,
	}
}

// Init initializes the config screen
func (c *ConfigScreen) Init() tea.Cmd {
	c.executing = false
	c.result = ""
	c.resultError = nil
	c.showGithubHelp = false
	c.showAnthropicHelp = false
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
			// If showing help, hide it instead of going back
			if c.showGithubHelp || c.showAnthropicHelp {
				c.showGithubHelp = false
				c.showAnthropicHelp = false
				return c, nil
			}

			// If in menu selection mode, go back to normal input mode
			if c.inMenuSelection {
				c.inMenuSelection = false
				return c, nil
			}

			// First check if all required fields are filled - if yes, save config before going back
			if c.githubTokenInput.Value() != "" && c.anthropicTokenInput.Value() != "" && c.cliCommandInput.Value() != "" {
				c.executing = true
				return c, c.startExecution()
			}
			return c, c.app.ChangeScreen(ScreenMainMenu)

		case key.Matches(msg, c.app.keyMap.Up, c.app.keyMap.Down):
			// If showing help, don't change focus
			if c.showGithubHelp || c.showAnthropicHelp {
				return c, nil
			}

			// Handle menu item selection if in menu mode
			if c.inMenuSelection {
				if key.Matches(msg, c.app.keyMap.Up) {
					c.selectedMenuItem--
					if c.selectedMenuItem < 0 {
						c.selectedMenuItem = len(c.menuItems) - 1
					}
				} else {
					c.selectedMenuItem++
					if c.selectedMenuItem >= len(c.menuItems) {
						c.selectedMenuItem = 0
					}
				}
				return c, nil
			}

			// Cycle through inputs in normal mode
			if key.Matches(msg, c.app.keyMap.Up) {
				c.focusedInput--
				if c.focusedInput < -1 { // -1 to select menu items
					c.focusedInput = len(c.inputs) - 1
				} else if c.focusedInput == -1 {
					// Switch to menu selection mode
					c.inMenuSelection = true
					return c, nil
				}
			} else {
				c.focusedInput++
				if c.focusedInput >= len(c.inputs) {
					c.focusedInput = -1 // -1 to select menu items
					// Switch to menu selection mode
					c.inMenuSelection = true
					return c, nil
				}
			}

			// Focus the appropriate input if not in menu selection
			if !c.inMenuSelection && c.focusedInput >= 0 {
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
			}

			return c, nil

		case key.Matches(msg, c.app.keyMap.Select):
			// Handle menu item selection if in menu mode
			if c.inMenuSelection {
				switch c.selectedMenuItem {
				case 0: // GitHub Token Help
					c.showGithubHelp = true
					c.showAnthropicHelp = false
					c.inMenuSelection = false
				case 1: // Anthropic API Key Help
					c.showAnthropicHelp = true
					c.showGithubHelp = false
					c.inMenuSelection = false
				case 2: // Save Configuration
					// Validate input
					if c.githubTokenInput.Value() == "" || c.anthropicTokenInput.Value() == "" || c.cliCommandInput.Value() == "" {
						c.result = "Please fill in all required fields"
						c.inMenuSelection = false
						return c, nil
					}

					// Start execution
					c.executing = true
					c.inMenuSelection = false
					return c, c.startExecution()
				}
				return c, nil
			}
			return c, nil
		}

	case executionResultMsg:
		c.result = msg.output
		c.resultError = msg.err

		// If save was successful, return to main menu immediately
		if c.resultError == nil {
			return c, c.app.ChangeScreen(ScreenMainMenu)
		}
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

	// Show GitHub help screen if enabled
	if c.showGithubHelp {
		content := theme.Title.Render("GitHub Token Guide") + "\n\n" +
			theme.Bold.Render("To get a GitHub Personal Access Token:") + "\n\n" +
			theme.Text.Render("1. Go to https://github.com/settings/tokens") + "\n" +
			theme.Text.Render("2. Click 'Generate new token' / 'Generate new token (classic)'") + "\n" +
			theme.Text.Render("3. Give it a name like 'Useful1 CLI'") + "\n" +
			theme.Text.Render("4. Set an expiration period (e.g., 90 days)") + "\n" +
			theme.Text.Render("5. Select these scopes:") + "\n" +
			theme.Text.Render("   - repo (all)") + "\n" +
			theme.Text.Render("   - workflow") + "\n" +
			theme.Text.Render("   - read:org") + "\n" +
			theme.Text.Render("   - user") + "\n" +
			theme.Text.Render("6. Click 'Generate token'") + "\n" +
			theme.Text.Render("7. Copy the generated token (you will only see it once!)") + "\n\n" +
			theme.Bold.Render("The token should look something like:") + "\n" +
			theme.Text.Render("ghp_1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p7q8r") + "\n\n" +
			theme.Faint.Render("Press ESC to go back to the configuration screen")

		return lipgloss.NewStyle().Width(c.app.GetWidth()).Align(lipgloss.Left).Render(content)
	}

	// Show Anthropic help screen if enabled
	if c.showAnthropicHelp {
		content := theme.Title.Render("Anthropic API Key Guide") + "\n\n" +
			theme.Bold.Render("To get an Anthropic API Key:") + "\n\n" +
			theme.Text.Render("1. Go to https://console.anthropic.com/") + "\n" +
			theme.Text.Render("2. Create an account or sign in") + "\n" +
			theme.Text.Render("3. Navigate to 'API Keys' in the dashboard") + "\n" +
			theme.Text.Render("4. Click 'Create Key'") + "\n" +
			theme.Text.Render("5. Give it a name like 'Useful1 CLI'") + "\n" +
			theme.Text.Render("6. Copy the generated API key (you will only see it once!)") + "\n\n" +
			theme.Bold.Render("The API key should look something like:") + "\n" +
			theme.Text.Render("sk-ant-api03-xxxxxxxxxxxx") + "\n\n" +
			theme.Faint.Render("Press ESC to go back to the configuration screen")

		return lipgloss.NewStyle().Width(c.app.GetWidth()).Align(lipgloss.Left).Render(content)
	}

	// Normal input view
	content := c.RenderTitle() + "\n\n"

	focusedStyle := theme.Bold.Foreground(lipgloss.Color(theme.Blue))
	normalStyle := theme.Text

	// Menu items at the top
	content += theme.Bold.Render("Options:") + "\n\n"

	// Render menu items
	for i, item := range c.menuItems {
		menuStyle := normalStyle
		if c.inMenuSelection && i == c.selectedMenuItem {
			menuStyle = focusedStyle
		}
		content += menuStyle.Render("• "+item) + "\n"
	}
	content += "\n"

	// Input form section
	content += theme.Subtitle.Render("Enter configuration details:") + "\n\n"

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
	content += theme.Bold.Render("CLI Command:") + "\n\n"

	// CLI command
	cliLabel := normalStyle.Render("CLI Command: ")
	if c.focusedInput == 2 {
		cliLabel = focusedStyle.Render("CLI Command: ")
	}
	content += cliLabel + c.cliCommandInput.View() + "\n" +
		theme.Faint.Render("    Enter command with arguments (default: claude --dangerously-skip-permissions)") + "\n\n"

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
	content += theme.Faint.Render("Use ↑/↓ to navigate, Enter to select option, ESC to save and go back") + "\n" +
		theme.Faint.Render("Navigate to top menu options for help and to save configuration") + "\n\n"

	// Footer
	content += c.RenderFooter()

	return lipgloss.NewStyle().Width(c.app.GetWidth()).Align(lipgloss.Left).Render(content)
}

// ShortHelp returns keybindings to be shown in the help menu
func (c *ConfigScreen) ShortHelp() []key.Binding {
	keys := []key.Binding{
		c.app.keyMap.Up,
		c.app.keyMap.Down,
		c.app.keyMap.Select,
		c.app.keyMap.Back,
		c.app.keyMap.Help,
		c.app.keyMap.Quit,
	}

	return keys
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
