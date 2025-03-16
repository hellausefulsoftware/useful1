package main

import (
	"fmt"
	"os"

	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "useful1",
		Short: "Automates GitHub tasks via CLI tool integration",
		Long:  `A CLI application with a colorblind-friendly TUI that automates GitHub operations like issue responses, PR creation, and test execution.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Run the TUI app as default
			runTUI()
		},
	}

	// Parse any command (only for help/completion purposes)
	respondCmd := &cobra.Command{
		Use:   "respond",
		Short: "Respond to GitHub issues",
		Run: func(cmd *cobra.Command, args []string) {
			// Run TUI with the respond screen
			runTUIWithScreen(tui.ScreenRespond)
		},
	}

	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Create a pull request",
		Run: func(cmd *cobra.Command, args []string) {
			// Run TUI with the PR screen
			runTUIWithScreen(tui.ScreenPR)
		},
	}

	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests",
		Run: func(cmd *cobra.Command, args []string) {
			// Run TUI with the test screen
			runTUIWithScreen(tui.ScreenTest)
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Generate or update configuration",
		Long:  `Interactive configuration setup that handles OAuth for GitHub and Anthropic and sets task budgets`,
		Run: func(cmd *cobra.Command, args []string) {
			// Run TUI with the config screen
			runTUIWithScreen(tui.ScreenConfig)
		},
	}

	monitorCmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor GitHub issues for mentions",
		Long:  `Start monitoring GitHub issues for user mentions and respond automatically`,
		Run: func(cmd *cobra.Command, args []string) {
			// Run TUI with the monitor screen
			runTUIWithScreen(tui.ScreenMonitor)
		},
	}

	// Add commands for help/completion
	rootCmd.AddCommand(respondCmd, prCmd, testCmd, configCmd, monitorCmd)

	// Execute root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "Warning: Error loading configuration: %v\n", err)
		}
	}

	// Launch TUI app with specified screen
	if err := tui.RunWithScreen(cfg, screen); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
