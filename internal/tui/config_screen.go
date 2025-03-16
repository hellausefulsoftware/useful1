package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/github"
)

// Field indices for the config screen
const (
	fieldGitHubToken = iota
	fieldGitHubUsername
	fieldAnthropicToken
	fieldCLICommand
	fieldMonitorInterval
	fieldIssueResponseBudget
	fieldPRBudget
	fieldTestBudget
	fieldDefaultBudget
	fieldCount // Total number of fields
)

// ConfigScreen is the screen for configuring settings
type ConfigScreen struct {
	BaseScreen
	githubTokenInput         textinput.Model
	githubUsernameInput      textinput.Model
	anthropicTokenInput      textinput.Model
	cliCommandInput          textinput.Model
	monitorIntervalInput     textinput.Model
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

// updateInputReferences updates all the input field references from the inputs slice
func (c *ConfigScreen) updateInputReferences() {
	c.githubTokenInput = c.inputs[fieldGitHubToken]
	c.githubUsernameInput = c.inputs[fieldGitHubUsername]
	c.anthropicTokenInput = c.inputs[fieldAnthropicToken]
	c.cliCommandInput = c.inputs[fieldCLICommand]
	c.monitorIntervalInput = c.inputs[fieldMonitorInterval]
	c.issueResponseBudgetInput = c.inputs[fieldIssueResponseBudget]
	c.prBudgetInput = c.inputs[fieldPRBudget]
	c.testBudgetInput = c.inputs[fieldTestBudget]
	c.defaultBudgetInput = c.inputs[fieldDefaultBudget]
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

	// GitHub username input
	githubUsernameInput := textinput.New()
	githubUsernameInput.Placeholder = "GitHub username (required for monitoring)"
	githubUsernameInput.Width = 50
	if app.GetConfig() != nil && app.GetConfig().GitHub.User != "" {
		// Don't show the placeholder text
		if app.GetConfig().GitHub.User != "ENTER_GITHUB_USERNAME_HERE" {
			githubUsernameInput.SetValue(app.GetConfig().GitHub.User)
		}
	}

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

	// Monitor interval input (in seconds)
	monitorIntervalInput := textinput.New()
	monitorIntervalInput.Placeholder = "Monitor poll interval in seconds (default: 60)"
	monitorIntervalInput.Width = 10
	if app.GetConfig() != nil {
		// Config stores interval in minutes, convert to seconds
		intervalSecs := app.GetConfig().Monitor.PollInterval * 60
		// If it's 0 or not set, use the default
		if intervalSecs == 0 {
			intervalSecs = 60 // default to 60 seconds
		}
		monitorIntervalInput.SetValue(fmt.Sprintf("%d", intervalSecs))
	} else {
		monitorIntervalInput.SetValue("60") // default to 60 seconds
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
		githubUsernameInput,
		anthropicTokenInput,
		cliCommandInput,
		monitorIntervalInput,
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

	// Verify we have the right number of inputs
	if len(inputs) != fieldCount {
		panic(fmt.Sprintf("Input field count mismatch: expected %d, got %d", fieldCount, len(inputs)))
	}

	return &ConfigScreen{
		BaseScreen:               NewBaseScreen(app, "Configuration"),
		githubTokenInput:         githubTokenInput,
		githubUsernameInput:      githubUsernameInput,
		anthropicTokenInput:      anthropicTokenInput,
		cliCommandInput:          cliCommandInput,
		monitorIntervalInput:     monitorIntervalInput,
		issueResponseBudgetInput: issueResponseBudgetInput,
		prBudgetInput:            prBudgetInput,
		testBudgetInput:          testBudgetInput,
		defaultBudgetInput:       defaultBudgetInput,
		inputs:                   inputs,
		focusedInput:             fieldGitHubToken,
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
				// Warn if GitHub username is missing (important for monitoring)
				if c.githubUsernameInput.Value() == "" {
					c.githubUsernameInput.SetValue("")   // Clear any value to show placeholder
					c.focusedInput = fieldGitHubUsername // Focus the GitHub username field
					// Update focus
					for i := 0; i < fieldCount; i++ {
						if i == c.focusedInput {
							c.inputs[i].Focus()
						} else {
							c.inputs[i].Blur()
						}
					}
					// Update input references
					c.updateInputReferences()
					return c, nil
				}
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
				// Only move up if not already at the first field
				if c.focusedInput > 0 {
					c.focusedInput--
				} else {
					// At the top field already, do nothing
					return c, nil
				}
			} else {
				// Only move down if not already at the last field
				if c.focusedInput < fieldCount-1 {
					c.focusedInput++
				} else {
					// At the bottom field already, do nothing
					return c, nil
				}
			}

			// Focus the appropriate input if not in menu selection
			if !c.inMenuSelection && c.focusedInput >= 0 {
				for i := 0; i < fieldCount; i++ {
					if i == c.focusedInput {
						c.inputs[i].Focus()
					} else {
						c.inputs[i].Blur()
					}
				}

				// Update the reference inputs
				c.updateInputReferences()
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
	if c.focusedInput >= 0 && c.focusedInput < fieldCount {
		var cmd tea.Cmd
		c.inputs[c.focusedInput], cmd = c.inputs[c.focusedInput].Update(msg)

		// Update the reference inputs
		c.updateInputReferences()

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
		content += menuStyle.Render(item) + "\n"
	}
	content += "\n"

	// Input form section
	content += theme.Subtitle.Render("Enter configuration details:") + "\n\n"

	// Authentication section
	content += theme.Bold.Render("Authentication:") + "\n\n"

	// GitHub token
	githubLabel := normalStyle.Render("GitHub Token")
	if c.focusedInput == fieldGitHubToken {
		githubLabel = focusedStyle.Render("GitHub Token")
	}
	content += githubLabel + "\n" + c.githubTokenInput.View() + "\n\n"

	// GitHub username
	usernameLabel := normalStyle.Render("GitHub Username")
	if c.focusedInput == fieldGitHubUsername {
		usernameLabel = focusedStyle.Render("GitHub Username")
	}
	content += usernameLabel + "\n" + c.githubUsernameInput.View() + "\n" +
		theme.Faint.Render("Required for monitoring assigned issues") + "\n\n"

	// Anthropic token
	anthropicLabel := normalStyle.Render("Anthropic API Key")
	if c.focusedInput == fieldAnthropicToken {
		anthropicLabel = focusedStyle.Render("Anthropic API Key")
	}
	content += anthropicLabel + "\n" + c.anthropicTokenInput.View() + "\n\n"

	// CLI section
	content += theme.Bold.Render("CLI Command:") + "\n\n"

	// CLI command
	cliLabel := normalStyle.Render("CLI Command")
	if c.focusedInput == fieldCLICommand {
		cliLabel = focusedStyle.Render("CLI Command")
	}
	content += cliLabel + "\n" + c.cliCommandInput.View() + "\n" +
		theme.Faint.Render("Enter command with arguments (default: claude --dangerously-skip-permissions)") + "\n\n"

	// Monitor Settings section
	content += theme.Bold.Render("Monitor Settings:") + "\n\n"

	// Poll interval
	intervalLabel := normalStyle.Render("Poll Interval (seconds)")
	if c.focusedInput == fieldMonitorInterval {
		intervalLabel = focusedStyle.Render("Poll Interval (seconds)")
	}
	content += intervalLabel + "\n" + c.monitorIntervalInput.View() + "\n" +
		theme.Faint.Render("How often to check for new issues (default: 60 seconds)") + "\n\n"

	// Budgets section
	content += theme.Bold.Render("Budgets:") + "\n\n"

	// Issue response budget
	issueLabel := normalStyle.Render("Issue Response Budget")
	if c.focusedInput == fieldIssueResponseBudget {
		issueLabel = focusedStyle.Render("Issue Response Budget")
	}
	content += issueLabel + "\n" + c.issueResponseBudgetInput.View() + "\n\n"

	// PR budget
	prLabel := normalStyle.Render("PR Creation Budget")
	if c.focusedInput == fieldPRBudget {
		prLabel = focusedStyle.Render("PR Creation Budget")
	}
	content += prLabel + "\n" + c.prBudgetInput.View() + "\n\n"

	// Test budget
	testLabel := normalStyle.Render("Test Run Budget")
	if c.focusedInput == fieldTestBudget {
		testLabel = focusedStyle.Render("Test Run Budget")
	}
	content += testLabel + "\n" + c.testBudgetInput.View() + "\n\n"

	// Default budget
	defaultLabel := normalStyle.Render("Default Budget")
	if c.focusedInput == fieldDefaultBudget {
		defaultLabel = focusedStyle.Render("Default Budget")
	}
	content += defaultLabel + "\n" + c.defaultBudgetInput.View() + "\n\n"

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
		githubToken := c.githubTokenInput.Value()
		c.configurator.SetGitHubToken(githubToken)
		c.configurator.SetAnthropicToken(c.anthropicTokenInput.Value())
		c.configurator.SetCLIToolPath(c.cliCommandInput.Value())

		// Get GitHub username from input field first
		username := c.githubUsernameInput.Value()

		// If input field is empty, try to get from API
		if username == "" {
			if githubToken != "" {
				// Create a temporary GitHub client to get user info
				tempClient := github.NewClient(githubToken)
				if user, err := tempClient.GetUserInfo(); err == nil && user.GetLogin() != "" {
					username = user.GetLogin()
				}
			}

			// If still empty, use placeholder
			if username == "" {
				username = "ENTER_GITHUB_USERNAME_HERE"
			}
		}

		c.configurator.SetGitHubUser(username)

		// Parse budget values from inputs
		budgets := make(map[string]float64)

		// Parse issue response budget
		issueResponseBudget, err := strconv.ParseFloat(c.issueResponseBudgetInput.Value(), 64)
		if err == nil {
			budgets["issue_response"] = issueResponseBudget
		} else {
			budgets["issue_response"] = 10.0 // Default if parsing fails
		}

		// Parse PR creation budget
		prBudget, err := strconv.ParseFloat(c.prBudgetInput.Value(), 64)
		if err == nil {
			budgets["pr_creation"] = prBudget
		} else {
			budgets["pr_creation"] = 15.0 // Default if parsing fails
		}

		// Parse test run budget
		testBudget, err := strconv.ParseFloat(c.testBudgetInput.Value(), 64)
		if err == nil {
			budgets["test_run"] = testBudget
		} else {
			budgets["test_run"] = 5.0 // Default if parsing fails
		}

		// Parse default budget
		defaultBudget, err := strconv.ParseFloat(c.defaultBudgetInput.Value(), 64)
		if err == nil {
			budgets["default"] = defaultBudget
		} else {
			budgets["default"] = 2.0 // Default if parsing fails
		}

		c.configurator.SetTaskBudgets(budgets)

		// Set monitoring settings from the current config or defaults
		// Get poll interval in seconds, convert to minutes for storage
		pollIntervalSecs := 60 // Default to 60 seconds
		if c.monitorIntervalInput.Value() != "" {
			if val, intervalErr := strconv.Atoi(c.monitorIntervalInput.Value()); intervalErr == nil && val > 0 {
				pollIntervalSecs = val
			}
		}
		// Convert to minutes (rounded up to nearest minute)
		pollIntervalMins := (pollIntervalSecs + 59) / 60

		repoFilter := []string{}
		assignedOnly := true // Always monitor assigned issues only

		c.configurator.SetMonitoringSettings(pollIntervalMins, repoFilter, assignedOnly)

		// Save configuration
		err = c.configurator.Save()

		// Prepare result message
		result := "Configuration saved successfully"
		if err != nil {
			result = "Error saving configuration: " + err.Error()
		}

		return executionResultMsg{
			output: result,
			err:    nil,
		}
	}
}
