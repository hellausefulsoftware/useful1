package config

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		Timeout int // in seconds
	}
	Budgets struct {
		IssueResponse float64
		PRCreation    float64
		TestRun       float64
		Default       float64
	}
	Monitor struct {
		PollInterval       int      // in minutes
		RepoFilter         []string // optional list of repositories to filter on (empty means all)
		AssignedIssuesOnly bool     // whether to only show issues assigned to the user
		AutoRespond        bool     // whether to automatically respond to issues
	}
	Logging struct {
		Output     io.Writer
		Level      string
		JSONFormat bool
	}
	VCS struct {
		Platform string // "github", "gitlab", "gogs", etc.
	}
}

// LoadConfig loads the configuration from standard locations
func LoadConfig() (*Config, error) {
	// Create a default config
	cfg := &Config{}

	// Set default values
	cfg.Monitor.PollInterval = 1 // 1 minute
	cfg.Budgets.Default = 5.0    // Default budget of $5
	cfg.Logging.Level = "info"   // Default log level
	cfg.VCS.Platform = "github"  // Default VCS platform
	cfg.CLI.Timeout = 120        // Default timeout of 120 seconds

	// Get config file path using GetConfigPath
	configFile := GetConfigPath()

	// If config file exists, load it
	if _, err := os.Stat(configFile); err == nil {
		file, err := os.Open(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %w", err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Printf("WARNING: Failed to close config file: %v\n", closeErr)
			}
		}()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
	}

	// Environment variables override config file
	if token := os.Getenv("USEFUL1_GITHUB_TOKEN"); token != "" {
		cfg.GitHub.Token = token
	}
	if user := os.Getenv("USEFUL1_GITHUB_USER"); user != "" {
		cfg.GitHub.User = user
	}
	if token := os.Getenv("USEFUL1_ANTHROPIC_TOKEN"); token != "" {
		cfg.Anthropic.Token = token
	}

	return cfg, nil
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".useful1", "config.json")
}

// Exists checks if configuration file exists
func Exists() bool {
	_, err := os.Stat(GetConfigPath())
	return err == nil
}

// encodeCredentials encodes sensitive credentials using base64
func encodeCredentials(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

// decodeCredentials decodes base64 encoded credentials
func decodeCredentials(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("failed to decode credential: %w", err)
	}
	return string(decoded), nil
}

// Load loads configuration from files, environment variables, etc.
func Load() (*Config, error) {
	config := &Config{}
	configPath := GetConfigPath()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration not found. Please run 'useful1 config' first")
	}

	// Read the file directly
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Unmarshal JSON
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Set longer timeout if log level is debug
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" || strings.ToLower(config.Logging.Level) == "debug" {
		// Increase timeout to 5 minutes (300 seconds) in debug mode
		config.CLI.Timeout = 300
	}

	// Decode credentials
	if config.GitHub.Token != "" {
		decodedToken, err := decodeCredentials(config.GitHub.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to decode GitHub token: %w", err)
		}
		config.GitHub.Token = decodedToken
	}

	if config.Anthropic.Token != "" {
		decodedToken, err := decodeCredentials(config.Anthropic.Token)
		if err != nil {
			return nil, fmt.Errorf("failed to decode Anthropic token: %w", err)
		}
		config.Anthropic.Token = decodedToken
	}

	// Check environment variables as override
	if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		config.GitHub.Token = envToken
	}

	if envToken := os.Getenv("ANTHROPIC_API_KEY"); envToken != "" {
		config.Anthropic.Token = envToken
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
		return fmt.Errorf("github token is required")
	}

	if config.Anthropic.Token == "" {
		return fmt.Errorf("anthropic token is required")
	}

	if config.CLI.Command == "" {
		return fmt.Errorf("cli command is required")
	}

	return nil
}

// SaveToFile saves the configuration to the specified file path
func (c *Config) SaveToFile(filePath string) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	configJSON, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, configJSON, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
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
	// Use default if empty
	if path == "" {
		path = "claude --dangerously-skip-permissions"
	}
	c.config.CLI.Command = path

	// Clear any previously set args, as they will now be incorporated in the command string
	c.config.CLI.Args = []string{}
}

// SetMonitoringSettings sets the issue monitoring settings
func (c *Configurator) SetMonitoringSettings(interval int, repoFilter []string, assignedOnly bool) {
	c.config.Monitor.PollInterval = interval
	c.config.Monitor.RepoFilter = repoFilter
	// Always set to true, we only want to monitor assigned issues
	c.config.Monitor.AssignedIssuesOnly = true
}

// Save saves the configuration to the user's home directory
func (c *Configurator) Save() error {
	// Create the config directory if it doesn't exist
	configDir := filepath.Join(os.Getenv("HOME"), ".useful1")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure CLI command has the default value if empty
	if c.config.CLI.Command == "" {
		c.config.CLI.Command = "claude --dangerously-skip-permissions"
	}

	// Make a copy of the config with encoded credentials
	configToSave := c.config

	// Base64 encode sensitive credentials
	if configToSave.GitHub.Token != "" {
		configToSave.GitHub.Token = encodeCredentials(configToSave.GitHub.Token)
	}
	if configToSave.Anthropic.Token != "" {
		configToSave.Anthropic.Token = encodeCredentials(configToSave.Anthropic.Token)
	}

	// Marshal to JSON
	configJSON, err := json.MarshalIndent(configToSave, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	// Write directly to file
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configJSON, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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
