package main

import (
	"fmt"
	"os"

	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	// Initialize logger with default configuration
	logging.Initialize(nil)

	// Define program-wide flags
	var programmatic bool
	var logLevel string
	var logJSON bool

	rootCmd := &cobra.Command{
		Use:   "useful1",
		Short: "Automates GitHub tasks via CLI tool integration",
		Long:  `A CLI application with a colorblind-friendly TUI that automates GitHub operations like issue responses, PR creation, and test execution.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Run the TUI app as default unless programmatic mode is enabled
			if programmatic {
				fmt.Println("Running in programmatic mode. Use a subcommand.")
				return
			}
			runTUI()
		},
	}

	// Add programmatic flag to all commands
	rootCmd.PersistentFlags().BoolVar(&programmatic, "programmatic", false, "Run in programmatic mode (machine-readable output)")

	// Add logging flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set logging level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&logJSON, "log-json", false, "Output logs in JSON format")

	// Configure logging based on flags
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Set log level based on flag
		var level logging.LogLevel
		switch logLevel {
		case "debug":
			level = logging.LogLevelDebug
		case "info":
			level = logging.LogLevelInfo
		case "warn":
			level = logging.LogLevelWarn
		case "error":
			level = logging.LogLevelError
		default:
			level = logging.LogLevelInfo
		}

		// Configure logger
		logging.Initialize(&logging.Config{
			Level:      level,
			Output:     os.Stdout,
			JSONFormat: logJSON,
		})

		logging.Info("Starting useful1", "version", "1.0.0")
	}

	// Define subcommands
	respondCmd := &cobra.Command{
		Use:   "respond",
		Short: "Respond to GitHub issues",
		Run: func(cmd *cobra.Command, args []string) {
			if programmatic {
				// Run CLI executor in programmatic mode
				runCLIExecutor(cmd, tui.ScreenRespond)
				return
			}
			// Run TUI with the respond screen
			runTUIWithScreen(tui.ScreenRespond)
		},
	}
	// Add flags for respond command
	respondCmd.Flags().String("issue", "", "GitHub issue number")
	respondCmd.Flags().String("issue-file", "", "File containing issue text")
	respondCmd.Flags().String("owner", "", "GitHub repository owner")
	respondCmd.Flags().String("repo", "", "GitHub repository name")
	respondCmd.Flags().Int("number", 0, "GitHub issue number (when used with issue-file)")
	respondCmd.Flags().String("template", "", "Response template name")
	respondCmd.Flags().Float64("budget", 0, "Budget for this operation")

	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Create a pull request",
		Run: func(cmd *cobra.Command, args []string) {
			if programmatic {
				// Run CLI executor in programmatic mode
				runCLIExecutor(cmd, tui.ScreenPR)
				return
			}
			// Run TUI with the PR screen
			runTUIWithScreen(tui.ScreenPR)
		},
	}
	// Add flags for pr command
	prCmd.Flags().String("branch", "", "Branch name for the pull request")
	prCmd.Flags().String("base", "main", "Base branch for the pull request")
	prCmd.Flags().String("title", "", "Title for the pull request")
	prCmd.Flags().String("body", "", "Body content for the pull request")

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests",
		Run: func(cmd *cobra.Command, args []string) {
			if programmatic {
				// Run CLI executor in programmatic mode
				runCLIExecutor(cmd, tui.ScreenTest)
				return
			}
			// In TUI mode, just show a message that this feature is not available in TUI
			logging.Info("Test command is only available in programmatic mode")
			fmt.Println("The test command is only available in programmatic mode. Use --programmatic flag.")
			os.Exit(1)
		},
	}
	// Add flags for test command
	testCmd.Flags().String("suite", "", "Test suite to run")
	testCmd.Flags().Bool("verbose", false, "Enable verbose output")

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Generate or update configuration",
		Long:  `Interactive configuration setup that handles OAuth for GitHub and Anthropic and sets task budgets`,
		Run: func(cmd *cobra.Command, args []string) {
			if programmatic {
				// Run CLI executor in programmatic mode
				runCLIExecutor(cmd, tui.ScreenConfig)
				return
			}
			// Run TUI with the config screen
			runTUIWithScreen(tui.ScreenConfig)
		},
	}
	// Add flags for config command
	configCmd.Flags().String("github-token", "", "GitHub API token")
	configCmd.Flags().String("anthropic-token", "", "Anthropic API token")
	configCmd.Flags().Float64("issue-budget", 0, "Budget for issue responses")
	configCmd.Flags().Float64("pr-budget", 0, "Budget for PR creation")

	monitorCmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor GitHub issues for mentions",
		Long:  `Start monitoring GitHub issues for user mentions and respond automatically`,
		Run: func(cmd *cobra.Command, args []string) {
			if programmatic {
				// Run CLI executor in programmatic mode
				runCLIExecutor(cmd, tui.ScreenMonitor)
				return
			}
			// Run TUI with the monitor screen
			runTUIWithScreen(tui.ScreenMonitor)
		},
	}
	// Add flags for monitor command
	monitorCmd.Flags().String("repo", "", "Repository to monitor (owner/repo format)")
	monitorCmd.Flags().Int("interval", 300, "Polling interval in seconds")
	monitorCmd.Flags().Bool("auto-respond", false, "Automatically respond to issues")

	// Add commands for help/completion
	rootCmd.AddCommand(respondCmd, prCmd, testCmd, configCmd, monitorCmd)

	// Execute root command
	if err := rootCmd.Execute(); err != nil {
		logging.Error("Failed to execute command", "error", err)
		os.Exit(1)
	}
}

// runTUI launches the TUI application
func runTUI() {
	runTUIWithScreen(tui.ScreenMainMenu)
}

// runTUIWithScreen launches the TUI application with a specific initial screen
func runTUIWithScreen(screen tui.ScreenType) {
	var cfg *config.Config
	var err error

	// Try to load config, but continue with nil if it doesn't exist yet
	if config.Exists() {
		cfg, err = config.Load()
		if err != nil {
			logging.Warn("Error loading configuration", "error", err)
		}
	}

	// Launch TUI app with specified screen
	if err := tui.RunWithScreen(cfg, screen); err != nil {
		logging.Error("Failed to run TUI", "error", err)
		os.Exit(1)
	}
}

// runCLIExecutor runs the CLI executor in programmatic mode (JSON output, no TUI)
func runCLIExecutor(cmd *cobra.Command, screenType tui.ScreenType) {
	var cfg *config.Config
	var err error

	// Always need config for CLI executor
	if !config.Exists() {
		logging.Error("Configuration is required for programmatic mode",
			"hint", "run 'useful1 config' first to create a configuration")

		programmatic, flagErr := cmd.Flags().GetBool("programmatic")
		if flagErr != nil {
			logging.Warn("Failed to get programmatic flag", "error", flagErr)
		} else if programmatic {
			fmt.Fprintf(os.Stderr, "{\"status\": \"error\", \"message\": \"Configuration is required for programmatic mode\"}\n")
		}
		os.Exit(1)
	}

	// Load configuration
	cfg, err = config.Load()
	if err != nil {
		logging.Error("Failed to load configuration", "error", err)

		programmatic, flagErr := cmd.Flags().GetBool("programmatic")
		if flagErr != nil {
			logging.Warn("Failed to get programmatic flag", "error", flagErr)
		} else if programmatic {
			fmt.Fprintf(os.Stderr, "{\"status\": \"error\", \"message\": \"Error loading configuration: %s\"}\n", err)
		}
		os.Exit(1)
	}

	// Create CLI executor
	executor := cli.NewExecutor(cfg)

	// Get command flags and arguments
	flags := cmd.Flags()

	// Process command based on screen type
	var cmdErr error
	switch screenType {
	case tui.ScreenRespond:
		// Extract values needed for respond command
		issueNumber, err := flags.GetString("issue")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get issue parameter: %w", err)
			break
		}
		issueFile, err := flags.GetString("issue-file")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get issue-file parameter: %w", err)
			break
		}
		owner, err := flags.GetString("owner")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get owner parameter: %w", err)
			break
		}
		repo, err := flags.GetString("repo")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get repo parameter: %w", err)
			break
		}

		if issueFile != "" && owner != "" && repo != "" {
			// For issue text in a file
			issueNum, err := flags.GetInt("number")
			if err != nil {
				cmdErr = fmt.Errorf("failed to get number parameter: %w", err)
				break
			}

			// Read issue text from file
			issueText, err := os.ReadFile(issueFile)
			if err != nil {
				cmdErr = fmt.Errorf("failed to read issue file: %w", err)
				break
			}

			cmdErr = executor.RespondToIssueText(owner, repo, issueNum, string(issueText))
		} else if issueNumber != "" {
			// For direct issue number
			template, err := flags.GetString("template")
			if err != nil {
				cmdErr = fmt.Errorf("failed to get template parameter: %w", err)
				break
			}
			cmdErr = executor.RespondToIssue(issueNumber, template)
		} else {
			cmdErr = fmt.Errorf("missing required arguments: issue or issue-file with owner/repo/number")
		}

	case tui.ScreenPR:
		// Extract values needed for PR command
		branch, err := flags.GetString("branch")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get branch parameter: %w", err)
			break
		}
		base, err := flags.GetString("base")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get base parameter: %w", err)
			break
		}
		title, err := flags.GetString("title")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get title parameter: %w", err)
			break
		}

		if branch == "" {
			cmdErr = fmt.Errorf("missing required argument: branch")
			break
		}

		if base == "" {
			base = "main" // Default base branch
		}

		cmdErr = executor.CreatePullRequest(branch, base, title)

	case tui.ScreenTest:
		// Extract values needed for test command
		suite, err := flags.GetString("suite")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get suite parameter: %w", err)
			break
		}
		cmdErr = executor.RunTests(suite)

	case tui.ScreenConfig:
		// Config just shows success in programmatic mode since config is already loaded
		logging.Info("Configuration loaded successfully")
		programmaticVal, flagErr := cmd.Flags().GetBool("programmatic")
		if flagErr != nil {
			logging.Warn("Failed to get programmatic flag", "error", flagErr)
		} else if programmaticVal {
			fmt.Println("{\"status\": \"success\", \"message\": \"Configuration loaded successfully\"}")
		}
		return

	case tui.ScreenMonitor:
		// Start monitoring in programmatic mode
		logging.Info("Starting monitoring")
		programmaticVal, flagErr := cmd.Flags().GetBool("programmatic")
		if flagErr != nil {
			logging.Warn("Failed to get programmatic flag", "error", flagErr)
		} else if programmaticVal {
			fmt.Println("{\"status\": \"starting\", \"message\": \"Monitoring started\"}")
		}

		// Implement monitor functionality here
		// For now, just show a placeholder message
		logging.Error("Programmatic monitoring not implemented yet")
		// Reuse variables declared above
		programmaticVal, flagErr = cmd.Flags().GetBool("programmatic")
		if flagErr != nil {
			logging.Warn("Failed to get programmatic flag", "error", flagErr)
		} else if programmaticVal {
			fmt.Println("{\"status\": \"error\", \"message\": \"Programmatic monitoring not implemented yet\"}")
		}
		os.Exit(1)

	default:
		cmdErr = fmt.Errorf("unknown screen type: %v", screenType)
	}

	// Handle any errors
	if cmdErr != nil {
		logging.Error("Command execution failed", "error", cmdErr)

		programmaticVal, flagErr := cmd.Flags().GetBool("programmatic")
		if flagErr != nil {
			logging.Warn("Failed to get programmatic flag", "error", flagErr)
		} else if programmaticVal {
			fmt.Fprintf(os.Stderr, "{\"status\": \"error\", \"message\": \"%s\"}\n", cmdErr.Error())
		}
		os.Exit(1)
	}
}
