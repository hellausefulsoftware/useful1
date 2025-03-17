// Package services provides service implementations for the workflow package
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"

	"github.com/hellausefulsoftware/useful1/internal/anthropic"
	"github.com/hellausefulsoftware/useful1/internal/cli"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"github.com/hellausefulsoftware/useful1/internal/models"
)

// GitHubImplementationService provides GitHub implementation services
type GitHubImplementationService struct {
	config *config.Config
}

// NewGitHubImplementationService creates a new GitHub implementation service
func NewGitHubImplementationService(cfg *config.Config) *GitHubImplementationService {
	return &GitHubImplementationService{
		config: cfg,
	}
}

// CreateImplementationPromptAndExecute creates an implementation plan and executes it using CLI
func (s *GitHubImplementationService) CreateImplementationPromptAndExecute(owner, repo, branchName string, issueNumber int) error {
	// Create a partial issue object to get started
	issue := &models.Issue{
		Owner:  owner,
		Repo:   repo,
		Number: issueNumber,
		Title:  fmt.Sprintf("Issue #%d", issueNumber), // Will be replaced if we get full details
		Body:   "Description not available",           // Will be replaced if we get full details
	}

	// Clone or update the repository and get the directory
	repoDir, err := s.cloneRepository(owner, repo, branchName, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to prepare repository: %w", err)
	}

	logging.Info("Creating implementation plan for issue",
		"owner", owner,
		"repo", repo,
		"branch", branchName,
		"issue", issueNumber,
		"dir", repoDir)

	// Create GitHub client
	githubClient := createGitHubClient(s.config)
	
	// Get the full issue details to generate an implementation plan
	fullIssue, err := s.getIssueDetails(githubClient, issue.Owner, issue.Repo, issue.Number)
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

	if s.config.Anthropic.Token == "" {
		// Use a simple default implementation placeholder
		implementationContent = fmt.Sprintf("# Implementation Plan for Issue #%d: %s\n\n",
			issue.Number, issue.Title)
		implementationContent += "## Problem Description\n\n"
		implementationContent += issue.Body + "\n\n"
		implementationContent += "## Implementation Notes\n\n"
		implementationContent += "The implementation details will be added here.\n"
	} else {
		// Create Anthropic analyzer with proper config
		analyzer := anthropic.NewAnalyzer(s.config)
		logging.Info("Created Anthropic analyzer for implementation plan",
			"token_available", s.config.Anthropic.Token != "",
			"token_length", len(s.config.Anthropic.Token))

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
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(currentDir) // Return to original directory when done

	if err := os.Chdir(repoDir); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
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

	logging.Info("Executing Claude CLI with implementation plan as prompt",
		"plan_length", len(implementationContent))

	// Create executor to handle CLI command execution
	executor := cli.NewExecutor(s.config)
	
	// Set up the arguments - only pass -p for the prompt
	// The executor will handle adding the command and any config-based args
	args := []string{"-p", implementationContent}
	
	// Execute the CLI tool using the executor
	output, err := executor.ExecuteWithOutput(args)
	if err != nil {
		logging.Error("Failed to execute Claude CLI with implementation plan",
			"error", err,
			"output", output)
		return fmt.Errorf("failed to execute Claude CLI: %w", err)
	}

	// Log the Claude CLI output for debugging
	logging.Info("Successfully executed Claude CLI with implementation plan",
		"output_length", len(output),
		"claude_output", output)
	logging.Debug("Claude CLI full output", "output", output)
	
	// Check if the git repo has any changes
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		logging.Error("Failed to check git status", "error", err)
		return fmt.Errorf("failed to check git status: %w", err)
	}
	
	// If there are changes, commit them
	if len(statusOut) > 0 {
		logging.Info("Repository has changes, preparing to commit")
		
		// List the changed files for the commit message
		changedFiles := []string{}
		for _, line := range strings.Split(string(statusOut), "\n") {
			if len(line) > 3 {
				changedFiles = append(changedFiles, strings.TrimSpace(line[2:]))
			}
		}
		
		// Create the Anthropic analyzer for commit message generation
		analyzer := anthropic.NewAnalyzer(s.config)
		
		// Generate a commit message
		commitMsg, err := analyzer.GenerateCommitMessage(
			&models.Issue{
				Number: issueNumber,
				Title: issue.Title,
				Body: issue.Body,
			},
			changedFiles,
			"Created files to implement solution")
			
		if err != nil {
			// Fallback commit message if generation fails
			commitMsg = fmt.Sprintf("feat: implement solution for issue #%d", issueNumber)
			logging.Warn("Failed to generate commit message, using fallback", 
				"error", err,
				"fallback", commitMsg)
		}
		
		// Add all changes
		addCmd := exec.Command("git", "add", ".")
		addOut, err := addCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to git add: %w\nOutput: %s", err, string(addOut))
		}
		
		// Commit the changes
		commitCmd := exec.Command("git", "commit", "-m", commitMsg)
		commitOut, err := commitCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to commit: %w\nOutput: %s", err, string(commitOut))
		}
		logging.Info("Successfully committed changes", "message", commitMsg)
		
		// Push to the branch
		pushCmd := exec.Command("git", "push", "origin", branchName)
		pushOut, err := pushCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to push: %w\nOutput: %s", err, string(pushOut))
		}
		logging.Info("Successfully pushed changes to remote")
	} else {
		logging.Info("No changes detected in repository after Claude CLI execution")
		
		// Check for unpushed commits before proceeding
		unpushedCmd := exec.Command("git", "rev-list", "@{u}..", "--count")
		unpushedOut, unpushedErr := unpushedCmd.CombinedOutput()
		
		if unpushedErr == nil {
			unpushedCount := strings.TrimSpace(string(unpushedOut))
			if unpushedCount != "0" {
				logging.Info("Found unpushed commits, pushing to remote", "count", unpushedCount)
				
				// Push commits to the remote branch
				pushCmd := exec.Command("git", "push", "origin", branchName)
				pushOut, pushErr := pushCmd.CombinedOutput()
				if pushErr != nil {
					logging.Warn("Failed to push commits", 
						"error", pushErr, 
						"output", string(pushOut))
				} else {
					logging.Info("Successfully pushed commits to remote branch")
				}
			} else {
				logging.Warn("No changes or unpushed commits found - PR creation may fail without commits")
			}
		} else {
			logging.Warn("Failed to check for unpushed commits - PR creation may fail without commits",
				"error", unpushedErr,
				"output", string(unpushedOut))
		}
	}
	
	return nil
}

// GenerateBranchAndTitle generates a branch name and PR title
func (s *GitHubImplementationService) GenerateBranchAndTitle(owner, repo, title, body string) (string, string, error) {
	logging.Info("Generating branch name and title",
		"owner", owner,
		"repo", repo,
		"title_length", len(title),
		"body_length", len(body))

	// Extract issue number from title if present
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

	// Create the issue model
	issueModel := &models.Issue{
		Owner:  owner,
		Repo:   repo,
		Number: issueNum,
		Title:  title,
		Body:   body,
	}

	// Use Anthropic API to generate branch name
	analyzer := anthropic.NewAnalyzer(s.config)
	branchName, err := analyzer.AnalyzeIssue(issueModel)
	if err != nil {
		logging.Warn("Failed to generate branch name with Anthropic API, falling back to simple generation", 
			"error", err)
		// Fall back to the default branch name
		branchName = fmt.Sprintf("feature/%s", sanitizeBranchName(title))
	}

	// Determine issue type from branch name prefix
	var issueType string
	if strings.HasPrefix(branchName, "bugfix/") {
		issueType = "bugfix"
	} else if strings.HasPrefix(branchName, "feature/") {
		issueType = "feature"
	} else if strings.HasPrefix(branchName, "chore/") {
		issueType = "chore"
	} else {
		issueType = "feature" // Default
	}

	// Create an appropriate PR title
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

	logging.Info("Generated branch name using Anthropic API",
		"branch", branchName,
		"type", issueType)
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

// createGitHubClient creates a GitHub client from config
func createGitHubClient(cfg *config.Config) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GitHub.Token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return github.NewClient(tc)
}

// cloneRepository clones a GitHub repository to a local directory
func (s *GitHubImplementationService) cloneRepository(owner, repo, branch string, issueNumber int) (string, error) {
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
		isValid, err := s.validateGitRepository(tempDir)
		if err != nil {
			return "", err
		}

		if !isValid {
			// Not a valid git repo, need to remove it and clone fresh
			repoExists = false
		} else {
			// It's a valid repository, update it
			err = s.updateExistingRepository(tempDir, branch)
			if err != nil {
				return "", err
			}
		}
	}

	// Handle non-existing or invalid repositories that need to be cloned
	if !repoExists {
		err = s.cloneFreshRepository(repoURL, tempDir, branch)
		if err != nil {
			return "", err
		}
	}

	logging.Info("Repository ready", "dir", tempDir, "branch", branch)
	return tempDir, nil
}

// validateGitRepository checks if a directory is a valid git repository
func (s *GitHubImplementationService) validateGitRepository(repoDir string) (bool, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(currentDir) // Return to original directory when done

	if err := os.Chdir(repoDir); err != nil {
		return false, fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Check if this is a valid git repository
	checkGitCmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	checkOut, err := checkGitCmd.CombinedOutput()

	if err != nil || strings.TrimSpace(string(checkOut)) != "true" {
		// Not a valid git repository, remove it
		logging.Warn("Directory exists but is not a valid git repository, removing it",
			"dir", repoDir)

		// Go back to original directory before removing
		os.Chdir(currentDir)

		// Remove the invalid repository directory
		if err := os.RemoveAll(repoDir); err != nil {
			return false, fmt.Errorf("failed to remove invalid repository directory: %w", err)
		}

		return false, nil
	}

	return true, nil
}

// updateExistingRepository updates an existing git repository
func (s *GitHubImplementationService) updateExistingRepository(repoDir, branch string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(currentDir) // Return to original directory when done

	if err := os.Chdir(repoDir); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
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

// getIssueDetails retrieves full details of an issue including comments
func (s *GitHubImplementationService) getIssueDetails(client *github.Client, owner, repo string, number int) (*models.Issue, error) {
	ctx := context.Background()

	// Get issue details
	issue, _, err := client.Issues.Get(ctx, owner, repo, number)
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
	comments, _, err := client.Issues.ListComments(
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

// cloneFreshRepository clones a fresh git repository
func (s *GitHubImplementationService) cloneFreshRepository(repoURL, repoDir, branch string) error {
	// Clone the repository - let git clone create the directory structure
	logging.Info("Cloning repository", "url", repoURL, "dir", repoDir)

	cloneCmd := exec.Command("git", "clone", repoURL, repoDir)
	cloneOut, err := cloneCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(cloneOut))
	}

	// Change to repo directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(currentDir) // Return to original directory when done

	if err := os.Chdir(repoDir); err != nil {
		return fmt.Errorf("failed to change to repository directory: %w", err)
	}

	// Checkout the branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutOut, err := checkoutCmd.CombinedOutput()
	if err != nil {
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