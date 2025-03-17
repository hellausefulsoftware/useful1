package config

import (
	"testing"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				GitHub: struct {
					Token string
					User  string
				}{
					Token: "github-token",
					User:  "user",
				},
				Anthropic: struct {
					Token string
				}{
					Token: "anthropic-token",
				},
				CLI: struct {
					Command string
					Args    []string
					Timeout int
				}{
					Command: "cli-command",
					Args:    []string{"arg1", "arg2"},
					Timeout: 120,
				},
			},
			expectErr: false,
		},
		{
			name: "missing github token",
			config: &Config{
				GitHub: struct {
					Token string
					User  string
				}{
					Token: "",
					User:  "user",
				},
				Anthropic: struct {
					Token string
				}{
					Token: "anthropic-token",
				},
				CLI: struct {
					Command string
					Args    []string
					Timeout int
				}{
					Command: "cli-command",
					Args:    []string{"arg1", "arg2"},
					Timeout: 120,
				},
			},
			expectErr: true,
		},
		{
			name: "missing anthropic token",
			config: &Config{
				GitHub: struct {
					Token string
					User  string
				}{
					Token: "github-token",
					User:  "user",
				},
				Anthropic: struct {
					Token string
				}{
					Token: "",
				},
				CLI: struct {
					Command string
					Args    []string
					Timeout int
				}{
					Command: "cli-command",
					Args:    []string{"arg1", "arg2"},
					Timeout: 120,
				},
			},
			expectErr: true,
		},
		{
			name: "missing cli command",
			config: &Config{
				GitHub: struct {
					Token string
					User  string
				}{
					Token: "github-token",
					User:  "user",
				},
				Anthropic: struct {
					Token string
				}{
					Token: "anthropic-token",
				},
				CLI: struct {
					Command string
					Args    []string
					Timeout int
				}{
					Command: "",
					Args:    []string{"arg1", "arg2"},
					Timeout: 120,
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateConfig() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestConfigurator(t *testing.T) {
	// For this test, we'll directly test configurator functions
	// without relying on Viper's config loading which has different key mapping

	configurator := NewConfigurator()

	// Set values
	configurator.SetGitHubToken("test-github-token")
	configurator.SetGitHubUser("test-user")
	configurator.SetAnthropicToken("test-anthropic-token")
	configurator.SetCLIToolPath("/bin/echo")

	budgets := map[string]float64{
		"issue_response": 0.5,
		"pr_creation":    1.0,
		"test_run":       0.3,
		"default":        0.2,
	}
	configurator.SetTaskBudgets(budgets)
	configurator.SetMonitoringSettings(5, []string{"repo1", "repo2"}, true)

	// Verify values are set correctly in the configurator
	if configurator.config.GitHub.Token != "test-github-token" {
		t.Errorf("GitHub token not set correctly, got %s, want %s",
			configurator.config.GitHub.Token, "test-github-token")
	}

	if configurator.config.GitHub.User != "test-user" {
		t.Errorf("GitHub user not set correctly, got %s, want %s",
			configurator.config.GitHub.User, "test-user")
	}

	if configurator.config.Anthropic.Token != "test-anthropic-token" {
		t.Errorf("Anthropic token not set correctly, got %s, want %s",
			configurator.config.Anthropic.Token, "test-anthropic-token")
	}

	if configurator.config.CLI.Command != "/bin/echo" {
		t.Errorf("CLI command not set correctly, got %s, want %s",
			configurator.config.CLI.Command, "/bin/echo")
	}

	// Test that monitoring settings were set
	if configurator.config.Monitor.PollInterval != 5 {
		t.Errorf("Poll interval not set correctly, got %d, want %d",
			configurator.config.Monitor.PollInterval, 5)
	}

	if len(configurator.config.Monitor.RepoFilter) != 2 ||
		configurator.config.Monitor.RepoFilter[0] != "repo1" ||
		configurator.config.Monitor.RepoFilter[1] != "repo2" {
		t.Errorf("Repo filter not set correctly, got %v, want %v",
			configurator.config.Monitor.RepoFilter, []string{"repo1", "repo2"})
	}
}
