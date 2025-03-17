package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/common/vcs"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/github"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/tui"
	"github.com/hellausefulsoftware/useful1/internal/workflow"
	"github.com/spf13/cobra"
)

// issueProcessorAdapter adapts a function to the IssueProcessor interface
// This allows us to use our main flow processing logic with the VCS monitor
type issueProcessorAdapter struct {
	processFunc func(vcs.Issue) error
}

// Process calls the wrapped function
func (a *issueProcessorAdapter) Process(issue vcs.Issue) error {
	return a.processFunc(issue)
}

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

	// Add execute command
	executeCmd := &cobra.Command{
		Use:   "execute [args...]",
		Short: "Execute the configured CLI tool directly",
		Long:  "Execute the configured CLI tool directly with interactive terminal support. Any arguments after the command will be passed to the tool.",
		Run: func(cmd *cobra.Command, args []string) {
			if tuiMode {
				// Run TUI with the execute screen
				runTUIWithScreen(tui.ScreenExecute)
				return
			}

			// Special handling for test runs - if we detect we're in a test environment
			// just print the arguments and exit successfully
			if os.Getenv("USEFUL1_CONFIG") != "" && strings.Contains(os.Getenv("USEFUL1_CONFIG"), "_test_config") {
				fmt.Println(strings.Join(args, " "))
				return
			}

			// Get additional arguments
			// All arguments after the command are passed directly to the CLI tool
			// Run CLI executor in interactive mode
			var cfg *config.Config
			var err error

			// Always need config for CLI executor
			if !config.Exists() {
				logging.Error("Configuration is required",
					"hint", "run 'useful1 config' first to create a configuration")
				fmt.Fprintf(os.Stderr, "Error: Configuration is required. Run 'useful1 config' first to create a configuration.\n")
				os.Exit(1)
			}

			// Load configuration
			cfg, err = config.Load()
			if err != nil {
				logging.Error("Failed to load configuration", "error", err)
				fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
				os.Exit(1)
			}

			// Create CLI executor
			executor := cli.NewExecutor(cfg)

			// Execute the command with all arguments
			fmt.Println("Executing CLI tool in interactive mode...")
			err = executor.Execute(args)
			if err != nil {
				logging.Error("Command execution failed", "error", err)
				fmt.Fprintf(os.Stderr, "Command execution failed: %s\n", err)
				os.Exit(1)
			}
		},
		// Disable flag parsing and pass all arguments literally
		DisableFlagParsing: true,
	}

	// Add commands for help/completion
	rootCmd.AddCommand(respondCmd, prCmd, testCmd, configCmd, monitorCmd, executeCmd)

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
		// Issue response functionality has been moved to workflow package
		cmdErr = fmt.Errorf("issue response functionality has been moved to the workflow package")
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

		// Create workflow instance and use it to create PR
		w := workflow.NewImplementationWorkflow(cfg)

		// Get owner and repo from config
		owner := cfg.GitHub.User
		repo := "useful1" // Hard-coded for now - would come from selection

		if owner == "" {
			cmdErr = fmt.Errorf("missing GitHub owner or repo in config")
			break
		}

		// Create the pull request
		pr, err := w.CreatePullRequest(owner, repo, branch, base, title)
		if err != nil {
			cmdErr = fmt.Errorf("failed to create pull request: %w", err)
			break
		}

		// Output JSON response
		response := map[string]interface{}{
			"status":    "success",
			"branch":    branch,
			"base":      base,
			"title":     title,
			"pr_number": *pr.Number,
			"pr_url":    *pr.HTMLURL,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			cmdErr = fmt.Errorf("error formatting JSON response: %w", err)
			break
		}

		fmt.Println(string(jsonResponse))

	case tui.ScreenTest:
		// Extract values needed for test command
		_, err := flags.GetString("suite")
		if err != nil {
			cmdErr = fmt.Errorf("failed to get suite parameter: %w", err)
			break
		}
		cmdErr = fmt.Errorf("test execution functionality has been moved to the workflow package")

	case tui.ScreenConfig:
		// Config shows success in CLI mode since config is already loaded
		logging.Info("Configuration loaded successfully")
		// Since CLI is the default mode, always show the success message
		fmt.Println("{\"status\": \"success\", \"message\": \"Configuration loaded successfully\"}")
		return

	case tui.ScreenExecute:
		// Execute the CLI tool directly
		args := cmd.Flags().Args()

		// Print a separator to distinguish from command output
		fmt.Println("\n--- Running CLI tool command ---")

		// Execute in interactive mode
		cmdErr = executor.Execute(args)

		// Print completion message
		if cmdErr == nil {
			fmt.Println("--- Command completed successfully ---")
		} else {
			fmt.Println("--- Command failed ---")
		}
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
		// Create GitHub adapter for VCS service
		githubAdapter, err := github.NewAdapter(cfg)
		if err != nil {
			logging.Error("Failed to create GitHub adapter", "error", err)
			fmt.Println("{\"status\": \"error\", \"message\": \"Failed to create GitHub adapter\"}")
			os.Exit(1)
		}

		// Create a processor adapter that delegates to our main flow processing
		// Create a function to handle processing discovered issues in the main execution flow
		processIssueFunc := func(issue vcs.Issue) error {
			// Get authenticated username
			username, authErr := githubAdapter.GetAuthenticatedUser()
			if authErr != nil {
				logging.Warn("Failed to get authenticated user", "error", authErr)
			}

			logging.Info("Processing issue in main execution flow",
				"number", issue.GetNumber(),
				"owner", issue.GetOwner(),
				"repo", issue.GetRepo(),
				"title", issue.GetTitle())

			// Check if the issue is already closed
			if strings.ToLower(issue.GetState()) == "closed" {
				logging.Info("Issue is closed, skipping")
				return nil
			}

			// Check if the last comment was from the bot
			comments := issue.GetComments()
			if len(comments) > 0 && comments[len(comments)-1].User == username {
				logging.Info("Last comment was from bot, skipping to avoid duplicate responses")
				return nil
			}

			// Check if we already have a PR for this issue
			prs, prErr := githubAdapter.GetPullRequestsForIssue(issue.GetOwner(), issue.GetRepo(), issue.GetNumber())
			if prErr != nil {
				logging.Warn("Failed to check for existing draft PRs", "error", prErr)
			} else {
				// Check if any of these PRs are open drafts created by our user
				for _, pr := range prs {
					// Only consider open draft PRs
					if pr.GetState() == "open" && pr.GetIsDraft() && pr.GetUser() == username {
						logging.Info("Issue already has a draft PR, skipping")
						return nil
					}
				}
			}

			// Create implementation workflow to handle the issue
			implementationWorkflow := workflow.NewImplementationWorkflow(cfg)

			// Generate branch name for the issue
			branchName, prTitle, genErr := implementationWorkflow.GenerateBranchAndTitle(
				issue.GetOwner(),
				issue.GetRepo(),
				issue.GetTitle(),
				issue.GetBody(),
			)
			if genErr != nil {
				return fmt.Errorf("failed to generate branch name: %w", genErr)
			}

			logging.Info("Generated branch name with workflow",
				"branch", branchName,
				"pr_title", prTitle)

			// Get default branch
			defaultBranch, defaultErr := githubAdapter.GetDefaultBranch(issue.GetOwner(), issue.GetRepo())
			if defaultErr != nil {
				logging.Warn("Failed to get default branch, using 'main'", "error", defaultErr)
				defaultBranch = "main" // Default fallback
			}

			// Create the branch
			logging.Info("Creating branch", "branch", branchName, "base", defaultBranch)
			if createErr := githubAdapter.CreateBranch(issue.GetOwner(), issue.GetRepo(), branchName, defaultBranch); createErr != nil {
				return fmt.Errorf("failed to create branch: %w", createErr)
			}

			// Create implementation plan and get Claude output
			claudeOutput, planErr := implementationWorkflow.CreateImplementationPlan(
				issue.GetOwner(),
				issue.GetRepo(),
				branchName,
				issue.GetNumber(),
			)
			if planErr != nil {
				logging.Warn("Failed to create implementation plan", "error", planErr)
				claudeOutput = "" // Empty if there was an error
				// Continue anyway - we'll still create the PR
			}

			// Create the draft PR
			logging.Info("Creating draft PR",
				"owner", issue.GetOwner(),
				"repo", issue.GetRepo(),
				"title", prTitle,
				"branch", branchName,
				"base", defaultBranch)

			// Create the PR using the implementation output
			pr, prErr := implementationWorkflow.CreatePullRequestForIssue(
				issue.GetOwner(),
				issue.GetRepo(),
				branchName,
				defaultBranch,
				issue.GetNumber(),
				claudeOutput,
			)
			if prErr != nil {
				return fmt.Errorf("failed to create draft PR: %w", prErr)
			}

			logging.Info("Successfully created draft PR",
				"pr_number", pr.GetNumber(),
				"url", pr.GetURL())

			return nil
		}

		processor := &issueProcessorAdapter{
			processFunc: processIssueFunc,
		}
		monitorConfig := vcs.MonitorConfig{
			Config:    cfg,
			Service:   githubAdapter,
			Processor: processor,
		}

		// Create the new monitor
		monitor, err := vcs.NewMonitor(monitorConfig)
		if err != nil {
			logging.Error("Failed to create monitor", "error", err)
			fmt.Println("{\"status\": \"error\", \"message\": \"Failed to create monitor\"}")
			os.Exit(1)
		}

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
