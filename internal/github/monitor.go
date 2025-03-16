package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/models"
	"golang.org/x/oauth2"
)

// Monitor handles GitHub issue monitoring
type Monitor struct {
	client       *github.Client
	config       *config.Config
	lastChecked  time.Time
	username     string
	responder    models.IssueResponder
	processedIDs map[int64]time.Time
	mutex        sync.Mutex
}

// NewMonitor creates a new issue monitor
func NewMonitor(cfg *config.Config, responder models.IssueResponder) *Monitor {
	// Create GitHub client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GitHub.Token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)
	
	// Check if username is configured
	username := cfg.GitHub.User
	if username == "" || username == "ENTER_GITHUB_USERNAME_HERE" {
		// Try to get username from API if not set
		logging.Warn("Username not configured or set to placeholder. Trying to get from GitHub API...")
		if user, _, err := client.Users.Get(context.Background(), ""); err == nil && user.GetLogin() != "" {
			username = user.GetLogin()
			logging.Info("Successfully retrieved username from GitHub API", "username", username)
		} else {
			logging.Error("Failed to get username from GitHub API", "error", err)
			logging.Warn("MONITORING WILL NOT WORK: Username is empty or placeholder. Please set your GitHub username in the config")
		}
	} else {
		logging.Info("Using configured username", "username", username)
	}

	return &Monitor{
		client:       client,
		config:       cfg,
		lastChecked:  time.Now().Add(-24 * time.Hour), // Start by checking the last 24 hours
		username:     username,
		responder:    responder,
		processedIDs: make(map[int64]time.Time),
	}
}

// Start begins the continuous monitoring process
func (m *Monitor) Start() error {
	logging.Info("Starting GitHub issue monitor")
	logging.Info("Monitoring for mentions of user", "username", m.username)

	if len(m.config.Monitor.RepoFilter) > 0 {
		logging.Info("Monitoring specific repositories", "count", len(m.config.Monitor.RepoFilter))
		for _, repo := range m.config.Monitor.RepoFilter {
			logging.Info("Monitoring repository", "repo", repo)
		}
	} else {
		logging.Info("Monitoring all accessible repositories")
	}

	// Loop indefinitely, checking for new issues
	for {
		if err := m.checkForMentions(); err != nil {
			logging.Error("Failed to check for mentions", "error", err)
		}

		// Update last checked time
		m.lastChecked = time.Now()

		// Wait for the configured poll interval - convert from minutes to seconds
		pollIntervalSeconds := m.config.Monitor.PollInterval * 60
		logging.Info("Waiting before next check", "seconds", pollIntervalSeconds)
		time.Sleep(time.Duration(pollIntervalSeconds) * time.Second)
	}
}

// MonitorResult represents the result of a monitoring operation with logs
type MonitorResult struct {
	Logs []string
	Err  error
}

// CheckOnce runs a single check for mentions
func (m *Monitor) CheckOnce() error {
	logging.Info("Running one-time check for mentions")
	logging.Info("Checking for mentions of user", "username", m.username)

	if len(m.config.Monitor.RepoFilter) > 0 {
		logging.Info("Checking specific repositories", "count", len(m.config.Monitor.RepoFilter))
		for _, repo := range m.config.Monitor.RepoFilter {
			logging.Info("Checking repository", "repo", repo)
		}
	} else {
		logging.Info("Checking all accessible repositories")
	}

	err := m.checkForMentions()

	if err != nil {
		logging.Error("Check failed", "error", err)
		return err
	}

	logging.Info("One-time check completed successfully")
	return nil
}

// checkForMentions checks for issues where the user is assigned
func (m *Monitor) checkForMentions() error {
	// Log the username we're checking for
	logging.Info("Checking for issues assigned to user", "username", m.username)
	
	// Only search for issues assigned to the user
	query := fmt.Sprintf("assignee:%s updated:>%s is:issue",
		m.username,
		m.lastChecked.Format(time.RFC3339),
	)
	
	logging.Info("Using search query", "query", query)
	
	// Create full URL used for debug purposes
	fullURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s", url.QueryEscape(query))
	logging.Info("Debug API URL", "url", fullURL)

	// Note about repo filtering:
	// Instead of filtering in the query which can cause permission issues,
	// we'll get all mentions and filter by repo in our code
	// This avoids GitHub API search restrictions while still providing the filtering
	
	// Log what repositories we'll be filtering for
	if len(m.config.Monitor.RepoFilter) > 0 {
		logging.Info("Will filter results for repositories", "repos", strings.Join(m.config.Monitor.RepoFilter, ", "))
	} else {
		logging.Info("No repository filter applied, will show all repositories")
	}

	logging.Info("Executing search query", "query", query)

	// Search for issues
	searchOpts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Perform the search
	ctx := context.Background()
	result, resp, err := m.client.Search.Issues(ctx, query, searchOpts)
	if err != nil {
		logging.Error("API call failed", "error", err)
		return fmt.Errorf("error searching for issues: %w", err)
	}

	// Log response info for debugging
	if resp != nil {
		logging.Info("API response info", 
			"status", resp.Status, 
			"rate_limit", resp.Rate.Limit,
			"rate_remaining", resp.Rate.Remaining)
	}

	logging.Info("Found issues with mentions according to GitHub API", "count", *result.Total)
	logging.Info("Note: GitHub search may include inaccessible repos or false positives")
	
	// Log username to verify it matches what GitHub expects
	logging.Info("Username being used for search", "username", m.username)
	
	var accessibleIssues int
	var matchingRepoIssues int
	
	// Log details about each issue found
	if *result.Total > 0 {
		logging.Info("Issues found in API results:")
		for i, issue := range result.Issues {
			logging.Info(fmt.Sprintf("Issue #%d", i+1), 
				"number", *issue.Number,
				"title", *issue.Title,
				"url", *issue.HTMLURL,
				"is_pr", issue.PullRequestLinks != nil)
		}
	} else {
		logging.Info("No issues found in API results")
	}

	// Process each issue
	for _, issue := range result.Issues {
		// Skip pull requests (even though we filter in the query, double-check)
		if issue.PullRequestLinks != nil {
			logging.Debug("Skipping PR", "number", *issue.Number)
			continue
		}

		// Extract owner/repo from issue URL
		parts := strings.Split(*issue.HTMLURL, "/")
		if len(parts) < 7 {
			logging.Warn("Skipping issue with invalid URL", "url", *issue.HTMLURL)
			continue
		}
		accessibleIssues++
		owner := parts[3]
		repo := parts[4]
		
		// Apply repository filter if configured 
		if len(m.config.Monitor.RepoFilter) > 0 {
			repoName := owner + "/" + repo
			repoFound := false
			
			for _, allowedRepo := range m.config.Monitor.RepoFilter {
				if strings.EqualFold(allowedRepo, repoName) {
					repoFound = true
					break
				}
			}
			
			// Skip issues that don't match our repository filter
			if !repoFound {
				logging.Debug("Issue does not match repository filter, skipping", 
					"repo", repoName, 
					"issue", *issue.Number)
				continue
			}
		}
		
		matchingRepoIssues++

		// Check if we've already processed this issue recently
		m.mutex.Lock()
		lastProcessed, exists := m.processedIDs[*issue.ID]
		m.mutex.Unlock()

		if exists {
			timeSince := time.Since(lastProcessed)
			// Skip if we processed this issue in the last hour
			if timeSince < time.Hour {
				logging.Debug("Skipping recently processed issue",
					"number", *issue.Number,
					"processed_ago", timeSince.Round(time.Second))
				continue
			}
		}

		// Get full issue data including comments
		fullIssue, err := m.getIssueWithComments(owner, repo, *issue.Number)
		if err != nil {
			logging.Error("Failed to get issue details", "error", err)
			continue
		}

		// Process the issue
		if err := m.processIssue(fullIssue); err != nil {
			logging.Error("Failed to process issue", "error", err)
			continue
		}

		// Mark issue as processed
		m.mutex.Lock()
		m.processedIDs[*issue.ID] = time.Now()
		m.mutex.Unlock()
	}

	// Cleanup old processed IDs to prevent memory leaks
	m.cleanupProcessedIDs()
	
	// Create a summary message for both logging and TUI display
	summaryMsg := fmt.Sprintf("Issues summary: %d reported by GitHub, %d accessible, %d matching repo filter", 
		*result.Total, accessibleIssues, matchingRepoIssues)
	
	// Log the summary - this is a special log that will be captured by the TUI
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

// getIssueWithComments retrieves an issue with all its comments
func (m *Monitor) getIssueWithComments(owner, repo string, number int) (*models.Issue, error) {
	ctx := context.Background()

	// Get issue details
	issue, _, err := m.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("error getting issue: %w", err)
	}

	// Create our issue object
	result := &models.Issue{
		Owner:     owner,
		Repo:      repo,
		Number:    number,
		Title:     *issue.Title,
		Body:      *issue.Body,
		User:      *issue.User.Login,
		State:     *issue.State,
		CreatedAt: *issue.CreatedAt,
		UpdatedAt: *issue.UpdatedAt,
		URL:       *issue.HTMLURL,
		Comments:  []*models.IssueComment{},
		Labels:    make([]string, 0, len(issue.Labels)),
		Assignees: make([]string, 0, len(issue.Assignees)),
	}

	// Add labels
	for _, label := range issue.Labels {
		if label.Name != nil {
			result.Labels = append(result.Labels, *label.Name)
		}
	}

	// Add assignees
	for _, assignee := range issue.Assignees {
		if assignee.Login != nil {
			result.Assignees = append(result.Assignees, *assignee.Login)
		}
	}

	// Check if the user is mentioned in the issue body
	if strings.Contains(strings.ToLower(*issue.Body), strings.ToLower("@"+m.username)) {
		result.MentionsUser = true
	}

	// Get comments
	comments, _, err := m.client.Issues.ListComments(
		ctx,
		owner,
		repo,
		number,
		&github.IssueListCommentsOptions{
			Sort:      github.String("created"),
			Direction: github.String("asc"),
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error getting comments: %w", err)
	}

	// Process comments
	for _, comment := range comments {
		// Add to our comments list
		result.Comments = append(result.Comments, &models.IssueComment{
			ID:        *comment.ID,
			User:      *comment.User.Login,
			Body:      *comment.Body,
			CreatedAt: *comment.CreatedAt,
		})

		// Check if the user is mentioned in the comment
		if strings.Contains(strings.ToLower(*comment.Body), strings.ToLower("@"+m.username)) {
			result.MentionsUser = true
		}
	}

	return result, nil
}

// processIssue processes an issue and responds if necessary
func (m *Monitor) processIssue(issue *models.Issue) error {
	logging.Info("Processing issue",
		"number", issue.Number,
		"owner", issue.Owner,
		"repo", issue.Repo,
		"title", issue.Title)

	// Apply repository filter if configured
	if len(m.config.Monitor.RepoFilter) > 0 {
		// Check if this issue belongs to one of our filtered repositories
		repoName := issue.Owner + "/" + issue.Repo
		repoFound := false
		
		for _, allowedRepo := range m.config.Monitor.RepoFilter {
			if strings.EqualFold(allowedRepo, repoName) {
				repoFound = true
				break
			}
		}
		
		if !repoFound {
			logging.Debug("Issue does not match repository filter, skipping", 
				"repo", repoName, 
				"allowed_repos", strings.Join(m.config.Monitor.RepoFilter, ", "))
			return nil
		}
	}

	// Check if we need to respond to this issue
	if !issue.MentionsUser {
		logging.Info("No direct mention found, skipping")
		return nil
	}

	// Check if the issue is already closed
	if strings.ToLower(issue.State) == "closed" {
		logging.Info("Issue is closed, skipping")
		return nil
	}

	logging.Info("Mention found, preparing response")

	// Check if the last comment was from the bot
	if len(issue.Comments) > 0 && issue.Comments[len(issue.Comments)-1].User == m.username {
		logging.Info("Last comment was from bot, skipping to avoid duplicate responses")
		return nil
	}

	// Create a combined text of the issue for analysis
	issueText := fmt.Sprintf("Issue #%d: %s\n\n", issue.Number, issue.Title)
	issueText += fmt.Sprintf("Created by: %s\n", issue.User)
	issueText += fmt.Sprintf("State: %s\n", issue.State)
	issueText += fmt.Sprintf("URL: %s\n", issue.URL)

	if len(issue.Labels) > 0 {
		issueText += fmt.Sprintf("Labels: %s\n", strings.Join(issue.Labels, ", "))
	}

	if len(issue.Assignees) > 0 {
		issueText += fmt.Sprintf("Assignees: %s\n", strings.Join(issue.Assignees, ", "))
	}

	issueText += "\nIssue Description:\n"
	issueText += issue.Body
	issueText += "\n\nComments:\n"

	for i, comment := range issue.Comments {
		issueText += fmt.Sprintf("\n--- Comment #%d by %s (%s) ---\n",
			i+1, comment.User, comment.CreatedAt.Format(time.RFC1123))
		issueText += comment.Body
		issueText += "\n"
	}

	// Add special instruction for the most recent mention
	issueText += "\n\nPlease focus on responding to the most recent mention of your username in this issue."

	// Use the issue responder to generate a response
	err := m.responder.RespondToIssueText(
		issue.Owner,
		issue.Repo,
		issue.Number,
		issueText,
	)

	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}

	logging.Info("Successfully responded to issue", "number", issue.Number)
	return nil
}

// GetRecentMentions gets the most recent mentions for reporting
func (m *Monitor) GetRecentMentions(limit int) ([]*models.Issue, error) {
	// Search for recent issues with mentions
	query := fmt.Sprintf("mentions:%s updated:>%s is:issue",
		m.username,
		time.Now().Add(-7*24*time.Hour).Format(time.RFC3339), // Last 7 days
	)

	// Note about repo filtering:
	// Instead of filtering in the query which can cause permission issues,
	// we'll get all mentions and filter by repo in our code
	// This avoids GitHub API search restrictions while still providing the filtering
	
	// Log what repositories we'll be filtering for
	if len(m.config.Monitor.RepoFilter) > 0 {
		logging.Info("Will filter recent mentions for repositories", "repos", strings.Join(m.config.Monitor.RepoFilter, ", "))
	} else {
		logging.Info("No repository filter applied, will show all recent mentions")
	}

	// Search for issues
	logging.Info("Searching for recent mentions", "query", query)

	searchOpts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	result, _, err := m.client.Search.Issues(context.Background(), query, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("error searching for recent mentions: %w", err)
	}

	var issues []*models.Issue

	// Process each issue
	for _, issue := range result.Issues {
		// Skip pull requests
		if issue.PullRequestLinks != nil {
			continue
		}

		// Extract owner/repo from issue URL
		parts := strings.Split(*issue.HTMLURL, "/")
		if len(parts) < 7 {
			continue
		}
		owner := parts[3]
		repo := parts[4]
		
		// Apply repository filter if configured
		if len(m.config.Monitor.RepoFilter) > 0 {
			repoName := owner + "/" + repo
			repoFound := false
			
			for _, allowedRepo := range m.config.Monitor.RepoFilter {
				if strings.EqualFold(allowedRepo, repoName) {
					repoFound = true
					break
				}
			}
			
			// Skip issues that don't match our repository filter
			if !repoFound {
				logging.Debug("Recent mention does not match repository filter, skipping", 
					"repo", repoName)
				continue
			}
		}

		// Create a simplified issue object
		simpleIssue := &models.Issue{
			Owner:     owner,
			Repo:      repo,
			Number:    *issue.Number,
			Title:     *issue.Title,
			User:      *issue.User.Login,
			State:     *issue.State,
			UpdatedAt: *issue.UpdatedAt,
			URL:       *issue.HTMLURL,
		}

		issues = append(issues, simpleIssue)

		// Limit the number of issues
		if len(issues) >= limit {
			break
		}
	}

	return issues, nil
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
		"check_mentions":   m.config.Monitor.CheckMentions,
		"repo_filters":     m.config.Monitor.RepoFilter,
	}

	return stats
}
