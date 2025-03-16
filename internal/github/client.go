package github

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/anthropic"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/models"
	"golang.org/x/oauth2"
)

// Client handles GitHub API interactions
type Client struct {
	client *github.Client
}

// NewClient creates a new GitHub client
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return &Client{
		client: github.NewClient(tc),
	}
}

// RespondToIssue posts a comment on a GitHub issue
func (c *Client) RespondToIssue(owner, repo string, issueNumber int, comment string) error {
	_, _, err := c.client.Issues.CreateComment(
		context.Background(),
		owner,
		repo,
		issueNumber,
		&github.IssueComment{
			Body: github.String(comment),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create issue comment: %w", err)
	}

	return nil
}

// CreatePullRequest creates a new pull request
func (c *Client) CreatePullRequest(owner, repo, title, body, head, base string) (*github.PullRequest, error) {
	newPR := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(body),
		Head:  github.String(head),
		Base:  github.String(base),
	}

	pr, _, err := c.client.PullRequests.Create(
		context.Background(),
		owner,
		repo,
		newPR,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return pr, nil
}

// CreateDraftPullRequest creates a new draft pull request
func (c *Client) CreateDraftPullRequest(owner, repo, title, body, head, base string) (*github.PullRequest, error) {
	// Check if branch name is empty - if so, we need to generate it
	logging.Debug("CreateDraftPullRequest called with parameters",
		"owner", owner,
		"repo", repo,
		"title", title,
		"head_branch", head,
		"base_branch", base,
		"body_length", len(body))

	if head == "" {
		logging.Info("Empty branch name provided, creating issue structure for Anthropic API",
			"owner", owner,
			"repo", repo)

		// Create a minimal issue structure for analysis
		issueNum := 0
		if strings.Contains(title, "#") {
			parts := strings.Split(title, "#")
			if len(parts) > 1 {
				numStr := strings.Split(parts[1], " ")[0]
				num, err := strconv.Atoi(numStr)
				if err == nil {
					issueNum = num
				}
			}
		}

		// Create a temporary issue object to pass to the Anthropic analyzer
		issueObj := &models.Issue{
			Owner:  owner,
			Repo:   repo,
			Number: issueNum,
			Title:  title,
			Body:   body,
		}

		// Access config to get Anthropic API key
		// Note: Ideally we should pass the config from monitor.go, but we'll load it here
		var branchName string
		cfg, err := config.LoadConfig()
		if err != nil {
			logging.Warn("Failed to load config for Anthropic API access, using fallback",
				"error", err)
			// Use fallback branch name generation
			var genErr error
			branchName, _, genErr = c.generateBranchAndTitle(owner, repo, title, body)
			if genErr != nil {
				return nil, fmt.Errorf("failed to generate branch name: %w", genErr)
			}
			logging.Info("Using fallback branch name", "branch", branchName)
		} else {
			// Create Anthropic analyzer with proper config
			analyzer := anthropic.NewAnalyzer(cfg)
			logging.Info("Created Anthropic analyzer",
				"token_available", cfg.Anthropic.Token != "",
				"token_length", len(cfg.Anthropic.Token))

			// Generate the branch name using Anthropic analyzer
			// This will use the proper classification-based branch prefixes (bugfix/, feature/, chore/)
			aiGeneratedBranch, aiErr := analyzer.AnalyzeIssue(issueObj)
			if aiErr != nil {
				logging.Warn("Failed to generate branch name with Anthropic API, using fallback",
					"error", aiErr)
				// Use fallback branch name generation
				var genErr error
				branchName, _, genErr = c.generateBranchAndTitle(owner, repo, title, body)
				if genErr != nil {
					return nil, fmt.Errorf("failed to generate branch name: %w", genErr)
				}
			} else {
				// Use the AI-generated branch name
				branchName = aiGeneratedBranch
				logging.Info("Using AI-generated branch name", "branch", branchName)
			}
		}

		// Set the head to the generated branch name and add Draft prefix to title
		title = "Draft: " + title
		head = branchName // CRITICAL BUG FIX: Set head to the generated branch name

		logging.Debug("Set head branch to generated branch name", "head", head)
		logging.Info("Generated branch and title for draft PR",
			"branch", head,
			"title", title)

		// Create the branch using our CreateBranch method
		logging.Info("Creating new branch for draft PR",
			"branch", head,
			"base", base)

		if err := c.CreateBranch(owner, repo, head, base); err != nil {
			logging.Error("Failed to create branch for draft PR",
				"branch", head,
				"error", err)
			return nil, fmt.Errorf("failed to create branch for draft PR: %w", err)
		}

		logging.Info("Successfully created branch for draft PR",
			"branch", head,
			"owner", owner,
			"repo", repo)
	}

	// Add marker to the body to indicate this was generated with useful1
	draftBody := body + "\n\n**This PR was generated using https://github.com/hellausefulsoftware/useful1**"

	newPR := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(draftBody),
		Head:  github.String(head),
		Base:  github.String(base),
		Draft: github.Bool(true), // Mark as draft
	}

	logging.Info("Attempting to create draft PR via GitHub API",
		"owner", owner,
		"repo", repo,
		"title", title,
		"head", head,
		"base", base,
		"draft", true)

	logging.Debug("Making GitHub API call to create PR",
		"owner", owner,
		"repo", repo,
		"head", *newPR.Head,
		"base", *newPR.Base,
		"draft", *newPR.Draft)

	pr, resp, err := c.client.PullRequests.Create(
		context.Background(),
		owner,
		repo,
		newPR,
	)

	if err != nil {
		// Log more details about the error
		if resp != nil {
			logging.Error("GitHub API error details",
				"status", resp.Status,
				"rate_limit", resp.Rate.Limit,
				"rate_remaining", resp.Rate.Remaining)
		}

		// Check if this is a common error that we can handle
		if strings.Contains(err.Error(), "No commits between") {
			logging.Warn("Cannot create PR: No commits between branches",
				"head", head,
				"base", base)
			return nil, fmt.Errorf("cannot create draft PR: no commits between branches: %w", err)
		}

		if strings.Contains(err.Error(), "A pull request already exists") {
			logging.Warn("Cannot create PR: A pull request already exists for these branches",
				"head", head,
				"base", base)
			return nil, fmt.Errorf("cannot create draft PR: a pull request already exists: %w", err)
		}

		return nil, fmt.Errorf("failed to create draft PR: %w", err)
	}

	return pr, nil
}

// GetPullRequestsForIssue gets all pull requests that reference an issue
func (c *Client) GetPullRequestsForIssue(owner, repo string, issueNumber int) ([]*github.PullRequest, error) {
	// Search for PRs that mention the issue number in different formats
	query := fmt.Sprintf("repo:%s/%s is:pr #%d OR \"issue %d\" OR \"fixes %d\" OR \"closes %d\"",
		owner, repo, issueNumber, issueNumber, issueNumber, issueNumber)

	logging.Debug("Searching for PRs with query", "query", query)

	var allPRs []*github.PullRequest
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// First, try the search method
	result, resp, err := c.client.Search.Issues(context.Background(), query, opts)
	if err != nil {
		logging.Debug("Search error", "error", err)
		// Fall back to listing all PRs
		return c.checkAllPullRequests(owner, repo, issueNumber)
	}

	if result.GetTotal() > 0 {
		logging.Debug("Found search results", "count", *result.Total)

		for _, issue := range result.Issues {
			if issue.PullRequestLinks != nil {
				// This is a PR, not an issue
				pr, _, prErr := c.client.PullRequests.Get(
					context.Background(),
					owner,
					repo,
					*issue.Number,
				)

				if prErr != nil {
					logging.Debug("Error getting PR", "number", *issue.Number, "error", prErr)
					continue
				}

				allPRs = append(allPRs, pr)
			}
		}

		// Get next pages if available
		for resp != nil && resp.NextPage != 0 {
			opts.Page = resp.NextPage
			result, resp, err = c.client.Search.Issues(context.Background(), query, opts)
			if err != nil {
				break
			}

			for _, issue := range result.Issues {
				if issue.PullRequestLinks != nil {
					// This is a PR, not an issue
					pr, _, err := c.client.PullRequests.Get(
						context.Background(),
						owner,
						repo,
						*issue.Number,
					)

					if err != nil {
						continue
					}

					allPRs = append(allPRs, pr)
				}
			}
		}
	} else {
		// If search returned no results, try listing all PRs
		additionalPRs, err := c.checkAllPullRequests(owner, repo, issueNumber)
		if err == nil {
			allPRs = append(allPRs, additionalPRs...)
		}
	}

	return allPRs, nil
}

// checkAllPullRequests gets all PRs in a repo and checks their bodies for issue references
func (c *Client) checkAllPullRequests(owner, repo string, issueNumber int) ([]*github.PullRequest, error) {
	var allPRs []*github.PullRequest
	opts := &github.PullRequestListOptions{
		State: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	logging.Debug("Listing all PRs to check for issue", "owner", owner, "repo", repo, "issue", issueNumber)

	for {
		prs, resp, err := c.client.PullRequests.List(
			context.Background(),
			owner,
			repo,
			opts,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to list PRs: %w", err)
		}

		// Check each PR body for the issue number
		issueRef := fmt.Sprintf("#%d", issueNumber)
		for _, pr := range prs {
			if pr.Body != nil && strings.Contains(*pr.Body, issueRef) {
				allPRs = append(allPRs, pr)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	logging.Debug("Found PRs that reference issue", "count", len(allPRs), "issue", issueNumber)
	return allPRs, nil
}

// CreateBranch creates a new branch from the specified base branch
func (c *Client) CreateBranch(owner, repo, branchName, baseBranch string) error {
	// Get the reference to the base branch
	baseRef, _, err := c.client.Git.GetRef(
		context.Background(),
		owner,
		repo,
		fmt.Sprintf("refs/heads/%s", baseBranch),
	)
	if err != nil {
		return fmt.Errorf("failed to get base branch reference: %w", err)
	}

	// Create a new reference (branch) using the SHA from the base branch
	newRef := &github.Reference{
		Ref: github.String(fmt.Sprintf("refs/heads/%s", branchName)),
		Object: &github.GitObject{
			SHA: baseRef.Object.SHA,
		},
	}

	_, _, err = c.client.Git.CreateRef(
		context.Background(),
		owner,
		repo,
		newRef,
	)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	logging.Info("Successfully created branch", "branch", branchName)
	return nil
}

// CreateImplementationFile creates a simple implementation.txt file in a local clone of the branch
func (c *Client) CreateImplementationFile(owner, repo, branchName string, issueNumber int) error {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user's home directory: %w", err)
	}

	// Create temp directory path
	tempDir := fmt.Sprintf("%s/.useful1/temp/%s_%d", homeDir, repo, issueNumber)

	// Create the temp directory if it doesn't exist
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	logging.Info("Creating implementation file in local clone",
		"owner", owner,
		"repo", repo,
		"branch", branchName,
		"issue", issueNumber,
		"dir", tempDir)

	// Create implementation.txt file with "WIP" content
	implementationPath := fmt.Sprintf("%s/implementation.txt", tempDir)
	err = os.WriteFile(implementationPath, []byte("WIP"), 0644)
	if err != nil {
		return fmt.Errorf("failed to create implementation.txt: %w", err)
	}

	// Create a commit with the new file
	commitMessage := "Add initial implementation file"

	// Create the file in the GitHub repo
	_, _, err = c.client.Repositories.CreateFile(
		context.Background(),
		owner,
		repo,
		"implementation.txt",
		&github.RepositoryContentFileOptions{
			Message: &commitMessage,
			Content: []byte("WIP"),
			Branch:  &branchName,
		},
	)

	if err != nil {
		logging.Warn("Failed to create implementation.txt in repository",
			"branch", branchName,
			"error", err)
		return fmt.Errorf("failed to create implementation file: %w", err)
	}

	logging.Info("Created implementation file and committed to branch",
		"branch", branchName,
		"local_path", implementationPath)

	return nil
}

// GetRepositories gets a list of repositories the authenticated user has access to
func (c *Client) GetRepositories() ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		repos, resp, err := c.client.Repositories.List(context.Background(), "", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// GetUserInfo gets information about the authenticated user
func (c *Client) GetUserInfo() (*github.User, error) {
	user, _, err := c.client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return user, nil
}

// GetIssues gets issues for a repository
func (c *Client) GetIssues(owner, repo string) ([]*github.Issue, error) {
	var allIssues []*github.Issue
	opts := &github.IssueListByRepoOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		issues, resp, err := c.client.Issues.ListByRepo(context.Background(), owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list issues: %w", err)
		}

		allIssues = append(allIssues, issues...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allIssues, nil
}

// GetIssueComments gets comments for an issue
func (c *Client) GetIssueComments(owner, repo string, issueNumber int) ([]*github.IssueComment, error) {
	var allComments []*github.IssueComment
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		comments, resp, err := c.client.Issues.ListComments(context.Background(), owner, repo, issueNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list comments: %w", err)
		}

		allComments = append(allComments, comments...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// generateBranchAndTitle generates a branch name and PR title
func (c *Client) generateBranchAndTitle(owner, repo, title, body string) (string, string, error) {
	logging.Info("Generating branch name and title",
		"owner", owner,
		"repo", repo,
		"title_length", len(title),
		"body_length", len(body))

	// 1. Extract issue number from title if present
	issueNum := 0
	if strings.Contains(title, "#") {
		parts := strings.Split(title, "#")
		if len(parts) > 1 {
			numStr := strings.Split(parts[1], " ")[0]
			num, err := strconv.Atoi(numStr)
			if err == nil {
				issueNum = num
				logging.Info("Extracted issue number from title", "issue_number", issueNum)
			}
		}
	}

	// 2. Analyze title and body to determine issue type
	// Look for clues in the text to classify as bug, feature, or chore
	issueType := "feature" // Default to feature

	lowerTitle := strings.ToLower(title)
	lowerBody := strings.ToLower(body)

	// Simple classification based on keywords
	if strings.Contains(lowerTitle, "bug") ||
		strings.Contains(lowerTitle, "fix") ||
		strings.Contains(lowerTitle, "issue") ||
		strings.Contains(lowerTitle, "problem") ||
		strings.Contains(lowerBody, "bug") ||
		strings.Contains(lowerBody, "fix") ||
		strings.Contains(lowerBody, "doesn't work") ||
		strings.Contains(lowerBody, "broken") {
		issueType = "bugfix"
	} else if strings.Contains(lowerTitle, "refactor") ||
		strings.Contains(lowerTitle, "clean") ||
		strings.Contains(lowerTitle, "doc") ||
		strings.Contains(lowerBody, "refactor") ||
		strings.Contains(lowerBody, "clean") ||
		strings.Contains(lowerBody, "document") {
		issueType = "chore"
	}

	logging.Info("Determined issue type", "type", issueType)

	// 3. Extract meaningful keywords from title
	words := strings.Fields(title)
	var keywords []string

	// Skip common words, articles, etc. to extract more meaningful terms
	skipWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true,
		"is": true, "in": true, "to": true, "for": true,
		"with": true, "of": true, "on": true, "draft": true,
		"fix": true, "issue": true, "bug": true, "pr": true,
		"pull": true, "request": true, "#": true,
	}

	// Get up to 3-5 meaningful words
	for _, word := range words {
		w := strings.ToLower(word)
		if !skipWords[w] && len(keywords) < 5 {
			// Remove special characters
			w = sanitizeBranchName(w)
			if w != "" && len(w) > 2 { // Skip very short words
				keywords = append(keywords, w)
			}
		}
	}

	// If we couldn't extract good keywords, use a more generic approach
	if len(keywords) < 2 {
		simplifiedTitle := sanitizeBranchName(title)
		keywords = strings.Split(simplifiedTitle, "-")
		// Limit to 3-5 meaningful segments
		if len(keywords) > 5 {
			keywords = keywords[:5]
		}
	}

	// 4. Create a descriptive branch name
	var descriptivePart string
	if len(keywords) > 0 {
		descriptivePart = strings.Join(keywords, "-")
	} else {
		descriptivePart = "update"
	}

	// 5. Generate the final branch name with proper prefix
	var branchName string
	if issueNum > 0 {
		// Format: type/issue-123-descriptive-name
		branchName = fmt.Sprintf("%s/issue-%d-%s", issueType, issueNum, descriptivePart)
	} else {
		// If we couldn't extract an issue number, use a timestamp
		timestamp := time.Now().Format("20060102")
		branchName = fmt.Sprintf("%s/%s-%s", issueType, timestamp, descriptivePart)
	}

	// 6. Create an appropriate PR title
	prTitle := title

	// Add type prefix to title if not already present
	if !strings.HasPrefix(strings.ToLower(title), "fix:") &&
		!strings.HasPrefix(strings.ToLower(title), "feature:") &&
		!strings.HasPrefix(strings.ToLower(title), "chore:") {
		switch issueType {
		case "bugfix":
			prTitle = "Fix: " + title
		case "feature":
			prTitle = "Feature: " + title
		case "chore":
			prTitle = "Chore: " + title
		}
	}

	logging.Info("Generated branch name with intelligent classification",
		"branch", branchName,
		"type", issueType,
		"keywords", strings.Join(keywords, ", "))
	logging.Info("Generated PR title", "title", prTitle)

	return branchName, prTitle, nil
}

// sanitizeBranchName creates a valid git branch name from a string
func sanitizeBranchName(input string) string {
	// Sanitize the title for use in a branch name
	sanitized := strings.ToLower(input)
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "")
	sanitized = strings.ReplaceAll(sanitized, ".", "")
	sanitized = strings.ReplaceAll(sanitized, ",", "")
	sanitized = strings.ReplaceAll(sanitized, "#", "")
	sanitized = strings.ReplaceAll(sanitized, "?", "")
	sanitized = strings.ReplaceAll(sanitized, "!", "")
	sanitized = strings.ReplaceAll(sanitized, "(", "")
	sanitized = strings.ReplaceAll(sanitized, ")", "")
	sanitized = strings.ReplaceAll(sanitized, "[", "")
	sanitized = strings.ReplaceAll(sanitized, "]", "")
	sanitized = strings.ReplaceAll(sanitized, "\"", "")
	sanitized = strings.ReplaceAll(sanitized, "'", "")
	sanitized = strings.ReplaceAll(sanitized, "`", "")

	// Remove consecutive dashes
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Trim dashes from the beginning and end
	sanitized = strings.Trim(sanitized, "-")

	// Limit branch name length
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
		// Ensure we don't end with a dash
		sanitized = strings.TrimRight(sanitized, "-")
	}

	return sanitized
}
