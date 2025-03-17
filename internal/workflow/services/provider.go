package services

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"golang.org/x/oauth2"
)

// Provider creates service instances
type Provider struct {
	config *config.Config
}

// NewProvider creates a new service provider
func NewProvider(cfg *config.Config) *Provider {
	return &Provider{
		config: cfg,
	}
}

// GetGitHubClient returns a GitHub API client
func (p *Provider) GetGitHubClient() *github.Client {
	// Create GitHub client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: p.config.GitHub.Token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

// GetCLIExecutor returns a CLI executor
func (p *Provider) GetCLIExecutor() *cli.Executor {
	return cli.NewExecutor(p.config)
}
