package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// ExecuteScreen represents the execute screen
type ExecuteScreen struct {
	BaseScreen
	argsInput textinput.Model
	output    string
	executing bool
}

// NewExecuteScreen creates a new execute screen
func NewExecuteScreen(app *App) Screen {
	es := &ExecuteScreen{
		BaseScreen: NewBaseScreen(app, "Execute CLI Tool"),
	}

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Enter arguments for CLI tool here and press Enter"
	ti.CharLimit = 200 // Increased to allow more arguments
	ti.Width = 50
	ti.Focus()
	es.argsInput = ti

	return es
}

// Init initializes the screen
func (e *ExecuteScreen) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles UI updates
func (e *ExecuteScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// First check if it's a shortcut key
		switch {
		case key.Matches(msg, e.app.keyMap.Back):
			return e, e.app.ChangeScreen(ScreenMainMenu)
		// Only match the Execute key if the key isn't being captured by the input field
		// or when the input field is not focused
		case !e.argsInput.Focused() && key.Matches(msg, e.app.keyMap.Execute) && !e.executing:
			// Execute command with entered arguments
			return e, e.executeCommand()
		case key.Matches(msg, e.app.keyMap.Select) && !e.executing:
			// Execute when pressing Enter
			return e, e.executeCommand()
		}

	case ExecuteOutputMsg:
		e.output = msg.Output
		e.executing = false
		return e, nil

	case ExecuteErrorMsg:
		e.output = fmt.Sprintf("Error: %s", msg.Error)
		e.executing = false
		return e, nil
	}

	if !e.executing {
		// Only update the input if we're not executing
		var cmd tea.Cmd
		e.argsInput, cmd = e.argsInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return e, tea.Batch(cmds...)
}

// View renders the UI
func (e *ExecuteScreen) View() string {
	// Create view with border
	title := e.RenderTitle()

	// Get CLI command from config
	cliCommand := "No CLI command configured"
	if e.app.config != nil && e.app.config.CLI.Command != "" {
		cliCommand = e.app.config.CLI.Command

		// Show with any configured default arguments
		if len(e.app.config.CLI.Args) > 0 {
			defaultArgs := strings.Join(e.app.config.CLI.Args, " ")
			cliCommand = fmt.Sprintf("%s %s", cliCommand, defaultArgs)
		}
	}

	// Display the command that will be executed
	commandInfo := e.app.theme.Bold.Render("Command: ") + e.app.theme.Text.Render(cliCommand)

	// Input field for arguments
	inputField := fmt.Sprintf("\n%s\n%s",
		e.app.theme.Bold.Render("Arguments:"),
		e.argsInput.View())

	// Help text
	helpText := "\nType your arguments above and press Enter to execute the command\n"
	helpText += e.app.theme.Faint.Render("Example: -p \"something\" to pass -p something to the CLI tool")
	helpText += "\n"
	helpText += e.app.theme.Faint.Render("The command will run interactively in your terminal")

	if e.executing {
		helpText = "\nPreparing interactive command execution...\n"
	}

	// Output
	outputSection := ""
	if e.output != "" {
		outputSection = fmt.Sprintf("\n%s\n%s",
			e.app.theme.Bold.Render("Output:"),
			e.app.theme.Text.Render(e.output))
	}

	// Footer
	footer := e.RenderFooter()

	// Combine all sections
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		commandInfo,
		inputField,
		helpText,
		outputSection,
		"",
		footer,
	)

	// Add border
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Render(content)
}

// ShortHelp returns keybindings
func (e *ExecuteScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		e.app.keyMap.Execute,
		e.app.keyMap.Select,
		e.app.keyMap.Back,
		e.app.keyMap.Help,
		e.app.keyMap.Quit,
	}
}

// executeCommand executes the CLI tool
func (e *ExecuteScreen) executeCommand() tea.Cmd {
	if e.app.config == nil || e.app.config.CLI.Command == "" {
		return func() tea.Msg {
			return ExecuteErrorMsg{Error: "No CLI tool configured"}
		}
	}

	e.executing = true

	// Parse the input into args, respecting quoted strings
	input := e.argsInput.Value()
	var args []string

	// Use a more sophisticated parsing approach to handle quoted arguments
	if input != "" {
		// Simple state machine to parse arguments with quoted strings
		var currentArg strings.Builder
		inQuotes := false
		inSingleQuotes := false

		for i := 0; i < len(input); i++ {
			c := input[i]

			switch c {
			case '"':
				if !inSingleQuotes {
					inQuotes = !inQuotes
				} else {
					currentArg.WriteByte(c)
				}
			case '\'':
				if !inQuotes {
					inSingleQuotes = !inSingleQuotes
				} else {
					currentArg.WriteByte(c)
				}
			case ' ', '\t':
				if !inQuotes && !inSingleQuotes {
					// Finish current argument if not empty
					if currentArg.Len() > 0 {
						args = append(args, currentArg.String())
						currentArg.Reset()
					}
				} else {
					// Add space to current argument if in quotes
					currentArg.WriteByte(c)
				}
			default:
				currentArg.WriteByte(c)
			}
		}

		// Add the last argument if not empty
		if currentArg.Len() > 0 {
			args = append(args, currentArg.String())
		}
	}

	// Show a message indicating we're switching to CLI mode for interactive output
	return func() tea.Msg {
		// Get CLI executor
		executor := e.app.CreateCLIExecutor()
		if executor == nil {
			return ExecuteErrorMsg{Error: "Failed to create CLI executor"}
		}

		// Log that we're executing the command
		logging.Info("Executing CLI command in interactive mode",
			"command", e.app.config.CLI.Command,
			"args", args)

		// Set a message to inform the user
		e.output = "Switching to interactive mode for command execution...\n"

		// We need to temporarily exit the TUI to allow direct terminal interaction
		// This creates a full-screen interactive view
		_ = tea.ExitAltScreen()

		fmt.Println("\n--- Interactive command execution ---")
		fmt.Printf("Running: %s %s\n\n", e.app.config.CLI.Command, strings.Join(args, " "))

		// Execute the command interactively using Execute instead of ExecuteWithOutput
		err := executor.Execute(args)

		// Print completion message
		if err != nil {
			fmt.Printf("\n--- Command execution failed: %s ---\n", err)
		} else {
			fmt.Println("\n--- Command completed successfully ---")
		}
		fmt.Println("Press Enter to return to TUI...")

		// Wait for user to press Enter
		if _, scanErr := fmt.Scanln(); scanErr != nil {
			logging.Warn("Error waiting for user input", "error", scanErr)
		}

		// Return to alt screen
		_ = tea.EnterAltScreen()

		// Return result
		if err != nil {
			return ExecuteErrorMsg{Error: err.Error()}
		}
		return ExecuteOutputMsg{Output: "Command executed in interactive mode. See terminal output for results."}
	}
}

// ExecuteOutputMsg is a message containing the output of a command
type ExecuteOutputMsg struct {
	Output string
}

// ExecuteErrorMsg is a message containing an error
type ExecuteErrorMsg struct {
	Error string
}
