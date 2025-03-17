// Package github provides GitHub-specific implementations of VCS interfaces
package github

import (
	"github.com/hellausefulsoftware/useful1/internal/common/vcs"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// Provider implements vcs.ServiceProvider for GitHub
type Provider struct {
	config *config.Config
}

// NewProvider creates a new GitHub service provider
func NewProvider(cfg *config.Config) *Provider {
	return &Provider{
		config: cfg,
	}
}

// GetService returns a GitHub implementation of vcs.Service
func (p *Provider) GetService() vcs.Service {
	adapter, err := NewAdapter(p.config)
	if err != nil {
		logging.Error("Failed to create GitHub adapter", "error", err)
		return nil
	}
	
	return adapter
}