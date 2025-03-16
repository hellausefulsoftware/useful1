package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/github"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	// Initialize logger with default configuration
	logging.Initialize(nil)

	// Define program-wide flags
	var tuiMode bool
	var logLevel string
	var logJSON bool

	rootCmd := &cobra.Command{
		Use:   "useful1",
		Short: "Automates GitHub tasks via CLI tool integration",
		Long:  `A CLI application with an optional colorblind-friendly TUI that automates GitHub operations like issue responses, PR creation, and test execution.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Run CLI mode by default, use TUI if --tui flag is present
			if tuiMode {
				runTUI()
				return
			}
			fmt.Println("Running in CLI mode. Use a subcommand or --help for available commands.")
			fmt.Println("Use --tui flag to launch in terminal user interface mode.")
		},
	}

	// Add TUI flag to all commands
	rootCmd.PersistentFlags().BoolVar(&tuiMode, "tui", false, "Run in Terminal User Interface mode")

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

		// Set output to stderr when in TUI mode to avoid breaking the interface
		output := os.Stdout
		if tuiMode {
			output = os.Stderr
		}

		// Configure logger
		logging.Initialize(&logging.Config{
			Level:      level,
			Output:     output,
			JSONFormat: logJSON,
		})

		logging.Info("Starting useful1", "version", "1.0.0")
	}

	// Define subcommands
	respondCmd := &cobra.Command{
		Use:   "respond",
		Short: "Respond to GitHub issues (use --tui for interactive mode)",
		Run: func(cmd *cobra.Command, args []string) {
			if tuiMode {
				// Run TUI with the respond screen
				runTUIWithScreen(tui.ScreenRespond)
				return
			}
			// Run CLI executor by default
			runCLIExecutor(cmd, tui.ScreenRespond)
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
		Short: "Create a pull request (use --tui for interactive mode)",
		Run: func(cmd *cobra.Command, args []string) {
			if tuiMode {
				// Run TUI with the PR screen
				runTUIWithScreen(tui.ScreenPR)
				return
			}
			// Run CLI executor by default
			runCLIExecutor(cmd, tui.ScreenPR)
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
			if tuiMode {
				// In TUI mode, just show a message that this feature is not available in TUI
				logging.Info("Test command is not available in TUI mode")
				fmt.Println("The test command is not available in TUI mode. Use CLI mode instead.")
				os.Exit(1)
				return
			}
			// Run CLI executor by default
			runCLIExecutor(cmd, tui.ScreenTest)
		},
	}
	// Add flags for test command
	testCmd.Flags().String("suite", "", "Test suite to run")
	testCmd.Flags().Bool("verbose", false, "Enable verbose output")

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Generate or update configuration (use --tui for interactive mode)",
		Long:  `Interactive configuration setup that handles OAuth for GitHub and Anthropic and sets task budgets. Add --tui flag for a more user-friendly interface.`,
		Run: func(cmd *cobra.Command, args []string) {
			if tuiMode {
				// Run TUI with the config screen
				runTUIWithScreen(tui.ScreenConfig)
				return
			}
			// Run CLI executor by default
			runCLIExecutor(cmd, tui.ScreenConfig)
		},
	}
	// Add flags for config command
	configCmd.Flags().String("github-token", "", "GitHub API token")
	configCmd.Flags().String("anthropic-token", "", "Anthropic API token")
	configCmd.Flags().Float64("issue-budget", 0, "Budget for issue responses")
	configCmd.Flags().Float64("pr-budget", 0, "Budget for PR creation")

	monitorCmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor GitHub issues assigned to user (use --tui for interactive mode)",
		Long:  `Start monitoring GitHub issues assigned to the user and respond automatically. Add --tui flag for interactive Terminal UI mode.`,
		Run: func(cmd *cobra.Command, args []string) {
			if tuiMode {
				// Run TUI with the monitor screen
				runTUIWithScreen(tui.ScreenMonitor)
				return
			}
			// Run CLI executor by default
			runCLIExecutor(cmd, tui.ScreenMonitor)
		},
	}
	// Add flags for monitor command
	monitorCmd.Flags().String("repo", "", "Repository to monitor (owner/repo format)")
	monitorCmd.Flags().Int("interval", 60, "Polling interval in seconds")
	monitorCmd.Flags().Bool("auto-respond", false, "Automatically respond to issues")
	monitorCmd.Flags().Bool("once", false, "Run a one-time check instead of continuous monitoring")

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

	// Ensure logs are sent to stderr in TUI mode to prevent interference with the UI
	logConfig := &logging.Config{
		Level:      logging.LogLevelInfo, // Use default level
		Output:     os.Stderr,            // Redirect to stderr
		JSONFormat: false,                // Use text format
	}
	logging.Initialize(logConfig)

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

// runCLIExecutor runs the CLI executor in standard CLI mode (JSON output, no TUI)
func runCLIExecutor(cmd *cobra.Command, screenType tui.ScreenType) {
	var cfg *config.Config
	var err error

	// Always need config for CLI executor
	if !config.Exists() {
		logging.Error("Configuration is required",
			"hint", "run 'useful1 config' first to create a configuration")

		// Since CLI is the default mode, always show the error
		fmt.Fprintf(os.Stderr, "{\"status\": \"error\", \"message\": \"Configuration is required. Run 'useful1 config' first to create a configuration.\"}\n")
		os.Exit(1)
	}

	// Load configuration
	cfg, err = config.Load()
	if err != nil {
		logging.Error("Failed to load configuration", "error", err)

		// Since CLI is the default mode, always show the error
		fmt.Fprintf(os.Stderr, "{\"status\": \"error\", \"message\": \"Error loading configuration: %s\"}\n", err)
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
		// Config shows success in CLI mode since config is already loaded
		logging.Info("Configuration loaded successfully")
		// Since CLI is the default mode, always show the success message
		fmt.Println("{\"status\": \"success\", \"message\": \"Configuration loaded successfully\"}")
		return

	case tui.ScreenMonitor:
		// Start monitoring in CLI mode
		logging.Info("Starting monitoring")
		// Since CLI is the default mode, always show the start message
		fmt.Println("{\"status\": \"starting\", \"message\": \"Monitoring started\"}")

		// Get monitor parameters from flags
		repo, err := flags.GetString("repo")
		if err != nil {
			logging.Warn("Failed to get repo flag", "error", err)
		}

		interval, err := flags.GetInt("interval")
		if err != nil {
			logging.Warn("Failed to get interval flag", "error", err)
			interval = 60 // Default to 60 seconds
		}

		// Get auto-respond flag, but we don't use it yet
		// It's here for future implementation
		_, err = flags.GetBool("auto-respond")
		if err != nil {
			logging.Warn("Failed to get auto-respond flag", "error", err)
		}

		// Update config with flags if provided
		if repo != "" {
			// Parse owner/repo format
			parts := strings.Split(repo, "/")
			if len(parts) != 2 {
				logging.Error("Invalid repo format, expected 'owner/repo'", "repo", repo)
				fmt.Println("{\"status\": \"error\", \"message\": \"Invalid repo format, expected 'owner/repo'\"}")
				os.Exit(1)
			}

			// Set repo filter in config
			cfg.Monitor.RepoFilter = []string{repo}
			logging.Info("Set repository filter", "repo", repo)
		}

		// Set poll interval in config (convert seconds to minutes)
		cfg.Monitor.PollInterval = interval / 60
		if interval%60 != 0 {
			logging.Warn("Interval not divisible by 60, rounding down", "seconds", interval, "minutes", interval/60)
		}
		logging.Info("Set poll interval", "seconds", interval, "minutes", cfg.Monitor.PollInterval)

		// Set if we're monitoring assigned issues only (always true for CLI mode)
		cfg.Monitor.AssignedIssuesOnly = true
		logging.Info("Monitoring assigned issues only")

		// Create monitor (no need to separately create GitHub client)
		monitor := github.NewMonitor(cfg, executor)

		// Check if username was found
		if monitor.GetUsername() == "" {
			logging.Error("Failed to determine GitHub username, check your token permissions")
			fmt.Println("{\"status\": \"error\", \"message\": \"Failed to determine GitHub username, check your token permissions\"}")
			os.Exit(1)
		}

		// Log what repositories we're monitoring
		var repoInfo string
		if len(cfg.Monitor.RepoFilter) > 0 {
			repoInfo = fmt.Sprintf("Monitoring repositories: %s", strings.Join(cfg.Monitor.RepoFilter, ", "))
		} else {
			repoInfo = "Monitoring all accessible repositories"
		}
		logging.Info(repoInfo)

		// Run once flag processing
		once, err := flags.GetBool("once")
		if err != nil {
			logging.Warn("Failed to get once flag", "error", err)
			once = false // Default to continuous monitoring
		}

		if once {
			// Run one-time check
			logging.Info("Running one-time check")
			err := monitor.CheckOnce()
			if err != nil {
				logging.Error("Check failed", "error", err)
				fmt.Printf("{\"status\": \"error\", \"message\": \"Check failed: %s\"}\n", err.Error())
				os.Exit(1)
			}

			// Get stats and output as JSON
			stats := monitor.GetStats()
			statsJSON, err := json.Marshal(stats)
			if err != nil {
				logging.Error("Failed to marshal stats", "error", err)
			} else {
				logging.Info("Monitoring stats", "stats", string(statsJSON))
			}

			fmt.Println("{\"status\": \"success\", \"message\": \"Check completed successfully\", \"stats\": " + string(statsJSON) + "}")
		} else {
			// Run continuous monitoring
			logging.Info("Starting continuous monitoring")
			fmt.Println("{\"status\": \"running\", \"message\": \"Continuous monitoring started\"}")

			// Start monitoring in a goroutine so we can capture signals
			monitorDone := make(chan struct{})
			go func() {
				err := monitor.Start()
				if err != nil {
					logging.Error("Monitoring failed", "error", err)
				}
				close(monitorDone)
			}()

			// Wait for interrupt signal
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			select {
			case <-sigChan:
				logging.Info("Received interrupt, shutting down")
				fmt.Println("{\"status\": \"stopping\", \"message\": \"Monitoring stopped by user\"}")
			case <-monitorDone:
				logging.Info("Monitoring completed")
				fmt.Println("{\"status\": \"completed\", \"message\": \"Monitoring completed\"}")
			}
		}

		os.Exit(0)

	default:
		cmdErr = fmt.Errorf("unknown screen type: %v", screenType)
	}

	// Handle any errors
	if cmdErr != nil {
		logging.Error("Command execution failed", "error", cmdErr)

		// Since CLI is the default mode, always show the error in JSON format
		fmt.Fprintf(os.Stderr, "{\"status\": \"error\", \"message\": \"%s\"}\n", cmdErr.Error())
		os.Exit(1)
	}
}
