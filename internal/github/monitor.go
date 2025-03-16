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
	logging.Info("Monitoring for issues assigned to user", "username", m.username)

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

// MonitorResult represents the result of a monitoring operation with logs
type MonitorResult struct {
	Logs []string
	Err  error
}

// CheckOnce runs a single check for assigned issues
func (m *Monitor) CheckOnce() error {
	logging.Info("Running one-time check for assigned issues")
	logging.Info("Checking for issues assigned to user", "username", m.username)

	if len(m.config.Monitor.RepoFilter) > 0 {
		logging.Info("Checking specific repositories", "count", len(m.config.Monitor.RepoFilter))
		for _, repo := range m.config.Monitor.RepoFilter {
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

	// Only search for open issues assigned to the user
	// We use a broader search here since we'll be creating draft PRs for all assigned issues
	query := fmt.Sprintf("assignee:%s updated:>%s is:issue is:open",
		m.username,
		m.lastChecked.Format(time.RFC3339),
	)

	logging.Info("Using search query", "query", query)

	// Create full URL used for debug purposes
	fullURL := fmt.Sprintf("https://api.github.com/search/issues?q=%s", url.QueryEscape(query))
	logging.Info("Debug API URL", "url", fullURL)

	// Note about repo filtering:
	// Instead of filtering in the query which can cause permission issues,
	// we'll get all assigned issues and filter by repo in our code
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

	logging.Info("Found assigned issues according to GitHub API", "count", *result.Total)
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

	// This issue is assigned to the user, which is all we care about

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

		// We'll collect all comments but no need to check for references
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

	// Check if the issue is already closed
	if strings.ToLower(issue.State) == "closed" {
		logging.Info("Issue is closed, skipping")
		return nil
	}

	// Check if we've already created a draft PR for this issue
	if hasDraftPR, err := m.hasDraftPullRequest(issue.Owner, issue.Repo, issue.Number); err != nil {
		logging.Warn("Failed to check for existing draft PRs", "error", err)
	} else if hasDraftPR {
		logging.Info("Issue already has a draft PR, skipping")
		return nil
	}

	// Create a draft PR for this issue
	if err := m.createDraftPullRequest(issue); err != nil {
		logging.Warn("Failed to create draft PR", "error", err)
		// Continue processing even if draft PR creation fails
	} else {
		logging.Info("Created draft PR for issue", "number", issue.Number)
	}

	// For assigned issues, we'll always add a comment with our response
	logging.Info("Issue is assigned to user, preparing response")

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

	// Add special instruction for the response
	issueText += "\n\nPlease focus on responding to the assigned issue based on the most recent comments and activity."

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

// GetRecentAssignedIssues gets the most recent assigned issues for reporting
func (m *Monitor) GetRecentAssignedIssues(limit int) ([]*models.Issue, error) {
	// Search for recent open issues assigned to the user
	// We only want issues that are assigned to us
	query := fmt.Sprintf("assignee:%s updated:>%s is:issue is:open",
		m.username,
		time.Now().Add(-7*24*time.Hour).Format(time.RFC3339), // Last 7 days
	)

	// Note about repo filtering:
	// Instead of filtering in the query which can cause permission issues,
	// we'll get all assigned issues and filter by repo in our code
	// This avoids GitHub API search restrictions while still providing the filtering

	// Log what repositories we'll be filtering for
	if len(m.config.Monitor.RepoFilter) > 0 {
		logging.Info("Will filter recent assigned issues for repositories", "repos", strings.Join(m.config.Monitor.RepoFilter, ", "))
	} else {
		logging.Info("No repository filter applied, will show all recent assigned issues")
	}

	// Search for issues
	logging.Info("Searching for recent assigned issues", "query", query)

	searchOpts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	result, _, err := m.client.Search.Issues(context.Background(), query, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("error searching for recent assigned issues: %w", err)
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
				logging.Debug("Recent assigned issue does not match repository filter, skipping",
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

// hasDraftPullRequest checks if there's already a draft PR for this issue
func (m *Monitor) hasDraftPullRequest(owner, repo string, issueNumber int) (bool, error) {
	// Create a client for checking pull requests
	client := NewClient(m.config.GitHub.Token)
	
	logging.Info("Checking for existing draft PRs", 
		"issue", issueNumber, 
		"owner", owner, 
		"repo", repo)
	
	// Get all PRs that reference this issue
	prs, err := client.GetPullRequestsForIssue(owner, repo, issueNumber)
	if err != nil {
		logging.Warn("Error getting PRs for issue", 
			"issue", issueNumber, 
			"error", err)
		return false, fmt.Errorf("failed to get pull requests for issue: %w", err)
	}
	
	logging.Info("Found PRs referencing issue", 
		"issue", issueNumber, 
		"count", len(prs))
	
	// Check if any of these PRs are drafts created by our user
	for _, pr := range prs {
		logging.Info("Examining PR", 
			"number", pr.GetNumber(), 
			"title", pr.GetTitle(), 
			"draft", pr.GetDraft(),
			"user", pr.GetUser().GetLogin())
		
		if pr.GetDraft() && pr.GetUser().GetLogin() == m.username {
			logging.Info("Found existing draft PR", 
				"number", pr.GetNumber(),
				"title", pr.GetTitle(),
				"url", pr.GetHTMLURL())
			return true, nil
		}
	}
	
	logging.Info("No existing draft PRs found for issue", "issue", issueNumber)
	return false, nil
}

// createDraftPullRequest creates a new draft PR for the issue
func (m *Monitor) createDraftPullRequest(issue *models.Issue) error {
	// Create a client for creating pull requests
	client := NewClient(m.config.GitHub.Token)
	
	// Prepare branch name based on issue number and title
	// Sanitize the title for use in a branch name
	sanitizedTitle := strings.ToLower(issue.Title)
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, " ", "-")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, "/", "-")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, ":", "")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, ".", "")
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, ",", "")
	
	// Limit branch name length
	if len(sanitizedTitle) > 50 {
		sanitizedTitle = sanitizedTitle[:50]
	}
	
	branchName := fmt.Sprintf("issue-%d-%s", issue.Number, sanitizedTitle)
	logging.Info("Preparing to create branch for issue", 
		"issue_number", issue.Number, 
		"branch", branchName,
		"owner", issue.Owner,
		"repo", issue.Repo)
	
	// Get the default branch for the repository (usually 'main' or 'master')
	defaultBranch := "main" // Default fallback
	
	// Try to get repo info to determine default branch
	repoInfo, _, err := client.client.Repositories.Get(context.Background(), issue.Owner, issue.Repo)
	if err != nil {
		logging.Warn("Failed to get repository info", "error", err)
	} else if repoInfo.GetDefaultBranch() != "" {
		defaultBranch = repoInfo.GetDefaultBranch()
		logging.Info("Using repository default branch", "branch", defaultBranch)
	}
	
	// Try to create the branch
	logging.Info("Creating branch", "branch", branchName, "from", defaultBranch)
	err = client.CreateBranch(issue.Owner, issue.Repo, branchName, defaultBranch)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}
	
	// Prepare PR title and body
	prTitle := fmt.Sprintf("Draft: Fix for issue #%d - %s", issue.Number, issue.Title)
	prBody := fmt.Sprintf("This is an automatically generated draft PR for issue #%d.\n\n", issue.Number)
	prBody += fmt.Sprintf("## Issue\n[%s](%s)\n\n", issue.Title, issue.URL)
	prBody += "## Description\n"
	prBody += "This PR was automatically created to track progress on the linked issue.\n\n"
	prBody += "## TODO\n- [ ] Implement solution\n- [ ] Add tests\n- [ ] Update documentation\n"
	
	logging.Info("Creating draft PR", 
		"title", prTitle, 
		"head", branchName, 
		"base", defaultBranch)
	
	// Create the draft PR
	pr, err := client.CreateDraftPullRequest(
		issue.Owner,
		issue.Repo,
		prTitle,
		prBody,
		branchName,
		defaultBranch,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create draft PR: %w", err)
	}
	
	logging.Info("Successfully created draft PR", 
		"pr_number", pr.GetNumber(),
		"url", pr.GetHTMLURL())
	
	return nil
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
		"repo_filters":     m.config.Monitor.RepoFilter,
	}

	return stats
}

// GetUsername returns the GitHub username being used for monitoring
func (m *Monitor) GetUsername() string {
	return m.username
}
