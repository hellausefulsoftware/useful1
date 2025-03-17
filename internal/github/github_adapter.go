// Package github provides GitHub-specific implementations of VCS interfaces
package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/common/vcs"
	"github.com/hellausefulsoftware/useful1/internal/config"
	"github.com/hellausefulsoftware/useful1/internal/logging"
	"golang.org/x/oauth2"
)

// Adapter provides a GitHub implementation of the vcs.Service interface
type Adapter struct {
	client   *github.Client
	config   *config.Config
	username string
}

// NewAdapter creates a new GitHub adapter
func NewAdapter(cfg *config.Config) (*Adapter, error) {
	if cfg.GitHub.Token == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	// Create GitHub client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GitHub.Token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	// Get username
	username := cfg.GitHub.User
	if username == "" {
		// Try to get username from API if not set
		logging.Info("Username not configured, getting from GitHub API...")
		if user, _, err := client.Users.Get(context.Background(), ""); err == nil && user.GetLogin() != "" {
			username = user.GetLogin()
			logging.Info("Retrieved username from GitHub API", "username", username)
		} else {
			logging.Error("Failed to get username from GitHub API", "error", err)
			return nil, fmt.Errorf("failed to get username from GitHub API: %w", err)
		}
	}

	return &Adapter{
		client:   client,
		config:   cfg,
		username: username,
	}, nil
}

// GetIssue retrieves a basic issue without comments
func (a *Adapter) GetIssue(owner, repo string, number int) (vcs.Issue, error) {
	ctx := context.Background()
	issue, _, err := a.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("error getting issue: %w", err)
	}

	return a.convertGitHubIssue(issue, owner, repo), nil
}

// GetIssueWithComments retrieves an issue with all its comments
func (a *Adapter) GetIssueWithComments(owner, repo string, number int) (vcs.Issue, error) {
	ctx := context.Background()

	// Get issue details
	issue, _, err := a.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("error getting issue: %w", err)
	}

	// Create base issue
	result := a.convertGitHubIssue(issue, owner, repo)

	// Get comments
	comments, _, err := a.client.Issues.ListComments(
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
	var vcsComments []vcs.IssueComment
	for _, comment := range comments {
		vcsComments = append(vcsComments, vcs.IssueComment{
			ID:        fmt.Sprintf("%d", comment.GetID()),
			User:      comment.User.GetLogin(),
			Body:      comment.GetBody(),
			CreatedAt: comment.GetCreatedAt(),
		})
	}

	// Add comments to issue
	baseIssue, ok := result.(*vcs.BaseIssue)
	if !ok {
		return nil, fmt.Errorf("failed to convert issue to BaseIssue type")
	}
	baseIssue.Comments = vcsComments

	return baseIssue, nil
}

// GetAssignedIssues retrieves issues assigned to a user since a specific time
func (a *Adapter) GetAssignedIssues(username string, since time.Time, limit int) ([]vcs.Issue, error) {
	// Only search for open issues assigned to the user
	query := fmt.Sprintf("assignee:%s updated:>%s is:issue is:open",
		username,
		since.Format(time.RFC3339),
	)

	logging.Info("Searching for assigned issues", "query", query)

	// Search for issues
	searchOpts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	// Perform the search
	ctx := context.Background()
	result, _, err := a.client.Search.Issues(ctx, query, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("error searching for issues: %w", err)
	}

	var issues []vcs.Issue

	// Convert GitHub issues to vcs.Issue
	for _, issue := range result.Issues {
		// Skip pull requests
		if issue.PullRequestLinks != nil {
			continue
		}

		// Extract owner/repo from issue URL
		parts := strings.Split(*issue.HTMLURL, "/")
		if len(parts) < 7 {
			logging.Warn("Skipping issue with invalid URL", "url", *issue.HTMLURL)
			continue
		}

		owner := parts[3]
		repo := parts[4]

		// Convert to vcs.Issue
		vcsIssue := &vcs.BaseIssue{
			Owner:     owner,
			Repo:      repo,
			Number:    *issue.Number,
			Title:     *issue.Title,
			Body:      *issue.Body,
			User:      *issue.User.Login,
			State:     *issue.State,
			CreatedAt: *issue.CreatedAt,
			UpdatedAt: *issue.UpdatedAt,
			URL:       *issue.HTMLURL,
		}

		issues = append(issues, vcsIssue)
	}

	return issues, nil
}

// RespondToIssue posts a comment on a GitHub issue
func (a *Adapter) RespondToIssue(owner, repo string, issueNumber int, comment string) error {
	_, _, err := a.client.Issues.CreateComment(
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

// GetRepository retrieves repository information
func (a *Adapter) GetRepository(owner, repo string) (vcs.Repository, error) {
	repoInfo, _, err := a.client.Repositories.Get(context.Background(), owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return &vcs.BaseRepository{
		Owner:         owner,
		Name:          repo,
		DefaultBranch: repoInfo.GetDefaultBranch(),
		URL:           repoInfo.GetHTMLURL(),
	}, nil
}

// GetDefaultBranch gets the default branch for a repository
func (a *Adapter) GetDefaultBranch(owner, repo string) (string, error) {
	repoInfo, _, err := a.client.Repositories.Get(context.Background(), owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}

	return repoInfo.GetDefaultBranch(), nil
}

// CloneRepository clones a GitHub repository to a local directory
func (a *Adapter) CloneRepository(owner, repo, branch string, issueNumber int) (string, error) {
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

	if repoExists {
		// Directory exists, update it
		currentDir, dirErr := os.Getwd()
		if dirErr != nil {
			return "", fmt.Errorf("failed to get current directory: %w", dirErr)
		}
		defer func() {
			if chDirErr := os.Chdir(currentDir); chDirErr != nil {
				logging.Warn("Failed to return to original directory", "error", chDirErr)
			}
		}() // Return to original directory when done

		if chDirErr := os.Chdir(tempDir); chDirErr != nil {
			return "", fmt.Errorf("failed to change to repository directory: %w", chDirErr)
		}

		// Fetch latest changes
		fetchCmd := exec.Command("git", "fetch", "origin")
		if _, fetchErr := fetchCmd.CombinedOutput(); fetchErr != nil {
			return "", fmt.Errorf("failed to fetch latest changes: %w", fetchErr)
		}

		// Checkout branch
		checkoutCmd := exec.Command("git", "checkout", branch)
		checkoutOutput, err := checkoutCmd.CombinedOutput()
		if err != nil {
			// Branch might not exist, create it
			createCmd := exec.Command("git", "checkout", "-b", branch)
			if _, err := createCmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("failed to create branch: %w\nOutput: %s", err, string(checkoutOutput))
			}
		}
	} else {
		// Clone the repository
		cloneCmd := exec.Command("git", "clone", repoURL, tempDir)
		cloneOutput, err := cloneCmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(cloneOutput))
		}

		// Change to repository directory
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		defer func() {
			if chDirErr := os.Chdir(currentDir); chDirErr != nil {
				logging.Warn("Failed to return to original directory", "error", chDirErr)
			}
		}() // Return to original directory when done

		if chDirErr := os.Chdir(tempDir); chDirErr != nil {
			return "", fmt.Errorf("failed to change to repository directory: %w", chDirErr)
		}

		// Create branch
		createCmd := exec.Command("git", "checkout", "-b", branch)
		if _, err := createCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to create branch: %w", err)
		}
	}

	return tempDir, nil
}

// CreateBranch creates a new branch from the specified base branch
func (a *Adapter) CreateBranch(owner, repo, branchName, baseBranch string) error {
	// Get the reference to the base branch
	baseRef, _, err := a.client.Git.GetRef(
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

	_, _, err = a.client.Git.CreateRef(
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

// CreateDraftPullRequest creates a new draft pull request
func (a *Adapter) CreateDraftPullRequest(owner, repo, title, body, head, base string) (vcs.PullRequest, error) {
	newPR := &github.NewPullRequest{
		Title: github.String(title),
		Body:  github.String(body),
		Head:  github.String(head),
		Base:  github.String(base),
		Draft: github.Bool(true), // Mark as draft
	}

	logging.Info("Creating draft PR",
		"owner", owner,
		"repo", repo,
		"title", title,
		"head", head,
		"base", base)

	pr, _, err := a.client.PullRequests.Create(
		context.Background(),
		owner,
		repo,
		newPR,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create draft PR: %w", err)
	}

	return &vcs.BasePullRequest{
		Number:     pr.GetNumber(),
		Title:      pr.GetTitle(),
		Body:       pr.GetBody(),
		State:      pr.GetState(),
		IsDraft:    pr.GetDraft(),
		User:       pr.User.GetLogin(),
		HeadBranch: head,
		BaseBranch: base,
		URL:        pr.GetHTMLURL(),
	}, nil
}

// GetPullRequestsForIssue gets all pull requests that reference an issue
func (a *Adapter) GetPullRequestsForIssue(owner, repo string, issueNumber int) ([]vcs.PullRequest, error) {
	// Search for PRs that mention the issue number in different formats
	query := fmt.Sprintf("repo:%s/%s is:pr #%d OR \"issue %d\" OR \"fixes %d\" OR \"closes %d\"",
		owner, repo, issueNumber, issueNumber, issueNumber, issueNumber)

	var vcsPRs []vcs.PullRequest
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	// Search for PRs
	result, resp, err := a.client.Search.Issues(context.Background(), query, opts)
	if err != nil {
		logging.Debug("Search error, falling back to listing all PRs", "error", err)
		return a.checkAllPullRequests(owner, repo, issueNumber)
	}

	if result.GetTotal() > 0 {
		for _, issue := range result.Issues {
			if issue.PullRequestLinks != nil {
				// This is a PR, not an issue
				pr, _, prErr := a.client.PullRequests.Get(
					context.Background(),
					owner,
					repo,
					*issue.Number,
				)

				if prErr != nil {
					logging.Debug("Error getting PR", "number", *issue.Number, "error", prErr)
					continue
				}

				vcsPRs = append(vcsPRs, &vcs.BasePullRequest{
					Number:     pr.GetNumber(),
					Title:      pr.GetTitle(),
					Body:       pr.GetBody(),
					State:      pr.GetState(),
					IsDraft:    pr.GetDraft(),
					User:       pr.User.GetLogin(),
					HeadBranch: pr.Head.GetRef(),
					BaseBranch: pr.Base.GetRef(),
					URL:        pr.GetHTMLURL(),
				})
			}
		}

		// Get next pages if available
		for resp != nil && resp.NextPage != 0 {
			opts.Page = resp.NextPage
			result, resp, err = a.client.Search.Issues(context.Background(), query, opts)
			if err != nil {
				break
			}

			for _, issue := range result.Issues {
				if issue.PullRequestLinks != nil {
					pr, _, err := a.client.PullRequests.Get(
						context.Background(),
						owner,
						repo,
						*issue.Number,
					)

					if err != nil {
						continue
					}

					vcsPRs = append(vcsPRs, &vcs.BasePullRequest{
						Number:     pr.GetNumber(),
						Title:      pr.GetTitle(),
						Body:       pr.GetBody(),
						State:      pr.GetState(),
						IsDraft:    pr.GetDraft(),
						User:       pr.User.GetLogin(),
						HeadBranch: pr.Head.GetRef(),
						BaseBranch: pr.Base.GetRef(),
						URL:        pr.GetHTMLURL(),
					})
				}
			}
		}
	} else {
		// If search returned no results, try listing all PRs
		additionalPRs, err := a.checkAllPullRequests(owner, repo, issueNumber)
		if err == nil {
			vcsPRs = append(vcsPRs, additionalPRs...)
		}
	}

	return vcsPRs, nil
}

// checkAllPullRequests gets all PRs in a repo and checks their bodies for issue references
func (a *Adapter) checkAllPullRequests(owner, repo string, issueNumber int) ([]vcs.PullRequest, error) {
	var vcsPRs []vcs.PullRequest
	opts := &github.PullRequestListOptions{
		State: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		prs, resp, err := a.client.PullRequests.List(
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
				vcsPRs = append(vcsPRs, &vcs.BasePullRequest{
					Number:     pr.GetNumber(),
					Title:      pr.GetTitle(),
					Body:       pr.GetBody(),
					State:      pr.GetState(),
					IsDraft:    pr.GetDraft(),
					User:       pr.User.GetLogin(),
					HeadBranch: pr.Head.GetRef(),
					BaseBranch: pr.Base.GetRef(),
					URL:        pr.GetHTMLURL(),
				})
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return vcsPRs, nil
}

// GetAuthenticatedUser gets the currently authenticated user
func (a *Adapter) GetAuthenticatedUser() (string, error) {
	// If we already have the username cached, return it
	if a.username != "" {
		return a.username, nil
	}

	// Otherwise, get it from the API
	user, _, err := a.client.Users.Get(context.Background(), "")
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	a.username = user.GetLogin()
	return a.username, nil
}

// GetRepositories gets a list of repositories the authenticated user has access to
func (a *Adapter) GetRepositories() ([]vcs.Repository, error) {
	var allRepos []vcs.Repository
	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		repos, resp, err := a.client.Repositories.List(context.Background(), "", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		// Convert GitHub repos to our Repository interface
		for _, repo := range repos {
			vcsRepo := &vcs.BaseRepository{
				Owner:           repo.GetOwner().GetLogin(),
				Name:            repo.GetName(),
				DefaultBranch:   repo.GetDefaultBranch(),
				URL:             repo.GetHTMLURL(),
				Description:     repo.GetDescription(),
				HasIssues:       repo.GetHasIssues(),
				StargazersCount: repo.GetStargazersCount(),
				ForksCount:      repo.GetForksCount(),
			}
			allRepos = append(allRepos, vcsRepo)
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// convertGitHubIssue converts a GitHub issue to vcs.Issue
func (a *Adapter) convertGitHubIssue(issue *github.Issue, owner, repo string) vcs.Issue {
	baseIssue := &vcs.BaseIssue{
		Owner:     owner,
		Repo:      repo,
		Number:    issue.GetNumber(),
		Title:     issue.GetTitle(),
		Body:      issue.GetBody(),
		User:      issue.User.GetLogin(),
		State:     issue.GetState(),
		CreatedAt: issue.GetCreatedAt(),
		UpdatedAt: issue.GetUpdatedAt(),
		URL:       issue.GetHTMLURL(),
		Comments:  []vcs.IssueComment{},
		Labels:    make([]string, 0, len(issue.Labels)),
		Assignees: make([]string, 0, len(issue.Assignees)),
	}

	// Add labels
	for _, label := range issue.Labels {
		if label.Name != nil {
			baseIssue.Labels = append(baseIssue.Labels, *label.Name)
		}
	}

	// Add assignees
	for _, assignee := range issue.Assignees {
		if assignee.Login != nil {
			baseIssue.Assignees = append(baseIssue.Assignees, *assignee.Login)
		}
	}

	return baseIssue
}
