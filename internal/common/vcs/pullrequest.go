// Package vcs provides interfaces and implementations for version control system interactions
package vcs

// PullRequest represents a generic pull request across VCS platforms
type PullRequest interface {
	GetNumber() int
	GetTitle() string
	GetBody() string
	GetState() string
	GetIsDraft() bool
	GetUser() string
	GetHeadBranch() string
	GetBaseBranch() string
	GetURL() string
}

// BasePullRequest provides a common implementation of PullRequest
type BasePullRequest struct {
	Number     int
	Title      string
	Body       string
	State      string
	IsDraft    bool
	User       string
	HeadBranch string
	BaseBranch string
	URL        string
}

// GetNumber returns the PR number
func (p *BasePullRequest) GetNumber() int { return p.Number }

// GetTitle returns the PR title
func (p *BasePullRequest) GetTitle() string { return p.Title }

// GetBody returns the PR body
func (p *BasePullRequest) GetBody() string { return p.Body }

// GetState returns the PR state (open/closed/merged)
func (p *BasePullRequest) GetState() string { return p.State }

// GetIsDraft returns whether the PR is a draft
func (p *BasePullRequest) GetIsDraft() bool { return p.IsDraft }

// GetUser returns the username of the PR creator
func (p *BasePullRequest) GetUser() string { return p.User }

// GetHeadBranch returns the head branch
func (p *BasePullRequest) GetHeadBranch() string { return p.HeadBranch }

// GetBaseBranch returns the base branch
func (p *BasePullRequest) GetBaseBranch() string { return p.BaseBranch }

// GetURL returns the PR URL
func (p *BasePullRequest) GetURL() string { return p.URL }
