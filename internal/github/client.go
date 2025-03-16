package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/hellausefulsoftware/useful1/internal/logging"
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
	// Add draft marker to the title to make it a draft PR
	draftBody := body + "\n\n**DRAFT: This is an automatically generated draft PR. Do not merge.**"
	
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
				pr, _, err := c.client.PullRequests.Get(
					context.Background(),
					owner,
					repo,
					*issue.Number,
				)
				
				if err != nil {
					logging.Debug("Error getting PR", "number", *issue.Number, "error", err)
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
	
	// Create an initial commit on the new branch to ensure we can make a PR
	// 1. Get the README file to modify
	fileContent, _, _, err := c.client.Repositories.GetContents(
		context.Background(),
		owner,
		repo,
		"README.md",
		&github.RepositoryContentGetOptions{
			Ref: branchName,
		},
	)
	
	if err != nil {
		logging.Warn("Failed to get README.md for initial commit", "error", err)
		// Continue anyway - we'll try with a new file instead
	}
	
	// 2. Create a commit with a simple change (we'll add a .wip file to show work in progress)
	var commitMessage string
	var commitOptions github.RepositoryContentFileOptions
	
	if fileContent != nil {
		// If README exists, update it with a WIP note at the bottom
		currentContent, _ := fileContent.GetContent()
		newContent := currentContent + "\n\n<!-- WIP: Draft PR for issue tracking -->\n"
		
		commitMessage = "Start work on issue - setup draft PR"
		commitOptions = github.RepositoryContentFileOptions{
			Message: &commitMessage,
			Content: []byte(newContent),
			Branch:  &branchName,
			SHA:     fileContent.SHA,
		}
		
		_, _, err = c.client.Repositories.UpdateFile(
			context.Background(),
			owner,
			repo,
			"README.md",
			&commitOptions,
		)
	} else {
		// If no README or it failed, create a .wip file
		wipContent := "# Work in Progress\nThis branch is for tracking work on an issue."
		commitMessage = "Initialize draft PR branch"
		commitOptions = github.RepositoryContentFileOptions{
			Message: &commitMessage,
			Content: []byte(wipContent),
			Branch:  &branchName,
		}
		
		_, _, err = c.client.Repositories.CreateFile(
			context.Background(),
			owner,
			repo,
			".wip",
			&commitOptions,
		)
	}
	
	if err != nil {
		logging.Warn("Failed to create initial commit on branch", 
			"branch", branchName, 
			"error", err)
		// Continue anyway - the branch creation was successful
	} else {
		logging.Info("Created initial commit on branch", "branch", branchName)
	}
	
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
