package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/anthropic"
	"github.com/hellausefulsoftware/useful1/internal/cli"
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
	logging.Debug("Creating new GitHub client", "token_exists", token != "", "token_length", len(token))
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	c := &Client{
		client: github.NewClient(tc),
	}
	logging.Debug("GitHub client created successfully")
	return c
}

// RespondToIssue posts a comment on a GitHub issue
func (c *Client) RespondToIssue(owner, repo string, issueNumber int, comment string) error {
	logging.Debug("RespondToIssue called", "owner", owner, "repo", repo, "issue", issueNumber)
	// This client was created with a GitHub token, so check if it's valid
	ts, ok := c.client.BaseURL.User.Password()
	logging.Debug("GitHub token details", "token_exists", ok, "token_length", len(ts))

	resp, _, err := c.client.Issues.CreateComment(
		context.Background(),
		owner,
		repo,
		issueNumber,
		&github.IssueComment{
			Body: github.String(comment),
		},
	)

	if err != nil {
		logging.Error("Failed to create issue comment", "error", err)
		return fmt.Errorf("failed to create issue comment: %w", err)
	}

	logging.Debug("Successfully created issue comment", "comment_id", resp.GetID())
	return nil
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

// CloneRepository clones a GitHub repository to a local directory
func (c *Client) CloneRepository(owner, repo, branch string, issueNumber int) (string, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user's home directory: %w", err)
	}

	// Create temp directory path
	tempDir := fmt.Sprintf("%s/.useful1/temp/%s_%d", homeDir, repo, issueNumber)

	// Create parent directory if it doesn't exist
	baseDir := fmt.Sprintf("%s/.useful1/temp", homeDir)
	err = os.MkdirAll(baseDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create base temp directory: %w", err)
	}

	// Check if directory already exists
	_, err = os.Stat(tempDir)
	repoExists := !os.IsNotExist(err)

	repoURL := fmt.Sprintf("git@github.com:%s/%s.git", owner, repo)
	logging.Info("Repository operations",
		"owner", owner,
		"repo", repo,
		"branch", branch,
		"issue", issueNumber,
		"dir", tempDir,
		"exists", repoExists)

	// Handle existing repositories
	if repoExists {
		// Check if the existing directory is a valid git repository
		// If not, remove it and clone fresh
		isValid, validateErr := c.validateGitRepository(tempDir)
		if validateErr != nil {
			return "", validateErr
		}

		if !isValid {
			// Not a valid git repo, need to remove it and clone fresh
			repoExists = false
		} else {
			// It's a valid repository, update it
			err = c.updateExistingRepository(tempDir, branch)
			if err != nil {
				return "", err
			}
		}
	}

	// Handle non-existing or invalid repositories that need to be cloned
	if !repoExists {
		err = c.cloneFreshRepository(repoURL, tempDir, branch)
		if err != nil {
			return "", err
		}
	}

	logging.Info("Repository ready", "dir", tempDir, "branch", branch)
	return tempDir, nil
}

// validateGitRepository checks if a directory is a valid git repository
func (c *Client) validateGitRepository(repoDir string) (bool, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() {
		if chDirErr := os.Chdir(currentDir); chDirErr != nil {
			fmt.Printf("WARNING: Failed to return to original directory: %v\n", chDirErr)
		}
	}() // Return to original directory when done

	if chDirErr := os.Chdir(repoDir); chDirErr != nil {
		return false, fmt.Errorf("failed to change to repository directory: %w", chDirErr)
	}

	// Check if this is a valid git repository
	checkGitCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	checkOut, err := checkGitCmd.CombinedOutput()

	if err != nil || strings.TrimSpace(string(checkOut)) != "true" {
		// Not a valid git repository, remove it
		logging.Warn("Directory exists but is not a valid git repository, removing it",
			"dir", repoDir)

		// Go back to original directory before removing
		if chDirErr := os.Chdir(currentDir); chDirErr != nil {
			logging.Warn("Failed to return to original directory", "error", chDirErr)
		}

		// Remove the invalid repository directory
		if err := os.RemoveAll(repoDir); err != nil {
			return false, fmt.Errorf("failed to remove invalid repository directory: %w", err)
		}

		return false, nil
	}

	return true, nil
}

// updateExistingRepository updates an existing git repository
func (c *Client) updateExistingRepository(repoDir, branch string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer func() {
		if chDirErr := os.Chdir(currentDir); chDirErr != nil {
			fmt.Printf("WARNING: Failed to return to original directory: %v\n", chDirErr)
		}
	}() // Return to original directory when done

	if chDirErr := os.Chdir(repoDir); chDirErr != nil {
		return fmt.Errorf("failed to change to repository directory: %w", chDirErr)
	}

	logging.Info("Repository directory exists and is valid, attempting to update", "dir", repoDir)

	// Fetch and checkout the branch
	// First, make sure we have the latest from remote
	fetchCmd := exec.Command("git", "fetch", "origin")
	fetchOut, err := fetchCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fetch latest changes: %w\nOutput: %s", err, string(fetchOut))
	}

	// Try to checkout the branch (it may already be checked out)
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutOut, err := checkoutCmd.CombinedOutput()
	if err != nil {
		// Branch might not exist locally yet
		checkoutTrackCmd := exec.Command("git", "checkout", "-b", branch, "--track", "origin/"+branch)
		trackOut, trackErr := checkoutTrackCmd.CombinedOutput()
		if trackErr != nil {
			logging.Warn("Failed to checkout tracking branch",
				"error", trackErr,
				"output", string(trackOut),
				"branch", branch)
			// Try to create the branch locally
			createBranchCmd := exec.Command("git", "checkout", "-b", branch)
			createOut, createErr := createBranchCmd.CombinedOutput()
			if createErr != nil {
				return fmt.Errorf("failed to create branch locally: %w\nOutput: %s", createErr, string(createOut))
			}
		}
	} else {
		logging.Info("Checked out branch", "branch", branch, "output", string(checkoutOut))
	}

	// Pull latest changes
	pullCmd := exec.Command("git", "pull", "origin", branch)
	pullOut, err := pullCmd.CombinedOutput()
	if err != nil {
		logging.Warn("Failed to pull latest changes, may be a new branch",
			"error", err,
			"output", string(pullOut),
			"branch", branch)
	} else {
		logging.Info("Pulled latest changes from remote", "branch", branch)
	}

	return nil
}

// cloneFreshRepository clones a fresh git repository
func (c *Client) cloneFreshRepository(repoURL, repoDir, branch string) error {
	// Clone the repository - let git clone create the directory structure
	logging.Info("Cloning repository", "url", repoURL, "dir", repoDir)

	cloneCmd := exec.Command("git", "clone", repoURL, repoDir)
	cloneOut, err := cloneCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(cloneOut))
	}

	// Change to repo directory
	currentDir, dirErr := os.Getwd()
	if dirErr != nil {
		return fmt.Errorf("failed to get current directory: %w", dirErr)
	}
	defer func() {
		if chDirErr := os.Chdir(currentDir); chDirErr != nil {
			fmt.Printf("WARNING: Failed to return to original directory: %v\n", chDirErr)
		}
	}() // Return to original directory when done

	if chDirErr := os.Chdir(repoDir); chDirErr != nil {
		return fmt.Errorf("failed to change to repository directory: %w", chDirErr)
	}

	// Checkout the branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutOut, checkoutErr := checkoutCmd.CombinedOutput()
	if checkoutErr != nil {
		// Branch might not exist locally yet
		fetchCmd := exec.Command("git", "fetch", "origin", branch+":"+branch)
		fetchOut, fetchErr := fetchCmd.CombinedOutput()
		if fetchErr != nil {
			logging.Warn("Failed to fetch branch",
				"error", fetchErr,
				"output", string(fetchOut),
				"branch", branch)

			// Create the branch
			createCmd := exec.Command("git", "checkout", "-b", branch)
			createOut, createErr := createCmd.CombinedOutput()
			if createErr != nil {
				return fmt.Errorf("failed to create branch: %w\nOutput: %s", createErr, string(createOut))
			}
		} else {
			// Try checkout again after fetch
			retryCmd := exec.Command("git", "checkout", branch)
			retryOut, retryErr := retryCmd.CombinedOutput()
			if retryErr != nil {
				return fmt.Errorf("failed to checkout branch after fetch: %w\nOutput: %s", retryErr, string(retryOut))
			}
		}
	} else {
		logging.Info("Checked out branch", "branch", branch, "output", string(checkoutOut))
	}

	return nil
}

// GetIssueDetails retrieves full details of an issue including comments
func (c *Client) GetIssueDetails(owner, repo string, number int) (*models.Issue, error) {
	ctx := context.Background()

	// Get issue details
	issue, _, err := c.client.Issues.Get(ctx, owner, repo, number)
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

	// Get comments
	comments, _, err := c.client.Issues.ListComments(
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
		// We'll continue even without comments
		logging.Warn("Failed to get issue comments", "error", err)
	} else {
		// Process comments
		for _, comment := range comments {
			// Add to our comments list
			result.Comments = append(result.Comments, &models.IssueComment{
				ID:        *comment.ID,
				User:      *comment.User.Login,
				Body:      *comment.Body,
				CreatedAt: *comment.CreatedAt,
			})
		}
	}

	return result, nil
}

// CreateImplementationFile creates an implementation plan as developer instructions and passes it to the CLI tool
func (c *Client) CreateImplementationFile(owner, repo, branchName string, issueNumber int) error {
	// Create a partial issue object to get started
	issue := &models.Issue{
		Owner:  owner,
		Repo:   repo,
		Number: issueNumber,
		Title:  fmt.Sprintf("Issue #%d", issueNumber), // Will be replaced if we get full details
		Body:   "Description not available",           // Will be replaced if we get full details
	}

	// Clone or update the repository and get the directory
	repoDir, err := c.CloneRepository(owner, repo, branchName, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to prepare repository: %w", err)
	}

	logging.Info("Creating implementation plan for issue",
		"owner", owner,
		"repo", repo,
		"branch", branchName,
		"issue", issueNumber,
		"dir", repoDir)

	// Get the full issue details to generate an implementation plan
	fullIssue, err := c.GetIssueDetails(issue.Owner, issue.Repo, issue.Number)
	if err != nil {
		logging.Warn("Failed to get full issue details, using limited issue data",
			"error", err)
		// Continue with the partial issue data we have
	} else {
		// Update issue with the full details
		issue = fullIssue
	}

	// Generate an implementation plan using Anthropic API if token is available
	var implementationContent string

	// Access config to get Anthropic API key
	cfg, err := config.LoadConfig()
	if err != nil || cfg.Anthropic.Token == "" {
		logging.Warn("Failed to load config or Anthropic API token not available",
			"error", err)
		logging.Warn("Config generated %v", cfg)
		// Use a simple default implementation placeholder
		implementationContent = fmt.Sprintf("# Implementation Plan for Issue #%d: %s\n\n",
			issue.Number, issue.Title)
		implementationContent += "## Problem Description\n\n"
		implementationContent += issue.Body + "\n\n"
		implementationContent += "## Implementation Notes\n\n"
		implementationContent += "The implementation details will be added here.\n"
	} else {
		// Create Anthropic analyzer with proper config
		analyzer := anthropic.NewAnalyzer(cfg)
		logging.Info("Created Anthropic analyzer for implementation plan",
			"token_available", cfg.Anthropic.Token != "",
			"token_length", len(cfg.Anthropic.Token))

		// Generate the implementation plan
		plan, planErr := analyzer.GenerateImplementationPlan(issue)
		if planErr != nil {
			logging.Warn("Failed to generate AI implementation plan, using fallback",
				"error", planErr)
			// Use fallback implementation placeholder
			implementationContent = fmt.Sprintf("# Implementation Plan for Issue #%d: %s\n\n",
				issue.Number, issue.Title)
			implementationContent += "## Problem Description\n\n"
			implementationContent += issue.Body + "\n\n"
			implementationContent += "## Implementation Notes\n\n"
			implementationContent += "The implementation details will be added here.\n"
		} else {
			// Format the AI-generated implementation plan
			implementationContent = fmt.Sprintf("# Developer Instructions for Issue #%d: %s\n\n",
				issue.Number, issue.Title)
			implementationContent += plan
			implementationContent += "\n\n---\n*Generated with Claude 3.7 Sonnet*"
			logging.Info("Successfully generated AI implementation plan",
				"plan_length", len(plan))
		}
	}

	// Change to repo directory to run git commands
	currentDir, dirErr := os.Getwd()
	if dirErr != nil {
		return fmt.Errorf("failed to get current directory: %w", dirErr)
	}
	defer func() {
		if chDirErr := os.Chdir(currentDir); chDirErr != nil {
			fmt.Printf("WARNING: Failed to return to original directory: %v\n", chDirErr)
		}
	}() // Return to original directory when done

	if chDirErr := os.Chdir(repoDir); chDirErr != nil {
		return fmt.Errorf("failed to change to repository directory: %w", chDirErr)
	}

	// Generate a description for the implementation (unused in this flow but kept for logging)
	changeSummary := "Created an implementation plan to address the issue"

	// No need to generate a commit message since we're not committing anything
	logging.Info("Preparing to pass implementation plan directly to CLI",
		"issue", issue.Number,
		"plan_summary", changeSummary,
		"plan_length", len(implementationContent))

	// Create a temporary file in the current directory to store the issue details
	issueDetailFile, err := os.CreateTemp("", "issue-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary issue detail file: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(issueDetailFile.Name()); removeErr != nil {
			logging.Warn("Failed to remove temporary issue detail file", "file", issueDetailFile.Name(), "error", removeErr)
		}
	}()

	// Write issue details to the file
	issueContent := fmt.Sprintf("Issue #%d: %s\n\n%s", issue.Number, issue.Title, issue.Body)
	if _, writeErr := issueDetailFile.WriteString(issueContent); writeErr != nil {
		return fmt.Errorf("failed to write to issue detail file: %w", writeErr)
	}
	if closeErr := issueDetailFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to close issue detail file: %w", closeErr)
	}

	// Create metadata for the CLI tool
	metadata := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"owner":     owner,
		"repo":      repo,
		"issue":     issueNumber,
		"url":       fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, issueNumber),
		"branch":    branchName,
	}

	// Create a temporary metadata file
	metadataFile, err := os.CreateTemp("", "metadata-*.json")
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer func() {
		if removeErr := os.Remove(metadataFile.Name()); removeErr != nil {
			logging.Warn("Failed to remove metadata file", "file", metadataFile.Name(), "error", removeErr)
		}
	}()

	// Write metadata to the file
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if _, writeErr := metadataFile.Write(metadataBytes); writeErr != nil {
		return fmt.Errorf("failed to write metadata: %w", writeErr)
	}
	if closeErr := metadataFile.Close(); closeErr != nil {
		return fmt.Errorf("failed to close metadata file: %w", closeErr)
	}

	// Execute the CLI tool with the implementation plan as a prompt
	// Use ExecuteCommand function directly without importing cli package
	// Prepare arguments for the command
	args := []string{
		"respond",
		"--issue-file", issueDetailFile.Name(),
		"--owner", owner,
		"--repo", repo,
		"--number", fmt.Sprintf("%d", issueNumber),
		"--budget", "10.00",
		"--metadata", metadataFile.Name(),
		"-p", implementationContent,
	}

	logging.Info("Executing Claude CLI with implementation plan as prompt",
		"args", args,
		"plan_length", len(implementationContent))

	// Get config to determine if we should skip permissions check
	cfg, err = config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create executor to handle CLI command execution
	executor := cli.NewExecutor(cfg)

	// Set up the arguments - only pass -p for the prompt
	// The executor will handle adding the command and any config-based args
	args = []string{"-p", implementationContent}

	// Execute the CLI tool using the executor
	var output string
	output, err = executor.ExecuteWithOutput(args)
	if err != nil {
		logging.Error("Failed to execute Claude CLI with implementation plan",
			"error", err,
			"output", output)
		return fmt.Errorf("failed to execute Claude CLI: %w", err)
	}

	logging.Info("Successfully executed Claude CLI with implementation plan",
		"output_length", len(output))

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
