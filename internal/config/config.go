package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	GitHub struct {
		Token string
		User  string
		// No owner/repo fields - we'll work with all accessible repos
	}
	Anthropic struct {
		Token string
	}
	CLI struct {
		Command string
		Args    []string
	}
	Prompt struct {
		ConfirmationPatterns []PromptPattern
	}
	Budgets struct {
		IssueResponse float64
		PRCreation    float64
		TestRun       float64
		Default       float64
	}
	Monitor struct {
		PollInterval  int      // in minutes
		CheckMentions bool     // whether to check for mentions
		RepoFilter    []string // optional list of repositories to filter on (empty means all)
	}
}

// PromptPattern defines a pattern to match in CLI output and its response criteria
type PromptPattern struct {
	Pattern  string
	Response string
	Criteria []string
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".useful1", "config.yaml")
}

// Exists checks if configuration file exists
func Exists() bool {
	_, err := os.Stat(GetConfigPath())
	return err == nil
}

// Load loads configuration from files, environment variables, etc.
func Load() (*Config, error) {
	config := &Config{}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Look only in the user's home directory for the config
	viper.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".useful1"))

	// Set environment variable prefix
	viper.SetEnvPrefix("USEFUL1")
	viper.AutomaticEnv()

	// Map environment variables
	viper.BindEnv("github.token", "GITHUB_TOKEN")
	viper.BindEnv("anthropic.token", "ANTHROPIC_API_KEY")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("configuration not found. Please run 'useful1 config' first")
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Unmarshal the configuration into the Config struct
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate the configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// validateConfig checks if the required configuration is present
func validateConfig(config *Config) error {
	if config.GitHub.Token == "" {
		return fmt.Errorf("GitHub token is required")
	}

	if config.Anthropic.Token == "" {
		return fmt.Errorf("Anthropic token is required")
	}

	if config.CLI.Command == "" {
		return fmt.Errorf("CLI command is required")
	}

	return nil
}

// Configurator helps build and save configuration
type Configurator struct {
	config Config
}

// NewConfigurator creates a new configurator
func NewConfigurator() *Configurator {
	return &Configurator{
		config: Config{},
	}
}

// SetGitHubToken sets the GitHub token
func (c *Configurator) SetGitHubToken(token string) {
	c.config.GitHub.Token = token
}

// SetGitHubUser sets the GitHub user
func (c *Configurator) SetGitHubUser(user string) {
	c.config.GitHub.User = user
}

// SetAnthropicToken sets the Anthropic token
func (c *Configurator) SetAnthropicToken(token string) {
	c.config.Anthropic.Token = token
}

// SetTaskBudgets sets the task budgets
func (c *Configurator) SetTaskBudgets(budgets map[string]float64) {
	c.config.Budgets.IssueResponse = budgets["issue_response"]
	c.config.Budgets.PRCreation = budgets["pr_creation"]
	c.config.Budgets.TestRun = budgets["test_run"]
	c.config.Budgets.Default = budgets["default"]
}

// SetCLIToolPath sets the CLI tool path
func (c *Configurator) SetCLIToolPath(path string) {
	c.config.CLI.Command = path
}

// SetMonitoringSettings sets the issue monitoring settings
func (c *Configurator) SetMonitoringSettings(interval int, checkMentions bool, repoFilter []string) {
	c.config.Monitor.PollInterval = interval
	c.config.Monitor.CheckMentions = checkMentions
	c.config.Monitor.RepoFilter = repoFilter
}

// Save saves the configuration to the user's home directory
func (c *Configurator) Save() error {
	// Create the config directory if it doesn't exist
	configDir := filepath.Join(os.Getenv("HOME"), ".useful1")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create viper instance for saving
	v := viper.New()

	// Set the config values
	v.Set("github.token", c.config.GitHub.Token)
	v.Set("github.user", c.config.GitHub.User)
	v.Set("anthropic.token", c.config.Anthropic.Token)
	v.Set("budgets.issue_response", c.config.Budgets.IssueResponse)
	v.Set("budgets.pr_creation", c.config.Budgets.PRCreation)
	v.Set("budgets.test_run", c.config.Budgets.TestRun)
	v.Set("budgets.default", c.config.Budgets.Default)
	v.Set("cli.command", c.config.CLI.Command)
	v.Set("monitor.poll_interval", c.config.Monitor.PollInterval)
	v.Set("monitor.check_mentions", c.config.Monitor.CheckMentions)
	v.Set("monitor.repo_filter", c.config.Monitor.RepoFilter)

	// Set default confirmation patterns
	v.Set("prompt.confirmation_patterns", []map[string]interface{}{
		{
			"pattern":  "Are you sure you want to proceed?",
			"response": "y",
			"criteria": []string{"No test failures detected"},
		},
		{
			"pattern":  "Do you want to create a PR?",
			"response": "yes",
			"criteria": []string{"Changes have been reviewed"},
		},
	})

	// Write the config file
	v.SetConfigFile(filepath.Join(configDir, "config.yaml"))
	return v.WriteConfig()
}

// PromptForCLIToolPath prompts the user for the CLI tool path
func PromptForCLIToolPath() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter the path to the CLI tool: ")
	path, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	// Trim whitespace
	path = strings.TrimSpace(path)

	// Validate the path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("the specified path doesn't exist: %s", path)
	}

	return path, nil
}
