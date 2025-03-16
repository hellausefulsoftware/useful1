package main

import (
	"fmt"
	"os"

	"github.com/hellausefulsoftware/useful1/internal/auth"
	"github.com/hellausefulsoftware/useful1/internal/budget"
	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/github"
	"github.com/hellausefulsoftware/useful1/internal/monitoring"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "useful1",
		Short: "Automates GitHub tasks via CLI tool integration",
		Long:  `A CLI application that wraps another CLI tool to automate GitHub operations like issue responses, PR creation, and test execution.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip config check for the config command itself
			if cmd.Name() == "config" {
				return nil
			}

			// Check if config exists
			if !config.Exists() {
				return fmt.Errorf("configuration not found. Please run 'useful1 config' first")
			}

			return nil
		},
	}

	// Load config for commands other than 'config'
	var cfg *config.Config
	var err error

	if len(os.Args) > 1 && os.Args[1] != "config" {
		cfg, err = config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
			os.Exit(1)
		}
	}

	// Respond to issues command
	respondCmd := &cobra.Command{
		Use:   "respond [issue-number]",
		Short: "Respond to GitHub issues",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueNumber := args[0]
			templateName, _ := cmd.Flags().GetString("template")

			executor := cli.NewExecutor(cfg)
			return executor.RespondToIssue(issueNumber, templateName)
		},
	}
	respondCmd.Flags().StringP("template", "t", "default", "Response template to use")

	// Create PR command
	prCmd := &cobra.Command{
		Use:   "pr [branch] [title]",
		Short: "Create a pull request",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			title := ""
			if len(args) > 1 {
				title = args[1]
			}

			base, _ := cmd.Flags().GetString("base")
			executor := cli.NewExecutor(cfg)
			return executor.CreatePullRequest(branch, base, title)
		},
	}
	prCmd.Flags().StringP("base", "b", "main", "Base branch for the pull request")

	// Run tests command
	testCmd := &cobra.Command{
		Use:   "test [test-suite]",
		Short: "Run tests",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			testSuite := ""
			if len(args) > 0 {
				testSuite = args[0]
			}

			executor := cli.NewExecutor(cfg)
			return executor.RunTests(testSuite)
		},
	}

	// Configuration command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Generate or update configuration",
		Long:  `Interactive configuration setup that handles OAuth for GitHub and Anthropic and sets task budgets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Starting configuration setup...")

			// Create new configuration handler
			configurator := config.NewConfigurator()

			// Step 1: GitHub OAuth
			fmt.Println("\n=== GitHub Authentication ===")
			githubAuth, err := auth.SetupGitHubOAuth()
			if err != nil {
				return fmt.Errorf("GitHub OAuth setup failed: %w", err)
			}
			configurator.SetGitHubToken(githubAuth.Token)
			configurator.SetGitHubUser(githubAuth.User)

			// Step 2: Anthropic OAuth
			fmt.Println("\n=== Anthropic Authentication ===")
			anthropicAuth, err := auth.SetupAnthropicOAuth()
			if err != nil {
				return fmt.Errorf("Anthropic OAuth setup failed: %w", err)
			}
			configurator.SetAnthropicToken(anthropicAuth.Token)

			// Step 3: Budget configuration
			fmt.Println("\n=== Task Budget Configuration ===")
			budgets, err := budget.SetupTaskBudgets()
			if err != nil {
				return fmt.Errorf("budget configuration failed: %w", err)
			}
			configurator.SetTaskBudgets(budgets)

			// Step 4: Monitoring settings
			fmt.Println("\n=== Issue Monitoring Configuration ===")
			monitorSettings, err := monitoring.SetupMonitoring()
			if err != nil {
				return fmt.Errorf("monitoring configuration failed: %w", err)
			}
			configurator.SetMonitoringSettings(
				monitorSettings.PollInterval,
				monitorSettings.CheckMentions,
				monitorSettings.RepoFilter,
			)

			// Step 5: CLI tool configuration
			fmt.Println("\n=== CLI Tool Configuration ===")
			toolPath, err := config.PromptForCLIToolPath()
			if err != nil {
				return fmt.Errorf("CLI tool configuration failed: %w", err)
			}
			configurator.SetCLIToolPath(toolPath)

			// Save configuration to user's home directory
			if err := configurator.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Println("\nConfiguration successfully saved to ~/.useful1/config.yaml")
			return nil
		},
	}

	// Monitor command to start issue monitoring
	monitorCmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor GitHub issues for mentions",
		Long:  `Start monitoring GitHub issues for user mentions and respond automatically`,
		RunE: func(cmd *cobra.Command, args []string) error {
			once, _ := cmd.Flags().GetBool("once")
			executor := cli.NewExecutor(cfg)
			monitor := github.NewMonitor(cfg, executor)

			if once {
				return monitor.CheckOnce()
			}
			return monitor.Start()
		},
	}

	// Add flags to monitor command
	monitorCmd.Flags().BoolP("once", "o", false, "Run once and exit rather than continuous monitoring")

	rootCmd.AddCommand(respondCmd, prCmd, testCmd, configCmd, monitorCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
