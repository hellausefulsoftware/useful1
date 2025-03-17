// Package vcs provides interfaces and implementations for version control system interactions
package vcs

import (
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// Provider creates VCS services based on configuration
type Provider struct {
	config *config.Config
}

// NewProvider creates a new service provider
func NewProvider(cfg *config.Config) *Provider {
	return &Provider{config: cfg}
}

// GetService returns the appropriate service based on configuration
func (p *Provider) GetService() Service {
	// Default to GitHub if not specified
	platform := p.config.VCS.Platform
	if platform == "" {
		platform = "github"
		logging.Info("No VCS platform specified in config, defaulting to GitHub")
	}

	// In actual implementation, we would have switch cases for different platforms
	// For now, the GitHub implementation will be handled separately
	logging.Info("Using VCS platform", "platform", platform)

	// The actual return will happen in the adapter implementation
	return nil
}
