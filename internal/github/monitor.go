package github

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/config"
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

	return &Monitor{
		client:       client,
		config:       cfg,
		lastChecked:  time.Now().Add(-24 * time.Hour), // Start by checking the last 24 hours
		username:     cfg.GitHub.User,
		responder:    responder,
		processedIDs: make(map[int64]time.Time),
	}
}

// Start begins the continuous monitoring process
func (m *Monitor) Start() error {
	fmt.Println("Starting GitHub issue monitor...")
	fmt.Printf("Monitoring for mentions of user: %s\n", m.username)

	if len(m.config.Monitor.RepoFilter) > 0 {
		fmt.Println("Monitoring these repositories:")
		for _, repo := range m.config.Monitor.RepoFilter {
			fmt.Printf("  - %s\n", repo)
		}
	} else {
		fmt.Println("Monitoring all accessible repositories")
	}

	// Loop indefinitely, checking for new issues
	for {
		if err := m.checkForMentions(); err != nil {
			fmt.Printf("Error checking for mentions: %v\n", err)
		}

		// Update last checked time
		m.lastChecked = time.Now()

		// Wait for the configured poll interval
		fmt.Printf("Waiting %d minutes before next check...\n", m.config.Monitor.PollInterval)
		time.Sleep(time.Duration(m.config.Monitor.PollInterval) * time.Minute)
	}
}

// CheckOnce runs a single check for mentions
func (m *Monitor) CheckOnce() error {
	fmt.Println("Running one-time check for mentions...")
	fmt.Printf("Checking for mentions of user: %s\n", m.username)

	if len(m.config.Monitor.RepoFilter) > 0 {
		fmt.Println("Checking these repositories:")
		for _, repo := range m.config.Monitor.RepoFilter {
			fmt.Printf("  - %s\n", repo)
		}
	} else {
		fmt.Println("Checking all accessible repositories")
	}

	err := m.checkForMentions()

	if err != nil {
		return err
	}

	fmt.Println("One-time check completed successfully")
	return nil
}

// checkForMentions checks for issues where the user is mentioned
func (m *Monitor) checkForMentions() error {
	fmt.Println("Checking for mentions...")

	// Search for issues with mentions
	query := fmt.Sprintf("mentions:%s updated:>%s is:issue",
		m.username,
		m.lastChecked.Format(time.RFC3339),
	)

	// Add repo filter if configured
	if len(m.config.Monitor.RepoFilter) > 0 {
		repos := strings.Join(m.config.Monitor.RepoFilter, " repo:")
		query += " repo:" + repos
	}

	fmt.Printf("Using search query: %s\n", query)

	// Search for issues
	searchOpts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	result, _, err := m.client.Search.Issues(context.Background(), query, searchOpts)
	if err != nil {
		return fmt.Errorf("error searching for issues: %w", err)
	}

	fmt.Printf("Found %d issues with mentions\n", *result.Total)

	// Process each issue
	for _, issue := range result.Issues {
		// Skip pull requests (even though we filter in the query, double-check)
		if issue.PullRequestLinks != nil {
			fmt.Printf("Skipping PR #%d\n", *issue.Number)
			continue
		}

		// Extract owner/repo from issue URL
		parts := strings.Split(*issue.HTMLURL, "/")
		if len(parts) < 7 {
			fmt.Printf("Skipping issue with invalid URL: %s\n", *issue.HTMLURL)
			continue
		}
		owner := parts[3]
		repo := parts[4]

		// Check if we've already processed this issue recently
		m.mutex.Lock()
		lastProcessed, exists := m.processedIDs[*issue.ID]
		m.mutex.Unlock()

		if exists {
			timeSince := time.Since(lastProcessed)
			// Skip if we processed this issue in the last hour
			if timeSince < time.Hour {
				fmt.Printf("Skipping recently processed issue #%d (processed %s ago)\n",
					*issue.Number, timeSince.Round(time.Second))
				continue
			}
		}

		// Get full issue data including comments
		fullIssue, err := m.getIssueWithComments(owner, repo, *issue.Number)
		if err != nil {
			fmt.Printf("Error getting issue details: %v\n", err)
			continue
		}

		// Process the issue
		if err := m.processIssue(fullIssue); err != nil {
			fmt.Printf("Error processing issue: %v\n", err)
			continue
		}

		// Mark issue as processed
		m.mutex.Lock()
		m.processedIDs[*issue.ID] = time.Now()
		m.mutex.Unlock()
	}

	// Cleanup old processed IDs to prevent memory leaks
	m.cleanupProcessedIDs()

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
	fmt.Printf("Processing issue #%d in %s/%s: %s\n",
		issue.Number, issue.Owner, issue.Repo, issue.Title)

	// Check if we need to respond to this issue
	if !issue.MentionsUser {
		fmt.Println("  - No direct mention found, skipping")
		return nil
	}

	// Check if the issue is already closed
	if strings.ToLower(issue.State) == "closed" {
		fmt.Println("  - Issue is closed, skipping")
		return nil
	}

	fmt.Println("  - Mention found, preparing response")

	// Check if the last comment was from the bot
	if len(issue.Comments) > 0 && issue.Comments[len(issue.Comments)-1].User == m.username {
		fmt.Println("  - Last comment was from bot, skipping to avoid duplicate responses")
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

	fmt.Printf("  - Successfully responded to issue #%d\n", issue.Number)
	return nil
}

// GetRecentMentions gets the most recent mentions for reporting
func (m *Monitor) GetRecentMentions(limit int) ([]*models.Issue, error) {
	// Search for recent issues with mentions
	query := fmt.Sprintf("mentions:%s updated:>%s is:issue",
		m.username,
		time.Now().Add(-7*24*time.Hour).Format(time.RFC3339), // Last 7 days
	)

	// Add repo filter if configured
	if len(m.config.Monitor.RepoFilter) > 0 {
		repos := strings.Join(m.config.Monitor.RepoFilter, " repo:")
		query += " repo:" + repos
	}

	// Search for issues
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
