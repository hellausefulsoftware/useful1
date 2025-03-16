package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v45/github"
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
