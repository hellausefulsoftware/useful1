// Package vcs provides interfaces and implementations for version control system interactions
package vcs

import (
	"fmt"

	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// Factory creates VCS service providers based on platform
type Factory struct {
	config *config.Config
	// Map of platform names to provider creator functions
	providers map[string]func(*config.Config) ServiceProvider
}

// NewFactory creates a new VCS factory
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{
		config:    cfg,
		providers: make(map[string]func(*config.Config) ServiceProvider),
	}
}

// RegisterProvider registers a provider creator function for a platform
func (f *Factory) RegisterProvider(platform string, creator func(*config.Config) ServiceProvider) {
	f.providers[platform] = creator
}

// GetServiceProvider returns the appropriate service provider based on configuration
func (f *Factory) GetServiceProvider() (ServiceProvider, error) {
	platform := f.config.VCS.Platform
	if platform == "" {
		platform = "github"
		logging.Info("No VCS platform specified in config, defaulting to GitHub")
	}

	creator, ok := f.providers[platform]
	if !ok {
		return nil, fmt.Errorf("no provider registered for platform: %s", platform)
	}

	provider := creator(f.config)
	return provider, nil
}

// GetService is a convenience method to get a Service directly
func (f *Factory) GetService() (Service, error) {
	provider, err := f.GetServiceProvider()
	if err != nil {
		return nil, err
	}

	service := provider.GetService()
	if service == nil {
		return nil, fmt.Errorf("failed to get service from provider for platform: %s", f.config.VCS.Platform)
	}

	return service, nil
}