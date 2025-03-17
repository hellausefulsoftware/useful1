// Package vcs provides interfaces and implementations for version control system interactions
package vcs

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
)

// IssueProcessor defines a function that processes a single issue
type IssueProcessor interface {
	Process(Issue) error
}

// Monitor provides a generic VCS monitor for any platform
type Monitor struct {
	service      Service
	config       *config.Config
	lastChecked  time.Time
	username     string
	processedIDs map[string]time.Time
	mutex        sync.Mutex
	processor    IssueProcessor
	repoFilter   []string
}

// MonitorConfig holds configuration for creating a monitor
type MonitorConfig struct {
	Config    *config.Config
	Service   Service
	Processor IssueProcessor
}

// NewMonitor creates a new VCS monitor with the given configuration
func NewMonitor(cfg MonitorConfig) (*Monitor, error) {
	if cfg.Config == nil {
		return nil, fmt.Errorf("config is required for monitor")
	}

	if cfg.Service == nil {
		return nil, fmt.Errorf("service is required for monitor")
	}

	// Get authenticated username
	username, err := cfg.Service.GetAuthenticatedUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated user: %w", err)
	}

	repoFilter := cfg.Config.Monitor.RepoFilter
	logging.Info("Creating new monitor",
		"username", username,
		"repo_filter_count", len(repoFilter))

	return &Monitor{
		service:      cfg.Service,
		config:       cfg.Config,
		lastChecked:  time.Now().Add(-24 * time.Hour), // Start by checking the last 24 hours
		username:     username,
		processedIDs: make(map[string]time.Time),
		processor:    cfg.Processor,
	}, nil
}

// Start begins the continuous monitoring process
func (m *Monitor) Start() error {
	logging.Info("Starting VCS monitor")
	logging.Info("Monitoring for issues assigned to user", "username", m.username)

	if len(m.repoFilter) > 0 {
		logging.Info("Monitoring specific repositories", "count", len(m.repoFilter))
		for _, repo := range m.repoFilter {
			logging.Info("Monitoring repository", "repo", repo)
		}
	} else {
		logging.Info("Monitoring all accessible repositories")
	}

	// Loop indefinitely, checking for new issues
	for {
		if err := m.checkForAssignedIssues(); err != nil {
			logging.Error("Failed to check for assigned issues", "error", err)
		}

		// Update last checked time
		m.lastChecked = time.Now()

		// Wait for the configured poll interval - convert from minutes to seconds
		pollIntervalSeconds := m.config.Monitor.PollInterval * 60
		logging.Info("Waiting before next check", "seconds", pollIntervalSeconds)
		time.Sleep(time.Duration(pollIntervalSeconds) * time.Second)
	}
}

// CheckOnce runs a single check for assigned issues
func (m *Monitor) CheckOnce() error {
	logging.Info("Running one-time check for assigned issues")
	logging.Info("Checking for issues assigned to user", "username", m.username)

	if len(m.repoFilter) > 0 {
		logging.Info("Checking specific repositories", "count", len(m.repoFilter))
		for _, repo := range m.repoFilter {
			logging.Info("Checking repository", "repo", repo)
		}
	} else {
		logging.Info("Checking all accessible repositories")
	}

	err := m.checkForAssignedIssues()
	if err != nil {
		logging.Error("Check failed", "error", err)
		return err
	}

	logging.Info("One-time check completed successfully")
	return nil
}

// checkForAssignedIssues checks for open issues where the user is assigned
func (m *Monitor) checkForAssignedIssues() error {
	// Log the username we're checking for
	logging.Info("Checking for issues assigned to user", "username", m.username)

	// Get assigned issues from the service since the last check time
	issues, err := m.service.GetAssignedIssues(m.username, m.lastChecked, 100)
	if err != nil {
		logging.Error("Failed to get assigned issues", "error", err)
		return fmt.Errorf("error getting assigned issues: %w", err)
	}

	logging.Info("Found assigned issues", "count", len(issues))

	var accessibleIssues int
	var matchingRepoIssues int

	// Process each issue
	for _, issue := range issues {
		accessibleIssues++

		// Apply repository filter if configured
		if len(m.repoFilter) > 0 {
			repoName := issue.GetOwner() + "/" + issue.GetRepo()
			repoFound := false

			for _, allowedRepo := range m.repoFilter {
				if strings.EqualFold(allowedRepo, repoName) {
					repoFound = true
					break
				}
			}

			// Skip issues that don't match our repository filter
			if !repoFound {
				logging.Debug("Issue does not match repository filter, skipping",
					"repo", repoName,
					"issue", issue.GetNumber())
				continue
			}
		}

		matchingRepoIssues++

		// Check if we've already processed this issue recently
		m.mutex.Lock()
		issueID := fmt.Sprintf("%s/%s#%d", issue.GetOwner(), issue.GetRepo(), issue.GetNumber())
		lastProcessed, exists := m.processedIDs[issueID]
		m.mutex.Unlock()

		if exists {
			timeSince := time.Since(lastProcessed)
			// Skip if we processed this issue in the last hour
			if timeSince < time.Hour {
				logging.Debug("Skipping recently processed issue",
					"number", issue.GetNumber(),
					"processed_ago", timeSince.Round(time.Second))
				continue
			}
		}

		// Get full issue data including comments
		fullIssue, err := m.service.GetIssueWithComments(issue.GetOwner(), issue.GetRepo(), issue.GetNumber())
		if err != nil {
			logging.Error("Failed to get issue details", "error", err)
			continue
		}

		// Process the issue using the issue processor
		if err := m.processor.Process(fullIssue); err != nil {
			logging.Error("Failed to process issue", "error", err)
			continue
		}

		// Mark issue as processed
		m.mutex.Lock()
		m.processedIDs[issueID] = time.Now()
		m.mutex.Unlock()
	}

	// Cleanup old processed IDs to prevent memory leaks
	m.cleanupProcessedIDs()

	// Create a summary message for both logging and TUI display
	summaryMsg := fmt.Sprintf("Issues summary: %d total, %d accessible, %d matching repo filter",
		len(issues), accessibleIssues, matchingRepoIssues)

	logging.Info(summaryMsg)
	return nil
}

// cleanupProcessedIDs removes entries older than 24 hours to prevent memory leaks
func (m *Monitor) cleanupProcessedIDs() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour)
	for id, timestamp := range m.processedIDs {
		if timestamp.Before(cutoff) {
			delete(m.processedIDs, id)
		}
	}
}

// GetStats returns monitoring statistics
func (m *Monitor) GetStats() map[string]interface{} {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	stats := map[string]interface{}{
		"start_time":       m.lastChecked.Add(-24 * time.Hour),
		"last_checked":     m.lastChecked,
		"issues_processed": len(m.processedIDs),
		"username":         m.username,
		"poll_interval":    m.config.Monitor.PollInterval,
		"repo_filters":     m.repoFilter,
	}

	return stats
}

// GetUsername returns the username being used for monitoring
func (m *Monitor) GetUsername() string {
	return m.username
}
